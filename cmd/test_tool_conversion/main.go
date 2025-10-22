//go:build ignore

package main

import (
	"fmt"
	"os"

	"claude-code-codex-companion/internal/conversion"
)

func main() {
	// 模拟一个包含 tool_calls 的 Chat Completions 响应
	mockResponse := `{
  "id": "chatcmpl-test123",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "test-model",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "",
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "glob_search",
              "arguments": "{\"pattern\": \"**/*.yaml\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 50,
    "completion_tokens": 20,
    "total_tokens": 70
  }
}`

	fmt.Println("=== Original Chat Completions Response ===")
	fmt.Println(mockResponse)
	fmt.Println()

	result, err := conversion.ConvertChatResponseJSONToResponses([]byte(mockResponse))
	if err != nil {
		fmt.Println("Error converting:", err)
		os.Exit(1)
	}

	fmt.Println("=== Converted Responses API Format ===")
	fmt.Println(string(result))
}
