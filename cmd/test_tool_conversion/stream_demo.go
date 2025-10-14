package main

import (
	"bytes"
	"fmt"
	"os"

	"claude-code-codex-companion/internal/conversion"
)

func main() {
	// 测试工具调用的流式转换
	// 模拟包含工具调用的上游Chat Completions SSE流
	mockChatSSEWithTool := `data: {"id":"test123","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}

data: {"id":"test123","model":"test-model","choices":[{"index":0,"delta":{"content":"I'll help you calculate that."}}]}

data: {"id":"test123","model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc123","function":{"name":"calculator","arguments":"{\"expression\":\"123 + 456\"}"}}]}}]}

data: {"id":"test123","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`

	fmt.Println("=== Original Chat Completions SSE with Tool Calls ===")
	fmt.Println(mockChatSSEWithTool)
	fmt.Println()

	reader := bytes.NewBufferString(mockChatSSEWithTool)
	var writer bytes.Buffer

	err := conversion.StreamChatToResponsesRealtime(reader, &writer)
	if err != nil {
		fmt.Println("Error converting:", err)
		os.Exit(1)
	}

	fmt.Println("=== Converted Responses API SSE with Tool Calls ===")
	fmt.Println(writer.String())
	
	// 也测试基本的文本转换
	fmt.Println("\n=== Testing basic text conversion ===")
	basicChatSSE := `data: {"id":"test123","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}

data: {"id":"test123","model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: {"id":"test123","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`

	reader2 := bytes.NewBufferString(basicChatSSE)
	var writer2 bytes.Buffer

	err = conversion.StreamChatToResponsesRealtime(reader2, &writer2)
	if err != nil {
		fmt.Println("Error converting basic chat:", err)
		os.Exit(1)
	}

	fmt.Println(writer2.String())
}
