// ==================================================================================
//
// DEPRECATED: This file contains logic for a conversion adapter factory.
// As of the October 2025 refactor, the proxy architecture has shifted to static handlers,
// where each route instantiates its required adapter directly (e.g., NewOpenAIChatAdapter).
// This factory is no longer used by the main proxy logic and is slated for removal.
// See internal/proxy/server.go for the new handler implementations.
//
// ==================================================================================
package conversion

import "claude-code-codex-companion/internal/logger"

// FormatAdapter 定义统一的格式适配器接口
type FormatAdapter interface {
	Name() string
	ParseRequestJSON(payload []byte) (*InternalRequest, error)
	BuildRequestJSON(req *InternalRequest) ([]byte, error)
	ParseResponseJSON(payload []byte) (*InternalResponse, error)
	BuildResponseJSON(resp *InternalResponse) ([]byte, error)
	ParseSSE(event string, data []byte) ([]InternalEvent, error)
	BuildSSE(events []InternalEvent) ([]SSEPayload, error)
}

// SSEPayload 描述单条 SSE 输出
type SSEPayload struct {
	Event string
	Data  interface{}
}

// AdapterFactory 统一创建适配器实例
type AdapterFactory struct {
	logger *logger.Logger
}

// NewAdapterFactory 创建适配器工厂
func NewAdapterFactory(logger *logger.Logger) *AdapterFactory {
	return &AdapterFactory{logger: logger}
}

// OpenAIChatAdapter 构造 OpenAI Chat 适配器
func (f *AdapterFactory) OpenAIChatAdapter() FormatAdapter {
	return &OpenAIChatFormatAdapter{
		logger: f.logger,
	}
}

// OpenAIResponsesAdapter 构造 OpenAI Responses 适配器
func (f *AdapterFactory) OpenAIResponsesAdapter() FormatAdapter {
	return &OpenAIResponsesFormatAdapter{
		logger: f.logger,
	}
}

// AnthropicAdapter 构造 Anthropic 适配器
func (f *AdapterFactory) AnthropicAdapter() FormatAdapter {
	return &AnthropicFormatAdapter{
		logger: f.logger,
	}
}

// GeminiAdapter 构造 Gemini 适配器
func (f *AdapterFactory) GeminiAdapter() FormatAdapter {
	return NewGeminiFormatAdapter()
}
