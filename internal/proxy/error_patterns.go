package proxy

import (
	"regexp"
	"strings"
)

// ErrorPattern 定义错误模式匹配规则
type ErrorPattern struct {
	Pattern         string   `json:"pattern"`          // 正则表达式模式
	Keywords        []string `json:"keywords"`         // 关键词列表
	StatusCodes     []int    `json:"status_codes"`     // 匹配的状态码
	EndpointTypes   []string `json:"endpoint_types"`   // 适用的端点类型
	ErrorMessage    string   `json:"error_message"`    // 错误描述
	RetryAction     string   `json:"retry_action"`     // 重试动作: retry, skip, blacklist
	Priority        int      `json:"priority"`         // 优先级，数字越大优先级越高
	MaxRetries      int      `json:"max_retries"`      // 最大重试次数
	RetryDelay      string   `json:"retry_delay"`      // 重试延迟
	CaseSensitive   bool     `json:"case_sensitive"`   // 是否大小写敏感
}

// ErrorPatternMatcher 错误模式匹配器
type ErrorPatternMatcher struct {
	patterns []ErrorPattern
}

// NewErrorPatternMatcher 创建错误模式匹配器
func NewErrorPatternMatcher() *ErrorPatternMatcher {
	matcher := &ErrorPatternMatcher{
		patterns: getDefaultErrorPatterns(),
	}
	return matcher
}

