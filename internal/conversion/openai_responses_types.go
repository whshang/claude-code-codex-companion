package conversion

// 简化版的 OpenAI Responses API 结构，用于格式转换

type OpenAIResponsesRequest struct {
	Model             string                   `json:"model"`
	Input             []OpenAIResponsesMessage `json:"input,omitempty"`    // 输入消息（Responses API 命名）
	Messages          []OpenAIResponsesMessage `json:"messages,omitempty"` // 🆕 兼容 messages 字段（双路径回退）
	Tools             []OpenAIResponsesTool    `json:"tools,omitempty"`
	ToolChoice        interface{}              `json:"tool_choice,omitempty"`
	Temperature       *float64                 `json:"temperature,omitempty"`
	TopP              *float64                 `json:"top_p,omitempty"`
	MaxOutputTokens   *int                     `json:"max_output_tokens,omitempty"`
	ParallelToolCalls *bool                    `json:"parallel_tool_calls,omitempty"`
	User              string                   `json:"user,omitempty"`
	Metadata          map[string]interface{}   `json:"metadata,omitempty"`
	// 🆕 采样控制参数 (参考 chat2response)
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
	N                *int               `json:"n,omitempty"`
	Stop             []string           `json:"stop,omitempty"`
	// 🆕 输出格式控制
	ResponseFormat *OpenAIResponseFormat `json:"response_format,omitempty"`
	// 🆕 推理相关字段
	ReasoningEffort    *string `json:"reasoning_effort,omitempty"`
	MaxReasoningTokens *int    `json:"max_reasoning_tokens,omitempty"`
}

type OpenAIResponsesMessage struct {
	Type    string                       `json:"type"` // 🔧 添加type字段(必需,值为"message")
	Role    string                       `json:"role"`
	Content []OpenAIResponsesContentItem `json:"content"`
}

type OpenAIResponsesContentItem struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"`
}

type OpenAIResponsesTool struct {
	Type     string                 `json:"type"`
	Function *OpenAIFunctionDef     `json:"function,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

type OpenAIResponsesResponse struct {
	ID     string                      `json:"id"`
	Model  string                      `json:"model"`
	Status string                      `json:"status,omitempty"`
	Output []OpenAIResponsesOutputItem `json:"output"`
	Usage  *OpenAIResponsesUsage       `json:"usage,omitempty"`
}

type OpenAIResponsesOutputItem struct {
	Type      string                       `json:"type"`
	ID        string                       `json:"id,omitempty"`
	Status    string                       `json:"status,omitempty"`
	Role      string                       `json:"role,omitempty"`
	Content   []OpenAIResponsesContentItem `json:"content,omitempty"`
	Name      string                       `json:"name,omitempty"`
	Arguments string                       `json:"arguments,omitempty"`
}

type OpenAIResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
