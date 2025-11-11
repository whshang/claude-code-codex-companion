package validator

import "fmt"

// ValidationErrorType 定义验证错误的类型
type ValidationErrorType int

const (
	// NetworkError 网络层错误（连接失败、超时等）- 应触发端点黑名单
	NetworkError ValidationErrorType = iota

	// BusinessError 业务逻辑错误（API返回错误响应）- 不应触发端点黑名单
	BusinessError

	// FormatError 格式错误（响应格式不符合预期）- 应触发端点黑名单
	FormatError

	// ClientValidationError 客户端验证错误（端点拒绝非官方客户端）- 端点正常，但拒绝测试请求
	ClientValidationError
)

// ValidationError 增强的验证错误，包含错误类型信息
type ValidationError struct {
	Type    ValidationErrorType
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// IsBusinessError 检查错误是否为业务逻辑错误
func (e *ValidationError) IsBusinessError() bool {
	return e.Type == BusinessError
}

// IsClientValidationError 检查错误是否为客户端验证错误
func (e *ValidationError) IsClientValidationError() bool {
	return e.Type == ClientValidationError
}

// IsSafeError 检查错误是否为"安全"错误（不应触发端点拉黑）
func (e *ValidationError) IsSafeError() bool {
	return e.Type == BusinessError || e.Type == ClientValidationError
}

// NewNetworkError 创建网络错误
func NewNetworkError(message string, cause error) *ValidationError {
	return &ValidationError{
		Type:    NetworkError,
		Message: message,
		Cause:   cause,
	}
}

// NewBusinessError 创建业务错误
func NewBusinessError(message string, cause error) *ValidationError {
	return &ValidationError{
		Type:    BusinessError,
		Message: message,
		Cause:   cause,
	}
}

// NewFormatError 创建格式错误
func NewFormatError(message string, cause error) *ValidationError {
	return &ValidationError{
		Type:    FormatError,
		Message: message,
		Cause:   cause,
	}
}

// NewClientValidationError 创建客户端验证错误
func NewClientValidationError(message string, cause error) *ValidationError {
	return &ValidationError{
		Type:    ClientValidationError,
		Message: message,
		Cause:   cause,
	}
}

// IsBusinessError 辅助函数，检查error是否为业务错误
func IsBusinessError(err error) bool {
	if verr, ok := err.(*ValidationError); ok {
		return verr.IsBusinessError()
	}
	return false
}

// IsClientValidationError 辅助函数，检查error是否为客户端验证错误
func IsClientValidationError(err error) bool {
	if verr, ok := err.(*ValidationError); ok {
		return verr.IsClientValidationError()
	}
	return false
}

// IsSafeError 辅助函数，检查error是否为安全错误（不应触发拉黑）
func IsSafeError(err error) bool {
	if verr, ok := err.(*ValidationError); ok {
		return verr.IsSafeError()
	}
	return false
}
