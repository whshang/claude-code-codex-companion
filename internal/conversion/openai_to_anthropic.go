package conversion

import (
	"errors"
	
	jsonutils "claude-code-codex-companion/internal/common/json"
)

// ConvertChatResponseJSONToAnthropic converts an OpenAI Chat Completions response
// into an Anthropic message response that Claude Code clients can consume.
func ConvertChatResponseJSONToAnthropic(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return nil, errors.New("empty response body")
	}

	var resp OpenAIResponse
	if err := jsonutils.SafeUnmarshal(body, &resp); err != nil {
		return nil, err
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
		}
	}

	if len(resp.Choices) > 0 {
		internal.FinishReason = resp.Choices[0].FinishReason
		for _, choice := range resp.Choices {
			internal.Messages = append(internal.Messages, openAIMessageToInternal(choice.Message))
		}
	}

	// Consolidate assistant/tool messages into Anthropic content blocks.
	var contentBlocks []AnthropicContentBlock
	for _, msg := range internal.Messages {
		if msg.Role != "assistant" && msg.Role != "tool" && msg.Role != "" {
			continue
		}
		anthMsg := internalMessageToAnthropic(msg)
		contentBlocks = append(contentBlocks, anthMsg.GetContentBlocks()...)
	}

	// Ensure we always return at least one content block to satisfy Anthropic schema.
	if len(contentBlocks) == 0 {
		contentBlocks = []AnthropicContentBlock{
			{Type: "text", Text: ""},
		}
	}

	out := AnthropicResponse{
		ID:      resp.ID,
		Type:    "message",
		Role:    "assistant",
		Model:   resp.Model,
		Content: contentBlocks,
	}

	if internal.FinishReason != "" {
		out.StopReason = normalizeOpenAIFinishReason(internal.FinishReason)
	}

	if resp.Usage != nil {
		out.Usage = &AnthropicUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}

	return jsonutils.SafeMarshal(out)
}
