package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

    "claude-code-codex-companion/internal/endpoint"
    jsonutils "claude-code-codex-companion/internal/common/json"
    "claude-code-codex-companion/internal/logger"
    "claude-code-codex-companion/internal/modelrewrite"
    "claude-code-codex-companion/internal/validator"
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
		requestBody, err = jsonutils.SafeMarshal(reqBody)
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
		// 修复：为OpenAI URL添加/v1前缀
		testURL = strings.TrimSuffix(ep.URLOpenAI, "/") + "/v1" + openaiPath
		result.URL = testURL

		// 根据端点配置选择测试模型
		testModel := s.selectTestModel(ep, "openai")
		originalModel = testModel

		// 🔧 修复: 根据路径选择正确的请求体格式
		var reqBody map[string]interface{}
		if openaiPath == "/responses" {
			// Codex/Responses API格式 (OpenAI新版)
			reqBody = map[string]interface{}{
				"model": testModel,
				"input": []map[string]interface{}{
					{
						"role": "user",
						"content": []map[string]interface{}{
							{"type": "input_text", "text": "Hi"},
						},
					},
				},
				"max_tokens": 10,
				"stream":     stream,
			}
		} else {
			// Chat Completions格式 (传统格式)
			reqBody = map[string]interface{}{
				"model":      testModel,
				"max_tokens": 10,
				"stream":     stream,
				"messages": []map[string]interface{}{
					{
						"role":    "user",
						"content": "Hi",
					},
				},
			}
		}

		requestBody, err = json.Marshal(reqBody)
		if err != nil {
			result.Error = fmt.Sprintf("failed to marshal request: %v", err)
			return result
		}
	} else if format == "gemini" {
	// 使用Gemini格式测试
	if ep.URLGemini == "" {
	  result.Error = "endpoint does not have Gemini URL configured"
			return result
		}
		testURL = ep.GetFullURLWithFormat("/v1beta/models/gemini-2.0-flash-exp:generateContent", "gemini")
		result.URL = testURL

		// 根据端点配置选择测试模型
		testModel := s.selectTestModel(ep, "gemini")
		if testModel == "test-model" {
			testModel = "gemini-2.0-flash-exp"
		}
		originalModel = testModel

		// Gemini格式请求体
		reqBody := map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"role":  "user",
					"parts": []map[string]interface{}{
						{"text": "Hi"},
					},
				},
			},
			"generationConfig": map[string]interface{}{
				"maxOutputTokens": 10,
			},
		}
		if stream {
			reqBody["generationConfig"].(map[string]interface{})["stream"] = true
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

	// 生成测试请求ID
	testRequestID := fmt.Sprintf("test-%s-%s-%d", ep.Name, format, time.Now().UnixNano())

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
		// 伪装成 Anthropic SDK 客户端（基于官方 SDK 格式）
		// 参考：anthropic-sdk-python/typescript 的真实请求头
		req.Header.Set("User-Agent", "anthropic-sdk-go/0.1.0")
		req.Header.Set("x-stainless-lang", "go")
		req.Header.Set("x-stainless-package-version", "0.1.0")
		req.Header.Set("x-stainless-runtime", "go")
		req.Header.Set("x-stainless-runtime-version", "1.21.0")
	} else if format == "openai" {
	// 伪装成 OpenAI SDK 客户端（基于真实抓包数据）
	// 参考：OpenAI/Python 1.10.0 的实际请求头
	req.Header.Set("User-Agent", "OpenAI/Go 1.0.0")
	req.Header.Set("x-stainless-lang", "go")
	req.Header.Set("x-stainless-package-version", "1.0.0")
	req.Header.Set("x-stainless-runtime", "go")
	req.Header.Set("x-stainless-runtime-version", "1.21.0")
	req.Header.Set("Openai-Beta", "responses=v1")
	} else if format == "gemini" {
		// Gemini API请求头
		req.Header.Set("User-Agent", "GoogleAI/Go 1.0.0")
	}

	// 添加认证头
	authValue, err := ep.GetAuthHeader()
	if err != nil {
		result.Error = fmt.Sprintf("failed to get auth header: %v", err)
		return result
	}
	if authValue != "" {
		// 根据端点的认证类型设置认证头
		authType := ep.AuthType
		if authType == "" {
			authType = "auto"
		}

		if format == "gemini" {
		// Gemini API使用 x-goog-api-key 头
		if strings.HasPrefix(authValue, "Bearer ") {
		req.Header.Set("x-goog-api-key", strings.TrimPrefix(authValue, "Bearer "))
		} else {
		req.Header.Set("x-goog-api-key", authValue)
		}
		} else if authType == "api_key" || authType == "x-api-key" {
		// 使用 x-api-key 头（仅限原生 Anthropic API）
		if strings.HasPrefix(authValue, "Bearer ") {
		req.Header.Set("x-api-key", strings.TrimPrefix(authValue, "Bearer "))
		} else {
		req.Header.Set("x-api-key", authValue)
		}
		// 某些端点需要同时设置 Authorization
		 if format == "openai" || !strings.Contains(ep.GetURLForFormat(format), "api.anthropic.com") {
		 if !strings.HasPrefix(authValue, "Bearer ") {
		  req.Header.Set("Authorization", "Bearer "+authValue)
		} else {
		  req.Header.Set("Authorization", authValue)
		}
		}
		} else {
			// auth_token, bearer, auto 或其他类型：使用 Authorization 头
			if !strings.HasPrefix(authValue, "Bearer ") && !strings.HasPrefix(authValue, "Basic ") {
				req.Header.Set("Authorization", "Bearer "+authValue)
			} else {
				req.Header.Set("Authorization", authValue)
			}
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

	// 记录测试请求到日志系统
	testStartTime := time.Now()
	if s.logger != nil {
		// 收集请求头信息
		requestHeaders := make(map[string]string)
		for key, values := range req.Header {
			if len(values) > 0 {
				requestHeaders[key] = values[0]
			}
		}

		// 记录测试请求开始
		s.logger.Info(fmt.Sprintf("📋 Test request: %s (%s format)", ep.Name, format), map[string]interface{}{
			"request_id":  testRequestID,
			"endpoint":    ep.Name,
			"format":      format,
			"url":         testURL,
			"model":       originalModel,
			"rewritten":   rewrittenModel,
		})
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
		// 记录失败的测试请求
		if s.logger != nil {
			s.logger.LogRequest(&logger.RequestLog{
				RequestID:      testRequestID,
				Timestamp:      testStartTime,
				Method:         "POST",
				Path:           testURL,
				Endpoint:       ep.Name,
				ClientType:     "test",
				RequestFormat:  format,
				OriginalModel:  originalModel,
				RewrittenModel: rewrittenModel,
				StatusCode:     0,
				Error:          err.Error(),
				DurationMs:     time.Since(testStartTime).Milliseconds(),
			})
		}
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
	errorMessage := ""
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 🔧 特殊处理：OpenAI格式404错误（端点不支持/responses路径）
		if format == "openai" && resp.StatusCode == 404 && strings.Contains(testURL, "/responses") {
			result.Error = fmt.Sprintf("HTTP 404: Endpoint does not support /responses (Codex format). Only supports Claude Code (/messages).")
			errorMessage = result.Error
			// 记录测试请求
			if s.logger != nil {
				s.logger.LogRequest(&logger.RequestLog{
					RequestID:      testRequestID,
					Timestamp:      testStartTime,
					Method:         "POST",
					Path:           testURL,
					Endpoint:       ep.Name,
					ClientType:     "test",
					RequestFormat:  format,
					OriginalModel:  originalModel,
					RewrittenModel: rewrittenModel,
					StatusCode:     resp.StatusCode,
					Error:          errorMessage,
					DurationMs:     time.Since(testStartTime).Milliseconds(),
					RequestBody:    string(requestBody),
					ResponseBody:   string(body),
				})
			}
			return result
		}

		// 尝试解析错误信息
		var errResp map[string]interface{}
		if json.Unmarshal(body, &errResp) == nil {
			if errorField, ok := errResp["error"]; ok {
				if errorMap, ok := errorField.(map[string]interface{}); ok {
					if msg, ok := errorMap["message"].(string); ok {
						result.Error = msg
						errorMessage = msg
					} else {
						result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
						errorMessage = result.Error
					}
				} else {
					result.Error = fmt.Sprintf("HTTP %d: %v", resp.StatusCode, errorField)
					errorMessage = result.Error
				}
			} else {
				result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
				errorMessage = result.Error
			}
		} else {
			result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
			errorMessage = result.Error
		}

		// 🔍 检查是否为客户端验证失败（端点正常，但拒绝测试请求）
		isClientValidationFailure := validator.CheckClientValidation(resp.StatusCode, string(body), errorMessage)

		// 🔧 修复：404错误不应被视为客户端验证失败，应该标记为失败
		if isClientValidationFailure && resp.StatusCode != 404 {
			// 客户端验证失败视为"成功"（端点正常响应，只是拒绝了测试）
			result.Success = true
			result.Error = "" // 清除错误信息
			// 在日志中标记为客户端验证拒绝
			s.logger.Info(fmt.Sprintf("Endpoint %s (%s) rejected test request (client validation): %s",
				ep.Name, format, errorMessage), nil)
		}

		// 记录测试请求（无论成功或失败）
		if s.logger != nil {
			s.logger.LogRequest(&logger.RequestLog{
				RequestID:      testRequestID,
				Timestamp:      testStartTime,
				Method:         "POST",
				Path:           testURL,
				Endpoint:       ep.Name,
				ClientType:     "test",
				RequestFormat:  format,
				OriginalModel:  originalModel,
				RewrittenModel: rewrittenModel,
				StatusCode:     resp.StatusCode,
				Error:          errorMessage,
				DurationMs:     time.Since(testStartTime).Milliseconds(),
				RequestBody:    string(requestBody),
				ResponseBody:   string(body),
			})
		}

		// 如果是客户端验证失败且不是404，直接返回成功结果
		if isClientValidationFailure && resp.StatusCode != 404 {
			return result
		}

		// 其他错误正常返回失败
		return result
	}

	// 验证响应格式
	var jsonResp map[string]interface{}
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		errorMsg := fmt.Sprintf("invalid JSON response: %v", err)
		result.Error = errorMsg

		// 🔍 检查JSON解析错误是否为客户端验证失败（通常是返回了HTML页面）
		if validator.CheckClientValidation(resp.StatusCode, string(body), errorMsg) && resp.StatusCode != 404 {
			result.Success = true
			result.Error = ""
			s.logger.Info(fmt.Sprintf("Endpoint %s (%s) rejected test request (non-JSON response): %s",
				ep.Name, format, errorMsg), nil)
			return result
		}

		return result
	}

	// 记录响应体用于调试（截断到前500字符）
	responsePreview := string(body)
	if len(responsePreview) > 500 {
		responsePreview = responsePreview[:500] + "..."
	}
	s.logger.Debug(fmt.Sprintf("Endpoint test response (%s): %s", format, responsePreview), nil)

	// 先检查是否是错误响应（某些端点可能用200状态码返回错误）
	// 🔧 修复: 忽略 error 字段为 null 的情况（某些API在正常响应中也包含 "error": null）
	if errorField, hasError := jsonResp["error"]; hasError && errorField != nil {
		if errorMap, ok := errorField.(map[string]interface{}); ok {
			if msg, ok := errorMap["message"].(string); ok {
				result.Error = fmt.Sprintf("API error: %s", msg)
				errorMessage = result.Error
			} else {
				result.Error = fmt.Sprintf("API error: %v", errorField)
				errorMessage = result.Error
			}
		} else {
			result.Error = fmt.Sprintf("API error: %v", errorField)
			errorMessage = result.Error
		}

		// 🔍 检查API错误是否为客户端验证失败
		if validator.CheckClientValidation(resp.StatusCode, string(body), errorMessage) && resp.StatusCode != 404 {
			result.Success = true
			result.Error = ""
			s.logger.Info(fmt.Sprintf("Endpoint %s (%s) rejected test request (API error): %s",
				ep.Name, format, errorMessage), nil)
			return result
		}

		// 记录200状态码但包含错误的请求
		if s.logger != nil {
			s.logger.LogRequest(&logger.RequestLog{
				RequestID:      testRequestID,
				Timestamp:      testStartTime,
				Method:         "POST",
				Path:           testURL,
				Endpoint:       ep.Name,
				ClientType:     "test",
				RequestFormat:  format,
				OriginalModel:  originalModel,
				RewrittenModel: rewrittenModel,
				StatusCode:     resp.StatusCode,
				Error:          errorMessage,
				DurationMs:     time.Since(testStartTime).Milliseconds(),
				RequestBody:    string(requestBody),
				ResponseBody:   string(body),
			})
		}
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

	// 测试成功 - 记录成功的测试请求
	result.Success = true
	if s.logger != nil {
		s.logger.LogRequest(&logger.RequestLog{
			RequestID:      testRequestID,
			Timestamp:      testStartTime,
			Method:         "POST",
			Path:           testURL,
			Endpoint:       ep.Name,
			ClientType:     "test",
			RequestFormat:  format,
			OriginalModel:  originalModel,
			RewrittenModel: rewrittenModel,
			StatusCode:     resp.StatusCode,
			DurationMs:     time.Since(testStartTime).Milliseconds(),
			RequestBody:    string(requestBody),
			ResponseBody:   string(body),
		})
	}

	// 🎓 持久化学习结果：测试成功后保存格式偏好
	// 只有在首次探测成功时才持久化（避免重复保存）
	if format == "openai" && (ep.OpenAIPreference == "" || ep.OpenAIPreference == "auto") {
		// 根据测试路径推断格式偏好
		testPath := result.URL
		if strings.Contains(testPath, "/responses") {
			ep.OpenAIPreference = "responses"
			s.logger.Info(fmt.Sprintf("🎓 Test: Learned OpenAI preference 'responses' for endpoint '%s'", ep.Name), nil)
		} else if strings.Contains(testPath, "/chat/completions") {
			ep.OpenAIPreference = "chat_completions"
			s.logger.Info(fmt.Sprintf("🎓 Test: Learned OpenAI preference 'chat_completions' for endpoint '%s'", ep.Name), nil)
		}

		// 持久化学习结果
		if s.persistenceHandler != nil && ep.OpenAIPreference != "" && ep.OpenAIPreference != "auto" {
			s.persistenceHandler.PersistEndpointLearning(ep)
		}
	}

	return result
}

// selectOpenAIPath 根据端点偏好选择OpenAI API路径
func (s *AdminServer) selectOpenAIPath(ep *endpoint.Endpoint) string {
	if ep.OpenAIPreference == "responses" {
		return "/responses"
	} else if ep.OpenAIPreference == "chat_completions" {
		return "/chat/completions"
	}
	
	// 默认优先尝试 responses 格式（Codex新格式，更先进）
	return "/responses"
}

// testOpenAIFormatWithRetry 测试OpenAI格式并支持格式重试
func (s *AdminServer) testOpenAIFormatWithRetry(ep *endpoint.Endpoint, timeout time.Duration) *EndpointTestResult {
	// 如果端点已经有明确的格式偏好，直接使用
	if ep.OpenAIPreference == "responses" || ep.OpenAIPreference == "chat_completions" {
		preferredPath := s.selectOpenAIPath(ep)
		return s.testOpenAIPath(ep, preferredPath, timeout)
}

// 自动模式：优先尝试 /responses 格式（Codex新格式）
responsesResult := s.testOpenAIPath(ep, "/responses", timeout)
	if responsesResult.Success {
		// 学习并存储成功的格式偏好
		s.updateOpenAIPreference(ep, "/responses")
		return responsesResult
}

	// 如果 /responses 失败，尝试 /chat/completions 格式
	chatCompletionsResult := s.testOpenAIPath(ep, "/chat/completions", timeout)
	if chatCompletionsResult.Success {
	// 学习并存储成功的格式偏好
	s.updateOpenAIPreference(ep, "/chat/completions")
	return chatCompletionsResult
	}

	// 两种格式都失败，返回第一个结果（通常是 /responses）
	return responsesResult
}

// testOpenAIPath 测试特定的OpenAI路径
func (s *AdminServer) testOpenAIPath(ep *endpoint.Endpoint, path string, timeout time.Duration) *EndpointTestResult {
	result := &EndpointTestResult{
		Format: "openai",
	}
	
	// 修复：为OpenAI URL添加/v1前缀
	testURL := strings.TrimSuffix(ep.URLOpenAI, "/") + "/v1" + path
	result.URL = testURL

	// 根据端点配置选择测试模型
	testModel := s.selectTestModel(ep, "openai")

	// 构建请求体（根据路径选择格式）
	var reqBody map[string]interface{}
	if path == "/responses" {
		// Codex新格式 - 使用结构化的input数组（与testEndpointFormatWithStream保持一致）
		reqBody = map[string]interface{}{
			"model": testModel,
			"input": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{"type": "input_text", "text": "Hi"},
					},
				},
			},
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

	// 初始化性能指标
	result.PerformanceMetrics = &PerformanceMetrics{
		TotalSize: 0,
	}

	// 计算首字节时间（近似值）
	if resp != nil {
		result.PerformanceMetrics.FirstByteTime = time.Since(startTime).Milliseconds()
	}

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

	// 更新性能指标
	result.PerformanceMetrics.ContentDownloadTime = time.Since(startTime).Milliseconds()
	result.PerformanceMetrics.TotalSize = int64(len(body))

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
	oldPreference := ep.OpenAIPreference
	
	if successfulPath == "/responses" {
		ep.OpenAIPreference = "responses"
	} else {
		ep.OpenAIPreference = "chat_completions"
	}
	
	// 🎓 持久化学习结果：如果偏好发生变化
	if oldPreference != ep.OpenAIPreference {
		s.logger.Info(fmt.Sprintf("🎓 Test: Learned OpenAI preference '%s' for endpoint '%s' (was: '%s')", 
			ep.OpenAIPreference, ep.Name, oldPreference), nil)
		
		// 尝试持久化学习结果
		if s.persistenceHandler != nil {
			s.persistenceHandler.PersistEndpointLearning(ep)
		} else {
			// 如果没有持久化处理器，至少记录到日志
			s.logger.Info(fmt.Sprintf("💾 Learning result not persisted (no persistence handler): %s -> %s", 
				ep.Name, ep.OpenAIPreference), nil)
		}
	}
}

