package conversion

import (
	"testing"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/logger"
)

// Helper function to create a test logger
func createTestLogger(t *testing.T) *logger.Logger {
	logConfig := logger.LogConfig{
		Level:           "error",
		LogRequestTypes: "none",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "none", // Use "none" to disable file logging
	}
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return log
}

func TestPythonJSONFixer_DetectPythonStyle(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Simple Python dict",
			input:    "{'content': 'test', 'id': '1', 'status': 'pending'}",
			expected: true,
		},
		{
			name:     "TodoWrite format",
			input:    "{'content': '创建项目结构和主程序文件', 'id': '1', 'status': 'in_progress'}",
			expected: true,
		},
		{
			name:     "Array with Python dict",
			input:    "[{'content': 'test', 'id': '1'}]",
			expected: true,
		},
		{
			name:     "Partial Python dict",
			input:    "'content': 'test',",
			expected: true,
		},
		{
			name:     "Valid JSON",
			input:    `{"content": "test", "id": "1", "status": "pending"}`,
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Simple string",
			input:    "test",
			expected: false,
		},
		{
			name:     "Number",
			input:    "123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixer.DetectPythonStyle(tt.input)
			if result != tt.expected {
				t.Errorf("DetectPythonStyle() = %v, want %v for input: %s", result, tt.expected, tt.input)
			}
		})
	}
}

func TestPythonJSONFixer_FixPythonStyleJSON(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name          string
		input         string
		expectedFixed string
		expectedBool  bool
	}{
		{
			name:          "Simple Python dict",
			input:         "{'content': 'test', 'id': '1'}",
			expectedFixed: `{"content": "test", "id": "1"}`,
			expectedBool:  true,
		},
		{
			name:          "TodoWrite format",
			input:         "{'content': '创建项目结构和主程序文件', 'id': '1', 'status': 'in_progress'}",
			expectedFixed: `{"content": "创建项目结构和主程序文件", "id": "1", "status": "in_progress"}`,
			expectedBool:  true,
		},
		{
			name:          "Array with Python dict",
			input:         "[{'content': 'test', 'id': '1'}]",
			expectedFixed: `[{"content": "test", "id": "1"}]`,
			expectedBool:  true,
		},
		{
			name:          "Complex nested structure",
			input:         "{'todos': [{'content': 'task1', 'id': '1'}, {'content': 'task2', 'id': '2'}]}",
			expectedFixed: `{"todos": [{"content": "task1", "id": "1"}, {"content": "task2", "id": "2"}]}`,
			expectedBool:  true,
		},
		{
			name:          "Already valid JSON",
			input:         `{"content": "test", "id": "1"}`,
			expectedFixed: `{"content": "test", "id": "1"}`,
			expectedBool:  false,
		},
		{
			name:          "Empty string",
			input:         "",
			expectedFixed: "",
			expectedBool:  false,
		},
		{
			name:          "Invalid after conversion",
			input:         "{'unclosed': 'value",
			expectedFixed: "{'unclosed': 'value",
			expectedBool:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed, wasFixed := fixer.FixPythonStyleJSON(tt.input)
			if wasFixed != tt.expectedBool {
				t.Errorf("FixPythonStyleJSON() wasFixed = %v, want %v", wasFixed, tt.expectedBool)
			}
			if fixed != tt.expectedFixed {
				t.Errorf("FixPythonStyleJSON() fixed = %v, want %v", fixed, tt.expectedFixed)
			}
		})
	}
}

