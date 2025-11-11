package endpoint

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"claude-code-codex-companion/internal/common/httpclient"
	commonutils "claude-code-codex-companion/internal/common/utils"
	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/interfaces"
	"claude-code-codex-companion/internal/oauth"
	"claude-code-codex-companion/internal/statistics"
	"claude-code-codex-companion/internal/utils"
)

type Status string

const (
	StatusActive      Status = "active"
	StatusInactive    Status = "inactive"
	StatusChecking    Status = "checking"
	StatusRecovering  Status = "recovering"  // 恢复中
	StatusDegraded    Status = "degraded"    // 性能降级
	StatusBlacklisted Status = "blacklisted" // 已拉黑
)

// BlacklistReason 记录端点被拉黑的原因
type BlacklistReason struct {
	// 导致失效的请求ID列表
	CausingRequestIDs []string `json:"causing_request_ids"`

	// 失效时间
	BlacklistedAt time.Time `json:"blacklisted_at"`

	// 失效时的错误信息摘要
	ErrorSummary string `json:"error_summary"`
}

// 删除不再需要的 RequestRecord 定义，因为已经移到 utils 包

type Endpoint struct {
	ID                 string                     `json:"id"`
	Name               string                     `json:"name"`
	URLAnthropic       string                     `json:"url_anthropic,omitempty"` // Anthropic格式URL
	URLOpenAI          string                     `json:"url_openai"`              // OpenAI格式URL
	URLGemini          string                     `json:"url_gemini,omitempty"`    // Gemini格式URL
	EndpointType       string                     `json:"endpoint_type"`           // 自动推断的端点类型（内部使用）
	AuthType           string                     `json:"auth_type"`
	AuthValue          string                     `json:"auth_value"`
	Enabled            bool                       `json:"enabled"`
	Priority           int                        `json:"priority"`
	Tags               []string                   `json:"tags"`                            // 新增：支持的tag列表
	ModelRewrite       *config.ModelRewriteConfig `json:"model_rewrite,omitempty"`         // 新增：模型重写配置
	Proxy              *config.ProxyConfig        `json:"proxy,omitempty"`                 // 新增：代理配置
	OAuthConfig        *config.OAuthConfig        `json:"oauth_config,omitempty"`          // 新增：OAuth配置
	HeaderOverrides    map[string]string          `json:"header_overrides,omitempty"`      // 新增：HTTP Header覆盖配置
	ParameterOverrides map[string]string          `json:"parameter_overrides,omitempty"`   // 新增：Request Parameters覆盖配置
	MaxTokensFieldName string                     `json:"max_tokens_field_name,omitempty"` // max_tokens 参数名转换选项
	RateLimitReset     *int64                     `json:"rate_limit_reset,omitempty"`      // Anthropic-Ratelimit-Unified-Reset
	RateLimitStatus    *string                    `json:"rate_limit_status,omitempty"`     // Anthropic-Ratelimit-Unified-Status
	EnhancedProtection bool                       `json:"enhanced_protection,omitempty"`   // 官方帐号增强保护：allowed_warning时即禁用端点
	SSEConfig          *config.SSEConfig          `json:"sse_config,omitempty"`            // SSE行为配置
	OpenAIPreference   string                     `json:"openai_preference,omitempty"`     // OpenAI格式偏好："responses"|"chat_completions"|"auto"
	SupportsResponses  *bool                      `json:"supports_responses,omitempty"`    // 显式声明 /responses 支持情况
	// 是否允许使用 /count_tokens 接口
	CountTokensEnabled bool `json:"count_tokens_enabled"`
	// 记录 count_tokens 支持情况（nil 表示未知）
	CountTokensSupport  *bool `json:"-"`
	countTokensMutex    sync.RWMutex
	Status              Status                `json:"status"`
	LastCheck           time.Time             `json:"last_check"`
	FailureCount        int                   `json:"failure_count"`
	TotalRequests       int                   `json:"total_requests"`
	SuccessRequests     int                   `json:"success_requests"`
	LastFailure         time.Time             `json:"last_failure"`
	SuccessiveSuccesses int                   `json:"successive_successes"` // 连续成功次数
	RequestHistory      *utils.CircularBuffer `json:"-"`                    // 使用环形缓冲区，不导出到JSON

	// 新增：被拉黑的原因（内存中，不持久化）
	BlacklistReason *BlacklistReason `json:"-"`

	// 新增：保护 BlacklistReason 的互斥锁
	blacklistMutex sync.RWMutex

	// 新增：上次记录跳过健康检查日志的时间（用于减少日志频率）
	lastSkipLogTime time.Time `json:"-"`

	// 新增：是否原生支持 Codex 格式（用于 /responses 路径的自动探测）
	// nil = 未探测，true = 支持原生 Codex 格式，false = 需要转换为 OpenAI 格式
	NativeCodexFormat *bool `json:"native_codex_format,omitempty"`

	// 新增：自动学习到的不支持的参数列表（运行时学习，不持久化）
	// 当API返回400错误时，自动检测并记录哪些参数不被支持
	// 例如：["tools", "tool_choice"] 表示这个端点不支持函数调用
	LearnedUnsupportedParams []string `json:"-"`

	// 新增：保护 LearnedUnsupportedParams 的互斥锁
	learnedParamsMutex sync.RWMutex

	// 新增：自动检测到的有效认证方式（运行时学习，不持久化）
	// "x-api-key" 或 "Authorization" 或空字符串(未检测)
	DetectedAuthHeader string `json:"-"`

	// 新增：保护 DetectedAuthHeader 的互斥锁
	AuthHeaderMutex sync.RWMutex

	// 新增：动态排序器引用（用于状态变化时触发排序更新）
	dynamicSorter *utils.DynamicEndpointSorter `json:"-"`

	// 方案A新增字段：智能转换标记
	NativeFormat bool   `json:"native_format"` // 是否原生支持客户端格式（true=无需转换）
	TargetFormat string `json:"target_format"` // 转换目标格式："anthropic"|"openai_chat"|"openai_responses"|"gemini"
	ClientType   string `json:"client_type"`   // 客户端类型过滤："claude_code"|"codex"|"openai"|"gemini"|""（空表示通用）

	// 统计信息
	Stats *statistics.EndpointStatistics `json:"-"`

	mutex sync.RWMutex
}

