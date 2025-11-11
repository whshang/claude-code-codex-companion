package config

// EndpointConfig 端点配置（完整版，支持所有功能）
type EndpointConfig struct {
	Name               string              `yaml:"name" json:"name"`
	URLAnthropic       string              `yaml:"url_anthropic,omitempty" json:"url_anthropic,omitempty"` // Anthropic格式URL
	URLOpenAI          string              `yaml:"url_openai,omitempty" json:"url_openai,omitempty"`       // OpenAI格式URL
	URLGemini          string              `yaml:"url_gemini,omitempty" json:"url_gemini,omitempty"`       // Gemini格式URL
	AuthType           string              `yaml:"auth_type" json:"auth_type"`
	AuthValue          string              `yaml:"auth_value" json:"auth_value"`
	Enabled            bool                `yaml:"enabled" json:"enabled"`
	Priority           int                 `yaml:"priority" json:"priority"`
	Tags               []string            `yaml:"tags" json:"tags"`                                                       // 支持的tag列表
	ModelRewrite       *ModelRewriteConfig `yaml:"model_rewrite,omitempty" json:"model_rewrite,omitempty"`                 // 模型重写配置
	Proxy              *ProxyConfig        `yaml:"proxy,omitempty" json:"proxy,omitempty"`                                 // 代理配置
	OAuthConfig        *OAuthConfig        `yaml:"oauth_config,omitempty" json:"oauth_config,omitempty"`                   // OAuth配置
	HeaderOverrides    map[string]string   `yaml:"header_overrides,omitempty" json:"header_overrides,omitempty"`           // HTTP Header覆盖配置
	ParameterOverrides map[string]string   `yaml:"parameter_overrides,omitempty" json:"parameter_overrides,omitempty"`     // Request Parameters覆盖配置
	MaxTokensFieldName string              `yaml:"max_tokens_field_name,omitempty" json:"max_tokens_field_name,omitempty"` // max_tokens 参数名转换选项
	RateLimitReset     *int64              `yaml:"rate_limit_reset,omitempty" json:"rate_limit_reset,omitempty"`           // Anthropic-Ratelimit-Unified-Reset
	RateLimitStatus    *string             `yaml:"rate_limit_status,omitempty" json:"rate_limit_status,omitempty"`         // Anthropic-Ratelimit-Unified-Status
	EnhancedProtection bool                `yaml:"enhanced_protection,omitempty" json:"enhanced_protection,omitempty"`     // 官方帐号增强保护：allowed_warning时即禁用端点
	SSEConfig          *SSEConfig          `yaml:"sse_config,omitempty" json:"sse_config,omitempty"`                       // SSE行为配置
	OpenAIPreference   string              `yaml:"openai_preference,omitempty" json:"openai_preference,omitempty"`         // OpenAI格式偏好："responses"|"chat_completions"|"auto"
	CountTokensEnabled *bool               `yaml:"count_tokens_enabled,omitempty" json:"count_tokens_enabled,omitempty"`   // 是否允许使用 /count_tokens 接口
	SupportsResponses  *bool               `yaml:"supports_responses,omitempty" json:"supports_responses,omitempty"`       // 显式声明是否原生支持 /responses 接口

	// 新增：智能转换标记（方案A核心字段）
	NativeFormat bool   `yaml:"native_format,omitempty" json:"native_format,omitempty"` // 是否原生支持客户端格式（true=无需转换）
	TargetFormat string `yaml:"target_format,omitempty" json:"target_format,omitempty"` // 转换目标格式："anthropic"|"openai_chat"|"openai_responses"|"gemini"
	ClientType   string `yaml:"client_type,omitempty" json:"client_type,omitempty"`     // 客户端类型过滤："claude_code"|"codex"|"openai"|"gemini"|""（空表示通用）
}

type Config struct {
	Server          ServerConfig          `yaml:"server"`
	Endpoints       []EndpointConfig      `yaml:"endpoints"` // 恢复：端点列表（方案A核心）
	Logging         LoggingConfig         `yaml:"logging"`
	Validation      ValidationConfig      `yaml:"validation"`
	Tagging         TaggingConfig         `yaml:"tagging"`            // 标签系统配置（永远启用）
	Timeouts        TimeoutConfig         `yaml:"timeouts"`           // 超时配置
	Blacklist       BlacklistConfig       `yaml:"blacklist"`          // 端点拉黑配置
	Conversion      ConversionConfig      `yaml:"conversion"`         // 格式转换配置
	Streaming       StreamingConfig       `yaml:"streaming"`          // 流式转换配置
	Tools           ToolsConfig           `yaml:"tools"`              // 工具调用配置
	HTTPClient      HTTPClientConfig      `yaml:"http_client"`        // HTTP客户端配置
	Monitoring      MonitoringConfig      `yaml:"monitoring"`         // 性能监控配置
	FormatDetection FormatDetectionConfig `yaml:"format_detection"`   // 格式检测配置
	Retry           RetryConfig           `yaml:"retry" json:"retry"` // 重试策略配置
}

