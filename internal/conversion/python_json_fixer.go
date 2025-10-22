package conversion

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/logger"
)

// PythonJSONAccumulator manages state across multiple SSE fragments
type PythonJSONAccumulator struct {
	buffer           string // Accumulated arguments content
	insideToolCall   bool   // Whether we're inside tool_calls.arguments
	pythonSyntaxMode bool   // Whether Python syntax has been detected
	braceDepth       int    // Nesting depth of braces
	inStringLiteral  bool   // Whether we're inside a string literal
	lastWasEscape    bool   // Whether the last character was an escape
}

// PythonJSONFixer handles the conversion of Python-style dictionary syntax to valid JSON
type PythonJSONFixer struct {
	logger      *logger.Logger
	config      config.PythonJSONFixingConfig
	accumulator *PythonJSONAccumulator
}

// NewPythonJSONFixer creates a new PythonJSONFixer instance
func NewPythonJSONFixer(log *logger.Logger) *PythonJSONFixer {
	// Default configuration
	defaultConfig := config.PythonJSONFixingConfig{
		Enabled:      config.Default.Validation.PythonJSONFix.Enabled,
		TargetTools:  []string{"TodoWrite"},
		DebugLogging: config.Default.Validation.PythonJSONFix.DebugLogging,
		MaxAttempts:  config.Default.Validation.PythonJSONFix.MaxAttempts,
	}

	return &PythonJSONFixer{
		logger:      log,
		config:      defaultConfig,
		accumulator: &PythonJSONAccumulator{},
	}
}

// NewPythonJSONFixerWithConfig creates a new PythonJSONFixer instance with custom configuration
func NewPythonJSONFixerWithConfig(log *logger.Logger, cfg config.PythonJSONFixingConfig) *PythonJSONFixer {
	return &PythonJSONFixer{
		logger:      log,
		config:      cfg,
		accumulator: &PythonJSONAccumulator{},
	}
}

// ProcessSSEFragment processes a single SSE fragment with accumulation support
// Returns: (processedFragment, shouldBuffer, wasFixed)
func (f *PythonJSONFixer) ProcessSSEFragment(input string) (string, bool, bool) {
	// First check if this fragment contains tool_calls arguments
	isInToolArgs := f.isInToolCallArguments(input)

	if isInToolArgs {
		f.accumulator.insideToolCall = true
	}

	// If we're inside tool_calls arguments, use accumulation logic
	if f.accumulator.insideToolCall {
		return f.processWithAccumulation(input)
	}

	// Otherwise, use traditional single-fragment processing
	fixed, wasFixed := f.FixPythonStyleJSON(input)
	return fixed, false, wasFixed
}

// processWithAccumulation handles fragments when we're inside tool_calls.arguments
func (f *PythonJSONFixer) processWithAccumulation(input string) (string, bool, bool) {
	// Extract arguments content from the fragment if possible
	argsContent := f.extractArgumentsContent(input)

	// Add to buffer
	f.accumulator.buffer += argsContent

	// Check if we should detect Python syntax in the accumulated buffer
	if !f.accumulator.pythonSyntaxMode {
		if f.DetectPythonStyle(f.accumulator.buffer) || f.DetectPythonStyleAccumulated(f.accumulator.buffer) {
			f.accumulator.pythonSyntaxMode = true
			f.debug("Python syntax detected in accumulated buffer", map[string]interface{}{
				"buffer": f.accumulator.buffer,
			})
		}
	}

	// Check if we should attempt conversion
	shouldConvert, canConvert := f.shouldAttemptConversion(input)

	if shouldConvert && canConvert && f.accumulator.pythonSyntaxMode {
		// Attempt to fix the accumulated buffer
		fixed, success := f.convertAccumulatedBuffer()
		if success {
			// Reset accumulator and return the fixed content
			f.resetAccumulator()
			return f.reconstructFragmentWithFixed(input, fixed), false, true
		}
	}

	// Check if we should reset (end of tool_calls.arguments)
	if f.shouldResetAccumulator(input) {
		f.resetAccumulator()
		return input, false, false
	}

	// Continue buffering
	return input, true, false
}

