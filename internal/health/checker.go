package health

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/modelrewrite"
)

type Checker struct {
	extractor      *RequestExtractor
	healthTimeouts config.HealthCheckTimeoutConfig
	modelRewriter  *modelrewrite.Rewriter
	defaultModel   string
}

type HealthCheckResult struct {
	URL             string
	Method          string
	StatusCode      int
	Duration        time.Duration
	RequestBody     []byte
	ResponseBody    []byte
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
	Model           string
}

func NewChecker(healthTimeouts config.HealthCheckTimeoutConfig, modelRewriter *modelrewrite.Rewriter, defaultModel string) *Checker {
	return &Checker{
		extractor:      NewRequestExtractor(),
		healthTimeouts: healthTimeouts,
		modelRewriter:  modelRewriter,
		defaultModel:   defaultModel,
	}
}

func (c *Checker) GetExtractor() *RequestExtractor {
	return c.extractor
}

func (c *Checker) CheckEndpointWithDetails(ep *endpoint.Endpoint) (*HealthCheckResult, error) {
	requestInfo := c.extractor.GetRequestInfo()

	// 实现模型选择优先级链：测试模型 -> 重写模型1 -> 重写模型2 -> ... -> 默认模型
	selectedModel := c.selectHealthCheckModel(ep, c.defaultModel)

	result := &HealthCheckResult{
		Method:          http.MethodPost,
		RequestHeaders:  make(map[string]string),
		ResponseHeaders: make(map[string]string),
		Model:           selectedModel,
	}
	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	healthCheckRequest := map[string]interface{}{
		"model":      selectedModel,
		"max_tokens": config.Default.HealthCheck.MaxTokens,
		"messages": []map[string]interface{}{{
			"role":    "user",
			"content": "你好",
		}},
		"temperature": config.Default.HealthCheck.Temperature,
	}

	requestBody, err := json.Marshal(healthCheckRequest)
	if err != nil {
		return result, fmt.Errorf("failed to marshal health check request: %v", err)
	}

	targetURL := ep.GetFullURL("/v1/messages")
	result.URL = targetURL

	tempReq, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		return result, fmt.Errorf("failed to create temporary request for model rewrite: %v", err)
	}

	for key, value := range requestInfo.Headers {
		tempReq.Header.Set(key, value)
	}

	_, _, err = c.modelRewriter.RewriteRequestWithTags(tempReq, ep.ModelRewrite, ep.Tags, "")
	if err != nil {
		return result, fmt.Errorf("model rewrite failed during health check: %v", err)
	}

	finalRequestBody, err := io.ReadAll(tempReq.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read rewritten request body: %v", err)
	}
	result.RequestBody = finalRequestBody

	// Update result.Model to reflect the actual model being sent after rewrite
	var rewrittenRequest map[string]interface{}
	if err := json.Unmarshal(finalRequestBody, &rewrittenRequest); err == nil {
		if rewrittenModel, ok := rewrittenRequest["model"].(string); ok {
			result.Model = rewrittenModel
		}
	}

	shouldConvert := ep.EndpointType == "openai" && ep.URLOpenAI != "" && ep.URLAnthropic == ""
	if shouldConvert {
		reqConverter := conversion.NewRequestConverter(nil)
		endpointInfo := &conversion.EndpointInfo{
			Type:               ep.EndpointType,
			MaxTokensFieldName: ep.MaxTokensFieldName,
		}

		convertedBody, _, err := reqConverter.Convert(finalRequestBody, endpointInfo)
		if err != nil {
			return result, fmt.Errorf("request format conversion failed during health check: %v", err)
		}
		finalRequestBody = convertedBody
		result.RequestBody = finalRequestBody
		targetURL = ep.GetFullURL("/chat/completions")
		result.URL = targetURL

		// Update result.Model after format conversion as well
		var convertedRequest map[string]interface{}
		if err := json.Unmarshal(finalRequestBody, &convertedRequest); err == nil {
			if convertedModel, ok := convertedRequest["model"].(string); ok {
				result.Model = convertedModel
			}
		}
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(finalRequestBody))
	if err != nil {
		return result, fmt.Errorf("failed to create final health check request: %v", err)
	}

	for key, value := range requestInfo.Headers {
		req.Header.Set(key, value)
	}

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/event-stream")
	}

	if ep.AuthType == "api_key" {
		req.Header.Set("x-api-key", ep.AuthValue)
	} else {
		authHeader, err := ep.GetAuthHeader()
		if err != nil {
			return result, fmt.Errorf("failed to get auth header: %v", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

	for key, values := range req.Header {
		if len(values) > 0 {
			result.RequestHeaders[key] = values[len(values)-1]
		}
	}

	client, err := ep.CreateHealthClient(c.healthTimeouts)
	if err != nil {
		return result, fmt.Errorf("failed to create health client for endpoint: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("health check request failed: %v", err)
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode

	for key, values := range resp.Header {
		if len(values) > 0 {
			result.ResponseHeaders[key] = values[len(values)-1]
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read health check response: %v", err)
	}
	result.ResponseBody = body

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	if !bytes.Contains(body, []byte("event:")) && !bytes.Contains(body, []byte("data:")) {
		var jsonResp map[string]interface{}
		if err := json.Unmarshal(body, &jsonResp); err != nil {
			return result, fmt.Errorf("health check response is neither valid SSE nor JSON: %v", err)
		}

		// 验证响应格式：支持 Anthropic 和 OpenAI 两种格式
		// Anthropic: {"content": [...]}
		// OpenAI: {"choices": [{"message": {"content": "..."}}]} 或 {"choices": [{"delta": {"content": "..."}}]}
		hasValidContent := false
		
		// 检查 Anthropic 格式
		if _, ok := jsonResp["content"]; ok {
			hasValidContent = true
		}
		
		// 检查 OpenAI 格式
		if choices, ok := jsonResp["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				// 检查 message.content (非流式)
				if message, ok := choice["message"].(map[string]interface{}); ok {
					if _, ok := message["content"]; ok {
						hasValidContent = true
					}
				}
				// 检查 delta (流式)
				if _, ok := choice["delta"].(map[string]interface{}); ok {
					hasValidContent = true  // delta 存在即认为有效
				}
			}
		}
		
		// 检查错误响应
		if !hasValidContent {
			if _, hasError := jsonResp["error"]; !hasError {
				return result, fmt.Errorf("health check response missing required fields (expected content or choices)")
			}
		}
	}

	return result, nil
}

func (c *Checker) CheckEndpoint(ep *endpoint.Endpoint) error {
	_, err := c.CheckEndpointWithDetails(ep)
	return err
}

// selectHealthCheckModel 实现模型选择优先级链：TargetModel -> 重写模型 -> 默认模型
func (c *Checker) selectHealthCheckModel(ep *endpoint.Endpoint, defaultModel string) string {
	// 优先级1: 使用模型重写配置中的 TargetModel（对应数据库中的 target_model 字段）
	if ep.ModelRewrite != nil && ep.ModelRewrite.TargetModel != "" {
		return ep.ModelRewrite.TargetModel
	}

	// 优先级2: 应用模型重写规则到默认模型
	if ep.ModelRewrite != nil && ep.ModelRewrite.Enabled && len(ep.ModelRewrite.Rules) > 0 {
		for _, rule := range ep.ModelRewrite.Rules {
			// 检查默认模型是否匹配重写规则
			if matched, err := filepath.Match(rule.SourcePattern, defaultModel); err == nil && matched {
				return rule.TargetModel
			}
		}
	}

	// 优先级3: 使用默认模型
	return defaultModel
}