type ServerConfig struct {
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	AutoSortEndpoints bool   `yaml:"auto_sort_endpoints" json:"auto_sort_endpoints"` // 是否自动调整端点排序

	// ✅ 新增：配置持久化设置
	ConfigFlushInterval string `yaml:"config_flush_interval,omitempty" json:"config_flush_interval,omitempty"` // 配置写入间隔（默认30s）
	ConfigMaxDirtyTime  string `yaml:"config_max_dirty_time,omitempty" json:"config_max_dirty_time,omitempty"` // 最大脏数据保留时间（默认5m）
}

// 新增：SSE行为配置结构
type SSEConfig struct {
	RequireDoneMarker bool `yaml:"require_done_marker" json:"require_done_marker"` // 是否要求[DONE]标记
}

// 新增：代理配置结构
type ProxyConfig struct {
	Type     string `yaml:"type" json:"type"`                             // "http" | "socks5"
	Address  string `yaml:"address" json:"address"`                       // 代理服务器地址，如 "127.0.0.1:1080"
	Username string `yaml:"username,omitempty" json:"username,omitempty"` // 代理认证用户名（可选）
	Password string `yaml:"password,omitempty" json:"password,omitempty"` // 代理认证密码（可选）
}

// 新增：OAuth 配置结构
type OAuthConfig struct {
	AccessToken  string   `yaml:"access_token" json:"access_token"`               // 访问令牌
	RefreshToken string   `yaml:"refresh_token" json:"refresh_token"`             // 刷新令牌
	ExpiresAt    int64    `yaml:"expires_at" json:"expires_at"`                   // 过期时间戳（毫秒）
	TokenURL     string   `yaml:"token_url" json:"token_url"`                     // Token刷新URL（必填）
	ClientID     string   `yaml:"client_id,omitempty" json:"client_id,omitempty"` // 客户端ID
	Scopes       []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`       // 权限范围
	AutoRefresh  bool     `yaml:"auto_refresh" json:"auto_refresh"`               // 是否自动刷新
}

// 新增：模型重写配置结构
type ModelRewriteConfig struct {
	Enabled     bool               `yaml:"enabled" json:"enabled"`                               // 是否启用模型重写
	Rules       []ModelRewriteRule `yaml:"rules" json:"rules"`                                   // 重写规则列表
	TargetModel string             `yaml:"target_model,omitempty" json:"target_model,omitempty"` // 健康检查测试模型（对应数据库 target_model 字段）
}

// 新增：模型重写规则
type ModelRewriteRule struct {
	SourcePattern string `yaml:"source_pattern" json:"source_pattern"` // 源模型通配符模式
	TargetModel   string `yaml:"target_model" json:"target_model"`     // 目标模型名称
}

type LoggingConfig struct {
	Level           string   `yaml:"level"`
	LogRequestTypes string   `yaml:"log_request_types"`
	LogRequestBody  string   `yaml:"log_request_body"`
	LogResponseBody string   `yaml:"log_response_body"`
	LogDirectory    string   `yaml:"log_directory"`
	ExcludePaths    []string `yaml:"exclude_paths,omitempty"` // 新增：不记录日志的路径列表
}

type ValidationConfig struct {
	PythonJSONFixing PythonJSONFixingConfig `yaml:"python_json_fixing"`
}

// PythonJSONFixing 配置结构
type PythonJSONFixingConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`             // 是否启用 Python JSON 修复
	TargetTools  []string `yaml:"target_tools" json:"target_tools"`   // 需要修复的工具列表
	DebugLogging bool     `yaml:"debug_logging" json:"debug_logging"` // 是否启用调试日志
	MaxAttempts  int      `yaml:"max_attempts" json:"max_attempts"`   // 最大修复尝试次数
}

