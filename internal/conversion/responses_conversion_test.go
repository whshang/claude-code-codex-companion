package conversion

import (
	"encoding/json"
	"testing"
)

func TestConvertResponsesRequestJSONToChat(t *testing.T) {
	input := `{
		"model": "gpt-4",
		"input": [
			{"role":"user","content":[{"type":"input_text","text":"Hello"}]},
			{"role":"assistant","content":[{"type":"output_text","text":"Hi there"}]}
		]
	}`

	converted, err := ConvertResponsesRequestJSONToChat([]byte(input))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	var req OpenAIRequest
	if err := json.Unmarshal(converted, &req); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if msg := req.Messages[0]; msg.Role != "user" || msg.Content != "Hello" {
		t.Fatalf("unexpected first message: %+v", msg)
	}
	if msg := req.Messages[1]; msg.Role != "assistant" || msg.Content != "Hi there" {
		t.Fatalf("unexpected second message: %+v", msg)
	}
}

func TestConvertChatResponseJSONToResponses_ToolCall(t *testing.T) {
	input := `{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"model": "gpt-4",
		"choices": [
			{
				"index": 0,
				"finish_reason": "tool_calls",
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [
						{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "search",
								"arguments": "{\"q\":\"hello\"}"
							}
						}
					]
				}
			}
		]
	}`

	converted, err := ConvertChatResponseJSONToResponses([]byte(input))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	var resp OpenAIResponsesResponse
	if err := json.Unmarshal(converted, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Output) < 2 {
		t.Fatalf("expected output to include function call, got %v", resp.Output)
	}

	functionItem := resp.Output[len(resp.Output)-1]
	if functionItem.Type != "function_call" {
		t.Fatalf("expected function_call item, got %s", functionItem.Type)
	}
	if functionItem.Arguments != "{\"q\":\"hello\"}" {
		t.Fatalf("unexpected arguments: %s", functionItem.Arguments)
	}
}
