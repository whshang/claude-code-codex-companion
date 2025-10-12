package config

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Endpoints  []EndpointConfig `yaml:"endpoints"`
	Logging    LoggingConfig    `yaml:"logging"`
	Validation ValidationConfig `yaml:"validation"`
	Tagging    TaggingConfig    `yaml:"tagging"`   // 标签系统配置（永远启用）
	Timeouts   TimeoutConfig    `yaml:"timeouts"`  // 超时配置
	I18n       I18nConfig       `yaml:"i18n"`      // 国际化配置
	Blacklist  BlacklistConfig  `yaml:"blacklist"` // 端点拉黑配置
	Conversion ConversionConfig `yaml:"conversion"` // 格式转换配置
}

// I18nConfig 国际化配置
type I18nConfig struct {
	Enabled         bool   `yaml:"enabled"`          // 是否启用国际化
	DefaultLanguage string `yaml:"default_language"` // 默认语言
	LocalesPath     string `yaml:"locales_path"`     // 语言文件路径
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type EndpointConfig struct {
	Name               string              `yaml:"name"`
	URLAnthropic       string              `yaml:"url_anthropic,omitempty"` // Anthropic格式URL
	URLOpenAI          string              `yaml:"url_openai,omitempty"`    // OpenAI格式URL
	AuthType           string              `yaml:"auth_type"`
	AuthValue          string              `yaml:"auth_value"`
	Enabled            bool                `yaml:"enabled"`
	Priority           int                 `yaml:"priority"`
	Tags               []string            `yaml:"tags"`                                                                   // 新增：支持的tag列表
	ModelRewrite       *ModelRewriteConfig `yaml:"model_rewrite,omitempty"`                                                // 新增：模型重写配置
	Proxy              *ProxyConfig        `yaml:"proxy,omitempty"`                                                        // 新增：代理配置
	OAuthConfig        *OAuthConfig        `yaml:"oauth_config,omitempty"`                                                 // 新增：OAuth配置
	HeaderOverrides    map[string]string   `yaml:"header_overrides,omitempty" json:"header_overrides,omitempty"`           // 新增：HTTP Header覆盖配置
	ParameterOverrides map[string]string   `yaml:"parameter_overrides,omitempty" json:"parameter_overrides,omitempty"`     // 新增：Request Parameters覆盖配置
	MaxTokensFieldName string              `yaml:"max_tokens_field_name,omitempty" json:"max_tokens_field_name,omitempty"` // max_tokens 参数名转换选项
	RateLimitReset     *int64              `yaml:"rate_limit_reset,omitempty" json:"rate_limit_reset,omitempty"`           // Anthropic-Ratelimit-Unified-Reset
	RateLimitStatus    *string             `yaml:"rate_limit_status,omitempty" json:"rate_limit_status,omitempty"`         // Anthropic-Ratelimit-Unified-Status
	EnhancedProtection bool                `yaml:"enhanced_protection,omitempty" json:"enhanced_protection,omitempty"`     // 官方帐号增强保护：allowed_warning时即禁用端点
	SSEConfig          *SSEConfig          `yaml:"sse_config,omitempty" json:"sse_config,omitempty"`                       // SSE行为配置
	OpenAIPreference   string              `yaml:"openai_preference,omitempty" json:"openai_preference,omitempty"`         // OpenAI格式偏好："responses"|"chat_completions"|"auto"
	// 新增：原生工具调用支持（学习或手动设置）。nil 表示未知/未学习。
	NativeToolSupport *bool `yaml:"native_tool_support,omitempty" json:"native_tool_support,omitempty"`
	// 新增：工具调用增强模式："auto"(默认) | "force"(总是注入增强) | "disable"(从不注入增强)
	ToolEnhancementMode string `yaml:"tool_enhancement_mode,omitempty" json:"tool_enhancement_mode,omitempty"`
	// 新增：是否允许使用 /count_tokens 接口（默认开启）
	CountTokensEnabled *bool `yaml:"count_tokens_enabled,omitempty" json:"count_tokens_enabled,omitempty"`
	// 新增：显式声明是否原生支持 /responses 接口（true 表示支持，false 表示需要转换）
	SupportsResponses *bool `yaml:"supports_responses,omitempty" json:"supports_responses,omitempty"`
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
	Enabled bool               `yaml:"enabled" json:"enabled"` // 是否启用模型重写
	Rules   []ModelRewriteRule `yaml:"rules" json:"rules"`     // 重写规则列表
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

// 新增：超时配置结构
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

// （已移除）全局 ToolCallingConfig：采用零配置 + 端点级自动学习/开关
