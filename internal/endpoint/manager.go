package endpoint

import (
	"fmt"
	"log"
	"sync"
	"time"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/statistics"
)

type HealthChecker interface {
	CheckEndpoint(ep *Endpoint) error
}

type Manager struct {
	selector          *Selector
	endpoints         []*Endpoint
	config            *config.Config
	mutex             sync.RWMutex
	healthChecker     HealthChecker
	healthTickers     map[string]*time.Ticker
	statisticsManager statistics.StatisticsManager
}

func NewManager(cfg *config.Config) (*Manager, error) {
	// Initialize statistics manager
	// Use log directory for statistics database, or current directory as fallback
	dataDirectory := cfg.Logging.LogDirectory
	if dataDirectory == "" {
		dataDirectory = "." // fallback to current directory
	}

	statisticsManager, err := statistics.NewStatisticsManager(dataDirectory)
	if err != nil {
		log.Printf("ERROR: Failed to initialize statistics manager: %v", err)
		return nil, fmt.Errorf("failed to initialize statistics manager: %w", err)
	}

	endpoints := make([]*Endpoint, 0, len(cfg.Endpoints))
	for _, endpointConfig := range cfg.Endpoints {
		endpoint := NewEndpoint(endpointConfig)
		
		// Initialize or inherit statistics data
		if err := initializeEndpointStatistics(endpoint, statisticsManager); err != nil {
			log.Printf("ERROR: Failed to initialize statistics for endpoint %s: %v", 
				endpoint.Name, err)
			return nil, fmt.Errorf("failed to initialize statistics for endpoint %s: %w", endpoint.Name, err)
		}
		
		endpoints = append(endpoints, endpoint)
	}

	manager := &Manager{
		selector:          NewSelector(endpoints),
		endpoints:         endpoints,
		config:            cfg,
		healthChecker:     nil, // 稍后设置
		healthTickers:     make(map[string]*time.Ticker),
		statisticsManager: statisticsManager,
	}

	return manager, nil
}

func (m *Manager) GetEndpoint() (*Endpoint, error) {
	return m.selector.SelectEndpoint()
}

// GetEndpointWithTags 根据tags选择endpoint
func (m *Manager) GetEndpointWithTags(tags []string) (*Endpoint, error) {
	return m.selector.SelectEndpointWithTags(tags)
}

// GetEndpointWithFormat 根据请求格式选择兼容的端点
// requestFormat: "anthropic" | "openai" | "unknown"
func (m *Manager) GetEndpointWithFormat(requestFormat string) (*Endpoint, error) {
	return m.selector.SelectEndpointWithFormat(requestFormat)
}

// GetEndpointWithFormatAndClient 根据请求格式和客户端类型选择兼容的端点
func (m *Manager) GetEndpointWithFormatAndClient(requestFormat string, clientType string) (*Endpoint, error) {
	return m.selector.SelectEndpointWithFormatAndClient(requestFormat, clientType)
}

// GetEndpointWithTagsAndFormat 根据tags和格式选择端点
func (m *Manager) GetEndpointWithTagsAndFormat(tags []string, requestFormat string) (*Endpoint, error) {
	return m.selector.SelectEndpointWithTagsAndFormat(tags, requestFormat)
}

// GetEndpointWithTagsFormatAndClient 根据tags、格式和客户端类型选择端点
func (m *Manager) GetEndpointWithTagsFormatAndClient(tags []string, requestFormat string, clientType string) (*Endpoint, error) {
	return m.selector.SelectEndpointWithTagsFormatAndClient(tags, requestFormat, clientType)
}

func (m *Manager) GetAllEndpoints() []*Endpoint {
	return m.selector.GetAllEndpoints()
}

func (m *Manager) RecordRequest(endpointID string, success bool, requestID string, firstByteTime time.Duration, responseTime time.Duration) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, endpoint := range m.endpoints {
		if endpoint.ID == endpointID {
			// Update in-memory statistics
			endpoint.RecordRequest(success, requestID, firstByteTime, responseTime)

			// Update database statistics if statistics manager is available
			if m.statisticsManager != nil {
				if err := m.statisticsManager.RecordRequest(endpointID, success); err != nil {
					// Log error but don't fail the operation
					// Statistics persistence failure should not break request processing
					log.Printf("WARNING: Failed to persist statistics for endpoint %s: %v",
						endpointID, err)
				}
			}
			break
		}
	}
}

