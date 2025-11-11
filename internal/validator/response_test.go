package validator

import (
	"testing"
)

func TestValidateMessageStartUsage(t *testing.T) {
	validator := NewResponseValidator()

	// 测试有效的message_start事件
	validMessageStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"usage": map[string]interface{}{
				"input_tokens":  100,
				"output_tokens": 50,
			},
		},
	}

	err := validator.ValidateMessageStartUsage(validMessageStart)
	if err != nil {
		t.Errorf("Expected valid message_start to pass, got error: %v", err)
	}
}

func TestDetectJSONContent(t *testing.T) {
	validator := NewResponseValidator()

	// 测试有效的JSON
	jsonContent := []byte(`{"id": "test", "model": "gpt-3.5-turbo"}`)
	if !validator.DetectJSONContent(jsonContent) {
		t.Error("Expected valid JSON to be detected as JSON content")
	}

	// 测试SSE内容
	sseContent := []byte(`data: {"id":"test","model":"gpt-3.5-turbo"}`)
	if validator.DetectJSONContent(sseContent) {
		t.Error("Expected SSE content to not be detected as JSON")
	}

	// 测试空内容
	if validator.DetectJSONContent([]byte{}) {
		t.Error("Expected empty content to not be detected as JSON")
	}
}

func TestValidateCompleteSSEStream(t *testing.T) {
	validator := NewResponseValidator()

	// 测试用例1: 完整的Anthropic SSE流应该通过
	completeAnthropicSSE := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","usage":{"input_tokens":100,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_stop
data: {"type":"message_stop"}
`)

	err := validator.ValidateCompleteSSEStream(completeAnthropicSSE, "anthropic", "", "")
	if err != nil {
		t.Errorf("Expected complete Anthropic SSE stream to pass validation, got error: %v", err)
	}

	// 测试用例2: 不完整的Anthropic SSE流（缺少message_stop）应该失败
	incompleteAnthropicSSE := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","usage":{"input_tokens":100,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"text":"Hello"}}
`)

	err = validator.ValidateCompleteSSEStream(incompleteAnthropicSSE, "anthropic", "", "")
	if err == nil {
		t.Error("Expected incomplete Anthropic SSE stream (missing message_stop) to fail validation")
	}
	if !contains(err.Error(), "incomplete SSE stream") {
		t.Errorf("Expected error message to contain 'incomplete SSE stream', got: %v", err)
	}

	// 测试用例3: 完整的OpenAI SSE流应该通过
	completeOpenAISSE := []byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}
data: [DONE]`)

	err = validator.ValidateCompleteSSEStream(completeOpenAISSE, "openai", "/v1/chat/completions", "https://api.openai.com")
	if err != nil {
		t.Errorf("Expected complete OpenAI SSE stream to pass validation, got error: %v", err)
	}

	// 测试用例4: 不完整的OpenAI SSE流（没有完成标志）应该失败
	trueIncompleteSSE := []byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null]}
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":null]}`)

	err = validator.ValidateCompleteSSEStream(trueIncompleteSSE, "openai", "/v1/chat/completions", "https://api.openai.com")
	if err == nil {
		t.Error("Expected incomplete OpenAI SSE stream (missing all completion markers) to fail validation")
	}
	if !contains(err.Error(), "missing finish_reason") && !contains(err.Error(), "missing [DONE]") {
		t.Errorf("Expected error message to contain completion marker error, got: %v", err)
	}

	// 测试用例5: 有finish_reason的流应该通过（新宽松策略）
	incompleteOpenAISSE := []byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)

	err = validator.ValidateCompleteSSEStream(incompleteOpenAISSE, "openai", "/v1/chat/completions", "https://example.com")
	if err != nil {
		t.Errorf("Expected OpenAI SSE with finish_reason to pass validation, got: %v", err)
	}

	// 测试用例6: OpenAI SSE流有response.completed事件应该通过
	responseCompletedSSE := []byte(`event: response.completed
data: {"type":"response.completed","id":"resp-123"}

data: {"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"}]}`)

	err = validator.ValidateCompleteSSEStream(responseCompletedSSE, "openai", "/v1/chat/completions", "https://api.openai.com")
	if err != nil {
		t.Errorf("Expected OpenAI SSE with response.completed to pass validation, got error: %v", err)
	}
}

func TestValidateResponseWithPathStreamingIntegration(t *testing.T) {
	validator := NewResponseValidator()

	// 测试用例1: 完整的Anthropic SSE流应该通过集成验证
	completeAnthropicSSE := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","usage":{"input_tokens":100,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_stop
data: {"type":"message_stop"}
`)

	err := validator.ValidateResponseWithPath(completeAnthropicSSE, true, "anthropic", "/v1/messages", "https://api.anthropic.com")
	if err != nil {
		t.Errorf("Expected complete Anthropic SSE to pass integrated validation, got error: %v", err)
	}

	// 测试用例2: 不完整的Anthropic SSE流应该在集成验证中失败
	incompleteAnthropicSSE := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","usage":{"input_tokens":100,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"text":"Hello"}}
`)

	err = validator.ValidateResponseWithPath(incompleteAnthropicSSE, true, "anthropic", "/v1/messages", "https://api.anthropic.com")
	if err == nil {
		t.Error("Expected incomplete Anthropic SSE to fail integrated validation")
	}
	if !contains(err.Error(), "incomplete SSE stream") {
		t.Errorf("Expected error message to contain 'incomplete SSE stream', got: %v", err)
	}

	// 测试用例3: 完整的OpenAI SSE流应该通过集成验证
	completeOpenAISSE := []byte(`data: {"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}]}
data: [DONE]`)

	err = validator.ValidateResponseWithPath(completeOpenAISSE, true, "openai", "/v1/chat/completions", "https://api.openai.com")
	if err != nil {
		t.Errorf("Expected complete OpenAI SSE to pass integrated validation, got error: %v", err)
	}

	// 测试用例4: 不完整的OpenAI SSE流应该在集成验证中失败
	incompleteOpenAISSE := []byte(`data: {"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}
data: {"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":null}]}`)

	err = validator.ValidateResponseWithPath(incompleteOpenAISSE, true, "openai", "/v1/chat/completions", "https://api.openai.com")
	if err == nil {
		t.Error("Expected incomplete OpenAI SSE to fail integrated validation")
	}
	if !contains(err.Error(), "incomplete") {
		t.Errorf("Expected error message to contain 'incomplete', got: %v", err)
	}

	// 测试用例5: 有finish_reason的流应该通过集成验证（新宽松策略）
	noDoneButFinishedOpenAISSE := []byte(`data: {"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}]}`)
	err = validator.ValidateResponseWithPath(noDoneButFinishedOpenAISSE, true, "openai", "/v1/chat/completions", "https://example.com")
	if err != nil {
		t.Errorf("Expected OpenAI SSE without [DONE] but with finish_reason to pass integrated validation, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
