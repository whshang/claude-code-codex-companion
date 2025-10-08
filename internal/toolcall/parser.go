package toolcall

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// FunctionCallParser parses XML-formatted function calls from LLM responses
type FunctionCallParser struct{}

// NewFunctionCallParser creates a new parser
func NewFunctionCallParser() *FunctionCallParser {
	return &FunctionCallParser{}
}

// Parse extracts function calls from XML response
func (p *FunctionCallParser) Parse(response string, triggerSignal string) (*ParseResult, error) {
	if response == "" || triggerSignal == "" {
		return &ParseResult{IsToolCall: false, TextContent: response}, nil
	}

	// Check if trigger signal exists
	if !strings.Contains(response, triggerSignal) {
		return &ParseResult{IsToolCall: false, TextContent: response}, nil
	}

	// Remove <think> blocks temporarily for parsing
	cleaned := p.removeThinkBlocks(response)

	// Find last occurrence of trigger signal
	lastSignalPos := strings.LastIndex(cleaned, triggerSignal)
	if lastSignalPos == -1 {
		return &ParseResult{IsToolCall: false, TextContent: response}, nil
	}

	// Extract content after trigger signal
	contentAfterSignal := cleaned[lastSignalPos:]

	// Extract <function_calls> block
	callsPattern := regexp.MustCompile(`<function_calls>([\s\S]*?)</function_calls>`)
	callsMatch := callsPattern.FindStringSubmatch(contentAfterSignal)
	if callsMatch == nil {
		return &ParseResult{IsToolCall: false, TextContent: response}, nil
	}

	callsContent := callsMatch[1]

	// Parse individual <function_call> blocks
	toolCalls, err := p.parseFunctionCallBlocks(callsContent)
	if err != nil {
		return nil, err
	}

	if len(toolCalls) == 0 {
		return &ParseResult{IsToolCall: false, TextContent: response}, nil
	}

	// Extract text content before trigger signal (from original response, preserving <think>)
	textContent := ""
	if lastSignalPosInOriginal := strings.LastIndex(response, triggerSignal); lastSignalPosInOriginal > 0 {
		textContent = strings.TrimSpace(response[:lastSignalPosInOriginal])
	}

	return &ParseResult{
		ToolCalls:   toolCalls,
		TextContent: textContent,
		IsToolCall:  true,
	}, nil
}

// removeThinkBlocks removes <think>...</think> blocks, supports nesting
func (p *FunctionCallParser) removeThinkBlocks(text string) string {
	for strings.Contains(text, "<think>") && strings.Contains(text, "</think>") {
		startPos := strings.Index(text, "<think>")
		if startPos == -1 {
			break
		}

		pos := startPos + 7 // len("<think>")
		depth := 1

		for pos < len(text) && depth > 0 {
			if strings.HasPrefix(text[pos:], "<think>") {
				depth++
				pos += 7
			} else if strings.HasPrefix(text[pos:], "</think>") {
				depth--
				pos += 8
			} else {
				pos++
			}
		}

		if depth == 0 {
			text = text[:startPos] + text[pos:]
		} else {
			break
		}
	}

	return text
}

// parseFunctionCallBlocks extracts individual function calls
func (p *FunctionCallParser) parseFunctionCallBlocks(callsContent string) ([]ToolCall, error) {
	callPattern := regexp.MustCompile(`<function_call>([\s\S]*?)</function_call>`)
	callMatches := callPattern.FindAllStringSubmatch(callsContent, -1)

	if len(callMatches) == 0 {
		return nil, nil
	}

	toolCalls := make([]ToolCall, 0, len(callMatches))

	for i, match := range callMatches {
		block := match[1]

		// Extract tool name
		toolPattern := regexp.MustCompile(`<tool>(.*?)</tool>`)
		toolMatch := toolPattern.FindStringSubmatch(block)
		if toolMatch == nil {
			continue
		}

		toolName := strings.TrimSpace(toolMatch[1])

		// Extract arguments
		args, err := p.parseArgs(block)
		if err != nil {
			return nil, fmt.Errorf("failed to parse args for tool %s: %w", toolName, err)
		}

		// Convert args to JSON string
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal args for tool %s: %w", toolName, err)
		}

		toolCall := ToolCall{
			ID:    fmt.Sprintf("call_%s", uuid.New().String()),
			Type:  "function",
			Index: i,
			Function: ToolCallFunction{
				Name:          toolName,
				Arguments:     args,
				ArgumentsJSON: string(argsJSON),
			},
		}

		toolCalls = append(toolCalls, toolCall)
	}

	return toolCalls, nil
}

// parseArgs extracts arguments from <args> block
func (p *FunctionCallParser) parseArgs(block string) (map[string]interface{}, error) {
	args := make(map[string]interface{})

	// Extract <args> block
	argsPattern := regexp.MustCompile(`<args>([\s\S]*?)</args>`)
	argsMatch := argsPattern.FindStringSubmatch(block)
	if argsMatch == nil {
		return args, nil
	}

	argsContent := argsMatch[1]

	// Match individual argument tags (supports hyphens and special characters)
	// Pattern: <key>value</key> where key can contain hyphens, underscores, etc.
	argPattern := regexp.MustCompile(`<([^\s>/]+)>([\s\S]*?)</\1>`)
	argMatches := argPattern.FindAllStringSubmatch(argsContent, -1)

	for _, match := range argMatches {
		key := match[1]
		value := match[2]

		// Try to parse as JSON, fallback to string
		args[key] = p.coerceValue(value)
	}

	return args, nil
}

// coerceValue attempts to parse value as JSON, falls back to string
func (p *FunctionCallParser) coerceValue(value string) interface{} {
	value = strings.TrimSpace(value)

	// Try parsing as JSON
	var jsonValue interface{}
	if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
		return jsonValue
	}

	// Return as string
	return value
}