// testSingleEndpoint 测试单个端点的所有配置格式
func (s *AdminServer) testSingleEndpoint(ep *endpoint.Endpoint) *BatchTestResult {
	startTime := time.Now()
	result := &BatchTestResult{
		EndpointName: ep.Name,
		Results:      make([]*EndpointTestResult, 0),
	}

    // 测试超时时间（放宽到20秒，减少 context deadline exceeded 误报）
    timeout := 20 * time.Second

	// 记录测试开始
	s.logger.Info(fmt.Sprintf("🧪 Testing endpoint: %s", ep.Name), nil)

	// 测试Anthropic格式（如果配置了）
	if ep.URLAnthropic != "" {
	anthropicResult := s.testEndpointFormat(ep, "anthropic", timeout)
	result.Results = append(result.Results, anthropicResult)
	if anthropicResult.Success {
	s.logger.Info(fmt.Sprintf("  ✅ Anthropic format test passed (%dms)", anthropicResult.ResponseTime), nil)
	// 记录测试成功到端点统计
	ep.RecordRequest(true, fmt.Sprintf("test-anthropic-%d", time.Now().UnixNano()), time.Duration(anthropicResult.PerformanceMetrics.FirstByteTime)*time.Millisecond, time.Duration(anthropicResult.ResponseTime)*time.Millisecond)
	} else {
	s.logger.Info(fmt.Sprintf("  ❌ Anthropic format test failed: %s", anthropicResult.Error), nil)
	// 记录测试失败到端点统计
	ep.RecordRequest(false, fmt.Sprintf("test-anthropic-%d", time.Now().UnixNano()), time.Duration(anthropicResult.PerformanceMetrics.FirstByteTime)*time.Millisecond, time.Duration(anthropicResult.ResponseTime)*time.Millisecond)
	}
	}

	// 测试OpenAI格式（如果配置了）
	if ep.URLOpenAI != "" {
	openaiResult := s.testOpenAIFormatWithRetry(ep, timeout)
	result.Results = append(result.Results, openaiResult)
	if openaiResult.Success {
	s.logger.Info(fmt.Sprintf("  ✅ OpenAI format test passed (%dms)", openaiResult.ResponseTime), nil)
	// 记录测试成功到端点统计
	ep.RecordRequest(true, fmt.Sprintf("test-openai-%d", time.Now().UnixNano()), time.Duration(openaiResult.PerformanceMetrics.FirstByteTime)*time.Millisecond, time.Duration(openaiResult.ResponseTime)*time.Millisecond)
	} else {
	s.logger.Info(fmt.Sprintf("  ❌ OpenAI format test failed: %s", openaiResult.Error), nil)
	// 记录测试失败到端点统计
	ep.RecordRequest(false, fmt.Sprintf("test-openai-%d", time.Now().UnixNano()), time.Duration(openaiResult.PerformanceMetrics.FirstByteTime)*time.Millisecond, time.Duration(openaiResult.ResponseTime)*time.Millisecond)
	}
	}

	// 测试Gemini格式（如果配置了）
    if ep.URLGemini != "" {
        geminiResult := s.testEndpointFormat(ep, "gemini", timeout)
        result.Results = append(result.Results, geminiResult)
		if geminiResult.Success {
			s.logger.Info(fmt.Sprintf("  ✅ Gemini format test passed (%dms)", geminiResult.ResponseTime), nil)
			// 记录测试成功到端点统计
			ep.RecordRequest(true, fmt.Sprintf("test-gemini-%d", time.Now().UnixNano()), time.Duration(geminiResult.PerformanceMetrics.FirstByteTime)*time.Millisecond, time.Duration(geminiResult.ResponseTime)*time.Millisecond)
		} else {
			s.logger.Info(fmt.Sprintf("  ❌ Gemini format test failed: %s", geminiResult.Error), nil)
			// 记录测试失败到端点统计
			ep.RecordRequest(false, fmt.Sprintf("test-gemini-%d", time.Now().UnixNano()), time.Duration(geminiResult.PerformanceMetrics.FirstByteTime)*time.Millisecond, time.Duration(geminiResult.ResponseTime)*time.Millisecond)
		}
	}
	result.TotalTime = time.Since(startTime).Milliseconds()
	s.logger.Info(fmt.Sprintf("📊 Endpoint %s test completed in %dms", ep.Name, result.TotalTime), nil)

    return result
}

