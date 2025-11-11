package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPError HTTP错误响应结构
type HTTPError struct {
	Status  int                    `json:"status"`
	Error   string                 `json:"error"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HTTPErrorHandler HTTP错误处理器
type HTTPErrorHandler struct {
	logger Logger // 可以接受任何实现了Logger接口的类型
}

// Logger 日志接口
type Logger interface {
	Error(message string, fields map[string]interface{})
	Warn(message string, fields map[string]interface{})
}

// NewHTTPErrorHandler 创建HTTP错误处理器
func NewHTTPErrorHandler(logger Logger) *HTTPErrorHandler {
	return &HTTPErrorHandler{
		logger: logger,
	}
}

// HandleError 处理错误并返回HTTP响应
func (h *HTTPErrorHandler) HandleError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	
	proxyErr := h.convertToProxyError(err)
	httpErr := h.convertToHTTPError(proxyErr)
	
	// 记录错误日志
	h.logError(proxyErr)
	
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErr.Status)
	
	// 写入响应
	if err := json.NewEncoder(w).Encode(httpErr); err != nil {
		// 如果JSON编码失败，回退到纯文本
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Internal error occurred")
	}
}

// convertToProxyError 转换为ProxyError
func (h *HTTPErrorHandler) convertToProxyError(err error) *ProxyError {
	if proxyErr, ok := IsProxyError(err); ok {
		return proxyErr
	}
	return ClassifyError(err)
}

// convertToHTTPError 转换为HTTP错误
func (h *HTTPErrorHandler) convertToHTTPError(proxyErr *ProxyError) *HTTPError {
	status := h.getHTTPStatus(proxyErr)
	
	httpErr := &HTTPError{
		Status:  status,
		Error:   http.StatusText(status),
		Message: proxyErr.Message,
	}
	
	// 添加详细信息（开发模式下）
	if h.shouldIncludeDetails(proxyErr) {
		httpErr.Details = map[string]interface{}{
			"type":      proxyErr.Type,
			"code":      proxyErr.Code,
			"timestamp": proxyErr.Timestamp,
		}
		
		if proxyErr.Component != "" {
			httpErr.Details["component"] = proxyErr.Component
		}
		
		if proxyErr.Operation != "" {
			httpErr.Details["operation"] = proxyErr.Operation
		}
		
		if proxyErr.Context != nil {
			httpErr.Details["context"] = proxyErr.Context
		}
	}
	
	return httpErr
}

// getHTTPStatus 根据错误类型获取HTTP状态码
func (h *HTTPErrorHandler) getHTTPStatus(proxyErr *ProxyError) int {
	switch proxyErr.Type {
	case ErrorTypeAuth, ErrorTypeOAuth:
		return http.StatusUnauthorized
		
	case ErrorTypeValidation, ErrorTypeConfig:
		return http.StatusBadRequest
		
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
		
	case ErrorTypeConnection, ErrorTypeNetwork:
		return http.StatusBadGateway
		
	case ErrorTypeEndpoint:
		if proxyErr.Code == "not_found" {
			return http.StatusNotFound
		}
		return http.StatusServiceUnavailable
		
	case ErrorTypeExternal:
		return http.StatusBadGateway
		
	case ErrorTypeConversion, ErrorTypeParsing:
		return http.StatusUnprocessableEntity
		
	case ErrorTypeProxy:
		switch proxyErr.Code {
		case "rate_limited":
			return http.StatusTooManyRequests
		case "forbidden":
			return http.StatusForbidden
		case "not_found":
			return http.StatusNotFound
		default:
			return http.StatusInternalServerError
		}
		
	default:
		return http.StatusInternalServerError
	}
}

// shouldIncludeDetails 判断是否应该包含详细错误信息
func (h *HTTPErrorHandler) shouldIncludeDetails(proxyErr *ProxyError) bool {
	// 在生产环境中，只对某些类型的错误返回详细信息
	switch proxyErr.Type {
	case ErrorTypeValidation, ErrorTypeConfig:
		return true
	default:
		// 可以通过环境变量或配置来控制
		return false
	}
}

// logError 记录错误日志
func (h *HTTPErrorHandler) logError(proxyErr *ProxyError) {
	if h.logger == nil {
		return
	}
	
	fields := map[string]interface{}{
		"error_type":  proxyErr.Type,
		"error_code":  proxyErr.Code,
		"timestamp":   proxyErr.Timestamp,
	}
	
	if proxyErr.Component != "" {
		fields["component"] = proxyErr.Component
	}
	
	if proxyErr.Operation != "" {
		fields["operation"] = proxyErr.Operation
	}
	
	if proxyErr.Context != nil {
		for k, v := range proxyErr.Context {
			fields["context_"+k] = v
		}
	}
	
	if proxyErr.Cause != nil {
		fields["cause"] = proxyErr.Cause.Error()
	}
	
	// 根据错误严重程度选择日志级别
	switch proxyErr.Type {
	case ErrorTypeValidation, ErrorTypeConfig, ErrorTypeAuth:
		h.logger.Warn(proxyErr.Message, fields)
	default:
		h.logger.Error(proxyErr.Message, fields)
	}
}

// WriteJSONError 写入JSON格式的错误响应
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	httpErr := &HTTPError{
		Status:  status,
		Error:   http.StatusText(status),
		Message: message,
	}
	
	if err := json.NewEncoder(w).Encode(httpErr); err != nil {
		// 如果JSON编码失败，回退到纯文本
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Error: %s", message)
	}
}