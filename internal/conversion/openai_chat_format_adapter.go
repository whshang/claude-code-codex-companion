package conversion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// OpenAIChatFormatAdapter 负责 OpenAI Chat Completions 与内部模型互转
type OpenAIChatFormatAdapter struct {
	logger *logger.Logger
}

func (a *OpenAIChatFormatAdapter) Name() string {
	return "openai_chat"
}

func (a *OpenAIChatFormatAdapter) ParseRequestJSON(payload []byte) (*InternalRequest, error) {
	var req OpenAIRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI chat request: %w", err)
	}

	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("openai chat request missing model")
	}

	internalReq := &InternalRequest{
		Model:               req.Model,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		MaxCompletionTokens: req.MaxCompletionTokens,
		MaxOutputTokens:     req.MaxOutputTokens,
		MaxTokens:           req.MaxTokens,
		Stop:                copyStringSlice(req.Stop),
		User:                req.User,
		Metadata:            make(map[string]interface{}),
		// 🆕 采样控制参数
		PresencePenalty:     req.PresencePenalty,
		FrequencyPenalty:    req.FrequencyPenalty,
		LogitBias:           cloneLogitBias(req.LogitBias),
		N:                   req.N,
		// 🆕 输出格式控制
		ResponseFormat:      convertOpenAIResponseFormatToInternal(req.ResponseFormat),
		// 🆕 推理相关字段
		ReasoningEffort:     req.ReasoningEffort,
		MaxReasoningTokens:  req.MaxReasoningTokens,
	}

	if req.User != "" {
		internalReq.Metadata["user_id"] = req.User
	}

	if req.Stream != nil {
		internalReq.Stream = *req.Stream
	}

	if req.ParallelToolCalls != nil {
		val := *req.ParallelToolCalls
		internalReq.ParallelToolCalls = &val
	}

	if req.ToolChoice != nil {
		internalReq.ToolChoice = convertOpenAIToolChoiceToInternal(req.ToolChoice)
	}

	if len(req.Tools) > 0 {
		internalReq.Tools = make([]InternalTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			if strings.ToLower(tool.Type) != "function" {
				continue
			}
			internalReq.Tools = append(internalReq.Tools, InternalTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			})
		}
	}

	internalReq.Messages = make([]InternalMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		internalReq.Messages = append(internalReq.Messages, convertOpenAIMessageToInternal(msg))
	}

	return internalReq, nil
}

func (a *OpenAIChatFormatAdapter) BuildRequestJSON(req *InternalRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("internal request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("internal request missing model")
	}

	out := OpenAIRequest{
		Model:               req.Model,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		MaxCompletionTokens: req.MaxCompletionTokens,
		MaxOutputTokens:     req.MaxOutputTokens,
		MaxTokens:           req.MaxTokens,
		Stop:                copyStringSlice(req.Stop),
		User:                req.User,
		// 🆕 采样控制参数
		PresencePenalty:     req.PresencePenalty,
		FrequencyPenalty:    req.FrequencyPenalty,
		LogitBias:           cloneLogitBias(req.LogitBias),
		N:                   req.N,
		// 🆕 输出格式控制
		ResponseFormat:      convertInternalResponseFormatToOpenAI(req.ResponseFormat),
	}

	if req.Stream {
		out.Stream = boolPtrValue(true)
	}

	if req.ParallelToolCalls != nil {
		val := *req.ParallelToolCalls
		out.ParallelToolCalls = &val
	}

	if len(req.Tools) > 0 {
		out.Tools = make([]OpenAITool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			out.Tools = append(out.Tools, OpenAITool{
				Type: "function",
				Function: OpenAIFunctionDef{
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

	out.Messages = make([]OpenAIMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		out.Messages = append(out.Messages, convertInternalMessageToOpenAI(msg))
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI chat request: %w", err)
	}
	return result, nil
}

func (a *OpenAIChatFormatAdapter) ParseResponseJSON(payload []byte) (*InternalResponse, error) {
	var resp OpenAIResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI chat response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai chat response contains no choices")
	}

	choice := resp.Choices[0]
	internalMsg := convertOpenAIMessageToInternal(choice.Message)
	internalMsg.FinishReason = normalizeOpenAIFinishReason(choice.FinishReason)

	internalResp := &InternalResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Message: &internalMsg,
		Usage:   convertOpenAIUsage(resp.Usage),
	}
	internalResp.StopReason = internalMsg.FinishReason

	return internalResp, nil
}

func (a *OpenAIChatFormatAdapter) BuildResponseJSON(resp *InternalResponse) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("internal response is nil")
	}
	if resp.Message == nil {
		return nil, fmt.Errorf("internal response missing message")
	}

	choice := OpenAIChoice{
		Index:        0,
		FinishReason: denormalizeOpenAIFinishReason(resp.Message.FinishReason),
		Message:      convertInternalMessageToOpenAI(*resp.Message),
	}

	out := OpenAIResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Choices: []OpenAIChoice{choice},
	}

	if resp.Usage != nil {
		out.Usage = &OpenAIUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      totalTokensFallback(resp.Usage),
		}
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI chat response: %w", err)
	}
	return result, nil
}

