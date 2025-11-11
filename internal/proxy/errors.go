package proxy

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	jsonutils "claude-code-codex-companion/internal/common/json"
	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/endpoint"
	"github.com/gin-gonic/gin"
)

// errors.go: 错误处理模块
// 负责代理过程中的错误学习、分类和自适应逻辑。
//
// 目标：
// - 包含 autoRemoveUnsupportedParams 和 learnUnsupportedParamsFromError 等函数。
// - 集中处理因上游端点返回特定错误而触发的自适应行为。
// - 定义和管理代理层面的特定错误类型。

const upstreamErrorHintsKey = "upstream_error_hints"

type upstreamErrorMatch struct {
	Message    string
	Action     string
	MaxRetries int
	Pattern    string
}

type upstreamError struct {
	rawMessage string
	action     string
	maxRetries int
	endpoint   string
	model      string
	pattern    string
}

func (e *upstreamError) Error() string {
	base := "upstream error"
	if e.endpoint != "" {
		base += fmt.Sprintf(" on endpoint %s", e.endpoint)
	}
	if e.model != "" {
		base += fmt.Sprintf(" (model %s)", e.model)
	}
	message := e.rawMessage
	if e.pattern != "" {
		message = fmt.Sprintf("%s (matched rule %q)", message, e.pattern)
	}
	return fmt.Sprintf("%s: %s", base, message)
}

func (e *upstreamError) Hint() string {
	if e == nil {
		return ""
	}
	target := "an upstream endpoint"
	if e.endpoint != "" {
		target = fmt.Sprintf("endpoint %s", e.endpoint)
	}
	if e.model != "" {
		target += fmt.Sprintf(" (model %s)", e.model)
	}

	hint := fmt.Sprintf("%s returned `%s`", target, e.rawMessage)
	if e.pattern != "" {
		hint += fmt.Sprintf(", matching retry rule %q", e.pattern)
	}
	switch e.action {
	case "switch_endpoint":
		hint += ". CCCC will try other endpoints automatically."
	case "retry_endpoint":
		hint += ". CCCC will retry this endpoint as per rule."
	case "":
		// no-op
	default:
		hint += fmt.Sprintf(". Applied retry action %q.", e.action)
	}
	return hint
}

func appendUpstreamHint(c *gin.Context, hint string) {
	if c == nil || hint == "" {
		return
	}
	if existing, ok := c.Get(upstreamErrorHintsKey); ok {
		if hints, ok := existing.([]string); ok {
			c.Set(upstreamErrorHintsKey, append(hints, hint))
			return
		}
	}
	c.Set(upstreamErrorHintsKey, []string{hint})
}

func getUpstreamHints(c *gin.Context) []string {
	if c == nil {
		return nil
	}
	if val, ok := c.Get(upstreamErrorHintsKey); ok {
		if hints, ok := val.([]string); ok {
			return hints
		}
	}
	return nil
}

// autoRemoveUnsupportedParams 自动移除端点已学习到的不支持参数
func (s *Server) autoRemoveUnsupportedParams(requestBody []byte, ep *endpoint.Endpoint) ([]byte, bool) {
	// 获取端点学习到的不支持参数列表
	unsupportedParams := ep.GetLearnedUnsupportedParams()
	if len(unsupportedParams) == 0 {
		return requestBody, false
	}

	// 解析请求体
	var requestData map[string]interface{}
	if err := jsonutils.SafeUnmarshal(requestBody, &requestData); err != nil {
		return requestBody, false
	}

	// 移除学习到的不支持参数
	modified := false
	for _, param := range unsupportedParams {
		if _, exists := requestData[param]; exists {
			delete(requestData, param)
			modified = true
			s.logger.Debug(fmt.Sprintf("Auto-removed '%s' parameter (learned from previous failures)", param))
		}
	}

	if !modified {
		return requestBody, false
	}

	// 重新序列化
	modifiedBody, err := jsonutils.SafeMarshal(requestData)
	if err != nil {
		return requestBody, false
	}

	return modifiedBody, true
}

// learnUnsupportedParamsFromError 从错误响应中学习不支持的参数
func (s *Server) learnUnsupportedParamsFromError(errorBody []byte, ep *endpoint.Endpoint, originalReqBody []byte) {
	if ep == nil || len(errorBody) == 0 {
		return
	}

	// 解析错误消息
	var errorData map[string]interface{}
	if err := jsonutils.SafeUnmarshal(errorBody, &errorData); err != nil {
		return // 无法解析为JSON,忽略
	}

	// 尝试从错误消息中提取参数名
	errorMsg := ""
	if msg, ok := errorData["message"].(string); ok {
		errorMsg = msg
	} else if err, ok := errorData["error"].(map[string]interface{}); ok {
		if msg, ok := err["message"].(string); ok {
			errorMsg = msg
		}
	} else if err, ok := errorData["error"].(string); ok {
		errorMsg = err
	}

	if errorMsg == "" {
		return
	}

	// 解析请求体以检查哪些参数存在
	var requestData map[string]interface{}
	if err := jsonutils.SafeUnmarshal(originalReqBody, &requestData); err != nil {
		return
	}

	// 常见的不支持参数关键词模式
	unsupportedPatterns := []struct {
		keywords []string
		params   []string
	}{
		{
			keywords: []string{"tool", "function", "function_call", "tool_choice"},
			params:   []string{"tools", "tool_choice", "functions", "function_call"},
		},
		{
			keywords: []string{"unsupported", "not supported", "invalid parameter", "unexpected parameter"},
			params:   []string{}, // 将从错误消息中动态提取
		},
	}

	errorMsgLower := strings.ToLower(errorMsg)

	// 检查每个模式
	for _, pattern := range unsupportedPatterns {
		matched := false
		for _, keyword := range pattern.keywords {
			if strings.Contains(errorMsgLower, keyword) {
				matched = true
				break
			}
		}

		if matched {
			// 如果模式匹配，学习对应的参数
			if len(pattern.params) > 0 {
				for _, param := range pattern.params {
					if _, exists := requestData[param]; exists {
						ep.LearnUnsupportedParam(param)
						s.logger.Info("Learned unsupported parameter from API error", map[string]interface{}{
							"endpoint":  ep.Name,
							"parameter": param,
							"error_msg": errorMsg,
						})
					}
				}
			} else {
				// 尝试从错误消息中提取参数名
				// 匹配类似 "parameter 'xxx' is not supported" 或 "unsupported parameter: xxx"
				paramNameRegex := regexp.MustCompile(`parameter["']?\s*([a-zA-Z_][a-zA-Z0-9_]*)`)
				matches := paramNameRegex.FindStringSubmatch(errorMsg)
				if len(matches) > 1 {
					paramName := matches[1]
					if _, exists := requestData[paramName]; exists {
						ep.LearnUnsupportedParam(paramName)
						s.logger.Info("Learned unsupported parameter from API error (regex)", map[string]interface{}{
							"endpoint":  ep.Name,
							"parameter": paramName,
							"error_msg": errorMsg,
						})
					}
				}
			}
		}
	}
}

