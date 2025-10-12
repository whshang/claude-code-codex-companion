package conversion

import (
	"fmt"
)

// ConvertOpenAIRequestJSONToAnthropic 将 OpenAI Chat Completions 请求转换为 Anthropic Messages 请求
func ConvertOpenAIRequestJSONToAnthropic(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	openaiAdapter := factory.OpenAIChatAdapter()
	anthropicAdapter := factory.AnthropicAdapter()

	internalReq, err := openaiAdapter.ParseRequestJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize OpenAI chat request: %w", err)
	}

	converted, err := anthropicAdapter.BuildRequestJSON(internalReq)
	if err != nil {
		return nil, fmt.Errorf("failed to render Anthropic request: %w", err)
	}
	return converted, nil
}
