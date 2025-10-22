package conversion

import "time"

// InternalRequest is the canonical in-memory representation used by the adapter
// layer when translating between provider specific request formats.
type InternalRequest struct {
	ID                  string                  `json:"id,omitempty"`
	Model               string                  `json:"model,omitempty"`
	Messages            []InternalMessage       `json:"messages,omitempty"`
	Tools               []InternalTool          `json:"tools,omitempty"`
	ToolChoice          *InternalToolChoice     `json:"tool_choice,omitempty"`
	Temperature         *float64                `json:"temperature,omitempty"`
	TopP                *float64                `json:"top_p,omitempty"`
	MaxCompletionTokens *int                    `json:"max_completion_tokens,omitempty"`
	MaxOutputTokens     *int                    `json:"max_output_tokens,omitempty"`
	MaxTokens           *int                    `json:"max_tokens,omitempty"`
	Stream              bool                    `json:"stream,omitempty"`
	Stop                []string                `json:"stop,omitempty"`
	User                string                  `json:"user,omitempty"`
	ParallelToolCalls   *bool                   `json:"parallel_tool_calls,omitempty"`
	Metadata            map[string]interface{}  `json:"metadata,omitempty"`
	PresencePenalty     *float64                `json:"presence_penalty,omitempty"`
	FrequencyPenalty    *float64                `json:"frequency_penalty,omitempty"`
	LogitBias           map[string]float64      `json:"logit_bias,omitempty"`
	N                   *int                    `json:"n,omitempty"`
	ResponseFormat      *InternalResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort     *string                 `json:"reasoning_effort,omitempty"`
	MaxReasoningTokens  *int                    `json:"max_reasoning_tokens,omitempty"`
	Thinking            *InternalThinking       `json:"thinking,omitempty"`
}

// InternalMessage represents a role based message comprised of structured
// content blocks (text, images, tool calls, etc).
type InternalMessage struct {
	Role       string             `json:"role"`
	Name       string             `json:"name,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
	Contents   []InternalContent  `json:"contents,omitempty"`
	ToolCalls  []InternalToolCall `json:"tool_calls,omitempty"`
}

// InternalContent captures individual pieces of message content.
type InternalContent struct {
	Type           string              `json:"type"`
	Text           string              `json:"text,omitempty"`
	ImageURL       string              `json:"image_url,omitempty"`
	ImageMediaType string              `json:"image_media_type,omitempty"`
	ToolUse        *InternalToolUse    `json:"tool_use,omitempty"`
	ToolResult     *InternalToolResult `json:"tool_result,omitempty"`
	Thinking       *InternalThinking   `json:"thinking,omitempty"`
}

// InternalTool describes a callable tool/function that the model may invoke.
type InternalTool struct {
	Type     string                `json:"type"`
	Function *InternalToolFunction `json:"function,omitempty"`
}

// InternalToolFunction provides the function metadata for a tool.
type InternalToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// InternalToolChoice captures tool routing preferences.
type InternalToolChoice struct {
	Type         string `json:"type"`
	FunctionName string `json:"function_name,omitempty"`
}

// InternalToolCall describes a single tool invocation or delta.
type InternalToolCall struct {
	ID        string `json:"id,omitempty"`
	Type      string `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Index     int    `json:"index,omitempty"`
	// Provider specific metadata that may be useful for streaming reconciliation.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// InternalResponseFormat mirrors OpenAI/Responses response_format settings.
type InternalResponseFormat struct {
	Type   string                 `json:"type"`
	Schema map[string]interface{} `json:"schema,omitempty"`
}

// TokenUsage captures prompt/completion token accounting.
type TokenUsage struct {
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	RecordedAt       time.Time `json:"recorded_at,omitempty"`
}

// InternalResponse is the canonical representation of a provider response.
type InternalResponse struct {
	ID           string                 `json:"id,omitempty"`
	Model        string                 `json:"model,omitempty"`
	Messages     []InternalMessage      `json:"messages,omitempty"`
	Content      []InternalContent      `json:"content,omitempty"`
	StopReason   string                 `json:"stop_reason,omitempty"`
	FinishReason string                 `json:"finish_reason,omitempty"`
	StopSequence string                 `json:"stop_sequence,omitempty"`
	TokenUsage   *TokenUsage            `json:"token_usage,omitempty"`
	Error        *ConversionError       `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Success      bool                   `json:"success"`
	Thinking     *InternalThinking      `json:"thinking,omitempty"`
}

// InternalEvent is used when normalising streaming events into a provider
// agnostic representation.
type InternalEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// InternalToolUse captures structured tool invocation data embedded in content blocks.
type InternalToolUse struct {
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Arguments    string                 `json:"arguments,omitempty"`
	ArgumentsMap map[string]interface{} `json:"arguments_map,omitempty"`
	Index        int                    `json:"index,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// InternalToolResult represents the output returned to the model after a tool call.
type InternalToolResult struct {
	ToolUseID   string                 `json:"tool_use_id,omitempty"`
	Content     string                 `json:"content,omitempty"`
	ContentList []InternalContent      `json:"content_list,omitempty"`
	IsError     bool                   `json:"is_error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// InternalThinking captures provider specific thinking / reasoning traces.
type InternalThinking struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Provider     string `json:"provider,omitempty"`
}