// DetectPythonStyleAccumulated checks accumulated buffer for Python-style patterns
func (f *PythonJSONFixer) DetectPythonStyleAccumulated(buffer string) bool {
	// Enhanced detection for accumulated content
	accumulatedPatterns := []string{
		`{'[^']*':\s*'[^']*'`,                // Complete key-value pair
		`{\s*'[^']*':\s*'[^']*',\s*'[^']*'`,  // Multiple key-value pairs
		`'[^']*':\s*'[^']*',\s*'[^']*':\s*'`, // Chained key-value pairs
		`^\s*{\s*'[^']*':\s*'`,               // Dict start with key
		`',\s*'[^']*':\s*'[^']*'\s*}`,        // Multiple entries with end
	}

	for _, pattern := range accumulatedPatterns {
		if matched, _ := regexp.MatchString(pattern, buffer); matched {
			return true
		}
	}

	// Check for partial but growing Python syntax
	if strings.Contains(buffer, "{'") || strings.Contains(buffer, "': '") {
		return true
	}

	return false
}

// isInToolCallArguments checks if the input contains tool_calls arguments
func (f *PythonJSONFixer) isInToolCallArguments(input string) bool {
	return strings.Contains(input, `"arguments"`) ||
		strings.Contains(input, `"function"`) ||
		(f.accumulator.insideToolCall && !f.shouldResetAccumulator(input))
}

// extractArgumentsContent extracts the arguments value from a JSON fragment
func (f *PythonJSONFixer) extractArgumentsContent(input string) string {
	// Look for arguments content patterns
	argumentsPattern := regexp.MustCompile(`"arguments":\s*"([^"]*)"`)
	if match := argumentsPattern.FindStringSubmatch(input); len(match) > 1 {
		return match[1]
	}

	// If we're already inside arguments, the whole input might be arguments content
	if f.accumulator.insideToolCall {
		// Remove JSON wrapper if present
		if strings.Contains(input, `"arguments":"`) {
			start := strings.Index(input, `"arguments":"`) + len(`"arguments":"`)
			end := strings.LastIndex(input, `"`)
			if end > start {
				return input[start:end]
			}
		}
	}

	return ""
}

// shouldAttemptConversion determines if we should try to convert the buffer
func (f *PythonJSONFixer) shouldAttemptConversion(input string) (should bool, can bool) {
	buffer := f.accumulator.buffer

	// Should convert if:
	// 1. We detect the end of arguments field
	// 2. Buffer contains what looks like complete Python dict
	// 3. Buffer reaches reasonable size threshold

	endsArguments := strings.Contains(input, `"}`) || strings.Contains(input, `"finish_reason"`)
	hasCompleteDict := strings.Count(buffer, "{") > 0 && strings.Count(buffer, "{") <= strings.Count(buffer, "}")
	bufferSizable := len(buffer) > 20

	should = endsArguments || (hasCompleteDict && bufferSizable)

	// Can convert if buffer has meaningful content
	can = len(strings.TrimSpace(buffer)) > 0 && (strings.Contains(buffer, "{") || strings.Contains(buffer, "'"))

	return should, can
}

// convertAccumulatedBuffer attempts to fix the accumulated buffer
func (f *PythonJSONFixer) convertAccumulatedBuffer() (string, bool) {
	buffer := strings.TrimSpace(f.accumulator.buffer)
	if buffer == "" {
		return "", false
	}

	// Apply the existing conversion logic
	fixed := f.convertPythonQuotes(buffer)

	// For accumulated content, we might have partial JSON, so be more lenient
	if f.isValidJSON(fixed) {
		return fixed, true
	}

	// Try to wrap in quotes if it looks like a partial value
	if !strings.HasPrefix(fixed, "{") && !strings.HasPrefix(fixed, "[") {
		wrappedFixed := `"` + strings.ReplaceAll(fixed, `"`, `\"`) + `"`
		if f.isValidJSON(wrappedFixed) {
			return wrappedFixed, true
		}
	}

	// Try to complete JSON structure if possible
	if strings.Contains(fixed, "{") && !strings.HasSuffix(fixed, "}") {
		completedFixed := fixed + "}"
		if f.isValidJSON(completedFixed) {
			return completedFixed, true
		}
	}

	return fixed, false // Return fixed version even if not valid JSON, let caller decide
}

