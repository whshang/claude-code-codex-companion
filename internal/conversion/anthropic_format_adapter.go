package conversion

import (
	"encoding/json"
	"fmt"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// AnthropicFormatAdapter normalises Anthropic JSON payloads to the internal
// representation used by the proxy and renders internal structures back to
// Anthropic specific JSON when forwarding requests downstream.
type AnthropicFormatAdapter struct {
	logger *logger.Logger
}

// NewAnthropicFormatAdapter constructs a new adapter instance.
func NewAnthropicFormatAdapter(log *logger.Logger) *AnthropicFormatAdapter {
	return &AnthropicFormatAdapter{logger: log}
}

func (a *AnthropicFormatAdapter) Name() string {
	return "anthropic"
}

// ParseRequestJSON converts an Anthropic request JSON payload into the
// provider agnostic InternalRequest structure.
func (a *AnthropicFormatAdapter) ParseRequestJSON(payload []byte) (*InternalRequest, error) {
	var req AnthropicRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, NewConversionError("parse_error", fmt.Sprintf("failed to parse Anthropics request: %v", err), err)
	}

	internal := &InternalRequest{
		Model:              req.Model,
		Temperature:        req.Temperature,
		TopP:               req.TopP,
		MaxTokens:          req.MaxTokens,
		Stop:               append([]string(nil), req.StopSequences...),
		Metadata:           cloneMetadata(req.Metadata),
		Stream:             req.Stream != nil && *req.Stream,
		ReasoningEffort:    nil,
		MaxReasoningTokens: nil,
	}

	if req.DisableParallelToolUse != nil {
		val := !*req.DisableParallelToolUse
		internal.ParallelToolCalls = &val
	}

	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			internal.Tools = append(internal.Tools, InternalTool{
				Type: "function",
				Function: &InternalToolFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  cloneAnyMap(tool.InputSchema),
				},
			})
		}
	}

	if req.ToolChoice != nil {
		internal.ToolChoice = &InternalToolChoice{
			Type:         req.ToolChoice.Type,
			FunctionName: req.ToolChoice.Name,
		}
	}

	if req.Thinking != nil && strings.EqualFold(req.Thinking.Type, "enabled") {
		internal.MaxReasoningTokens = ptrInt(req.Thinking.BudgetTokens)
		switch {
		case req.Thinking.BudgetTokens <= 0:
			// leave nil to signal feature disabled
		case req.Thinking.BudgetTokens <= 5000:
			internal.ReasoningEffort = ptrString("low")
		case req.Thinking.BudgetTokens <= 15000:
			internal.ReasoningEffort = ptrString("medium")
		default:
			internal.ReasoningEffort = ptrString("high")
		}
	}

	if thinking := InternalThinkingFromAnthropic(req.Thinking); thinking != nil {
		internal.Thinking = thinking
	}

	// System instructions are represented as an assistant-less system message
	if sysText := anthropicSystemToText(req.System); sysText != "" {
		internal.Messages = append(internal.Messages, InternalMessage{
			Role: "system",
			Contents: []InternalContent{
				{Type: "text", Text: sysText},
			},
		})
	}

	for _, msg := range req.Messages {
		internal.Messages = append(internal.Messages, anthMessageToInternal(msg))
	}

	return internal, nil
}

// BuildRequestJSON renders an InternalRequest as an Anthropic specific JSON
// payload.
func (a *AnthropicFormatAdapter) BuildRequestJSON(req *InternalRequest) ([]byte, error) {
	if req == nil {
		return json.Marshal(nil)
	}

	out := AnthropicRequest{
		Model:         req.Model,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		MaxTokens:     req.MaxTokens,
		StopSequences: append([]string(nil), req.Stop...),
		Metadata:      cloneMetadata(req.Metadata),
	}

	if req.Stream {
		out.Stream = ptrBool(true)
	}

	if req.ParallelToolCalls != nil && !*req.ParallelToolCalls {
		disable := true
		out.DisableParallelToolUse = &disable
	}

	if len(req.Tools) > 0 {
		out.Tools = make([]AnthropicTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			if tool.Type != "function" || tool.Function == nil {
				continue
			}
			out.Tools = append(out.Tools, AnthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: cloneAnyMap(tool.Function.Parameters),
			})
		}
	}

	if req.ToolChoice != nil {
		out.ToolChoice = &AnthropicToolChoice{
			Type: req.ToolChoice.Type,
			Name: req.ToolChoice.FunctionName,
		}
	}

	ApplyInternalThinkingToAnthropic(req, &out, newDefaultThinkingMapper(a.logger))

	// Build system prompt + conversational messages
	var messages []AnthropicMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			out.System = appendSystemInstruction(out.System, msg)
			continue
		}
		messages = append(messages, internalMessageToAnthropic(msg))
	}
	out.Messages = messages

	return json.Marshal(out)
}

