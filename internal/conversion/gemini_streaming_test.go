package conversion

import (
	"bytes"
	"strings"
	"testing"
)

func TestStreamGeminiSSEToOpenAI(t *testing.T) {
	// Gemini SSE 示例数据
	geminiSSE := `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}

data: [DONE]
`

	reader := strings.NewReader(geminiSSE)
	var writer bytes.Buffer

	err := StreamGeminiSSEToOpenAI(reader, &writer)
	if err != nil {
		t.Fatalf("StreamGeminiSSEToOpenAI failed: %v", err)
	}

	result := writer.String()
	if !strings.Contains(result, "data: ") {
		t.Error("Expected OpenAI SSE format with 'data: ' prefix")
	}
	if !strings.Contains(result, "[DONE]") {
		t.Error("Expected [DONE] marker in output")
	}
}

func TestStreamGeminiSSEToAnthropic(t *testing.T) {
	// Gemini SSE 示例数据
	geminiSSE := `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]},"finishReason":"STOP"}]}

data: [DONE]
`

	reader := strings.NewReader(geminiSSE)
	var writer bytes.Buffer

	err := StreamGeminiSSEToAnthropic(reader, &writer)
	if err != nil {
		t.Fatalf("StreamGeminiSSEToAnthropic failed: %v", err)
	}

	result := writer.String()
	if !strings.Contains(result, "event: ") {
		t.Error("Expected Anthropic SSE format with 'event: ' prefix")
	}
	if !strings.Contains(result, "message_stop") {
		t.Error("Expected message_stop event in output")
	}
}
