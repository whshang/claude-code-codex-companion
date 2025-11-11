package utils

import (
	"encoding/json"
	"strings"
	
	commonjson "claude-code-codex-companion/internal/common/json"
)

// ExtractStringField extracts a string field from JSON data
// 兼容性函数，委托给新的json工具包
func ExtractStringField(data []byte, field string) (string, error) {
	return commonjson.ExtractField[string](data, field)
}

// ExtractNestedStringField extracts a nested string field from JSON data
// path should be like ["metadata", "user_id"]
// 兼容性函数，委托给新的json工具包
func ExtractNestedStringField(data []byte, path []string) (string, error) {
	return commonjson.ExtractNestedField[string](data, path)
}

// ExtractModelFromRequestBody extracts the model name from request body JSON
func ExtractModelFromRequestBody(body string) string {
	if body == "" {
		return ""
	}
	
	model, _ := ExtractStringField([]byte(body), "model")
	return model
}


// TruncateBody truncates body content to specified length
func TruncateBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "... [truncated]"
}

// ThinkingInfo contains extracted thinking mode information
type ThinkingInfo struct {
	Enabled     bool `json:"enabled"`
	BudgetTokens int  `json:"budget_tokens"`
}

// ExtractThinkingInfo extracts thinking mode information from request body
func ExtractThinkingInfo(body string) (*ThinkingInfo, error) {
	if body == "" {
		return nil, nil
	}
	
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}
	
	thinkingField, exists := parsed["thinking"]
	if !exists {
		return nil, nil
	}
	
	thinkingMap, ok := thinkingField.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	
	info := &ThinkingInfo{}
	
	// Check if thinking is enabled
	if typeValue, ok := thinkingMap["type"].(string); ok && typeValue == "enabled" {
		info.Enabled = true
	}
	
	// Extract budget tokens
	if budgetValue, ok := thinkingMap["budget_tokens"]; ok {
		if budgetFloat, ok := budgetValue.(float64); ok {
			info.BudgetTokens = int(budgetFloat)
		}
	}
	
	return info, nil
}

// ExtractSessionIDFromUserID extracts session ID from user_id field
// Format: user_xxx_account__session_<uuid>
// Returns the UUID part after "_account__session_", or empty string if not found
func ExtractSessionIDFromUserID(userID string) string {
	if userID == "" {
		return ""
	}
	
	// Look for the pattern "_account__session_"
	sessionPrefix := "_account__session_"
	index := strings.LastIndex(userID, sessionPrefix)
	if index == -1 {
		return ""
	}
	
	// Extract everything after "_account__session_"
	sessionID := userID[index+len(sessionPrefix):]
	if sessionID == "" {
		return ""
	}
	
	return sessionID
}

// ExtractSessionIDFromRequestBody extracts session ID from request body JSON
// by first extracting metadata.user_id and then extracting session ID from it
func ExtractSessionIDFromRequestBody(body string) string {
	if body == "" {
		return ""
	}
	
	// Extract user_id from metadata
	userID, err := ExtractNestedStringField([]byte(body), []string{"metadata", "user_id"})
	if err != nil || userID == "" {
		return ""
	}
	
	// Extract session ID from user_id
	return ExtractSessionIDFromUserID(userID)
}