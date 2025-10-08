package utils

import (
	"encoding/json"
	"math"
	"unicode/utf8"
)

// EstimateTokenCount 对请求体做粗略 token 估算，当上游不支持 /count_tokens 时作为兜底。
// 算法：提取所有字符串字段，按 4 个字符近似 1 个 token，并加上轻微的结构成本。
func EstimateTokenCount(body []byte) int {
	if len(body) == 0 {
		return 0
	}

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		// 若非 JSON，则按字节长度估算
		return approximateFromString(string(body))
	}

	tokens := estimateFromValue(payload)
	if tokens <= 0 {
		tokens = approximateFromString(string(body))
	}

	if tokens < 0 {
		tokens = 0
	}
	return tokens
}

func estimateFromValue(v interface{}) int {
	switch val := v.(type) {
	case string:
		return approximateFromString(val)
	case []interface{}:
		total := 0
		for _, item := range val {
			total += estimateFromValue(item)
		}
		return total
	case map[string]interface{}:
		total := 0
		for _, item := range val {
			total += estimateFromValue(item)
		}
		return total
	default:
		return 0
	}
}

func approximateFromString(s string) int {
	trimmed := s
	if trimmed == "" {
		return 0
	}
	runeCount := utf8.RuneCountInString(trimmed)
	if runeCount == 0 {
		return 0
	}

	est := int(math.Ceil(float64(runeCount) / 4.0))
	if est < 1 {
		est = 1
	}
	return est
}
