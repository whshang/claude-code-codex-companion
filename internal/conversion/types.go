package conversion

import "net/http"

// RequestAdapter defines the interface for converting inbound HTTP requests
// between provider formats. This is used by the proxy layer when it needs to
// translate a request before forwarding it to an upstream.
type RequestAdapter interface {
	ConvertRequest(req *http.Request) (*http.Request, error)
}

// EndpointInfo captures endpoint specific hints that influence request
// translation behaviour (for example the preferred max tokens field).
type EndpointInfo struct {
	Type               string
	MaxTokensFieldName string
}

// Converter describes the high level request/response conversion helpers used
// by the compatibility layer.
type Converter interface {
	ConvertRequest(anthropicReq []byte, endpointInfo *EndpointInfo) ([]byte, *ConversionContext, error)
	ConvertResponse(openaiResp []byte, ctx *ConversionContext, isStreaming bool) ([]byte, error)
	ShouldConvert(endpointType string) bool
}

// ConversionError represents a structured error generated while translating
// between formats. It wraps the underlying cause (when available) so callers
// can inspect or unwrap it.
type ConversionError struct {
	Type    string
	Message string
	Err     error
}

func (e *ConversionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ConversionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewConversionError creates a new ConversionError instance with optional
// underlying error cause.
func NewConversionError(errorType, message string, err error) *ConversionError {
	return &ConversionError{
		Type:    errorType,
		Message: message,
		Err:     err,
	}
}
