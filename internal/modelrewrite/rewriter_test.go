package modelrewrite

import (
	"strings"
	"testing"
	"claude-code-codex-companion/internal/logger"
)

func TestSSEResponseRewrite(t *testing.T) {
	// 创建模拟日志器
	logConfig := logger.LogConfig{
		Level:           "debug",
		LogRequestTypes: "all",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "./test_logs",
	}
	mockLogger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	rewriter := NewRewriter(*mockLogger)

	// 模拟SSE响应
	sseResponse := `data: {"type":"message_start","message":{"id":"msg_123","model":"deepseek-chat","role":"assistant"}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn","usage":{"output_tokens":1}}}

data: {"type":"message_stop"}

data: [DONE]
`

	// 测试重写
	result, err := rewriter.RewriteResponse([]byte(sseResponse), "claude-3-haiku-20240307", "deepseek-chat")
	if err != nil {
		t.Fatalf("SSE rewrite failed: %v", err)
	}

	resultStr := string(result)
	
	// 验证原始模型名被正确恢复
	if !strings.Contains(resultStr, `"model":"claude-3-haiku-20240307"`) {
		t.Errorf("Expected original model name not found in result")
		t.Logf("Result: %s", resultStr)
	}
	
	// 验证重写后的模型名被完全替换
	if strings.Contains(resultStr, `"model":"deepseek-chat"`) {
		t.Errorf("Rewritten model name still exists in result")
		t.Logf("Result: %s", resultStr)
	}
}

func TestJSONResponseRewrite(t *testing.T) {
	// 创建模拟日志器
	logConfig := logger.LogConfig{
		Level:           "debug",
		LogRequestTypes: "all",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "./test_logs",
	}
	mockLogger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	rewriter := NewRewriter(*mockLogger)

	// 模拟JSON响应
	jsonResponse := `{"id":"msg_123","model":"deepseek-chat","role":"assistant","content":"Hello"}`

	// 测试重写
	result, err := rewriter.RewriteResponse([]byte(jsonResponse), "claude-3-haiku-20240307", "deepseek-chat")
	if err != nil {
		t.Fatalf("JSON rewrite failed: %v", err)
	}

	resultStr := string(result)
	
	// 验证原始模型名被正确恢复
	if !strings.Contains(resultStr, `"model":"claude-3-haiku-20240307"`) {
		t.Errorf("Expected original model name not found in result")
		t.Logf("Result: %s", resultStr)
	}
}

func TestNoRewriteNeeded(t *testing.T) {
	// 创建模拟日志器
	logConfig := logger.LogConfig{
		Level:           "debug",
		LogRequestTypes: "all",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "./test_logs",
	}
	mockLogger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	rewriter := NewRewriter(*mockLogger)

	// 没有模型字段的响应
	response := `{"id":"msg_123","role":"assistant","content":"Hello"}`

	// 测试重写
	result, err := rewriter.RewriteResponse([]byte(response), "claude-3-haiku-20240307", "deepseek-chat")
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	// 应该保持原样
	if string(result) != response {
		t.Errorf("Response should remain unchanged when no model field present")
	}
}