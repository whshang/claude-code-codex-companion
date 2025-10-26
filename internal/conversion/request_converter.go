package conversion

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// RequestConverter 请求转换器 - 基于参考实现
type RequestConverter struct {
	logger      *logger.Logger
	modelMapper *ModelMapper
}

// NewRequestConverter 创建请求转换器
func NewRequestConverter(logger *logger.Logger) *RequestConverter {
	return &RequestConverter{
		logger:      logger,
		modelMapper: NewModelMapper(logger),
	}
}

// Convert 转换 Anthropic 请求为 OpenAI 格式 - 基于参考实现
func (c *RequestConverter) Convert(anthropicReq []byte, endpointInfo *EndpointInfo) ([]byte, *ConversionContext, error) {
	// 解析 Anthropic 请求
	var anthReq AnthropicRequest
	if err := json.Unmarshal(anthropicReq, &anthReq); err != nil {
		return nil, nil, NewConversionError("parse_error", "Failed to parse Anthropic request", err)
	}

	mapper := newDefaultThinkingMapper(c.logger)

	// 创建转换上下文
	ctx := &ConversionContext{
		SourceFormat: "anthropic",
		TargetFormat: "openai",
		Headers:      make(map[string]string),
		Metadata: map[string]interface{}{
			"tool_call_id_map": make(map[string]string),
			"is_streaming":     anthReq.Stream != nil && *anthReq.Stream,
			"stop_sequences":   anthReq.StopSequences,
		},
	}

	// 构建 OpenAI 请求
	out := OpenAIRequest{
		Model: anthReq.Model,
	}
	if c.modelMapper != nil {
		mapped := c.modelMapper.MapModel(out.Model, GetMappingForConversion("anthropic", "openai"))
		if mapped != "" {
			out.Model = mapped
			ctx.Metadata["mapped_model"] = mapped
		}
	}

	// 温控映射
	out.Temperature = anthReq.Temperature
	out.TopP = anthReq.TopP

	// 根据端点配置处理 max_tokens 字段名转换
	if endpointInfo != nil && endpointInfo.MaxTokensFieldName != "" {
		// 根据配置的字段名设置对应字段
		switch endpointInfo.MaxTokensFieldName {
		case "max_completion_tokens":
			out.MaxCompletionTokens = anthReq.MaxTokens
		case "max_output_tokens":
			out.MaxOutputTokens = anthReq.MaxTokens
		default:
			// 默认或未知值，保持原始字段名
			out.MaxTokens = anthReq.MaxTokens
		}
	} else {
		// 没有配置时根据端点类型选择最常用的字段名
		if endpointInfo != nil && strings.ToLower(endpointInfo.Type) == "openai" {
			out.MaxCompletionTokens = anthReq.MaxTokens
		} else {
			out.MaxTokens = anthReq.MaxTokens
		}
	}
	out.Stream = anthReq.Stream
	out.Stop = anthReq.StopSequences

	// 处理用户ID
	if anthReq.Metadata != nil {
		if userID, ok := anthReq.Metadata["user_id"].(string); ok && userID != "" {
			out.User = userID
		}
	}

	// 工具映射
	for _, t := range anthReq.Tools {
		out.Tools = append(out.Tools, OpenAITool{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema, // JSON Schema 原样给到 parameters
			},
		})
	}

	// tool_choice 映射 - 只有在有工具时才设置
	if len(anthReq.Tools) > 0 {
		if anthReq.ToolChoice != nil {
			switch anthReq.ToolChoice.Type {
			case "auto":
				out.ToolChoice = "auto"
			case "any":
				// OpenAI 没有"any"语义；你可以：
				// 方案 A：用 "required" 强制必须走工具（更贴近"有就用"）
				// 方案 B：用 "auto"（由模型自己判断）
				// 这里选择更"强"的 A，避免模型直接文本结束：
				out.ToolChoice = "required"
			case "tool":
				out.ToolChoice = map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name": anthReq.ToolChoice.Name,
					},
				}
			default:
				out.ToolChoice = "auto"
			}
		} else {
			// 当有工具但没有指定 tool_choice 时，默认为 "auto"
			out.ToolChoice = "auto"
		}
	}
	// 如果没有工具，不设置 tool_choice

	// System 映射（可选）
	if s := c.anthropicSystemToText(anthReq.System); s != "" {
		out.Messages = append(out.Messages, OpenAIMessage{
			Role:    "system",
			Content: s,
		})
	}

	// 为了将 tool_result 正确串联到对应的 tool_call_id，
	// 需要一个（工具名 -> 最近一次生成的 call id）的映射（当上游历史中包含过 tool_use）
	latestCallIDByName := map[string]string{}

	// 验证Anthropic请求必须有messages字段
	if len(anthReq.Messages) == 0 {
		return nil, nil, NewConversionError("invalid_request", "Anthropic request must contain at least one message", fmt.Errorf("missing messages field"))
	}

	// 遍历对话消息，逐条转换
	for _, m := range anthReq.Messages {
		switch m.Role {
		case "user":
			// 用户消息可以包含 text / image / tool_result
			// 其中 tool_result 需转成 role:"tool"
			// 其他（text/image）转为 role:"user"
			//
			// 重要：为了确保相同 ID 的 assistant 和 tool 消息紧挨着，
			// 我们需要先输出所有 tool_result，然后再输出 user 消息
			// 使用新的 GetContentBlocks 方法获取内容块
			contentBlocks := m.GetContentBlocks()
			var userBlocks []AnthropicContentBlock
			var toolResults []AnthropicContentBlock
			for _, bl := range contentBlocks {
				if bl.Type == "tool_result" {
					toolResults = append(toolResults, bl)
				} else {
					userBlocks = append(userBlocks, bl)
				}
			}

			// 先处理 tool_result -> role:"tool"
			// 这样确保 assistant 和 tool 消息紧挨着
			for _, tr := range toolResults {
				if tr.ToolUseID == "" {
					// 尝试通过工具名匹配最近一次生成的 call id
					if tr.Name != "" {
						if fallbackID, exists := latestCallIDByName[tr.Name]; exists && fallbackID != "" {
							tr.ToolUseID = fallbackID
						}
					}
					if tr.ToolUseID == "" {
						return nil, nil, errors.New("user.tool_result is missing tool_use_id")
					}
				}

				// 提取 tool_result 的内容
				var content string
				switch v := tr.Content.(type) {
				case string:
					// content 是字符串，直接使用
					content = v
				case []AnthropicContentBlock:
					// content 是 AnthropicContentBlock 数组，提取文本
					var sb strings.Builder
					for _, c := range v {
						if c.Type == "text" {
							sb.WriteString(c.Text)
						}
					}
					content = sb.String()
				case []interface{}:
					// content 是 interface{} 数组，尝试提取文本
					var sb strings.Builder
					for _, item := range v {
						if blockMap, ok := item.(map[string]interface{}); ok {
							if typ, exists := blockMap["type"].(string); exists && typ == "text" {
								if text, exists := blockMap["text"].(string); exists {
									sb.WriteString(text)
								}
							}
						}
					}
					content = sb.String()
				default:
					content = ""
				}

				out.Messages = append(out.Messages, OpenAIMessage{
					Role:       "tool",
					ToolCallID: tr.ToolUseID,
					Content:    strings.TrimSpace(content),
				})
			}

			// 然后处理 user 内容（text/image）
			if len(userBlocks) > 0 {
				om := OpenAIMessage{Role: "user"}
				var oaParts []OpenAIMessageContent
				var sb strings.Builder // 拼接纯文本（当没有图片时可直接用字符串）
				hasImage := false
				for _, bl := range userBlocks {
					switch bl.Type {
					case "text":
						sb.WriteString(bl.Text)
					case "image":
						if bl.Source != nil && strings.EqualFold(bl.Source.Type, "base64") {
							// 有图片必须走数组 content
							hasImage = true
							oaParts = append(oaParts, OpenAIMessageContent{
								Type: "image_url",
								ImageURL: &OpenAIImageURL{
									URL: c.makeDataURL(bl.Source.MediaType, bl.Source.Data),
								},
							})
						}
					}
				}
				if hasImage {
					// 将已有文本（若有）也塞进 parts
					txt := strings.TrimSpace(sb.String())
					if txt != "" {
						oaParts = append(oaParts, OpenAIMessageContent{
							Type: "text",
							Text: txt,
						})
					}
					om.Content = oaParts
				} else {
					om.Content = strings.TrimSpace(sb.String())
				}
				if om.Content != "" {
					out.Messages = append(out.Messages, om)
				}
			}

		case "assistant":
			// assistant 可以包含：text + tool_use（一个或多个）
			// 使用新的 GetContentBlocks 方法获取内容块
			contentBlocks := m.GetContentBlocks()
			var textParts []string
			var toolUses []AnthropicContentBlock
			for _, bl := range contentBlocks {
				switch bl.Type {
				case "text":
					if bl.Text != "" {
						textParts = append(textParts, bl.Text)
					}
				case "tool_use":
					toolUses = append(toolUses, bl)
				}
			}
			om := OpenAIMessage{
				Role: "assistant",
			}
			// 文本合并
			if len(textParts) > 0 {
				om.Content = strings.Join(textParts, "\n")
			} else {
				om.Content = "" // OpenAI 允许空字符串
			}
			// tool_use -> tool_calls
			for _, tu := range toolUses {
				// OpenAI 需要 arguments 是"字符串化"的 JSON
				args := string(tu.Input)
				if !json.Valid([]byte(args)) {
					// 如果上游不是 JSON，尝试多种修复策略
					if args == "" {
						args = "{}" // 空参数
					} else {
						// 尝试修复JSON格式
						if fixedArgs, err := fixInvalidJSON(args); err == nil {
							args = fixedArgs
						} else {
							// 最后兜底：包一层字符串
							b, _ := json.Marshal(string(tu.Input))
							args = string(b)
						}
					}
				}
				call := OpenAIToolCall{
					ID:   tu.ID, // 用原始 id，便于和后续 tool_result 对齐
					Type: "function",
					Function: OpenAIToolCallDetail{
						Name:      tu.Name,
						Arguments: args,
					},
				}
				om.ToolCalls = append(om.ToolCalls, call)
				// 记录最近 callId，供可能的降级匹配（此处只做示例）
				latestCallIDByName[tu.Name] = tu.ID
			}
			out.Messages = append(out.Messages, om)

		default:
			// 其它角色（理论上 Anthropic 就 user/assistant）
			// 忽略或报错，这里选择忽略
		}
	}

	// 处理并行工具调用设置
	if anthReq.DisableParallelToolUse != nil && *anthReq.DisableParallelToolUse {
		out.ParallelToolCalls = boolPtr(false)
	}

	if thinking := InternalThinkingFromAnthropic(anthReq.Thinking); thinking != nil {
		applyThinkingForOpenAIRequest(thinking, &out, mapper)
		if c.logger != nil {
			c.logger.Debug("Converted thinking mode to OpenAI reasoning mode", map[string]interface{}{
				"provider":      thinking.Provider,
				"budget_tokens": thinking.BudgetTokens,
				"reasoning":     out.ReasoningEffort,
			})
		}
	}

	// 记录忽略的字段
	if c.logger != nil {
		if anthReq.TopK != nil {
			c.logger.Debug("Ignoring top_k field (not supported by OpenAI)")
		}
	}

	// 序列化结果
	result, err := json.Marshal(out)
	if err != nil {
		return nil, nil, NewConversionError("marshal_error", "Failed to marshal OpenAI request", err)
	}

	if c.logger != nil {
		c.logger.Debug("Request conversion completed")
	}

	return result, ctx, nil
}

