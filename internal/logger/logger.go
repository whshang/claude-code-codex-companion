package logger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"claude-code-codex-companion/internal/utils"

	"github.com/sirupsen/logrus"
)

const logBodyPreviewLimit = 2048

type RequestLog struct {
	Timestamp             time.Time         `json:"timestamp"`
	RequestID             string            `json:"request_id"`
	Endpoint              string            `json:"endpoint"`
	Method                string            `json:"method"`
	Path                  string            `json:"path"`
	StatusCode            int               `json:"status_code"`
	DurationMs            int64             `json:"duration_ms"`
	AttemptNumber         int               `json:"attempt_number"` // 尝试次数（1表示第一次，2表示第一次重试，等等）
	RequestHeaders        map[string]string `json:"request_headers"`
	RequestBody           string            `json:"request_body"`
	ResponseHeaders       map[string]string `json:"response_headers"`
	ResponseBody          string            `json:"response_body"`
	RequestBodyHash       string            `json:"request_body_hash,omitempty"`
	ResponseBodyHash      string            `json:"response_body_hash,omitempty"`
	RequestBodyTruncated  bool              `json:"request_body_truncated"`
	ResponseBodyTruncated bool              `json:"response_body_truncated"`
	Error                 string            `json:"error,omitempty"`
	RequestBodySize       int               `json:"request_body_size"`
	ResponseBodySize      int               `json:"response_body_size"`
	IsStreaming           bool              `json:"is_streaming"`
	WasStreaming          bool              `json:"was_streaming"`
	ConversionPath        string            `json:"conversion_path,omitempty"`
	SupportsResponsesFlag string            `json:"supports_responses_flag,omitempty"`
	Model                 string            `json:"model,omitempty"`           // 显示的模型名（原始模型名）
	OriginalModel         string            `json:"original_model,omitempty"`  // 新增：客户端请求的原始模型名
	RewrittenModel        string            `json:"rewritten_model,omitempty"` // 新增：重写后发送给上游的模型名
	ModelRewriteApplied   bool              `json:"model_rewrite_applied"`     // 新增：是否发生了模型重写
	Tags                  []string          `json:"tags,omitempty"`
	ContentTypeOverride   string            `json:"content_type_override,omitempty"`
	SessionID             string            `json:"session_id,omitempty"`
	// Thinking mode fields
	ThinkingEnabled      bool `json:"thinking_enabled"`       // 是否启用了 thinking 模式
	ThinkingBudgetTokens int  `json:"thinking_budget_tokens"` // thinking 模式的 budget tokens
	// 修改前的原始数据
	OriginalRequestURL      string            `json:"original_request_url,omitempty"`
	OriginalRequestHeaders  map[string]string `json:"original_request_headers,omitempty"`
	OriginalRequestBody     string            `json:"original_request_body,omitempty"`
	OriginalResponseHeaders map[string]string `json:"original_response_headers,omitempty"`
	OriginalResponseBody    string            `json:"original_response_body,omitempty"`
	// 修改后的最终数据
	FinalRequestURL      string            `json:"final_request_url,omitempty"`
	FinalRequestHeaders  map[string]string `json:"final_request_headers,omitempty"`
	FinalRequestBody     string            `json:"final_request_body,omitempty"`
	FinalResponseHeaders map[string]string `json:"final_response_headers,omitempty"`
	FinalResponseBody    string            `json:"final_response_body,omitempty"`

	// 新增：导致端点失效的请求ID（如果当前请求是对被拉黑端点的请求）
	BlacklistCausingRequestIDs []string `json:"blacklist_causing_request_ids,omitempty"`

	// 新增：端点失效时间（如果适用）
	EndpointBlacklistedAt *time.Time `json:"endpoint_blacklisted_at,omitempty"`

	// 新增：客户端类型和请求格式检测
	ClientType          string  `json:"client_type,omitempty"`          // "claude-code" | "codex" | "unknown"
	RequestFormat       string  `json:"request_format,omitempty"`       // "anthropic" | "openai" | "unknown"
	TargetFormat        string  `json:"target_format,omitempty"`        // 目标端点的格式类型
	FormatConverted     bool    `json:"format_converted"`               // 是否进行了格式转换
	DetectionConfidence float64 `json:"detection_confidence,omitempty"` // 格式检测置信度 (0.0-1.0)
	DetectedBy          string  `json:"detected_by,omitempty"`          // 检测方法: "path" | "body-structure" | "default"

	// 新增：端点失效原因摘要
	EndpointBlacklistReason string `json:"endpoint_blacklist_reason,omitempty"`

	// 新增：性能监控和分析字段
	PerformanceMetrics struct {
		NetworkLatencyMs   int64 `json:"network_latency_ms,omitempty"`   // 网络延迟
		ProcessingLatencyMs int64 `json:"processing_latency_ms,omitempty"` // 处理延迟
		TotalLatencyMs     int64 `json:"total_latency_ms,omitempty"`     // 总延迟
		BandwidthUsageKB   int64 `json:"bandwidth_usage_kb,omitempty"`   // 带宽使用量
		MemoryUsageMB      int64 `json:"memory_usage_mb,omitempty"`      // 内存使用量
	} `json:"performance_metrics,omitempty"`

	// 新增：错误分类和详细信息
	ErrorCategory    string            `json:"error_category,omitempty"`    // 错误类别: "network" | "timeout" | "auth" | "validation" | "server"
	ErrorDetails     map[string]interface{} `json:"error_details,omitempty"` // 错误详细信息
	RetryAttempts    int               `json:"retry_attempts"`            // 重试次数
	LastRetryError   string            `json:"last_retry_error,omitempty"`  // 最后一次重试的错误

	// 新增：端点健康状态
	EndpointHealthStatus string `json:"endpoint_health_status,omitempty"` // 端点健康状态: "healthy" | "degraded" | "unhealthy"
	EndpointResponseTime int64  `json:"endpoint_response_time,omitempty"` // 端点响应时间

	// 新增：转换质量指标
	ConversionQuality struct {
		SuccessRate    float64 `json:"success_rate"`    // 转换成功率
		ErrorRate      float64 `json:"error_rate"`      // 转换错误率
		FormatAccuracy float64 `json:"format_accuracy"` // 格式转换准确率
	} `json:"conversion_quality,omitempty"`

	// 新增：客户端行为分析
	ClientBehavior struct {
		RequestsPerMinute float64 `json:"requests_per_minute"`
		AverageSessionDuration int64 `json:"average_session_duration"`
		PreferredModel     string  `json:"preferred_model"`
		FeatureUsage       map[string]bool `json:"feature_usage"`
	} `json:"client_behavior,omitempty"`
}