// 网络超时配置（代理和健康检查共用）
type TimeoutConfig struct {
	// 网络超时设置（代理和健康检查共用）
	TLSHandshake   string `yaml:"tls_handshake" json:"tls_handshake"`     // TLS握手超时，默认10s
	ResponseHeader string `yaml:"response_header" json:"response_header"` // 响应头超时，默认60s
	IdleConnection string `yaml:"idle_connection" json:"idle_connection"` // 空闲连接超时，默认90s
	// 健康检查特有配置
	HealthCheckTimeout string `yaml:"health_check_timeout" json:"health_check_timeout"` // 健康检查整体响应超时，默认30s
	CheckInterval      string `yaml:"check_interval" json:"check_interval"`             // 健康检查间隔，默认30s
	RecoveryThreshold  int    `yaml:"recovery_threshold" json:"recovery_threshold"`     // 连续成功多少次后恢复端点，默认1
}

// 代理客户端超时配置（内部使用，从TimeoutConfig转换）
type ProxyTimeoutConfig struct {
	TLSHandshake   string `yaml:"tls_handshake" json:"tls_handshake"`
	ResponseHeader string `yaml:"response_header" json:"response_header"`
	IdleConnection string `yaml:"idle_connection" json:"idle_connection"`
	OverallRequest string `yaml:"overall_request" json:"overall_request"` // 保持为空，无限制
}

// 健康检查超时配置（内部使用，从TimeoutConfig转换）
type HealthCheckTimeoutConfig struct {
	TLSHandshake      string `yaml:"tls_handshake" json:"tls_handshake"`
	ResponseHeader    string `yaml:"response_header" json:"response_header"`
	IdleConnection    string `yaml:"idle_connection" json:"idle_connection"`
	OverallRequest    string `yaml:"overall_request" json:"overall_request"`
	CheckInterval     string `yaml:"check_interval" json:"check_interval"`
	RecoveryThreshold int    `yaml:"recovery_threshold" json:"recovery_threshold"`
}

// ToProxyTimeoutConfig 将TimeoutConfig转换为ProxyTimeoutConfig
func (tc *TimeoutConfig) ToProxyTimeoutConfig() ProxyTimeoutConfig {
	return ProxyTimeoutConfig{
		TLSHandshake:   tc.TLSHandshake,
		ResponseHeader: tc.ResponseHeader,
		IdleConnection: tc.IdleConnection,
		OverallRequest: "", // 代理不设置整体超时，支持流式响应
	}
}

// ToHealthCheckTimeoutConfig 将TimeoutConfig转换为HealthCheckTimeoutConfig
func (tc *TimeoutConfig) ToHealthCheckTimeoutConfig() HealthCheckTimeoutConfig {
	return HealthCheckTimeoutConfig{
		TLSHandshake:      tc.TLSHandshake,
		ResponseHeader:    tc.ResponseHeader,
		IdleConnection:    tc.IdleConnection,
		OverallRequest:    tc.HealthCheckTimeout,
		CheckInterval:     tc.CheckInterval,
		RecoveryThreshold: tc.RecoveryThreshold,
	}
}

// Tag系统配置结构 (永远启用)
type TaggingConfig struct {
	PipelineTimeout string         `yaml:"pipeline_timeout"`
	Taggers         []TaggerConfig `yaml:"taggers"`
}

// 端点拉黑配置结构
type BlacklistConfig struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`                         // 是否启用端点拉黑功能
	AutoBlacklist     bool `yaml:"auto_blacklist" json:"auto_blacklist"`           // 是否自动拉黑失败的端点
	BusinessErrorSafe bool `yaml:"business_error_safe" json:"business_error_safe"` // 业务错误是否安全（不触发拉黑）
	ConfigErrorSafe   bool `yaml:"config_error_safe" json:"config_error_safe"`     // 配置错误是否安全（不触发拉黑）
	ServerErrorSafe   bool `yaml:"server_error_safe" json:"server_error_safe"`     // 服务器错误是否安全（不触发拉黑）
	SSEValidationSafe bool `yaml:"sse_validation_safe" json:"sse_validation_safe"` // SSE验证错误是否安全（不触发拉黑）
}

type TaggerConfig struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`         // "builtin" | "starlark"
	BuiltinType string                 `yaml:"builtin_type"` // 内置类型: "path" | "header" | "body-json" | "method" | "query"
	Tag         string                 `yaml:"tag"`          // 标记的tag名称
	Enabled     bool                   `yaml:"enabled"`
	Priority    int                    `yaml:"priority"` // 执行优先级(未使用，因为并发执行)
	Config      map[string]interface{} `yaml:"config"`   // tagger特定配置
}

// ConversionConfig 格式转换配置
type ConversionConfig struct {
	// 转换适配器模式：legacy | unified | auto
	// legacy: 使用原有的直接函数调用转换
	// unified: 使用新的适配器架构转换（推荐）
	// auto: 智能选择，优先使用unified，失败时自动回退到legacy
	AdapterMode string `yaml:"adapter_mode" json:"adapter_mode"`
	// 转换模式验证：在模式切换后进行小流量验证
	ValidateModeSwitch bool `yaml:"validate_mode_switch" json:"validate_mode_switch"`
	// 转换失败回退阈值：当失败率达到此百分比时，自动回退到legacy模式
	FailbackThreshold int `yaml:"failback_threshold" json:"failback_threshold"` // 默认: 30 (30%)
}