// boolPtr 返回bool指针
func boolPtr(b bool) *bool {
	return &b
}

// stringPtr 返回string指针
func stringPtr(s string) *string {
	return &s
}

// fixInvalidJSON 尝试修复无效的JSON字符串
func fixInvalidJSON(input string) (string, error) {
	// 尝试添加引号修复
	if fixed := fixJSONQuotes(input); json.Valid([]byte(fixed)) {
		return fixed, nil
	}

	// 尝试修复缺少的大括号
	if fixed := fixJSONBraces(input); json.Valid([]byte(fixed)) {
		return fixed, nil
	}

	// 尝试移除JSON注释
	if fixed := removeJSONComments(input); json.Valid([]byte(fixed)) {
		return fixed, nil
	}

	// 尝试修复转义字符
	if fixed := fixJSONEscaping(input); json.Valid([]byte(fixed)) {
		return fixed, nil
	}

	return "", fmt.Errorf("unable to fix JSON: %s", input)
}

// fixJSONQuotes 修复JSON字符串的引号问题
func fixJSONQuotes(input string) string {
	// 简单的引号修复逻辑
	// 这里可以添加更复杂的修复逻辑
	return input
}

// fixJSONBraces 修复JSON的括号问题
func fixJSONBraces(input string) string {
	// 确保有开始和结束的大括号
	if !strings.HasPrefix(strings.TrimSpace(input), "{") {
		input = "{" + input
	}
	if !strings.HasSuffix(strings.TrimSpace(input), "}") {
		input = input + "}"
	}
	return input
}