// reconstructFragmentWithFixed rebuilds the input fragment with fixed arguments
func (f *PythonJSONFixer) reconstructFragmentWithFixed(original, fixed string) string {
	// Replace the arguments content in the original fragment
	argumentsPattern := regexp.MustCompile(`("arguments":\s*")([^"]*)(")`)
	return argumentsPattern.ReplaceAllString(original, `${1}`+fixed+`${3}`)
}

// shouldResetAccumulator determines if we should reset the accumulator state
func (f *PythonJSONFixer) shouldResetAccumulator(input string) bool {
	// Reset when we see the end of tool_calls or move to next field
	return strings.Contains(input, `"finish_reason"`) ||
		strings.Contains(input, `"index"`) ||
		strings.Contains(input, `[DONE]`) ||
		(strings.Contains(input, `}`) && !strings.Contains(input, `"arguments"`))
}

// resetAccumulator resets the accumulator state
func (f *PythonJSONFixer) resetAccumulator() {
	f.accumulator.buffer = ""
	f.accumulator.insideToolCall = false
	f.accumulator.pythonSyntaxMode = false
	f.accumulator.braceDepth = 0
	f.accumulator.inStringLiteral = false
	f.accumulator.lastWasEscape = false
}

// FixPythonStyleJSON attempts to fix Python-style JSON syntax and returns the fixed string
// along with a boolean indicating whether any fixes were applied
func (f *PythonJSONFixer) FixPythonStyleJSON(input string) (string, bool) {
	if !f.DetectPythonStyle(input) {
		return input, false
	}

	f.debug("Detected Python-style JSON, attempting to fix", map[string]interface{}{
		"original": input,
	})

	// Check if Python syntax is within an arguments field
	argumentsPattern := regexp.MustCompile(`("arguments":\s*")([^"]*)(")`)
	if argumentsPattern.MatchString(input) {
		// Fix the arguments content specifically
		fixed := argumentsPattern.ReplaceAllStringFunc(input, func(match string) string {
			submatches := argumentsPattern.FindStringSubmatch(match)
			if len(submatches) == 4 {
				prefix := submatches[1]
				argumentsContent := submatches[2]
				suffix := submatches[3]

				// Fix Python quotes in the arguments content
				fixedContent := f.convertPythonQuotes(argumentsContent)
				return prefix + fixedContent + suffix
			}
			return match
		})

		// For arguments content, validate the outer JSON structure rather than the inner content
		if f.isValidJSON(fixed) {
			f.debug("Successfully fixed Python-style JSON in arguments", map[string]interface{}{
				"original": input,
				"fixed":    fixed,
			})
			return fixed, true
		} else {
			// Even if overall JSON validation fails, return the fixed version for arguments
			// This is because arguments content might be incomplete in SSE streams
			f.debug("Fixed Python-style JSON in arguments (no validation)", map[string]interface{}{
				"original": input,
				"fixed":    fixed,
			})
			return fixed, true
		}
	} else {
		// Use existing logic for non-arguments content
		fixed := f.convertPythonQuotes(input)

		// Validate the fixed JSON
		if f.isValidJSON(fixed) {
			f.debug("Successfully fixed Python-style JSON", map[string]interface{}{
				"original": input,
				"fixed":    fixed,
			})
			return fixed, true
		}
	}

	f.debug("Failed to fix Python-style JSON - result is not valid JSON", map[string]interface{}{
		"original": input,
	})

	return input, false
}

