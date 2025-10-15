package conversion

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// StreamChatToResponsesRealtime 实时流式转换：边读边写，避免缓冲整个流
// 🔧 优化点：增强工具调用检测、改进错误处理、支持finish_reason映射
func StreamChatToResponsesRealtime(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	respID := generateResponseID()
	var model string
	var usage *OpenAIUsage
	var finishReason string
	sentCreated := false

	// 🆕 工具调用状态跟踪（支持多个工具调用）
	toolCallStates := make(map[int]*struct {
		id        string
		name      string
		arguments strings.Builder
		started   bool
	})

	// 实时流式转换
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 处理data行（兼容 "data:" 和 "data: " 两种格式）
		if strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "data :") {
			dataContent := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), "data :"))

			// 检查结束标记
			if dataContent == "[DONE]" {
				break
			}

			// 解析JSON chunk
			var chunk OpenAIStreamChunk
			if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
				// 🔧 优化：记录解析错误但继续处理
				continue
			}

			// 更新响应信息
			if chunk.ID != "" {
				respID = chunk.ID
			}
			if chunk.Model != "" {
				model = chunk.Model
			}
			if chunk.Usage != nil {
				usage = chunk.Usage
			}

			// 发送 response.created（仅一次）
			if !sentCreated {
				created := map[string]interface{}{
					"type": "response.created",
					"response": map[string]interface{}{
						"id":    respID,
						"model": model,
					},
				}
				if err := writeResponsesSSEEvent(w, "response.created", created); err != nil {
					return err
				}
				sentCreated = true
			}

			// 处理每个chunk的内容
			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]

				// 🔧 优化：记录finish_reason用于最终事件
				if choice.FinishReason != "" {
					finishReason = normalizeFinishReason(choice.FinishReason)
				}

				// 处理文本内容
				if choice.Delta.Content != nil {
					var contentStr string
					switch v := choice.Delta.Content.(type) {
					case string:
						contentStr = v
					case []byte:
						contentStr = string(v)
					default:
						// 其他类型暂时忽略
					}

					if contentStr != "" {
						textDelta := map[string]interface{}{
							"type":         "response.output_text.delta",
							"response_id":  respID,
							"delta":        contentStr,
							"output_index": 0,
						}
						if err := writeResponsesSSEEvent(w, "response.output_text.delta", textDelta); err != nil {
							return err
						}
					}
				}

				// 🆕 优化：增强工具调用处理，支持增量更新
				if choice.Delta.ToolCalls != nil {
					for _, toolCall := range choice.Delta.ToolCalls {
						idx := toolCall.Index
						if idx < 0 {
							idx = 0 // 默认索引
						}

						// 获取或创建工具调用状态
						state, exists := toolCallStates[idx]
						if !exists {
							state = &struct {
								id        string
								name      string
								arguments strings.Builder
								started   bool
							}{}
							toolCallStates[idx] = state
						}

						// 更新ID和名称
						if toolCall.ID != "" && state.id == "" {
							state.id = toolCall.ID
						}
						if toolCall.Function.Name != "" && state.name == "" {
							state.name = toolCall.Function.Name
						}

						// 发送工具调用开始事件（仅一次）
						if !state.started && (state.id != "" || state.name != "") {
							start := map[string]interface{}{
								"type":         "response.function_call.started",
								"response_id":  respID,
								"output_index": 0,
							}
							if state.id != "" {
								start["id"] = state.id
							}
							if state.name != "" {
								start["name"] = state.name
							}
							if err := writeResponsesSSEEvent(w, "response.function_call.started", start); err != nil {
								return err
							}
							state.started = true
						}

						// 累积并发送参数delta
						if toolCall.Function.Arguments != "" {
							state.arguments.WriteString(toolCall.Function.Arguments)
							argDelta := map[string]interface{}{
								"type":         "response.function_call_arguments.delta",
								"response_id":  respID,
								"delta":        toolCall.Function.Arguments,
								"output_index": 0,
							}
							if state.id != "" {
								argDelta["id"] = state.id
							}
							if err := writeResponsesSSEEvent(w, "response.function_call_arguments.delta", argDelta); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// 🆕 发送所有工具调用的完成事件
	for _, state := range toolCallStates {
		if state.started && state.id != "" {
			completed := map[string]interface{}{
				"type":         "response.function_call.completed",
				"response_id":  respID,
				"id":           state.id,
				"output_index": 0,
			}
			if err := writeResponsesSSEEvent(w, "response.function_call.completed", completed); err != nil {
				return err
			}
		}
	}

	// 发送 response.completed
	completed := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":    respID,
			"model": model,
		},
	}

	// 🔧 优化：使用标准化的finish_reason
	if finishReason != "" {
		completed["finish_reason"] = finishReason
	} else {
		completed["finish_reason"] = "stop"
	}

	if usage != nil {
		completed["usage"] = map[string]interface{}{
			"input_tokens":  usage.PromptTokens,
			"output_tokens": usage.CompletionTokens,
			"total_tokens":  usage.TotalTokens,
		}
	}

	return writeResponsesSSEEvent(w, "response.completed", completed)
}

// normalizeFinishReason 标准化完成原因
func normalizeFinishReason(reason string) string {
	switch reason {
	case "tool_calls", "function_call":
		return "tool_calls"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "content_filter"
	default:
		return "stop"
	}
}

// writeResponsesSSEEvent 写入单个Responses API格式的SSE事件
func writeResponsesSSEEvent(w io.Writer, event string, payload interface{}) error {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if event != "" {
		if _, err := io.WriteString(w, "event: "); err != nil {
			return err
		}
		if _, err := io.WriteString(w, event); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "data: "); err != nil {
		return err
	}
	if _, err := w.Write(bytes); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n\n"); err != nil {
		return err
	}
	return nil
}
