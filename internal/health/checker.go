package health

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/modelrewrite"
)

type Checker struct {
	extractor       *RequestExtractor
	healthTimeouts  config.HealthCheckTimeoutConfig
	modelRewriter   *modelrewrite.Rewriter
}

func NewChecker(healthTimeouts config.HealthCheckTimeoutConfig, modelRewriter *modelrewrite.Rewriter) *Checker {
	return &Checker{
		extractor:      NewRequestExtractor(),
		healthTimeouts: healthTimeouts,
		modelRewriter:  modelRewriter,
	}
}

func (c *Checker) GetExtractor() *RequestExtractor {
	return c.extractor
}

func (c *Checker) CheckEndpoint(ep *endpoint.Endpoint) error {
	requestInfo := c.extractor.GetRequestInfo()
	
	// 构造健康检查请求
	healthCheckRequest := map[string]interface{}{
		"model":       requestInfo.Model,
		"max_tokens":  config.Default.HealthCheck.MaxTokens,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "hello",
			},
		},
		"system": []map[string]interface{}{
			{
				"type": "text",
				"text": "Analyze if this message indicates a new conversation topic. If it does, extract a 2-3 word title that captures the new topic. Format your response as a JSON object with two fields: 'isNewTopic' (boolean) and 'title' (string, or null if isNewTopic is false). Only include these fields, no other text.",
			},
		},
		"temperature": config.Default.HealthCheck.Temperature,
		"metadata": map[string]interface{}{
			"user_id": requestInfo.UserID,
		},
		"stream": config.Default.HealthCheck.StreamMode,
	}

	// 将请求序列化为JSON
	requestBody, err := json.Marshal(healthCheckRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal health check request: %v", err)
	}

	// 获取目标URL（稍后可能会被格式转换修改）
	targetURL := ep.GetFullURL("/messages")
	
	// 创建临时HTTP请求用于模型重写处理
	tempReq, err := http.NewRequest("POST", targetURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create temporary request for model rewrite: %v", err)
	}

	// 复制从实际请求中提取的头部（用于模型重写）
	for key, value := range requestInfo.Headers {
		tempReq.Header.Set(key, value)
	}

	// 应用模型重写（如果配置了）
	// 健康检查时没有真实客户端类型，使用空字符串避免触发隐式重写
	_, _, err = c.modelRewriter.RewriteRequestWithTags(tempReq, ep.ModelRewrite, ep.Tags, "")
	if err != nil {
		return fmt.Errorf("model rewrite failed during health check: %v", err)
	}

	// 如果进行了模型重写，获取重写后的请求体
	finalRequestBody, err := io.ReadAll(tempReq.Body)
	if err != nil {
		return fmt.Errorf("failed to read rewritten request body: %v", err)
	}

	// 格式转换（在模型重写之后）
	// 注意：健康检查的请求已经是Anthropic格式，只在需要时转换为OpenAI格式
	shouldConvert := ep.EndpointType == "openai" && ep.URLOpenAI != "" && ep.URLAnthropic == ""
	if shouldConvert {
		// 将Anthropic格式请求转换为OpenAI格式
		reqConverter := conversion.NewRequestConverter(nil)
		endpointInfo := &conversion.EndpointInfo{
			Type:               ep.EndpointType,
			MaxTokensFieldName: ep.MaxTokensFieldName,
		}

		convertedBody, _, err := reqConverter.Convert(finalRequestBody, endpointInfo)
		if err != nil {
			return fmt.Errorf("request format conversion failed during health check: %v", err)
		}
		finalRequestBody = convertedBody

		// 对于OpenAI端点，需要更新目标URL
		targetURL = ep.GetFullURL("/chat/completions")
	}

	// 构造最终的HTTP请求
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(finalRequestBody))
	if err != nil {
		return fmt.Errorf("failed to create final health check request: %v", err)
	}

	// 复制从实际请求中提取的头部（包含默认值）
	for key, value := range requestInfo.Headers {
		req.Header.Set(key, value)
	}

	// 单独设置认证头部（不包含在默认headers中）
	if ep.AuthType == "api_key" {
		req.Header.Set("x-api-key", ep.AuthValue)
	} else {
		authHeader, err := ep.GetAuthHeader()
		if err != nil {
			return fmt.Errorf("failed to get auth header: %v", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

	// 执行请求 - 使用端点特定的HTTP客户端
	client, err := ep.CreateHealthClient(c.healthTimeouts)
	if err != nil {
		return fmt.Errorf("failed to create health client for endpoint: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %v", err)
	}
	defer resp.Body.Close()
	
	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 读取响应体验证是否为有效流式响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read health check response: %v", err)
	}

	// 简单验证：检查是否包含SSE格式的流式响应
	if !bytes.Contains(body, []byte("event:")) && !bytes.Contains(body, []byte("data:")) {
		// 如果不是流式响应，检查是否为有效的JSON响应
		var jsonResp map[string]interface{}
		if err := json.Unmarshal(body, &jsonResp); err != nil {
			return fmt.Errorf("health check response is neither valid SSE nor JSON: %v", err)
		}
		
		// 检查是否包含Anthropic响应的基本字段
		if _, hasContent := jsonResp["content"]; !hasContent {
			if _, hasError := jsonResp["error"]; !hasError {
				return fmt.Errorf("health check response missing required fields")
			}
		}
	}

	return nil
}