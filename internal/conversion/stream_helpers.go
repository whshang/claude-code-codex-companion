package conversion

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"claude-code-codex-companion/internal/logger"
)

func parseOpenAISSEToMessage(data []byte) (*InternalMessage, error) {
	parser := NewSSEParser(nil)
	chunks, err := parser.ParseSSEStream(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI SSE: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("empty OpenAI SSE stream")
	}

	msg, err := AggregateOpenAIChunks((*logger.Logger)(nil), chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate OpenAI chunks: %w", err)
	}
	return msg, nil
}

func buildResponsesSSE(msg *InternalMessage, responseID, model string, usage *InternalUsage) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil message")
	}
	if responseID == "" {
		responseID = generateResponseID()
	}
	if model == "" {
		model = msg.Model
	}

	var builder strings.Builder

	writeSSEEvent(&builder, "response.created", map[string]interface{}{
		"type": "response.created",
		"response": map[string]interface{}{
			"id":    responseID,
			"model": model,
		},
	})

	for _, content := range msg.Contents {
		switch content.Type {
		case "text":
			if content.Text != "" {
				writeSSEEvent(&builder, "response.output_text.delta", map[string]interface{}{
					"type":         "response.output_text.delta",
					"response_id":  responseID,
					"delta":        content.Text,
					"output_index": 0,
				})
			}
		}
	}

	for idx, call := range msg.ToolCalls {
		callID := call.ID
		if callID == "" {
			callID = fmt.Sprintf("tool_call_%d", idx)
		}
		writeSSEEvent(&builder, "response.function_call.started", map[string]interface{}{
			"type":         "response.function_call.started",
			"response_id":  responseID,
			"id":           callID,
			"name":         call.Name,
			"output_index": 0,
		})
		if call.Arguments != "" {
			writeSSEEvent(&builder, "response.function_call_arguments.delta", map[string]interface{}{
				"type":         "response.function_call_arguments.delta",
				"response_id":  responseID,
				"id":           callID,
				"delta":        call.Arguments,
				"output_index": 0,
			})
		}
		writeSSEEvent(&builder, "response.function_call.completed", map[string]interface{}{
			"type":         "response.function_call.completed",
			"response_id":  responseID,
			"id":           callID,
			"output_index": 0,
		})
	}

	completed := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":    responseID,
			"model": model,
		},
		"finish_reason": msg.FinishReason,
	}
	if usage != nil {
		completed["usage"] = map[string]interface{}{
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
			"total_tokens":  usage.TotalTokens,
		}
	}
	writeSSEEvent(&builder, "response.completed", completed)

	return []byte(builder.String()), nil
}