func NewEndpoint(cfg config.EndpointConfig) *Endpoint {
	// 自动推断端点类型
	endpointType := inferEndpointType(cfg)

	countTokensEnabled := true
	if cfg.CountTokensEnabled != nil {
		countTokensEnabled = *cfg.CountTokensEnabled
	}

	openAIPreference := cfg.OpenAIPreference
	var nativeCodexFormat *bool
	if cfg.SupportsResponses != nil {
		val := *cfg.SupportsResponses
		copyVal := val
		nativeCodexFormat = &copyVal
		if val {
			if openAIPreference == "" || openAIPreference == "auto" {
				openAIPreference = "responses"
			}
		} else {
			if openAIPreference == "" || openAIPreference == "auto" {
				openAIPreference = "chat_completions"
			}
		}
	}

	// 方案A：自动推断 ClientType, NativeFormat 和 TargetFormat
	nativeFormat := cfg.NativeFormat
	targetFormat := cfg.TargetFormat
	clientType := cfg.ClientType

	// 1. 自动推断 client_type (基于URL配置)
	if clientType == "" {
		if cfg.URLAnthropic != "" && cfg.URLOpenAI == "" {
			// 只有Anthropic URL → 专为Claude Code客户端
			clientType = "claude_code"
		} else if cfg.URLOpenAI != "" && cfg.URLAnthropic == "" {
			// 只有OpenAI URL → 专为Codex客户端
			clientType = "codex"
		}
		// 两个URL都有 → 保持空字符串，表示universal（真正的通用端点）
	}

	// 2. 自动推断 native_format 和 target_format
	if !nativeFormat && targetFormat == "" {
		// 有Anthropic URL且无OpenAI URL -> 原生支持Anthropic
		if cfg.URLAnthropic != "" && cfg.URLOpenAI == "" {
			nativeFormat = true
		}
		// 有OpenAI URL -> 需要判断是否需要转换
		if cfg.URLOpenAI != "" {
			// 如果同时有两个URL，认为是智能路由，原生支持
			if cfg.URLAnthropic != "" {
				nativeFormat = true
			} else {
				// 只有OpenAI URL，可能需要转换
				nativeFormat = false
				targetFormat = "openai_chat" // 默认目标格式
			}
		}
	}

	return &Endpoint{
		ID:                 generateID(cfg.Name),
		Name:               cfg.Name,
		URLAnthropic:       cfg.URLAnthropic, // Anthropic格式URL
		URLOpenAI:          cfg.URLOpenAI,    // OpenAI格式URL
		URLGemini:          cfg.URLGemini,    // Gemini格式URL
		EndpointType:       endpointType,
		AuthType:           cfg.AuthType,
		AuthValue:          cfg.AuthValue,
		Enabled:            config.GetBoolWithDefault(cfg.Enabled, true, config.Default.Endpoint.Enabled),
		Priority:           config.GetIntWithDefault(cfg.Priority, config.Default.Endpoint.Priority),
		Tags:               cfg.Tags,
		ModelRewrite:       cfg.ModelRewrite,
		Proxy:              cfg.Proxy,
		OAuthConfig:        cfg.OAuthConfig,
		NativeFormat:       nativeFormat,
		TargetFormat:       targetFormat,
		ClientType:         clientType,
		HeaderOverrides:    cfg.HeaderOverrides,
		ParameterOverrides: cfg.ParameterOverrides,
		MaxTokensFieldName: cfg.MaxTokensFieldName,
		RateLimitReset:     cfg.RateLimitReset,
		RateLimitStatus:    cfg.RateLimitStatus,
		EnhancedProtection: cfg.EnhancedProtection,
		SSEConfig:          cfg.SSEConfig,
		OpenAIPreference:   openAIPreference,
		SupportsResponses:  cfg.SupportsResponses,
		CountTokensEnabled: countTokensEnabled,
		NativeCodexFormat:  nativeCodexFormat,
		Status:             StatusActive,
		LastCheck:          time.Now(),
		RequestHistory:     utils.NewCircularBuffer(100, 140*time.Second),
	}
}

