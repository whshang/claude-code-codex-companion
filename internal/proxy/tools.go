package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"

	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/endpoint"
)

// tools.go: 工具调用模块
// 负责处理与模型工具调用（Tool Use）相关的所有逻辑。
//
// 目标：
// - 包含检测、解析和转换工具调用的相关函数。
// - 处理从不同模型格式（如 Anthropic 的 tool_use 和 OpenAI 的 tool_calls）到内部表示的转换。
// - 确保在响应转换过程中，工具调用的结果能被正确地格式化回客户端期望的格式。

// requestHasTools 检查请求体中是否包含 tools 参数
func (s *Server) requestHasTools(requestBody []byte) bool {
	var reqMap map[string]interface{}
	if err := json.Unmarshal(requestBody, &reqMap); err != nil {
		return false
	}

	if tools, ok := reqMap["tools"].([]interface{}); ok && len(tools) > 0 {
		return true
	}

	return false
}

// convertChatCompletionsToResponsesSSE 将 OpenAI /chat/completions SSE 格式转换为 /responses API 格式
// Codex 客户端使用 /responses API，期望的事件格式为：
//   - {"type": "response.created", "response": {...}}
//   - {"type": "response.output_text.delta", "delta": "..."}
//   - {"type": "response.completed", "response": {...}}
func (s *Server) convertChatCompletionsToResponsesSSE(body []byte, endpointName string) []byte {
	unified := func() ([]byte, error) {
		var buf bytes.Buffer
		if err := conversion.StreamChatCompletionsToResponses(bytes.NewReader(body), &buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	var (
		result []byte
		mode   conversion.ConversionMode
		err    error
	)

	if s.conversionManager != nil {
		result, mode, err = s.conversionManager.Convert("chat_sse_to_responses", endpointName, unified, nil)
	} else {
		result, err = unified()
		mode = conversion.ConversionModeUnified
	}

	if err != nil {
		s.logger.Error("Failed to convert chat completions SSE to responses format", err, map[string]interface{}{
			"operation": "chat_sse_to_responses",
			"mode":      string(mode),
			"endpoint":  endpointName,
		})
		return body
	}
	if len(result) == 0 {
		return body
	}
	return result
}

// convertChatCompletionToResponse 将 OpenAI /chat/completions 非流式响应转换为 Codex /responses 格式
// OpenAI格式: {"id":"xxx","object":"chat.completion","created":123,"model":"xxx","choices":[{"index":0,"message":{"role":"assistant","content":"..."}}]}
// Codex格式: {"type":"response","id":"xxx","object":"response","created":123,"model":"xxx","choices":[{"index":0,"message":{"role":"assistant","content":"..."}}]}
func (s *Server) convertChatCompletionToResponse(body []byte, endpointName string) ([]byte, error) {
	unified := func() ([]byte, error) {
		return conversion.ConvertChatResponseJSONToResponses(body)
	}

	var (
		result []byte
		mode   conversion.ConversionMode
		err    error
	)

	if s.conversionManager != nil {
		result, mode, err = s.conversionManager.Convert("chat_json_to_responses", endpointName, unified, nil)
	} else {
		result, err = unified()
		mode = conversion.ConversionModeUnified
	}

	if err != nil {
		s.logger.Error("Failed to convert chat response to Responses format", err, map[string]interface{}{
			"operation": "chat_json_to_responses",
			"mode":      string(mode),
			"endpoint":  endpointName,
		})
		return nil, err
	}
	if result == nil {
		return body, nil
	}
	return result, nil
}

// convertCodexToOpenAI 将 Codex /responses 请求转换为 OpenAI /chat/completions 请求
func (s *Server) convertCodexToOpenAI(requestBody []byte, endpointName string) ([]byte, error) {
	unified := func() ([]byte, error) {
		return conversion.ConvertResponsesRequestJSONToChat(requestBody)
	}

	var (
		result []byte
		mode   conversion.ConversionMode
		err    error
	)

	if s.conversionManager != nil {
		result, mode, err = s.conversionManager.Convert("responses_json_to_chat", endpointName, unified, nil)
	} else {
		result, err = unified()
		mode = conversion.ConversionModeUnified
	}

	if err != nil {
		s.logger.Debug("Skipping Codex->OpenAI conversion", map[string]interface{}{
			"error":    err.Error(),
			"mode":     string(mode),
			"endpoint": endpointName,
		})
		return nil, nil
	}
	return result, nil
}

// convertResponseJSONToSSE 将 Responses JSON 响应转换为 SSE 流格式
// 用于当客户端期望流式响应但上游返回非流式 JSON 时
func (s *Server) convertResponseJSONToSSE(jsonBody []byte) []byte {
	var respData map[string]interface{}
	if err := json.Unmarshal(jsonBody, &respData); err != nil {
		s.logger.Error("Failed to parse Response JSON", err)
		return nil
	}

	// 添加调试日志，查看实际的响应结构
	s.logger.Debug("Converting Response JSON to SSE", map[string]interface{}{
		"response_keys": func() []string {
			keys := make([]string, 0, len(respData))
			for k := range respData {
				keys = append(keys, k)
			}
			return keys
		}(),
		"response_preview": string(jsonBody[:min(200, len(jsonBody))]),
	})

	// 构造 SSE 事件流
	var buf bytes.Buffer

	// 1. response.created 事件
	createdEvent := map[string]interface{}{
		"type":     "response.created",
		"response": respData,
	}
	if createdJSON, err := json.Marshal(createdEvent); err == nil {
		buf.WriteString("event: response.created\n")
		buf.WriteString(fmt.Sprintf("data: %s\n\n", string(createdJSON)))
	}

	// 2. response.output_text.delta 事件（包含完整内容）
	// Responses API 格式的结构是: {"output": [{"content": [{"text": "..."}]}]}
	var contentText string

	// 从 Responses API 格式中提取内容
	if output, ok := respData["output"].([]interface{}); ok && len(output) > 0 {
		if outputItem, ok := output[0].(map[string]interface{}); ok {
			if content, ok := outputItem["content"].([]interface{}); ok && len(content) > 0 {
				if contentItem, ok := content[0].(map[string]interface{}); ok {
					if text, ok := contentItem["text"].(string); ok && text != "" {
						contentText = text
					}
				}
			}
		}
	}

	// 如果没有找到内容，尝试从 Chat Completion 格式中提取（兼容性）
	if contentText == "" {
		if choices, ok := respData["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if message, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok && content != "" {
						contentText = content
					}
				}
			}
		}
	}

	// 如果还是没有找到内容，尝试其他可能的字段
	if contentText == "" {
		if text, ok := respData["text"].(string); ok {
			contentText = text
		} else if content, ok := respData["content"].(string); ok {
			contentText = content
		}
	}

	if contentText != "" {
		deltaEvent := map[string]interface{}{
			"type":  "response.output_text.delta",
			"delta": contentText,
		}
		if deltaJSON, err := json.Marshal(deltaEvent); err == nil {
			buf.WriteString("event: response.output_text.delta\n")
			buf.WriteString(fmt.Sprintf("data: %s\n\n", string(deltaJSON)))
		}
	} else {
		s.logger.Debug("No content found in Response JSON for SSE conversion")
	}

	// 3. response.completed 事件
	completedEvent := map[string]interface{}{
		"type":     "response.completed",
		"response": respData,
	}
	if completedJSON, err := json.Marshal(completedEvent); err == nil {
		buf.WriteString("event: response.completed\n")
		buf.WriteString(fmt.Sprintf("data: %s\n\n", string(completedJSON)))
	}

	return buf.Bytes()
}

// 动态更新端点的Codex支持状态
func (s *Server) updateEndpointCodexSupport(ep *endpoint.Endpoint, isCodex bool) {
	if ep == nil {
		return
	}

	// 使用端点的公共方法来安全地更新状态
	ep.UpdateNativeCodexSupport(isCodex)
	s.logger.Info(fmt.Sprintf("Updated endpoint %s native_codex_support to %v", ep.Name, isCodex))
}
