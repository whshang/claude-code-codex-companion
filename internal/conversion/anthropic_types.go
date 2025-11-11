package conversion

import "encoding/json"

// Anthropic API 结构定义 - 基于参考实现

// AnthropicRequest Anthropic 请求结构
type AnthropicRequest struct {
	Model       string        `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	System      interface{}   `json:"system,omitempty"` // string | []AnthropicContentBlock
	Tools       []AnthropicTool    `json:"tools,omitempty"`  // name, description, input_schema(JSON Schema)
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	TopK        *int          `json:"top_k,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"` // Anthropic: 输出最大 token
	ToolChoice  *AnthropicToolChoice `json:"tool_choice,omitempty"`
	Metadata    map[string]interface{}  `json:"metadata,omitempty"`
	Stream      *bool         `json:"stream,omitempty"` // 是否要求流式
	StopSequences []string    `json:"stop_sequences,omitempty"`
	Thinking    *AnthropicThinking `json:"thinking,omitempty"` // 将被忽略，OpenAI不支持
	DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`
}

// AnthropicThinking 思考模式配置
type AnthropicThinking struct {
	Type         string `json:"type,omitempty"`          // "enabled" 表示启用思考模式
	BudgetTokens int    `json:"budget_tokens,omitempty"` // 思考模式的token预算
}

// AnthropicMessage 消息体
type AnthropicMessage struct {
	Role    string      `json:"role"` // "user" | "assistant"
	Content interface{} `json:"content"` // string | []AnthropicContentBlock
}

// AnthropicContentBlock 内容块（Claude Code 会混用 text / image / tool_use / tool_result）
type AnthropicContentBlock struct {
	Type string `json:"type"` // "text" | "image" | "tool_use" | "tool_result" | "text_delta" | "input_json_delta"

	// text
	Text string `json:"text,omitempty"`

	// image (仅支持 base64)
	// Anthropic: {type:"image", source:{type:"base64", media_type:"image/png", data:"..."}}
	Source *AnthropicImageSource `json:"source,omitempty"`

	// tool_use（由 assistant 发出）
	// Anthropic: {type:"tool_use", id:"...", name:"LS", input:{...}}
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result（由 user 发回）
	// Anthropic: {type:"tool_result", tool_use_id:"...", content:[{type:"text", text:"..."}, ...], is_error?:bool}
	// content 可能是字符串或者 []AnthropicContentBlock 数组
	ToolUseID string             `json:"tool_use_id,omitempty"`
	Content   interface{}        `json:"content,omitempty"`
	IsError   *bool              `json:"is_error,omitempty"`

	// 用于流式事件的增量字段
	PartialJSON string `json:"partial_json,omitempty"` // 用于 input_json_delta
}

// AnthropicImageSource 图片源
type AnthropicImageSource struct {
	Type      string `json:"type"` // "base64"
	MediaType string `json:"media_type"`
	Data      string `json:"data"` // base64 内容
}

// AnthropicTool 工具定义：input_schema 是 JSON Schema
type AnthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"` // JSON Schema
}

// AnthropicToolChoice 工具选择
type AnthropicToolChoice struct {
	Type string `json:"type"`           // "auto"|"any"|"tool"
	Name string `json:"name,omitempty"` // 当 Type=="tool" 时指定工具名
}

// AnthropicResponse Anthropic 响应（精简）
type AnthropicResponse struct {
	ID           string             `json:"id,omitempty"`
	Type         string             `json:"type,omitempty"` // "message"
	Role         string             `json:"role"`           // "assistant"
	Model        string             `json:"model,omitempty"`
	StopReason   string             `json:"stop_reason,omitempty"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        *AnthropicUsage    `json:"usage,omitempty"`
	Content      []AnthropicContentBlock `json:"content"`
}

// AnthropicUsage 使用统计
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicStreamEvent 流式事件
type AnthropicStreamEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// AnthropicMessageStart 消息开始事件
type AnthropicMessageStart struct {
	Type    string             `json:"type"`
	Message *AnthropicResponse `json:"message"`
}

// AnthropicContentBlockForStart 专门用于 content_block_start 事件的结构体
// 确保 text 字段始终被序列化，即使为空
type AnthropicContentBlockForStart struct {
	Type string `json:"type"` // "text" | "tool_use"
	Text string `json:"-"`    // 使用自定义序列化

	// tool_use 字段（当 Type 为 "tool_use" 时使用）
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// MarshalJSON 自定义 JSON 序列化
// 对于 "text" 类型，始终包含 text 字段；对于 "tool_use" 类型，省略 text 字段
func (c AnthropicContentBlockForStart) MarshalJSON() ([]byte, error) {
	type Alias AnthropicContentBlockForStart
	aux := &struct {
		Text *string `json:"text,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(&c),
	}
	
	// 只有当 Type 为 "text" 时才包含 text 字段
	if c.Type == "text" {
		aux.Text = &c.Text
	}
	
	return json.Marshal(aux)
}

