package conversion

import (
	"encoding/json"
	"fmt"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// AnthropicFormatAdapter 负责 Anthropic 消息格式与内部模型之间的转换
type AnthropicFormatAdapter struct {
	logger *logger.Logger
}

func (a *AnthropicFormatAdapter) Name() string {
	return "anthropic"
}

func (a *AnthropicFormatAdapter) ParseRequestJSON(payload []byte) (*InternalRequest, error) {
	var req AnthropicRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic request: %w", err)
	}

	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("anthropic request missing model")
	}

	internalReq := &InternalRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stop:        append([]string(nil), req.StopSequences...),
		Metadata:    cloneStringInterfaceMap(req.Metadata),
		Tools:       []InternalTool{},
		Messages:    []InternalMessage{},
	}

	if req.Metadata != nil {
		if userID, ok := req.Metadata["user_id"].(string); ok && userID != "" {
			internalReq.User = userID
		} else if user, ok := req.Metadata["user"].(string); ok && user != "" {
			internalReq.User = user
		}
	}

	if req.Stream != nil {
		internalReq.Stream = *req.Stream
	}

	if req.DisableParallelToolUse != nil {
		val := !*req.DisableParallelToolUse
		internalReq.ParallelToolCalls = &val
	}

	if len(req.Tools) > 0 {
		internalReq.Tools = make([]InternalTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			internalReq.Tools = append(internalReq.Tools, InternalTool{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			})
		}
	}

	if req.ToolChoice != nil {
		internalReq.ToolChoice = convertAnthropicToolChoiceToInternal(req.ToolChoice)
	}

	if req.System != nil {
		systemMsg := convertAnthropicSystemToInternal(req.System)
		if systemMsg != nil {
			internalReq.Messages = append(internalReq.Messages, *systemMsg)
		}
	}

	for _, message := range req.Messages {
		internalReq.Messages = append(internalReq.Messages, convertAnthropicMessageToInternal(message))
	}

	return internalReq, nil
}

func (a *AnthropicFormatAdapter) BuildRequestJSON(req *InternalRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("internal request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("internal request missing model")
	}

	anth := AnthropicRequest{
		Model:         req.Model,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		MaxTokens:     req.MaxTokens,
		StopSequences: append([]string(nil), req.Stop...),
		Metadata:      cloneStringInterfaceMap(req.Metadata),
	}

	if req.User != "" {
		if anth.Metadata == nil {
			anth.Metadata = make(map[string]interface{})
		}
		if _, exists := anth.Metadata["user_id"]; !exists {
			anth.Metadata["user_id"] = req.User
		}
	}

	if req.Stream {
		stream := true
		anth.Stream = &stream
	}

	if req.ParallelToolCalls != nil {
		val := !*req.ParallelToolCalls
		anth.DisableParallelToolUse = &val
	}

	if len(req.Tools) > 0 {
		anth.Tools = make([]AnthropicTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			anth.Tools = append(anth.Tools, AnthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: tool.Parameters,
			})
		}
	}

	if req.ToolChoice != nil {
		anth.ToolChoice = buildAnthropicToolChoice(req.ToolChoice)
	}

	var systemBlocks []AnthropicContentBlock
	messages := make([]AnthropicMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if strings.EqualFold(msg.Role, "system") {
			systemBlocks = append(systemBlocks, buildAnthropicContentBlocksFromInternal(msg, true)...)
			continue
		}
		converted := buildAnthropicMessageFromInternal(msg)
		if converted != nil {
			messages = append(messages, *converted)
		}
	}

	if len(systemBlocks) > 0 {
		if len(systemBlocks) == 1 && systemBlocks[0].Type == "text" {
			anth.System = systemBlocks[0].Text
		} else {
			systemContent := make([]AnthropicContentBlock, len(systemBlocks))
			copy(systemContent, systemBlocks)
			anth.System = systemContent
		}
	}

	anth.Messages = messages

	result, err := json.Marshal(anth)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Anthropic request: %w", err)
	}
	return result, nil
}

func (a *AnthropicFormatAdapter) ParseResponseJSON(payload []byte) (*InternalResponse, error) {
	var resp AnthropicResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	internalMsg := InternalMessage{
		ID:           resp.ID,
		Model:        resp.Model,
		Role:         resp.Role,
		Contents:     []InternalContent{},
		ToolCalls:    []InternalToolCall{},
		FinishReason: normalizeAnthropicStopReason(resp.StopReason),
		StopSequence: resp.StopSequence,
	}

	for idx, block := range resp.Content {
		appendAnthropicContentBlock(&internalMsg, block, idx)
	}

	internalResp := &InternalResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Message: &internalMsg,
	}
	internalResp.StopReason = internalMsg.FinishReason

	if resp.Usage != nil {
		internalResp.Usage = &InternalUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return internalResp, nil
}