// ParseResponseJSON converts an Anthropic response payload into an
// InternalResponse.
func (a *AnthropicFormatAdapter) ParseResponseJSON(payload []byte) (*InternalResponse, error) {
	var resp AnthropicResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, NewConversionError("parse_error", fmt.Sprintf("failed to parse Anthropic response: %v", err), err)
	}

	internal := &InternalResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		StopReason:   resp.StopReason,
		StopSequence: resp.StopSequence,
		FinishReason: resp.StopReason,
		Success:      true,
		Metadata:     map[string]interface{}{},
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			internal.Content = append(internal.Content, InternalContent{
				Type: "text",
				Text: block.Text,
			})
		case "tool_use":
			toolUse := convertAnthropicToolUse(block)
			internal.Messages = append(internal.Messages, InternalMessage{
				Role:      "assistant",
				ToolCalls: []InternalToolCall{toolUseToInternalCall(toolUse)},
				Contents: []InternalContent{{
					Type:    "tool_use",
					ToolUse: toolUse,
				}},
			})
		case "tool_result":
			toolResult := convertAnthropicToolResult(block)
			internal.Messages = append(internal.Messages, InternalMessage{
				Role: "tool",
				Contents: []InternalContent{{
					Type:       "tool_result",
					Text:       toolResult.Content,
					ToolResult: toolResult,
				}},
				ToolCallID: toolResult.ToolUseID,
			})
		}
	}

	if resp.Usage != nil {
		internal.TokenUsage = &TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return internal, nil
}

// BuildResponseJSON renders an internal response back to Anthropic JSON.
func (a *AnthropicFormatAdapter) BuildResponseJSON(resp *InternalResponse) ([]byte, error) {
	out := AnthropicResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      resp.Model,
		StopReason: resp.FinishReason,
	}

	if resp.StopSequence != "" {
		out.StopSequence = resp.StopSequence
	}

	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			out.Content = append(out.Content, AnthropicContentBlock{
				Type: "text",
				Text: content.Text,
			})
		case "image":
			out.Content = append(out.Content, AnthropicContentBlock{
				Type: "image",
				Source: &AnthropicImageSource{
					Type:      "base64",
					MediaType: content.ImageMediaType,
					Data:      content.ImageURL,
				},
			})
		}
	}

	if resp.TokenUsage != nil {
		out.Usage = &AnthropicUsage{
			InputTokens:  resp.TokenUsage.PromptTokens,
			OutputTokens: resp.TokenUsage.CompletionTokens,
		}
	}

	return json.Marshal(out)
}