func (m *Manager) UpdateEndpoints(endpointConfigs []config.EndpointConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create map of existing endpoints by name for intelligent matching
	existingEndpointsByName := make(map[string]*Endpoint)
	for _, endpoint := range m.endpoints {
		existingEndpointsByName[endpoint.Name] = endpoint
	}

	newEndpoints := make([]*Endpoint, 0, len(endpointConfigs))
	for _, cfg := range endpointConfigs {
		// Check if an endpoint with the same name already exists
		if existingEndpoint, exists := existingEndpointsByName[cfg.Name]; exists {
			// Same name endpoint exists - preserve statistics and update configuration
			endpoint := m.updateExistingEndpoint(existingEndpoint, cfg)
			newEndpoints = append(newEndpoints, endpoint)
		} else {
			// New endpoint - create fresh with inherited statistics from database
			endpoint := NewEndpoint(cfg)
			if m.statisticsManager != nil {
				if err := initializeEndpointStatistics(endpoint, m.statisticsManager); err != nil {
					log.Printf("WARNING: Failed to load statistics for new endpoint %s: %v",
						cfg.Name, err)
				} else if endpoint.TotalRequests > 0 {
					log.Printf("Inherited statistics for endpoint %s: TotalRequests=%d",
						cfg.Name, endpoint.TotalRequests)
				}
			}
			newEndpoints = append(newEndpoints, endpoint)
		}
	}

	// Clean up statistics for endpoints that were removed
	if m.statisticsManager != nil {
		m.cleanupRemovedEndpoints(endpointConfigs)
	}

	// 停止旧的健康检查
	m.stopHealthChecks()

	m.endpoints = newEndpoints
	m.selector.UpdateEndpoints(newEndpoints)

	// 设置动态排序器引用并触发更新
	m.setDynamicSorterForEndpoints()

	// 重新启动健康检查
	m.startHealthChecks()
}


func (m *Manager) SetHealthChecker(checker HealthChecker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.healthChecker = checker
	
	// 启动健康检查
	m.startHealthChecks()
}

// ResetEndpointStatus resets an endpoint's status to active and clears failure statistics
func (m *Manager) ResetEndpointStatus(endpointName string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, endpoint := range m.endpoints {
		if endpoint.Name == endpointName {
			endpoint.MarkActive()
			return nil
		}
	}

	return fmt.Errorf("endpoint not found: %s", endpointName)
}

func (m *Manager) startHealthChecks() {
	// 如果没有健康检查器，不启动
	if m.healthChecker == nil {
		return
	}

	// 获取健康检查间隔配置，使用统一默认值
	interval := config.GetTimeoutDuration(m.config.Timeouts.CheckInterval, config.GetTimeoutDuration(config.Default.Timeouts.CheckInterval, 30*time.Second))
	
	for _, endpoint := range m.endpoints {
		if endpoint.Enabled {
			ticker := time.NewTicker(interval)
			m.healthTickers[endpoint.ID] = ticker
			
			go m.runHealthCheck(endpoint, ticker)
		}
	}
}

func (m *Manager) stopHealthChecks() {
	for _, ticker := range m.healthTickers {
		ticker.Stop()
	}
	m.healthTickers = make(map[string]*time.Ticker)
}

func (m *Manager) runHealthCheck(endpoint *Endpoint, ticker *time.Ticker) {
	// 获取恢复阈值配置，使用统一默认值
	recoveryThreshold := config.GetIntWithDefault(m.config.Timeouts.RecoveryThreshold, config.Default.Timeouts.RecoveryThreshold)
	
	for range ticker.C {
		// 只对不可用的端点进行健康检查
		if endpoint.Status != StatusInactive {
			continue
		}
		
		// Anthropic官方端点特例：在rate limit reset时间之前跳过健康检查
		if endpoint.ShouldSkipHealthCheckUntilReset() {
			// 只在合适的时机记录日志，避免过于频繁
			if endpoint.ShouldLogSkipHealthCheck() {
				remaining := endpoint.GetRateLimitResetTimeRemaining()
				log.Printf("DEBUG: Skipping health check for Anthropic official endpoint %s until rate limit reset (remaining: %d seconds)", 
					endpoint.Name, remaining)
			}
			continue
		}
		
		// 如果是Anthropic官方端点且曾经有rate limit信息，记录恢复健康检查的信息
		if endpoint.IsAnthropicEndpoint() {
			resetTime, _ := endpoint.GetRateLimitState()
			if resetTime != nil {
				log.Printf("DEBUG: Performing health check for Anthropic official endpoint %s (rate limit reset time has passed)", 
					endpoint.Name)
			}
		}
		
		if err := m.healthChecker.CheckEndpoint(endpoint); err != nil {
			// 健康检查失败，重置连续成功次数
			endpoint.RecordRequest(false, "health-check", 0, 0)
		} else {
			// 健康检查成功，记录成功并检查是否达到恢复阈值
			endpoint.RecordRequest(true, "health-check", 0, 0)
			if endpoint.GetSuccessiveSuccesses() >= recoveryThreshold {
				// 达到恢复阈值，恢复为可用状态
				endpoint.MarkActive()
			}
		}
	}
}