// inferEndpointType 自动推断端点类型
func inferEndpointType(cfg config.EndpointConfig) string {
	// 根据配置的URL推断端点类型
	if cfg.URLGemini != "" && cfg.URLAnthropic == "" && cfg.URLOpenAI == "" {
		// 只有Gemini URL
		return "gemini"
	}
	if cfg.URLAnthropic != "" && cfg.URLOpenAI == "" && cfg.URLGemini == "" {
		// 只有Anthropic URL
		return "anthropic"
	}
	if cfg.URLOpenAI != "" && cfg.URLAnthropic == "" && cfg.URLGemini == "" {
		// 只有OpenAI URL
		return "openai"
	}
	if cfg.URLAnthropic != "" && cfg.URLOpenAI != "" && cfg.URLGemini == "" {
		// Anthropic和OpenAI URL都有，优先使用openai（支持Codex客户端智能路由）
		return "openai"
	}
	if cfg.URLAnthropic != "" && cfg.URLGemini != "" && cfg.URLOpenAI == "" {
		// Anthropic和Gemini URL都有，优先使用anthropic
		return "anthropic"
	}
	if cfg.URLOpenAI != "" && cfg.URLGemini != "" && cfg.URLAnthropic == "" {
		// OpenAI和Gemini URL都有，优先使用openai
		return "openai"
	}
	if cfg.URLAnthropic != "" && cfg.URLOpenAI != "" && cfg.URLGemini != "" {
		// 三个URL都有，优先使用openai（最通用的格式）
		return "openai"
	}

	// 默认值（不应该到达这里，因为配置验证会确保至少有一个URL）
	return "anthropic"
}

// 实现 EndpointSorter 接口
func (e *Endpoint) GetPriority() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.Priority
}

func (e *Endpoint) IsEnabled() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.Enabled
}

func (e *Endpoint) GetAuthHeader() (string, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	switch e.AuthType {
	case "api_key":
		return e.AuthValue, nil // api_key 直接返回值，会用 x-api-key 头部
	case "auth_token":
		return "Bearer " + e.AuthValue, nil // auth_token 使用 Bearer 前缀
	case "oauth":
		if e.OAuthConfig == nil {
			return "", fmt.Errorf("oauth config is required for oauth auth_type")
		}

		// 检查 token 是否需要刷新
		if oauth.IsTokenExpired(e.OAuthConfig) {
			return "", fmt.Errorf("oauth token expired, refresh required")
		}

		return oauth.GetAuthorizationHeader(e.OAuthConfig), nil
	case "auto":
		// auto 类型默认使用 Bearer 格式（与 proxy_logic.go 中的期望一致）
		return "Bearer " + e.AuthValue, nil
	default:
		return e.AuthValue, nil
	}
}

func (e *Endpoint) GetTags() []string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// 返回tags的副本以避免并发修改
	tags := make([]string, len(e.Tags))
	copy(tags, e.Tags)
	return tags
}

// GetHeaderOverrides 安全地获取Header覆盖配置的副本
func (e *Endpoint) GetHeaderOverrides() map[string]string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if e.HeaderOverrides == nil {
		return nil
	}

	// 返回HeaderOverrides的副本以避免并发修改
	overrides := make(map[string]string, len(e.HeaderOverrides))
	for k, v := range e.HeaderOverrides {
		overrides[k] = v
	}
	return overrides
}

// GetParameterOverrides 安全地获取Parameter覆盖配置的副本
func (e *Endpoint) GetParameterOverrides() map[string]string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if e.ParameterOverrides == nil {
		return nil
	}

	// 返回ParameterOverrides的副本以避免并发修改
	overrides := make(map[string]string, len(e.ParameterOverrides))
	for k, v := range e.ParameterOverrides {
		overrides[k] = v
	}
	return overrides
}

// ToTaggedEndpoint 将Endpoint转换为TaggedEndpoint
func (e *Endpoint) ToTaggedEndpoint() interfaces.TaggedEndpoint {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	tags := make([]string, len(e.Tags))
	copy(tags, e.Tags)

	// 优先使用Anthropic URL，如果为空则使用OpenAI URL
	url := e.URLAnthropic
	if url == "" {
		url = e.URLOpenAI
	}

	return interfaces.TaggedEndpoint{
		Name:     e.Name,
		URL:      url,
		Tags:     tags,
		Priority: e.Priority,
		Enabled:  e.Enabled,
	}
}

