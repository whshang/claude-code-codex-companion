package conversion

import (
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// InternalContent 表示统一模型中的内容块
type InternalContent struct {
	Type      string `json:"type"`                 // "text" | "image"
	Text      string `json:"text,omitempty"`       // 文本内容
	ImageURL  string `json:"image_url,omitempty"`  // 图片URL（data: 或 http）
	MediaType string `json:"media_type,omitempty"` // 图片媒体类型
}

// InternalToolCall 表示统一模型中的工具调用
type InternalToolCall struct {
	Index     int    `json:"index"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // 累积后的完整 JSON 文本
}

// InternalUsage 表示统一模型中的 token 统计
type InternalUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// InternalMessage 表示统一模型中的完整消息
type InternalMessage struct {
	ID           string             `json:"id,omitempty"`
	Model        string             `json:"model,omitempty"`
	Role         string             `json:"role,omitempty"`
	ToolCallID   string             `json:"tool_call_id,omitempty"`
	Contents     []InternalContent  `json:"contents,omitempty"`
	ToolCalls    []InternalToolCall `json:"tool_calls,omitempty"`
	FinishReason string             `json:"finish_reason,omitempty"`
	Usage        *InternalUsage     `json:"usage,omitempty"`
	StopSequence string             `json:"stop_sequence,omitempty"`
}

// InternalTool 定义统一的工具结构
type InternalTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// InternalToolChoice 表示统一的工具选择策略
type InternalToolChoice struct {
	Type string `json:"type"` // "auto"|"none"|"required"|"tool"
	Name string `json:"name,omitempty"`
}

// InternalRequest 表示统一的请求结构
type InternalRequest struct {
	Model               string                 `json:"model"`
	Messages            []InternalMessage      `json:"messages"`
	Tools               []InternalTool         `json:"tools,omitempty"`
	ToolChoice          *InternalToolChoice    `json:"tool_choice,omitempty"`
	Temperature         *float64               `json:"temperature,omitempty"`
	TopP                *float64               `json:"top_p,omitempty"`
	MaxOutputTokens     *int                   `json:"max_output_tokens,omitempty"`
	MaxCompletionTokens *int                   `json:"max_completion_tokens,omitempty"`
	MaxTokens           *int                   `json:"max_tokens,omitempty"`
	Stream              bool                   `json:"stream,omitempty"`
	Stop                []string               `json:"stop,omitempty"`
	User                string                 `json:"user,omitempty"`
	ParallelToolCalls   *bool                  `json:"parallel_tool_calls,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	// 🆕 采样控制参数 (参考 chat2response)
	PresencePenalty     *float64               `json:"presence_penalty,omitempty"`  // 存在惩罚 (-2.0 to 2.0)
	FrequencyPenalty    *float64               `json:"frequency_penalty,omitempty"` // 频率惩罚 (-2.0 to 2.0)
	LogitBias           map[string]float64     `json:"logit_bias,omitempty"`        // token ID 到偏置值的映射
	N                   *int                   `json:"n,omitempty"`                 // 生成多个候选响应
	// 🆕 输出格式控制
	ResponseFormat      *InternalResponseFormat `json:"response_format,omitempty"`  // 输出格式约束
	// 🆕 推理相关字段 (o1 模型)
	ReasoningEffort     *string                `json:"reasoning_effort,omitempty"`     // "low"|"medium"|"high" 推理强度
	MaxReasoningTokens  *int                   `json:"max_reasoning_tokens,omitempty"` // 推理阶段的最大 token 数
}

// InternalResponseFormat 定义统一的输出格式约束
type InternalResponseFormat struct {
	Type   string                 `json:"type"`             // "text"|"json_object"|"json_schema"
	Schema map[string]interface{} `json:"schema,omitempty"` // JSON Schema (可选)
}

// InternalResponse 表示统一的响应结构
type InternalResponse struct {
	ID         string                 `json:"id,omitempty"`
	Model      string                 `json:"model,omitempty"`
	Message    *InternalMessage       `json:"message,omitempty"`
	Usage      *InternalUsage         `json:"usage,omitempty"`
	StopReason string                 `json:"stop_reason,omitempty"`
	FinishType string                 `json:"finish_type,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

// InternalEventType 用于标识统一事件类型
type InternalEventType string

const (
	InternalEventMessageStart InternalEventType = "message_start"
	InternalEventRoleDelta    InternalEventType = "role_delta"
	InternalEventTextDelta    InternalEventType = "text_delta"
	InternalEventImage        InternalEventType = "image"
	InternalEventToolStart    InternalEventType = "tool_start"
	InternalEventToolDelta    InternalEventType = "tool_delta"
	InternalEventToolStop     InternalEventType = "tool_stop"
	InternalEventUsage        InternalEventType = "usage"
	InternalEventFinish       InternalEventType = "finish"
	InternalEventMessageStop  InternalEventType = "message_stop"
)

// InternalEvent 表示统一流事件
type InternalEvent struct {
	Type           InternalEventType `json:"type"`
	MessageID      string            `json:"message_id,omitempty"`
	Model          string            `json:"model,omitempty"`
	Role           string            `json:"role,omitempty"`
	Index          int               `json:"index,omitempty"`
	TextDelta      string            `json:"text_delta,omitempty"`
	ImageURL       string            `json:"image_url,omitempty"`
	MediaType      string            `json:"media_type,omitempty"`
	ToolCallID     string            `json:"tool_call_id,omitempty"`
	ToolName       string            `json:"tool_name,omitempty"`
	ArgumentsDelta string            `json:"arguments_delta,omitempty"`
	FinishReason   string            `json:"finish_reason,omitempty"`
	StopSequence   string            `json:"stop_sequence,omitempty"`
	Usage          *InternalUsage    `json:"usage,omitempty"`
}

// ConversionMetadata 包含转换过程中的元信息
type ConversionMetadata struct {
	OriginalEventCount int    `json:"original_event_count"`
	ProcessingNotes    string `json:"processing_notes,omitempty"`
}

// ConversionResult 表示一次流式转换的结果
type ConversionResult struct {
	Events   []InternalEvent    `json:"events"`
	Metadata ConversionMetadata `json:"metadata"`
}

// AnthropicSSEEvent 用于兼容现有的 Anthropic SSE 构建逻辑
type AnthropicSSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// MessageAggregator 将统一事件流聚合为语义完整的消息状态
type MessageAggregator struct {
	logger           *logger.Logger
	textBuilder      strings.Builder
	toolCallBuilders map[int]*InternalToolCall
	message          *InternalMessage
	pythonFixer      *PythonJSONFixer
}

// NewMessageAggregator 创建一个事件聚合器
func NewMessageAggregator(logger *logger.Logger) *MessageAggregator {
	return &MessageAggregator{
		logger:           logger,
		toolCallBuilders: make(map[int]*InternalToolCall),
		message: &InternalMessage{
			Contents:  []InternalContent{},
			ToolCalls: []InternalToolCall{},
		},
		pythonFixer: NewPythonJSONFixer(logger),
	}
}

// Reset 清空聚合器状态
func (a *MessageAggregator) Reset() {
	a.textBuilder.Reset()
	a.toolCallBuilders = make(map[int]*InternalToolCall)
	a.message = &InternalMessage{
		Contents:  []InternalContent{},
		ToolCalls: []InternalToolCall{},
	}
}

// ApplyEvent 应用统一事件到聚合状态
func (a *MessageAggregator) ApplyEvent(event InternalEvent) {
	if a.message == nil {
		a.Reset()
	}

	switch event.Type {
	case InternalEventMessageStart:
		if event.MessageID != "" {
			a.message.ID = event.MessageID
		}
		if event.Model != "" {
			a.message.Model = event.Model
		}
		if event.Role != "" {
			a.message.Role = event.Role
		}
		if event.Usage != nil {
			a.ensureUsage().InputTokens = event.Usage.InputTokens
			a.ensureUsage().TotalTokens = event.Usage.TotalTokens
		}
	case InternalEventRoleDelta:
		if event.Role != "" {
			a.message.Role = event.Role
		}
	case InternalEventTextDelta:
		if event.TextDelta != "" {
			a.textBuilder.WriteString(event.TextDelta)
		}
	case InternalEventImage:
		a.flushText()
		a.message.Contents = append(a.message.Contents, InternalContent{
			Type:      "image",
			ImageURL:  event.ImageURL,
			MediaType: event.MediaType,
		})
	case InternalEventToolStart:
		a.flushText()
		tc := &InternalToolCall{
			Index: event.Index,
			ID:    event.ToolCallID,
			Name:  event.ToolName,
		}
		a.toolCallBuilders[event.Index] = tc
	case InternalEventToolDelta:
		builder, ok := a.toolCallBuilders[event.Index]
		if !ok {
			builder = &InternalToolCall{
				Index: event.Index,
				ID:    event.ToolCallID,
				Name:  event.ToolName,
			}
			a.toolCallBuilders[event.Index] = builder
		}
		builder.Arguments += event.ArgumentsDelta
	case InternalEventToolStop:
		if builder, ok := a.toolCallBuilders[event.Index]; ok {
			a.appendToolCall(*builder)
			delete(a.toolCallBuilders, event.Index)
		}
	case InternalEventUsage:
		if event.Usage != nil {
			usage := a.ensureUsage()
			if event.Usage.InputTokens > 0 {
				usage.InputTokens = event.Usage.InputTokens
			}
			if event.Usage.OutputTokens > 0 {
				usage.OutputTokens = event.Usage.OutputTokens
			}
			if event.Usage.TotalTokens > 0 {
				usage.TotalTokens = event.Usage.TotalTokens
			} else if usage.InputTokens > 0 && usage.OutputTokens > 0 {
				usage.TotalTokens = usage.InputTokens + usage.OutputTokens
			}
		}
	case InternalEventFinish:
		if event.FinishReason != "" {
			a.message.FinishReason = event.FinishReason
		}
		if event.StopSequence != "" {
			a.message.StopSequence = event.StopSequence
		}
	case InternalEventMessageStop:
		a.flushText()
		for _, builder := range a.toolCallBuilders {
			a.appendToolCall(*builder)
		}
		a.toolCallBuilders = make(map[int]*InternalToolCall)
	}
}

// Snapshot 返回当前聚合后的消息副本
func (a *MessageAggregator) Snapshot() *InternalMessage {
	a.flushText()
	clone := *a.message
	clone.Contents = append([]InternalContent(nil), a.message.Contents...)
	clone.ToolCalls = append([]InternalToolCall(nil), a.message.ToolCalls...)

	if clone.Usage != nil {
		usageCopy := *clone.Usage
		clone.Usage = &usageCopy
	}

	// 应用 Python JSON 修复（如需）
	for idx := range clone.ToolCalls {
		tc := &clone.ToolCalls[idx]
		if a.pythonFixer != nil && a.pythonFixer.ShouldApplyFix(tc.Name, tc.Arguments) {
			if fixed, ok := a.pythonFixer.FixPythonStyleJSON(tc.Arguments); ok {
				tc.Arguments = fixed
			}
		}
	}

	return &clone
}

func (a *MessageAggregator) ensureUsage() *InternalUsage {
	if a.message.Usage == nil {
		a.message.Usage = &InternalUsage{}
	}
	return a.message.Usage
}

func (a *MessageAggregator) flushText() {
	if a.textBuilder.Len() == 0 {
		return
	}
	text := a.textBuilder.String()
	a.textBuilder.Reset()
	if strings.TrimSpace(text) == "" {
		// 即便是空格，也作为内容保留，避免意外丢弃换行
	}
	a.message.Contents = append(a.message.Contents, InternalContent{
		Type: "text",
		Text: text,
	})
}

func (a *MessageAggregator) appendToolCall(call InternalToolCall) {
	if call.Arguments == "" && call.Name == "" {
		return
	}
	a.message.ToolCalls = append(a.message.ToolCalls, call)
}