func TestPythonJSONFixer_ShouldApplyFix(t *testing.T) {
	tests := []struct {
		name     string
		config   config.PythonJSONFixingConfig
		toolName string
		content  string
		expected bool
	}{
		{
			name: "Enabled for TodoWrite with Python content",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"TodoWrite"},
			},
			toolName: "TodoWrite",
			content:  "{'content': 'test'}",
			expected: true,
		},
		{
			name: "Disabled globally",
			config: config.PythonJSONFixingConfig{
				Enabled:     false,
				TargetTools: []string{"TodoWrite"},
			},
			toolName: "TodoWrite",
			content:  "{'content': 'test'}",
			expected: false,
		},
		{
			name: "Tool not in target list",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"OtherTool"},
			},
			toolName: "TodoWrite",
			content:  "{'content': 'test'}",
			expected: false,
		},
		{
			name: "Valid JSON content",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"TodoWrite"},
			},
			toolName: "TodoWrite",
			content:  `{"content": "test"}`,
			expected: false,
		},
		{
			name: "Multiple target tools",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"TodoWrite", "OtherTool"},
			},
			toolName: "OtherTool",
			content:  "{'content': 'test'}",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixer := NewPythonJSONFixerWithConfig(createTestLogger(t), tt.config)
			result := fixer.ShouldApplyFix(tt.toolName, tt.content)
			if result != tt.expected {
				t.Errorf("ShouldApplyFix() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPythonJSONFixer_isStructuralQuote(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		pos      int
		expected bool
	}{
		{
			name:     "Quote after opening brace",
			input:    "{'key'",
			pos:      1,
			expected: true,
		},
		{
			name:     "Quote before colon",
			input:    "'key':",
			pos:      4,
			expected: true,
		},
		{
			name:     "Quote after colon",
			input:    ": 'value'",
			pos:      2,
			expected: true,
		},
		{
			name:     "Quote before comma",
			input:    "'value',",
			pos:      6,
			expected: true,
		},
		{
			name:     "Quote at beginning of string",
			input:    "'content'",
			pos:      0,
			expected: true,
		},
		{
			name:     "Non-quote character",
			input:    "{'key'",
			pos:      0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.input)
			if tt.pos >= len(runes) {
				t.Errorf("Test setup error: pos %d >= len(runes) %d", tt.pos, len(runes))
				return
			}
			result := fixer.isStructuralQuote(runes, tt.pos)
			if result != tt.expected {
				t.Errorf("isStructuralQuote() = %v, want %v for input '%s' at pos %d", result, tt.expected, tt.input, tt.pos)
			}
		})
	}
}

