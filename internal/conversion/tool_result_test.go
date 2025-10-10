package conversion

import (
	"encoding/json"
	"testing"

	"claude-code-codex-companion/internal/logger"
)

// getTestLogger creates a test logger
func getTestLoggerForToolResult() *logger.Logger {
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level:           "debug",
		LogRequestTypes: "all",
		LogDirectory:    "",
	})
	return testLogger
}

// TestToolResultWithNestedContent 测试嵌套 content 中的 tool_use_id 是否正确解析
func TestToolResultWithNestedContent(t *testing.T) {
	// 模拟 Claude Code 发送的包含 tool_result 的请求
	// tool_result 的 content 是数组格式，包含嵌套的 tool_use_id
	requestJSON := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_123456",
						"content": [
							{
								"type": "text",
								"text": "Tool execution result"
							}
						]
					}
				]
			}
		],
		"max_tokens": 1024
	}`

	// 转换为 OpenAI 格式
	converter := NewRequestConverter(getTestLoggerForToolResult())
	resultBytes, _, err := converter.Convert([]byte(requestJSON), &EndpointInfo{Type: "openai"})
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	var openaiReq OpenAIRequest
	if err := json.Unmarshal(resultBytes, &openaiReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证转换结果
	if len(openaiReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(openaiReq.Messages))
	}

	msg := openaiReq.Messages[0]
	if msg.Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", msg.Role)
	}

	if msg.ToolCallID != "toolu_123456" {
		t.Errorf("Expected ToolCallID 'toolu_123456', got '%s'", msg.ToolCallID)
	}

	expectedContent := "Tool execution result"
	if msg.Content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, msg.Content)
	}

	t.Logf("✅ Tool result with nested content parsed successfully")
	t.Logf("   ToolCallID: %s", msg.ToolCallID)
	t.Logf("   Content: %s", msg.Content)
}

// TestToolResultWithStringContent 测试字符串格式 content 的 tool_result
func TestToolResultWithStringContent(t *testing.T) {
	requestJSON := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_789",
						"content": "Simple string result"
					}
				]
			}
		],
		"max_tokens": 1024
	}`

	converter := NewRequestConverter(getTestLoggerForToolResult())
	resultBytes, _, err := converter.Convert([]byte(requestJSON), &EndpointInfo{Type: "openai"})
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	var openaiReq OpenAIRequest
	if err := json.Unmarshal(resultBytes, &openaiReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(openaiReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(openaiReq.Messages))
	}

	msg := openaiReq.Messages[0]
	if msg.ToolCallID != "toolu_789" {
		t.Errorf("Expected ToolCallID 'toolu_789', got '%s'", msg.ToolCallID)
	}

	if msg.Content != "Simple string result" {
		t.Errorf("Expected content 'Simple string result', got '%s'", msg.Content)
	}

	t.Logf("✅ Tool result with string content parsed successfully")
}

// TestToolResultWithError 测试带 is_error 的 tool_result
func TestToolResultWithError(t *testing.T) {
	requestJSON := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_error",
						"content": [
							{
								"type": "text",
								"text": "Error occurred"
							}
						],
						"is_error": true
					}
				]
			}
		],
		"max_tokens": 1024
	}`

	var anthReq AnthropicRequest
	if err := json.Unmarshal([]byte(requestJSON), &anthReq); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// 验证 is_error 字段是否正确解析
	msg := anthReq.Messages[0]
	blocks := msg.GetContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(blocks))
	}

	block := blocks[0]
	if block.Type != "tool_result" {
		t.Errorf("Expected type 'tool_result', got '%s'", block.Type)
	}

	if block.ToolUseID != "toolu_error" {
		t.Errorf("Expected ToolUseID 'toolu_error', got '%s'", block.ToolUseID)
	}

	if block.IsError == nil || !*block.IsError {
		t.Errorf("Expected IsError to be true")
	}

	t.Logf("✅ Tool result with error flag parsed successfully")
}

// TestMixedUserContent 测试用户消息混合 text 和 tool_result
func TestMixedUserContent(t *testing.T) {
	requestJSON := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{
				"role": "assistant",
				"content": [
					{
						"type": "text",
						"text": "I'll use a tool."
					},
					{
						"type": "tool_use",
						"id": "toolu_abc",
						"name": "get_weather",
						"input": {"city": "Tokyo"}
					}
				]
			},
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_abc",
						"content": [
							{
								"type": "text",
								"text": "Weather: Sunny, 25°C"
							}
						]
					},
					{
						"type": "text",
						"text": "Thanks! What about tomorrow?"
					}
				]
			}
		],
		"max_tokens": 1024
	}`

	converter := NewRequestConverter(getTestLoggerForToolResult())
	resultBytes, _, err := converter.Convert([]byte(requestJSON), &EndpointInfo{Type: "openai"})
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	var openaiReq OpenAIRequest
	if err := json.Unmarshal(resultBytes, &openaiReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 应该产生 3 条消息：assistant (tool_call), tool (result), user (text)
	if len(openaiReq.Messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(openaiReq.Messages))
	}

	// 验证第一条：assistant with tool_call
	if openaiReq.Messages[0].Role != "assistant" {
		t.Errorf("Message 0: Expected role 'assistant', got '%s'", openaiReq.Messages[0].Role)
	}
	if len(openaiReq.Messages[0].ToolCalls) != 1 {
		t.Errorf("Message 0: Expected 1 tool call, got %d", len(openaiReq.Messages[0].ToolCalls))
	}

	// 验证第二条：tool result
	if openaiReq.Messages[1].Role != "tool" {
		t.Errorf("Message 1: Expected role 'tool', got '%s'", openaiReq.Messages[1].Role)
	}
	if openaiReq.Messages[1].ToolCallID != "toolu_abc" {
		t.Errorf("Message 1: Expected ToolCallID 'toolu_abc', got '%s'", openaiReq.Messages[1].ToolCallID)
	}

	// 验证第三条：user text
	if openaiReq.Messages[2].Role != "user" {
		t.Errorf("Message 2: Expected role 'user', got '%s'", openaiReq.Messages[2].Role)
	}
	expectedUserContent := "Thanks! What about tomorrow?"
	if openaiReq.Messages[2].Content != expectedUserContent {
		t.Errorf("Message 2: Expected content '%s', got '%s'", expectedUserContent, openaiReq.Messages[2].Content)
	}

	t.Logf("✅ Mixed user content (tool_result + text) parsed successfully")
}