func (a *AnthropicFormatAdapter) BuildResponseJSON(resp *InternalResponse) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("internal response is nil")
	}
	if resp.Message == nil {
		return nil, fmt.Errorf("internal response missing message")
	}

	contentBlocks := buildAnthropicContentBlocksFromInternal(*resp.Message, false)

	out := AnthropicResponse{
		ID:           resp.ID,
		Type:         "message",
		Role:         resp.Message.Role,
		Model:        resp.Model,
		StopReason:   denormalizeAnthropicStopReason(resp.Message.FinishReason),
		StopSequence: resp.Message.StopSequence,
		Content:      contentBlocks,
	}

	if resp.Usage != nil {
		out.Usage = &AnthropicUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		}
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Anthropic response: %w", err)
	}
	return result, nil
}

// ParseSSE / BuildSSE 留待流式阶段实现
func (a *AnthropicFormatAdapter) ParseSSE(event string, data []byte) ([]InternalEvent, error) {
	return nil, fmt.Errorf("ParseSSE not implemented")
}

func (a *AnthropicFormatAdapter) BuildSSE(events []InternalEvent) ([]SSEPayload, error) {
	return nil, fmt.Errorf("BuildSSE not implemented")
}

func convertAnthropicToolChoiceToInternal(choice *AnthropicToolChoice) *InternalToolChoice {
	if choice == nil {
		return nil
	}

	switch choice.Type {
	case "auto":
		return &InternalToolChoice{Type: "auto"}
	case "any":
		return &InternalToolChoice{Type: "required"}
	case "tool":
		return &InternalToolChoice{Type: "tool", Name: choice.Name}
	default:
		return &InternalToolChoice{Type: choice.Type, Name: choice.Name}
	}
}

func buildAnthropicToolChoice(choice *InternalToolChoice) *AnthropicToolChoice {
	if choice == nil {
		return nil
	}

	switch choice.Type {
	case "auto", "":
		return &AnthropicToolChoice{Type: "auto"}
	case "required", "any":
		return &AnthropicToolChoice{Type: "any"}
	case "tool":
		return &AnthropicToolChoice{Type: "tool", Name: choice.Name}
	default:
		return &AnthropicToolChoice{Type: choice.Type, Name: choice.Name}
	}
}

func convertAnthropicSystemToInternal(system interface{}) *InternalMessage {
	if system == nil {
		return nil
	}
	msg := InternalMessage{
		Role:     "system",
		Contents: []InternalContent{},
	}
	appendAnthropicSystemContent(&msg, system)
	if len(msg.Contents) == 0 {
		return nil
	}
	return &msg
}

func appendAnthropicSystemContent(msg *InternalMessage, content interface{}) {
	switch v := content.(type) {
	case string:
		if v != "" {
			msg.Contents = append(msg.Contents, InternalContent{
				Type: "text",
				Text: v,
			})
		}
	case []interface{}:
		for _, item := range v {
			appendAnthropicSystemContent(msg, item)
		}
	case []AnthropicContentBlock:
		for _, block := range v {
			appendAnthropicContentBlock(msg, block, 0)
		}
	case map[string]interface{}:
		if text, ok := v["text"].(string); ok && text != "" {
			msg.Contents = append(msg.Contents, InternalContent{
				Type: "text",
				Text: text,
			})
		}
	default:
		msg.Contents = append(msg.Contents, InternalContent{
			Type: "text",
			Text: fmt.Sprintf("%v", v),
		})
	}
}

func convertAnthropicMessageToInternal(message AnthropicMessage) InternalMessage {
	internal := InternalMessage{
		Role:      message.Role,
		Contents:  []InternalContent{},
		ToolCalls: []InternalToolCall{},
	}

	blocks := message.GetContentBlocks()
	for idx, block := range blocks {
		appendAnthropicContentBlock(&internal, block, idx)
	}
	return internal
}

