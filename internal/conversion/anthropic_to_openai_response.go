package conversion

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ConvertAnthropicResponseJSONToOpenAI 将 Anthropic 非流式响应 JSON 转换为 OpenAI Chat Completions JSON。
func ConvertAnthropicResponseJSONToOpenAI(body []byte) ([]byte, error) {
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	openaiResp, err := buildOpenAIResponseFromAnthropic(&anthropicResp)
	if err != nil {
		return nil, err
	}

	converted, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI response: %w", err)
	}

	return converted, nil
}

func buildOpenAIResponseFromAnthropic(anth *AnthropicResponse) (*OpenAIResponse, error) {
	choiceMessage := OpenAIMessage{Role: anth.Role}

	var contents []OpenAIMessageContent
	var toolCalls []OpenAIToolCall

	for _, block := range anth.Content {
		switch block.Type {
		case "text":
			contents = append(contents, OpenAIMessageContent{Type: "text", Text: block.Text})
		case "tool_use":
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   block.ID,
				Type: "function",
				Function: OpenAIToolCallDetail{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		case "image":
			if block.Source != nil {
				contents = append(contents, OpenAIMessageContent{
					Type: "image_url",
					ImageURL: &OpenAIImageURL{
						URL: fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data),
					},
				})
			}
		}
	}

	if len(contents) == 1 && contents[0].Type == "text" {
		// 兼容旧版字符串格式
		choiceMessage.Content = contents[0].Text
	} else if len(contents) > 0 {
		choiceMessage.Content = contents
	}

	if len(toolCalls) > 0 {
		choiceMessage.ToolCalls = toolCalls
	}

	usage := &OpenAIUsage{}
	if anth.Usage != nil {
		usage = &OpenAIUsage{
			PromptTokens:     anth.Usage.InputTokens,
			CompletionTokens: anth.Usage.OutputTokens,
			TotalTokens:      anth.Usage.InputTokens + anth.Usage.OutputTokens,
		}
	}

	finish := mapAnthropicStopReason(anth.StopReason)

	openaiResp := &OpenAIResponse{
		ID:    anth.ID,
		Model: anth.Model,
		Choices: []OpenAIChoice{
			{
				Index:        0,
				FinishReason: finish,
				Message:      choiceMessage,
			},
		},
		Usage: usage,
	}

	return openaiResp, nil
}

func mapAnthropicStopReason(reason string) string {
	switch reason {
	case "":
		return ""
	case "tool_use", "tool_result":
		return "tool_calls"
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	default:
		return reason
	}
}

// ConvertAnthropicSSEToOpenAI 将 Anthropic SSE 转换为 OpenAI SSE（最少一段 chunk）。
func ConvertAnthropicSSEToOpenAI(body []byte) ([]byte, error) {
	agg, err := aggregateAnthropicSSE(body)
	if err != nil {
		return nil, err
	}

	sse := buildOpenAISSEFromAggregate(agg)
	return []byte(sse), nil
}

type anthropicSSEAggregate struct {
	MessageID    string
	Model        string
	TextBuilder  strings.Builder
	ToolCalls    []OpenAIToolCall
	FinishReason string
	Usage        *OpenAIUsage
	RoleEmitted  bool
}

