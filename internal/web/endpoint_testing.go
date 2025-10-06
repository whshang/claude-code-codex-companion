package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/logger"
	"claude-code-codex-companion/internal/modelrewrite"
)

// EndpointTestResult 单个格式测试结果
type EndpointTestResult struct {
	Format             string              `json:"format"`                        // "anthropic" 或 "openai"
	Success            bool                `json:"success"`                       // 是否成功
	ResponseTime       int64               `json:"response_time"`                 // 响应时间(毫秒)
	StatusCode         int                 `json:"status_code"`                   // HTTP状态码
	Error              string              `json:"error"`                         // 错误信息
	URL                string              `json:"url"`                           // 测试的URL
	PerformanceMetrics *PerformanceMetrics `json:"performance_metrics,omitempty"` // 详细性能指标
	ModelRewrite       *ModelRewriteInfo   `json:"model_rewrite,omitempty"`       // 模型重写信息
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	DNSLookupTime       int64 `json:"dns_lookup_time,omitempty"`       // DNS查找时间(毫秒)
	TCPConnectTime      int64 `json:"tcp_connect_time,omitempty"`      // TCP连接时间(毫秒)
	TLSHandshakeTime    int64 `json:"tls_handshake_time,omitempty"`    // TLS握手时间(毫秒)
	FirstByteTime       int64 `json:"first_byte_time,omitempty"`       // 首字节时间(毫秒)
	ContentDownloadTime int64 `json:"content_download_time,omitempty"` // 内容下载时间(毫秒)
	TotalSize           int64 `json:"total_size"`                      // 响应大小(字节)
}

// ModelRewriteInfo 模型重写信息
type ModelRewriteInfo struct {
	OriginalModel  string `json:"original_model,omitempty"`  // 原始模型
	RewrittenModel string `json:"rewritten_model,omitempty"` // 重写后模型
	RuleApplied    bool   `json:"rule_applied"`              // 是否应用了重写规则
}

// BatchTestResult 批量测试结果
type BatchTestResult struct {
	EndpointName string                `json:"endpoint_name"`
	Results      []*EndpointTestResult `json:"results"`
	TotalTime    int64                 `json:"total_time"` // 总耗时(毫秒)
}

// testEndpointFormat 测试单个端点的单个格式
func (s *AdminServer) testEndpointFormat(ep *endpoint.Endpoint, format string, timeout time.Duration) *EndpointTestResult {
	return s.testEndpointFormatWithStream(ep, format, timeout, false)
}