// GetEffectiveURL 根据请求格式返回对应的URL
// requestFormat: "anthropic" | "openai" | "gemini"
func (e *Endpoint) GetEffectiveURL(requestFormat string) string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// 根据请求格式选择合适的URL
	if requestFormat == "anthropic" && e.URLAnthropic != "" {
		return e.URLAnthropic
	}
	if requestFormat == "openai" && e.URLOpenAI != "" {
		return e.URLOpenAI
	}
	if requestFormat == "gemini" && e.URLGemini != "" {
		return e.URLGemini
	}

	// 默认策略：优先使用Anthropic URL，其次OpenAI URL，最后Gemini URL
	if e.URLAnthropic != "" {
		return e.URLAnthropic
	}
	if e.URLOpenAI != "" {
		return e.URLOpenAI
	}
	return e.URLGemini
}

func (e *Endpoint) GetFullURL(path string) string {
	return e.GetFullURLWithFormat(path, "")
}

// HasURLForFormat 检查端点是否配置了指定格式的URL
// requestFormat: "anthropic" | "openai" | "gemini"
func (e *Endpoint) HasURLForFormat(requestFormat string) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	switch requestFormat {
	case "anthropic":
		return e.URLAnthropic != ""
	case "openai":
		return e.URLOpenAI != ""
	case "gemini":
		return e.URLGemini != ""
	default:
		// 未知格式或未指定，检查是否至少有一个URL
		return e.URLAnthropic != "" || e.URLOpenAI != "" || e.URLGemini != ""
	}
}

// GetFullURLWithFormat 根据请求格式构建完整URL
// 支持格式转换：当端点没有对应格式的URL时，使用另一个家族的URL并依赖格式转换逻辑
//
// 规则：
// 1. 仅在端点为指定格式配置了URL时返回构造后的结果
// 2. 未配置对应格式的URL时直接返回空字符串，由调用方决定是否执行格式转换
// 3. 当未指定格式时保持历史优先级（Anthropic → OpenAI → Gemini）
func (e *Endpoint) GetFullURLWithFormat(path string, requestFormat string) string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// 获取对应格式的URL
	var baseURL string
	switch strings.ToLower(requestFormat) {
	case "anthropic":
		baseURL = e.URLAnthropic
	case "openai":
		baseURL = e.URLOpenAI
	case "gemini":
		baseURL = e.URLGemini
	default:
		// 未指定格式或格式未知 - 使用传统的优先级策略（向后兼容）
		if e.URLAnthropic != "" {
			baseURL = e.URLAnthropic
		} else if e.URLOpenAI != "" {
			baseURL = e.URLOpenAI
		} else {
			baseURL = e.URLGemini
		}
	}

	if baseURL == "" {
		return ""
	}

	// 如果请求格式未指定，根据选中的URL推断
	format := strings.ToLower(requestFormat)
	if format == "" {
		switch baseURL {
		case e.URLAnthropic:
			format = "anthropic"
		case e.URLOpenAI:
			format = "openai"
		case e.URLGemini:
			format = "gemini"
		}
	}

	trimmedBase := strings.TrimRight(baseURL, "/")

	hasPathSegment := func(base, segment string) bool {
		return strings.HasSuffix(base, segment) || strings.Contains(base, segment+"/")
	}

	// 智能添加版本前缀（当路径中没有显式的 /v1/ 时）
	if !strings.Contains(path, "/v1/") {
		switch format {
		case "anthropic":
			needsV1 := false
			if strings.Contains(trimmedBase, "api.anthropic.com") && !hasPathSegment(trimmedBase, "/v1") {
				needsV1 = true
			}
			if (strings.Contains(path, "/messages") || strings.Contains(path, "/complete")) && !hasPathSegment(trimmedBase, "/v1") {
				needsV1 = true
			}
			if needsV1 {
				trimmedBase = trimmedBase + "/v1"
			}
		case "openai":
			if (strings.Contains(path, "/chat/completions") ||
				strings.Contains(path, "/completions") ||
				strings.Contains(path, "/responses")) && !hasPathSegment(trimmedBase, "/v1") {
				trimmedBase = trimmedBase + "/v1"
			}
		case "gemini":
			if strings.Contains(path, "/models/") && !hasPathSegment(trimmedBase, "/v1beta") {
				trimmedBase = trimmedBase + "/v1beta"
			}
		}
	}

	// Gemini在添加v1beta后直接返回
	if format == "gemini" && strings.Contains(trimmedBase, "/v1beta") && strings.Contains(path, "/models/") {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return trimmedBase + path
	}

	if path == "" || path == "/" {
		return trimmedBase
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return trimmedBase + path
}

// 优化 IsAvailable 方法，减少锁的持有时间
func (e *Endpoint) IsAvailable() bool {
	e.mutex.RLock()
	enabled := e.Enabled
	status := e.Status
	e.mutex.RUnlock()

	return enabled && status == StatusActive
}

