package conversion

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"claude-code-codex-companion/internal/logger"
)

// OpenAIChatFormatAdapter normalises OpenAI Chat Completions payloads.
type OpenAIChatFormatAdapter struct {
	logger *logger.Logger
}

// NewOpenAIChatFormatAdapter creates a new adapter instance.
func NewOpenAIChatFormatAdapter(log *logger.Logger) *OpenAIChatFormatAdapter {
	return &OpenAIChatFormatAdapter{logger: log}
}

func (o *OpenAIChatFormatAdapter) Name() string {
	return "openai_chat"
}

func (o *OpenAIChatFormatAdapter) ParseRequestJSON(payload []byte) (*InternalRequest, error) {
	var req OpenAIRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, NewConversionError("parse_error", fmt.Sprintf("failed to parse OpenAI request: %v", err), err)
	}

	stream := false
	if req.Stream != nil {
		stream = *req.Stream
	}

	internal := &InternalRequest{
		Model:               req.Model,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		MaxCompletionTokens: req.MaxCompletionTokens,
		MaxOutputTokens:     req.MaxOutputTokens,
		MaxTokens:           req.MaxTokens,
		Stream:              stream,
		Stop:                append([]string(nil), req.Stop...),
		User:                req.User,
		ParallelToolCalls:   req.ParallelToolCalls,
		PresencePenalty:     req.PresencePenalty,
		FrequencyPenalty:    req.FrequencyPenalty,
		LogitBias:           cloneLogitBias(req.LogitBias),
		N:                   req.N,
		ResponseFormat:      convertOpenAIResponseFormatToInternal(req.ResponseFormat),
		ReasoningEffort:     req.ReasoningEffort,
		MaxReasoningTokens:  req.MaxReasoningTokens,
	}

	internal.Messages = openAIMessagesToInternal(req.Messages)
	internal.Tools = openAIToolsToInternal(req.Tools)
	internal.ToolChoice = convertOpenAIToolChoice(req.ToolChoice)
	if thinking := InternalThinkingFromOpenAI(req.ReasoningEffort, req.MaxReasoningTokens); thinking != nil {
		internal.Thinking = thinking
	}

	return internal, nil
}

func (o *OpenAIChatFormatAdapter) BuildRequestJSON(req *InternalRequest) ([]byte, error) {
	if req == nil {
		return json.Marshal(nil)
	}

	out := OpenAIRequest{
		Model:               req.Model,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		MaxCompletionTokens: req.MaxCompletionTokens,
		MaxOutputTokens:     req.MaxOutputTokens,
		MaxTokens:           req.MaxTokens,
		Stop:                append([]string(nil), req.Stop...),
		User:                req.User,
		ParallelToolCalls:   req.ParallelToolCalls,
		PresencePenalty:     req.PresencePenalty,
		FrequencyPenalty:    req.FrequencyPenalty,
		LogitBias:           cloneLogitBias(req.LogitBias),
		N:                   req.N,
		ResponseFormat:      convertInternalResponseFormatToOpenAI(req.ResponseFormat),
		ReasoningEffort:     req.ReasoningEffort,
		MaxReasoningTokens:  req.MaxReasoningTokens,
	}

	if req.Stream {
		out.Stream = ptrBool(true)
	}

	out.Messages = internalMessagesToOpenAI(req.Messages)
	out.Tools = internalToolsToOpenAI(req.Tools)
	out.ToolChoice = convertInternalToolChoice(req.ToolChoice)
	ApplyInternalThinkingToOpenAI(req, &out, newDefaultThinkingMapper(o.logger))

	return json.Marshal(out)
}

func (o *OpenAIChatFormatAdapter) ParseResponseJSON(payload []byte) (*InternalResponse, error) {
	var resp OpenAIResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, NewConversionError("parse_error", fmt.Sprintf("failed to parse OpenAI response: %v", err), err)
	}

	internal := &InternalResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Success: true,
	}

	if resp.Usage != nil {
		internal.TokenUsage = &TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
			RecordedAt:       time.Now(),
		}
	}

	if len(resp.Choices) > 0 {
		internal.FinishReason = resp.Choices[0].FinishReason
		for _, choice := range resp.Choices {
			internal.Messages = append(internal.Messages, openAIMessageToInternal(choice.Message))
		}
	}

	return internal, nil
}