// AnthropicContentBlockStart 内容块开始事件
type AnthropicContentBlockStart struct {
	Type         string                         `json:"type"`
	Index        int                            `json:"index"`
	ContentBlock *AnthropicContentBlockForStart `json:"content_block"`  // 使用专门的结构体
}

// AnthropicContentBlockDelta 内容块增量事件
type AnthropicContentBlockDelta struct {
	Type  string                 `json:"type"`
	Index int                    `json:"index"`
	Delta *AnthropicContentBlock `json:"delta"`
}

// AnthropicContentBlockStop 内容块结束事件
type AnthropicContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// AnthropicMessageDelta 消息增量事件
type AnthropicMessageDelta struct {
	Type  string                           `json:"type"`
	Delta *AnthropicMessageDeltaContent    `json:"delta"`
	Usage *AnthropicUsage                  `json:"usage,omitempty"` // Usage is sibling to delta, not inside it
}

// AnthropicMessageDeltaContent represents only the fields that should be in message_delta.delta
type AnthropicMessageDeltaContent struct {
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// AnthropicMessageStop 消息结束事件
type AnthropicMessageStop struct {
	Type string `json:"type"`
}

// GetContentBlocks 将 Content 字段转换为统一的 []AnthropicContentBlock 格式
func (m *AnthropicMessage) GetContentBlocks() []AnthropicContentBlock {
	switch v := m.Content.(type) {
	case string:
		// 如果是字符串，转换为单个 text 类型的 content block
		if v != "" {
			return []AnthropicContentBlock{
				{
					Type: "text",
					Text: v,
				},
			}
		}
		return []AnthropicContentBlock{}
	case []interface{}:
		// 如果是 interface{} 数组，尝试转换为 AnthropicContentBlock 数组
		var blocks []AnthropicContentBlock
		for _, item := range v {
			if blockMap, ok := item.(map[string]interface{}); ok {
				block := AnthropicContentBlock{}
				if typ, exists := blockMap["type"].(string); exists {
					block.Type = typ
				}
				if text, exists := blockMap["text"].(string); exists {
					block.Text = text
				}
				if id, exists := blockMap["id"].(string); exists {
					block.ID = id
				}
				if name, exists := blockMap["name"].(string); exists {
					block.Name = name
				}
				if input, exists := blockMap["input"]; exists {
					if inputBytes, err := json.Marshal(input); err == nil {
						block.Input = json.RawMessage(inputBytes)
					}
				}
				if toolUseID, exists := blockMap["tool_use_id"].(string); exists {
					block.ToolUseID = toolUseID
				}
				if content, exists := blockMap["content"]; exists {
					// content 可能是字符串或者 []interface{} 数组
					if contentStr, ok := content.(string); ok {
						// content 是字符串，直接设置
						block.Content = contentStr
					} else if contentArray, ok := content.([]interface{}); ok {
						// content 是数组，递归处理嵌套的 content
						var contentBlocks []AnthropicContentBlock
						for _, contentItem := range contentArray {
							if contentMap, ok := contentItem.(map[string]interface{}); ok {
								contentBlock := AnthropicContentBlock{}
								if typ, exists := contentMap["type"].(string); exists {
									contentBlock.Type = typ
								}
								if text, exists := contentMap["text"].(string); exists {
									contentBlock.Text = text
								}
								contentBlocks = append(contentBlocks, contentBlock)
							}
						}
						block.Content = contentBlocks
					}
				}
				if isError, exists := blockMap["is_error"].(bool); exists {
					block.IsError = &isError
				}
				if source, exists := blockMap["source"].(map[string]interface{}); exists {
					block.Source = &AnthropicImageSource{}
					if typ, ok := source["type"].(string); ok {
						block.Source.Type = typ
					}
					if mediaType, ok := source["media_type"].(string); ok {
						block.Source.MediaType = mediaType
					}
					if data, ok := source["data"].(string); ok {
						block.Source.Data = data
					}
				}
				blocks = append(blocks, block)
			}
		}
		return blocks
	case []AnthropicContentBlock:
		// 如果已经是正确类型，直接返回
		return v
	default:
		// 其他情况，返回空数组
		return []AnthropicContentBlock{}
	}
}