// DetectPythonStyle checks if the input contains Python-style dictionary syntax
func (f *PythonJSONFixer) DetectPythonStyle(input string) bool {
	// Check for Python syntax within JSON strings (arguments field)
	argumentsPattern := regexp.MustCompile(`"arguments":\s*"([^"]*)"`)
	if match := argumentsPattern.FindStringSubmatch(input); len(match) > 1 {
		argumentsContent := match[1]
		if f.detectPythonSyntaxInString(argumentsContent) {
			return true
		}
	}

	// Complete patterns that indicate Python-style syntax
	completePatterns := []string{
		`{'[^']*':\s*'[^']*'}`,    // Single key-value pair: {'key': 'value'}
		`'[^']*':\s*'[^']*'`,      // Key-value fragment: 'key': 'value'
		`\[{'[^']*':\s*'[^']*'`,   // Array start: [{'key': 'value'
		`{'[^']*':\s*'[^']*',`,    // Multiple keys start: {'key': 'value',
		`',\s*'[^']*':\s*'[^']*'`, // Middle key-value: , 'key': 'value'
	}

	// Check complete patterns first
	for _, pattern := range completePatterns {
		if matched, _ := regexp.MatchString(pattern, input); matched {
			return true
		}
	}

	// SSE Stream fragment patterns - for handling split content across multiple chunks
	streamPatterns := []string{
		`^{'$`,                              // Opening dict: {'
		`^'[^']*':\s*'[^']*$`,               // Key with incomplete value: 'key': 'val
		`^[^']*',\s*'[^']*$`,                // Value end with new key start: ue', 'newkey
		`^':\s*'[^']*$`,                     // Continuation after key: ': 'value
		`^'[^']*':\s*'$`,                    // Key with colon: 'key': '
		`^'[^']*'},\s*{'$`,                  // Object transition: 'value'}, {'
		`'},\s*{'[^']*$`,                    // Object end to start: }, {'key
		`^[^']*'},\s*{'$`,                   // Value end to new object: alue'}, {'
		`^'[^']*':\s*'$`,                    // Key with start of value: 'key': '
		`':\s*'[^']*',?\s*$`,                // Key-value completion: ': 'value',
		`^[^']*',\s*'$`,                     // Value end, new key start: value', '
		`^\s*'[^']*':\s*'$`,                 // Key with colon space: 'status': '
		`^'[^']*'}\s*$`,                     // Key with object end: 'value'}
		`^\s*'[^']*':\s*$`,                  // Key with colon: 'status':
		`'}\s*,?\s*$`,                       // Object closing: '}
		`^'\s*$`,                            // Just a quote: '
		`[a-zA-Z0-9_]+'\s*:\s*'[a-zA-Z0-9]`, // Simple key-value pattern: key': 'val
	}

	// Check stream fragment patterns
	for _, pattern := range streamPatterns {
		if matched, _ := regexp.MatchString(pattern, input); matched {
			return true
		}
	}

	return false
}

// detectPythonSyntaxInString checks for Python syntax within a string value
func (f *PythonJSONFixer) detectPythonSyntaxInString(content string) bool {
	// Patterns that indicate Python dictionary syntax within a string
	pythonInStringPatterns := []string{
		`{'[^']*':\s*'[^']*'}`,     // Complete dict: {'key': 'value'}
		`{'[^']*':\s*'[^']*',`,     // Dict start: {'key': 'value',
		`',\s*'[^']*':\s*'[^']*'}`, // Dict middle to end: , 'key': 'value'}
		`'[^']*':\s*'[^']*'`,       // Key-value pair: 'key': 'value'
		`^{'[^']*':\s*'`,           // Dict start: {'key': '
		`':\s*'[^']*'$`,            // Value end: ': 'value'
		`'},\s*{'[^']*'`,           // Dict transition: '}, {'key'
	}

	for _, pattern := range pythonInStringPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}

	return false
}

