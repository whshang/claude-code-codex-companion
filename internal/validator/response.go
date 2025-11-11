package validator

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type ResponseValidator struct {
	// 注释：strict_mode 和 validate_stream 已永久启用
}

func NewResponseValidator() *ResponseValidator {
	return &ResponseValidator{}
}

func (v *ResponseValidator) ValidateAnthropicResponse(body []byte, isStreaming bool) error {
	return v.ValidateResponse(body, isStreaming, "anthropic", "")
}

func (v *ResponseValidator) ValidateResponse(body []byte, isStreaming bool, endpointType, endpointURL string) error {
	return v.ValidateResponseWithPath(body, isStreaming, endpointType, "", endpointURL)
}

func (v *ResponseValidator) ValidateResponseWithPath(body []byte, isStreaming bool, endpointType, path, endpointURL string) error {
	// 流式验证和严格模式已永久启用

	// 跳过 count_tokens 接口的 Anthropic 格式验证
	if isCountTokensEndpoint(path) {
		// count_tokens 接口只做基本 JSON 格式验证
		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("invalid JSON response: %v", err)
		}
		// count_tokens 应该返回包含 input_tokens 的响应
		if _, hasInputTokens := response["input_tokens"]; hasInputTokens {
			return nil
		}
		return fmt.Errorf("count_tokens response missing input_tokens field")
	}

	if isStreaming {
		// 首先进行基本的SSE chunk验证
		if err := v.ValidateSSEChunk(body, endpointType); err != nil {
			return err
		}
		// 然后验证完整SSE流的完整性
		return v.ValidateCompleteSSEStream(body, endpointType, path, endpointURL)
	}
	return v.ValidateStandardResponse(body, endpointType)
}

// isCountTokensEndpoint 检查是否为 count_tokens 接口
func isCountTokensEndpoint(path string) bool {
	return strings.Contains(path, "/count_tokens")
}

func (v *ResponseValidator) ValidateStandardResponse(body []byte, endpointType string) error {
	// 首先检查空响应体
	if len(body) == 0 {
		return NewBusinessError("endpoint returned empty response body", nil)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		// JSON解析失败，可能是HTML错误页面或其他格式
		bodyPreview := string(body)
		if len(bodyPreview) > 200 {
			bodyPreview = bodyPreview[:200] + "..."
		}
		return NewBusinessError(fmt.Sprintf("invalid JSON response: %v (preview: %s)", err, bodyPreview), err)
	}

	// 首先检查是否包含error字段（业务错误）
	if errorField, hasError := response["error"]; hasError {
		// 这是一个业务错误响应，不应触发端点黑名单
		errorMsg := "API returned error response"
		if errorMap, ok := errorField.(map[string]interface{}); ok {
			if msg, ok := errorMap["message"].(string); ok {
				errorMsg = msg
			}
		}
		return NewBusinessError(errorMsg, nil)
	}

	// 严格模式已永久启用
	if endpointType == "anthropic" {
		requiredFields := []string{"id", "type", "content", "model"}
		for _, field := range requiredFields {
			if _, exists := response[field]; !exists {
				return NewFormatError(fmt.Sprintf("missing required field: %s", field), nil)
			}
		}

		if msgType, ok := response["type"].(string); !ok || msgType != "message" {
			return NewFormatError(fmt.Sprintf("invalid message type: expected 'message', got '%v'", response["type"]), nil)
		}

		if role, exists := response["role"]; exists {
			if roleStr, ok := role.(string); !ok || roleStr != "assistant" {
				return NewFormatError(fmt.Sprintf("invalid role: expected 'assistant', got '%v'", role), nil)
			}
		}
	} else if endpointType == "openai" {
		// OpenAI格式验证：检查基本结构
		// 注意：某些OpenAI兼容API（如Kimi）可能不返回id字段，所以只检查model字段
		if _, hasModel := response["model"]; !hasModel {
			return NewFormatError("missing required field for OpenAI format: model", nil)
		}

		// 验证是否有choices字段（已经在前面检查过error了）
		if _, hasChoices := response["choices"]; !hasChoices {
			return NewFormatError("OpenAI response missing 'choices' field", nil)
		}

		// 如果有object字段，验证其值（可选）
		if objectType, ok := response["object"].(string); ok && strings.TrimSpace(objectType) != "" {
			if objectType != "chat.completion" && objectType != "chat.completion.chunk" {
				return NewFormatError(fmt.Sprintf("invalid object type for OpenAI: expected 'chat.completion' or 'chat.completion.chunk', got '%v'", objectType), nil)
			}
		}
	} else {
		// 非严格模式：只要是有效JSON且包含content或choices字段即可（error已在前面检查）
		if _, hasContent := response["content"]; hasContent {
			return nil
		}
		if _, hasChoices := response["choices"]; hasChoices {
			return nil // OpenAI格式通常有choices字段
		}
		// 如果既没有content也没有choices，认为是无效响应
		return NewFormatError("response missing both 'content' and 'choices' fields", nil)
	}

	return nil
}

