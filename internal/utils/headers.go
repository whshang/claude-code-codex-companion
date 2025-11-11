package utils

import (
	"net/http"
	"strings"
)


// ExtractRequestHeaders extracts relevant headers from HTTP request, excluding sensitive ones
func ExtractRequestHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	
	// Headers to exclude from extraction
	excludeHeaders := map[string]bool{
		"authorization":   true,
		"x-api-key":      true,
		"content-length": true,
		"host":           true,
	}

	for key, values := range headers {
		lowKey := strings.ToLower(key)
		if !excludeHeaders[lowKey] && len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result
}

// HeadersToMap converts http.Header to map[string]string (takes first value if multiple)
func HeadersToMap(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}