// RetryConfig 重试策略配置
type RetryConfig struct {
	UpstreamErrors []UpstreamErrorRule `yaml:"upstream_errors" json:"upstream_errors"`
}

// UpstreamErrorRule 定义上游错误的匹配与处理方式
type UpstreamErrorRule struct {
	Pattern         string `yaml:"pattern" json:"pattern"`
	Action          string `yaml:"action,omitempty" json:"action,omitempty"`           // retry_endpoint | switch_endpoint
	MaxRetries      int    `yaml:"max_retries,omitempty" json:"max_retries,omitempty"` // 同一端点最大重试次数（受全局限制）
	CaseInsensitive bool   `yaml:"case_insensitive,omitempty" json:"case_insensitive,omitempty"`
}

// （已移除）全局 ToolCallingConfig：采用零配置 + 端点级自动学习/开关

// StreamingConfig 流式转换配置
type StreamingConfig struct {
	Timeout             string `yaml:"timeout" json:"timeout"`                             // 流式超时时间，默认30s
	MaxRetries          int    `yaml:"max_retries" json:"max_retries"`                     // 最大重试次数，默认3
	MinChunkSize        int    `yaml:"min_chunk_size" json:"min_chunk_size"`               // 最小数据包大小，默认10
	EnableSSEValidation bool   `yaml:"enable_sse_validation" json:"enable_sse_validation"` // 是否启用SSE格式验证
	EnableCaching       bool   `yaml:"enable_caching" json:"enable_caching"`               // 是否启用流式缓存
}

// ToolsConfig 工具调用配置
type ToolsConfig struct {
	Timeout          string `yaml:"timeout" json:"timeout"`                     // 工具调用超时时间，默认60s
	MaxParallel      int    `yaml:"max_parallel" json:"max_parallel"`           // 最大并行工具数，默认10
	EnableValidation bool   `yaml:"enable_validation" json:"enable_validation"` // 是否启用工具验证
	EnableCaching    bool   `yaml:"enable_caching" json:"enable_caching"`       // 是否启用工具缓存
}

// HTTPClientConfig HTTP客户端配置
type HTTPClientConfig struct {
	MaxConnsPerHost   int  `yaml:"max_conns_per_host" json:"max_conns_per_host"`   // 每主机最大连接数，默认10
	WriteBufferSize   int  `yaml:"write_buffer_size" json:"write_buffer_size"`     // 写缓冲区大小，默认4096
	ReadBufferSize    int  `yaml:"read_buffer_size" json:"read_buffer_size"`       // 读缓冲区大小，默认4096
	ForceAttemptHTTP2 bool `yaml:"force_attempt_http2" json:"force_attempt_http2"` // 是否强制使用HTTP/2
	EnableCompression bool `yaml:"enable_compression" json:"enable_compression"`   // 是否启用压缩
	EnableKeepAlive   bool `yaml:"enable_keep_alive" json:"enable_keep_alive"`     // 是否启用长连接
}

// MonitoringConfig 性能监控配置
type MonitoringConfig struct {
	CollectionInterval    string `yaml:"collection_interval" json:"collection_interval"`         // 指标收集间隔，默认60s
	SlowRequestThreshold  string `yaml:"slow_request_threshold" json:"slow_request_threshold"`   // 慢请求阈值，默认5s
	EnableDetailedMetrics bool   `yaml:"enable_detailed_metrics" json:"enable_detailed_metrics"` // 是否启用详细指标
	EnableRequestTracing  bool   `yaml:"enable_request_tracing" json:"enable_request_tracing"`   // 是否启用请求追踪
}

// FormatDetectionConfig 格式检测配置
type FormatDetectionConfig struct {
	CacheMaxSize                 int  `yaml:"cache_max_size" json:"cache_max_size"`                                   // 缓存最大大小，默认1000
	LRUCacheSize                 int  `yaml:"lru_cache_size" json:"lru_cache_size"`                                   // LRU缓存大小，默认500
	EnablePathCaching            bool `yaml:"enable_path_caching" json:"enable_path_caching"`                         // 是否启用路径缓存
	EnableBodyStructureDetection bool `yaml:"enable_body_structure_detection" json:"enable_body_structure_detection"` // 是否启用请求体结构检测
}