func (v *ResponseValidator) ValidateSSEChunk(chunk []byte, endpointType string) error {
	lines := bytes.Split(chunk, []byte("\n"))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if bytes.HasPrefix(line, []byte("event: ")) {
			eventType := string(line[7:])

			if endpointType == "anthropic" {
				validEvents := []string{
					"message_start", "content_block_start", "ping",
					"content_block_delta", "content_block_stop", "message_stop",
					"message_delta", "error",
				}

				valid := false
				for _, validEvent := range validEvents {
					if eventType == validEvent {
						valid = true
						break
					}
				}

				if !valid {
					return fmt.Errorf("invalid SSE event type for Anthropic: %s", eventType)
				}
			}
			// OpenAI格式通常不使用event字段，或者使用不同的事件类型，这里不做严格验证
		}

		// 兼容 "data:" 和 "data: " 两种写法
		if bytes.HasPrefix(line, []byte("data:")) {
			dataContent := bytes.TrimSpace(line[len("data:"):])
			if len(dataContent) == 0 || string(dataContent) == "[DONE]" {
				continue
			}

			var data map[string]interface{}
			if err := json.Unmarshal(dataContent, &data); err != nil {
				return fmt.Errorf("invalid JSON in SSE data: %v", err)
			}

			// 严格模式已永久启用
			if endpointType == "anthropic" {
				if _, hasType := data["type"]; !hasType {
					return fmt.Errorf("missing 'type' field in SSE data")
				}

				// 检查message_start事件的usage统计
				if err := v.ValidateMessageStartUsage(data); err != nil {
					return err
				}
			} else if endpointType == "openai" {
				// OpenAI格式：允许两种流式形态
				// 1) Chat Completions chunk: 可能包含 choices / delta / finish_reason
				// 2) Responses API 事件: {"type":"response.*", ...} 或 顶层包含 response 对象
				if _, hasModel := data["model"]; !hasModel {
					// 容忍无顶层model的情况，但需命中至少一种已知结构
					t, hasType := data["type"].(string)
					isResponsesEvent := hasType && strings.HasPrefix(t, "response.")
					_, hasResponseObj := data["response"].(map[string]interface{})
					choicesRaw, hasChoices := data["choices"]
					_, hasDelta := data["delta"]
					_, hasUsage := data["usage"]
					_, hasObject := data["object"]
					_, hasID := data["id"]
					if !(isResponsesEvent || hasResponseObj || hasChoices || hasDelta || hasUsage || hasObject || hasID || hasType) {
						return fmt.Errorf("missing 'model' field in OpenAI SSE data")
					}

					if hasChoices {
						switch choices := choicesRaw.(type) {
						case []interface{}:
							if len(choices) > 0 {
								if first, ok := choices[0].(map[string]interface{}); ok {
									nestedModel, hasNestedModel := first["model"]
									if hasNestedModel {
										if _, ok := nestedModel.(string); !ok && nestedModel != nil {
											return fmt.Errorf("invalid 'model' field in OpenAI SSE data: expected string")
										}
									}
								}
							}
						default:
							return fmt.Errorf("invalid 'choices' field in OpenAI SSE data")
						}
					}
				}
				// OpenAI格式不要求type和object字段
			}
		}
	}

	return nil
}

// ValidateCompleteSSEStream 验证完整的SSE流是否包含所有必需的事件
func (v *ResponseValidator) ValidateCompleteSSEStream(body []byte, endpointType, path, endpointURL string) error {
	if endpointType == "anthropic" {
		return v.validateAnthropicSSECompleteness(body)
	} else if endpointType == "openai" {
		return v.validateOpenAISSECompleteness(body, path, endpointURL)
	}
	return nil
}

