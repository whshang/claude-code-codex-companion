package conversion

import (
	"bytes"
	"fmt"
)

// ConvertAnthropicResponseJSONToOpenAI 将 Anthropic 非流式响应 JSON 转换为 OpenAI Chat Completions JSON。
func ConvertAnthropicResponseJSONToOpenAI(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	anthropicAdapter := factory.AnthropicAdapter()
	openaiAdapter := factory.OpenAIChatAdapter()

	internalResp, err := anthropicAdapter.ParseResponseJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize Anthropic response: %w", err)
	}

	converted, err := openaiAdapter.BuildResponseJSON(internalResp)
	if err != nil {
		return nil, fmt.Errorf("failed to render OpenAI chat response: %w", err)
	}
	return converted, nil
}

// ConvertAnthropicSSEToOpenAI 将 Anthropic SSE 转换为 OpenAI SSE（最少一段 chunk）。
func ConvertAnthropicSSEToOpenAI(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := StreamAnthropicSSEToOpenAI(bytes.NewReader(body), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
