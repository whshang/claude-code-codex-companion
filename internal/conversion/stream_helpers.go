package conversion

import (
    "fmt"
    "io"
    "strings"
    "time"
    
    jsonutils "claude-code-codex-companion/internal/common/json"
)

// Removed unused helpers: parseOpenAISSEToMessage, buildResponsesSSE

// buildResponsesSSEFromChunks 直接从OpenAI chunks构建Responses SSE，无需先聚合为消息
func buildResponsesSSEFromChunks(chunks []OpenAIStreamChunk, responseID, model string, usage *OpenAIUsage) ([]byte, error) {
    if len(chunks) == 0 {
        // 容错：上游未产生任何可解析的chunks，仍然构造最小的Responses事件，避免中断流
        if responseID == "" {
            responseID = generateResponseID()
        }
        var builder strings.Builder
        // response.created
        writeSSEEvent(&builder, "response.created", map[string]interface{}{
            "type": "response.created",
            "response": map[string]interface{}{
                "id":    responseID,
                "model": model,
            },
        })
        // 直接完成（无增量）
        completed := map[string]interface{}{
            "type": "response.completed",
            "response": map[string]interface{}{
                "id":    responseID,
                "model": model,
            },
            "finish_reason": "stop",
        }
        writeSSEEvent(&builder, "response.completed", completed)
        return []byte(builder.String()), nil
    }
	if responseID == "" {
		responseID = generateResponseID()
	}
	if model == "" {
		// 从第一个chunk获取模型名
		if len(chunks) > 0 {
			model = chunks[0].Model
		}
	}

	var builder strings.Builder

	// 发送response.created事件
	writeSSEEvent(&builder, "response.created", map[string]interface{}{
		"type": "response.created",
		"response": map[string]interface{}{
			"id":    responseID,
			"model": model,
		},
	})

	// 处理chunks，提取文本和工具调用
	textBuilder := strings.Builder{}
	toolCalls := make(map[int]*struct {
		id        string
		name      string
		arguments strings.Builder
	})
	toolOrder := []int{}
	var finishReason string

	for _, chunk := range chunks {
		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]
			
			// 处理文本内容
			if choice.Delta.Content != nil {
				if contentStr, ok := choice.Delta.Content.(string); ok {
					textBuilder.WriteString(contentStr)
					
					// 发送文本delta事件
					writeSSEEvent(&builder, "response.output_text.delta", map[string]interface{}{
						"type":         "response.output_text.delta",
						"response_id":  responseID,
						"delta":        contentStr,
						"output_index": 0,
					})
				}
			}
			
			// 处理工具调用
			if choice.Delta.ToolCalls != nil {
				for _, toolCall := range choice.Delta.ToolCalls {
					idx := toolCall.Index
					
					if _, exists := toolCalls[idx]; !exists {
						toolCalls[idx] = &struct {
							id        string
							name      string
							arguments strings.Builder
						}{}
						toolOrder = append(toolOrder, idx)
					}
					
					tc := toolCalls[idx]
					
					if toolCall.ID != "" {
						if tc.id == "" {
							tc.id = toolCall.ID
							// 发送工具调用开始事件
							writeSSEEvent(&builder, "response.function_call.started", map[string]interface{}{
								"type":         "response.function_call.started",
								"response_id":  responseID,
								"id":           tc.id,
								"output_index": 0,
							})
						}
					}
					
					if toolCall.Function.Name != "" {
						tc.name = toolCall.Function.Name
						// 发送工具调用名称事件
						writeSSEEvent(&builder, "response.function_call.started", map[string]interface{}{
							"type":         "response.function_call.started",
							"response_id":  responseID,
							"id":           tc.id,
							"name":         tc.name,
							"output_index": 0,
						})
					}
					
					if toolCall.Function.Arguments != "" {
						tc.arguments.WriteString(toolCall.Function.Arguments)
						// 发送参数delta事件
						writeSSEEvent(&builder, "response.function_call_arguments.delta", map[string]interface{}{
							"type":         "response.function_call_arguments.delta",
							"response_id":  responseID,
							"id":           tc.id,
							"delta":        toolCall.Function.Arguments,
							"output_index": 0,
						})
					}
				}
			}
			
			// 记录完成原因
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
	}

	// 为所有工具调用发送完成事件
	for _, idx := range toolOrder {
		tc := toolCalls[idx]
		if tc.id != "" {
			writeSSEEvent(&builder, "response.function_call.completed", map[string]interface{}{
				"type":         "response.function_call.completed",
				"response_id":  responseID,
				"id":           tc.id,
				"output_index": 0,
			})
		}
	}

	// 发送response.completed事件
	completed := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":    responseID,
			"model": model,
		},
		"finish_reason": finishReason,
	}
	
	if usage != nil {
		completed["usage"] = map[string]interface{}{
			"input_tokens":  usage.PromptTokens,
			"output_tokens": usage.CompletionTokens,
			"total_tokens":  usage.TotalTokens,
		}
	}
	
	writeSSEEvent(&builder, "response.completed", completed)

	return []byte(builder.String()), nil
}