func aggregateAnthropicSSE(body []byte) (*anthropicSSEAggregate, error) {
	aggregate := &anthropicSSEAggregate{}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Split(bufio.ScanLines)

	var currentEvent string
	var dataBuilder strings.Builder

	flush := func() error {
		if currentEvent == "" {
			return nil
		}
		data := strings.TrimSpace(dataBuilder.String())
		dataBuilder.Reset()
		if data == "" {
			currentEvent = ""
			return nil
		}

		switch currentEvent {
		case "message_start":
			var payload struct {
				Message struct {
					ID    string          `json:"id"`
					Model string          `json:"model"`
					Usage *AnthropicUsage `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return fmt.Errorf("failed to parse message_start: %w", err)
			}
			aggregate.MessageID = payload.Message.ID
			aggregate.Model = payload.Message.Model
			if payload.Message.Usage != nil {
				aggregate.Usage = &OpenAIUsage{
					PromptTokens: payload.Message.Usage.InputTokens,
				}
			}

		case "content_block_delta":
			var payload struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				ContentBlock struct {
					Type  string          `json:"type"`
					ID    string          `json:"id"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content_block"`
			}
			// 某些事件没有 content_block 字段，忽略解析错误
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				if strings.ToLower(payload.Delta.Type) == "text_delta" {
					aggregate.TextBuilder.WriteString(payload.Delta.Text)
				}
			}

		case "content_block_start":
			var payload struct {
				ContentBlock struct {
					Type  string          `json:"type"`
					ID    string          `json:"id"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				if payload.ContentBlock.Type == "tool_use" {
					aggregate.ToolCalls = append(aggregate.ToolCalls, OpenAIToolCall{
						ID:   payload.ContentBlock.ID,
						Type: "function",
						Function: OpenAIToolCallDetail{
							Name:      payload.ContentBlock.Name,
							Arguments: string(payload.ContentBlock.Input),
						},
					})
				}
			}

		case "message_delta":
			var payload struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage *AnthropicUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				if payload.Delta.StopReason != "" {
					aggregate.FinishReason = mapAnthropicStopReason(payload.Delta.StopReason)
				}
				if payload.Usage != nil {
					if aggregate.Usage == nil {
						aggregate.Usage = &OpenAIUsage{}
					}
					aggregate.Usage.PromptTokens = payload.Usage.InputTokens
					aggregate.Usage.CompletionTokens = payload.Usage.OutputTokens
					aggregate.Usage.TotalTokens = payload.Usage.InputTokens + payload.Usage.OutputTokens
				}
			}

		case "message_stop":
			var payload struct {
				StopReason string `json:"stop_reason"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				if payload.StopReason != "" {
					aggregate.FinishReason = mapAnthropicStopReason(payload.StopReason)
				}
			}
		}

		currentEvent = ""
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			if err := flush(); err != nil {
				return nil, err
			}
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		} else if strings.HasPrefix(line, "data: ") {
			if dataBuilder.Len() > 0 {
				dataBuilder.WriteByte('\n')
			}
			dataBuilder.WriteString(strings.TrimPrefix(line, "data: "))
		} else if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}

	return aggregate, nil
}

func buildOpenAISSEFromAggregate(agg *anthropicSSEAggregate) string {
	created := time.Now().Unix()

	// 第一块，包含 role 与文本内容
	delta := map[string]interface{}{
		"role": "assistant",
	}
	if text := agg.TextBuilder.String(); text != "" {
		delta["content"] = []map[string]interface{}{{
			"type": "text",
			"text": text,
		}}
	}

	chunk := map[string]interface{}{
		"id":      agg.MessageID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   agg.Model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         delta,
				"finish_reason": nil,
			},
		},
	}

	if len(agg.ToolCalls) > 0 {
		toolChunks := make([]map[string]interface{}, 0, len(agg.ToolCalls))
		for _, call := range agg.ToolCalls {
			toolChunks = append(toolChunks, map[string]interface{}{
				"index": call.Index,
				"id":    call.ID,
				"type":  call.Type,
				"function": map[string]interface{}{
					"name":      call.Function.Name,
					"arguments": call.Function.Arguments,
				},
			})
		}
		chunk["choices"].([]map[string]interface{})[0]["delta"].(map[string]interface{})["tool_calls"] = toolChunks
	}

	firstJSON, _ := json.Marshal(chunk)

	// 结束块
	finalChunk := map[string]interface{}{
		"id":      agg.MessageID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   agg.Model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": agg.FinishReason,
			},
		},
	}

	finalJSON, _ := json.Marshal(finalChunk)

	var builder strings.Builder
	builder.WriteString("data: ")
	builder.WriteString(string(firstJSON))
	builder.WriteString("\n\n")
	builder.WriteString("data: ")
	builder.WriteString(string(finalJSON))
	builder.WriteString("\n\n")
	builder.WriteString("data: [DONE]\n\n")

	return builder.String()
}
