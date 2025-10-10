package conversion

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	var buf bytes.Buffer
	if err := StreamAnthropicSSEToOpenAI(bytes.NewReader(body), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
