package utils

import (
	"net/http"
	
	"claude-code-codex-companion/internal/common/httpclient"
)

// 兼容性函数，委托给新的httpclient包

// GetProxyClient returns the shared HTTP client for proxy requests
func GetProxyClient() *http.Client {
	return httpclient.GetProxyClient()
}

// GetHealthClient returns the shared HTTP client for health checks
func GetHealthClient() *http.Client {
	return httpclient.GetHealthClient()
}

