package proxyclient

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	commonutils "claude-code-codex-companion/internal/common/utils"
	"claude-code-codex-companion/internal/config"

	"golang.org/x/net/proxy"
)

// ProxyDialer 代理拨号器接口
type ProxyDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// CreateHTTPClient 创建支持代理的 HTTP 客户端
func CreateHTTPClient(proxyConfig *config.ProxyConfig, timeoutConfig config.ProxyTimeoutConfig) (*http.Client, error) {
	transport := &http.Transport{
		TLSHandshakeTimeout:   commonutils.ParseDuration(timeoutConfig.TLSHandshake, config.GetTimeoutDuration(config.Default.Timeouts.TLSHandshake, 10*time.Second)),
		ResponseHeaderTimeout: commonutils.ParseDuration(timeoutConfig.ResponseHeader, config.GetTimeoutDuration(config.Default.Timeouts.ResponseHeader, 60*time.Second)),
		IdleConnTimeout:       commonutils.ParseDuration(timeoutConfig.IdleConnection, config.GetTimeoutDuration(config.Default.Timeouts.IdleConnection, 90*time.Second)),
		DisableKeepAlives:     false,
		MaxIdleConns:          config.Default.HTTPClient.MaxIdleConns,
		MaxIdleConnsPerHost:   config.Default.HTTPClient.MaxIdlePerHost,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	// 如果配置了代理，设置代理
	if proxyConfig != nil {
		dialer, err := createProxyDialer(proxyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy dialer: %v", err)
		}

		transport.DialContext = dialer.DialContext
	}

	client := &http.Client{
		Transport: transport,
	}

	// 设置总体超时（如果配置了）
	if timeoutConfig.OverallRequest != "" {
		if duration := commonutils.ParseDuration(timeoutConfig.OverallRequest, 0); duration > 0 {
			client.Timeout = duration
		}
	}

	return client, nil
}

// createProxyDialer 根据代理配置创建代理拨号器
func createProxyDialer(proxyConfig *config.ProxyConfig) (ProxyDialer, error) {
	switch proxyConfig.Type {
	case "http":
		return createHTTPProxyDialer(proxyConfig)
	case "socks5":
		return createSOCKS5ProxyDialer(proxyConfig)
	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", proxyConfig.Type)
	}
}

// createHTTPProxyDialer 创建 HTTP 代理拨号器
func createHTTPProxyDialer(proxyConfig *config.ProxyConfig) (ProxyDialer, error) {
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   proxyConfig.Address,
	}

	// 如果有认证信息，添加到 URL
	if proxyConfig.Username != "" && proxyConfig.Password != "" {
		proxyURL.User = url.UserPassword(proxyConfig.Username, proxyConfig.Password)
	}

	return &httpProxyDialer{
		proxyURL: proxyURL,
		dialer: &net.Dialer{
			Timeout:   config.Default.ProxyDialer.Timeout,
			KeepAlive: config.Default.ProxyDialer.KeepAlive,
		},
	}, nil
}

// createSOCKS5ProxyDialer 创建 SOCKS5 代理拨号器
func createSOCKS5ProxyDialer(proxyConfig *config.ProxyConfig) (ProxyDialer, error) {
	var auth *proxy.Auth
	if proxyConfig.Username != "" && proxyConfig.Password != "" {
		auth = &proxy.Auth{
			User:     proxyConfig.Username,
			Password: proxyConfig.Password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyConfig.Address, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 proxy: %v", err)
	}

	// 如果返回的dialer支持DialContext，直接使用
	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		return &socks5ProxyDialer{contextDialer: contextDialer}, nil
	}

	// 否则包装为支持context的dialer
	return &socks5ProxyDialer{dialer: dialer}, nil
}

// httpProxyDialer HTTP 代理拨号器实现
type httpProxyDialer struct {
	proxyURL *url.URL
	dialer   *net.Dialer
}

func (h *httpProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// 连接到代理服务器
	proxyConn, err := h.dialer.DialContext(ctx, "tcp", h.proxyURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to HTTP proxy %s: %v", h.proxyURL.Host, err)
	}

	// 发送 CONNECT 请求
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}

	// 添加认证头（如果需要）
	if h.proxyURL.User != nil {
		connectReq.Header.Set("Proxy-Authorization", "Basic "+commonutils.BasicAuth(h.proxyURL.User.String()))
	}

	// 发送请求
	if err := connectReq.Write(proxyConn); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to send CONNECT request: %v", err)
	}

	// 读取响应
	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), connectReq)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		proxyConn.Close()
		return nil, fmt.Errorf("proxy returned non-200 status: %s", resp.Status)
	}

	return proxyConn, nil
}

// socks5ProxyDialer SOCKS5 代理拨号器实现
type socks5ProxyDialer struct {
	contextDialer proxy.ContextDialer
	dialer        proxy.Dialer
}

func (s *socks5ProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if s.contextDialer != nil {
		return s.contextDialer.DialContext(ctx, network, address)
	}

	// 对于不支持context的dialer，在goroutine中执行
	type result struct {
		conn net.Conn
		err  error
	}

	resultCh := make(chan result, 1)
	go func() {
		conn, err := s.dialer.Dial(network, address)
		resultCh <- result{conn: conn, err: err}
	}()

	select {
	case res := <-resultCh:
		return res.conn, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