// testEndpointFormatWithStream 测试单个端点的单个格式，支持流式测试
func (s *AdminServer) testEndpointFormatWithStream(ep *endpoint.Endpoint, format string, timeout time.Duration, stream bool) *EndpointTestResult {
	result := &EndpointTestResult{
		Format: format,
	}

	// 构建测试URL和请求
	var testURL string
	var requestBody []byte
	var err error
	var originalModel string

	if format == "anthropic" {
		// 使用Anthropic格式测试
		if ep.URLAnthropic == "" {
			result.Error = "endpoint does not have Anthropic URL configured"
			return result
		}
		testURL = ep.URLAnthropic + "/v1/messages"
		result.URL = testURL

		// 根据端点配置选择测试模型
		testModel := s.selectTestModel(ep, "anthropic")
		originalModel = testModel

		// Anthropic格式请求体
		reqBody := map[string]interface{}{
			"model":      testModel,
			"max_tokens": 10,
			"stream":     stream, // 支持流式测试
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hi",
				},
			},
		}
		requestBody, err = json.Marshal(reqBody)
		if err != nil {
			result.Error = fmt.Sprintf("failed to marshal request: %v", err)
			return result
		}
	} else if format == "openai" {
		// 使用OpenAI格式测试
		if ep.URLOpenAI == "" {
			result.Error = "endpoint does not have OpenAI URL configured"
			return result
		}

		// 根据端点偏好选择API格式
		openaiPath := s.selectOpenAIPath(ep)
		testURL = ep.URLOpenAI + openaiPath
		result.URL = testURL

		// 根据端点配置选择测试模型
		testModel := s.selectTestModel(ep, "openai")
		originalModel = testModel

		// OpenAI格式请求体
		reqBody := map[string]interface{}{
			"model":      testModel,
			"max_tokens": 10,
			"stream":     stream, // 支持流式测试
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hi",
				},
			},
		}
		requestBody, err = json.Marshal(reqBody)
		if err != nil {
			result.Error = fmt.Sprintf("failed to marshal request: %v", err)
			return result
		}
	} else {
		result.Error = "invalid format: " + format
		return result
	}

	// 创建HTTP请求
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", testURL, bytes.NewReader(requestBody))
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		return result
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if format == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	// 添加认证头
	authValue, err := ep.GetAuthHeader()
	if err != nil {
		result.Error = fmt.Sprintf("failed to get auth header: %v", err)
		return result
	}
	if authValue != "" {
		if format == "anthropic" {
			req.Header.Set("x-api-key", authValue)
		} else {
			req.Header.Set("Authorization", authValue)
		}
	}

	// 应用模型重写（如果配置了）
	var rewrittenModel string
	result.ModelRewrite = &ModelRewriteInfo{
		OriginalModel: originalModel,
		RuleApplied:   false,
	}

	if ep.ModelRewrite != nil && ep.ModelRewrite.Enabled {
		// 创建用于测试的日志器
		testLogger := logger.Logger{}
		if s.logger != nil {
			testLogger = *s.logger
		}
		rewriter := modelrewrite.NewRewriter(testLogger)
		// 获取端点标签用于隐式重写规则
		endpointTags := ep.Tags
		clientType := s.detectClientType(req)

		_, newModel, rewriteErr := rewriter.RewriteRequestWithTags(req, ep.ModelRewrite, endpointTags, clientType)
		if rewriteErr != nil {
			result.Error = fmt.Sprintf("model rewrite failed: %v", rewriteErr)
			return result
		}
		if newModel != "" {
			rewrittenModel = newModel
			result.ModelRewrite.RewrittenModel = newModel
			result.ModelRewrite.RuleApplied = true
			// 重新读取请求体以获取重写后的内容
			if newBody, readErr := io.ReadAll(req.Body); readErr == nil {
				requestBody = newBody
				req.Body = io.NopCloser(bytes.NewReader(newBody))
			}
		}
	}

	// 发送请求并计时
	startTime := time.Now()

	// 创建自定义HTTP客户端以获取详细性能指标
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: false, // 启用连接复用以获得更准确的性能数据
		},
	}

	// 记录请求开始时间用于计算各阶段耗时
	requestStartTime := time.Now()

	resp, err := client.Do(req)
	responseTime := time.Since(startTime).Milliseconds()
	result.ResponseTime = responseTime

	// 初始化性能指标
	result.PerformanceMetrics = &PerformanceMetrics{
		TotalSize: 0,
	}

	// 计算首字节时间（近似值）
	if resp != nil {
		result.PerformanceMetrics.FirstByteTime = time.Since(requestStartTime).Milliseconds()
	}

	if err != nil {
		result.Error = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// 读取响应体并计算内容下载时间
	downloadStartTime := time.Now()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read response: %v", err)
		return result
	}

	// 更新性能指标
	result.PerformanceMetrics.ContentDownloadTime = time.Since(downloadStartTime).Milliseconds()
	result.PerformanceMetrics.TotalSize = int64(len(body))

	// 应用响应模型重写（如果请求时进行了重写）
	if rewrittenModel != "" && originalModel != "" {
		testLogger := logger.Logger{}
		if s.logger != nil {
			testLogger = *s.logger
		}
		rewriter := modelrewrite.NewRewriter(testLogger)
		if restoredBody, restoreErr := rewriter.RewriteResponse(body, originalModel, rewrittenModel); restoreErr == nil {
			body = restoredBody
		}
	}

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 尝试解析错误信息
		var errResp map[string]interface{}
		if json.Unmarshal(body, &errResp) == nil {
			if errorField, ok := errResp["error"]; ok {
				if errorMap, ok := errorField.(map[string]interface{}); ok {
					if msg, ok := errorMap["message"].(string); ok {
						result.Error = msg
					} else {
						result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
					}
				} else {
					result.Error = fmt.Sprintf("HTTP %d: %v", resp.StatusCode, errorField)
				}
			} else {
				result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
			}
		} else {
			result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return result
	}

	// 验证响应格式
	var jsonResp map[string]interface{}
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		result.Error = fmt.Sprintf("invalid JSON response: %v", err)
		return result
	}

	// 检查响应是否符合预期格式
	if stream {
		// 流式响应验证
		if !s.validateStreamResponse(string(body), format) {
			result.Error = fmt.Sprintf("invalid %s stream response format", format)
			return result
		}
	} else {
		// 非流式响应验证
		if format == "anthropic" {
			if _, hasContent := jsonResp["content"]; !hasContent {
				result.Error = "missing 'content' field in Anthropic response"
				return result
			}
		} else if format == "openai" {
			if _, hasChoices := jsonResp["choices"]; !hasChoices {
				result.Error = "missing 'choices' field in OpenAI response"
				return result
			}
		}
	}

	// 测试成功
	result.Success = true
	return result
}

