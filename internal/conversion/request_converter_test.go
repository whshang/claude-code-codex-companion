package conversion

import (
	"encoding/json"
	"testing"

	"claude-code-codex-companion/internal/logger"
)

// 辅助函数
func intPtr(i int) *int {
	return &i
}

func getTestLogger() *logger.Logger {
	// Create a simple test logger
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level:           "debug",
		LogRequestTypes: "all",
		LogDirectory:    "", // Empty to avoid file operations in tests
	})
	return testLogger
}

func TestConvertAnthropicRequestToOpenAI_SimpleText(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())

	if converter == nil {
		t.Fatal("Converter should not be nil")
	}

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "Hello, how are you?"},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证基本字段
	if oaReq.Model != "claude-3-sonnet-20240229" {
		t.Errorf("Expected model 'claude-3-sonnet-20240229', got '%s'", oaReq.Model)
	}

	if oaReq.MaxCompletionTokens == nil {
		t.Fatal("MaxCompletionTokens should not be nil")
	}
	if *oaReq.MaxCompletionTokens != 1024 {
		t.Errorf("Expected max_completion_tokens 1024, got %d", *oaReq.MaxCompletionTokens)
	}

	// 验证消息转换
	if len(oaReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(oaReq.Messages))
	}

	msg := oaReq.Messages[0]
	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	if msg.Content != "Hello, how are you?" {
		t.Errorf("Expected content 'Hello, how are you?', got '%v'", msg.Content)
	}

	// 验证上下文
	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithTools(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "What files are in the current directory?"},
				},
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "list_files",
				Description: "List files in a directory",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		ToolChoice: &AnthropicToolChoice{Type: "auto"},
		MaxTokens:  intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证工具转换
	if len(oaReq.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(oaReq.Tools))
	}

	tool := oaReq.Tools[0]
	if tool.Type != "function" {
		t.Errorf("Expected tool type 'function', got '%s'", tool.Type)
	}

	if tool.Function.Name != "list_files" {
		t.Errorf("Expected function name 'list_files', got '%s'", tool.Function.Name)
	}

	// 验证tool_choice转换
	if oaReq.ToolChoice != "auto" {
		t.Errorf("Expected tool_choice 'auto', got '%v'", oaReq.ToolChoice)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithToolUse(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "List files in /tmp"},
				},
			},
			{
				Role: "assistant",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "I'll list the files for you."},
					{
						Type:  "tool_use",
						ID:    "call_123",
						Name:  "list_files",
						Input: json.RawMessage(`{"path": "/tmp"}`),
					},
				},
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "list_files",
				Description: "List files in a directory",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证消息数量（user + assistant）
	if len(oaReq.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(oaReq.Messages))
	}

	// 验证assistant消息的tool_calls
	assistantMsg := oaReq.Messages[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", assistantMsg.Role)
	}

	if assistantMsg.Content != "I'll list the files for you." {
		t.Errorf("Expected content 'I'll list the files for you.', got '%v'", assistantMsg.Content)
	}

	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}

	toolCall := assistantMsg.ToolCalls[0]
	if toolCall.ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", toolCall.ID)
	}

	if toolCall.Function.Name != "list_files" {
		t.Errorf("Expected function name 'list_files', got '%s'", toolCall.Function.Name)
	}

	expectedArgs := `{"path":"/tmp"}` // JSON.Marshal没有空格
	if toolCall.Function.Arguments != expectedArgs {
		t.Errorf("Expected arguments '%s', got '%s'", expectedArgs, toolCall.Function.Arguments)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithToolResult(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{
						Type:      "tool_result",
						ToolUseID: "call_123",
						Content: []AnthropicContentBlock{
							{Type: "text", Text: "file1.txt\nfile2.txt"},
						},
					},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证tool result转换为tool消息
	if len(oaReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(oaReq.Messages))
	}

	msg := oaReq.Messages[0]
	if msg.Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", msg.Role)
	}

	if msg.ToolCallID != "call_123" {
		t.Errorf("Expected tool_call_id 'call_123', got '%s'", msg.ToolCallID)
	}

	if msg.Content != "file1.txt\nfile2.txt" {
		t.Errorf("Expected content 'file1.txt\\nfile2.txt', got '%v'", msg.Content)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithImage(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "What's in this image?"},
					{
						Type: "image",
						Source: &AnthropicImageSource{
							Type:      "base64",
							MediaType: "image/png",
							Data:      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChAGH",
						},
					},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证消息转换
	if len(oaReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(oaReq.Messages))
	}

	msg := oaReq.Messages[0]
	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	// 应该是数组格式的content（因为有图片）
	contentInterface, ok := msg.Content.([]interface{})
	if !ok {
		t.Fatalf("Expected content to be array, got %T", msg.Content)
	}
	
	// 将interface{}数组转换为OpenAIMessageContent数组
	var contentArray []OpenAIMessageContent
	for _, item := range contentInterface {
		if itemMap, ok := item.(map[string]interface{}); ok {
			content := OpenAIMessageContent{
				Type: itemMap["type"].(string),
			}
			if content.Type == "text" {
				content.Text = itemMap["text"].(string)
			} else if content.Type == "image_url" {
				if imageURL, ok := itemMap["image_url"].(map[string]interface{}); ok {
					content.ImageURL = &OpenAIImageURL{
						URL: imageURL["url"].(string),
					}
				}
			}
			contentArray = append(contentArray, content)
		}
	}

	if len(contentArray) != 2 {
		t.Fatalf("Expected 2 content parts, got %d", len(contentArray))
	}

	// 验证图片部分
	var imageContent *OpenAIMessageContent
	var textContent *OpenAIMessageContent
	for i := range contentArray {
		if contentArray[i].Type == "image_url" {
			imageContent = &contentArray[i]
		} else if contentArray[i].Type == "text" {
			textContent = &contentArray[i]
		}
	}

	if imageContent == nil {
		t.Fatal("Image content not found")
	}

	if textContent == nil {
		t.Fatal("Text content not found")
	}

	if textContent.Text != "What's in this image?" {
		t.Errorf("Expected text 'What's in this image?', got '%s'", textContent.Text)
	}

	expectedURL := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChAGH"
	if imageContent.ImageURL.URL != expectedURL {
		t.Errorf("Expected image URL '%s', got '%s'", expectedURL, imageContent.ImageURL.URL)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestToolChoiceMapping(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	testCases := []struct {
		name           string
		anthToolChoice *AnthropicToolChoice
		expectedOA     interface{}
	}{
		{
			name:           "auto",
			anthToolChoice: &AnthropicToolChoice{Type: "auto"},
			expectedOA:     "auto",
		},
		{
			name:           "any -> required",
			anthToolChoice: &AnthropicToolChoice{Type: "any"},
			expectedOA:     "required",
		},
		{
			name:           "tool with name",
			anthToolChoice: &AnthropicToolChoice{Type: "tool", Name: "list_files"},
			expectedOA: map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": "list_files",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			anthReq := AnthropicRequest{
				Model: "claude-3-sonnet-20240229",
				Messages: []AnthropicMessage{
					{
						Role: "user",
						Content: []AnthropicContentBlock{
							{Type: "text", Text: "Test"},
						},
					},
				},
				Tools: []AnthropicTool{
					{
						Name:        "list_files",
						Description: "List files in a directory",
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"path": map[string]interface{}{
									"type":        "string",
									"description": "Directory path",
								},
							},
							"required": []string{"path"},
						},
					},
				},
				ToolChoice: tc.anthToolChoice,
				MaxTokens:  intPtr(1024),
			}

			reqBytes, _ := json.Marshal(anthReq)
			result, _, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

			if err != nil {
				t.Fatalf("Conversion failed: %v", err)
			}

			var oaReq OpenAIRequest
			if err := json.Unmarshal(result, &oaReq); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			// 比较结果
			expectedJson, _ := json.Marshal(tc.expectedOA)
			actualJson, _ := json.Marshal(oaReq.ToolChoice)

			if string(expectedJson) != string(actualJson) {
				t.Errorf("Expected tool_choice %s, got %s", string(expectedJson), string(actualJson))
			}
		})
	}
}

