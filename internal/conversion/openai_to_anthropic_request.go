package conversion

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ConvertOpenAIRequestJSONToAnthropic 将 OpenAI Chat Completions 请求 JSON 转换为 Anthropic Messages 请求 JSON。
// 该函数专门用于在仅配置 Anthropic URL 时服务 OpenAI / Codex 客户端。
func ConvertOpenAIRequestJSONToAnthropic(body []byte) ([]byte, error) {
	var openaiReq OpenAIRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI request: %w", err)
	}

	anthropicReq, err := buildAnthropicRequestFromOpenAI(&openaiReq)
	if err != nil {
		return nil, err
	}

	converted, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Anthropic request: %w", err)
	}

	return converted, nil
}

func buildAnthropicRequestFromOpenAI(openaiReq *OpenAIRequest) (*AnthropicRequest, error) {
	anthropicReq := &AnthropicRequest{
		Model: openaiReq.Model,
	}

	// Stream flag
	if openaiReq.Stream != nil {
		anthropicReq.Stream = openaiReq.Stream
	}

	// Temperature / top_p
	anthropicReq.Temperature = openaiReq.Temperature
	anthropicReq.TopP = openaiReq.TopP

	// Max tokens（OpenAI 可能提供多个字段）
	if openaiReq.MaxTokens != nil {
		anthropicReq.MaxTokens = openaiReq.MaxTokens
	} else if openaiReq.MaxOutputTokens != nil {
		anthropicReq.MaxTokens = openaiReq.MaxOutputTokens
	} else if openaiReq.MaxCompletionTokens != nil {
		anthropicReq.MaxTokens = openaiReq.MaxCompletionTokens
	}

	// Stop sequences
	if len(openaiReq.Stop) > 0 {
		anthropicReq.StopSequences = openaiReq.Stop
	}

	// Parallel tool calls
	if openaiReq.ParallelToolCalls != nil {
		disable := !*openaiReq.ParallelToolCalls
		anthropicReq.DisableParallelToolUse = &disable
	}

	// Metadata.user_id
	if openaiReq.User != "" {
		anthropicReq.Metadata = map[string]interface{}{
			"user_id": openaiReq.User,
		}
	}

	// Tools
	if len(openaiReq.Tools) > 0 {
		for _, tool := range openaiReq.Tools {
			if strings.ToLower(tool.Type) != "function" {
				continue
			}
			anthropicReq.Tools = append(anthropicReq.Tools, AnthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			})
		}
	}

	// Tool choice
	if openaiReq.ToolChoice != nil {
		choice, err := convertOpenAIToolChoice(openaiReq.ToolChoice)
		if err != nil {
			return nil, err
		}
		anthropicReq.ToolChoice = choice
	}

	// System messages需要拆分
	var systemSegments []string

	for _, msg := range openaiReq.Messages {
		role := strings.ToLower(msg.Role)
		switch role {
		case "system", "developer":
			text, err := extractOpenAIMessagePlainText(msg.Content)
			if err != nil {
				return nil, err
			}
			if text != "" {
				systemSegments = append(systemSegments, text)
			}
			continue
		default:
		}

		convertedMsg, err := convertOpenAIMessageToAnthropic(msg)
		if err != nil {
			return nil, err
		}
		if convertedMsg != nil {
			anthropicReq.Messages = append(anthropicReq.Messages, *convertedMsg)
		}
	}

	if len(systemSegments) == 1 {
		anthropicReq.System = systemSegments[0]
	} else if len(systemSegments) > 1 {
		anthropicReq.System = strings.Join(systemSegments, "\n\n")
	}

	return anthropicReq, nil
}

func convertOpenAIToolChoice(choice interface{}) (*AnthropicToolChoice, error) {
	switch v := choice.(type) {
	case string:
		normalized := strings.ToLower(v)
		switch normalized {
		case "auto":
			return &AnthropicToolChoice{Type: "auto"}, nil
		case "none":
			// Anthropic 没有 none 的语义，返回 nil 表示不指定
			return nil, nil
		case "required":
			// 近似使用 any（要求必须使用工具）
			return &AnthropicToolChoice{Type: "any"}, nil
		default:
			return nil, fmt.Errorf("unsupported tool_choice value: %s", v)
		}
	case map[string]interface{}:
		// 期望 {"type":"function","function":{"name":"..."}}
		if toolType, ok := v["type"].(string); ok && strings.ToLower(toolType) == "function" {
			if function, ok := v["function"].(map[string]interface{}); ok {
				if name, ok := function["name"].(string); ok {
					return &AnthropicToolChoice{Type: "tool", Name: name}, nil
				}
			}
		}
		return nil, fmt.Errorf("unsupported structured tool_choice: %v", v)
	default:
		return nil, fmt.Errorf("unsupported tool_choice type: %T", choice)
	}
}