func appendAnthropicContentBlock(msg *InternalMessage, block AnthropicContentBlock, index int) {
	switch block.Type {
	case "text":
		if block.Text != "" {
			msg.Contents = append(msg.Contents, InternalContent{
				Type: "text",
				Text: block.Text,
			})
		}
	case "image":
		if block.Source != nil && block.Source.Data != "" {
			dataURL := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
			msg.Contents = append(msg.Contents, InternalContent{
				Type:      "image",
				ImageURL:  dataURL,
				MediaType: block.Source.MediaType,
			})
		}
	case "tool_use":
		args := string(block.Input)
		msg.ToolCalls = append(msg.ToolCalls, InternalToolCall{
			Index:     index,
			ID:        block.ID,
			Name:      block.Name,
			Arguments: args,
		})
	case "tool_result":
		msg.ToolCallID = block.ToolUseID
		resultText := extractToolResultText(block.Content)
		if resultText != "" {
			msg.Contents = append(msg.Contents, InternalContent{
				Type: "tool_result",
				Text: resultText,
			})
		}
	default:
		if block.Text != "" {
			msg.Contents = append(msg.Contents, InternalContent{
				Type: "text",
				Text: block.Text,
			})
		}
	}
}

func extractToolResultText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []AnthropicContentBlock:
		var sb strings.Builder
		for _, block := range v {
			if block.Type == "text" && block.Text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(block.Text)
			}
		}
		return sb.String()
	case []interface{}:
		var sb strings.Builder
		for _, item := range v {
			if blockMap, ok := item.(map[string]interface{}); ok {
				if t, ok := blockMap["type"].(string); ok && t == "text" {
					if text, ok := blockMap["text"].(string); ok {
						if sb.Len() > 0 {
							sb.WriteString("\n")
						}
						sb.WriteString(text)
					}
				}
			} else if str, ok := item.(string); ok {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(str)
			}
		}
		return sb.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func buildAnthropicMessageFromInternal(msg InternalMessage) *AnthropicMessage {
	role := msg.Role
	if strings.EqualFold(role, "tool") {
		role = "user"
	}

	contentBlocks := buildAnthropicContentBlocksFromInternal(msg, false)
	if len(contentBlocks) == 0 && len(msg.ToolCalls) == 0 {
		return nil
	}

	return &AnthropicMessage{
		Role:    role,
		Content: contentBlocks,
	}
}

func buildAnthropicContentBlocksFromInternal(msg InternalMessage, system bool) []AnthropicContentBlock {
	var blocks []AnthropicContentBlock

	for _, content := range msg.Contents {
		switch content.Type {
		case "image", "image_url":
			if strings.HasPrefix(content.ImageURL, "data:") {
				dataParts := strings.SplitN(content.ImageURL[5:], ";base64,", 2)
				if len(dataParts) == 2 {
					blocks = append(blocks, AnthropicContentBlock{
						Type: "image",
						Source: &AnthropicImageSource{
							Type:      "base64",
							MediaType: dataParts[0],
							Data:      dataParts[1],
						},
					})
				}
			}
		case "tool_result":
			fallthrough
		default:
			if content.Text != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "text",
					Text: content.Text,
				})
			}
		}
	}

	if !system {
		for _, call := range msg.ToolCalls {
			raw := ensureValidJSON(call.Arguments)
			blocks = append(blocks, AnthropicContentBlock{
				Type:  "tool_use",
				ID:    call.ID,
				Name:  call.Name,
				Input: raw,
			})
		}

		if strings.EqualFold(msg.Role, "tool") || msg.ToolCallID != "" {
			resultText := strings.Builder{}
			for _, c := range msg.Contents {
				if c.Text != "" {
					if resultText.Len() > 0 {
						resultText.WriteString("\n")
					}
					resultText.WriteString(c.Text)
				}
			}
			blocks = []AnthropicContentBlock{
				{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content: []AnthropicContentBlock{
						{Type: "text", Text: resultText.String()},
					},
				},
			}
		}
	}

	return blocks
}

func ensureValidJSON(arguments string) json.RawMessage {
	if arguments == "" {
		return json.RawMessage("null")
	}
	if json.Valid([]byte(arguments)) {
		return json.RawMessage(arguments)
	}
	quoted, _ := json.Marshal(arguments)
	return json.RawMessage(quoted)
}

func normalizeAnthropicStopReason(reason string) string {
	switch reason {
	case "tool_use":
		return "tool_use"
	case "max_tokens":
		return "max_tokens"
	case "stop_sequence":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

func denormalizeAnthropicStopReason(reason string) string {
	switch reason {
	case "tool_use":
		return "tool_use"
	case "max_tokens":
		return "max_tokens"
	case "stop_sequence":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

func cloneStringInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