func (o *OpenAIChatFormatAdapter) BuildResponseJSON(resp *InternalResponse) ([]byte, error) {
	if resp == nil {
		return json.Marshal(nil)
	}

	out := OpenAIResponse{
		ID:    resp.ID,
		Model: resp.Model,
	}

	if resp.TokenUsage != nil {
		out.Usage = &OpenAIUsage{
			PromptTokens:     resp.TokenUsage.PromptTokens,
			CompletionTokens: resp.TokenUsage.CompletionTokens,
			TotalTokens:      resp.TokenUsage.TotalTokens,
		}
	}

	if len(resp.Messages) == 0 && len(resp.Content) > 0 {
		// synthesise assistant message from content blocks
		resp.Messages = append(resp.Messages, InternalMessage{
			Role:     "assistant",
			Contents: resp.Content,
		})
	}

	for idx, msg := range resp.Messages {
		out.Choices = append(out.Choices, OpenAIChoice{
			Index:        idx,
			FinishReason: resp.FinishReason,
			Message:      internalMessageToOpenAI(msg),
		})
	}

	return json.Marshal(out)
}

func (o *OpenAIChatFormatAdapter) ParseSSE(event string, data []byte) ([]InternalEvent, error) {
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

func (o *OpenAIChatFormatAdapter) BuildSSE(events []InternalEvent) ([]SSEPayload, error) {
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

func openAIMessagesToInternal(messages []OpenAIMessage) []InternalMessage {
	result := make([]InternalMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, openAIMessageToInternal(msg))
	}
	return result
}

func openAIMessageToInternal(msg OpenAIMessage) InternalMessage {
	internal := InternalMessage{
		Role:       msg.Role,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
	}

	// content can be string or []OpenAIMessageContent
	switch content := msg.Content.(type) {
	case string:
		if msg.Role == "tool" {
			internal.Contents = append(internal.Contents, InternalContent{
				Type: "tool_result",
				Text: content,
				ToolResult: &InternalToolResult{
					ToolUseID: msg.ToolCallID,
					Content:   content,
				},
			})
		} else {
			internal.Contents = append(internal.Contents, InternalContent{
				Type: "text",
				Text: content,
			})
		}
	case []interface{}:
		for _, raw := range content {
			if part, ok := raw.(map[string]interface{}); ok {
				if partType, _ := part["type"].(string); partType == "image_url" {
					if urlMap, ok := part["image_url"].(map[string]interface{}); ok {
						if urlStr, _ := urlMap["url"].(string); urlStr != "" {
							internal.Contents = append(internal.Contents, InternalContent{
								Type:     "image",
								ImageURL: urlStr,
							})
						}
					}
				} else if text, _ := part["text"].(string); text != "" {
					internal.Contents = append(internal.Contents, InternalContent{
						Type: "text",
						Text: text,
					})
				}
			}
		}
	case []OpenAIMessageContent:
		for _, part := range content {
			switch part.Type {
			case "image_url":
				if part.ImageURL != nil {
					internal.Contents = append(internal.Contents, InternalContent{
						Type:     "image",
						ImageURL: part.ImageURL.URL,
					})
				}
			default:
				internal.Contents = append(internal.Contents, InternalContent{
					Type: "text",
					Text: part.Text,
				})
			}
		}
	}

	for _, call := range msg.ToolCalls {
		argsMap := map[string]interface{}{}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &argsMap); err != nil {
			argsMap = nil
		}
		internal.ToolCalls = append(internal.ToolCalls, InternalToolCall{
			ID:        call.ID,
			Type:      call.Type,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
			Index:     call.Index,
			Metadata:  map[string]interface{}{},
		})
		internal.Contents = append(internal.Contents, InternalContent{
			Type: "tool_use",
			ToolUse: &InternalToolUse{
				ID:           call.ID,
				Name:         call.Function.Name,
				Arguments:    call.Function.Arguments,
				ArgumentsMap: argsMap,
				Index:        call.Index,
			},
		})
	}

	return internal
}

func internalMessagesToOpenAI(messages []InternalMessage) []OpenAIMessage {
	result := make([]OpenAIMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, internalMessageToOpenAI(msg))
	}
	return result
}