// StorageInterface 定义存储接口
type StorageInterface interface {
	SaveLog(log *RequestLog)
	GetLogs(limit, offset int, failedOnly bool) ([]*RequestLog, int, error)
	GetAllLogsByRequestID(requestID string) ([]*RequestLog, error)
	CleanupLogsByDays(days int) (int64, error)
	Close() error
	GetStats() (map[string]interface{}, error)
}

type Logger struct {
	logger  *logrus.Logger
	storage StorageInterface
	config  LogConfig
	monitor *PerformanceMonitor // 性能监控器
	startTime time.Time         // 服务启动时间
}

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	requests       int64
	errors         int64
	totalLatency   int64
	startTime      time.Time
	memoryUsage    int64
	bandwidthUsage int64
}

func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		startTime: time.Now(),
	}
}

// RecordRequest 记录请求性能数据
func (p *PerformanceMonitor) RecordRequest(latency time.Duration, bytesSent, bytesReceived int) {
	atomic.AddInt64(&p.requests, 1)
	atomic.AddInt64(&p.totalLatency, latency.Nanoseconds()/1000000)
	atomic.AddInt64(&p.bandwidthUsage, int64(bytesSent+bytesReceived))
}

// RecordError 记录错误
func (p *PerformanceMonitor) RecordError() {
	atomic.AddInt64(&p.errors, 1)
}

