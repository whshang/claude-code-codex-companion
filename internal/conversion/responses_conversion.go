package conversion

import "fmt"

// ConvertResponsesRequestJSONToChat 将 OpenAI Responses 请求转换为 Chat Completions 请求
func ConvertResponsesRequestJSONToChat(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	responsesAdapter := factory.OpenAIResponsesAdapter()
	chatAdapter := factory.OpenAIChatAdapter()

	internalReq, err := responsesAdapter.ParseRequestJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize responses request: %w", err)
	}

	converted, err := chatAdapter.BuildRequestJSON(internalReq)
	if err != nil {
		return nil, fmt.Errorf("failed to render chat request: %w", err)
	}
	return converted, nil
}

// ConvertResponsesResponseJSONToChat 将 OpenAI Responses 响应转换为 Chat Completions 响应
func ConvertResponsesResponseJSONToChat(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	responsesAdapter := factory.OpenAIResponsesAdapter()
	chatAdapter := factory.OpenAIChatAdapter()

	internalResp, err := responsesAdapter.ParseResponseJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize responses response: %w", err)
	}

	converted, err := chatAdapter.BuildResponseJSON(internalResp)
	if err != nil {
		return nil, fmt.Errorf("failed to render chat response: %w", err)
	}
	return converted, nil
}

// Anthropic <-> Responses JSON conversions removed (cross-family conversion no longer supported)

// ConvertChatRequestJSONToResponses 将 Chat Completions 请求转换为 Responses 请求
func ConvertChatRequestJSONToResponses(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	chatAdapter := factory.OpenAIChatAdapter()
	responsesAdapter := factory.OpenAIResponsesAdapter()

	internalReq, err := chatAdapter.ParseRequestJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize chat request: %w", err)
	}

	converted, err := responsesAdapter.BuildRequestJSON(internalReq)
	if err != nil {
		return nil, fmt.Errorf("failed to render responses request: %w", err)
	}
	return converted, nil
}

// ConvertChatResponseJSONToResponses 将 Chat Completions 响应转换为 Responses 响应
func ConvertChatResponseJSONToResponses(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	chatAdapter := factory.OpenAIChatAdapter()
	responsesAdapter := factory.OpenAIResponsesAdapter()

	internalResp, err := chatAdapter.ParseResponseJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize chat response: %w", err)
	}

	converted, err := responsesAdapter.BuildResponseJSON(internalResp)
	if err != nil {
		return nil, fmt.Errorf("failed to render responses response: %w", err)
	}
	return converted, nil
}

// ConvertAnthropicResponseJSONToChat 将 Anthropic 响应转换为 Chat Completions 响应
func ConvertAnthropicResponseJSONToChat(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	anthropicAdapter := factory.AnthropicAdapter()
	chatAdapter := factory.OpenAIChatAdapter()

	internalResp, err := anthropicAdapter.ParseResponseJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize anthropic response: %w", err)
	}

	converted, err := chatAdapter.BuildResponseJSON(internalResp)
	if err != nil {
		return nil, fmt.Errorf("failed to render chat response: %w", err)
	}
	return converted, nil
}