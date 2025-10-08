package toolcall

import (
	"encoding/json"
	"fmt"
	"time"
)

// Enhancer is the main facade for tool calling enhancement
type Enhancer struct {
	promptGen *PromptGenerator
	parser    *FunctionCallParser
	cache     *ToolCallCache
}

// NewEnhancer creates a new enhancer instance
func NewEnhancer(cacheConfig CacheConfig) *Enhancer {
	enhancer := &Enhancer{
		promptGen: NewPromptGenerator(),
		parser:    NewFunctionCallParser(),
		cache:     NewToolCallCache(cacheConfig),
	}

	// Start background cleanup routine for cache
	enhancer.cache.StartCleanupRoutine(5 * time.Minute)

	return enhancer
}

// EnhanceRequest enhances a request with tool calling capabilities
func (e *Enhancer) EnhanceRequest(tools []Tool, messages []map[string]interface{}) (*EnhanceResult, string, error) {
	// Check if enhancement is needed
	if len(tools) == 0 {
		return &EnhanceResult{
			ShouldEnhance: false,
		}, "", nil
	}

	// Generate system prompt with tool descriptions
	systemPrompt, triggerSignal := e.promptGen.GeneratePrompt(tools)

	// Enhance system prompt with recent tool call context
	enhancedPrompt := e.enhanceWithContext(systemPrompt)

	return &EnhanceResult{
		SystemPrompt:  enhancedPrompt,
		TriggerSignal: triggerSignal,
		ShouldEnhance: true,
	}, triggerSignal, nil
}

// enhanceWithContext adds recent tool call context to system prompt
func (e *Enhancer) enhanceWithContext(basePrompt string) string {
	// Get recent tool calls from cache
	recentCalls := e.cache.GetRecent(5)

	if len(recentCalls) == 0 {
		return basePrompt
	}

	// Build context section
	contextSection := "\n\n**RECENT TOOL CALLS CONTEXT:**\n"
	contextSection += "The following tools were recently called. You can reference their results in the conversation history:\n\n"

	for i, call := range recentCalls {
		argsJSON, _ := json.Marshal(call.Arguments)
		contextSection += fmt.Sprintf("%d. Tool: %s\n", i+1, call.Name)
		contextSection += fmt.Sprintf("   Arguments: %s\n", string(argsJSON))
		if call.Description != "" {
			contextSection += fmt.Sprintf("   Description: %s\n", call.Description)
		}
		contextSection += "\n"
	}

	return basePrompt + contextSection
}

// ParseResponse parses LLM response to extract tool calls
func (e *Enhancer) ParseResponse(response string, triggerSignal string) (*ParseResult, error) {
	result, err := e.parser.Parse(response, triggerSignal)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Store tool calls in cache for context awareness
	if result.IsToolCall {
		for _, toolCall := range result.ToolCalls {
			mapping := ToolCallMapping{
				Name:        toolCall.Function.Name,
				Arguments:   toolCall.Function.Arguments,
				Description: "", // Can be enhanced with tool description lookup
				CreatedAt:   time.Now(),
			}
			e.cache.Set(toolCall.ID, mapping)
		}
	}

	return result, nil
}

// CreateStreamingDetector creates a new streaming detector with the given trigger signal
func (e *Enhancer) CreateStreamingDetector(triggerSignal string) *StreamingDetector {
	return NewStreamingDetector(triggerSignal)
}

// EnhanceToolResult enhances tool execution result with context
func (e *Enhancer) EnhanceToolResult(toolCallID string, result interface{}) map[string]interface{} {
	enhanced := map[string]interface{}{
		"type": "tool_result",
		"tool_use_id": toolCallID,
		"content": result,
	}

	// Add context from cache if available
	if mapping, found := e.cache.Get(toolCallID); found {
		enhanced["tool_name"] = mapping.Name

		// Add arguments summary for better context
		if len(mapping.Arguments) > 0 {
			argsJSON, _ := json.Marshal(mapping.Arguments)
			enhanced["tool_arguments"] = string(argsJSON)
		}
	}

	return enhanced
}

// GetCacheStats returns cache statistics
func (e *Enhancer) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size": e.cache.Size(),
		"max_size": e.cache.config.MaxSize,
		"ttl_seconds": e.cache.config.TTL.Seconds(),
	}
}

// ClearCache clears all cached tool call mappings
func (e *Enhancer) ClearCache() {
	e.cache.Clear()
}