// selectOpenAIPath 根据端点偏好选择OpenAI API路径
func (s *AdminServer) selectOpenAIPath(ep *endpoint.Endpoint) string {
	if ep.OpenAIPreference == "responses" {
		return "/responses"
	} else if ep.OpenAIPreference == "chat_completions" {
		return "/chat/completions"
	}
	
	// 默认优先尝试 responses 格式（Codex新格式）
	return "/responses"
}

// testOpenAIFormatWithRetry 测试OpenAI格式并支持格式重试
func (s *AdminServer) testOpenAIFormatWithRetry(ep *endpoint.Endpoint, timeout time.Duration) *EndpointTestResult {
	// 先尝试首选格式
	preferredPath := s.selectOpenAIPath(ep)
	result := s.testOpenAIPath(ep, preferredPath, timeout)
	
	// 如果失败且是自动模式，尝试另一种格式
	if !result.Success && ep.OpenAIPreference == "auto" {
		var alternativePath string
		if preferredPath == "/responses" {
			alternativePath = "/chat/completions"
		} else {
			alternativePath = "/responses"
		}
		
		alternativeResult := s.testOpenAIPath(ep, alternativePath, timeout)
		if alternativeResult.Success {
			// 记录成功的格式偏好（这里应该更新端点配置）
			s.updateOpenAIPreference(ep, alternativePath)
			return alternativeResult
		}
	}
	
	return result
}

// testOpenAIPath 测试特定的OpenAI路径
func (s *AdminServer) testOpenAIPath(ep *endpoint.Endpoint, path string, timeout time.Duration) *EndpointTestResult {
	result := &EndpointTestResult{
		Format: "openai",
	}
	
	testURL := ep.URLOpenAI + path
	result.URL = testURL

	// 根据端点配置选择测试模型
	testModel := s.selectTestModel(ep, "openai")
	originalModel := testModel

	// 构建请求体（根据路径选择格式）
	var reqBody map[string]interface{}
	if path == "/responses" {
		// Codex新格式
		reqBody = map[string]interface{}{
			"model": testModel,
			"input": "Hi",
			"max_tokens": 10,
			"stream": false,
		}
	} else {
		// 传统chat/completions格式
		reqBody = map[string]interface{}{
			"model": testModel,
			"max_tokens": 10,
			"stream": false,
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hi",
				},
			},
		}
	}

	requestBody, err := json.Marshal(reqBody)
	if err != nil {
		result.Error = fmt.Sprintf("failed to marshal request: %v", err)
		return result
	}

	// 创建HTTP请求
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", testURL, bytes.NewReader(requestBody))
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		return result
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 添加认证头
	authValue, err := ep.GetAuthHeader()
	if err != nil {
		result.Error = fmt.Sprintf("failed to get auth header: %v", err)
		return result
	}
	if authValue != "" {
		req.Header.Set("Authorization", authValue)
	}

	// 发送请求并计时
	startTime := time.Now()
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	responseTime := time.Since(startTime).Milliseconds()
	result.ResponseTime = responseTime

	if err != nil {
		result.Error = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read response: %v", err)
		return result
	}

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		return result
	}

	// 验证响应格式
	var jsonResp map[string]interface{}
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		result.Error = fmt.Sprintf("invalid JSON response: %v", err)
		return result
	}

	// 检查响应是否符合预期格式
	if path == "/responses" {
		if _, hasOutput := jsonResp["output"]; !hasOutput {
			result.Error = "missing 'output' field in OpenAI responses format"
			return result
		}
	} else {
		if _, hasChoices := jsonResp["choices"]; !hasChoices {
			result.Error = "missing 'choices' field in OpenAI chat completions format"
			return result
		}
	}

	result.Success = true
	return result
}