// ParseSSE / BuildSSE 将在流式阶段实现
func (a *OpenAIChatFormatAdapter) ParseSSE(event string, data []byte) ([]InternalEvent, error) {
	return nil, fmt.Errorf("ParseSSE not implemented")
}

func (a *OpenAIChatFormatAdapter) BuildSSE(events []InternalEvent) ([]SSEPayload, error) {
	return nil, fmt.Errorf("BuildSSE not implemented")
}

func convertOpenAIToolChoiceToInternal(choice interface{}) *InternalToolChoice {
	switch v := choice.(type) {
	case string:
		switch v {
		case "auto":
			return &InternalToolChoice{Type: "auto"}
		case "none":
			return &InternalToolChoice{Type: "none"}
		case "required":
			return &InternalToolChoice{Type: "required"}
		default:
			return &InternalToolChoice{Type: v}
		}
	case map[string]interface{}:
		if t, ok := v["type"].(string); ok {
			if t == "function" {
				if fn, ok := v["function"].(map[string]interface{}); ok {
					name, _ := fn["name"].(string)
					return &InternalToolChoice{Type: "tool", Name: name}
				}
			}
			return &InternalToolChoice{Type: t}
		}
	}
	return nil
}

func buildOpenAIToolChoice(choice *InternalToolChoice) interface{} {
	if choice == nil {
		return nil
	}

	switch choice.Type {
	case "", "auto":
		return "auto"
	case "none":
		return "none"
	case "required":
		return "required"
	case "tool":
		if choice.Name == "" {
			return map[string]interface{}{"type": "function"}
		}
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": choice.Name,
			},
		}
	default:
		return choice.Type
	}
}