// initializeEndpointStatistics initializes or inherits statistics for an endpoint
func initializeEndpointStatistics(endpoint *Endpoint, statisticsManager statistics.StatisticsManager) error {
	// 获取主URL用于统计 (优先Anthropic URL)
	url := endpoint.URLAnthropic
	if url == "" {
		url = endpoint.URLOpenAI
	}

	// Initialize statistics in database and inherit existing data if available
	dbStats, err := statisticsManager.InitializeEndpointStatistics(
		endpoint.Name, url, endpoint.EndpointType, endpoint.AuthType)
	if err != nil {
		return err
	}

	// Update endpoint's in-memory statistics with persisted data
	endpoint.mutex.Lock()
	endpoint.TotalRequests = dbStats.TotalRequests
	endpoint.SuccessRequests = dbStats.SuccessRequests
	endpoint.FailureCount = dbStats.FailureCount
	endpoint.SuccessiveSuccesses = dbStats.SuccessiveSuccesses
	endpoint.LastFailure = dbStats.LastFailure
	endpoint.mutex.Unlock()

	return nil
}

// updateExistingEndpoint updates an existing endpoint's configuration while preserving statistics
func (m *Manager) updateExistingEndpoint(existingEndpoint *Endpoint, newConfig config.EndpointConfig) *Endpoint {
	// Create new endpoint with updated configuration but preserve statistics
	newEndpoint := NewEndpoint(newConfig)
	
	// Copy statistics from existing endpoint to preserve accumulated data
	existingEndpoint.mutex.RLock()
	newEndpoint.mutex.Lock()
	newEndpoint.TotalRequests = existingEndpoint.TotalRequests
	newEndpoint.SuccessRequests = existingEndpoint.SuccessRequests
	newEndpoint.FailureCount = existingEndpoint.FailureCount
	newEndpoint.SuccessiveSuccesses = existingEndpoint.SuccessiveSuccesses
	newEndpoint.LastFailure = existingEndpoint.LastFailure
	newEndpoint.Status = existingEndpoint.Status
	newEndpoint.LastCheck = existingEndpoint.LastCheck
	
	// Preserve request history for health checking
	newEndpoint.RequestHistory = existingEndpoint.RequestHistory
	newEndpoint.mutex.Unlock()
	existingEndpoint.mutex.RUnlock()

	// 获取主URL用于统计 (优先Anthropic URL)
	url := newEndpoint.URLAnthropic
	if url == "" {
		url = newEndpoint.URLOpenAI
	}

	// Update database metadata if statistics manager is available
	if m.statisticsManager != nil {
		if err := m.statisticsManager.UpdateEndpointMetadata(
			newEndpoint.ID, newEndpoint.Name, url,
			newEndpoint.EndpointType, newEndpoint.AuthType); err != nil {
			log.Printf("WARNING: Failed to update metadata for endpoint %s: %v",
				newEndpoint.Name, err)
		}
	}

	return newEndpoint
}

// cleanupRemovedEndpoints removes statistics for endpoints that are no longer in configuration
func (m *Manager) cleanupRemovedEndpoints(newConfigs []config.EndpointConfig) {
	// Create set of new endpoint names
	newEndpointNames := make(map[string]bool)
	for _, cfg := range newConfigs {
		newEndpointNames[cfg.Name] = true
	}

	// Check existing endpoints and remove statistics for those not in new config
	for _, endpoint := range m.endpoints {
		if !newEndpointNames[endpoint.Name] {
			// Endpoint was removed - delete its statistics
			log.Printf("Cleaning up statistics for removed endpoint: %s", endpoint.Name)
			if err := m.statisticsManager.DeleteStatistics(endpoint.ID); err != nil {
				log.Printf("WARNING: Failed to delete statistics for removed endpoint %s: %v",
					endpoint.Name, err)
			}
		}
	}
}

// setDynamicSorterForEndpoints 为所有端点设置动态排序器引用
func (m *Manager) setDynamicSorterForEndpoints() {
	// 这里需要通过adminServer来设置动态排序器引用
	// 由于循环依赖的问题，我们通过selector来传递引用
	if m.selector != nil {
		m.selector.SetDynamicSorterForEndpoints(m.endpoints)
	}
}

