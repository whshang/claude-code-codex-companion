package toolcall

import (
	"strings"
)

// StreamingDetector detects function calls in streaming responses
type StreamingDetector struct {
	triggerSignal  string
	contentBuffer  string
	state          string // "detecting" or "tool_parsing"
	inThinkBlock   bool
	thinkDepth     int
	lastYieldIndex int
}

// NewStreamingDetector creates a new streaming detector
func NewStreamingDetector(triggerSignal string) *StreamingDetector {
	return &StreamingDetector{
		triggerSignal:  triggerSignal,
		contentBuffer:  "",
		state:          "detecting",
		inThinkBlock:   false,
		thinkDepth:     0,
		lastYieldIndex: 0,
	}
}

// ProcessChunk processes a streaming chunk and returns whether it's a tool call
// and the content that should be yielded to the client
func (d *StreamingDetector) ProcessChunk(delta string) (isToolCall bool, contentToYield string) {
	if delta == "" {
		return false, ""
	}

	d.contentBuffer += delta

	// Update think block tracking
	d.updateThinkBlockState()

	switch d.state {
	case "detecting":
		return d.handleDetectingState()
	case "tool_parsing":
		return d.handleToolParsingState()
	default:
		return false, delta
	}
}

// updateThinkBlockState updates the think block depth tracking
func (d *StreamingDetector) updateThinkBlockState() {
	// Count opening and closing think tags in the buffer
	openCount := strings.Count(d.contentBuffer, "<think>")
	closeCount := strings.Count(d.contentBuffer, "</think>")

	d.thinkDepth = openCount - closeCount
	d.inThinkBlock = d.thinkDepth > 0
}

// handleDetectingState processes chunks in detecting state
func (d *StreamingDetector) handleDetectingState() (isToolCall bool, contentToYield string) {
	// Don't trigger inside think blocks
	if d.inThinkBlock {
		return d.yieldNewContent()
	}

	// Check if trigger signal appears
	if strings.Contains(d.contentBuffer, d.triggerSignal) {
		// Find the position of trigger signal
		triggerPos := strings.Index(d.contentBuffer, d.triggerSignal)

		// Yield content before trigger signal
		contentBeforeTrigger := d.contentBuffer[:triggerPos]
		d.lastYieldIndex = len(contentBeforeTrigger)

		// Switch to tool parsing state
		d.state = "tool_parsing"

		return false, contentBeforeTrigger[d.lastYieldIndex:]
	}

	// No trigger signal yet, yield new content
	return d.yieldNewContent()
}

// handleToolParsingState processes chunks in tool parsing state
func (d *StreamingDetector) handleToolParsingState() (isToolCall bool, contentToYield string) {
	// Check if we have complete function_calls block
	if strings.Contains(d.contentBuffer, "</function_calls>") {
		// Tool call complete
		d.state = "detecting" // Reset for potential next tool call
		return true, ""
	}

	// Still accumulating tool call content, don't yield anything
	return false, ""
}

// yieldNewContent yields content that hasn't been yielded yet
func (d *StreamingDetector) yieldNewContent() (isToolCall bool, contentToYield string) {
	if len(d.contentBuffer) > d.lastYieldIndex {
		// Calculate safe yield position (avoid cutting in middle of potential trigger signal)
		safeYieldLength := d.calculateSafeYieldLength()

		if safeYieldLength > d.lastYieldIndex {
			content := d.contentBuffer[d.lastYieldIndex:safeYieldLength]
			d.lastYieldIndex = safeYieldLength
			return false, content
		}
	}

	return false, ""
}

// calculateSafeYieldLength calculates how much content can be safely yielded
// without potentially cutting a trigger signal in half
func (d *StreamingDetector) calculateSafeYieldLength() int {
	bufferLen := len(d.contentBuffer)
	triggerLen := len(d.triggerSignal)

	// Reserve space for potential trigger signal
	if bufferLen > triggerLen {
		return bufferLen - triggerLen
	}

	return d.lastYieldIndex
}

// GetAccumulatedContent returns all accumulated content
func (d *StreamingDetector) GetAccumulatedContent() string {
	return d.contentBuffer
}

// IsToolCallComplete checks if a complete tool call has been detected
func (d *StreamingDetector) IsToolCallComplete() bool {
	return d.state == "tool_parsing" &&
		strings.Contains(d.contentBuffer, d.triggerSignal) &&
		strings.Contains(d.contentBuffer, "</function_calls>")
}

// Reset resets the detector state
func (d *StreamingDetector) Reset() {
	d.contentBuffer = ""
	d.state = "detecting"
	d.inThinkBlock = false
	d.thinkDepth = 0
	d.lastYieldIndex = 0
}
