package conversion

import (
	"encoding/json"
	"testing"
)

// 测试 Chat → Internal → Responses 完整转换链（包含所有新字段）
func TestChatToResponsesFullFieldMapping(t *testing.T) {
	// 构造包含所有字段的 Chat Completions 请求
	chatJSON := `{
		"model": "gpt-4o-mini",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello"}
		],
		"temperature": 0.7,
		"top_p": 0.9,
		"max_completion_tokens": 1024,
		"presence_penalty": 0.5,
		"frequency_penalty": 0.3,
		"logit_bias": {"123": 2.0, "456": -1.5},
		"n": 3,
		"stop": ["STOP", "END"],
		"user": "test-user",
		"parallel_tool_calls": true,
		"response_format": {
			"type": "json_object",
			"schema": {"type": "object", "properties": {"result": {"type": "string"}}}
		},
		"tools": [{
			"type": "function",
			"function": {
				"name": "search",
				"description": "Search the web",
				"parameters": {"type": "object", "properties": {"query": {"type": "string"}}}
			}
		}],
		"tool_choice": "auto"
	}`

	factory := NewAdapterFactory(nil)
	chatAdapter := factory.OpenAIChatAdapter()
	responsesAdapter := factory.OpenAIResponsesAdapter()

	// 阶段1: Chat → Internal
	internalReq, err := chatAdapter.ParseRequestJSON([]byte(chatJSON))
	if err != nil {
		t.Fatalf("ParseRequestJSON failed: %v", err)
	}

	// 验证 Internal 结构所有字段
	if internalReq.Model != "gpt-4o-mini" {
		t.Errorf("Expected model gpt-4o-mini, got %s", internalReq.Model)
	}
	if internalReq.Temperature == nil || *internalReq.Temperature != 0.7 {
		t.Errorf("Temperature not mapped correctly")
	}
	if internalReq.TopP == nil || *internalReq.TopP != 0.9 {
		t.Errorf("TopP not mapped correctly")
	}
	if internalReq.MaxCompletionTokens == nil || *internalReq.MaxCompletionTokens != 1024 {
		t.Errorf("MaxCompletionTokens not mapped correctly")
	}
	// 🆕 验证采样参数
	if internalReq.PresencePenalty == nil || *internalReq.PresencePenalty != 0.5 {
		t.Errorf("PresencePenalty not mapped correctly")
	}
	if internalReq.FrequencyPenalty == nil || *internalReq.FrequencyPenalty != 0.3 {
		t.Errorf("FrequencyPenalty not mapped correctly")
	}
	if len(internalReq.LogitBias) != 2 {
		t.Errorf("LogitBias not mapped correctly, got %d items", len(internalReq.LogitBias))
	}
	if internalReq.N == nil || *internalReq.N != 3 {
		t.Errorf("N not mapped correctly")
	}
	if len(internalReq.Stop) != 2 {
		t.Errorf("Stop sequences not mapped correctly")
	}
	// 🆕 验证 ResponseFormat
	if internalReq.ResponseFormat == nil {
		t.Fatal("ResponseFormat should not be nil")
	}
	if internalReq.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat.Type expected json_object, got %s", internalReq.ResponseFormat.Type)
	}
	if len(internalReq.ResponseFormat.Schema) == 0 {
		t.Error("ResponseFormat.Schema should not be empty")
	}

	// 阶段2: Internal → Responses
	responsesJSON, err := responsesAdapter.BuildRequestJSON(internalReq)
	if err != nil {
		t.Fatalf("BuildRequestJSON failed: %v", err)
	}

	var responsesReq OpenAIResponsesRequest
	if err := json.Unmarshal(responsesJSON, &responsesReq); err != nil {
		t.Fatalf("Failed to unmarshal responses request: %v", err)
	}

	// 验证 Responses 结构所有字段
	if responsesReq.Model != "gpt-4o-mini" {
		t.Errorf("Responses model mismatch")
	}
	if responsesReq.PresencePenalty == nil || *responsesReq.PresencePenalty != 0.5 {
		t.Errorf("Responses PresencePenalty not preserved")
	}
	if responsesReq.FrequencyPenalty == nil || *responsesReq.FrequencyPenalty != 0.3 {
		t.Errorf("Responses FrequencyPenalty not preserved")
	}
	if len(responsesReq.LogitBias) != 2 {
		t.Errorf("Responses LogitBias not preserved")
	}
	if responsesReq.N == nil || *responsesReq.N != 3 {
		t.Errorf("Responses N not preserved")
	}
	if len(responsesReq.Stop) != 2 {
		t.Errorf("Responses Stop not preserved")
	}
	if responsesReq.ResponseFormat == nil || responsesReq.ResponseFormat.Type != "json_object" {
		t.Error("Responses ResponseFormat not preserved")
	}
}

