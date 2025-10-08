package toolcall

import "time"

// Tool represents a function tool definition
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function details
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a parsed tool call
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function ToolCallFunction       `json:"function"`
	Index    int                    `json:"index,omitempty"`
}

// ToolCallFunction represents the function in a tool call
type ToolCallFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"-"` // Internal use
	ArgumentsJSON string              `json:"arguments"` // JSON string for API compatibility
}

// ToolCallMapping stores tool call details for context awareness
type ToolCallMapping struct {
	Name        string                 `json:"name"`
	Arguments   map[string]interface{} `json:"args"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"created_at"`
}

// EnhanceRequest represents a request to enhance with tool calling
type EnhanceRequest struct {
	Tools         []Tool
	Messages      []map[string]interface{}
	TriggerSignal string
}

// EnhanceResult represents the enhanced request result
type EnhanceResult struct {
	SystemPrompt  string
	TriggerSignal string
	ShouldEnhance bool
}

// ParseResult represents the parsed tool calls from response
type ParseResult struct {
	ToolCalls   []ToolCall
	TextContent string
	IsToolCall  bool
}