// RecordRequest 记录请求结果
// firstByteTime: 首字节返回时间（TTFB），用于性能排序
// responseTime: 完整响应时间（包含下载），用于统计参考
func (e *Endpoint) RecordRequest(success bool, requestID string, firstByteTime time.Duration, responseTime time.Duration) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	now := time.Now()

	// 添加到环形缓冲区（包含请求ID、首字节时间和完整响应时间）
	record := utils.RequestRecord{
		Timestamp:     now,
		Success:       success,
		RequestID:     requestID,
		FirstByteTime: firstByteTime,
		ResponseTime:  responseTime,
	}
	e.RequestHistory.Add(record)

	e.TotalRequests++
	if success {
		e.SuccessRequests++
		e.FailureCount = 0      // 重置失败计数
		e.SuccessiveSuccesses++ // 增加连续成功次数
		// 如果成功且之前是不可用状态，恢复为可用
		if e.Status == StatusInactive {
			// 释放 mutex 以避免死锁，因为 MarkActive 需要获取 mutex
			e.mutex.Unlock()
			e.MarkActive()
			// 触发动态排序更新
			e.triggerDynamicSortUpdate()
			e.mutex.Lock()
		}
	} else {
		e.FailureCount++
		e.LastFailure = now
		e.SuccessiveSuccesses = 0 // 重置连续成功次数

		// 使用环形缓冲区检查是否应该标记为不可用
		if e.Status == StatusActive && e.RequestHistory.ShouldMarkInactive(now) {
			// 释放 mutex 以避免死锁，因为 MarkInactiveWithReason 需要获取 mutex
			e.mutex.Unlock()
			e.MarkInactiveWithReason()
			// 触发动态排序更新
			e.triggerDynamicSortUpdate()
			e.mutex.Lock()
		}
	}
}

func (e *Endpoint) MarkInactive() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Status = StatusInactive
}

// MarkInactiveWithReason 标记端点为失效并记录原因
func (e *Endpoint) MarkInactiveWithReason() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.Status == StatusActive {
		e.Status = StatusInactive

		// 从循环缓冲区获取导致失效的请求ID
		failedRequestIDs := e.RequestHistory.GetRecentFailureRequestIDs(time.Now())

		// 构建失效原因记录
		e.blacklistMutex.Lock()
		e.BlacklistReason = &BlacklistReason{
			BlacklistedAt:     time.Now(),
			CausingRequestIDs: failedRequestIDs,
			ErrorSummary:      fmt.Sprintf("Endpoint failed due to %d consecutive failures", len(failedRequestIDs)),
		}
		e.blacklistMutex.Unlock()
	}
}

func (e *Endpoint) MarkActive() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Status = StatusActive
	e.FailureCount = 0
	e.SuccessiveSuccesses = 0 // 重置连续成功次数

	// 清除失效原因记录
	e.blacklistMutex.Lock()
	e.BlacklistReason = nil
	e.blacklistMutex.Unlock()

	// 重置跳过健康检查日志时间，确保下次rate limit时能立即记录
	e.lastSkipLogTime = time.Time{}

	// 清理历史记录
	e.RequestHistory.Clear()
}

func (e *Endpoint) GetSuccessiveSuccesses() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.SuccessiveSuccesses
}

func generateID(name string) string {
	// Use stable ID based on endpoint name hash for statistics persistence
	return statistics.GenerateEndpointID(name)
}

// CreateProxyClient 为这个端点创建支持代理的HTTP客户端
func (e *Endpoint) CreateProxyClient(timeoutConfig config.ProxyTimeoutConfig) (*http.Client, error) {
	e.mutex.RLock()
	proxyConfig := e.Proxy
	e.mutex.RUnlock()

	factory := httpclient.NewFactory()
	clientConfig := httpclient.ClientConfig{
		Type: httpclient.ClientTypeEndpoint,
		Timeouts: httpclient.TimeoutConfig{
			TLSHandshake:   commonutils.ParseDuration(timeoutConfig.TLSHandshake, 10*time.Second),
			ResponseHeader: commonutils.ParseDuration(timeoutConfig.ResponseHeader, 60*time.Second),
			IdleConnection: commonutils.ParseDuration(timeoutConfig.IdleConnection, 90*time.Second),
			OverallRequest: commonutils.ParseDuration(timeoutConfig.OverallRequest, 0),
		},
		ProxyConfig: proxyConfig,
	}

	return factory.CreateClient(clientConfig)
}

