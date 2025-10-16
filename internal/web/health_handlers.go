package web

import (
	"net/http"
	"runtime"
	"time"

	"claude-code-codex-companion/internal/logger"
	"github.com/gin-gonic/gin"
)

// handleHealthCheck 系统健康检查端点
func (s *AdminServer) handleHealthCheck(c *gin.Context) {
	health := make(map[string]interface{})
	
	// 基本系统信息
	health["status"] = "healthy"
	health["timestamp"] = time.Now().Unix()
	health["uptime"] = time.Since(s.startTime).String()
	health["version"] = s.version
	
	// 系统资源信息
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	health["memory"] = map[string]interface{}{
		"alloc_bytes":      m.Alloc,
		"total_alloc_bytes": m.TotalAlloc,
		"sys_bytes":        m.Sys,
		"num_gc":           m.NumGC,
		"goroutines":       runtime.NumGoroutine(),
	}
	
	// 数据库健康状态
	health["database"] = s.logger.GetDatabaseHealth()
	
	// 端点状态
	endpoints := s.endpointManager.GetAllEndpoints()
	activeEndpoints := 0
	healthyEndpoints := 0
	
	for _, ep := range endpoints {
		if ep.IsEnabled() {
			activeEndpoints++
			if ep.IsAvailable() {
				healthyEndpoints++
			}
		}
	}
	
	health["endpoints"] = map[string]interface{}{
		"total":   len(endpoints),
		"active":  activeEndpoints,
		"healthy": healthyEndpoints,
	}
	
	// 检查是否有严重错误
	if dbStatus, ok := health["database"].(map[string]interface{}); ok {
		if status, exists := dbStatus["status"]; exists && status != "healthy" {
			health["status"] = "degraded"
		}
	}
	
	if healthyEndpoints < activeEndpoints {
		health["status"] = "degraded"
	}
	
	// 返回相应的HTTP状态码
	statusCode := http.StatusOK
	if health["status"] == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, health)
}

// handleDiagnostics 详细的系统诊断信息
func (s *AdminServer) handleDiagnostics(c *gin.Context) {
	diagnostics := make(map[string]interface{})
	
	// 系统信息
	diagnostics["system"] = map[string]interface{}{
		"go_version":   runtime.Version(),
		"num_cpu":      runtime.NumCPU(),
		"num_goroutine": runtime.NumGoroutine(),
		"timestamp":    time.Now().Unix(),
	}
	
	// 内存详情
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	diagnostics["memory"] = map[string]interface{}{
		"alloc":         m.Alloc,
		"total_alloc":   m.TotalAlloc,
		"sys":           m.Sys,
		"lookups":       m.Lookups,
		"mallocs":       m.Mallocs,
		"frees":         m.Frees,
		"heap_alloc":    m.HeapAlloc,
		"heap_sys":      m.HeapSys,
		"heap_idle":     m.HeapIdle,
		"heap_inuse":    m.HeapInuse,
		"heap_released": m.HeapReleased,
		"heap_objects":  m.HeapObjects,
		"stack_inuse":   m.StackInuse,
		"stack_sys":     m.StackSys,
		"gc_cpu_fraction": m.GCCPUFraction,
		"num_gc":        m.NumGC,
		"num_forced_gc": m.NumForcedGC,
		"gc_pause_fraction": float64(m.PauseTotalNs) / float64(m.NumGC*1000000000),
	}
	
	// 端点详细信息
	endpoints := s.endpointManager.GetAllEndpoints()
	endpointDetails := make([]map[string]interface{}, 0, len(endpoints))
	
	for _, ep := range endpoints {
		detail := map[string]interface{}{
			"name":       ep.Name,
			"enabled":    ep.IsEnabled(),
			"healthy":    ep.IsAvailable(),
			"last_check": "N/A",
			"fail_count": 0,
			"type":       ep.EndpointType,
		}
		endpointDetails = append(endpointDetails, detail)
	}
	diagnostics["endpoints"] = endpointDetails
	
	// 数据库诊断
	storage := s.logger.GetStorage()
	if storage != nil {
		if gormStorage, ok := storage.(*logger.GORMStorage); ok {
			if stats, err := gormStorage.GetStats(); err != nil {
				diagnostics["database_error"] = err.Error()
			} else {
				diagnostics["database"] = stats
			}
		} else {
			diagnostics["database_error"] = "Unknown storage type"
		}
	} else {
		diagnostics["database_error"] = "Storage not available"
	}
	
	// 配置信息（不包含敏感数据）
	diagnostics["config"] = map[string]interface{}{
		"server_port":     s.config.Server.Port,
		"server_host":     s.config.Server.Host,
		"web_admin_enabled": true, // Web admin is now integrated into main server
		"logging_level":   s.config.Logging.Level,
		"endpoints_count": 0, // Deprecated: Dynamic endpoints removed.
	}
	
	c.JSON(http.StatusOK, diagnostics)
}

// startTime 应该在 AdminServer 初始化时设置
func (s *AdminServer) setStartTime() {
	s.startTime = time.Now()
}