func TestSystemMessageHandling(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	testCases := []struct {
		name           string
		system         interface{}
		expectedExists bool
		expectedText   string
	}{
		{
			name:           "string system",
			system:         "You are a helpful assistant.",
			expectedExists: true,
			expectedText:   "You are a helpful assistant.",
		},
		{
			name: "blocks system",
			system: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "You are a helpful assistant.",
				},
			},
			expectedExists: true,
			expectedText:   "You are a helpful assistant.",
		},
		{
			name:           "nil system",
			system:         nil,
			expectedExists: false,
			expectedText:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			anthReq := AnthropicRequest{
				Model: "claude-3-sonnet-20240229",
				System: tc.system,
				Messages: []AnthropicMessage{
					{
						Role: "user",
						Content: []AnthropicContentBlock{
							{Type: "text", Text: "Hello"},
						},
					},
				},
				MaxTokens: intPtr(1024),
			}

			reqBytes, _ := json.Marshal(anthReq)
			result, _, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

			if err != nil {
				t.Fatalf("Conversion failed: %v", err)
			}

			var oaReq OpenAIRequest
			if err := json.Unmarshal(result, &oaReq); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			// 检查系统消息
			var systemMsg *OpenAIMessage
			for _, msg := range oaReq.Messages {
				if msg.Role == "system" {
					systemMsg = &msg
					break
				}
			}

			if tc.expectedExists {
				if systemMsg == nil {
					t.Fatal("Expected system message, but not found")
				}
				if systemMsg.Content != tc.expectedText {
					t.Errorf("Expected system content '%s', got '%v'", tc.expectedText, systemMsg.Content)
				}
			} else {
				if systemMsg != nil {
					t.Fatal("Expected no system message, but found one")
				}
			}
		})
	}
}

