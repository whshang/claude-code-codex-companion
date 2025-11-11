package conversion

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"claude-code-codex-companion/internal/logger"
)

// OpenAIResponsesFormatAdapter normalises the OpenAI Responses API payloads.
type OpenAIResponsesFormatAdapter struct {
	logger *logger.Logger
}

// NewOpenAIResponsesFormatAdapter constructs a new adapter instance.
func NewOpenAIResponsesFormatAdapter(log *logger.Logger) *OpenAIResponsesFormatAdapter {
	return &OpenAIResponsesFormatAdapter{logger: log}
}

func (o *OpenAIResponsesFormatAdapter) Name() string {
	return "openai_responses"
}

func (o *OpenAIResponsesFormatAdapter) ParseRequestJSON(payload []byte) (*InternalRequest, error) {
	var req OpenAIResponsesRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, NewConversionError("parse_error", fmt.Sprintf("failed to parse OpenAI responses request: %v", err), err)
	}

	internal := &InternalRequest{
		Model:             req.Model,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		MaxOutputTokens:   req.MaxOutputTokens,
		ParallelToolCalls: req.ParallelToolCalls,
		User:              req.User,
		Metadata:          cloneMetadata(req.Metadata),
		PresencePenalty:   req.PresencePenalty,
		FrequencyPenalty:  req.FrequencyPenalty,
		LogitBias:         cloneLogitBias(req.LogitBias),
		N:                 req.N,
		Stop:              append([]string(nil), req.Stop...),
		ResponseFormat:    convertOpenAIResponseFormatToInternal(req.ResponseFormat),
	}

	messages := req.Input
	if len(messages) == 0 {
		messages = req.Messages
	}
	internal.Messages = responsesMessagesToInternal(messages)

	internal.Tools = responsesToolsToInternal(req.Tools)
	internal.ToolChoice = convertOpenAIToolChoice(req.ToolChoice)
	if thinking := InternalThinkingFromOpenAI(req.ReasoningEffort, req.MaxReasoningTokens); thinking != nil {
		internal.Thinking = thinking
	}

	return internal, nil
}

func (o *OpenAIResponsesFormatAdapter) BuildRequestJSON(req *InternalRequest) ([]byte, error) {
	if req == nil {
		return json.Marshal(nil)
	}

	out := OpenAIResponsesRequest{
		Model:             req.Model,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		MaxOutputTokens:   req.MaxOutputTokens,
		ParallelToolCalls: req.ParallelToolCalls,
		User:              req.User,
		Metadata:          cloneMetadata(req.Metadata),
		PresencePenalty:   req.PresencePenalty,
		FrequencyPenalty:  req.FrequencyPenalty,
		LogitBias:         cloneLogitBias(req.LogitBias),
		N:                 req.N,
		Stop:              append([]string(nil), req.Stop...),
		ResponseFormat:    convertInternalResponseFormatToOpenAI(req.ResponseFormat),
	}

	out.Input = internalMessagesToResponses(req.Messages)
	out.Tools = internalToolsToResponses(req.Tools)
	out.ToolChoice = convertInternalToolChoice(req.ToolChoice)
	ApplyInternalThinkingToOpenAI(req, &out, newDefaultThinkingMapper(o.logger))

	return json.Marshal(out)
}

func (o *OpenAIResponsesFormatAdapter) ParseResponseJSON(payload []byte) (*InternalResponse, error) {
	var resp OpenAIResponsesResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, NewConversionError("parse_error", fmt.Sprintf("failed to parse OpenAI responses response: %v", err), err)
	}

	internal := &InternalResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Success: resp.Status == "" || strings.EqualFold(resp.Status, "completed"),
	}

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			internal.Messages = append(internal.Messages, responsesMessageToInternal(item))
		case "function_call":
			internal.Messages = append(internal.Messages, InternalMessage{
				Role: "assistant",
				ToolCalls: []InternalToolCall{{
					ID:        item.ID,
					Type:      "function",
					Name:      item.Name,
					Arguments: item.Arguments,
				}},
			})
		}
	}

	if resp.Usage != nil {
		internal.TokenUsage = &TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
			RecordedAt:       time.Now(),
		}
	}

	return internal, nil
}

func (o *OpenAIResponsesFormatAdapter) BuildResponseJSON(resp *InternalResponse) ([]byte, error) {
	if resp == nil {
		return json.Marshal(nil)
	}

	out := OpenAIResponsesResponse{
		ID:     resp.ID,
		Model:  resp.Model,
		Status: "in_progress",
	}
	if resp.Success {
		out.Status = "completed"
	}

	out.Output = internalMessagesToResponsesOutput(resp.Messages)

	if resp.TokenUsage != nil {
		out.Usage = &OpenAIResponsesUsage{
			InputTokens:  resp.TokenUsage.PromptTokens,
			OutputTokens: resp.TokenUsage.CompletionTokens,
			TotalTokens:  resp.TokenUsage.TotalTokens,
		}
	}

	return json.Marshal(out)
}

func (o *OpenAIResponsesFormatAdapter) ParseSSE(event string, data []byte) ([]InternalEvent, error) {
	if strings.TrimSpace(event) == "" {
		return nil, nil
	}
	var decoded map[string]interface{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &decoded); err != nil {
			decoded = map[string]interface{}{
				"raw": string(data),
			}
		}
	}
	return []InternalEvent{{Type: event, Data: decoded}}, nil
}

func (o *OpenAIResponsesFormatAdapter) BuildSSE(events []InternalEvent) ([]SSEPayload, error) {
	payloads := make([]SSEPayload, 0, len(events))
	for _, ev := range events {
		payloads = append(payloads, SSEPayload{
			Event: ev.Type,
			Data:  ev.Data,
		})
	}
	return payloads, nil
}

