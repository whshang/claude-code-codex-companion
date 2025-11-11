package httpclient

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"claude-code-codex-companion/internal/config"
)

// ClientType 定义客户端类型
type ClientType string

const (
	ClientTypeProxy       ClientType = "proxy"
	ClientTypeHealth      ClientType = "health"
	ClientTypeEndpoint    ClientType = "endpoint"
)

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	TLSHandshake     time.Duration
	ResponseHeader   time.Duration
	IdleConnection   time.Duration
	OverallRequest   time.Duration // 0表示无超时
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Type            ClientType
	Timeouts        TimeoutConfig
	ProxyConfig     *config.ProxyConfig
	MaxIdleConns    int
	MaxIdlePerHost  int
	DisableKeepAlive bool
	InsecureSkipVerify bool
	// 新增内存和连接优化配置
	MaxConnsPerHost int // 最大连接数限制
	ForceAttemptHTTP2 bool // 强制使用HTTP/2
	WriteBufferSize int // 写缓冲区大小
	ReadBufferSize  int // 读缓冲区大小
}

// Factory HTTP客户端工厂
type Factory struct {
	defaultConfigs map[ClientType]ClientConfig
}

// NewFactory 创建新的HTTP客户端工厂
func NewFactory() *Factory {
	return &Factory{
		defaultConfigs: map[ClientType]ClientConfig{
			ClientTypeProxy: {
				Type: ClientTypeProxy,
				Timeouts: TimeoutConfig{
					TLSHandshake:   config.GetTimeoutDuration(config.Default.Timeouts.TLSHandshake, 10*time.Second),
					ResponseHeader: config.GetTimeoutDuration(config.Default.Timeouts.ResponseHeader, 60*time.Second),
					IdleConnection: config.GetTimeoutDuration(config.Default.Timeouts.IdleConnection, 90*time.Second),
					OverallRequest: 0, // 流式请求无超时
				},
				MaxIdleConns:   config.Default.HTTPClient.MaxIdleConns,
				MaxIdlePerHost: config.Default.HTTPClient.MaxIdlePerHost,
				// 新增优化配置
				MaxConnsPerHost: 100, // 限制每个主机的最大连接数
				ForceAttemptHTTP2: true,
				WriteBufferSize: 32 * 1024, // 32KB写缓冲区
				ReadBufferSize:  32 * 1024, // 32KB读缓冲区
			},
			ClientTypeHealth: {
				Type: ClientTypeHealth,
				Timeouts: TimeoutConfig{
					TLSHandshake:   config.GetTimeoutDuration(config.Default.Timeouts.TLSHandshake, 10*time.Second),
					ResponseHeader: config.GetTimeoutDuration(config.Default.Timeouts.ResponseHeader, 60*time.Second),
					IdleConnection: config.GetTimeoutDuration(config.Default.Timeouts.IdleConnection, 90*time.Second),
					OverallRequest: config.GetTimeoutDuration(config.Default.Timeouts.HealthCheckTimeout, 30*time.Second),
				},
				MaxIdleConns:   config.Default.HTTPClient.MaxIdleConns,
				MaxIdlePerHost: config.Default.HTTPClient.MaxIdlePerHost,
				// 健康检查使用较小的连接池
				MaxConnsPerHost: 10,
				WriteBufferSize: 8 * 1024,
				ReadBufferSize:  8 * 1024,
			},
			ClientTypeEndpoint: {
				Type: ClientTypeEndpoint,
				Timeouts: TimeoutConfig{
					TLSHandshake:   config.GetTimeoutDuration(config.Default.Timeouts.TLSHandshake, 10*time.Second),
					ResponseHeader: config.GetTimeoutDuration(config.Default.Timeouts.ResponseHeader, 60*time.Second),
					IdleConnection: config.GetTimeoutDuration(config.Default.Timeouts.IdleConnection, 90*time.Second),
					OverallRequest: 0,
				},
				MaxIdleConns:   config.Default.HTTPClient.MaxIdleConns,
				MaxIdlePerHost: config.Default.HTTPClient.MaxIdlePerHost,
				// 端点客户端使用最大连接池
				MaxConnsPerHost: 200,
				ForceAttemptHTTP2: true,
				WriteBufferSize: 64 * 1024, // 64KB写缓冲区
				ReadBufferSize:  64 * 1024, // 64KB读缓冲区
			},
		},
	}
}