func shouldMarkResponsesUnsupported(status int, body []byte) bool {
	switch status {
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return true
	case http.StatusBadRequest:
		// inspect payload for unsupported hints
		var messageCandidates []string
		if len(body) > 0 {
			var payload map[string]interface{}
			if err := jsonutils.SafeUnmarshal(body, &payload); err == nil {
				if msg, ok := payload["message"].(string); ok {
					messageCandidates = append(messageCandidates, msg)
				}
				if errField, ok := payload["error"]; ok {
					switch v := errField.(type) {
					case string:
						messageCandidates = append(messageCandidates, v)
					case map[string]interface{}:
						if msg, ok := v["message"].(string); ok {
							messageCandidates = append(messageCandidates, msg)
						}
					}
				}
			}

			for _, candidate := range messageCandidates {
				if containsResponsesUnsupportedHint(candidate) {
					return true
				}
			}

			// fallback to body preview search
			bodyPreview := body
			if len(bodyPreview) > 4096 {
				bodyPreview = bodyPreview[:4096]
			}
			if containsResponsesUnsupportedHint(string(bodyPreview)) {
				return true
			}
		}
	}
	return false
}

func containsResponsesUnsupportedHint(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	keywords := []string{
		"unknown path",
		"unsupported",
		"not supported",
		"no route",
		"invalid path",
		"unknown endpoint",
		"unrecognized endpoint",
		"no such route",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

var defaultUpstreamErrorKeywords = []string{
	"api error:",
	"cannot read properties of undefined",
	"internal server error",
}

func detectUpstreamErrorResponse(body []byte, rules []config.UpstreamErrorRule) *upstreamErrorMatch {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}
	var payload map[string]interface{}
	if err := jsonutils.SafeUnmarshal(body, &payload); err == nil {
		candidates = append(candidates, collectUpstreamErrorTexts(payload)...)
	}

	for _, rule := range rules {
		if rule.Pattern == "" {
			continue
		}
		if matchAnyPattern(candidates, rule.Pattern, rule.CaseInsensitive) {
			action := strings.ToLower(strings.TrimSpace(rule.Action))
			if action == "" {
				action = "switch_endpoint"
			}
			return &upstreamErrorMatch{
				Message:    trimmed,
				Action:     action,
				MaxRetries: rule.MaxRetries,
				Pattern:    rule.Pattern,
			}
		}
	}

	for _, kw := range defaultUpstreamErrorKeywords {
		if matchAnyPattern(candidates, kw, true) {
			return &upstreamErrorMatch{
				Message: trimmed,
				Action:  "switch_endpoint",
				Pattern: kw,
			}
		}
	}

	return nil
}

func collectUpstreamErrorTexts(payload map[string]interface{}) []string {
	texts := make([]string, 0)

	if errField, ok := payload["error"]; ok {
		switch v := errField.(type) {
		case string:
			texts = append(texts, strings.TrimSpace(v))
		case map[string]interface{}:
			if msg, ok := v["message"].(string); ok {
				texts = append(texts, strings.TrimSpace(msg))
			}
		}
	}

	if choices, ok := payload["choices"].([]interface{}); ok {
		for _, item := range choices {
			choice, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					texts = append(texts, strings.TrimSpace(content))
				}
			}
		}
	}

	if content, ok := payload["content"]; ok {
		switch blocks := content.(type) {
		case []interface{}:
			for _, item := range blocks {
				if blockMap, ok := item.(map[string]interface{}); ok {
					if text, ok := blockMap["text"].(string); ok {
						texts = append(texts, strings.TrimSpace(text))
					}
				}
			}
		case string:
			texts = append(texts, strings.TrimSpace(blocks))
		}
	}

	return texts
}

func matchAnyPattern(texts []string, pattern string, caseInsensitive bool) bool {
	if pattern == "" {
		return false
	}
	for _, text := range texts {
		if matchPattern(text, pattern, caseInsensitive) {
			return true
		}
	}
	return false
}

func matchPattern(text, pattern string, caseInsensitive bool) bool {
	if caseInsensitive {
		return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
	}
	return strings.Contains(text, pattern)
}