// updateOpenAIPreference 更新端点的OpenAI格式偏好
func (s *AdminServer) updateOpenAIPreference(ep *endpoint.Endpoint, successfulPath string) {
	if successfulPath == "/responses" {
		ep.OpenAIPreference = "responses"
	} else {
		ep.OpenAIPreference = "chat_completions"
	}
}

// testSingleEndpoint 测试单个端点的所有配置格式
func (s *AdminServer) testSingleEndpoint(ep *endpoint.Endpoint) *BatchTestResult {
	startTime := time.Now()
	result := &BatchTestResult{
		EndpointName: ep.Name,
		Results:      make([]*EndpointTestResult, 0),
	}

	// 测试超时时间
	timeout := 30 * time.Second

	// 测试Anthropic格式（如果配置了）
	if ep.URLAnthropic != "" {
		anthropicResult := s.testEndpointFormat(ep, "anthropic", timeout)
		result.Results = append(result.Results, anthropicResult)
	}

	// 测试OpenAI格式（如果配置了）
	if ep.URLOpenAI != "" {
		openaiResult := s.testOpenAIFormatWithRetry(ep, timeout)
		result.Results = append(result.Results, openaiResult)
	}

	result.TotalTime = time.Since(startTime).Milliseconds()
	return result
}

// testAllEndpoints 批量测试所有端点
func (s *AdminServer) testAllEndpoints() []*BatchTestResult {
	allEndpoints := s.endpointManager.GetAllEndpoints()
	results := make([]*BatchTestResult, 0, len(allEndpoints))

	for _, ep := range allEndpoints {
		// 测试所有端点，不论启用状态
		result := s.testSingleEndpoint(ep)
		results = append(results, result)
	}

	return results
}

// selectTestModel 根据端点配置和格式选择合适的测试模型
func (s *AdminServer) selectTestModel(ep *endpoint.Endpoint, format string) string {
	// 如果端点有模型重写配置，使用重写规则的目标模型作为测试模型
	if ep.ModelRewrite != nil && ep.ModelRewrite.Enabled && len(ep.ModelRewrite.Rules) > 0 {
		// 使用第一条规则的源模式作为测试模型（会被重写为目标模型）
		return ep.ModelRewrite.Rules[0].SourcePattern
	}

	// 根据格式返回默认测试模型
	if format == "anthropic" {
		return "claude-sonnet-4-20250514"
	} else if format == "openai" {
		return "gpt-5"
	}

	return "test-model"
}

// detectClientType 检测客户端类型
func (s *AdminServer) detectClientType(req *http.Request) string {
	userAgent := req.Header.Get("User-Agent")

	if strings.Contains(strings.ToLower(userAgent), "claude-code") {
		return "claude-code"
	} else if strings.Contains(strings.ToLower(userAgent), "codex") {
		return "codex"
	}

	// 根据其他特征检测
	if strings.Contains(req.Header.Get("Content-Type"), "anthropic") {
		return "claude-code"
	}

	return "unknown"
}

// TestModelRewrite 测试模型重写规则
func (s *AdminServer) TestModelRewrite(endpointName string, testModel string) (*ModelRewriteTestResult, error) {
	ep := s.getEndpointByName(endpointName)
	if ep == nil {
		return nil, fmt.Errorf("endpoint not found: %s", endpointName)
	}

	result := &ModelRewriteTestResult{
		EndpointName:  endpointName,
		OriginalModel: testModel,
		Success:       false,
	}

	if ep.ModelRewrite == nil || !ep.ModelRewrite.Enabled {
		result.Error = "Model rewrite is not enabled for this endpoint"
		return result, nil
	}

	// 创建重写器测试规则
	testLogger := logger.Logger{}
	if s.logger != nil {
		testLogger = *s.logger
	}
	rewriter := modelrewrite.NewRewriter(testLogger)
	rewrittenModel, matchedPattern, matched := rewriter.TestRewriteRule(testModel, ep.ModelRewrite.Rules)

	result.RewrittenModel = rewrittenModel
	result.MatchedPattern = matchedPattern
	result.Success = matched

	if !matched {
		result.Error = "No rewrite rule matched the test model"
	}

	return result, nil
}

