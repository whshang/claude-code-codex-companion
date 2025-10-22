package conversion

import (
	"bytes"
	"strings"
	"testing"
)

func TestStreamOpenAISSEToAnthropic_Text(t *testing.T) {
	openaiSSE := `data: {"id":"chatcmpl-123","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"}}]}
data: {"id":"chatcmpl-123","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello, world!"}}]}
data: {"id":"chatcmpl-123","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}

data: [DONE]
`

	reader := strings.NewReader(openaiSSE)
	var writer bytes.Buffer

	if err := StreamOpenAISSEToAnthropic(reader, &writer); err != nil {
		t.Fatalf("StreamOpenAISSEToAnthropic failed: %v", err)
	}

	output := writer.String()
	if !strings.Contains(output, "event: message_start") {
		t.Errorf("expected message_start event, got %s", output)
	}
	if !strings.Contains(output, "text_delta") {
		t.Errorf("expected text_delta content block, got %s", output)
	}
	if !strings.Contains(output, "message_stop") {
		t.Errorf("expected message_stop event, got %s", output)
	}
	if !strings.Contains(output, `"input_tokens":10`) || !strings.Contains(output, `"output_tokens":5`) {
		t.Errorf("expected usage information in message_delta, got %s", output)
	}
}

func TestStreamOpenAISSEToAnthropic_ToolCall(t *testing.T) {
	openaiSSE := `data: {"id":"chatcmpl-456","model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"do_something","arguments":"{\"foo\":1}"}}]}}]}
data: {"id":"chatcmpl-456","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`

	reader := strings.NewReader(openaiSSE)
	var writer bytes.Buffer

	if err := StreamOpenAISSEToAnthropic(reader, &writer); err != nil {
		t.Fatalf("StreamOpenAISSEToAnthropic failed: %v", err)
	}

	output := writer.String()
	if !strings.Contains(output, "tool_use") {
		t.Errorf("expected tool_use content block, got %s", output)
	}
	if !strings.Contains(output, "input_json_delta") {
		t.Errorf("expected input_json_delta for tool arguments, got %s", output)
	}
	if !strings.Contains(output, `"stop_reason":"tool_use"`) {
		t.Errorf("expected stop_reason tool_use, got %s", output)
	}
}