// validateAnthropicSSECompleteness 验证Anthropic SSE流的完整性
func (v *ResponseValidator) validateAnthropicSSECompleteness(body []byte) error {
	lines := bytes.Split(body, []byte("\n"))
	hasMessageStart := false
	hasMessageStop := false

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("event: ")) {
			eventType := string(line[7:])
			switch eventType {
			case "message_start":
				hasMessageStart = true
			case "message_stop":
				hasMessageStop = true
			}
		}
	}

	if hasMessageStart && !hasMessageStop {
		return fmt.Errorf("incomplete SSE stream: has message_start but missing message_stop event")
	}

	return nil
}

// validateOpenAISSECompleteness 验证OpenAI SSE流的完整性
// 支持三种完整性标记：
// 1. Chat Completions: finish_reason 字段
// 2. Responses API: response.completed 事件
// 3. 标准终止: [DONE] 标记
// 有任意一种即可认为流完整
// 使用端点配置的SSE设置来决定是否要求[DONE]标记
func (v *ResponseValidator) validateOpenAISSECompleteness(body []byte, path, endpointURL string) error {
	lines := bytes.Split(body, []byte("\n"))
	hasFinishReason := false
	hasResponseCompleted := false
	hasDoneMarker := strings.Contains(string(body), "[DONE]")

	// 检查完整性标志
	for _, line := range lines {
		line = bytes.TrimSpace(line)

		// 检查 Responses API 的完成事件
		if bytes.HasPrefix(line, []byte("event: ")) {
			eventType := string(line[7:])
			if eventType == "response.completed" || eventType == "response.done" {
				hasResponseCompleted = true
				break
			}
		}

		// 检查 Chat Completions 的 finish_reason
		if bytes.HasPrefix(line, []byte("data: ")) {
			dataContent := line[6:]
			if len(dataContent) == 0 || string(dataContent) == "[DONE]" {
				continue
			}

			var data map[string]interface{}
			if err := json.Unmarshal(dataContent, &data); err != nil {
				continue
			}

			// Chat Completions 格式
			if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if finishReason, exists := choice["finish_reason"]; exists && finishReason != nil {
						hasFinishReason = true
						break
					}
				}
			}

			// Responses API 格式：检查 status
			if typeVal, hasType := data["type"]; hasType {
				if typeStr, ok := typeVal.(string); ok && (typeStr == "response.completed" || typeStr == "response.done") {
					hasResponseCompleted = true
					break
				}
			}
		}
	}

	// 智能判断：有任意一种完整性标志即可认为流完整
	// 这种宽松的策略允许系统自动适应不同API提供商的行为
	if hasFinishReason || hasResponseCompleted || hasDoneMarker {
		return nil
	}

	return fmt.Errorf("incomplete OpenAI SSE stream: missing finish_reason, response.completed, and [DONE] marker")
}

func (v *ResponseValidator) DecompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip data: %v", err)
	}

	return decompressed, nil
}

func (v *ResponseValidator) GetDecompressedBody(body []byte, contentEncoding string) ([]byte, error) {
	if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
		return v.DecompressGzip(body)
	}
	return body, nil
}

func (v *ResponseValidator) IsGzipContent(contentEncoding string) bool {
	return strings.Contains(strings.ToLower(contentEncoding), "gzip")
}