// MatchError 匹配错误模式
func (m *ErrorPatternMatcher) MatchError(statusCode int, errorMsg string, responseBody string, endpointType string) *ErrorPattern {
	var bestMatch *ErrorPattern
	highestPriority := -1

	for i := range m.patterns {
		pattern := &m.patterns[i]
		
		// 检查状态码匹配
		if len(pattern.StatusCodes) > 0 {
			found := false
			for _, code := range pattern.StatusCodes {
				if code == statusCode {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 检查端点类型匹配
		if len(pattern.EndpointTypes) > 0 {
			found := false
			for _, epType := range pattern.EndpointTypes {
				if epType == endpointType {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 检查关键词匹配
		if len(pattern.Keywords) > 0 {
			found := false
			searchText := errorMsg + " " + responseBody
			if !pattern.CaseSensitive {
				searchText = strings.ToLower(searchText)
			}
			
			for _, keyword := range pattern.Keywords {
				keywordText := keyword
				if !pattern.CaseSensitive {
					keywordText = strings.ToLower(keyword)
				}
				if strings.Contains(searchText, keywordText) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 检查正则表达式匹配
		if pattern.Pattern != "" {
			regex, err := regexp.Compile(pattern.Pattern)
			if err != nil {
				continue
			}
			searchText := errorMsg + " " + responseBody
			if !pattern.CaseSensitive {
				searchText = strings.ToLower(searchText)
			}
			if !regex.MatchString(searchText) {
				continue
			}
		}

		// 如果匹配成功，检查优先级
		if pattern.Priority > highestPriority {
			highestPriority = pattern.Priority
			bestMatch = pattern
		}
	}

	return bestMatch
}

// getDefaultErrorPatterns 获取默认错误模式
func getDefaultErrorPatterns() []ErrorPattern {
	return []ErrorPattern{
		// Ollama 特定错误
		{
			Pattern:         `\[\] is too short.*messages`,
			Keywords:        []string{"[] is too short", "messages"},
			StatusCodes:     []int{400},
			EndpointTypes:   []string{"openai", "ollama"},
			ErrorMessage:    "Ollama: Messages array is empty or null",
			RetryAction:     "skip",
			Priority:        100,
			MaxRetries:      0,
			RetryDelay:      "0s",
			CaseSensitive:   false,
		},
		{
			Pattern:         `messages.*is too short`,
			Keywords:        []string{"is too short", "messages"},
			StatusCodes:     []int{400},
			EndpointTypes:   []string{"openai", "ollama"},
			ErrorMessage:    "Ollama: Messages array too short",
			RetryAction:     "skip",
			Priority:        95,
			MaxRetries:      0,
			RetryDelay:      "0s",
			CaseSensitive:   false,
		},
		
		// 网关错误
		{
			Pattern:         ``,
			Keywords:        []string{"Bad gateway", "502"},
			StatusCodes:     []int{502, 503, 504},
			EndpointTypes:   []string{},
			ErrorMessage:    "Gateway error - endpoint temporarily unavailable",
			RetryAction:     "retry",
			Priority:        90,
			MaxRetries:      2,
			RetryDelay:      "2s",
			CaseSensitive:   false,
		},
		
		// 速率限制
		{
			Pattern:         ``,
			Keywords:        []string{"rate limit", "too many requests", "quota exceeded"},
			StatusCodes:     []int{429},
			EndpointTypes:   []string{},
			ErrorMessage:    "Rate limit exceeded",
			RetryAction:     "retry",
			Priority:        80,
			MaxRetries:      1,
			RetryDelay:      "5s",
			CaseSensitive:   false,
		},
		
		// 认证错误
		{
			Pattern:         ``,
			Keywords:        []string{"unauthorized", "authentication", "invalid api key"},
			StatusCodes:     []int{401},
			EndpointTypes:   []string{},
			ErrorMessage:    "Authentication failed",
			RetryAction:     "blacklist",
			Priority:        70,
			MaxRetries:      0,
			RetryDelay:      "0s",
			CaseSensitive:   false,
		},
		
		// 模型不可用
		{
			Pattern:         ``,
			Keywords:        []string{"model not found", "model not available", "invalid model"},
			StatusCodes:     []int{400, 404},
			EndpointTypes:   []string{},
			ErrorMessage:    "Model not available on this endpoint",
			RetryAction:     "skip",
			Priority:        60,
			MaxRetries:      0,
			RetryDelay:      "0s",
			CaseSensitive:   false,
		},
		
		// 超时错误
		{
			Pattern:         ``,
			Keywords:        []string{"timeout", "deadline exceeded", "context deadline"},
			StatusCodes:     []int{408, 504},
			EndpointTypes:   []string{},
			ErrorMessage:    "Request timeout",
			RetryAction:     "retry",
			Priority:        50,
			MaxRetries:      1,
			RetryDelay:      "3s",
			CaseSensitive:   false,
		},
		
		// 服务器内部错误
		{
			Pattern:         ``,
			Keywords:        []string{"internal server error", "server error", "something went wrong"},
			StatusCodes:     []int{500, 502, 503},
			EndpointTypes:   []string{},
			ErrorMessage:    "Server internal error",
			RetryAction:     "retry",
			Priority:        40,
			MaxRetries:      2,
			RetryDelay:      "1s",
			CaseSensitive:   false,
		},
	}
}

// RetryDecision 重试决策
type RetryDecision struct {
	ShouldRetry     bool   `json:"should_retry"`
	Action          string `json:"action"`           // retry, skip, blacklist
	MaxRetries      int    `json:"max_retries"`      // 最大重试次数
	RetryDelay      string `json:"retry_delay"`      // 重试延迟
	Reason          string `json:"reason"`           // 决策原因
	MatchedPattern  string `json:"matched_pattern"`  // 匹配的模式
}

// MakeRetryDecision 根据错误信息做出重试决策
func (m *ErrorPatternMatcher) MakeRetryDecision(statusCode int, errorMsg string, responseBody string, endpointType string, attemptNumber int) *RetryDecision {
	pattern := m.MatchError(statusCode, errorMsg, responseBody, endpointType)
	
	if pattern == nil {
		// 没有匹配到已知模式，使用默认策略
		return &RetryDecision{
			ShouldRetry: false,
			Action:      "skip",
			MaxRetries:  0,
			RetryDelay:  "0s",
			Reason:      "Unknown error pattern",
		}
	}

	// 检查是否已达到最大重试次数
	if attemptNumber > pattern.MaxRetries {
		return &RetryDecision{
			ShouldRetry:    false,
			Action:         "skip",
			MaxRetries:     pattern.MaxRetries,
			RetryDelay:     pattern.RetryDelay,
			Reason:         "Maximum retries exceeded",
			MatchedPattern: pattern.ErrorMessage,
		}
	}

	return &RetryDecision{
		ShouldRetry:    true,
		Action:         pattern.RetryAction,
		MaxRetries:     pattern.MaxRetries,
		RetryDelay:     pattern.RetryDelay,
		Reason:         pattern.ErrorMessage,
		MatchedPattern: pattern.ErrorMessage,
	}
}