// convertPythonQuotes converts Python-style single quotes to JSON double quotes
func (f *PythonJSONFixer) convertPythonQuotes(input string) string {
	runes := []rune(input)
	result := make([]rune, 0, len(runes))

	for i := 0; i < len(runes); i++ {
		if runes[i] == '\'' && f.isStructuralQuote(runes, i) {
			// Convert structural single quotes to double quotes
			result = append(result, '"')
		} else {
			result = append(result, runes[i])
		}
	}

	return string(result)
}

// isStructuralQuote determines if a quote at the given position is structural (part of JSON syntax)
// rather than content within a string value
func (f *PythonJSONFixer) isStructuralQuote(runes []rune, pos int) bool {
	if pos >= len(runes) || runes[pos] != '\'' {
		return false
	}

	// Enhanced heuristic for SSE stream fragments and complete structures

	// Find the preceding non-whitespace character
	prevNonSpace := -1
	for i := pos - 1; i >= 0; i-- {
		if !unicode.IsSpace(runes[i]) {
			prevNonSpace = i
			break
		}
	}

	// Find the following non-whitespace character
	nextNonSpace := -1
	for i := pos + 1; i < len(runes); i++ {
		if !unicode.IsSpace(runes[i]) {
			nextNonSpace = i
			break
		}
	}

	// Structural quotes typically appear:
	// 1. After {, [, or , (start of key or value)
	// 2. Before :, ,, }, or ] (end of key or value)
	// 3. At the beginning of input (pos == 0)
	// 4. At the end of input

	// Special case: beginning of input (common in SSE fragments)
	if prevNonSpace == -1 {
		return true
	}

	// Check preceding context
	if prevNonSpace >= 0 {
		prevChar := runes[prevNonSpace]
		if prevChar == '{' || prevChar == '[' || prevChar == ',' || prevChar == ':' {
			return true
		}
	}

	// Check following context
	if nextNonSpace >= 0 {
		nextChar := runes[nextNonSpace]
		if nextChar == ':' || nextChar == ',' || nextChar == '}' || nextChar == ']' {
			return true
		}
	}

	// Special case: end of input (common in SSE fragments)
	if nextNonSpace == -1 {
		return true
	}

	// Enhanced heuristics for SSE stream fragments
	// If we have very limited context, be more permissive
	inputLength := len(runes)

	// For very short fragments (likely SSE chunks), assume structural if it contains typical patterns
	if inputLength <= 5 {
		return true
	}

	// Look for common SSE fragment patterns around the quote
	startIdx := pos - 2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := pos + 3
	if endIdx > len(runes) {
		endIdx = len(runes)
	}

	context := string(runes[startIdx:endIdx])

	// Common SSE fragment patterns that indicate structural quotes
	fragmentPatterns := []string{
		"{'", // Start of dict
		"':", // Key separator
		"',", // Value separator
		"'}", // End of dict entry
		"' ", // Quote with space (often structural)
	}

	for _, pattern := range fragmentPatterns {
		if strings.Contains(context, pattern) {
			return true
		}
	}

	return false
}

// isValidJSON checks if the given string is valid JSON
func (f *PythonJSONFixer) isValidJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func (f *PythonJSONFixer) debug(msg string, fields map[string]interface{}) {
	if !f.config.DebugLogging || f.logger == nil {
		return
	}
	if fields == nil {
		fields = map[string]interface{}{}
	}
	f.logger.Debug(msg, fields)
}

// ShouldApplyFix determines if the fix should be applied based on tool name and other criteria
func (f *PythonJSONFixer) ShouldApplyFix(toolName string, content string) bool {
	// Check if fixing is enabled
	if !f.config.Enabled {
		return false
	}

	// Check if the tool is in the target tools list
	for _, targetTool := range f.config.TargetTools {
		if targetTool == toolName {
			// Only apply if we detect Python-style syntax
			return f.DetectPythonStyle(content)
		}
	}

	return false
}
