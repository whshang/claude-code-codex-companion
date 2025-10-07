package logger

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"claude-code-codex-companion/internal/utils"

	"github.com/sirupsen/logrus"
)

type RequestLog struct {
	Timestamp            time.Time         `json:"timestamp"`
	RequestID            string            `json:"request_id"`
	Endpoint             string            `json:"endpoint"`
	Method               string            `json:"method"`
	Path                 string            `json:"path"`
	StatusCode           int               `json:"status_code"`
	DurationMs           int64             `json:"duration_ms"`
	AttemptNumber        int               `json:"attempt_number"`        // 尝试次数（1表示第一次，2表示第一次重试，等等）
	RequestHeaders       map[string]string `json:"request_headers"`
	RequestBody          string            `json:"request_body"`
	ResponseHeaders      map[string]string `json:"response_headers"`
	ResponseBody         string            `json:"response_body"`
	Error                string            `json:"error,omitempty"`
	RequestBodySize      int               `json:"request_body_size"`
	ResponseBodySize     int               `json:"response_body_size"`
	IsStreaming          bool              `json:"is_streaming"`
	Model                string            `json:"model,omitempty"`                // 显示的模型名（原始模型名）
	OriginalModel        string            `json:"original_model,omitempty"`       // 新增：客户端请求的原始模型名
	RewrittenModel       string            `json:"rewritten_model,omitempty"`      // 新增：重写后发送给上游的模型名
	ModelRewriteApplied  bool              `json:"model_rewrite_applied"`          // 新增：是否发生了模型重写
	Tags                 []string          `json:"tags,omitempty"`
	ContentTypeOverride  string            `json:"content_type_override,omitempty"`
	SessionID            string            `json:"session_id,omitempty"`
	// Thinking mode fields
	ThinkingEnabled      bool              `json:"thinking_enabled"`               // 是否启用了 thinking 模式
	ThinkingBudgetTokens int               `json:"thinking_budget_tokens"`         // thinking 模式的 budget tokens
	// 修改前的原始数据
	OriginalRequestURL      string            `json:"original_request_url,omitempty"`
	OriginalRequestHeaders  map[string]string `json:"original_request_headers,omitempty"`
	OriginalRequestBody     string            `json:"original_request_body,omitempty"`
	OriginalResponseHeaders map[string]string `json:"original_response_headers,omitempty"`
	OriginalResponseBody    string            `json:"original_response_body,omitempty"`
	// 修改后的最终数据
	FinalRequestURL         string            `json:"final_request_url,omitempty"`
	FinalRequestHeaders     map[string]string `json:"final_request_headers,omitempty"`
	FinalRequestBody        string            `json:"final_request_body,omitempty"`
	FinalResponseHeaders    map[string]string `json:"final_response_headers,omitempty"`
	FinalResponseBody       string            `json:"final_response_body,omitempty"`
	
	// 新增：导致端点失效的请求ID（如果当前请求是对被拉黑端点的请求）
	BlacklistCausingRequestIDs []string `json:"blacklist_causing_request_ids,omitempty"`
	
	// 新增：端点失效时间（如果适用）
	EndpointBlacklistedAt *time.Time `json:"endpoint_blacklisted_at,omitempty"`
	
	// 新增：端点失效原因摘要
	EndpointBlacklistReason string `json:"endpoint_blacklist_reason,omitempty"`

	// 新增：客户端类型和请求格式检测
	ClientType         string  `json:"client_type,omitempty"`          // "claude-code" | "codex" | "unknown"
	RequestFormat      string  `json:"request_format,omitempty"`       // "anthropic" | "openai" | "unknown"
	TargetFormat       string  `json:"target_format,omitempty"`        // 目标端点的格式类型
	FormatConverted    bool    `json:"format_converted"`               // 是否进行了格式转换
	DetectionConfidence float64 `json:"detection_confidence,omitempty"` // 格式检测置信度 (0.0-1.0)
	DetectedBy         string  `json:"detected_by,omitempty"`          // 检测方法: "path" | "body-structure" | "default"
}

// LoggerInterface 日志接口
type LoggerInterface interface {
	// 基本日志功能
	LogRequest(req *RequestLog)
	GetLogs(limit, offset int, failedOnly bool) ([]*RequestLog, int, error)
	GetAllLogsByRequestID(requestID string) ([]*RequestLog, error)
	CleanupLogsByDays(days int) (int64, error)
	Close() error
	
	// 统计功能
	GetStats() (map[string]interface{}, error)
	
	// 健康检查功能
	GetDatabaseHealth() map[string]interface{}
	GetStorage() interface{} // 返回底层存储实现
}

type Logger struct {
	logger  *logrus.Logger
	storage StorageInterface
	config  LogConfig
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

type LogConfig struct {
	Level           string
	LogRequestTypes string
	LogRequestBody  string
	LogResponseBody string
	LogDirectory    string
}

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

	return &Logger{
		logger:  logger,
		storage: storage,
		config:  config,
	}, nil
}

func (l *Logger) LogRequest(log *RequestLog) {
	// 总是记录到存储，方便Web界面查看
	l.storage.SaveLog(log)

	// 根据配置决定是否输出到控制台
	shouldLog := l.shouldLogRequest(log.StatusCode)

	if shouldLog {
		fields := logrus.Fields{
			"request_id":   log.RequestID,
			"endpoint":     log.Endpoint,
			"method":       log.Method,
			"path":         log.Path,
			"status_code":  log.StatusCode,
			"duration_ms":  log.DurationMs,
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
	if l.config.LogResponseBody != "none" && len(body) > 0 {
		if l.config.LogResponseBody == "truncated" {
			log.ResponseBody = utils.TruncateBody(string(body), 1024)
		} else {
			log.ResponseBody = string(body)
		}
	}
	
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
		"status": "unknown",
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