// CreateHealthClient 为健康检查创建HTTP客户端（使用与代理相同的配置，但超时较短）
func (e *Endpoint) CreateHealthClient(timeoutConfig config.HealthCheckTimeoutConfig) (*http.Client, error) {
	e.mutex.RLock()
	proxyConfig := e.Proxy
	e.mutex.RUnlock()

	factory := httpclient.NewFactory()
	clientConfig := httpclient.ClientConfig{
		Type: httpclient.ClientTypeHealth,
		Timeouts: httpclient.TimeoutConfig{
			TLSHandshake:   commonutils.ParseDuration(timeoutConfig.TLSHandshake, 5*time.Second),
			ResponseHeader: commonutils.ParseDuration(timeoutConfig.ResponseHeader, 30*time.Second),
			IdleConnection: commonutils.ParseDuration(timeoutConfig.IdleConnection, 60*time.Second),
			OverallRequest: commonutils.ParseDuration(timeoutConfig.OverallRequest, 30*time.Second),
		},
		ProxyConfig: proxyConfig,
	}

	return factory.CreateClient(clientConfig)
}

// RefreshOAuthToken 刷新 OAuth token
func (e *Endpoint) RefreshOAuthToken(timeoutConfig config.ProxyTimeoutConfig) error {
	return e.RefreshOAuthTokenWithCallback(timeoutConfig, nil)
}

// RefreshOAuthTokenWithCallback 刷新 OAuth token 并可选地调用回调函数
func (e *Endpoint) RefreshOAuthTokenWithCallback(timeoutConfig config.ProxyTimeoutConfig, onTokenRefreshed func(*Endpoint) error) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.AuthType != "oauth" {
		return fmt.Errorf("endpoint is not configured for oauth authentication")
	}

	if e.OAuthConfig == nil {
		return fmt.Errorf("oauth config is nil")
	}

	// 创建HTTP客户端用于刷新请求
	factory := httpclient.NewFactory()
	clientConfig := httpclient.ClientConfig{
		Type: httpclient.ClientTypeProxy,
		Timeouts: httpclient.TimeoutConfig{
			TLSHandshake:   commonutils.ParseDuration(timeoutConfig.TLSHandshake, 10*time.Second),
			ResponseHeader: commonutils.ParseDuration(timeoutConfig.ResponseHeader, 60*time.Second),
			IdleConnection: commonutils.ParseDuration(timeoutConfig.IdleConnection, 90*time.Second),
			OverallRequest: commonutils.ParseDuration(timeoutConfig.OverallRequest, 30*time.Second),
		},
		ProxyConfig: e.Proxy,
	}

	client, err := factory.CreateClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create http client for token refresh: %v", err)
	}

	// 刷新token
	newOAuthConfig, err := oauth.RefreshToken(e.OAuthConfig, client)
	if err != nil {
		return fmt.Errorf("failed to refresh oauth token: %v", err)
	}

	// 更新配置
	e.OAuthConfig = newOAuthConfig

	// 如果提供了回调函数，调用它来处理配置持久化
	if onTokenRefreshed != nil {
		if err := onTokenRefreshed(e); err != nil {
			// 回调失败，但token已经刷新成功，只记录错误
			return fmt.Errorf("oauth token refreshed successfully but failed to persist to config file: %v", err)
		}
	}

	return nil
}

// GetAuthHeaderWithRefresh 获取认证头部，如果需要会自动刷新OAuth token
func (e *Endpoint) GetAuthHeaderWithRefresh(timeoutConfig config.ProxyTimeoutConfig) (string, error) {
	return e.GetAuthHeaderWithRefreshCallback(timeoutConfig, nil)
}

// GetAuthHeaderWithRefreshCallback 获取认证头部，如果需要会自动刷新OAuth token，支持回调
func (e *Endpoint) GetAuthHeaderWithRefreshCallback(timeoutConfig config.ProxyTimeoutConfig, onTokenRefreshed func(*Endpoint) error) (string, error) {
	// 首先尝试获取认证头部
	authHeader, err := e.GetAuthHeader()

	if e.AuthType == "oauth" {
		if err != nil {
			// 如果获取失败且token确实过期，尝试刷新
			if oauth.IsTokenExpired(e.OAuthConfig) {
				if refreshErr := e.RefreshOAuthTokenWithCallback(timeoutConfig, onTokenRefreshed); refreshErr != nil {
					return "", fmt.Errorf("failed to refresh oauth token: %v", refreshErr)
				}
				// 重新获取认证头部
				return e.GetAuthHeader()
			}
			// 如果不是因为过期导致的错误，直接返回错误
			return "", err
		}

		// 即使获取成功，也检查是否应该主动刷新
		if oauth.ShouldRefreshToken(e.OAuthConfig) {
			// 主动刷新，但如果失败不影响当前请求
			if refreshErr := e.RefreshOAuthTokenWithCallback(timeoutConfig, onTokenRefreshed); refreshErr != nil {
				// 刷新失败，记录日志但继续使用当前token
				// 这里可以添加日志记录
			} else {
				// 刷新成功，获取新的认证头部
				if newAuthHeader, newErr := e.GetAuthHeader(); newErr == nil {
					return newAuthHeader, nil
				}
			}
		}
	}

	return authHeader, err
}

