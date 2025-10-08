package toolcall

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// TriggerSignalGenerator generates random trigger signals for tool calling
type TriggerSignalGenerator struct{}

// NewTriggerSignalGenerator creates a new trigger signal generator
func NewTriggerSignalGenerator() *TriggerSignalGenerator {
	return &TriggerSignalGenerator{}
}

// Generate creates a random self-closing trigger signal
// Format: <Function_XXXX_Start/> where XXXX is 4 random alphanumeric characters
func (g *TriggerSignalGenerator) Generate() string {
	// Generate 3 random bytes (will produce 4 base64 characters)
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based generation if random fails
		return fmt.Sprintf("<Function_%d_Start/>", getCurrentTimestamp())
	}

	// Encode to base64 and take first 4 alphanumeric characters
	encoded := base64.RawURLEncoding.EncodeToString(bytes)
	if len(encoded) > 4 {
		encoded = encoded[:4]
	}

	// Replace non-alphanumeric characters
	randomStr := sanitizeForXML(encoded)

	return fmt.Sprintf("<Function_%s_Start/>", randomStr)
}

// sanitizeForXML ensures the string only contains alphanumeric characters
func sanitizeForXML(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		}
	}
	// Ensure we have at least 4 characters
	for len(result) < 4 {
		result = append(result, 'X')
	}
	return string(result)
}

// getCurrentTimestamp returns current Unix timestamp modulo 10000
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano() % 10000
}