// --- helper conversions -----------------------------------------------------

func responsesMessagesToInternal(messages []OpenAIResponsesMessage) []InternalMessage {
	if len(messages) == 0 {
		return nil
	}
	result := make([]InternalMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, responsesMessageToInternal(OpenAIResponsesOutputItem{
			Type:    msg.Type,
			Role:    msg.Role,
			Content: msg.Content,
		}))
	}
	return result
}

func responsesMessageToInternal(item OpenAIResponsesOutputItem) InternalMessage {
	internal := InternalMessage{
		Role: item.Role,
	}

	for _, content := range item.Content {
		switch content.Type {
		case "image":
			internal.Contents = append(internal.Contents, InternalContent{
				Type:     "image",
				ImageURL: content.ImageURL.URL,
			})
		default:
			internal.Contents = append(internal.Contents, InternalContent{
				Type: "text",
				Text: content.Text,
			})
		}
	}

	if item.Type == "function_call" && item.Arguments != "" {
		internal.ToolCalls = append(internal.ToolCalls, InternalToolCall{
			ID:        item.ID,
			Type:      "function",
			Name:      item.Name,
			Arguments: item.Arguments,
		})
		internal.Contents = append(internal.Contents, InternalContent{
			Type: "tool_use",
			ToolUse: &InternalToolUse{
				ID:        item.ID,
				Name:      item.Name,
				Arguments: item.Arguments,
			},
		})
	}

	return internal
}

func responsesToolsToInternal(tools []OpenAIResponsesTool) []InternalTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]InternalTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function == nil {
			continue
		}
		result = append(result, InternalTool{
			Type: "function",
			Function: &InternalToolFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  cloneAnyMap(tool.Function.Parameters),
			},
		})
	}
	return result
}

func internalMessagesToResponses(messages []InternalMessage) []OpenAIResponsesMessage {
	if len(messages) == 0 {
		return nil
	}

	result := make([]OpenAIResponsesMessage, 0, len(messages))
	for _, msg := range messages {
		existingCallIDs := make(map[string]struct{})
		for _, call := range msg.ToolCalls {
			if call.ID != "" {
				existingCallIDs[call.ID] = struct{}{}
			}
		}
		contentItems := make([]OpenAIResponsesContentItem, 0, len(msg.Contents))
		for _, content := range msg.Contents {
			switch content.Type {
			case "image":
				contentItems = append(contentItems, OpenAIResponsesContentItem{
					Type: "image",
					ImageURL: &OpenAIImageURL{
						URL: content.ImageURL,
					},
				})
			default:
				contentItems = append(contentItems, OpenAIResponsesContentItem{
					Type: "input_text",
					Text: content.Text,
				})
			}
		}
		result = append(result, OpenAIResponsesMessage{
			Type:    "message",
			Role:    msg.Role,
			Content: contentItems,
		})
	}
	return result
}

func internalMessagesToResponsesOutput(messages []InternalMessage) []OpenAIResponsesOutputItem {
	if len(messages) == 0 {
		return nil
	}

	result := make([]OpenAIResponsesOutputItem, 0, len(messages))
	for _, msg := range messages {
		contentItems := make([]OpenAIResponsesContentItem, 0, len(msg.Contents))
		for _, content := range msg.Contents {
			switch content.Type {
			case "image":
				contentItems = append(contentItems, OpenAIResponsesContentItem{
					Type:     "image",
					ImageURL: &OpenAIImageURL{URL: content.ImageURL},
				})
			default:
				contentItems = append(contentItems, OpenAIResponsesContentItem{
					Type: "output_text",
					Text: content.Text,
				})
			}
		}
		result = append(result, OpenAIResponsesOutputItem{
			Type:    "message",
			Role:    msg.Role,
			Content: contentItems,
		})

		existingCallIDs := make(map[string]struct{})
		for _, call := range msg.ToolCalls {
			if call.ID != "" {
				existingCallIDs[call.ID] = struct{}{}
			}
			result = append(result, OpenAIResponsesOutputItem{
				Type:      "function_call",
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
			})
		}
		for _, content := range msg.Contents {
			if content.ToolResult != nil {
				text := content.ToolResult.Content
				if text == "" {
					text = content.Text
				}
				result = append(result, OpenAIResponsesOutputItem{
					Type: "tool_result",
					ID:   content.ToolResult.ToolUseID,
					Content: []OpenAIResponsesContentItem{
						{
							Type: "output_text",
							Text: text,
						},
					},
				})
			}
			if content.ToolUse != nil {
				args := content.ToolUse.Arguments
				if args == "" && content.ToolUse.ArgumentsMap != nil {
					if marshalled, err := json.Marshal(content.ToolUse.ArgumentsMap); err == nil {
						args = string(marshalled)
					}
				}
				if _, exists := existingCallIDs[content.ToolUse.ID]; !exists {
					result = append(result, OpenAIResponsesOutputItem{
						Type:      "function_call",
						ID:        content.ToolUse.ID,
						Name:      content.ToolUse.Name,
						Arguments: args,
					})
				}
			}
		}
	}
	return result
}

func internalToolsToResponses(tools []InternalTool) []OpenAIResponsesTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]OpenAIResponsesTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function == nil {
			continue
		}
		result = append(result, OpenAIResponsesTool{
			Type: "function",
			Function: &OpenAIFunctionDef{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  cloneAnyMap(tool.Function.Parameters),
			},
		})
	}
	return result
}
