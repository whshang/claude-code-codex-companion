package conversion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertOpenAIRequestJSONToAnthropic(t *testing.T) {
	openaiReq := `{
        "model": "gpt-5",
        "messages": [
            {"role": "system", "content": "You are helpful"},
            {"role": "user", "content": [{"type": "text", "text": "Hello"}]},
            {"role": "assistant", "content": [{"type": "text", "text": "World"}]}
        ],
        "temperature": 0.4,
        "top_p": 0.9,
        "max_tokens": 128,
        "user": "tester"
    }`

	converted, err := ConvertOpenAIRequestJSONToAnthropic([]byte(openaiReq))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	var anth AnthropicRequest
	if err := json.Unmarshal(converted, &anth); err != nil {
		t.Fatalf("failed to unmarshal anthropic request: %v", err)
	}

	if anth.Model != "gpt-5" {
		t.Fatalf("expected model gpt-5, got %s", anth.Model)
	}
	if anth.MaxTokens == nil || *anth.MaxTokens != 128 {
		t.Fatalf("expected max_tokens 128, got %v", anth.MaxTokens)
	}
	if anth.Temperature == nil || *anth.Temperature != 0.4 {
		t.Fatalf("unexpected temperature: %v", anth.Temperature)
	}
	if anth.Metadata["user_id"] != "tester" {
		t.Fatalf("expected metadata.user_id tester, got %v", anth.Metadata["user_id"])
	}
	if len(anth.Messages) != 2 {
		t.Fatalf("expected 2 messages (excluding system), got %d", len(anth.Messages))
	}
	if anth.System != "You are helpful" {
		t.Fatalf("expected system prompt preserved, got %s", anth.System)
	}
}

func TestConvertAnthropicResponseJSONToOpenAI(t *testing.T) {
	anthropicResp := `{
        "id": "msg_123",
        "type": "message",
        "role": "assistant",
        "model": "claude-sonnet",
        "stop_reason": "end_turn",
        "usage": {"input_tokens": 10, "output_tokens": 5},
        "content": [
            {"type": "text", "text": "Hello OpenAI"}
        ]
    }`

	converted, err := ConvertAnthropicResponseJSONToOpenAI([]byte(anthropicResp))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	var oa OpenAIResponse
	if err := json.Unmarshal(converted, &oa); err != nil {
		t.Fatalf("failed to unmarshal openai response: %v", err)
	}

	if len(oa.Choices) != 1 {
		t.Fatalf("expected single choice, got %d", len(oa.Choices))
	}
	if oa.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected finish_reason stop, got %s", oa.Choices[0].FinishReason)
	}
	if oa.Usage == nil || oa.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage conversion: %+v", oa.Usage)
	}
}

func TestConvertAnthropicSSEToOpenAI(t *testing.T) {
	anthropicSSE := "" +
		"event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_456\",\"model\":\"claude-sonnet\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\",\"stop_reason\":\"end_turn\"}\n\n"

	converted, err := ConvertAnthropicSSEToOpenAI([]byte(anthropicSSE))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	result := string(converted)
	if !strings.Contains(result, "chat.completion.chunk") {
		t.Fatalf("expected chat.completion.chunk in converted SSE: %s", result)
	}
	if !strings.Contains(result, "[DONE]") {
		t.Fatalf("expected DONE terminator in converted SSE")
	}
}

func TestStreamChatCompletionsToResponses(t *testing.T) {
	input := "" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-test\",\"choices\":[{\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-test\",\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-test\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"

	var buf strings.Builder
	if err := StreamChatCompletionsToResponses(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("stream conversion failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "response.created") {
		t.Fatalf("expected response.created event in output: %s", output)
	}
	if !strings.Contains(output, "response.output_text.delta") {
		t.Fatalf("expected response.output_text.delta event in output: %s", output)
	}
	if !strings.Contains(output, "response.completed") {
		t.Fatalf("expected response.completed event in output: %s", output)
	}
}

func TestStreamAnthropicSSEToOpenAI(t *testing.T) {
	input := "" +
		"event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-test\",\"usage\":{\"input_tokens\":5}}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"},\"content_block\":{\"id\":\"cb1\"}}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":5,\"output_tokens\":3}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\",\"stop_reason\":\"end_turn\"}\n\n"

	var buf strings.Builder
	if err := StreamAnthropicSSEToOpenAI(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("stream anthropic->openai failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "chat.completion.chunk") {
		t.Fatalf("expected chat.completion.chunk in output: %s", output)
	}
	if !strings.Contains(output, "[DONE]") {
		t.Fatalf("expected [DONE] marker in output: %s", output)
	}
}
