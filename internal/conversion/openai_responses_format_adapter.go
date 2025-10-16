package conversion

import (
	"encoding/json"
	"fmt"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// OpenAIResponsesFormatAdapter 负责 OpenAI Responses 与内部模型互转
type OpenAIResponsesFormatAdapter struct {
	logger *logger.Logger
}

func (a *OpenAIResponsesFormatAdapter) Name() string {
	return "openai_responses"
}

func (a *OpenAIResponsesFormatAdapter) ParseRequestJSON(payload []byte) (*InternalRequest, error) {
	var req OpenAIResponsesRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI responses request: %w", err)
	}

	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("responses request missing model")
	}

	internalReq := &InternalRequest{
		Model:             req.Model,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		MaxOutputTokens:   req.MaxOutputTokens,
		ParallelToolCalls: req.ParallelToolCalls,
		User:              req.User,
		Metadata:          cloneStringInterfaceMap(req.Metadata),
		Tools:             []InternalTool{},
		Messages:          []InternalMessage{},
		Stop:              append([]string(nil), req.Stop...),
		// 🆕 采样控制参数
		PresencePenalty:   req.PresencePenalty,
		FrequencyPenalty:  req.FrequencyPenalty,
		LogitBias:         cloneLogitBias(req.LogitBias),
		N:                 req.N,
		// 🆕 输出格式控制
		ResponseFormat:    convertOpenAIResponseFormatToInternal(req.ResponseFormat),
	}

	if len(req.Tools) > 0 {
		internalReq.Tools = make([]InternalTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			if strings.ToLower(tool.Type) == "function" && tool.Function != nil {
				internalReq.Tools = append(internalReq.Tools, InternalTool{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				})
			}
		}
	}

	if req.ToolChoice != nil {
		internalReq.ToolChoice = convertOpenAIToolChoiceToInternal(req.ToolChoice)
	}

	// 🆕 双路径回退：优先使用 input，回退到 messages（参考 chat2response）
	inputMessages := req.Input
	if len(inputMessages) == 0 && len(req.Messages) > 0 {
		inputMessages = req.Messages
	}

	for _, msg := range inputMessages {
		internalReq.Messages = append(internalReq.Messages, convertResponsesMessageToInternal(msg))
	}

	return internalReq, nil
}

func (a *OpenAIResponsesFormatAdapter) BuildRequestJSON(req *InternalRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("internal request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("internal request missing model")
	}

	out := OpenAIResponsesRequest{
		Model:             req.Model,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		MaxOutputTokens:   req.MaxOutputTokens,
		ParallelToolCalls: req.ParallelToolCalls,
		User:              req.User,
		Metadata:          cloneStringInterfaceMap(req.Metadata),
		Stop:              append([]string(nil), req.Stop...),
		// 🆕 采样控制参数
		PresencePenalty:   req.PresencePenalty,
		FrequencyPenalty:  req.FrequencyPenalty,
		LogitBias:         cloneLogitBias(req.LogitBias),
		N:                 req.N,
		// 🆕 输出格式控制
		ResponseFormat:    convertInternalResponseFormatToOpenAI(req.ResponseFormat),
	}

	if len(req.Tools) > 0 {
		out.Tools = make([]OpenAIResponsesTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			out.Tools = append(out.Tools, OpenAIResponsesTool{
				Type: "function",
				Function: &OpenAIFunctionDef{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
	}

	if req.ToolChoice != nil {
		out.ToolChoice = buildOpenAIToolChoice(req.ToolChoice)
	}

	// 🔧 修复:使用标准 messages 字段而非 input (88code 端点要求)
	out.Messages = make([]OpenAIResponsesMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		out.Messages = append(out.Messages, convertInternalMessageToResponses(msg))
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI responses request: %w", err)
	}
	return result, nil
}

func (a *OpenAIResponsesFormatAdapter) ParseResponseJSON(payload []byte) (*InternalResponse, error) {
	var resp OpenAIResponsesResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI responses response: %w", err)
	}

	var finalMsg *InternalMessage
	toolCalls := []InternalToolCall{}

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			msg := convertResponsesOutputToInternal(item)
			finalMsg = &msg
		case "tool_call", "function_call":
			if item.Name != "" {
				toolCalls = append(toolCalls, InternalToolCall{
					Name:      item.Name,
					Arguments: item.Arguments,
				})
			}
		}
	}

	if finalMsg == nil {
		finalMsg = &InternalMessage{
			Role:      "assistant",
			Contents:  []InternalContent{},
			ToolCalls: []InternalToolCall{},
		}
	}
	if len(toolCalls) > 0 {
		finalMsg.ToolCalls = append(finalMsg.ToolCalls, toolCalls...)
	}
	if finalMsg.FinishReason == "" {
		finalMsg.FinishReason = "end_turn"
	}

	internalResp := &InternalResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Message: finalMsg,
	}

	if resp.Usage != nil {
		internalResp.Usage = &InternalUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	return internalResp, nil
}

func (a *OpenAIResponsesFormatAdapter) BuildResponseJSON(resp *InternalResponse) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("internal response is nil")
	}
	if resp.Message == nil {
		return nil, fmt.Errorf("internal response missing message")
	}

	outputItems := buildResponsesOutputFromInternal(*resp.Message)

	out := OpenAIResponsesResponse{
		ID:     resp.ID,
		Model:  resp.Model,
		Status: "completed",
		Output: outputItems,
	}

	if resp.Usage != nil {
		out.Usage = &OpenAIResponsesUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  totalTokensFallback(resp.Usage),
		}
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI responses response: %w", err)
	}
	return result, nil
}

