package health

import (
	"net/http"
	"strings"
	"sync"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/utils"
)

type RequestInfo struct {
	Model    string            `json:"model"`
	UserID   string            `json:"user_id"`
	Headers  map[string]string `json:"headers"`
	Extracted bool             `json:"extracted"`
}

type RequestExtractor struct {
	mutex       sync.RWMutex
	requestInfo *RequestInfo
}

func NewRequestExtractor() *RequestExtractor {
	return &RequestExtractor{
		requestInfo: &RequestInfo{
			Model:     config.Default.HealthCheck.Model,
			UserID:    config.Default.HealthCheck.UserID,
			Headers:   config.Default.HealthCheck.Headers,
			Extracted: false, // false表示使用默认值，true表示已从实际请求中提取
		},
	}
}

func (re *RequestExtractor) ExtractFromRequest(body []byte, headers http.Header) bool {
	re.mutex.Lock()
	defer re.mutex.Unlock()

	// 总是尝试从请求中提取信息来覆盖默认值
	extracted := false

	// 提取模型信息
	model := re.extractModel(body)
	if model != "" && strings.HasPrefix(model, "claude-3-5") {
		re.requestInfo.Model = model
		extracted = true
	}

	// 提取用户ID
	userID := re.extractUserID(body)
	if userID != "" {
		re.requestInfo.UserID = userID
		extracted = true
	}

	// 提取请求头
	requestHeaders := re.extractHeaders(headers)
	if len(requestHeaders) > 0 {
		// 合并请求头，新的头部会覆盖旧的
		for k, v := range requestHeaders {
			re.requestInfo.Headers[k] = v
		}
		extracted = true
	}

	// 如果成功提取了任何信息，标记为已提取
	if extracted {
		re.requestInfo.Extracted = true
	}

	return extracted
}

func (re *RequestExtractor) extractModel(body []byte) string {
	model, _ := utils.ExtractStringField(body, "model")
	return model
}

func (re *RequestExtractor) extractUserID(body []byte) string {
	userID, _ := utils.ExtractNestedStringField(body, []string{"metadata", "user_id"})
	return userID
}

func (re *RequestExtractor) extractHeaders(headers http.Header) map[string]string {
	return utils.ExtractRequestHeaders(headers)
}

func (re *RequestExtractor) GetRequestInfo() *RequestInfo {
	re.mutex.RLock()
	defer re.mutex.RUnlock()

	// 返回引用而不是深拷贝，因为 RequestInfo 的字段都是不可变的
	// 如果需要修改，调用者应该自己进行拷贝
	return re.requestInfo
}

func (re *RequestExtractor) HasExtracted() bool {
	re.mutex.RLock()
	defer re.mutex.RUnlock()
	return re.requestInfo.Extracted
}

// copyHeaders 函数不再需要，删除
// func copyHeaders(headers map[string]string) map[string]string {
//     if headers == nil {
//         return make(map[string]string)
//     }
//     
//     result := make(map[string]string, len(headers))
//     for k, v := range headers {
//         result[k] = v
//     }
//     return result
// }