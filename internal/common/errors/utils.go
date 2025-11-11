package errors

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"
)

// IsProxyError 检查是否为代理错误
func IsProxyError(err error) (*ProxyError, bool) {
	var proxyErr *ProxyError
	if errors.As(err, &proxyErr) {
		return proxyErr, true
	}
	return nil, false
}

// HasErrorType 检查错误链中是否包含指定类型的错误
func HasErrorType(err error, errorType ErrorType) bool {
	proxyErr, ok := IsProxyError(err)
	if !ok {
		return false
	}
	return proxyErr.Type == errorType
}

// HasErrorCode 检查错误链中是否包含指定代码的错误
func HasErrorCode(err error, code string) bool {
	proxyErr, ok := IsProxyError(err)
	if !ok {
		return false
	}
	return proxyErr.Code == code
}

// WrapIfNotProxyError 如果不是ProxyError则包装
func WrapIfNotProxyError(err error, errorType ErrorType, code, message string) error {
	if err == nil {
		return nil
	}
	
	if _, ok := IsProxyError(err); ok {
		return err
	}
	
	return WrapError(errorType, code, message, err)
}

// ClassifyError 自动分类常见错误类型
func ClassifyError(err error) *ProxyError {
	if err == nil {
		return nil
	}
	
	// 如果已经是ProxyError，直接返回
	if proxyErr, ok := IsProxyError(err); ok {
		return proxyErr
	}
	
	errStr := err.Error()
	errStrLower := strings.ToLower(errStr)
	
	// 网络相关错误
	if isNetworkError(err, errStrLower) {
		return classifyNetworkError(err, errStr)
	}
	
	// 解析相关错误
	if isParsingError(errStrLower) {
		return NewParsingError("json", "Failed to parse data", err)
	}
	
	// URL相关错误
	if _, ok := err.(*url.Error); ok {
		return NewNetworkError("url_error", "Invalid URL or URL request failed", err)
	}
	
	// 默认为内部错误
	return NewInternalError("unknown", "Unclassified error occurred", err)
}

// isNetworkError 判断是否为网络错误
func isNetworkError(err error, errStrLower string) bool {
	// 检查具体的网络错误类型
	if _, ok := err.(net.Error); ok {
		return true
	}
	
	// 检查系统调用错误
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	
	// 检查错误消息中的关键词
	networkKeywords := []string{
		"connection", "network", "timeout", "refused", 
		"reset", "broken pipe", "no route to host",
		"host unreachable", "dns",
	}
	
	for _, keyword := range networkKeywords {
		if strings.Contains(errStrLower, keyword) {
			return true
		}
	}
	
	return false
}

// classifyNetworkError 细分网络错误类型
func classifyNetworkError(err error, errStr string) *ProxyError {
	errStrLower := strings.ToLower(errStr)
	
	// 超时错误
	if strings.Contains(errStrLower, "timeout") {
		return NewTimeoutError("network_request", err)
	}
	
	// 连接错误
	if strings.Contains(errStrLower, "connection refused") ||
		strings.Contains(errStrLower, "connect: connection refused") {
		return NewConnectionError("target_server", err)
	}
	
	if strings.Contains(errStrLower, "connection reset") ||
		strings.Contains(errStrLower, "broken pipe") {
		return NewConnectionError("target_server", err).
			WithContext("reason", "connection_lost")
	}
	
	// DNS错误
	if strings.Contains(errStrLower, "no such host") ||
		strings.Contains(errStrLower, "dns") {
		return NewNetworkError("dns_error", "DNS resolution failed", err)
	}
	
	// 路由错误
	if strings.Contains(errStrLower, "no route to host") ||
		strings.Contains(errStrLower, "host unreachable") {
		return NewNetworkError("routing_error", "Host unreachable", err)
	}
	
	// 通用网络错误
	return NewNetworkError("generic", "Network operation failed", err)
}

// isParsingError 判断是否为解析错误
func isParsingError(errStrLower string) bool {
	parsingKeywords := []string{
		"unmarshal", "marshal", "json", "parse", "decode", "encode",
		"invalid character", "unexpected end",
	}
	
	for _, keyword := range parsingKeywords {
		if strings.Contains(errStrLower, keyword) {
			return true
		}
	}
	
	return false
}

// ErrorChain 获取错误链
func ErrorChain(err error) []string {
	if err == nil {
		return nil
	}
	
	var chain []string
	current := err
	
	for current != nil {
		chain = append(chain, current.Error())
		current = errors.Unwrap(current)
	}
	
	return chain
}

// ErrorSummary 获取错误摘要
func ErrorSummary(err error) string {
	if err == nil {
		return "no error"
	}
	
	if proxyErr, ok := IsProxyError(err); ok {
		summary := fmt.Sprintf("%s (%s)", proxyErr.Message, proxyErr.Code)
		if proxyErr.Component != "" {
			summary += fmt.Sprintf(" in %s", proxyErr.Component)
		}
		if proxyErr.Operation != "" {
			summary += fmt.Sprintf(" during %s", proxyErr.Operation)
		}
		return summary
	}
	
	return err.Error()
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	proxyErr, ok := IsProxyError(err)
	if !ok {
		// 对于非ProxyError，使用启发式规则
		return isRetryableHeuristic(err)
	}
	
	switch proxyErr.Type {
	case ErrorTypeTimeout, ErrorTypeConnection, ErrorTypeNetwork:
		return true
	case ErrorTypeProxy:
		// 某些代理错误可能可重试
		return proxyErr.Code == "temporary_failure"
	case ErrorTypeExternal:
		// 外部服务错误通常可重试
		return true
	default:
		return false
	}
}

// isRetryableHeuristic 启发式判断是否可重试
func isRetryableHeuristic(err error) bool {
	errStr := strings.ToLower(err.Error())
	
	retryableKeywords := []string{
		"timeout", "connection refused", "connection reset",
		"temporary failure", "service unavailable",
		"too many requests", "server error",
	}
	
	for _, keyword := range retryableKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}
	
	return false
}