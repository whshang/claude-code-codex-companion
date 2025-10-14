package conversion

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// StreamChatToResponsesRealtime 实时流式转换：边读边写，避免缓冲整个流
func StreamChatToResponsesRealtime(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	respID := generateResponseID()
	var model string
	var usage *OpenAIUsage
	sentCreated := false

	// 实时流式转换
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 处理data行
		if strings.HasPrefix(line, "data:") {
			dataContent := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			// 检查结束标记
			if dataContent == "[DONE]" {
				break
			}

			// 解析JSON chunk
			var chunk OpenAIStreamChunk
			if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
				continue // 跳过无效chunk
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

				// 处理文本内容（处理 content 字段，无论是 nil, "", 还是实际内容）
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

				// 处理工具调用
				if choice.Delta.ToolCalls != nil {
					for _, toolCall := range choice.Delta.ToolCalls {
						// 发送工具调用开始事件
						if toolCall.ID != "" || toolCall.Function.Name != "" {
							start := map[string]interface{}{
								"type":         "response.function_call.started",
								"response_id":  respID,
								"id":           toolCall.ID,
								"output_index": 0,
							}
							if toolCall.Function.Name != "" {
								start["name"] = toolCall.Function.Name
							}
							if err := writeResponsesSSEEvent(w, "response.function_call.started", start); err != nil {
								return err
							}
						}

						// 发送参数delta
						if toolCall.Function.Arguments != "" {
							argDelta := map[string]interface{}{
								"type":         "response.function_call_arguments.delta",
								"response_id":  respID,
								"id":           toolCall.ID,
								"delta":        toolCall.Function.Arguments,
								"output_index": 0,
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

	// 发送 response.completed
	completed := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":    respID,
			"model": model,
		},
		"finish_reason": "stop",
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
