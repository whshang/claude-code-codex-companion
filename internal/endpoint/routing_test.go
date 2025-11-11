package endpoint

import (
	"testing"
)

// TestHasURLForFormat 测试端点URL格式检测
func TestHasURLForFormat(t *testing.T) {
	tests := []struct {
		name           string
		urlAnthropic   string
		urlOpenAI      string
		format         string
		expectedResult bool
	}{
		{
			name:           "Has Anthropic URL for anthropic format",
			urlAnthropic:   "https://api.anthropic.com",
			urlOpenAI:      "",
			format:         "anthropic",
			expectedResult: true,
		},
		{
			name:           "Has OpenAI URL for openai format",
			urlAnthropic:   "",
			urlOpenAI:      "https://api.openai.com",
			format:         "openai",
			expectedResult: true,
		},
		{
			name:           "No Anthropic URL for anthropic format",
			urlAnthropic:   "",
			urlOpenAI:      "https://api.openai.com",
			format:         "anthropic",
			expectedResult: false,
		},
		{
			name:           "No OpenAI URL for openai format",
			urlAnthropic:   "https://api.anthropic.com",
			urlOpenAI:      "",
			format:         "openai",
			expectedResult: false,
		},
		{
			name:           "Has both URLs for anthropic format",
			urlAnthropic:   "https://api.anthropic.com",
			urlOpenAI:      "https://api.openai.com",
			format:         "anthropic",
			expectedResult: true,
		},
		{
			name:           "Has both URLs for openai format",
			urlAnthropic:   "https://api.anthropic.com",
			urlOpenAI:      "https://api.openai.com",
			format:         "openai",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := &Endpoint{
				Name:         "test-endpoint",
				URLAnthropic: tt.urlAnthropic,
				URLOpenAI:    tt.urlOpenAI,
				EndpointType: "openai", // 默认类型
			}

			result := ep.HasURLForFormat(tt.format)
			if result != tt.expectedResult {
				t.Errorf("HasURLForFormat() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

// TestGetFullURLWithFormat 测试严格URL返回逻辑
func TestGetFullURLWithFormat(t *testing.T) {
	tests := []struct {
		name         string
		urlAnthropic string
		urlOpenAI    string
		path         string
		format       string
		expectedURL  string // 空字符串表示应该返回空
	}{
		{
			name:         "Anthropic format with Anthropic URL",
			urlAnthropic: "https://api.anthropic.com",
			urlOpenAI:    "",
			path:         "/v1/messages",
			format:       "anthropic",
			expectedURL:  "https://api.anthropic.com/v1/messages",
		},
		{
			name:         "OpenAI format with OpenAI URL",
			urlAnthropic: "",
			urlOpenAI:    "https://api.openai.com",
			path:         "/v1/responses",
			format:       "openai",
			expectedURL:  "https://api.openai.com/v1/responses",
		},
		{
			name:         "OpenAI responses path auto adds v1",
			urlAnthropic: "",
			urlOpenAI:    "https://api.openai.com",
			path:         "/responses",
			format:       "openai",
			expectedURL:  "https://api.openai.com/v1/responses",
		},
		{
			name:         "OpenAI responses with trailing slash base",
			urlAnthropic: "",
			urlOpenAI:    "https://api.openai.com/",
			path:         "/responses",
			format:       "openai",
			expectedURL:  "https://api.openai.com/v1/responses",
		},
		{
			name:         "OpenAI chat completions with trailing slash base",
			urlAnthropic: "",
			urlOpenAI:    "https://api.openai.com/",
			path:         "/chat/completions",
			format:       "openai",
			expectedURL:  "https://api.openai.com/v1/chat/completions",
		},
		{
			name:         "OpenAI auto format fallback adds v1 for responses",
			urlAnthropic: "",
			urlOpenAI:    "https://api.openai.com",
			path:         "/responses",
			format:       "",
			expectedURL:  "https://api.openai.com/v1/responses",
		},
		{
			name:         "Anthropic format without Anthropic URL should return empty",
			urlAnthropic: "",
			urlOpenAI:    "https://api.openai.com",
			path:         "/v1/messages",
			format:       "anthropic",
			expectedURL:  "", // 应该返回空，不回退到 OpenAI
		},
		{
			name:         "OpenAI format without OpenAI URL should return empty",
			urlAnthropic: "https://api.anthropic.com",
			urlOpenAI:    "",
			path:         "/v1/responses",
			format:       "openai",
			expectedURL:  "", // 应该返回空，不回退到 Anthropic
		},
		{
			name:         "Empty format with only Anthropic URL",
			urlAnthropic: "https://api.anthropic.com",
			urlOpenAI:    "",
			path:         "/v1/messages",
			format:       "", // 空格式应该优先使用 Anthropic
			expectedURL:  "https://api.anthropic.com/v1/messages",
		},
		{
			name:         "Empty format with only OpenAI URL",
			urlAnthropic: "",
			urlOpenAI:    "https://api.openai.com",
			path:         "/v1/responses",
			format:       "", // 空格式应该使用 OpenAI
			expectedURL:  "https://api.openai.com/v1/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := &Endpoint{
				Name:         "test-endpoint",
				URLAnthropic: tt.urlAnthropic,
				URLOpenAI:    tt.urlOpenAI,
				EndpointType: "openai",
			}

			result := ep.GetFullURLWithFormat(tt.path, tt.format)
			if result != tt.expectedURL {
				t.Errorf("GetFullURLWithFormat() = %q, want %q", result, tt.expectedURL)
			}
		})
	}
}