// ParseSSE normalises a single SSE line into InternalEvents. For now the
// adapter simply wraps the raw JSON payload.
func (a *AnthropicFormatAdapter) ParseSSE(event string, data []byte) ([]InternalEvent, error) {
	if len(event) == 0 {
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

	return []InternalEvent{
		{Type: event, Data: decoded},
	}, nil
}

// BuildSSE converts internal events back to SSE payloads understood by
// Anthropic clients.
func (a *AnthropicFormatAdapter) BuildSSE(events []InternalEvent) ([]SSEPayload, error) {
	payloads := make([]SSEPayload, 0, len(events))
	for _, ev := range events {
		payloads = append(payloads, SSEPayload{
			Event: ev.Type,
			Data:  ev.Data,
		})
	}
	return payloads, nil
}

// --- helper functions ----------------------------------------------------------------

func anthropicSystemToText(system interface{}) string {
	switch v := system.(type) {
	case string:
		return strings.TrimSpace(v)
	case []AnthropicContentBlock:
		sb := strings.Builder{}
		for _, block := range v {
			if block.Type == "text" && block.Text != "" {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(block.Text)
			}
		}
		return sb.String()
	default:
		return ""
	}
}

func anthMessageToInternal(msg AnthropicMessage) InternalMessage {
	out := InternalMessage{Role: msg.Role}
	for _, block := range msg.GetContentBlocks() {
		switch block.Type {
		case "text":
			out.Contents = append(out.Contents, InternalContent{
				Type: "text",
				Text: block.Text,
			})
		case "image":
			if block.Source != nil {
				out.Contents = append(out.Contents, InternalContent{
					Type:           "image",
					ImageURL:       block.Source.Data,
					ImageMediaType: block.Source.MediaType,
				})
			}
		case "tool_use":
			toolUse := convertAnthropicToolUse(block)
			out.ToolCalls = append(out.ToolCalls, toolUseToInternalCall(toolUse))
			out.Contents = append(out.Contents, InternalContent{
				Type:    "tool_use",
				ToolUse: toolUse,
			})
		case "tool_result":
			toolResult := convertAnthropicToolResult(block)
			out.Contents = append(out.Contents, InternalContent{
				Type:       "tool_result",
				Text:       toolResult.Content,
				ToolResult: toolResult,
			})
		case "thinking":
			out.Contents = append(out.Contents, InternalContent{
				Type: "thinking",
				Thinking: &InternalThinking{
					Provider: "anthropic",
					Type:     block.Type,
					Text:     block.Text,
				},
			})
		default:
			// Ignore unsupported block types
		}
	}
	return out
}

func internalMessageToAnthropic(msg InternalMessage) AnthropicMessage {
	var blocks []AnthropicContentBlock
	for _, content := range msg.Contents {
		switch content.Type {
		case "image":
			blocks = append(blocks, AnthropicContentBlock{
				Type: "image",
				Source: &AnthropicImageSource{
					Type:      "base64",
					MediaType: content.ImageMediaType,
					Data:      content.ImageURL,
				},
			})
		case "tool_use":
			if content.ToolUse != nil {
				inputJSON := ensureJSONRaw(content.ToolUse.Arguments, content.ToolUse.ArgumentsMap)
				blocks = append(blocks, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    content.ToolUse.ID,
					Name:  content.ToolUse.Name,
					Input: inputJSON,
				})
			}
		case "tool_result":
			if content.ToolResult != nil {
				resultBlock := AnthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: content.ToolResult.ToolUseID,
				}
				if content.ToolResult.Content != "" {
					resultBlock.Content = []AnthropicContentBlock{
						{
							Type: "text",
							Text: content.ToolResult.Content,
						},
					}
				} else if len(content.ToolResult.ContentList) > 0 {
					resultBlock.Content = internalContentsToAnthropic(content.ToolResult.ContentList)
				}
				if content.ToolResult.IsError {
					isErr := true
					resultBlock.IsError = &isErr
				}
				blocks = append(blocks, resultBlock)
			}
		case "thinking":
			if content.Thinking != nil {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "thinking",
					Text: content.Thinking.Text,
				})
			}
		default:
			blocks = append(blocks, AnthropicContentBlock{
				Type: "text",
				Text: content.Text,
			})
		}
	}
	for _, call := range msg.ToolCalls {
		blocks = append(blocks, AnthropicContentBlock{
			Type: "tool_use",
			ID:   call.ID,
			Name: call.Name,
			Input: json.RawMessage(func() []byte {
				if call.Arguments == "" {
					return []byte("{}")
				}
				return []byte(call.Arguments)
			}()),
		})
	}
	return AnthropicMessage{
		Role:    msg.Role,
		Content: blocks,
	}
}

func extractToolResultText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []AnthropicContentBlock:
		sb := strings.Builder{}
		for _, block := range v {
			if block.Type == "text" {
				sb.WriteString(block.Text)
			}
		}
		return sb.String()
	case []interface{}:
		sb := strings.Builder{}
		for _, item := range v {
			if obj, ok := item.(map[string]interface{}); ok {
				if text, ok := obj["text"].(string); ok {
					sb.WriteString(text)
				}
			}
		}
		return sb.String()
	default:
		return ""
	}
}

