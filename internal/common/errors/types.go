package errors

import (
	"fmt"
	"time"
)

// ErrorType 定义错误类型
type ErrorType string

const (
	// 网络相关错误
	ErrorTypeNetwork     ErrorType = "network_error"
	ErrorTypeTimeout     ErrorType = "timeout_error"
	ErrorTypeConnection  ErrorType = "connection_error"
	
	// 配置相关错误
	ErrorTypeConfig      ErrorType = "config_error"
	ErrorTypeValidation  ErrorType = "validation_error"
	
	// 转换相关错误
	ErrorTypeConversion  ErrorType = "conversion_error"
	ErrorTypeParsing     ErrorType = "parsing_error"
	
	// 认证相关错误
	ErrorTypeAuth        ErrorType = "auth_error"
	ErrorTypeOAuth       ErrorType = "oauth_error"
	
	// 代理相关错误
	ErrorTypeProxy       ErrorType = "proxy_error"
	ErrorTypeEndpoint    ErrorType = "endpoint_error"
	
	// 系统相关错误
	ErrorTypeInternal    ErrorType = "internal_error"
	ErrorTypeExternal    ErrorType = "external_error"
)

// ProxyError 统一的代理错误类型
type ProxyError struct {
	Type      ErrorType              `json:"type"`
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Cause     error                  `json:"-"` // 原始错误，不序列化
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	
	// 错误追踪信息
	Component string `json:"component,omitempty"` // 发生错误的组件
	Operation string `json:"operation,omitempty"` // 发生错误的操作
}

// Error 实现error接口
func (e *ProxyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Type, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// Unwrap 支持错误链
func (e *ProxyError) Unwrap() error {
	return e.Cause
}

// Is 支持错误比较
func (e *ProxyError) Is(target error) bool {
	if t, ok := target.(*ProxyError); ok {
		return e.Type == t.Type && e.Code == t.Code
	}
	return false
}

// WithContext 添加上下文信息
func (e *ProxyError) WithContext(key string, value interface{}) *ProxyError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithComponent 添加组件信息
func (e *ProxyError) WithComponent(component string) *ProxyError {
	e.Component = component
	return e
}

// WithOperation 添加操作信息
func (e *ProxyError) WithOperation(operation string) *ProxyError {
	e.Operation = operation
	return e
}

// NewProxyError 创建新的代理错误
func NewProxyError(errorType ErrorType, code, message string) *ProxyError {
	return &ProxyError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// WrapError 包装现有错误
func WrapError(errorType ErrorType, code, message string, cause error) *ProxyError {
	return &ProxyError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// 预定义的常用错误

// Network Errors
func NewNetworkError(code, message string, cause error) *ProxyError {
	return WrapError(ErrorTypeNetwork, code, message, cause)
}

func NewTimeoutError(operation string, cause error) *ProxyError {
	return WrapError(ErrorTypeTimeout, "timeout", 
		fmt.Sprintf("Operation %s timed out", operation), cause).
		WithOperation(operation)
}

func NewConnectionError(target string, cause error) *ProxyError {
	return WrapError(ErrorTypeConnection, "connection_failed",
		fmt.Sprintf("Failed to connect to %s", target), cause).
		WithContext("target", target)
}

// Config Errors
func NewConfigError(field, message string) *ProxyError {
	return NewProxyError(ErrorTypeConfig, "invalid_config", message).
		WithContext("field", field)
}

func NewValidationError(field, message string) *ProxyError {
	return NewProxyError(ErrorTypeValidation, "validation_failed", message).
		WithContext("field", field)
}

// Conversion Errors
func NewConversionError(stage, message string, cause error) *ProxyError {
	return WrapError(ErrorTypeConversion, "conversion_failed", message, cause).
		WithContext("stage", stage)
}

func NewParsingError(dataType, message string, cause error) *ProxyError {
	return WrapError(ErrorTypeParsing, "parse_failed", message, cause).
		WithContext("data_type", dataType)
}

// Auth Errors
func NewAuthError(authType, message string) *ProxyError {
	return NewProxyError(ErrorTypeAuth, "auth_failed", message).
		WithContext("auth_type", authType)
}

func NewOAuthError(stage, message string, cause error) *ProxyError {
	return WrapError(ErrorTypeOAuth, "oauth_failed", message, cause).
		WithContext("stage", stage)
}

// Proxy Errors  
func NewProxyErrorWithCode(code, message string) *ProxyError {
	return NewProxyError(ErrorTypeProxy, code, message)
}

func NewEndpointError(endpoint, message string, cause error) *ProxyError {
	return WrapError(ErrorTypeEndpoint, "endpoint_failed", message, cause).
		WithContext("endpoint", endpoint)
}

// System Errors
func NewInternalError(component, message string, cause error) *ProxyError {
	return WrapError(ErrorTypeInternal, "internal_error", message, cause).
		WithComponent(component)
}

func NewExternalError(service, message string, cause error) *ProxyError {
	return WrapError(ErrorTypeExternal, "external_service_error", message, cause).
		WithContext("service", service)
}