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

	var toolCalls []ToolCall

	// Extract <function_calls> block（标准XML格式）
	callsPattern := regexp.MustCompile(`<function_calls>([\s\S]*?)</function_calls>`)
	callsMatch := callsPattern.FindStringSubmatch(contentAfterSignal)
	var err error
	if callsMatch != nil {
		toolCalls, err = p.parseFunctionCallBlocks(callsMatch[1])
		if err != nil {
			return nil, err
		}
	} else {
		// 尝试解析 legacy Toolify 风格 <Function=xxx> 块
		toolCalls, err = p.parseLegacyFunctionBlocks(contentAfterSignal)
		if err != nil {
			return nil, err
		}
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

	toolCalls := make([]ToolCall, 0)

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

	// Support <invoke name="tool"> ... </invoke> format (Toolify-style)
	invokePattern := regexp.MustCompile(`<invoke\s+[^>]*name="([^"]+)"[^>]*>([\s\S]*?)</invoke>`)
	invokeMatches := invokePattern.FindAllStringSubmatch(callsContent, -1)
	for _, match := range invokeMatches {
		toolName := strings.TrimSpace(match[1])
		if toolName == "" {
			continue
		}
		argsMap, err := p.parseInvokeArgs(match[2])
		if err != nil {
			return nil, fmt.Errorf("failed to parse invoke args for tool %s: %w", toolName, err)
		}
		argsJSON, err := json.Marshal(argsMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal invoke args for tool %s: %w", toolName, err)
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:    fmt.Sprintf("call_%s", uuid.New().String()),
			Type:  "function",
			Index: len(toolCalls),
			Function: ToolCallFunction{
				Name:          toolName,
				Arguments:     argsMap,
				ArgumentsJSON: string(argsJSON),
			},
		})
	}

	if len(toolCalls) == 0 {
		return nil, nil
	}

	return toolCalls, nil
}

// parseLegacyFunctionBlocks 解析 <Function=xxx>...</Function=xxx> legacy 格式
func (p *FunctionCallParser) parseLegacyFunctionBlocks(content string) ([]ToolCall, error) {
	// (?is) 模式：?i 忽略大小写，?s 让 . 匹配换行
	functionPattern := regexp.MustCompile(`(?is)<function=([^>]+)>(.*?)</function=\s*\1>`)
	functionMatches := functionPattern.FindAllStringSubmatch(content, -1)
	if len(functionMatches) == 0 {
		return nil, nil
	}

	toolCalls := make([]ToolCall, 0, len(functionMatches))
	paramPattern := regexp.MustCompile(`(?is)<parameter=([^>]+)>(.*?)</parameter>`)

	for idx, match := range functionMatches {
		rawName := strings.TrimSpace(match[1])
		if rawName == "" {
			continue
		}

		// Toolify 有时会把工具名写成 Function=ToolName 或 Function=tool.toolName
		toolName := rawName

		args := make(map[string]interface{})
		parameters := paramPattern.FindAllStringSubmatch(match[2], -1)
		for _, param := range parameters {
			key := strings.TrimSpace(param[1])
			if key == "" {
				continue
			}
			value := strings.TrimSpace(param[2])
			args[key] = p.coerceValue(value)
		}

		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal legacy args for tool %s: %w", toolName, err)
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:    fmt.Sprintf("call_%s", uuid.New().String()),
			Type:  "function",
			Index: idx,
			Function: ToolCallFunction{
				Name:          toolName,
				Arguments:     args,
				ArgumentsJSON: string(argsJSON),
			},
		})
	}

	return toolCalls, nil
}

// parseInvokeArgs extracts parameters from <invoke> blocks
func (p *FunctionCallParser) parseInvokeArgs(block string) (map[string]interface{}, error) {
	args := make(map[string]interface{})

	paramPattern := regexp.MustCompile(`<parameter\s+[^>]*name="([^"]+)"[^>]*>([\s\S]*?)</parameter>`)
	matches := paramPattern.FindAllStringSubmatch(block, -1)
	for _, match := range matches {
		key := strings.TrimSpace(match[1])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(match[2])
		args[key] = p.coerceValue(value)
	}

	// Also handle <args>...</args> style nested content inside invoke
	argsPattern := regexp.MustCompile(`<args>([\s\S]*?)</args>`)
	if nested := argsPattern.FindStringSubmatch(block); nested != nil {
		nestedArgs, err := p.parseArgs(nested[0])
		if err != nil {
			return nil, err
		}
		for k, v := range nestedArgs {
			args[k] = v
		}
	}

	return args, nil
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
