package conversion

import (
	"fmt"

	"claude-code-codex-companion/internal/logger"
)

// AggregateOpenAIChunks 将 OpenAI SSE 增量聚合为统一内部消息
func AggregateOpenAIChunks(log *logger.Logger, chunks []OpenAIStreamChunk) (*InternalMessage, error) {
	if len(chunks) == 0 {
		return nil, NewConversionError("aggregation_error", "No chunks to aggregate", nil)
	}

	aggregator := NewMessageAggregator(log)
	toolStarted := make(map[int]bool)

	for i, chunk := range chunks {
		if chunk.ID != "" || chunk.Model != "" || i == 0 {
			aggregator.ApplyEvent(InternalEvent{
				Type:      InternalEventMessageStart,
				MessageID: chunk.ID,
				Model:     chunk.Model,
			})
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Role != "" {
				aggregator.ApplyEvent(InternalEvent{
					Type: InternalEventRoleDelta,
					Role: choice.Delta.Role,
				})
			}

			applyOpenAIContentDelta(choice.Delta.Content, aggregator)

			if len(choice.Delta.ToolCalls) > 0 {
				for _, toolCall := range choice.Delta.ToolCalls {
					if !toolStarted[toolCall.Index] {
						aggregator.ApplyEvent(InternalEvent{
							Type:       InternalEventToolStart,
							Index:      toolCall.Index,
							ToolCallID: toolCall.ID,
							ToolName:   toolCall.Function.Name,
						})
						toolStarted[toolCall.Index] = true
					}
					if toolCall.Function.Arguments != "" {
						aggregator.ApplyEvent(InternalEvent{
							Type:           InternalEventToolDelta,
							Index:          toolCall.Index,
							ToolCallID:     toolCall.ID,
							ToolName:       toolCall.Function.Name,
							ArgumentsDelta: toolCall.Function.Arguments,
						})
					}
				}
			}

			if choice.FinishReason != "" {
				aggregator.ApplyEvent(InternalEvent{
					Type:         InternalEventFinish,
					FinishReason: normalizeOpenAIFinishReason(choice.FinishReason),
				})
			}

			if choice.Usage != nil {
				aggregator.ApplyEvent(InternalEvent{
					Type: InternalEventUsage,
					Usage: &InternalUsage{
						InputTokens:  choice.Usage.PromptTokens,
						OutputTokens: choice.Usage.CompletionTokens,
						TotalTokens:  choice.Usage.TotalTokens,
					},
				})
			}
		}

		if chunk.Usage != nil {
			aggregator.ApplyEvent(InternalEvent{
				Type: InternalEventUsage,
				Usage: &InternalUsage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
					TotalTokens:  chunk.Usage.TotalTokens,
				},
			})
		}
	}

	aggregator.ApplyEvent(InternalEvent{Type: InternalEventMessageStop})
	return aggregator.Snapshot(), nil
}

func applyOpenAIContentDelta(content interface{}, aggregator *MessageAggregator) {
	if content == nil {
		return
	}

	switch v := content.(type) {
	case string:
		if v != "" {
			aggregator.ApplyEvent(InternalEvent{
				Type:      InternalEventTextDelta,
				TextDelta: v,
			})
		}
	case []OpenAIMessageContent:
		for _, block := range v {
			switch block.Type {
			case "text":
				if block.Text != "" {
					aggregator.ApplyEvent(InternalEvent{
						Type:      InternalEventTextDelta,
						TextDelta: block.Text,
					})
				}
			case "image_url":
				if block.ImageURL != nil {
					aggregator.ApplyEvent(InternalEvent{
						Type:      InternalEventImage,
						ImageURL:  block.ImageURL.URL,
						MediaType: detectMediaType(block.ImageURL.URL),
					})
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			switch block := item.(type) {
			case string:
				if block != "" {
					aggregator.ApplyEvent(InternalEvent{
						Type:      InternalEventTextDelta,
						TextDelta: block,
					})
				}
			case map[string]interface{}:
				blockType, _ := block["type"].(string)
				switch blockType {
				case "text":
					if text, ok := block["text"].(string); ok && text != "" {
						aggregator.ApplyEvent(InternalEvent{
							Type:      InternalEventTextDelta,
							TextDelta: text,
						})
					}
				case "image_url":
					if imageInfo, ok := block["image_url"].(map[string]interface{}); ok {
						if url, ok := imageInfo["url"].(string); ok && url != "" {
							aggregator.ApplyEvent(InternalEvent{
								Type:      InternalEventImage,
								ImageURL:  url,
								MediaType: detectMediaType(url),
							})
						}
					}
				}
			}
		}
	default:
		// 未识别的内容类型，尝试进行字符串化
		if str := fmt.Sprintf("%v", v); str != "" {
			aggregator.ApplyEvent(InternalEvent{
				Type:      InternalEventTextDelta,
				TextDelta: str,
			})
		}
	}
}

func normalizeOpenAIFinishReason(reason string) string {
	switch reason {
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	case "stop_sequence":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

func detectMediaType(url string) string {
	if url == "" {
		return ""
	}
	if len(url) > 5 && url[:5] == "data:" {
		// data:<media>;base64,...
		for i := 5; i < len(url); i++ {
			if url[i] == ';' || url[i] == ',' {
				return url[5:i]
			}
		}
		return ""
	}
	return ""
}