func convertOpenAIMessageToInternal(msg OpenAIMessage) InternalMessage {
	internal := InternalMessage{
		Role:       msg.Role,
		ToolCallID: msg.ToolCallID,
		Contents:   []InternalContent{},
		ToolCalls:  []InternalToolCall{},
	}

	appendOpenAIContentForInternal(&internal, msg.Content)

	for idx, call := range msg.ToolCalls {
		id := call.ID
		if id == "" {
			id = fmt.Sprintf("tool_call_%d", idx)
		}
		internal.ToolCalls = append(internal.ToolCalls, InternalToolCall{
			Index:     idx,
			ID:        id,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}

	return internal
}

func convertInternalMessageToOpenAI(msg InternalMessage) OpenAIMessage {
	out := OpenAIMessage{
		Role:       msg.Role,
		ToolCallID: msg.ToolCallID,
	}

	contentList := make([]OpenAIMessageContent, 0, len(msg.Contents))
	for _, content := range msg.Contents {
		switch content.Type {
		case "image", "image_url":
			if strings.TrimSpace(content.ImageURL) == "" {
				continue
			}
			contentList = append(contentList, OpenAIMessageContent{
				Type: "image_url",
				ImageURL: &OpenAIImageURL{
					URL: content.ImageURL,
				},
			})
		case "tool_result":
			if content.Text != "" {
				contentList = append(contentList, OpenAIMessageContent{
					Type: "text",
					Text: content.Text,
				})
			}
		default:
			if content.Text != "" {
				contentList = append(contentList, OpenAIMessageContent{
					Type: "text",
					Text: content.Text,
				})
			}
		}
	}

	switch len(contentList) {
	case 0:
		out.Content = ""
	case 1:
		if contentList[0].Type == "text" && contentList[0].ImageURL == nil {
			out.Content = contentList[0].Text
		} else {
			out.Content = contentList
		}
	default:
		out.Content = contentList
	}

	for idx, call := range msg.ToolCalls {
		id := call.ID
		if id == "" {
			id = fmt.Sprintf("tool_call_%d", idx)
		}
		out.ToolCalls = append(out.ToolCalls, OpenAIToolCall{
			Index: idx,
			ID:    id,
			Type:  "function",
			Function: OpenAIToolCallDetail{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}

	return out
}

func appendOpenAIContentForInternal(msg *InternalMessage, content interface{}) {
	if content == nil {
		return
	}

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
			appendOpenAIContentItem(msg, item)
		}
	case map[string]interface{}:
		appendOpenAIContentItem(msg, v)
	case []OpenAIMessageContent:
		for _, block := range v {
			switch block.Type {
			case "image_url":
				if block.ImageURL != nil && block.ImageURL.URL != "" {
					msg.Contents = append(msg.Contents, InternalContent{
						Type:      "image",
						ImageURL:  block.ImageURL.URL,
						MediaType: detectMediaType(block.ImageURL.URL),
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
	default:
		msg.Contents = append(msg.Contents, InternalContent{
			Type: "text",
			Text: fmt.Sprintf("%v", v),
		})
	}
}

func appendOpenAIContentItem(msg *InternalMessage, item interface{}) {
	switch block := item.(type) {
	case string:
		if block != "" {
			msg.Contents = append(msg.Contents, InternalContent{
				Type: "text",
				Text: block,
			})
		}
	case map[string]interface{}:
		contentType, _ := block["type"].(string)
		switch contentType {
		case "text":
			if text, ok := block["text"].(string); ok && text != "" {
				msg.Contents = append(msg.Contents, InternalContent{
					Type: "text",
					Text: text,
				})
			}
		case "image_url":
			if imageMap, ok := block["image_url"].(map[string]interface{}); ok {
				if url, ok := imageMap["url"].(string); ok && url != "" {
					msg.Contents = append(msg.Contents, InternalContent{
						Type:      "image",
						ImageURL:  url,
						MediaType: detectMediaType(url),
					})
				}
			}
		case "output_text":
			if text, ok := block["text"].(string); ok && text != "" {
				msg.Contents = append(msg.Contents, InternalContent{
					Type: "text",
					Text: text,
				})
			}
		default:
			if text, ok := block["text"].(string); ok && text != "" {
				msg.Contents = append(msg.Contents, InternalContent{
					Type: "text",
					Text: text,
				})
			}
		}
	default:
		msg.Contents = append(msg.Contents, InternalContent{
			Type: "text",
			Text: fmt.Sprintf("%v", block),
		})
	}
}

func convertOpenAIUsage(respUsage *OpenAIUsage) *InternalUsage {
	if respUsage == nil {
		return nil
	}
	internal := &InternalUsage{
		InputTokens:  respUsage.PromptTokens,
		OutputTokens: respUsage.CompletionTokens,
		TotalTokens:  respUsage.TotalTokens,
	}
	if internal.TotalTokens == 0 && (internal.InputTokens > 0 || internal.OutputTokens > 0) {
		internal.TotalTokens = internal.InputTokens + internal.OutputTokens
	}
	return internal
}

func totalTokensFallback(usage *InternalUsage) int {
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	return usage.InputTokens + usage.OutputTokens
}

func denormalizeOpenAIFinishReason(reason string) string {
	switch reason {
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "", "end_turn":
		return "stop"
	default:
		return reason
	}
}

func boolPtrValue(v bool) *bool {
	return &v
}

// 🆕 Helper函数：克隆 LogitBias
func cloneLogitBias(src map[string]float64) map[string]float64 {
	if src == nil {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyStringSlice 创建字符串切片的深度拷贝，保持空切片的语义
func copyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	if len(src) == 0 {
		return []string{} // 保持空切片的语义
	}
	return append([]string(nil), src...)
}

// 🆕 OpenAI ResponseFormat → Internal ResponseFormat
func convertOpenAIResponseFormatToInternal(rf *OpenAIResponseFormat) *InternalResponseFormat {
	if rf == nil {
		return nil
	}
	internal := &InternalResponseFormat{
		Type: rf.Type,
	}
	if len(rf.Schema) > 0 {
		internal.Schema = make(map[string]interface{}, len(rf.Schema))
		for k, v := range rf.Schema {
			internal.Schema[k] = v
		}
	}
	return internal
}

// 🆕 Internal ResponseFormat → OpenAI ResponseFormat
func convertInternalResponseFormatToOpenAI(irf *InternalResponseFormat) *OpenAIResponseFormat {
	if irf == nil {
		return nil
	}
	openai := &OpenAIResponseFormat{
		Type: irf.Type,
	}
	if len(irf.Schema) > 0 {
		openai.Schema = make(map[string]interface{}, len(irf.Schema))
		for k, v := range irf.Schema {
			openai.Schema[k] = v
		}
	}
	return openai
}

// NewOpenAIChatAdapter creates a new adapter for the static proxy.
func NewOpenAIChatAdapter() RequestAdapter {
	return &OpenAIChatFormatAdapter{}
}

// ConvertRequest implements the RequestAdapter interface for the new static proxy.
// It currently acts as a pass-through, as the new architecture prioritizes routing over conversion.
func (a *OpenAIChatFormatAdapter) ConvertRequest(req *http.Request) (*http.Request, error) {
	// No-op for now. The request is passed through as is.
	return req, nil
}