// GetStats 获取性能统计
func (p *PerformanceMonitor) GetStats() map[string]interface{} {
	requests := atomic.LoadInt64(&p.requests)
	errors := atomic.LoadInt64(&p.errors)
	totalLatency := atomic.LoadInt64(&p.totalLatency)
	uptime := time.Since(p.startTime).Seconds()

	stats := map[string]interface{}{
		"total_requests":    requests,
		"total_errors":      errors,
		"error_rate":        0.0,
		"avg_latency_ms":    0.0,
		"uptime_seconds":    uptime,
		"requests_per_second": 0.0,
		"bandwidth_usage_mb": float64(atomic.LoadInt64(&p.bandwidthUsage)) / (1024 * 1024),
	}

	if requests > 0 {
		stats["avg_latency_ms"] = float64(totalLatency) / float64(requests)
		stats["error_rate"] = float64(errors) / float64(requests) * 100
	}

	if uptime > 0 {
		stats["requests_per_second"] = float64(requests) / uptime
	}

	return stats
}

type LogConfig struct {
	Level           string
	LogRequestTypes string
	LogRequestBody  string
	LogResponseBody string
	LogDirectory    string
	ExcludePaths    []string
}

// NewLogger 创建新的日志记录器
func NewLogger(config LogConfig) (*Logger, error) {
	logger := logrus.New()

	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	// Use GORM storage instead of SQLite storage
	storage, err := NewGORMStorage(config.LogDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GORM log storage: %v", err)
	}

	// 初始化性能监控器
	monitor := NewPerformanceMonitor()

	return &Logger{
		logger:     logger,
		storage:    storage,
		config:     config,
		monitor:    monitor,
		startTime:  time.Now(),
	}, nil
}

func (l *Logger) LogRequest(log *RequestLog) {
	// 检查是否应该排除此路径的日志
	if l.shouldExcludePath(log.Path) {
		return
	}
	
	// 总是记录到存储，方便Web界面查看
	l.storage.SaveLog(log)

	// 根据配置决定是否输出到控制台
	shouldLog := l.shouldLogRequest(log.StatusCode)

	if shouldLog {
		fields := logrus.Fields{
			"request_id":  log.RequestID,
			"endpoint":    log.Endpoint,
			"method":      log.Method,
			"path":        log.Path,
			"status_code": log.StatusCode,
			"duration_ms": log.DurationMs,
		}

		if log.Error != "" {
			fields["error"] = log.Error
		}

		if log.Model != "" {
			fields["model"] = log.Model
		}

		if len(log.Tags) > 0 {
			fields["tags"] = log.Tags
		}


		// Note: Request and response bodies are not logged to console
		// They are available in the web admin interface

		if log.StatusCode >= 400 {
			l.logger.WithFields(fields).Error("Request failed")
		} else {
			l.logger.WithFields(fields).Info("Request completed")
		}
	}
}

// shouldExcludePath checks if a path should be excluded from logging
func (l *Logger) shouldExcludePath(path string) bool {
	if len(l.config.ExcludePaths) == 0 {
		return false
	}
	
	for _, excludePath := range l.config.ExcludePaths {
		if path == excludePath {
			return true
		}
	}
	return false
}

// shouldLogRequest determines if a request should be logged to console based on configuration
func (l *Logger) shouldLogRequest(statusCode int) bool {
	switch l.config.LogRequestTypes {
	case "failed":
		return statusCode >= 400
	case "success":
		return statusCode < 400
	case "all":
		return true
	default:
		return true
	}
}

func (l *Logger) Info(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		l.logger.WithFields(fields[0]).Info(msg)
	} else {
		l.logger.Info(msg)
	}
}

func (l *Logger) Error(msg string, err error, fields ...logrus.Fields) {
	baseFields := logrus.Fields{}
	if err != nil {
		baseFields["error"] = err.Error()
	}

	if len(fields) > 0 {
		for k, v := range fields[0] {
			baseFields[k] = v
		}
	}

	l.logger.WithFields(baseFields).Error(msg)
}