func internalMessageToOpenAI(msg InternalMessage) OpenAIMessage {
	out := OpenAIMessage{
		Role:       msg.Role,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
	}

	contentParts := make([]OpenAIMessageContent, 0, len(msg.Contents))
	hasImage := false
	var textBuilder strings.Builder

	for _, content := range msg.Contents {
		switch content.Type {
		case "image":
			hasImage = true
			contentParts = append(contentParts, OpenAIMessageContent{
				Type: "image_url",
				ImageURL: &OpenAIImageURL{
					URL: content.ImageURL,
				},
			})
		case "tool_result":
			textBuilder.WriteString(content.Text)
		default:
			textBuilder.WriteString(content.Text)
		}
	}

	if hasImage {
		text := strings.TrimSpace(textBuilder.String())
		if text != "" {
			contentParts = append(contentParts, OpenAIMessageContent{
				Type: "text",
				Text: text,
			})
		}
		out.Content = contentParts
	} else {
		out.Content = textBuilder.String()
	}

	handledToolCalls := len(msg.ToolCalls) > 0
	if handledToolCalls {
		for idx, call := range msg.ToolCalls {
			args := call.Arguments
			if args == "" && call.Metadata != nil {
				if raw, ok := call.Metadata["arguments_map"]; ok {
					if bytes, err := json.Marshal(raw); err == nil {
						args = string(bytes)
					}
				}
			}
			out.ToolCalls = append(out.ToolCalls, OpenAIToolCall{
				Index: idx,
				ID:    call.ID,
				Type:  "function",
				Function: OpenAIToolCallDetail{
					Name:      call.Name,
					Arguments: args,
				},
			})
		}
	} else {
		for idx, content := range msg.Contents {
			if content.ToolUse == nil {
				continue
			}
			args := content.ToolUse.Arguments
			if args == "" && content.ToolUse.ArgumentsMap != nil {
				if bytes, err := json.Marshal(content.ToolUse.ArgumentsMap); err == nil {
					args = string(bytes)
				}
			}
			out.ToolCalls = append(out.ToolCalls, OpenAIToolCall{
				Index: idx,
				ID:    content.ToolUse.ID,
				Type:  "function",
				Function: OpenAIToolCallDetail{
					Name:      content.ToolUse.Name,
					Arguments: args,
				},
			})
		}
		handledToolCalls = len(out.ToolCalls) > 0
	}

	if handledToolCalls && out.Content == nil && msg.Role == "assistant" {
		out.Content = ""
	}

	return out
}

func openAIToolsToInternal(tools []OpenAITool) []InternalTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]InternalTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, InternalTool{
			Type: tool.Type,
			Function: &InternalToolFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  cloneAnyMap(tool.Function.Parameters),
			},
		})
	}
	return result
}

func internalToolsToOpenAI(tools []InternalTool) []OpenAITool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]OpenAITool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function == nil {
			continue
		}
		result = append(result, OpenAITool{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  cloneAnyMap(tool.Function.Parameters),
			},
		})
	}
	return result
}

func convertOpenAIToolChoice(choice interface{}) *InternalToolChoice {
	switch v := choice.(type) {
	case string:
		if v == "" {
			return nil
		}
		return &InternalToolChoice{Type: v}
	case map[string]interface{}:
		t, _ := v["type"].(string)
		if t == "" {
			return nil
		}
		internal := &InternalToolChoice{Type: t}
		if fn, ok := v["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				internal.FunctionName = name
			}
		}
		return internal
	default:
		return nil
	}
}

func convertInternalToolChoice(choice *InternalToolChoice) interface{} {
	if choice == nil {
		return nil
	}
	if choice.Type == "function" && choice.FunctionName != "" {
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": choice.FunctionName,
			},
		}
	}
	return choice.Type
}

func cloneLogitBias(bias map[string]float64) map[string]float64 {
	if bias == nil {
		return nil
	}
	result := make(map[string]float64, len(bias))
	for k, v := range bias {
		result[k] = v
	}
	return result
}

func convertOpenAIResponseFormatToInternal(rf *OpenAIResponseFormat) *InternalResponseFormat {
	if rf == nil {
		return nil
	}
	return &InternalResponseFormat{
		Type:   rf.Type,
		Schema: cloneAnyMap(rf.Schema),
	}
}

func convertInternalResponseFormatToOpenAI(rf *InternalResponseFormat) *OpenAIResponseFormat {
	if rf == nil {
		return nil
	}
	return &OpenAIResponseFormat{
		Type:   rf.Type,
		Schema: cloneAnyMap(rf.Schema),
	}
}