func appendSystemInstruction(existing interface{}, msg InternalMessage) interface{} {
	if len(msg.Contents) == 0 {
		return existing
	}
	text := msg.Contents[0].Text
	if text == "" {
		return existing
	}
	if existing == nil {
		return text
	}
	switch cur := existing.(type) {
	case string:
		return cur + "\n" + text
	case []AnthropicContentBlock:
		return append(cur, AnthropicContentBlock{
			Type: "text",
			Text: text,
		})
	default:
		return text
	}
}

func cloneMetadata(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneAnyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func ptrBool(v bool) *bool       { return &v }
func ptrInt(v int) *int          { return &v }
func ptrString(v string) *string { return &v }

func convertAnthropicToolUse(block AnthropicContentBlock) *InternalToolUse {
	argsString := strings.TrimSpace(string(block.Input))
	if argsString == "" {
		argsString = "{}"
	}
	var argsMap map[string]interface{}
	if json.Valid([]byte(argsString)) {
		_ = json.Unmarshal([]byte(argsString), &argsMap)
	}
	return &InternalToolUse{
		ID:           block.ID,
		Name:         block.Name,
		Arguments:    argsString,
		ArgumentsMap: argsMap,
		Metadata:     map[string]interface{}{},
	}
}

func convertAnthropicToolResult(block AnthropicContentBlock) *InternalToolResult {
	result := &InternalToolResult{
		ToolUseID: block.ToolUseID,
		Content:   extractToolResultText(block.Content),
		IsError:   block.IsError != nil && *block.IsError,
	}
	if list := anthropicAnyToInternalContents(block.Content); len(list) > 0 {
		result.ContentList = list
	}
	return result
}

func anthropicAnyToInternalContents(content interface{}) []InternalContent {
	var result []InternalContent
	switch v := content.(type) {
	case []AnthropicContentBlock:
		for _, block := range v {
			switch block.Type {
			case "text":
				result = append(result, InternalContent{
					Type: "text",
					Text: block.Text,
				})
			case "image":
				if block.Source != nil {
					result = append(result, InternalContent{
						Type:           "image",
						ImageURL:       block.Source.Data,
						ImageMediaType: block.Source.MediaType,
					})
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if blockMap, ok := item.(map[string]interface{}); ok {
				if typ, ok := blockMap["type"].(string); ok {
					switch typ {
					case "text":
						if text, ok := blockMap["text"].(string); ok {
							result = append(result, InternalContent{
								Type: "text",
								Text: text,
							})
						}
					case "image":
						if src, ok := blockMap["source"].(map[string]interface{}); ok {
							url, _ := src["data"].(string)
							mediaType, _ := src["media_type"].(string)
							result = append(result, InternalContent{
								Type:           "image",
								ImageURL:       url,
								ImageMediaType: mediaType,
							})
						}
					}
				}
			}
		}
	}
	return result
}

func internalContentsToAnthropic(contents []InternalContent) []AnthropicContentBlock {
	var blocks []AnthropicContentBlock
	for _, content := range contents {
		switch content.Type {
		case "text":
			blocks = append(blocks, AnthropicContentBlock{
				Type: "text",
				Text: content.Text,
			})
		case "image":
			blocks = append(blocks, AnthropicContentBlock{
				Type: "image",
				Source: &AnthropicImageSource{
					Type:      "base64",
					MediaType: content.ImageMediaType,
					Data:      content.ImageURL,
				},
			})
		}
	}
	return blocks
}

func ensureJSONRaw(text string, fallback map[string]interface{}) json.RawMessage {
	if strings.TrimSpace(text) == "" {
		if fallback != nil {
			if b, err := json.Marshal(fallback); err == nil {
				return json.RawMessage(b)
			}
		}
		return json.RawMessage([]byte("{}"))
	}
	if !json.Valid([]byte(text)) {
		if fallback != nil {
			if b, err := json.Marshal(fallback); err == nil {
				return json.RawMessage(b)
			}
		}
		return json.RawMessage([]byte("{}"))
	}
	return json.RawMessage([]byte(text))
}

func toolUseToInternalCall(use *InternalToolUse) InternalToolCall {
	if use == nil {
		return InternalToolCall{}
	}
	meta := map[string]interface{}{}
	if use.ArgumentsMap != nil {
		meta["arguments_map"] = use.ArgumentsMap
	}
	return InternalToolCall{
		ID:        use.ID,
		Type:      "function",
		Name:      use.Name,
		Arguments: use.Arguments,
		Index:     use.Index,
		Metadata:  meta,
	}
}