// 测试 Responses → Internal → Chat 完整转换链（包含双路径回退）
func TestResponsesToChatFullFieldMapping(t *testing.T) {
	// 测试 input 字段路径
	responsesJSON := `{
		"model": "gpt-4o",
		"input": [
			{"role": "user", "content": [{"type": "input_text", "text": "Hello from input"}]}
		],
		"temperature": 0.8,
		"presence_penalty": 1.0,
		"frequency_penalty": 0.5,
		"logit_bias": {"789": 3.0},
		"n": 2,
		"stop": ["TERMINATE"],
		"response_format": {"type": "json_schema", "schema": {"type": "object"}}
	}`

	factory := NewAdapterFactory(nil)
	responsesAdapter := factory.OpenAIResponsesAdapter()
	chatAdapter := factory.OpenAIChatAdapter()

	// 阶段1: Responses → Internal
	internalReq, err := responsesAdapter.ParseRequestJSON([]byte(responsesJSON))
	if err != nil {
		t.Fatalf("ParseRequestJSON failed: %v", err)
	}

	// 验证所有字段
	if internalReq.PresencePenalty == nil || *internalReq.PresencePenalty != 1.0 {
		t.Errorf("PresencePenalty not parsed correctly")
	}
	if internalReq.N == nil || *internalReq.N != 2 {
		t.Errorf("N not parsed correctly")
	}
	if len(internalReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(internalReq.Messages))
	}
	if len(internalReq.Messages[0].Contents) == 0 || internalReq.Messages[0].Contents[0].Text != "Hello from input" {
		t.Errorf("Message content not parsed from input field")
	}

	// 阶段2: Internal → Chat
	chatJSON, err := chatAdapter.BuildRequestJSON(internalReq)
	if err != nil {
		t.Fatalf("BuildRequestJSON failed: %v", err)
	}

	var chatReq OpenAIRequest
	if err := json.Unmarshal(chatJSON, &chatReq); err != nil {
		t.Fatalf("Failed to unmarshal chat request: %v", err)
	}

	// 验证 Chat 结构保留了所有字段
	if chatReq.PresencePenalty == nil || *chatReq.PresencePenalty != 1.0 {
		t.Errorf("Chat PresencePenalty not preserved")
	}
	if chatReq.N == nil || *chatReq.N != 2 {
		t.Errorf("Chat N not preserved")
	}
	if chatReq.ResponseFormat == nil || chatReq.ResponseFormat.Type != "json_schema" {
		t.Error("Chat ResponseFormat not preserved")
	}
}

// 测试双路径回退：messages 作为 input 的回退
func TestResponsesMessagesFallback(t *testing.T) {
	// 使用 messages 而不是 input
	responsesJSON := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": [{"type": "input_text", "text": "Hello from messages"}]}
		]
	}`

	factory := NewAdapterFactory(nil)
	responsesAdapter := factory.OpenAIResponsesAdapter()

	internalReq, err := responsesAdapter.ParseRequestJSON([]byte(responsesJSON))
	if err != nil {
		t.Fatalf("ParseRequestJSON failed: %v", err)
	}

	if len(internalReq.Messages) != 1 {
		t.Fatalf("Expected 1 message from messages fallback, got %d", len(internalReq.Messages))
	}
	if internalReq.Messages[0].Contents[0].Text != "Hello from messages" {
		t.Errorf("Message content not parsed from messages fallback")
	}
}

// 测试 LogitBias 克隆（防止共享引用）
func TestLogitBiasCloning(t *testing.T) {
	original := map[string]float64{"123": 1.0, "456": -1.0}
	cloned := cloneLogitBias(original)

	// 修改克隆不应影响原始
	cloned["123"] = 999.0

	if original["123"] != 1.0 {
		t.Errorf("Original LogitBias was mutated, expected 1.0 got %f", original["123"])
	}
	if cloned["123"] != 999.0 {
		t.Errorf("Cloned LogitBias not independent")
	}
}

// 测试 ResponseFormat 转换
func TestResponseFormatConversion(t *testing.T) {
	openaiRF := &OpenAIResponseFormat{
		Type: "json_schema",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
		},
	}

	// OpenAI → Internal
	internalRF := convertOpenAIResponseFormatToInternal(openaiRF)
	if internalRF == nil {
		t.Fatal("Internal ResponseFormat should not be nil")
	}
	if internalRF.Type != "json_schema" {
		t.Errorf("Type not converted correctly")
	}
	if len(internalRF.Schema) == 0 {
		t.Error("Schema not converted")
	}

	// Internal → OpenAI
	convertedBack := convertInternalResponseFormatToOpenAI(internalRF)
	if convertedBack == nil {
		t.Fatal("Converted back ResponseFormat should not be nil")
	}
	if convertedBack.Type != "json_schema" {
		t.Errorf("Type not converted back correctly")
	}
	if len(convertedBack.Schema) == 0 {
		t.Error("Schema not converted back")
	}

	// 测试 nil 处理
	if convertOpenAIResponseFormatToInternal(nil) != nil {
		t.Error("Should return nil for nil input")
	}
	if convertInternalResponseFormatToOpenAI(nil) != nil {
		t.Error("Should return nil for nil input")
	}
}

// 测试边界情况：空 LogitBias 和 Stop
func TestEmptyCollections(t *testing.T) {
	chatJSON := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "test"}],
		"logit_bias": {},
		"stop": []
	}`

	factory := NewAdapterFactory(nil)
	chatAdapter := factory.OpenAIChatAdapter()

	internalReq, err := chatAdapter.ParseRequestJSON([]byte(chatJSON))
	if err != nil {
		t.Fatalf("ParseRequestJSON failed: %v", err)
	}

	// 空 LogitBias 应该被保留为非nil空map，空 Stop 可以是nil（因为使用append([]string(nil)会创建新切片）
	if internalReq.LogitBias == nil {
		t.Error("Empty LogitBias should not be nil")
	}
	if len(internalReq.LogitBias) != 0 {
		t.Error("Empty LogitBias should have 0 items")
	}
	// Stop 使用 append([]string(nil), ...) 创建，所以空数组会变成nil，这是正常的
	if len(internalReq.Stop) != 0 {
		t.Error("Empty Stop should have 0 items")
	}
}
