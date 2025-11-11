package config

import (
	"time"
)

// DefaultValues 集中管理所有默认值
type DefaultValues struct {
	// 服务器配置默认值
	Server struct {
		Host string
		Port int
	}

	// 超时配置默认值 (字符串格式用于配置文件)
	Timeouts struct {
		TLSHandshake       string
		ResponseHeader     string
		IdleConnection     string
		HealthCheckTimeout string
		CheckInterval      string
		RecoveryThreshold  int
	}

	// HTTP客户端配置默认值（统一配置）
	HTTPClient struct {
		MaxIdleConns   int
		MaxIdlePerHost int
	}

	// 健康检查默认值
	HealthCheck struct {
		MaxTokens        int
		Temperature      float64
		StreamMode       bool
		Model            string
		UserID           string
		Headers          map[string]string
		FailureThreshold int
	}

	// 日志配置默认值
	Logging struct {
		Level            string
		LogRequestTypes  string
		LogRequestBody   string
		LogResponseBody  string
		LogDirectory     string
		BodyTruncateSize int
		ExcludePaths     []string
	}

	// 端点配置默认值
	Endpoint struct {
		Type     string
		Priority int
		Enabled  bool
	}

	// 验证配置默认值
	Validation struct {
		PythonJSONFix struct {
			Enabled      bool
			MaxAttempts  int
			DebugLogging bool
		}
	}

	// 标签配置默认值
	Tagging struct {
		PipelineTimeout string
	}

	// 拉黑配置默认值
	Blacklist struct {
		Enabled           bool
		AutoBlacklist     bool
		BusinessErrorSafe bool
		ConfigErrorSafe   bool
		ServerErrorSafe   bool
		SSEValidationSafe bool
	}

	// 数据库配置默认值
	Database struct {
		CacheSize       int
		MmapSize        int64
		BusyTimeout     int
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
		MaxRetries      int
	}

	// 分页默认值
	Pagination struct {
		DefaultPage  int
		DefaultLimit int
		MaxPages     int
	}

	// 代理拨号器默认值
	ProxyDialer struct {
		Timeout   time.Duration
		KeepAlive time.Duration
	}

	// （已移除）ToolCalling 全局默认：采用零配置 + 端点级自动学习/开关
}