// buildResponsesSSEFromChunks 直接从OpenAI chunks构建Responses SSE，无需先聚合为消息
func buildResponsesSSEFromChunks(chunks []OpenAIStreamChunk, responseID, model string, usage *OpenAIUsage) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks provided")
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
		if err := json.Unmarshal([]byte(dataContent), &event); err == nil {
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
		if err := json.Unmarshal([]byte(dataContent), &event); err != nil {
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
		if err := json.Unmarshal([]byte(dataContent), &event); err != nil {
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
		if err := json.Unmarshal([]byte(dataContent), &event); err != nil {
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
		if err := json.Unmarshal([]byte(dataContent), &event); err != nil {
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
		data, err := json.Marshal(chunk)
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

func parseResponsesSSE(data []byte) (*InternalMessage, string, string, *InternalUsage, error) {
	scanner := bufioNewScanner(data)
	var currentEvent string
	responseID := ""
	model := ""
	finishReason := "end_turn"
	textBuilder := strings.Builder{}
	toolCalls := make(map[string]*InternalToolCall)
	var toolOrder []string
	usage := &InternalUsage{}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if payload == "[DONE]" {
			continue
		}

		var dataMap map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &dataMap); err != nil {
			continue
		}

		if currentEvent == "" {
			if eventType, ok := dataMap["type"].(string); ok {
				currentEvent = eventType
			}
		}

		switch currentEvent {
		case "response.created":
			if resp, ok := dataMap["response"].(map[string]interface{}); ok {
				if id, ok := resp["id"].(string); ok {
					responseID = id
				}
				if m, ok := resp["model"].(string); ok {
					model = m
				}
			}
		case "response.output_text.delta":
			if delta, ok := dataMap["delta"].(string); ok {
				textBuilder.WriteString(delta)
			}
		case "response.function_call.started":
			id, _ := dataMap["id"].(string)
			name, _ := dataMap["name"].(string)
			if id == "" {
				id = fmt.Sprintf("tool_call_%d", len(toolCalls))
			}
			toolCalls[id] = &InternalToolCall{
				ID:   id,
				Name: name,
			}
			toolOrder = append(toolOrder, id)
		case "response.function_call_arguments.delta":
			id, _ := dataMap["id"].(string)
			if call, ok := toolCalls[id]; ok {
				if delta, ok := dataMap["delta"].(string); ok {
					call.Arguments += delta
				}
			}
		case "response.completed":
			if reason, ok := dataMap["finish_reason"].(string); ok && reason != "" {
				finishReason = reason
			}
			if usageMap, ok := dataMap["usage"].(map[string]interface{}); ok {
				if v, ok := usageMap["input_tokens"].(float64); ok {
					usage.InputTokens = int(v)
				}
				if v, ok := usageMap["output_tokens"].(float64); ok {
					usage.OutputTokens = int(v)
				}
				if v, ok := usageMap["total_tokens"].(float64); ok {
					usage.TotalTokens = int(v)
				}
			}
		}
		currentEvent = ""
	}

	if err := scanner.Err(); err != nil {
		return nil, "", "", nil, err
	}

	msg := &InternalMessage{
		Role:         "assistant",
		Contents:     []InternalContent{},
		ToolCalls:    []InternalToolCall{},
		FinishReason: finishReason,
	}

	if textBuilder.Len() > 0 {
		msg.Contents = append(msg.Contents, InternalContent{Type: "text", Text: textBuilder.String()})
	}

	for _, id := range toolOrder {
		if call := toolCalls[id]; call != nil {
			msg.ToolCalls = append(msg.ToolCalls, *call)
		}
	}

	return msg, responseID, model, usage, nil
}

func buildChatSSE(msg *InternalMessage, completionID, model string, usage *InternalUsage) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil message")
	}
	if completionID == "" {
		completionID = generateMessageID()
	}
	if model == "" {
		model = msg.Model
	}

	createdAt := time.Now().Unix()

	var builder strings.Builder

	initial := map[string]interface{}{
		"id":      completionID,
		"object":  "chat.completion.chunk",
		"created": createdAt,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"role": msg.Role,
				},
				"finish_reason": nil,
			},
		},
	}
	writeChatChunk(&builder, initial)

	for _, content := range msg.Contents {
		switch content.Type {
		case "text":
			if content.Text != "" {
				writeChatChunk(&builder, map[string]interface{}{
					"id":      completionID,
					"object":  "chat.completion.chunk",
					"created": createdAt,
					"model":   model,
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]interface{}{
								"content": content.Text,
							},
							"finish_reason": nil,
						},
					},
				})
			}
		}
	}

	for idx, call := range msg.ToolCalls {
		callID := call.ID
		if callID == "" {
			callID = fmt.Sprintf("tool_call_%d", idx)
		}
		writeChatChunk(&builder, map[string]interface{}{
			"id":      completionID,
			"object":  "chat.completion.chunk",
			"created": createdAt,
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{
							{
								"index": idx,
								"id":    callID,
								"type":  "function",
								"function": map[string]interface{}{
									"name":      call.Name,
									"arguments": call.Arguments,
								},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		})
	}

	finalChunk := map[string]interface{}{
		"id":      completionID,
		"object":  "chat.completion.chunk",
		"created": createdAt,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": denormalizeOpenAIFinishReason(msg.FinishReason),
			},
		},
	}
	if usage != nil {
		finalChunk["usage"] = map[string]interface{}{
			"prompt_tokens":     usage.InputTokens,
			"completion_tokens": usage.OutputTokens,
			"total_tokens":      totalTokensFallback(usage),
		}
	}
	writeChatChunk(&builder, finalChunk)
	builder.WriteString("data: [DONE]\n\n")
	return []byte(builder.String()), nil
}

func writeSSEEvent(builder *strings.Builder, event string, payload interface{}) {
	bytes, err := json.Marshal(payload)
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
	bytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	builder.WriteString("data: ")
	builder.Write(bytes)
	builder.WriteString("\n\n")
}

func bufioNewScanner(data []byte) *bufio.Scanner {
	reader := bytes.NewReader(data)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)
	return scanner
}

func generateResponseID() string {
	return fmt.Sprintf("resp_%d", time.Now().UnixNano())
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

func convertAnthropicSSEToOpenAISSE(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := StreamAnthropicSSEToOpenAI(bytes.NewReader(data), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