// TestEndpoint 对外方法：测试单个端点（便于CLI探针复用）
func (s *AdminServer) TestEndpoint(endpointName string) *BatchTestResult {
    ep := s.getEndpointByName(endpointName)
    if ep == nil {
        return &BatchTestResult{EndpointName: endpointName, Results: []*EndpointTestResult{&EndpointTestResult{Error: "endpoint not found"}}}
    }
    return s.testSingleEndpoint(ep)
}

// testAllEndpoints 并行批量测试所有端点
func (s *AdminServer) testAllEndpoints() []*BatchTestResult {
	allEndpoints := s.endpointManager.GetAllEndpoints()
	results := make([]*BatchTestResult, len(allEndpoints))

	// 使用 WaitGroup 进行并发测试
	var wg sync.WaitGroup

	// 并发测试所有端点
	for i, ep := range allEndpoints {
		wg.Add(1)
		go func(index int, endpoint *endpoint.Endpoint) {
			defer wg.Done()

			// 测试单个端点
			result := s.testSingleEndpoint(endpoint)
			results[index] = result

			// 实时记录测试结果到日志
			s.logger.Info(fmt.Sprintf("📊 Endpoint test completed: %s (Anthropic: %v, OpenAI: %v)",
				endpoint.Name,
				hasSuccessfulTest(result, "anthropic"),
				hasSuccessfulTest(result, "openai"),
			), nil)
		}(i, ep)
	}

	// 等待所有测试完成
	wg.Wait()

	return results
}