// convertResponsesEventToChatChunk 将Responses事件转换为OpenAI Chat chunk
func convertResponsesEventToChatChunk(eventType, dataContent, responseID, model string) (*OpenAIStreamChunk, error) {
	switch eventType {
	case "response.created":
		// 解析response信息
		var event struct {
			Response struct {
				ID    string `json:"id"`
				Model string `json:"model"`
			} `json:"response"`
		}
		if err := jsonutils.SafeUnmarshal([]byte(dataContent), &event); err == nil {
			if event.Response.ID != "" {
				responseID = event.Response.ID
			}
			if event.Response.Model != "" {
				model = event.Response.Model
			}
		}
		
		return &OpenAIStreamChunk{
			ID:    responseID,
			Model: model,
			Choices: []OpenAIStreamChoice{{
				Index: 0,
				Delta: OpenAIMessage{
					Role: "assistant",
				},
				FinishReason: "",
			}},
		}, nil
		
	case "response.output_text.delta":
		var event struct {
			Delta string `json:"delta"`
		}
		if err := jsonutils.SafeUnmarshal([]byte(dataContent), &event); err != nil {
			return nil, err
		}
		
		return &OpenAIStreamChunk{
			ID:    responseID,
			Model: model,
			Choices: []OpenAIStreamChoice{{
				Index: 0,
				Delta: OpenAIMessage{
					Content: event.Delta,
				},
				FinishReason: "",
			}},
		}, nil
		
	case "response.function_call.started":
		var event struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := jsonutils.SafeUnmarshal([]byte(dataContent), &event); err != nil {
			return nil, err
		}
		
		return &OpenAIStreamChunk{
			ID:    responseID,
			Model: model,
			Choices: []OpenAIStreamChoice{{
				Index: 0,
				Delta: OpenAIMessage{
					ToolCalls: []OpenAIToolCall{{
						Index: 0,
						ID:    event.ID,
						Function: OpenAIToolCallDetail{
							Name: event.Name,
						},
					}},
				},
				FinishReason: "",
			}},
		}, nil
		
	case "response.function_call_arguments.delta":
		var event struct {
			ID    string `json:"id"`
			Delta string `json:"delta"`
		}
		if err := jsonutils.SafeUnmarshal([]byte(dataContent), &event); err != nil {
			return nil, err
		}
		
		return &OpenAIStreamChunk{
			ID:    responseID,
			Model: model,
			Choices: []OpenAIStreamChoice{{
				Index: 0,
				Delta: OpenAIMessage{
					ToolCalls: []OpenAIToolCall{{
						Index: 0,
						ID:    event.ID,
						Function: OpenAIToolCallDetail{
							Arguments: event.Delta,
						},
					}},
				},
				FinishReason: "",
			}},
		}, nil
		
	case "response.completed":
		var event struct {
			FinishReason string `json:"finish_reason"`
			Usage        struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := jsonutils.SafeUnmarshal([]byte(dataContent), &event); err != nil {
			return nil, err
		}
		
		usage := &OpenAIUsage{
			PromptTokens:     event.Usage.InputTokens,
			CompletionTokens: event.Usage.OutputTokens,
			TotalTokens:      event.Usage.TotalTokens,
		}
		
		return &OpenAIStreamChunk{
			ID:    responseID,
			Model: model,
			Choices: []OpenAIStreamChoice{{
				Index:        0,
				Delta:        OpenAIMessage{},
				FinishReason: event.FinishReason,
			}},
			Usage: usage,
		}, nil
	}
	
	return nil, nil // 忽略其他事件类型
}

// buildChatSSEFromChunks 从OpenAI chunks构建Chat格式的SSE流
func buildChatSSEFromChunks(chunks []OpenAIStreamChunk, w io.Writer) error {
	for _, chunk := range chunks {
		data, err := jsonutils.SafeMarshal(chunk)
		if err != nil {
			continue
		}
		
		if _, err := fmt.Fprintf(w, "data: %s\n\n", string(data)); err != nil {
			return err
		}
	}
	
	// 写入结束标记
	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	
	return nil
}

// Removed unused helper: parseResponsesSSE

// Removed unused helper: buildChatSSE

func writeSSEEvent(builder *strings.Builder, event string, payload interface{}) {
	bytes, err := jsonutils.SafeMarshal(payload)
	if err != nil {
		return
	}
	if event != "" {
		builder.WriteString("event: ")
		builder.WriteString(event)
		builder.WriteString("\n")
	}
	builder.WriteString("data: ")
	builder.Write(bytes)
	builder.WriteString("\n\n")
}

func writeChatChunk(builder *strings.Builder, payload map[string]interface{}) {
	bytes, err := jsonutils.SafeMarshal(payload)
	if err != nil {
		return
	}
	builder.WriteString("data: ")
	builder.Write(bytes)
	builder.WriteString("\n\n")
}

// Removed unused helper: bufioNewScanner

func generateResponseID() string {
	return fmt.Sprintf("resp_%d", time.Now().UnixNano())
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// Removed unused helper: convertAnthropicSSEToOpenAISSE

// Removed unused helper: readAll
