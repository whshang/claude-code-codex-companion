package httpclient

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	commonutils "claude-code-codex-companion/internal/common/utils"
	"claude-code-codex-companion/internal/config"

	"golang.org/x/net/proxy"
)

// ProxyDialer 代理拨号器接口
type ProxyDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// createProxyDialer 根据代理配置创建代理拨号器
func (f *Factory) createProxyDialer(proxyConfig *config.ProxyConfig) (ProxyDialer, error) {
	switch proxyConfig.Type {
	case "http":
		return f.createHTTPProxyDialer(proxyConfig)
	case "socks5":
		return f.createSOCKS5ProxyDialer(proxyConfig)
	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", proxyConfig.Type)
	}
}

// createHTTPProxyDialer 创建HTTP代理拨号器
func (f *Factory) createHTTPProxyDialer(proxyConfig *config.ProxyConfig) (ProxyDialer, error) {
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   proxyConfig.Address,
	}

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

// createSOCKS5ProxyDialer 创建SOCKS5代理拨号器
func (f *Factory) createSOCKS5ProxyDialer(proxyConfig *config.ProxyConfig) (ProxyDialer, error) {
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

	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		return &socks5ProxyDialer{contextDialer: contextDialer}, nil
	}

	return &socks5ProxyDialer{dialer: dialer}, nil
}

// httpProxyDialer HTTP代理拨号器实现
type httpProxyDialer struct {
	proxyURL *url.URL
	dialer   *net.Dialer
}

func (h *httpProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	proxyConn, err := h.dialer.DialContext(ctx, "tcp", h.proxyURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to HTTP proxy %s: %v", h.proxyURL.Host, err)
	}

	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}

	if h.proxyURL.User != nil {
		connectReq.Header.Set("Proxy-Authorization", "Basic "+commonutils.BasicAuth(h.proxyURL.User.String()))
	}

	if err := connectReq.Write(proxyConn); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to send CONNECT request: %v", err)
	}

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

// socks5ProxyDialer SOCKS5代理拨号器实现
type socks5ProxyDialer struct {
	contextDialer proxy.ContextDialer
	dialer        proxy.Dialer
}

func (s *socks5ProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if s.contextDialer != nil {
		return s.contextDialer.DialContext(ctx, network, address)
	}

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