// removeJSONComments 移除JSON中的注释
func removeJSONComments(input string) string {
	// 移除单行注释
	result := strings.ReplaceAll(input, "//", "")
	// 移除多行注释
	result = strings.ReplaceAll(result, "/*", "")
	result = strings.ReplaceAll(result, "*/", "")
	return result
}

// fixJSONEscaping 修复JSON转义字符
func fixJSONEscaping(input string) string {
	// 修复常见的转义问题
	result := strings.ReplaceAll(input, "\\\\", "\\")
	result = strings.ReplaceAll(result, "\\\"", "\"")
	return result
}

// anthropicSystemToText 将可能为 string 或 []AnthropicContentBlock 的 system 收敛为纯文本（保守策略）
func (c *RequestConverter) anthropicSystemToText(sys interface{}) string {
	switch v := sys.(type) {
	case nil:
		return ""
	case string:
		return v
	case []interface{}:
		var sb strings.Builder
		for _, it := range v {
			// 只抽取 text，忽略非 text（比如 image）
			if m, ok := it.(map[string]interface{}); ok {
				if t, _ := m["type"].(string); t == "text" {
					if tx, _ := m["text"].(string); tx != "" {
						sb.WriteString(tx)
						sb.WriteString("\n")
					}
				}
			}
		}
		return strings.TrimSpace(sb.String())
	default:
		// 尝试按 AnthropicContentBlock 反序列
		b, _ := json.Marshal(v)
		var blocks []AnthropicContentBlock
		if err := json.Unmarshal(b, &blocks); err == nil {
			var sb strings.Builder
			for _, bl := range blocks {
				if bl.Type == "text" && bl.Text != "" {
					sb.WriteString(bl.Text)
					sb.WriteString("\n")
				}
			}
			return strings.TrimSpace(sb.String())
		}
		return ""
	}
}

// makeDataURL 将base64数据转换为data URL格式
func (c *RequestConverter) makeDataURL(mediaType, data string) string {
	return "data:" + mediaType + ";base64," + data
}