// hasSuccessfulTest 检查特定格式的测试是否成功
func hasSuccessfulTest(result *BatchTestResult, format string) bool {
	if result == nil || result.Results == nil {
		return false
	}
	for _, r := range result.Results {
		if r.Format == format && r.Success {
			return true
		}
	}
	return false
}

// selectTestModel 根据端点配置和格式选择合适的测试模型
// 返回应用模型重写规则后的实际模型
func (s *AdminServer) selectTestModel(ep *endpoint.Endpoint, format string) string {
// 根据格式返回默认测试模型
var defaultModel string
if format == "anthropic" {
defaultModel = "claude-sonnet-4-5-20250929"
} else if format == "openai" {
defaultModel = "gpt-5"
} else if format == "gemini" {
defaultModel = "gemini-2.0-flash-exp"
} else {
		return "test-model"
	}

	// 如果端点有模型重写配置，应用重写规则获取实际使用的模型
	if ep.ModelRewrite != nil && ep.ModelRewrite.Enabled && len(ep.ModelRewrite.Rules) > 0 {
		// 创建模型重写器来应用规则
		testLogger := logger.Logger{}
		if s.logger != nil {
			testLogger = *s.logger
		}
		rewriter := modelrewrite.NewRewriter(testLogger)

		// 应用模型重写规则，获取实际会使用的模型
		rewrittenModel, _, matched := rewriter.TestRewriteRule(defaultModel, ep.ModelRewrite.Rules)
		if matched {
			// 返回重写后的模型
			return rewrittenModel
		}
		// 如果没有匹配的规则，返回默认模型
		return defaultModel
	}

	return defaultModel
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