func convertOpenAIMessageToAnthropic(msg OpenAIMessage) (*AnthropicMessage, error) {
	role := strings.ToLower(msg.Role)

	switch role {
	case "user":
		blocks, err := convertOpenAIContentToAnthropicBlocks(msg.Content)
		if err != nil {
			return nil, err
		}
		return &AnthropicMessage{Role: "user", Content: blocks}, nil

	case "assistant":
		blocks, err := convertOpenAIContentToAnthropicBlocks(msg.Content)
		if err != nil {
			return nil, err
		}

		// 工具调用
		if len(msg.ToolCalls) > 0 {
			for _, call := range msg.ToolCalls {
				input := json.RawMessage(call.Function.Arguments)
				blocks = append(blocks, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    call.ID,
					Name:  call.Function.Name,
					Input: input,
				})
			}
		}

		return &AnthropicMessage{Role: "assistant", Content: blocks}, nil

	case "tool":
		// OpenAI tool role -> Anthropic user 消息中的 tool_result 内容块
		contentStr, err := extractOpenAIMessagePlainText(msg.Content)
		if err != nil {
			return nil, err
		}
		block := AnthropicContentBlock{
			Type:      "tool_result",
			ToolUseID: msg.ToolCallID,
			Content:   contentStr,
		}
		return &AnthropicMessage{Role: "user", Content: []AnthropicContentBlock{block}}, nil

	default:
		// 其他角色暂不支持
		return nil, nil
	}
}

func convertOpenAIContentToAnthropicBlocks(content interface{}) ([]AnthropicContentBlock, error) {
	if content == nil {
		return []AnthropicContentBlock{}, nil
	}

	switch v := content.(type) {
	case string:
		if v == "" {
			return []AnthropicContentBlock{}, nil
		}
		return []AnthropicContentBlock{{Type: "text", Text: v}}, nil

	case []interface{}:
		var blocks []AnthropicContentBlock
		for _, item := range v {
			block, err := convertOpenAIMessagePart(item)
			if err != nil {
				return nil, err
			}
			if block != nil {
				blocks = append(blocks, *block)
			}
		}
		return blocks, nil

	default:
		return nil, fmt.Errorf("unsupported OpenAI message content type: %T", content)
	}
}

func convertOpenAIMessagePart(item interface{}) (*AnthropicContentBlock, error) {
	switch part := item.(type) {
	case map[string]interface{}:
		partType, _ := part["type"].(string)
		switch strings.ToLower(partType) {
		case "text":
			text, _ := part["text"].(string)
			return &AnthropicContentBlock{Type: "text", Text: text}, nil

		case "image_url":
			img := &AnthropicContentBlock{Type: "image"}
			if urlInfo, ok := part["image_url"].(map[string]interface{}); ok {
				if urlStr, ok := urlInfo["url"].(string); ok {
					src, err := convertOpenAIImageURL(urlStr)
					if err != nil {
						return nil, err
					}
					img.Source = src
					return img, nil
				}
			}
			return nil, errors.New("invalid image_url structure")
		}

	case map[string]string:
		// JSON 解码为 map[string]string 的情况
		if partType, ok := part["type"]; ok && partType == "text" {
			return &AnthropicContentBlock{Type: "text", Text: part["text"]}, nil
		}
	}

	return nil, fmt.Errorf("unsupported message content part: %v", item)
}

func convertOpenAIImageURL(urlStr string) (*AnthropicImageSource, error) {
	if strings.HasPrefix(urlStr, "data:") {
		// data:image/png;base64,...
		commaIdx := strings.Index(urlStr, ",")
		if commaIdx == -1 {
			return nil, fmt.Errorf("invalid data URL: %s", urlStr)
		}
		metadata := urlStr[len("data:"):commaIdx]
		data := urlStr[commaIdx+1:]
		if !strings.HasSuffix(metadata, ";base64") {
			return nil, fmt.Errorf("only base64 data URLs are supported: %s", metadata)
		}
		mediaType := strings.TrimSuffix(metadata, ";base64")

		if _, err := base64.StdEncoding.DecodeString(data); err != nil {
			return nil, fmt.Errorf("invalid base64 data: %w", err)
		}

		return &AnthropicImageSource{
			Type:      "base64",
			MediaType: mediaType,
			Data:      data,
		}, nil
	}

	return nil, fmt.Errorf("remote image URLs are not supported by Anthropic format: %s", urlStr)
}

func extractOpenAIMessagePlainText(content interface{}) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []interface{}:
		var builder strings.Builder
		for _, item := range v {
			switch part := item.(type) {
			case map[string]interface{}:
				if strings.ToLower(fmt.Sprint(part["type"])) == "text" {
					if text, ok := part["text"].(string); ok {
						builder.WriteString(text)
					}
				}
			}
		}
		return builder.String(), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported message content type for plain text extraction: %T", content)
	}
}