// GetBlacklistReason 安全地获取被拉黑原因信息
func (e *Endpoint) GetBlacklistReason() *BlacklistReason {
	e.blacklistMutex.RLock()
	defer e.blacklistMutex.RUnlock()

	if e.BlacklistReason == nil {
		return nil
	}

	// 返回深度拷贝以避免并发修改
	return &BlacklistReason{
		CausingRequestIDs: append([]string{}, e.BlacklistReason.CausingRequestIDs...),
		BlacklistedAt:     e.BlacklistReason.BlacklistedAt,
		ErrorSummary:      e.BlacklistReason.ErrorSummary,
	}
}

// UpdateRateLimitState 更新endpoint的rate limit状态（线程安全）
func (e *Endpoint) UpdateRateLimitState(reset *int64, status *string) (bool, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 检查是否有变化
	changed := false

	// 比较reset值
	if (e.RateLimitReset == nil) != (reset == nil) {
		changed = true
	} else if e.RateLimitReset != nil && reset != nil && *e.RateLimitReset != *reset {
		changed = true
	}

	// 比较status值
	if (e.RateLimitStatus == nil) != (status == nil) {
		changed = true
	} else if e.RateLimitStatus != nil && status != nil && *e.RateLimitStatus != *status {
		changed = true
	}

	// 如果有变化，更新状态
	if changed {
		e.RateLimitReset = reset
		e.RateLimitStatus = status
	}

	return changed, nil
}

// GetRateLimitState 安全地获取rate limit状态
func (e *Endpoint) GetRateLimitState() (*int64, *string) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	var reset *int64
	var status *string

	if e.RateLimitReset != nil {
		resetCopy := *e.RateLimitReset
		reset = &resetCopy
	}

	if e.RateLimitStatus != nil {
		statusCopy := *e.RateLimitStatus
		status = &statusCopy
	}

	return reset, status
}

// IsAnthropicEndpoint 检查是否为api.anthropic.com端点
func (e *Endpoint) IsAnthropicEndpoint() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	// 检查Anthropic URL是否包含官方域名
	if e.URLAnthropic != "" {
		return strings.Contains(strings.ToLower(e.URLAnthropic), "api.anthropic.com")
	}
	return false
}

// ShouldMonitorRateLimit 检查是否应该监控此端点的rate limit
func (e *Endpoint) ShouldMonitorRateLimit() bool {
	return e.IsAnthropicEndpoint()
}

// ShouldSkipCountTokens 判断是否已经确认该端点不支持 count_tokens
func (e *Endpoint) ShouldSkipCountTokens() bool {
	if !e.CountTokensEnabled {
		return true
	}
	e.countTokensMutex.RLock()
	defer e.countTokensMutex.RUnlock()

	return e.CountTokensSupport != nil && !*e.CountTokensSupport
}

// MarkCountTokensSupport 记录端点是否支持 count_tokens（运行时学习，不默认持久化）
func (e *Endpoint) MarkCountTokensSupport(supported bool) {
	e.countTokensMutex.Lock()
	defer e.countTokensMutex.Unlock()

	e.CountTokensSupport = &supported
}

// ShouldSkipHealthCheckUntilReset 检查是否应跳过健康检查直到rate limit reset时间
func (e *Endpoint) ShouldSkipHealthCheckUntilReset() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// 1. 必须是Anthropic官方端点
	if e.URLAnthropic == "" || !strings.Contains(strings.ToLower(e.URLAnthropic), "api.anthropic.com") {
		return false
	}

	// 2. 必须有rate limit reset信息
	if e.RateLimitReset == nil {
		return false
	}

	// 3. 当前时间必须小于reset时间
	currentTime := time.Now().Unix()
	return currentTime < *e.RateLimitReset
}

// GetRateLimitResetTimeRemaining 获取距离rate limit reset还有多长时间（秒）
func (e *Endpoint) GetRateLimitResetTimeRemaining() int64 {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if e.RateLimitReset == nil {
		return 0
	}

	currentTime := time.Now().Unix()
	remaining := *e.RateLimitReset - currentTime
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ShouldLogSkipHealthCheck 判断是否应该记录跳过健康检查的日志
// 策略：首次跳过时记录，然后每5分钟记录一次，避免日志过多
func (e *Endpoint) ShouldLogSkipHealthCheck() bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	now := time.Now()
	// 如果从未记录过，或者距离上次记录超过5分钟，则应该记录
	if e.lastSkipLogTime.IsZero() || now.Sub(e.lastSkipLogTime) >= 5*time.Minute {
		e.lastSkipLogTime = now
		return true
	}
	return false
}

// ShouldDisableOnAllowedWarning 检查是否应该在allowed_warning状态下禁用端点
// 只有同时满足以下条件时才返回true：
// 1. 启用了增强保护 (EnhancedProtection = true)
// 2. 是Anthropic官方端点 (api.anthropic.com)
// 3. 当前rate limit状态为allowed_warning
func (e *Endpoint) ShouldDisableOnAllowedWarning() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// 必须启用增强保护
	if !e.EnhancedProtection {
		return false
	}

	// 必须是Anthropic官方端点
	if e.URLAnthropic == "" || !strings.Contains(strings.ToLower(e.URLAnthropic), "api.anthropic.com") {
		return false
	}

	// 必须有rate limit status信息且为allowed_warning
	if e.RateLimitStatus == nil || *e.RateLimitStatus != "allowed_warning" {
		return false
	}

	return true
}