// ModelRewriteTestResult 模型重写测试结果
type ModelRewriteTestResult struct {
	EndpointName   string `json:"endpoint_name"`
	OriginalModel  string `json:"original_model"`
	RewrittenModel string `json:"rewritten_model"`
	MatchedPattern string `json:"matched_pattern"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// validateStreamResponse 验证流式响应格式
func (s *AdminServer) validateStreamResponse(responseBody string, format string) bool {
	if format == "anthropic" {
		// Anthropic SSE格式验证
		lines := strings.Split(responseBody, "\n")
		hasDataEvent := false
		hasDoneMarker := false

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
				hasDataEvent = true
				// 验证是否为有效JSON
				jsonStr := strings.TrimPrefix(line, "data: ")
				var eventData map[string]interface{}
				if json.Unmarshal([]byte(jsonStr), &eventData) != nil {
					return false
				}
			} else if line == "data: [DONE]" {
				hasDoneMarker = true
			}
		}

		return hasDataEvent && hasDoneMarker
	} else if format == "openai" {
		// OpenAI SSE格式验证
		lines := strings.Split(responseBody, "\n")
		hasDataEvent := false
		hasDoneMarker := false

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
				hasDataEvent = true
				// 验证是否为有效JSON
				jsonStr := strings.TrimPrefix(line, "data: ")
				var eventData map[string]interface{}
				if json.Unmarshal([]byte(jsonStr), &eventData) != nil {
					return false
				}
				// 检查是否有choices字段
				if _, hasChoices := eventData["choices"]; !hasChoices {
					return false
				}
			} else if line == "data: [DONE]" {
				hasDoneMarker = true
			}
		}

		return hasDataEvent && hasDoneMarker
	}

	return false
}

// TestEndpointStreaming 测试端点的流式响应
func (s *AdminServer) TestEndpointStreaming(endpointName string, format string) (*EndpointTestResult, error) {
	ep := s.getEndpointByName(endpointName)
	if ep == nil {
		return nil, fmt.Errorf("endpoint not found: %s", endpointName)
	}

	timeout := 30 * time.Second
	result := s.testEndpointFormatWithStream(ep, format, timeout, true)

	return result, nil
}

// BatchTestStreaming 批量测试所有端点的流式响应
func (s *AdminServer) BatchTestStreaming() []*BatchTestResult {
	allEndpoints := s.endpointManager.GetAllEndpoints()
	results := make([]*BatchTestResult, 0, len(allEndpoints))

	for _, ep := range allEndpoints {
		startTime := time.Now()
		batchResult := &BatchTestResult{
			EndpointName: ep.Name,
			Results:      make([]*EndpointTestResult, 0),
		}

		timeout := 30 * time.Second

		// 测试Anthropic格式流式响应（如果配置了）
		if ep.URLAnthropic != "" {
			anthropicResult := s.testEndpointFormatWithStream(ep, "anthropic", timeout, true)
			anthropicResult.Format = "anthropic-stream"
			batchResult.Results = append(batchResult.Results, anthropicResult)
		}

		// 测试OpenAI格式流式响应（如果配置了）
		if ep.URLOpenAI != "" {
			openaiResult := s.testEndpointFormatWithStream(ep, "openai", timeout, true)
			openaiResult.Format = "openai-stream"
			batchResult.Results = append(batchResult.Results, openaiResult)
		}

		batchResult.TotalTime = time.Since(startTime).Milliseconds()
		results = append(results, batchResult)
	}

	return results
}

// getEndpointByName 根据名称获取端点
func (s *AdminServer) getEndpointByName(name string) *endpoint.Endpoint {
	allEndpoints := s.endpointManager.GetAllEndpoints()
	for _, ep := range allEndpoints {
		if ep.Name == name {
			return ep
		}
	}
	return nil
}
