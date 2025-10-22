package conversion

import (
	"encoding/json"
	"testing"
)

func TestConvertChatResponseJSONToAnthropic_TextOnly(t *testing.T) {
	input := `{"id":"chatcmpl-123","model":"gpt-4","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"Hello World"}}]}`
	output, err := ConvertChatResponseJSONToAnthropic([]byte(input))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	var anthropic AnthropicResponse
	if err := json.Unmarshal(output, &anthropic); err != nil {
		t.Fatalf("invalid anthropic JSON: %v", err)
	}

	if anthropic.Type != "message" {
		t.Errorf("expected message type, got %s", anthropic.Type)
	}
	if anthropic.StopReason != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %s", anthropic.StopReason)
	}
	if len(anthropic.Content) != 1 || anthropic.Content[0].Text != "Hello World" {
		t.Fatalf("unexpected content blocks: %+v", anthropic.Content)
	}
}

func TestConvertChatResponseJSONToAnthropic_ToolCall(t *testing.T) {
	input := `{
		"id": "chatcmpl-456",
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"finish_reason": "tool_calls",
			"message": {
				"role": "assistant",
				"tool_calls": [{
					"id": "tool_1",
					"type": "function",
					"function": {
						"name": "search",
						"arguments": "{\"query\":\"weather\"}"
					}
				}]
			}
		}]
	}`

	output, err := ConvertChatResponseJSONToAnthropic([]byte(input))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	var anthropic AnthropicResponse
	if err := json.Unmarshal(output, &anthropic); err != nil {
		t.Fatalf("invalid anthropic JSON: %v", err)
	}

	if anthropic.StopReason != "tool_use" {
		t.Errorf("expected stop_reason tool_use, got %s", anthropic.StopReason)
	}
	foundTool := false
	for _, block := range anthropic.Content {
		if block.Type == "tool_use" && block.Name == "search" {
			foundTool = true
			break
		}
	}
	if !foundTool {
		t.Fatalf("expected tool_use block in content: %+v", anthropic.Content)
	}
}