func (v *ResponseValidator) ValidateMessageStartUsage(eventData map[string]interface{}) error {
	eventType, ok := eventData["type"].(string)
	if !ok || eventType != "message_start" {
		return nil
	}

	message, ok := eventData["message"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid message_start: missing message field")
	}

	usage, ok := message["usage"].(map[string]interface{})
	if !ok {
		// 某些兼容实现不会在 message_start 中返回 usage，放宽要求
		return nil
	}

	// 检查是否存在 input_tokens 和 output_tokens 字段
	_, hasInputTokens := usage["input_tokens"]
	_, hasOutputTokens := usage["output_tokens"]

	if hasInputTokens && hasOutputTokens {
		// 如果存在标准字段，直接认为是合法的（不管值是什么）
		return nil
	} else {
		// 如果不存在标准字段，检查是否为不合法的格式
		promptTokens := float64(-1)
		completionTokens := float64(-1)
		totalTokens := float64(-1)

		if val, ok := usage["prompt_tokens"]; ok {
			if num, ok := val.(float64); ok {
				promptTokens = num
			}
		}

		if val, ok := usage["completion_tokens"]; ok {
			if num, ok := val.(float64); ok {
				completionTokens = num
			}
		}

		if val, ok := usage["total_tokens"]; ok {
			if num, ok := val.(float64); ok {
				totalTokens = num
			}
		}

		// 只有当三个字段都存在且都为0时才判定为不合法
		if promptTokens == 0 && completionTokens == 0 && totalTokens == 0 {
			return fmt.Errorf("invalid usage stats: prompt_tokens, completion_tokens and total_tokens are all zero, indicating malformed response")
		}
	}

	return nil
}

// DetectJSONContent 检测内容是否为JSON格式（而非SSE格式）
func (v *ResponseValidator) DetectJSONContent(body []byte) bool {
	if len(body) == 0 {
		return false
	}

	// 检查是否为有效JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return false
	}

	// 如果能成功解析为JSON，就是JSON内容
	// 不需要检查内容中是否包含SSE相关字符串，因为JSON可以包含任何字符串内容
	return true
}

// DetectSSEContent 检测内容是否为SSE格式
func (v *ResponseValidator) DetectSSEContent(body []byte) bool {
	if len(body) == 0 {
		return false
	}

	bodyStr := string(body)

	// 首先检查是否是有效的JSON，如果是JSON则不是SSE
	trimmed := strings.TrimSpace(bodyStr)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		// 尝试解析JSON来确认
		var temp interface{}
		if json.Unmarshal(body, &temp) == nil {
			return false // 是有效的JSON，不是SSE
		}
	}

	// 检查SSE格式：必须有以"event: "或"data: "开头的行
	lines := strings.Split(bodyStr, "\n")
	hasEventLine := false
	hasDataLine := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "event: ") {
			hasEventLine = true
		}
		if strings.HasPrefix(trimmedLine, "data: ") {
			hasDataLine = true
		}
	}

	// SSE格式必须至少有一个data:行，通常还有event:行
	return hasDataLine && (hasEventLine || len(lines) > 1)
}

// SmartDetectContentType 智能检测内容类型并返回应该设置的Content-Type和覆盖信息
// 返回值: (newContentType, overrideInfo)
// - newContentType: 应该设置的Content-Type，空字符串表示不需要修改
// - overrideInfo: 覆盖信息，用于日志记录，格式如 "json->sse" 或 "sse->json"
func (v *ResponseValidator) SmartDetectContentType(body []byte, currentContentType string, statusCode int) (string, string) {
	if statusCode != 200 || len(body) == 0 {
		return "", "" // 只处理200状态码的响应
	}

	// 标准化当前Content-Type
	currentContentTypeLower := strings.ToLower(currentContentType)
	isCurrentSSE := strings.Contains(currentContentTypeLower, "text/event-stream")
	isCurrentJSON := strings.Contains(currentContentTypeLower, "application/json")
	isCurrentPlain := strings.Contains(currentContentTypeLower, "text/plain")

	// 检测实际内容类型
	isActualSSE := v.DetectSSEContent(body)
	isActualJSON := v.DetectJSONContent(body)

	// 决定是否需要覆盖Content-Type
	if isActualSSE && !isCurrentSSE {
		// 内容是SSE但Content-Type不是，覆盖为SSE
		if isCurrentJSON {
			return "text/event-stream; charset=utf-8", "json->sse"
		} else if isCurrentPlain {
			return "text/event-stream; charset=utf-8", "plain->sse"
		} else {
			return "text/event-stream; charset=utf-8", "unknown->sse"
		}
	} else if isActualJSON && !isCurrentJSON {
		// 内容是JSON但Content-Type不是，覆盖为JSON
		if isCurrentSSE {
			return "application/json", "sse->json"
		} else if isCurrentPlain {
			return "application/json", "plain->json"
		} else {
			return "application/json", "unknown->json"
		}
	}

	// 不需要覆盖
	return "", ""
}