func (l *Logger) Debug(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		l.logger.WithFields(fields[0]).Debug(msg)
	} else {
		l.logger.Debug(msg)
	}
}

func (l *Logger) GetLogs(limit, offset int, failedOnly bool) ([]*RequestLog, int, error) {
	if l.storage == nil {
		return []*RequestLog{}, 0, nil
	}
	return l.storage.GetLogs(limit, offset, failedOnly)
}

func (l *Logger) GetAllLogsByRequestID(requestID string) ([]*RequestLog, error) {
	if l.storage == nil {
		return []*RequestLog{}, nil
	}
	return l.storage.GetAllLogsByRequestID(requestID)
}

func (l *Logger) CleanupLogsByDays(days int) (int64, error) {
	if l.storage == nil {
		return 0, fmt.Errorf("storage not available")
	}
	return l.storage.CleanupLogsByDays(days)
}

func (l *Logger) CreateRequestLog(requestID, endpoint, method, path string) *RequestLog {
	return &RequestLog{
		Timestamp: time.Now(),
		RequestID: requestID,
		Endpoint:  endpoint,
		Method:    method,
		Path:      path,
	}
}

func (l *Logger) UpdateRequestLog(log *RequestLog, req *http.Request, resp *http.Response, body []byte, duration time.Duration, err error) {
	log.DurationMs = duration.Nanoseconds() / 1000000

	if req != nil {
		log.RequestHeaders = utils.HeadersToMap(req.Header)
		log.IsStreaming = req.Header.Get("Accept") == "text/event-stream" ||
			req.Header.Get("Accept") == "application/json, text/event-stream"
	}

	if resp != nil {
		log.StatusCode = resp.StatusCode
		log.ResponseHeaders = utils.HeadersToMap(resp.Header)

		// 检查响应是否为流式
		if resp.Header.Get("Content-Type") != "" {
			contentType := resp.Header.Get("Content-Type")
			if strings.Contains(contentType, "text/event-stream") {
				log.IsStreaming = true
			}
		}
	}

	log.ResponseBodySize = len(body)
	if len(body) > 0 {
		sum := sha256.Sum256(body)
		preview := body
		truncated := false
		if len(preview) > logBodyPreviewLimit {
			preview = preview[:logBodyPreviewLimit]
			truncated = true
		}
		log.ResponseBodyHash = hex.EncodeToString(sum[:])
		log.ResponseBodyTruncated = truncated
		if l.config.LogResponseBody != "none" {
			log.ResponseBody = string(preview)
		}
	}
	log.WasStreaming = log.IsStreaming

	if err != nil {
		log.Error = err.Error()
	}
}

// Close closes the logger and its storage backend
func (l *Logger) Close() error {
	if l.storage != nil {
		return l.storage.Close()
	}
	return nil
}

// GetDatabaseHealth 获取数据库健康状态
func (l *Logger) GetDatabaseHealth() map[string]interface{} {
	if gormStorage, ok := l.storage.(*GORMStorage); ok {
		return gormStorage.GetDatabaseHealth()
	}
	return map[string]interface{}{
		"status":  "unknown",
		"message": "Not using GORM storage",
	}
}

// GetStorage 获取底层存储实现
func (l *Logger) GetStorage() interface{} {
	return l.storage
}

// GetStats 获取统计信息
func (l *Logger) GetStats() (map[string]interface{}, error) {
	if l.storage == nil {
		return nil, fmt.Errorf("storage not available")
	}
	return l.storage.GetStats()
}

// UpdateConfig 更新日志配置（用于热更新）
func (l *Logger) UpdateConfig(newConfig LogConfig) {
	// 更新日志级别
	level, err := logrus.ParseLevel(newConfig.Level)
	if err == nil {
		l.logger.SetLevel(level)
	}
	
	// 更新配置
	l.config = newConfig
}