// CreateClient 根据配置创建HTTP客户端
func (f *Factory) CreateClient(config ClientConfig) (*http.Client, error) {
	// 合并默认配置
	if defaultConfig, exists := f.defaultConfigs[config.Type]; exists {
		config = f.mergeConfigs(defaultConfig, config)
	}

	transport := &http.Transport{
		TLSHandshakeTimeout:   config.Timeouts.TLSHandshake,
		ResponseHeaderTimeout: config.Timeouts.ResponseHeader,
		IdleConnTimeout:       config.Timeouts.IdleConnection,
		DisableKeepAlives:     config.DisableKeepAlive,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdlePerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost, // 新增连接限制
		ForceAttemptHTTP2:     config.ForceAttemptHTTP2, // 强制使用HTTP/2
		WriteBufferSize:       config.WriteBufferSize, // 优化缓冲区
		ReadBufferSize:        config.ReadBufferSize,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
		DisableCompression: false, // 确保启用压缩
	}

	// 如果配置了代理，设置代理拨号器
	if config.ProxyConfig != nil {
		dialer, err := f.createProxyDialer(config.ProxyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy dialer: %v", err)
		}
		transport.DialContext = dialer.DialContext
	}

	client := &http.Client{
		Transport: &gzipRoundTripper{transport: transport},
		Timeout:   config.Timeouts.OverallRequest,
	}

	return client, nil
}

// CreateProxyClient 创建代理客户端（兼容性方法）
func (f *Factory) CreateProxyClient(timeouts TimeoutConfig) *http.Client {
	config := ClientConfig{
		Type:     ClientTypeProxy,
		Timeouts: timeouts,
	}
	client, _ := f.CreateClient(config)
	return client
}

// CreateHealthClient 创建健康检查客户端（兼容性方法）
func (f *Factory) CreateHealthClient(timeouts TimeoutConfig) *http.Client {
	config := ClientConfig{
		Type:     ClientTypeHealth,
		Timeouts: timeouts,
	}
	client, _ := f.CreateClient(config)
	return client
}

// CreateEndpointClient 创建端点客户端
func (f *Factory) CreateEndpointClient(proxyConfig *config.ProxyConfig, timeouts TimeoutConfig) (*http.Client, error) {
	config := ClientConfig{
		Type:        ClientTypeEndpoint,
		Timeouts:    timeouts,
		ProxyConfig: proxyConfig,
	}
	return f.CreateClient(config)
}

// mergeConfigs 合并配置，优先使用传入的配置
func (f *Factory) mergeConfigs(defaultConfig, userConfig ClientConfig) ClientConfig {
	result := defaultConfig
	
	// 只覆盖非零值
	if userConfig.Timeouts.TLSHandshake != 0 {
		result.Timeouts.TLSHandshake = userConfig.Timeouts.TLSHandshake
	}
	if userConfig.Timeouts.ResponseHeader != 0 {
		result.Timeouts.ResponseHeader = userConfig.Timeouts.ResponseHeader
	}
	if userConfig.Timeouts.IdleConnection != 0 {
		result.Timeouts.IdleConnection = userConfig.Timeouts.IdleConnection
	}
	if userConfig.Timeouts.OverallRequest != 0 {
		result.Timeouts.OverallRequest = userConfig.Timeouts.OverallRequest
	}
	if userConfig.MaxIdleConns != 0 {
		result.MaxIdleConns = userConfig.MaxIdleConns
	}
	if userConfig.MaxIdlePerHost != 0 {
		result.MaxIdlePerHost = userConfig.MaxIdlePerHost
	}
	if userConfig.ProxyConfig != nil {
		result.ProxyConfig = userConfig.ProxyConfig
	}
	
	result.DisableKeepAlive = userConfig.DisableKeepAlive
	result.InsecureSkipVerify = userConfig.InsecureSkipVerify
	
	return result
}

// ParseTimeoutWithDefault 解析超时字符串，失败时返回默认值
func ParseTimeoutWithDefault(value, fieldName string, defaultDuration time.Duration) (time.Duration, error) {
	if value == "" {
		return defaultDuration, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s timeout: %v", fieldName, err)
	}
	return d, nil
}

// gzipRoundTripper 自定义RoundTripper，用于处理gzip压缩响应
type gzipRoundTripper struct {
	transport http.RoundTripper
}

// RoundTrip 实现RoundTripper接口，自动处理gzip解压缩
func (grt *gzipRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// 确保请求包含Accept-Encoding: gzip头部
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	resp, err := grt.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 检查响应是否使用gzip压缩
	if resp.Header.Get("Content-Encoding") == "gzip" {
		// 包装response body以自动解压gzip
		resp.Body = &gzipReadCloser{source: resp.Body}
	}

	return resp, nil
}

// gzipReadCloser 包装Reader以提供gzip解压缩功能
type gzipReadCloser struct {
	source io.ReadCloser
	gzipReader *gzip.Reader
}

// Read 实现io.Reader接口
func (grc *gzipReadCloser) Read(p []byte) (n int, err error) {
	if grc.gzipReader == nil {
		grc.gzipReader, err = gzip.NewReader(grc.source)
		if err != nil {
			return 0, err
		}
	}
	return grc.gzipReader.Read(p)
}

// Close 实现io.Closer接口
func (grc *gzipReadCloser) Close() error {
	if grc.gzipReader != nil {
		grc.gzipReader.Close()
	}
	return grc.source.Close()
}