func TestToolChoiceOnlyWhenToolsPresent(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())

	// 测试用例1：没有工具时，不应该设置tool_choice
	t.Run("no_tools_no_tool_choice", func(t *testing.T) {
		anthReq := AnthropicRequest{
			Model: "claude-3-sonnet-20240229",
			Messages: []AnthropicMessage{
				{
					Role: "user",
					Content: []AnthropicContentBlock{
						{Type: "text", Text: "Hello, how are you?"},
					},
				},
			},
			MaxTokens: intPtr(1024),
		}

		reqBytes, _ := json.Marshal(anthReq)
		result, _, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

		if err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		var oaReq OpenAIRequest
		if err := json.Unmarshal(result, &oaReq); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		// 验证没有工具
		if len(oaReq.Tools) != 0 {
			t.Errorf("Expected 0 tools, got %d", len(oaReq.Tools))
		}

		// 验证没有设置tool_choice
		if oaReq.ToolChoice != nil {
			t.Errorf("Expected tool_choice to be nil when no tools present, got %v", oaReq.ToolChoice)
		}
	})

	// 测试用例2：有工具但没有指定tool_choice时，应该设置为"auto"
	t.Run("has_tools_default_auto", func(t *testing.T) {
		anthReq := AnthropicRequest{
			Model: "claude-3-sonnet-20240229",
			Messages: []AnthropicMessage{
				{
					Role: "user",
					Content: []AnthropicContentBlock{
						{Type: "text", Text: "List files"},
					},
				},
			},
			Tools: []AnthropicTool{
				{
					Name:        "list_files",
					Description: "List files in a directory",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "Directory path",
							},
						},
						"required": []string{"path"},
					},
				},
			},
			MaxTokens: intPtr(1024),
		}

		reqBytes, _ := json.Marshal(anthReq)
		result, _, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

		if err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		var oaReq OpenAIRequest
		if err := json.Unmarshal(result, &oaReq); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		// 验证有工具
		if len(oaReq.Tools) != 1 {
			t.Errorf("Expected 1 tool, got %d", len(oaReq.Tools))
		}

		// 验证设置了tool_choice为"auto"
		if oaReq.ToolChoice == nil {
			t.Fatal("Expected tool_choice to be set when tools are present, got nil")
		}
		if oaReq.ToolChoice != "auto" {
			t.Errorf("Expected tool_choice 'auto' when tools present but no tool_choice specified, got %v", oaReq.ToolChoice)
		}
	})

	// 测试用例3：有工具且指定了tool_choice时，应该正确设置
	t.Run("has_tools_explicit_choice", func(t *testing.T) {
		anthReq := AnthropicRequest{
			Model: "claude-3-sonnet-20240229",
			Messages: []AnthropicMessage{
				{
					Role: "user",
					Content: []AnthropicContentBlock{
						{Type: "text", Text: "List files"},
					},
				},
			},
			Tools: []AnthropicTool{
				{
					Name:        "list_files",
					Description: "List files in a directory",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "Directory path",
							},
						},
						"required": []string{"path"},
					},
				},
			},
			ToolChoice: &AnthropicToolChoice{Type: "any"},
			MaxTokens:  intPtr(1024),
		}

		reqBytes, _ := json.Marshal(anthReq)
		result, _, err := converter.Convert(reqBytes, &EndpointInfo{Type: "openai"})

		if err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		var oaReq OpenAIRequest
		if err := json.Unmarshal(result, &oaReq); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		// 验证有工具
		if len(oaReq.Tools) != 1 {
			t.Errorf("Expected 1 tool, got %d", len(oaReq.Tools))
		}

		// 验证设置了tool_choice为"required"（any映射为required）
		if oaReq.ToolChoice == nil {
			t.Fatal("Expected tool_choice to be set when tools are present, got nil")
		}
		if oaReq.ToolChoice != "required" {
			t.Errorf("Expected tool_choice 'required' when tool_choice is 'any', got %v", oaReq.ToolChoice)
		}
	})
}