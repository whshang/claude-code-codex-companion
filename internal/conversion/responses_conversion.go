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

// ConvertAnthropicResponseJSONToResponses 将 Anthropic 响应转换为 OpenAI Responses 响应
func ConvertAnthropicResponseJSONToResponses(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	anthropicAdapter := factory.AnthropicAdapter()
	responsesAdapter := factory.OpenAIResponsesAdapter()

	internalResp, err := anthropicAdapter.ParseResponseJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize Anthropic response: %w", err)
	}

	converted, err := responsesAdapter.BuildResponseJSON(internalResp)
	if err != nil {
		return nil, fmt.Errorf("failed to render responses payload: %w", err)
	}
	return converted, nil
}

// ConvertResponsesResponseJSONToAnthropic 将 OpenAI Responses 响应转换为 Anthropic 响应
func ConvertResponsesResponseJSONToAnthropic(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	responsesAdapter := factory.OpenAIResponsesAdapter()
	anthropicAdapter := factory.AnthropicAdapter()

	internalResp, err := responsesAdapter.ParseResponseJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize responses response: %w", err)
	}

	converted, err := anthropicAdapter.BuildResponseJSON(internalResp)
	if err != nil {
		return nil, fmt.Errorf("failed to render Anthropic response: %w", err)
	}
	return converted, nil
}

// ConvertResponsesRequestJSONToAnthropic 将 OpenAI Responses 请求转换为 Anthropic 请求
func ConvertResponsesRequestJSONToAnthropic(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	responsesAdapter := factory.OpenAIResponsesAdapter()
	anthropicAdapter := factory.AnthropicAdapter()

	internalReq, err := responsesAdapter.ParseRequestJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize responses request: %w", err)
	}

	converted, err := anthropicAdapter.BuildRequestJSON(internalReq)
	if err != nil {
		return nil, fmt.Errorf("failed to render Anthropic request: %w", err)
	}
	return converted, nil
}

// ConvertAnthropicRequestJSONToResponses 将 Anthropic 请求转换为 OpenAI Responses 请求
func ConvertAnthropicRequestJSONToResponses(body []byte) ([]byte, error) {
	factory := NewAdapterFactory(nil)
	anthropicAdapter := factory.AnthropicAdapter()
	responsesAdapter := factory.OpenAIResponsesAdapter()

	internalReq, err := anthropicAdapter.ParseRequestJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize Anthropic request: %w", err)
	}

	converted, err := responsesAdapter.BuildRequestJSON(internalReq)
	if err != nil {
		return nil, fmt.Errorf("failed to render responses request: %w", err)
	}
	return converted, nil
}

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