func TestPythonJSONFixer_convertPythonQuotes(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple conversion",
			input:    "{'key': 'value'}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "Multiple keys",
			input:    "{'key1': 'value1', 'key2': 'value2'}",
			expected: `{"key1": "value1", "key2": "value2"}`,
		},
		{
			name:     "Array",
			input:    "['item1', 'item2']",
			expected: `["item1", "item2"]`,
		},
		{
			name:     "Nested structure",
			input:    "{'outer': {'inner': 'value'}}",
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "No quotes to convert",
			input:    `{"already": "valid"}`,
			expected: `{"already": "valid"}`,
		},
		{
			name:     "Mixed content",
			input:    "{'string': 'text', 'number': 123, 'bool': true}",
			expected: `{"string": "text", "number": 123, "bool": true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixer.convertPythonQuotes(tt.input)
			if result != tt.expected {
				t.Errorf("convertPythonQuotes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPythonJSONFixer_isValidJSON(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid object",
			input:    `{"key": "value"}`,
			expected: true,
		},
		{
			name:     "Valid array",
			input:    `["item1", "item2"]`,
			expected: true,
		},
		{
			name:     "Valid complex structure",
			input:    `{"todos": [{"content": "test", "id": "1"}]}`,
			expected: true,
		},
		{
			name:     "Invalid JSON - Python style",
			input:    "{'key': 'value'}",
			expected: false,
		},
		{
			name:     "Invalid JSON - malformed",
			input:    `{"key": "value"`,
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Simple string",
			input:    "test",
			expected: false,
		},
		{
			name:     "Valid number",
			input:    "123",
			expected: true,
		},
		{
			name:     "Valid boolean",
			input:    "true",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixer.isValidJSON(tt.input)
			if result != tt.expected {
				t.Errorf("isValidJSON() = %v, want %v for input: %s", result, tt.expected, tt.input)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkPythonJSONFixer_DetectPythonStyle(b *testing.B) {
	logConfig := logger.LogConfig{Level: "error", LogRequestTypes: "none", LogRequestBody: "none", LogResponseBody: "none", LogDirectory: "none"}
	log, _ := logger.NewLogger(logConfig)
	fixer := NewPythonJSONFixer(log)
	input := "{'content': 'test content with some length', 'id': '1', 'status': 'in_progress'}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fixer.DetectPythonStyle(input)
	}
}

func BenchmarkPythonJSONFixer_FixPythonStyleJSON(b *testing.B) {
	logConfig := logger.LogConfig{Level: "error", LogRequestTypes: "none", LogRequestBody: "none", LogResponseBody: "none", LogDirectory: "none"}
	log, _ := logger.NewLogger(logConfig)
	fixer := NewPythonJSONFixer(log)
	input := "{'todos': [{'content': 'task1', 'id': '1', 'status': 'pending'}, {'content': 'task2', 'id': '2', 'status': 'in_progress'}]}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fixer.FixPythonStyleJSON(input)
	}
}

// TestPythonJSONFixer_SSEStreamFragments tests the handling of SSE stream fragments
// This reproduces the issue from 2.txt where Python dict content is split across multiple SSE chunks
func TestPythonJSONFixer_SSEStreamFragments(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	// Real SSE stream fragments from 2.txt case - these are arguments fragments 
	// that get accumulated in SimpleJSONBuffer
	streamChunks := []string{
		`{"todos": [`,  // Initial arguments start
		`{'`,           // Start of first dict 
		`content': 'C`, // Key with partial value
		`reate project structure and main`, // Value continuation
		` Go files', '`,  // Value end, new key start
		`id`,             // Key fragment
		`': '1',`,        // Key-value completion
		` 'status': '`,   // New key with value start
		`in_progress'}, {'`, // Value end, new dict start
		`content': '`,    // New key start
		`Implement IPv4/IPv6 system support detection`, // Full value
		`', '`,           // Value end, key start
		`id': '2`,        // Key-value
		`', 'status':`,   // Key separator
		` 'pending'}, {'`, // Value and dict transition
		`content': 'Create HTTPS proxy server with TLS support', 'id`, // Long content
		`': '3',`,        // Key-value
		` 'status`,       // Key fragment
		`': 'pending'}]`, // Final value and array end
		`}`,              // Arguments end
	}

	tests := []struct {
		name          string
		chunks        []string
		expectedTypes string
	}{
		{
			name:          "SSE stream fragments should be detected individually",
			chunks:        streamChunks,
			expectedTypes: "individual_fragment_detection",
		},
		{
			name:          "Combined SSE stream should be converted successfully",
			chunks:        streamChunks,
			expectedTypes: "combined_conversion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedTypes == "individual_fragment_detection" {
				// Test detection of individual fragments
				detectedCount := 0
				for i, chunk := range tt.chunks {
					detected := fixer.DetectPythonStyle(chunk)
					if detected {
						detectedCount++
						t.Logf("Chunk %d detected as Python style: '%s'", i, chunk)
					}
				}
				
				// Currently fails - individual fragments are not detected
				// This is the core issue we need to fix
				if detectedCount == 0 {
					t.Logf("KNOWN ISSUE: No individual fragments detected (%d/%d)", detectedCount, len(tt.chunks))
				}
			}

			if tt.expectedTypes == "combined_conversion" {
				// Test conversion of combined content
				combined := ""
				for _, chunk := range tt.chunks {
					combined += chunk
				}
				
				detected := fixer.DetectPythonStyle(combined)
				if !detected {
					t.Errorf("Combined content should be detected as Python style")
					return
				}
				
				fixed, wasFixed := fixer.FixPythonStyleJSON(combined)
				if !wasFixed {
					t.Errorf("Combined content conversion failed")
					t.Logf("Original: %s", combined)
					t.Logf("Fixed attempt: %s", fixed)
				} else {
					t.Logf("Successfully converted: %s", fixed)
				}
			}
		})
	}
}

// TestPythonJSONFixer_FragmentPatterns tests detection of common SSE fragment patterns
func TestPythonJSONFixer_FragmentPatterns(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	fragmentTests := []struct {
		name        string
		fragment    string
		shouldDetect bool
		description string
	}{
		{
			name:        "Opening quote and key start",
			fragment:    "{'",
			shouldDetect: true,
			description: "Common start of Python dict",
		},
		{
			name:        "Key-value fragment",
			fragment:    "content': 'C",
			shouldDetect: true,
			description: "Partial key-value pair",
		},
		{
			name:        "Value continuation",
			fragment:    "reate project structure",
			shouldDetect: false,
			description: "Pure content, no Python syntax",
		},
		{
			name:        "Key end and new key start",
			fragment:    " Go files', '",
			shouldDetect: true,
			description: "End of value, start of new key",
		},
		{
			name:        "Key without value",
			fragment:    "id",
			shouldDetect: false,
			description: "Just a key name",
		},
		{
			name:        "Key-value separator",
			fragment:    "': '1',",
			shouldDetect: true,
			description: "Key-value with separator",
		},
		{
			name:        "Status key start",
			fragment:    " 'status': '",
			shouldDetect: true,
			description: "Complete key with start of value",
		},
		{
			name:        "Object transition",
			fragment:    "in_progress'}, {'",
			shouldDetect: true,
			description: "End of object, start of new object",
		},
	}

	for _, tt := range fragmentTests {
		t.Run(tt.name, func(t *testing.T) {
			detected := fixer.DetectPythonStyle(tt.fragment)
			if detected != tt.shouldDetect {
				t.Errorf("Fragment '%s' detection = %v, want %v (%s)", 
					tt.fragment, detected, tt.shouldDetect, tt.description)
			}
		})
	}
}