// UpdateNativeCodexSupport 动态更新端点的Codex支持状态
func (e *Endpoint) UpdateNativeCodexSupport(supported bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 如果已经有明确的判断，不再更新
	if e.NativeCodexFormat != nil {
		return
	}

	// 设置端点的Codex支持状态
	e.NativeCodexFormat = &supported
}

// LearnUnsupportedParam 记录一个不支持的参数
func (e *Endpoint) LearnUnsupportedParam(param string) {
	e.learnedParamsMutex.Lock()
	defer e.learnedParamsMutex.Unlock()

	// 检查是否已经记录
	for _, p := range e.LearnedUnsupportedParams {
		if p == param {
			return // 已存在
		}
	}

	e.LearnedUnsupportedParams = append(e.LearnedUnsupportedParams, param)
}

// IsParamUnsupported 检查参数是否已被学习为不支持
func (e *Endpoint) IsParamUnsupported(param string) bool {
	e.learnedParamsMutex.RLock()
	defer e.learnedParamsMutex.RUnlock()

	for _, p := range e.LearnedUnsupportedParams {
		if p == param {
			return true
		}
	}
	return false
}

// GetLearnedUnsupportedParams 获取所有学习到的不支持参数
func (e *Endpoint) GetLearnedUnsupportedParams() []string {
	e.learnedParamsMutex.RLock()
	defer e.learnedParamsMutex.RUnlock()

	result := make([]string, len(e.LearnedUnsupportedParams))
	copy(result, e.LearnedUnsupportedParams)
	return result
}

// GetURL 获取主URL用于日志记录等场景 (优先Anthropic URL)
// GetURL 返回端点的基础URL（用于日志记录和显示）
// 优先返回 URLAnthropic,因为它通常是主URL
// 其次返回 URLOpenAI，最后返回 URLGemini
func (e *Endpoint) GetURL() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// 优先返回主URL(通常是Anthropic)
	if e.URLAnthropic != "" {
		return e.URLAnthropic
	}
	if e.URLOpenAI != "" {
		return e.URLOpenAI
	}
	return e.URLGemini
}

// GetURLForFormat 根据请求格式返回对应的URL
// 用于日志记录实际使用的URL
func (e *Endpoint) GetURLForFormat(requestFormat string) string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if requestFormat == "openai" && e.URLOpenAI != "" {
		return e.URLOpenAI
	}
	if requestFormat == "anthropic" && e.URLAnthropic != "" {
		return e.URLAnthropic
	}
	if requestFormat == "gemini" && e.URLGemini != "" {
		return e.URLGemini
	}

	// 回退到默认逻辑
	if e.URLAnthropic != "" {
		return e.URLAnthropic
	}
	if e.URLOpenAI != "" {
		return e.URLOpenAI
	}
	return e.URLGemini
}

// SetDynamicSorter 设置动态排序器引用
func (e *Endpoint) SetDynamicSorter(sorter *utils.DynamicEndpointSorter) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.dynamicSorter = sorter
}

// triggerDynamicSortUpdate 触发动态排序更新
func (e *Endpoint) triggerDynamicSortUpdate() {
	e.mutex.RLock()
	sorter := e.dynamicSorter
	e.mutex.RUnlock()

	if sorter != nil {
		sorter.ForceUpdate()
	}
}

// IsOpenAIOnly 判断端点是否为 OpenAI-only 模式
// 规则：仅配置了 url_openai，没有配置 url_anthropic
func (e *Endpoint) IsOpenAIOnly() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.URLOpenAI != "" && e.URLAnthropic == ""
}

// IsAnthropicOnly 判断端点是否为 Anthropic-only 模式
// 规则：仅配置了 url_anthropic，没有配置 url_openai
func (e *Endpoint) IsAnthropicOnly() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.URLAnthropic != "" && e.URLOpenAI == ""
}

// 动态端点接口实现
func (e *Endpoint) GetName() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.Name
}

func (e *Endpoint) GetLastResponseTime() time.Duration {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if e.RequestHistory == nil {
		return 0
	}

	return e.RequestHistory.GetLastResponseTime()
}

func (e *Endpoint) GetSuccessRate() float64 {
	e.mutex.RLock()
	total := e.TotalRequests
	success := e.SuccessRequests
	e.mutex.RUnlock()

	if total == 0 {
		return 0
	}
	return float64(success) / float64(total) * 100
}

func (e *Endpoint) GetFailureCount() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.FailureCount
}

func (e *Endpoint) GetTotalRequests() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.TotalRequests
}

func (e *Endpoint) SetPriority(priority int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Priority = priority
}
