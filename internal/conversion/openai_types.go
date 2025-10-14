package conversion

// OpenAI API 结构定义 - 基于参考实现

// OpenAIRequest OpenAI Chat Completions 请求（2025 以后官方推荐字段名）
type OpenAIRequest struct {
	Model               string                 `json:"model"`
	Messages            []OpenAIMessage        `json:"messages"`
	Tools               []OpenAITool           `json:"tools,omitempty"`       // functions
	ToolChoice          interface{}            `json:"tool_choice,omitempty"` // "none"|"auto"|{"type":"function","function":{"name":...}}|"required"
	Temperature         *float64               `json:"temperature,omitempty"`
	TopP                *float64               `json:"top_p,omitempty"`
	MaxCompletionTokens *int                   `json:"max_completion_tokens,omitempty"` // OpenAI: 输出最大 token
	MaxOutputTokens     *int                   `json:"max_output_tokens,omitempty"`     // OpenAI: 新的输出最大 token 字段
	MaxTokens           *int                   `json:"max_tokens,omitempty"`            // 兼容保留：老字段，有些代理仍在用
	Stream              *bool                  `json:"stream,omitempty"`
	Stop                []string               `json:"stop,omitempty"`
	User                string                 `json:"user,omitempty"`
	ParallelToolCalls   *bool                  `json:"parallel_tool_calls,omitempty"`
	// 🆕 采样控制参数 (参考 chat2response)
	PresencePenalty     *float64               `json:"presence_penalty,omitempty"`  // 存在惩罚 (-2.0 to 2.0)
	FrequencyPenalty    *float64               `json:"frequency_penalty,omitempty"` // 频率惩罚 (-2.0 to 2.0)
	LogitBias           map[string]float64     `json:"logit_bias,omitempty"`        // token ID 到偏置值的映射
	N                   *int                   `json:"n,omitempty"`                 // 生成多个候选响应
	// 🆕 输出格式控制
	ResponseFormat      *OpenAIResponseFormat  `json:"response_format,omitempty"`   // {"type":"json_object"|"text",...}
	// 推理相关字段 (o1 模型)
	ReasoningEffort     *string                `json:"reasoning_effort,omitempty"`     // "low"|"medium"|"high" 推理强度
	MaxReasoningTokens  *int                   `json:"max_reasoning_tokens,omitempty"` // 推理阶段的最大 token 数
}

// OpenAIResponseFormat 定义输出格式约束
type OpenAIResponseFormat struct {
	Type   string                 `json:"type"`             // "text"|"json_object"|"json_schema"
	Schema map[string]interface{} `json:"schema,omitempty"` // 当 type="json_schema" 时的 JSON Schema
}

// OpenAIMessage OpenAI 消息结构
type OpenAIMessage struct {
	Role       string `json:"role"`              // "system" | "user" | "assistant" | "tool"
	Content    interface{} `json:"content,omitempty"` // string | []OpenAIMessageContent
	Name       string `json:"name,omitempty"`    // 对 "tool" 角色无须设置
	ToolCallID string `json:"tool_call_id,omitempty"`
	// 仅 assistant 会用到
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIMessageContent 复合内容：text / image_url
type OpenAIMessageContent struct {
	Type     string      `json:"type"` // "text" | "image_url"
	Text     string      `json:"text,omitempty"`
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"`
}

// OpenAIImageURL 图片URL结构
type OpenAIImageURL struct {
	// OpenAI 支持 "data:image/png;base64,..." 形式
	URL string `json:"url"`
}

// OpenAITool 工具（function）
type OpenAITool struct {
	Type     string        `json:"type"` // "function"
	Function OpenAIFunctionDef `json:"function"`
}

// OpenAIFunctionDef 函数定义
type OpenAIFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenAIToolCall 工具调用
type OpenAIToolCall struct {
	Index    int              `json:"index,omitempty"`    // 在streaming中用于标识工具调用的索引
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function OpenAIToolCallDetail `json:"function"`
}

// OpenAIToolCallDetail 工具调用详情
type OpenAIToolCallDetail struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON text
}

// OpenAIResponse OpenAI 响应（非流式）
type OpenAIResponse struct {
	ID      string     `json:"id"`
	Model   string     `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIChoice 选择结构
type OpenAIChoice struct {
	Index        int       `json:"index"`
	FinishReason string    `json:"finish_reason"` // "stop"|"tool_calls"|...
	Message      OpenAIMessage `json:"message"`
}

// OpenAIUsage 使用统计
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamChunk OpenAI 流式片段（SSE 的 delta 合并结果；这里假定你已收集完所有 chunk）
type OpenAIStreamChunk struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Choices []OpenAIStreamChoice `json:"choices"`
	Usage   *OpenAIUsage     `json:"usage,omitempty"` // 可能在最后一个chunk中包含
}

// OpenAIStreamChoice 流式选择
type OpenAIStreamChoice struct {
	Index        int       `json:"index"`
	Delta        OpenAIMessage `json:"delta"`         // 增量：content 片段、或 tool_calls 的增量
	FinishReason string    `json:"finish_reason"` // 片段可能为空，最后一个包含 finish_reason
	Usage        *OpenAIUsage `json:"usage,omitempty"` // 可能在最后一个choice中包含usage信息
}