func (a *OpenAIResponsesFormatAdapter) ParseSSE(event string, data []byte) ([]InternalEvent, error) {
	return nil, fmt.Errorf("ParseSSE not implemented")
}

func (a *OpenAIResponsesFormatAdapter) BuildSSE(events []InternalEvent) ([]SSEPayload, error) {
	return nil, fmt.Errorf("BuildSSE not implemented")
}

// cloneStringInterfaceMap performs a shallow clone of a map[string]interface{}
func cloneStringInterfaceMap(src map[string]interface{}) map[string]interface{} {
    if src == nil {
        return nil
    }
    out := make(map[string]interface{}, len(src))
    for k, v := range src {
        out[k] = v
    }
    return out
}

func convertResponsesMessageToInternal(msg OpenAIResponsesMessage) InternalMessage {
	internal := InternalMessage{
		Role:      msg.Role,
		Contents:  []InternalContent{},
		ToolCalls: []InternalToolCall{},
	}

	for _, content := range msg.Content {
		switch content.Type {
		case "input_text", "output_text", "text":
			if content.Text != "" {
				internal.Contents = append(internal.Contents, InternalContent{
					Type: "text",
					Text: content.Text,
				})
			}
		case "input_image", "image_url":
			if content.ImageURL != nil && content.ImageURL.URL != "" {
				internal.Contents = append(internal.Contents, InternalContent{
					Type:      "image",
					ImageURL:  content.ImageURL.URL,
					MediaType: detectMediaType(content.ImageURL.URL),
				})
			}
		default:
			if content.Text != "" {
				internal.Contents = append(internal.Contents, InternalContent{
					Type: "text",
					Text: content.Text,
				})
			}
		}
	}

	return internal
}

func convertInternalMessageToResponses(msg InternalMessage) OpenAIResponsesMessage {
	contentItems := []OpenAIResponsesContentItem{}

	for _, content := range msg.Contents {
		switch content.Type {
		case "image", "image_url":
			if strings.TrimSpace(content.ImageURL) == "" {
				continue
			}
			contentItems = append(contentItems, OpenAIResponsesContentItem{
				Type:     "input_image",
				ImageURL: &OpenAIImageURL{URL: content.ImageURL},
			})
		default:
			if content.Text != "" {
				contentItems = append(contentItems, OpenAIResponsesContentItem{
					Type: responsesContentTypeForRole(msg.Role, content.Type),
					Text: content.Text,
				})
			}
		}
	}

	if len(contentItems) == 0 {
		contentItems = append(contentItems, OpenAIResponsesContentItem{
			Type: responsesContentTypeForRole(msg.Role, "text"),
			Text: "",
		})
	}

	return OpenAIResponsesMessage{
		Type:    "message",  // 🔧 修复:添加type字段
		Role:    msg.Role,
		Content: contentItems,
	}
}

func responsesContentTypeForRole(role, contentType string) string {
	switch contentType {
	case "image", "image_url":
		return "input_image"
	}
	switch strings.ToLower(role) {
	case "user":
		return "input_text"
	case "assistant":
		return "output_text"
	case "tool":
		return "tool_result"
	default:
		return "text"
	}
}

func convertResponsesOutputToInternal(item OpenAIResponsesOutputItem) InternalMessage {
	internal := InternalMessage{
		Role:      item.Role,
		Contents:  []InternalContent{},
		ToolCalls: []InternalToolCall{},
	}

	for _, content := range item.Content {
		switch content.Type {
		case "output_text", "text":
			if content.Text != "" {
				internal.Contents = append(internal.Contents, InternalContent{
					Type: "text",
					Text: content.Text,
				})
			}
		case "image_url":
			if content.ImageURL != nil && content.ImageURL.URL != "" {
				internal.Contents = append(internal.Contents, InternalContent{
					Type:      "image",
					ImageURL:  content.ImageURL.URL,
					MediaType: detectMediaType(content.ImageURL.URL),
				})
			}
		}
	}

	internal.FinishReason = "end_turn"
	return internal
}

func buildResponsesOutputFromInternal(msg InternalMessage) []OpenAIResponsesOutputItem {
	contentItems := []OpenAIResponsesContentItem{}

	for _, content := range msg.Contents {
		switch content.Type {
		case "image", "image_url":
			if strings.TrimSpace(content.ImageURL) == "" {
				continue
			}
			contentItems = append(contentItems, OpenAIResponsesContentItem{
				Type:     "image_url",
				ImageURL: &OpenAIImageURL{URL: content.ImageURL},
			})
		default:
			if content.Text != "" {
				contentItems = append(contentItems, OpenAIResponsesContentItem{
					Type: "output_text",
					Text: content.Text,
				})
			}
		}
	}

	if len(contentItems) == 0 {
		contentItems = append(contentItems, OpenAIResponsesContentItem{
			Type: "output_text",
			Text: "",
		})
	}

	items := []OpenAIResponsesOutputItem{
		{
			Type:    "message",
			Status:  "completed",
			Role:    msg.Role,
			Content: contentItems,
		},
	}

	for _, call := range msg.ToolCalls {
		items = append(items, OpenAIResponsesOutputItem{
			Type:      "function_call",
			Status:    "completed",
			Name:      call.Name,
			Arguments: call.Arguments,
		})
	}

	return items
}
