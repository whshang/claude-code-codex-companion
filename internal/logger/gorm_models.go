package logger

import (
	"encoding/json"
	"time"
)

// GormRequestLog - 完全对应现有 request_logs 表结构的GORM模型
type GormRequestLog struct {
	// 主键和基础字段
	ID            uint      `gorm:"primaryKey;column:id;autoIncrement"`
	Timestamp     time.Time `gorm:"column:timestamp;index:idx_timestamp;not null"`
	RequestID     string    `gorm:"column:request_id;index:idx_request_id;size:100;not null"`
	Endpoint      string    `gorm:"column:endpoint;index:idx_endpoint;size:200;not null"`
	Method        string    `gorm:"column:method;size:10;not null"`
	Path          string    `gorm:"column:path;size:500;not null"`
	StatusCode    int       `gorm:"column:status_code;index:idx_status_code;default:0"`
	DurationMs    int64     `gorm:"column:duration_ms;default:0"`
	AttemptNumber int       `gorm:"column:attempt_number;default:1"`

	// 请求数据字段
	RequestHeaders  string `gorm:"column:request_headers;type:text;default:'{}'"`
	RequestBody     string `gorm:"column:request_body;type:text;default:''"`
	RequestBodySize int    `gorm:"column:request_body_size;default:0"`

	// 响应数据字段
	ResponseHeaders  string `gorm:"column:response_headers;type:text;default:'{}'"`
	ResponseBody     string `gorm:"column:response_body;type:text;default:''"`
	ResponseBodySize int    `gorm:"column:response_body_size;default:0"`
	IsStreaming      bool   `gorm:"column:is_streaming;default:false"`
	WasStreaming     bool   `gorm:"column:was_streaming;default:false"`

	// 模型和标签字段
	Model               string `gorm:"column:model;size:100;default:''"`
	Error               string `gorm:"column:error;type:text;default:''"`
	Tags                string `gorm:"column:tags;type:text;default:'[]'"` // JSON array
	ContentTypeOverride string `gorm:"column:content_type_override;size:100;default:''"`
	SessionID           string `gorm:"column:session_id;size:100;default:''"`

	// 模型重写字段
	OriginalModel       string `gorm:"column:original_model;size:100;default:''"`
	RewrittenModel      string `gorm:"column:rewritten_model;size:100;default:''"`
	ModelRewriteApplied bool   `gorm:"column:model_rewrite_applied;default:false"`

	// Thinking 模式字段
	ThinkingEnabled      bool `gorm:"column:thinking_enabled;default:false"`
	ThinkingBudgetTokens int  `gorm:"column:thinking_budget_tokens;default:0"`

	// 原始请求/响应字段
	OriginalRequestURL      string `gorm:"column:original_request_url;size:500;default:''"`
	OriginalRequestHeaders  string `gorm:"column:original_request_headers;type:text;default:'{}'"`
	OriginalRequestBody     string `gorm:"column:original_request_body;type:text;default:''"`
	OriginalResponseHeaders string `gorm:"column:original_response_headers;type:text;default:'{}'"`
	OriginalResponseBody    string `gorm:"column:original_response_body;type:text;default:''"`

	// 最终请求/响应字段
	FinalRequestURL       string `gorm:"column:final_request_url;size:500;default:''"`
	FinalRequestHeaders   string `gorm:"column:final_request_headers;type:text;default:'{}'"`
	FinalRequestBody      string `gorm:"column:final_request_body;type:text;default:''"`
	FinalResponseHeaders  string `gorm:"column:final_response_headers;type:text;default:'{}'"`
	FinalResponseBody     string `gorm:"column:final_response_body;type:text;default:''"`
	RequestBodyHash       string `gorm:"column:request_body_hash;size:128;default:''"`
	ResponseBodyHash      string `gorm:"column:response_body_hash;size:128;default:''"`
	RequestBodyTruncated  bool   `gorm:"column:request_body_truncated;default:false"`
	ResponseBodyTruncated bool   `gorm:"column:response_body_truncated;default:false"`
	ConversionPath        string `gorm:"column:conversion_path;size:100;default:''"`
	SupportsResponsesFlag string `gorm:"column:supports_responses_flag;size:20;default:''"`

	// 新增：被拉黑端点相关字段
	BlacklistCausingRequestIDs string     `gorm:"column:blacklist_causing_request_ids;type:text;default:'[]'"`
	EndpointBlacklistedAt      *time.Time `gorm:"column:endpoint_blacklisted_at"`
	EndpointBlacklistReason    string     `gorm:"column:endpoint_blacklist_reason;type:text;default:''"`

	// 新增：客户端类型和请求格式检测字段
	ClientType          string  `gorm:"column:client_type;size:50;index:idx_client_type;default:''"`
	RequestFormat       string  `gorm:"column:request_format;size:50;index:idx_request_format;default:''"`
	TargetFormat        string  `gorm:"column:target_format;size:50;default:''"`
	FormatConverted     bool    `gorm:"column:format_converted;index:idx_format_converted;default:false"`
	DetectionConfidence float64 `gorm:"column:detection_confidence;default:0"`
	DetectedBy          string  `gorm:"column:detected_by;size:50;default:''"`

	// 创建时间（现有字段）
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

// 指定表名，与现有数据库表完全一致
func (GormRequestLog) TableName() string {
	return "request_logs"
}

// 转换方法：从现有RequestLog到GormRequestLog
func ConvertToGormRequestLog(log *RequestLog) *GormRequestLog {
	gormLog := &GormRequestLog{
		Timestamp:                  log.Timestamp,
		RequestID:                  log.RequestID,
		Endpoint:                   log.Endpoint,
		Method:                     log.Method,
		Path:                       log.Path,
		StatusCode:                 log.StatusCode,
		DurationMs:                 log.DurationMs,
		AttemptNumber:              log.AttemptNumber,
		RequestBody:                log.RequestBody,
		RequestBodySize:            log.RequestBodySize,
		ResponseBody:               log.ResponseBody,
		ResponseBodySize:           log.ResponseBodySize,
		IsStreaming:                log.IsStreaming,
		WasStreaming:               log.WasStreaming,
		Model:                      log.Model,
		Error:                      log.Error,
		ContentTypeOverride:        log.ContentTypeOverride,
		SessionID:                  log.SessionID,
		RequestBodyHash:            log.RequestBodyHash,
		ResponseBodyHash:           log.ResponseBodyHash,
		RequestBodyTruncated:       log.RequestBodyTruncated,
		ResponseBodyTruncated:      log.ResponseBodyTruncated,
		ConversionPath:             log.ConversionPath,
		SupportsResponsesFlag:      log.SupportsResponsesFlag,
		OriginalModel:              log.OriginalModel,
		RewrittenModel:             log.RewrittenModel,
		ModelRewriteApplied:        log.ModelRewriteApplied,
		ThinkingEnabled:            log.ThinkingEnabled,
		ThinkingBudgetTokens:       log.ThinkingBudgetTokens,
		OriginalRequestURL:         log.OriginalRequestURL,
		OriginalRequestBody:        log.OriginalRequestBody,
		OriginalResponseBody:       log.OriginalResponseBody,
		FinalRequestURL:            log.FinalRequestURL,
		FinalRequestBody:           log.FinalRequestBody,
		FinalResponseBody:          log.FinalResponseBody,
		BlacklistCausingRequestIDs: marshalTagsToJSON(log.BlacklistCausingRequestIDs),
		EndpointBlacklistedAt:      log.EndpointBlacklistedAt,
		EndpointBlacklistReason:    log.EndpointBlacklistReason,
		// 新增：客户端类型和请求格式检测字段
		ClientType:          log.ClientType,
		RequestFormat:       log.RequestFormat,
		TargetFormat:        log.TargetFormat,
		FormatConverted:     log.FormatConverted,
		DetectionConfidence: log.DetectionConfidence,
		DetectedBy:          log.DetectedBy,
	}

	// 转换JSON字段
	gormLog.RequestHeaders = marshalToJSON(log.RequestHeaders)
	gormLog.ResponseHeaders = marshalToJSON(log.ResponseHeaders)
	gormLog.Tags = marshalTagsToJSON(log.Tags)
	gormLog.OriginalRequestHeaders = marshalToJSON(log.OriginalRequestHeaders)
	gormLog.OriginalResponseHeaders = marshalToJSON(log.OriginalResponseHeaders)
	gormLog.FinalRequestHeaders = marshalToJSON(log.FinalRequestHeaders)
	gormLog.FinalResponseHeaders = marshalToJSON(log.FinalResponseHeaders)

	return gormLog
}

// 转换方法：从GormRequestLog到现有RequestLog
func ConvertFromGormRequestLog(gormLog *GormRequestLog) *RequestLog {
	log := &RequestLog{
		Timestamp:                  gormLog.Timestamp,
		RequestID:                  gormLog.RequestID,
		Endpoint:                   gormLog.Endpoint,
		Method:                     gormLog.Method,
		Path:                       gormLog.Path,
		StatusCode:                 gormLog.StatusCode,
		DurationMs:                 gormLog.DurationMs,
		AttemptNumber:              gormLog.AttemptNumber,
		RequestBody:                gormLog.RequestBody,
		RequestBodySize:            gormLog.RequestBodySize,
		ResponseBody:               gormLog.ResponseBody,
		ResponseBodySize:           gormLog.ResponseBodySize,
		IsStreaming:                gormLog.IsStreaming,
		WasStreaming:               gormLog.WasStreaming,
		RequestBodyHash:            gormLog.RequestBodyHash,
		ResponseBodyHash:           gormLog.ResponseBodyHash,
		RequestBodyTruncated:       gormLog.RequestBodyTruncated,
		ResponseBodyTruncated:      gormLog.ResponseBodyTruncated,
		ConversionPath:             gormLog.ConversionPath,
		SupportsResponsesFlag:      gormLog.SupportsResponsesFlag,
		Model:                      gormLog.Model,
		Error:                      gormLog.Error,
		ContentTypeOverride:        gormLog.ContentTypeOverride,
		SessionID:                  gormLog.SessionID,
		OriginalModel:              gormLog.OriginalModel,
		RewrittenModel:             gormLog.RewrittenModel,
		ModelRewriteApplied:        gormLog.ModelRewriteApplied,
		ThinkingEnabled:            gormLog.ThinkingEnabled,
		ThinkingBudgetTokens:       gormLog.ThinkingBudgetTokens,
		OriginalRequestURL:         gormLog.OriginalRequestURL,
		OriginalRequestBody:        gormLog.OriginalRequestBody,
		OriginalResponseBody:       gormLog.OriginalResponseBody,
		FinalRequestURL:            gormLog.FinalRequestURL,
		FinalRequestBody:           gormLog.FinalRequestBody,
		FinalResponseBody:          gormLog.FinalResponseBody,
		BlacklistCausingRequestIDs: unmarshalTagsFromJSON(gormLog.BlacklistCausingRequestIDs),
		EndpointBlacklistedAt:      gormLog.EndpointBlacklistedAt,
		EndpointBlacklistReason:    gormLog.EndpointBlacklistReason,
		// 新增：客户端类型和请求格式检测字段
		ClientType:          gormLog.ClientType,
		RequestFormat:       gormLog.RequestFormat,
		TargetFormat:        gormLog.TargetFormat,
		FormatConverted:     gormLog.FormatConverted,
		DetectionConfidence: gormLog.DetectionConfidence,
		DetectedBy:          gormLog.DetectedBy,
	}

	// 转换JSON字段
	log.RequestHeaders = unmarshalFromJSON(gormLog.RequestHeaders)
	log.ResponseHeaders = unmarshalFromJSON(gormLog.ResponseHeaders)
	log.Tags = unmarshalTagsFromJSON(gormLog.Tags)
	log.OriginalRequestHeaders = unmarshalFromJSON(gormLog.OriginalRequestHeaders)
	log.OriginalResponseHeaders = unmarshalFromJSON(gormLog.OriginalResponseHeaders)
	log.FinalRequestHeaders = unmarshalFromJSON(gormLog.FinalRequestHeaders)
	log.FinalResponseHeaders = unmarshalFromJSON(gormLog.FinalResponseHeaders)

	return log
}

// JSON序列化辅助函数
func marshalToJSON(headers map[string]string) string {
	if headers == nil {
		return "{}"
	}
	data, err := json.Marshal(headers)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func marshalTagsToJSON(tags []string) string {
	if tags == nil {
		return "[]"
	}
	data, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func unmarshalFromJSON(jsonStr string) map[string]string {
	var headers map[string]string
	if jsonStr == "" || jsonStr == "{}" {
		return make(map[string]string)
	}
	if err := json.Unmarshal([]byte(jsonStr), &headers); err != nil {
		return make(map[string]string)
	}
	return headers
}

func unmarshalTagsFromJSON(jsonStr string) []string {
	var tags []string
	if jsonStr == "" || jsonStr == "[]" || jsonStr == "null" {
		return []string{}
	}
	if err := json.Unmarshal([]byte(jsonStr), &tags); err != nil {
		return []string{}
	}
	return tags
}
