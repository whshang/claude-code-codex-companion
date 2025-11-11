package statistics

import (
	"fmt"
	"sync"
	"time"
)

// MemoryManager is a fallback statistics manager that stores data in memory only
// This is used when SQLite/CGO is not available
type MemoryManager struct {
	statistics map[string]*EndpointStatistics
	mutex      sync.RWMutex
}

// NewMemoryManager creates a new memory-only statistics manager
func NewMemoryManager() *MemoryManager {
	return &MemoryManager{
		statistics: make(map[string]*EndpointStatistics),
	}
}

// LoadStatistics loads statistics for a specific endpoint ID (memory only)
func (m *MemoryManager) LoadStatistics(endpointID string) (*EndpointStatistics, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if stats, exists := m.statistics[endpointID]; exists {
		// Return a copy to avoid concurrent modification
		statsCopy := *stats
		return &statsCopy, nil
	}
	return nil, nil // Not found
}

// LoadStatisticsByName loads statistics for an endpoint by name (memory only)
func (m *MemoryManager) LoadStatisticsByName(endpointName string) (*EndpointStatistics, error) {
	endpointID := GenerateEndpointID(endpointName)
	return m.LoadStatistics(endpointID)
}

// SaveStatistics saves statistics to memory
func (m *MemoryManager) SaveStatistics(stats *EndpointStatistics) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	stats.LastUpdated = time.Now().UTC()
	// Store a copy to avoid external modifications
	statsCopy := *stats
	m.statistics[stats.ID] = &statsCopy
	
	return nil
}

// RecordRequest records a request result and updates statistics (memory only)
func (m *MemoryManager) RecordRequest(endpointID string, success bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	stats, exists := m.statistics[endpointID]
	if !exists {
		return fmt.Errorf("endpoint statistics not found for ID %s", endpointID)
	}

	// Update statistics based on request result
	stats.TotalRequests++
	stats.LastUpdated = time.Now().UTC()

	if success {
		stats.SuccessRequests++
		stats.FailureCount = 0 // Reset consecutive failures
		stats.SuccessiveSuccesses++
	} else {
		stats.FailureCount++
		stats.SuccessiveSuccesses = 0 // Reset consecutive successes
		stats.LastFailure = time.Now().UTC()
	}

	return nil
}

// InitializeEndpointStatistics creates new statistics record in memory
func (m *MemoryManager) InitializeEndpointStatistics(endpointName, url, endpointType, authType string) (*EndpointStatistics, error) {
	endpointID := GenerateEndpointID(endpointName)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if statistics already exist
	if existing, exists := m.statistics[endpointID]; exists {
		// Update metadata in existing record
		existing.URL = url
		existing.EndpointType = endpointType
		existing.AuthType = authType
		existing.LastUpdated = time.Now().UTC()
		return existing, nil
	}

	// Create new statistics record
	now := time.Now().UTC()
	stats := &EndpointStatistics{
		ID:                  endpointID,
		Name:                endpointName,
		URL:                 url,
		EndpointType:        endpointType,
		AuthType:            authType,
		TotalRequests:       0,
		SuccessRequests:     0,
		FailureCount:        0,
		SuccessiveSuccesses: 0,
		LastUpdated:         now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	m.statistics[endpointID] = stats
	return stats, nil
}

// UpdateEndpointMetadata updates the metadata fields of an endpoint's statistics (memory only)
func (m *MemoryManager) UpdateEndpointMetadata(endpointID, name, url, endpointType, authType string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	stats, exists := m.statistics[endpointID]
	if !exists {
		return fmt.Errorf("endpoint statistics not found for ID %s", endpointID)
	}

	stats.Name = name
	stats.URL = url
	stats.EndpointType = endpointType
	stats.AuthType = authType
	stats.LastUpdated = time.Now().UTC()
	
	return nil
}

// DeleteStatistics removes statistics record from memory
func (m *MemoryManager) DeleteStatistics(endpointID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	delete(m.statistics, endpointID)
	return nil
}

// GetAllStatistics returns all endpoint statistics from memory
func (m *MemoryManager) GetAllStatistics() ([]*EndpointStatistics, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	allStats := make([]*EndpointStatistics, 0, len(m.statistics))
	for _, stats := range m.statistics {
		// Return copies to avoid concurrent modification
		statsCopy := *stats
		allStats = append(allStats, &statsCopy)
	}
	
	return allStats, nil
}

// GetStatisticsSummary returns a summary of all statistics from memory
func (m *MemoryManager) GetStatisticsSummary() (map[string]interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	totalEndpoints := len(m.statistics)
	totalRequests := 0
	totalSuccesses := 0

	for _, stats := range m.statistics {
		totalRequests += stats.TotalRequests
		totalSuccesses += stats.SuccessRequests
	}

	summary := map[string]interface{}{
		"total_endpoints": totalEndpoints,
		"total_requests":  totalRequests,
		"total_successes": totalSuccesses,
		"overall_success_rate": func() float64 {
			if totalRequests == 0 {
				return 0.0
			}
			return float64(totalSuccesses) / float64(totalRequests) * 100.0
		}(),
	}

	return summary, nil
}

// Close is a no-op for memory manager
func (m *MemoryManager) Close() error {
	return nil
}

// GetDatabasePath returns a message indicating this is memory-only
func (m *MemoryManager) GetDatabasePath() string {
	return "memory-only (no persistent storage)"
}