// Default 全局默认值实例
var Default = DefaultValues{
	Server: struct {
		Host string
		Port int
	}{
		Host: "0.0.0.0",
		Port: 8080,
	},

	Timeouts: struct {
		TLSHandshake       string
		ResponseHeader     string
		IdleConnection     string
		HealthCheckTimeout string
		CheckInterval      string
		RecoveryThreshold  int
	}{
		TLSHandshake:       "10s",
		ResponseHeader:     "60s",
		IdleConnection:     "90s",
		HealthCheckTimeout: "60s",
		CheckInterval:      "30s",
		RecoveryThreshold:  1,
	},

	HTTPClient: struct {
		MaxIdleConns   int
		MaxIdlePerHost int
	}{
		MaxIdleConns:   50,
		MaxIdlePerHost: 10,
	},

	HealthCheck: struct {
		MaxTokens        int
		Temperature      float64
		StreamMode       bool
		Model            string
		UserID           string
		Headers          map[string]string
		FailureThreshold int
	}{
		MaxTokens:   512,
		Temperature: 0,
		StreamMode:  true,
		Model:       "claude-sonnet-4-20250514",
		UserID:      "user_test_account__session_test",
		Headers: map[string]string{
			"Accept":          "application/json",
			"Accept-Encoding": "gzip, deflate",
			"Accept-Language": "*",
			"Anthropic-Beta":  "fine-grained-tool-streaming-2025-05-14",
			"Anthropic-Dangerous-Direct-Browser-Access": "true",
			"Anthropic-Version":                         "2023-06-01",
			"Connection":                                "keep-alive",
			"Content-Type":                              "application/json",
			"Sec-Fetch-Mode":                            "cors",
			"User-Agent":                                "claude-cli/1.0.56 (external, cli)",
			"X-App":                                     "cli",
			"X-Stainless-Arch":                          "x64",
			"X-Stainless-Helper-Method":                 "stream",
			"X-Stainless-Lang":                          "js",
			"X-Stainless-Os":                            "Windows",
			"X-Stainless-Package-Version":               "0.55.1",
			"X-Stainless-Retry-Count":                   "0",
			"X-Stainless-Runtime":                       "node",
			"X-Stainless-Runtime-Version":               "v22.17.0",
			"X-Stainless-Timeout":                       "600",
		},
		FailureThreshold: 3,
	},

	Logging: struct {
		Level            string
		LogRequestTypes  string
		LogRequestBody   string
		LogResponseBody  string
		LogDirectory     string
		BodyTruncateSize int
		ExcludePaths     []string
	}{
		Level:            "info",
		LogRequestTypes:  "all",
		LogRequestBody:   "none",
		LogResponseBody:  "none",
		LogDirectory:     "./logs",
		BodyTruncateSize: 1000,
		ExcludePaths:     []string{"/v1/messages/count_tokens"}, // 默认过滤token计数请求
	},

	Endpoint: struct {
		Type     string
		Priority int
		Enabled  bool
	}{
		Type:     "anthropic",
		Priority: 1,
		Enabled:  true,
	},

	Validation: struct {
		PythonJSONFix struct {
			Enabled      bool
			MaxAttempts  int
			DebugLogging bool
		}
	}{
		PythonJSONFix: struct {
			Enabled      bool
			MaxAttempts  int
			DebugLogging bool
		}{
			Enabled:      true,
			MaxAttempts:  3,
			DebugLogging: false,
		},
	},

	Tagging: struct {
		PipelineTimeout string
	}{
		PipelineTimeout: "5s",
	},

	Blacklist: struct {
		Enabled           bool
		AutoBlacklist     bool
		BusinessErrorSafe bool
		ConfigErrorSafe   bool
		ServerErrorSafe   bool
		SSEValidationSafe bool
	}{
		Enabled:           true,  // 默认启用拉黑功能
		AutoBlacklist:     true,  // 默认启用自动拉黑
		BusinessErrorSafe: true,  // 默认业务错误不触发拉黑
		ConfigErrorSafe:   false, // 默认配置错误会触发拉黑
		ServerErrorSafe:   false, // 默认服务器错误会触发拉黑
		SSEValidationSafe: false, // 默认SSE验证错误会触发拉黑
	},

	Database: struct {
		CacheSize       int
		MmapSize        int64
		BusyTimeout     int
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
		MaxRetries      int
	}{
		CacheSize:       10000,
		MmapSize:        268435456, // 256MB
		BusyTimeout:     5000,      // 5秒
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		MaxRetries:      3,
	},

	Pagination: struct {
		DefaultPage  int
		DefaultLimit int
		MaxPages     int
	}{
		DefaultPage:  1,
		DefaultLimit: 100,
		MaxPages:     1,
	},

	ProxyDialer: struct {
		Timeout   time.Duration
		KeepAlive time.Duration
	}{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	},

	// ToolCalling 默认已移除
}

// GetTimeoutDuration 获取超时配置的Duration值，如果配置为空则返回默认值
func GetTimeoutDuration(configValue string, defaultValue time.Duration) time.Duration {
	if configValue == "" {
		return defaultValue
	}
	if d, err := time.ParseDuration(configValue); err == nil {
		return d
	}
	return defaultValue
}

// GetStringWithDefault 获取字符串配置值，如果为空则返回默认值
func GetStringWithDefault(configValue, defaultValue string) string {
	if configValue == "" {
		return defaultValue
	}
	return configValue
}

// GetIntWithDefault 获取整数配置值，如果为0则返回默认值
func GetIntWithDefault(configValue, defaultValue int) int {
	if configValue == 0 {
		return defaultValue
	}
	return configValue
}

// GetBoolWithDefault 获取布尔配置值，提供显式默认值
func GetBoolWithDefault(configValue bool, hasValue bool, defaultValue bool) bool {
	if !hasValue {
		return defaultValue
	}
	return configValue
}
