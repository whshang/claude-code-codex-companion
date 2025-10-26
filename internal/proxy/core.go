package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	jsonutils "claude-code-codex-companion/internal/common/json"
	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/tagging"
	"claude-code-codex-companion/internal/utils"

	"github.com/gin-gonic/gin"
)

// core.go: 核心代理逻辑模块
// 这是代理转发功能的核心，负责协调请求的整个生命周期。
//
// 目标：
// - 包含重构后的 proxyToEndpoint 函数。
// - 将原先庞大的 proxyToEndpoint 拆分为更小的、职责单一的函数，如：
//   - prepareRequest: 准备和验证请求
//   - executeRequest: 执行对上游端点的HTTP请求
//   - handleResponse: 处理上游响应
//   - finalizeRequest: 完成请求和日志记录
// - 定义请求状态管理结构体，以替代对 gin.Context 的过度依赖。

const responseCaptureLimit = 64 * 1024
const conversionStageSeparator = "|"

// RequestContext 请求上下文结构体，用于减少对 gin.Context 的依赖
type RequestContext struct {
	RequestID             string
	Path                  string
	InboundPath           string
	RequestBody           []byte
	FinalRequestBody      []byte
	Tags                  []string
	OriginalModel         string
	RewrittenModel        string
	ClientRequestFormat   string
	EndpointRequestFormat string
	ActualEndpointFormat  string
	NeedsConversion       bool
	CodexNeedsConversion  bool
	ConversionStages      []string
	AttemptNumber         int
	StartTime             time.Time
	EndpointStartTime     time.Time
	FirstByteTime         time.Duration
	LastError             error  // 记录最后一次错误
	LastStatusCode        int    // 记录最后一次状态码
	LastResponseBody      string // 记录最后一次响应体
}

// NewRequestContext 创建新的请求上下文
func NewRequestContext(c *gin.Context, requestBody []byte, path string, attemptNumber int) *RequestContext {
	ctx := &RequestContext{
		RequestID:         c.GetString("request_id"),
		Path:              path,
		InboundPath:       path,
		RequestBody:       requestBody,
		FinalRequestBody:  requestBody,
		AttemptNumber:     attemptNumber,
		StartTime:         time.Now(),
		EndpointStartTime: time.Now(),
	}

	// 从 gin.Context 获取标签信息
	if taggedRequest, exists := c.Get("tagged_request"); exists {
		if tr, ok := taggedRequest.(*tagging.TaggedRequest); ok && tr != nil {
			ctx.Tags = tr.Tags
		}
	}

	// 从 gin.Context 获取格式检测信息
	if detection, exists := c.Get("format_detection"); exists {
		if det, ok := detection.(*utils.FormatDetectionResult); ok {
			ctx.ClientRequestFormat = string(det.Format)
		}
	}

	return ctx
}

// prepareRequest 准备和验证请求
func (s *Server) prepareRequest(c *gin.Context, ep *endpoint.Endpoint, ctx *RequestContext) error {
	// 检查是否为 count_tokens 请求到 OpenAI 端点
	isCountTokensRequest := strings.Contains(ctx.Path, "/count_tokens")
	isOpenAIEndpoint := ep.EndpointType == "openai"

	// OpenAI 端点不支持 count_tokens，立即尝试下一个端点
	if isCountTokensRequest && isOpenAIEndpoint {
		s.logger.Debug(fmt.Sprintf("Skipping count_tokens request on OpenAI endpoint %s", ep.Name))
		c.Set("skip_health_record", true)
		c.Set("skip_logging", true)
		c.Set("count_tokens_openai_skip", true)
		c.Set("last_error", fmt.Errorf("count_tokens not supported on OpenAI endpoint"))
		c.Set("last_status_code", http.StatusNotFound)
		return fmt.Errorf("count_tokens not supported on OpenAI endpoint")
	}

	if isCountTokensRequest && ep.ShouldSkipCountTokens() {
		s.logger.Debug(fmt.Sprintf("Skipping count_tokens request on endpoint %s (previously detected unsupported)", ep.Name))
		c.Set("skip_health_record", true)
		c.Set("skip_logging", true)
		c.Set("count_tokens_openai_skip", true)
		c.Set("last_error", fmt.Errorf("count_tokens not supported on endpoint"))
		c.Set("last_status_code", http.StatusNotFound)
		return fmt.Errorf("count_tokens not supported on endpoint")
	}

	// 早期检查：端点必须至少有一个URL（支持格式转换）
	if ep.URLAnthropic == "" && ep.URLOpenAI == "" && ep.URLGemini == "" {
		s.logger.Debug("Skipping endpoint: no URL configured", map[string]interface{}{
			"endpoint": ep.Name,
		})
		c.Set("skip_health_record", true)
		c.Set("last_error", fmt.Errorf("endpoint %s has no URL configured", ep.Name))
		c.Set("last_status_code", http.StatusBadGateway)
		return fmt.Errorf("endpoint %s has no URL configured", ep.Name)
	}

	return nil
}

// determineEndpointFormat 确定端点格式和转换需求
func (s *Server) determineEndpointFormat(c *gin.Context, ep *endpoint.Endpoint, ctx *RequestContext) error {
	var formatDetection *utils.FormatDetectionResult
	if detection, exists := c.Get("format_detection"); exists {
		if det, ok := detection.(*utils.FormatDetectionResult); ok {
			formatDetection = det
		}
	}

	ctx.EndpointRequestFormat = ctx.ClientRequestFormat

	// 解析端点支持的上游格式，并决定是否需要格式转换
	needsConversion := false
	actualEndpointFormat := ""

	if formatDetection != nil && formatDetection.Format != utils.FormatUnknown {
		switch formatDetection.Format {
		case utils.FormatAnthropic:
			if ep.URLAnthropic != "" {
				// 优先使用原生 Anthropic 端点
				actualEndpointFormat = "anthropic"
				ctx.EndpointRequestFormat = "anthropic"
			} else if ep.URLOpenAI != "" {
				// 使用 OpenAI 端点进行格式转换
				actualEndpointFormat = "openai"
				ctx.EndpointRequestFormat = "openai"
				needsConversion = true

				// 转换请求路径：/messages -> /chat/completions
				if strings.Contains(ctx.Path, "/messages") {
					ctx.Path = strings.Replace(ctx.Path, "/messages", "/chat/completions", 1)
					s.logger.Debug("Converted request path from /messages to /chat/completions for OpenAI endpoint", map[string]interface{}{
						"original_path":  ctx.InboundPath,
						"converted_path": ctx.Path,
						"endpoint":       ep.Name,
					})
				}
			} else if ep.URLGemini != "" {
				actualEndpointFormat = "gemini"
				ctx.EndpointRequestFormat = "gemini"
				needsConversion = true
			}
		case utils.FormatOpenAI:
			if ep.URLOpenAI != "" {
				actualEndpointFormat = "openai"
				ctx.EndpointRequestFormat = "openai"
			} else if ep.URLAnthropic != "" {
				actualEndpointFormat = "anthropic"
				ctx.EndpointRequestFormat = "anthropic"
				needsConversion = true

				// 转换请求路径：/chat/completions -> /messages
				if strings.Contains(ctx.Path, "/chat/completions") {
					ctx.Path = strings.Replace(ctx.Path, "/chat/completions", "/messages", 1)
					s.logger.Debug("Converted request path from /chat/completions to /messages for Anthropic endpoint", map[string]interface{}{
						"original_path":  ctx.InboundPath,
						"converted_path": ctx.Path,
						"endpoint":       ep.Name,
					})
				}
				//  also handle /responses -> /messages conversion
				if strings.Contains(ctx.Path, "/responses") {
					ctx.Path = strings.Replace(ctx.Path, "/responses", "/messages", 1)
					s.logger.Debug("Converted request path from /responses to /messages for Anthropic endpoint", map[string]interface{}{
						"original_path":  ctx.InboundPath,
						"converted_path": ctx.Path,
						"endpoint":       ep.Name,
					})
				}
			} else if ep.URLGemini != "" {
				actualEndpointFormat = "gemini"
				ctx.EndpointRequestFormat = "gemini"
				needsConversion = true
			}
		case utils.RequestFormat("gemini"):
			if ep.URLGemini != "" {
				actualEndpointFormat = "gemini"
				ctx.EndpointRequestFormat = "gemini"
			} else if ep.URLOpenAI != "" {
				actualEndpointFormat = "openai"
				ctx.EndpointRequestFormat = "openai"
				needsConversion = true
			} else if ep.URLAnthropic != "" {
				actualEndpointFormat = "anthropic"
				ctx.EndpointRequestFormat = "anthropic"
				needsConversion = true
			}
		default:
			if ep.URLAnthropic != "" {
				actualEndpointFormat = "anthropic"
				if ctx.EndpointRequestFormat == "" {
					ctx.EndpointRequestFormat = "anthropic"
				}
			} else if ep.URLOpenAI != "" {
				actualEndpointFormat = "openai"
				if ctx.EndpointRequestFormat == "" {
					ctx.EndpointRequestFormat = "openai"
				}
			} else if ep.URLGemini != "" {
				actualEndpointFormat = "gemini"
				if ctx.EndpointRequestFormat == "" {
					ctx.EndpointRequestFormat = "gemini"
				}
			}
		}
	} else {
		if ep.URLAnthropic != "" {
			actualEndpointFormat = "anthropic"
		} else if ep.URLOpenAI != "" {
			actualEndpointFormat = "openai"
		} else if ep.URLGemini != "" {
			actualEndpointFormat = "gemini"
		}
		if ctx.EndpointRequestFormat == "" {
			ctx.EndpointRequestFormat = actualEndpointFormat
		}
	}

	if actualEndpointFormat == "" && ctx.EndpointRequestFormat == "" {
		targetFormat := ctx.ClientRequestFormat
		if targetFormat == "" {
			targetFormat = "unknown"
		}
		s.logger.Debug("Skipping endpoint: no compatible upstream URL for request format", map[string]interface{}{
			"endpoint":        ep.Name,
			"request_format":  targetFormat,
			"available_urls":  map[string]bool{"anthropic": ep.URLAnthropic != "", "openai": ep.URLOpenAI != "", "gemini": ep.URLGemini != ""},
			"endpoint_type":   ep.EndpointType,
			"inbound_path":    ctx.InboundPath,
			"effective_path":  ctx.Path,
			"client_detected": formatDetection != nil && formatDetection.Format != utils.FormatUnknown,
		})
		c.Set("skip_health_record", true)
		c.Set("last_error", fmt.Errorf("endpoint %s has no compatible URL for request format %s", ep.Name, targetFormat))
		c.Set("last_status_code", http.StatusBadGateway)
		return fmt.Errorf("endpoint %s has no compatible URL for request format %s", ep.Name, targetFormat)
	}

	if ctx.ClientRequestFormat != "" && actualEndpointFormat != "" && ctx.ClientRequestFormat != actualEndpointFormat {
		needsConversion = true
	}

	ctx.NeedsConversion = needsConversion
	ctx.ActualEndpointFormat = actualEndpointFormat

	s.logger.Debug("Format routing decision", map[string]interface{}{
		"request_format":         ctx.ClientRequestFormat,
		"actual_endpoint_format": actualEndpointFormat,
		"needs_conversion":       needsConversion,
		"path_conversion":        ctx.InboundPath != ctx.Path,
		"original_path":          ctx.InboundPath,
		"effective_path":         ctx.Path,
		"detection_confidence": func() float64 {
			if formatDetection != nil {
				return formatDetection.Confidence
			}
			return 0
		}(),
	})

	return nil
}

// executeRequest 执行对上游端点的HTTP请求
func (s *Server) executeRequest(c *gin.Context, ep *endpoint.Endpoint, ctx *RequestContext) (*http.Response, error) {
	// 获取目标URL
	targetURL := ep.GetFullURLWithFormat(ctx.Path, ctx.EndpointRequestFormat)
	if targetURL == "" {
		s.logger.Debug("Skipping endpoint: GetFullURLWithFormat returned empty URL", map[string]interface{}{
			"endpoint":       ep.Name,
			"path":           ctx.Path,
			"request_format": ctx.EndpointRequestFormat,
		})
		c.Set("skip_health_record", true)
		c.Set("last_error", fmt.Errorf("endpoint %s returned empty URL", ep.Name))
		c.Set("last_status_code", http.StatusBadGateway)
		return nil, fmt.Errorf("endpoint %s returned empty URL", ep.Name)
	}

	// 创建HTTP请求
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(ctx.FinalRequestBody))
	if err != nil {
		s.logger.Error("Failed to create request", err)
		duration := time.Since(ctx.EndpointStartTime)
		createRequestError := fmt.Sprintf("Failed to create request: %v", err)
		setConversionContext(c, ctx.ConversionStages)
		s.logSimpleRequest(ctx.RequestID, targetURL, c.Request.Method, ctx.Path, ctx.RequestBody, ctx.FinalRequestBody, c, nil, nil, nil, duration, fmt.Errorf(createRequestError), false, ctx.Tags, "", "", "", ctx.AttemptNumber, targetURL)
		c.Set("last_error", fmt.Errorf(createRequestError))
		c.Set("last_status_code", 0)
		return nil, fmt.Errorf(createRequestError)
	}

	// 设置请求头
	for key, values := range c.Request.Header {
		if key == "Authorization" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 设置认证头
	authHeader, err := ep.GetAuthHeaderWithRefreshCallback(s.config.Timeouts.ToProxyTimeoutConfig(), s.createOAuthTokenRefreshCallback())
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to get auth header: %v", err), err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		c.Set("last_error", err)
		c.Set("last_status_code", http.StatusUnauthorized)
		return nil, err
	}
	req.Header.Set("Authorization", authHeader)

	// 根据格式补全必需的头信息
	if ctx.EndpointRequestFormat == "anthropic" {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if req.Header.Get("anthropic-version") == "" {
			req.Header.Set("anthropic-version", "2023-06-01")
		}
		if ep.AuthValue != "" {
			req.Header.Set("x-api-key", ep.AuthValue)
		}
	} else if ctx.EndpointRequestFormat == "openai" {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// 为这个端点创建支持代理的HTTP客户端
	client, err := ep.CreateProxyClient(s.config.Timeouts.ToProxyTimeoutConfig())
	if err != nil {
		s.logger.Error("Failed to create proxy client for endpoint", err)
		duration := time.Since(ctx.EndpointStartTime)
		setConversionContext(c, ctx.ConversionStages)
		s.logSimpleRequest(ctx.RequestID, targetURL, c.Request.Method, ctx.Path, ctx.RequestBody, ctx.FinalRequestBody, c, req, nil, nil, duration, err, s.isRequestExpectingStream(req), ctx.Tags, "", ctx.OriginalModel, ctx.RewrittenModel, ctx.AttemptNumber, targetURL)
		c.Set("last_error", err)
		c.Set("last_status_code", 0)
		return nil, err
	}

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		duration := time.Since(ctx.EndpointStartTime)
		s.logSimpleRequest(ctx.RequestID, targetURL, c.Request.Method, ctx.Path, ctx.RequestBody, ctx.FinalRequestBody, c, req, nil, nil, duration, err, s.isRequestExpectingStream(req), ctx.Tags, "", ctx.OriginalModel, ctx.RewrittenModel, ctx.AttemptNumber, targetURL)
		c.Set("last_error", err)
		c.Set("last_status_code", 0)
		return nil, err
	}

	// 捕获首字节时间（TTFB - Time To First Byte）
	ctx.FirstByteTime = time.Since(ctx.EndpointStartTime)

	return resp, nil
}

// handleResponse 处理上游响应
func (s *Server) handleResponse(c *gin.Context, resp *http.Response, ep *endpoint.Endpoint, ctx *RequestContext) (bool, error) {
	// 只有2xx状态码才认为是成功，其他所有状态码都尝试下一个端点
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		duration := time.Since(ctx.EndpointStartTime)
		body, _ := io.ReadAll(resp.Body)

		// 解压响应体用于日志记录
		contentEncoding := resp.Header.Get("Content-Encoding")
		decompressedBody, err := s.validator.GetDecompressedBody(body, contentEncoding)
		if err != nil {
			decompressedBody = body
		}

		// 记录错误信息到上下文
		ctx.LastStatusCode = resp.StatusCode
		ctx.LastResponseBody = string(decompressedBody)

		// 使用错误模式匹配器分析错误
		retryDecision := s.errorPatternMatcher.MakeRetryDecision(
			resp.StatusCode,
			"",
			ctx.LastResponseBody,
			ep.EndpointType,
			ctx.AttemptNumber,
		)

		s.logger.Debug("Error pattern analysis", map[string]interface{}{
			"status_code":     resp.StatusCode,
			"endpoint":        ep.Name,
			"endpoint_type":   ep.EndpointType,
			"attempt":         ctx.AttemptNumber,
			"retry_decision":  retryDecision.Action,
			"should_retry":    retryDecision.ShouldRetry,
			"reason":          retryDecision.Reason,
			"response_body":   ctx.LastResponseBody,
		})

		targetURL := ep.GetURLForFormat(ctx.EndpointRequestFormat)
		setConversionContext(c, ctx.ConversionStages)
		s.logSimpleRequest(ctx.RequestID, targetURL, c.Request.Method, ctx.Path, ctx.RequestBody, ctx.FinalRequestBody, c, nil, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(c.Request), ctx.Tags, "", ctx.OriginalModel, ctx.RewrittenModel, ctx.AttemptNumber, targetURL)
		
		// 根据错误模式匹配结果决定下一步动作
		switch retryDecision.Action {
		case "blacklist":
			s.logger.Error(fmt.Sprintf("Blacklisting endpoint %s due to error pattern: %s", ep.Name, retryDecision.Reason), fmt.Errorf("error pattern matched"))
			ep.MarkInactiveWithReason()
			return false, fmt.Errorf("endpoint %s blacklisted due to error pattern: %s", ep.Name, retryDecision.Reason)
		case "skip":
			s.logger.Debug(fmt.Sprintf("Skipping endpoint %s due to error pattern: %s", ep.Name, retryDecision.Reason))
			return false, fmt.Errorf("HTTP error %d from endpoint %s (pattern: %s)", resp.StatusCode, ep.Name, retryDecision.Reason)
		case "retry":
			if retryDecision.ShouldRetry && ctx.AttemptNumber <= retryDecision.MaxRetries {
				s.logger.Debug(fmt.Sprintf("Retrying endpoint %s after error pattern: %s (attempt %d/%d)", ep.Name, retryDecision.Reason, ctx.AttemptNumber, retryDecision.MaxRetries))
				return false, fmt.Errorf("HTTP error %d from endpoint %s - will retry (pattern: %s)", resp.StatusCode, ep.Name, retryDecision.Reason)
			}
			s.logger.Debug(fmt.Sprintf("Max retries exceeded for endpoint %s, skipping", ep.Name))
			return false, fmt.Errorf("HTTP error %d from endpoint %s (max retries exceeded)", resp.StatusCode, ep.Name)
		default:
			s.logger.Debug(fmt.Sprintf("HTTP error %d from endpoint %s, trying next endpoint", resp.StatusCode, ep.Name))
			return false, fmt.Errorf("HTTP error %d from endpoint %s", resp.StatusCode, ep.Name)
		}
	}

	// 处理成功响应
	originalContentType := resp.Header.Get("Content-Type")
	isStreamingResponse := strings.Contains(strings.ToLower(originalContentType), "text/event-stream")

	if isStreamingResponse {
		success, _, _, _ := s.handleStreamingResponse(
			c,
			resp,
			nil, // req parameter not needed for streaming
			ep,
			ctx.RequestID,
			ctx.Path,
			ctx.InboundPath,
			ctx.RequestBody,
			ctx.FinalRequestBody,
			ctx.OriginalModel,
			ctx.RewrittenModel,
			ctx.Tags,
			ctx.EndpointRequestFormat,
			ctx.ActualEndpointFormat,
			nil,   // formatDetection not needed for streaming
			false, // actuallyUsingOpenAIURL
			false, // isCountTokensRequest
			ctx.EndpointStartTime,
			ctx.AttemptNumber,
			ctx.ClientRequestFormat,
			&ctx.ConversionStages,
			ctx.FirstByteTime,
		)
		return success, nil
	}

	// 处理非流式响应
	var responseBodyBuffer bytes.Buffer
	decompressedCapture := newLimitedBuffer(responseCaptureLimit)
	teeReader := io.TeeReader(resp.Body, decompressedCapture)
	if _, err := responseBodyBuffer.ReadFrom(teeReader); err != nil {
		s.logger.Error("Failed to read response body", err)
		duration := time.Since(ctx.EndpointStartTime)
		readError := fmt.Sprintf("Failed to read response body: %v", err)
		targetURL := ep.GetURLForFormat(ctx.EndpointRequestFormat)
		setConversionContext(c, ctx.ConversionStages)
		s.logSimpleRequest(ctx.RequestID, targetURL, c.Request.Method, ctx.Path, ctx.RequestBody, ctx.FinalRequestBody, c, nil, resp, nil, duration, fmt.Errorf(readError), s.isRequestExpectingStream(c.Request), ctx.Tags, "", ctx.OriginalModel, ctx.RewrittenModel, ctx.AttemptNumber, targetURL)
		c.Set("last_error", fmt.Errorf(readError))
		c.Set("last_status_code", resp.StatusCode)
		return false, fmt.Errorf(readError)
	}
	responseBody := responseBodyBuffer.Bytes()

	// 解压响应体仅用于日志记录和验证
	contentEncoding := resp.Header.Get("Content-Encoding")
	decompressedBody, err := s.validator.GetDecompressedBody(responseBody, contentEncoding)
	if err != nil {
		s.logger.Error("Failed to decompress response body", err)
		duration := time.Since(ctx.EndpointStartTime)
		decompressError := fmt.Sprintf("Failed to decompress response body: %v", err)
		targetURL := ep.GetURLForFormat(ctx.EndpointRequestFormat)
		setConversionContext(c, ctx.ConversionStages)
		s.logSimpleRequest(ctx.RequestID, targetURL, c.Request.Method, ctx.Path, ctx.RequestBody, ctx.FinalRequestBody, c, nil, resp, responseBody, duration, fmt.Errorf(decompressError), s.isRequestExpectingStream(c.Request), ctx.Tags, "", ctx.OriginalModel, ctx.RewrittenModel, ctx.AttemptNumber, targetURL)
		c.Set("last_error", fmt.Errorf(decompressError))
		c.Set("last_status_code", resp.StatusCode)
		return false, fmt.Errorf(decompressError)
	}

	// 执行响应格式转换（如果需要）
	finalResponseBody := decompressedBody
	if ctx.NeedsConversion {
		convertedResponseBody, err := s.convertResponseBody(ctx, decompressedBody)
		if err != nil {
			s.logger.Error("Response body conversion failed", err)
			duration := time.Since(ctx.EndpointStartTime)
			targetURL := ep.GetURLForFormat(ctx.EndpointRequestFormat)
			setConversionContext(c, ctx.ConversionStages)
			s.logSimpleRequest(ctx.RequestID, targetURL, c.Request.Method, ctx.Path, ctx.RequestBody, ctx.FinalRequestBody, c, nil, resp, decompressedBody, duration, err, s.isRequestExpectingStream(c.Request), ctx.Tags, "", ctx.OriginalModel, ctx.RewrittenModel, ctx.AttemptNumber, targetURL)
			c.Set("last_error", err)
			c.Set("last_status_code", resp.StatusCode)
			return false, err
		}
		finalResponseBody = convertedResponseBody
		ctx.ConversionStages = append(ctx.ConversionStages, fmt.Sprintf("response:%s->%s", ctx.EndpointRequestFormat, ctx.ClientRequestFormat))

		s.logger.Debug("Successfully converted response body", map[string]interface{}{
			"original_size":  len(decompressedBody),
			"converted_size": len(convertedResponseBody),
		})
	}

	// 设置响应状态码和头部
	c.Status(resp.StatusCode)
	for key, values := range resp.Header {
		keyLower := strings.ToLower(key)
		if keyLower == "content-length" || keyLower == "content-encoding" {
			continue
		} else {
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}

	// 发送响应体
	c.Writer.Write(finalResponseBody)

	// 记录成功日志
	setConversionContext(c, ctx.ConversionStages)
	updateSupportsResponsesContext(c, ep)

	// 清除错误信息（成功情况）
	c.Set("last_error", nil)
	c.Set("last_status_code", resp.StatusCode)

	return true, nil
}

// convertRequestBody 转换请求体格式
func (s *Server) convertRequestBody(ctx *RequestContext) ([]byte, error) {
	if ctx.ClientRequestFormat == "anthropic" && ctx.EndpointRequestFormat == "openai" {
		// Anthropic -> OpenAI 转换
		endpointInfo := &conversion.EndpointInfo{
			Type:               "openai",
			MaxTokensFieldName: "max_tokens",
		}

		converter := conversion.NewRequestConverter(s.logger)
		convertedBody, _, err := converter.Convert(ctx.RequestBody, endpointInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Anthropic request to OpenAI format: %w", err)
		}

		s.logger.Debug("Successfully converted Anthropic request to OpenAI format", map[string]interface{}{
			"original_size":  len(ctx.RequestBody),
			"converted_size": len(convertedBody),
		})

		return convertedBody, nil
	}

	if ctx.ClientRequestFormat == "openai" && ctx.EndpointRequestFormat == "anthropic" {
		// OpenAI -> Anthropic 转换
		factory := conversion.NewAdapterFactory(nil)
		chatAdapter := factory.OpenAIChatAdapter()
		anthropicAdapter := factory.AnthropicAdapter()

		// 解析 OpenAI 请求
		internalReq, err := chatAdapter.ParseRequestJSON(ctx.RequestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OpenAI request: %w", err)
		}

		// 转换为 Anthropic 格式
		convertedBody, err := anthropicAdapter.BuildRequestJSON(internalReq)
		if err != nil {
			return nil, fmt.Errorf("failed to convert OpenAI request to Anthropic format: %w", err)
		}

		s.logger.Debug("Successfully converted OpenAI request to Anthropic format", map[string]interface{}{
			"original_size":  len(ctx.RequestBody),
			"converted_size": len(convertedBody),
		})

		return convertedBody, nil
	}

	// 不需要转换或其他格式转换
	return ctx.RequestBody, nil
}

// convertResponseBody 转换响应体格式
func (s *Server) convertResponseBody(ctx *RequestContext, responseBody []byte) ([]byte, error) {
	if ctx.EndpointRequestFormat == "openai" && ctx.ClientRequestFormat == "anthropic" {
		// OpenAI -> Anthropic 转换
		convertedBody, err := conversion.ConvertChatResponseJSONToAnthropic(responseBody)
		if err != nil {
			return nil, fmt.Errorf("failed to convert OpenAI response to Anthropic format: %w", err)
		}

		s.logger.Debug("Successfully converted OpenAI response to Anthropic format", map[string]interface{}{
			"original_size":  len(responseBody),
			"converted_size": len(convertedBody),
		})

		return convertedBody, nil
	}

	if ctx.EndpointRequestFormat == "anthropic" && ctx.ClientRequestFormat == "openai" {
		// Anthropic -> OpenAI 转换
		convertedBody, err := conversion.ConvertAnthropicResponseJSONToChat(responseBody)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Anthropic response to OpenAI format: %w", err)
		}

		s.logger.Debug("Successfully converted Anthropic response to OpenAI format", map[string]interface{}{
			"original_size":  len(responseBody),
			"converted_size": len(convertedBody),
		})

		return convertedBody, nil
	}

	// 不需要转换或其他格式转换
	return responseBody, nil
}

// proxyToEndpoint 重构后的主代理函数
func (s *Server) proxyToEndpoint(c *gin.Context, ep *endpoint.Endpoint, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest, attemptNumber int) (bool, bool, time.Duration, time.Duration) {
	// 创建请求上下文
	ctx := NewRequestContext(c, requestBody, path, attemptNumber)

	// 准备请求
	if err := s.prepareRequest(c, ep, ctx); err != nil {
		elapsed := time.Since(ctx.EndpointStartTime)
		return false, true, elapsed, 0 // 尝试下一个端点
	}

	// 确定端点格式和转换需求
	if err := s.determineEndpointFormat(c, ep, ctx); err != nil {
		elapsed := time.Since(ctx.EndpointStartTime)
		return false, true, elapsed, 0 // 尝试下一个端点
	}

	// 执行格式转换（如果需要）
	if ctx.NeedsConversion {
		s.logger.Debug("Starting request format conversion", map[string]interface{}{
			"original_format": ctx.ClientRequestFormat,
			"target_format":   ctx.EndpointRequestFormat,
			"original_size":   len(ctx.RequestBody),
			"original_body":   string(ctx.RequestBody),
		})
		
		convertedBody, err := s.convertRequestBody(ctx)
		if err != nil {
			s.logger.Error("Request body conversion failed", err)
			elapsed := time.Since(ctx.EndpointStartTime)
			return false, true, elapsed, 0 // 尝试下一个端点
		}
		ctx.FinalRequestBody = convertedBody
		ctx.ConversionStages = append(ctx.ConversionStages, fmt.Sprintf("request:%s->%s", ctx.ClientRequestFormat, ctx.EndpointRequestFormat))
		
		s.logger.Debug("Request conversion completed", map[string]interface{}{
			"converted_size": len(convertedBody),
			"converted_body": string(convertedBody),
		})
	}

	// 执行模型重写（如果配置了重写规则）
	if ep.ModelRewrite != nil && ep.ModelRewrite.Enabled && len(ep.ModelRewrite.Rules) > 0 {
		// 从请求体中提取当前模型名
		currentModel := s.extractModelFromRequest(ctx.FinalRequestBody)
		if currentModel != "" {
			ctx.OriginalModel = currentModel

			// 获取客户端类型
			clientType := ""
			if detection, exists := c.Get("format_detection"); exists {
				if det, ok := detection.(*utils.FormatDetectionResult); ok && det != nil {
					clientType = string(det.ClientType)
				}
			}

			// 应用模型重写规则
			originalModel, rewrittenModel, err := s.modelRewriter.RewriteRequestWithTags(
				&http.Request{Body: io.NopCloser(bytes.NewReader(ctx.FinalRequestBody))},
				ep.ModelRewrite,
				ep.Tags,
				clientType,
			)
			if err != nil {
				s.logger.Error("Model rewrite failed", err)
				elapsed := time.Since(ctx.EndpointStartTime)
				return false, true, elapsed, 0 // 尝试下一个端点
			}

			// 如果模型被重写，更新请求体
			if originalModel != "" && rewrittenModel != "" && originalModel != rewrittenModel {
				ctx.OriginalModel = originalModel
				ctx.RewrittenModel = rewrittenModel
				ctx.ConversionStages = append(ctx.ConversionStages, fmt.Sprintf("model:%s->%s", originalModel, rewrittenModel))

				// 更新请求体中的模型名
				updatedBody, err := s.updateModelInRequestBody(ctx.FinalRequestBody, rewrittenModel)
				if err != nil {
					s.logger.Error("Failed to update model in request body", err)
					elapsed := time.Since(ctx.EndpointStartTime)
					return false, true, elapsed, 0 // 尝试下一个端点
				}
				ctx.FinalRequestBody = updatedBody

				s.logger.Debug("Model rewritten in request", map[string]interface{}{
					"original":  originalModel,
					"rewritten": rewrittenModel,
					"endpoint":  ep.Name,
				})
			}
		}
	}

	// 执行请求
	resp, err := s.executeRequest(c, ep, ctx)
	if err != nil {
		elapsed := time.Since(ctx.EndpointStartTime)
		return false, true, elapsed, 0 // 尝试下一个端点
	}
	defer resp.Body.Close()

	// 处理响应
	success, err := s.handleResponse(c, resp, ep, ctx)
	if err != nil {
		elapsed := time.Since(ctx.EndpointStartTime)
		return false, true, elapsed, 0 // 尝试下一个端点
	}

	duration := time.Since(ctx.EndpointStartTime)
	return success, false, duration, ctx.FirstByteTime
}

// updateModelInRequestBody 更新请求体中的模型名称
func (s *Server) updateModelInRequestBody(requestBody []byte, newModel string) ([]byte, error) {
	if len(requestBody) == 0 {
		return requestBody, nil
	}

	// 解析JSON
	var requestData map[string]interface{}
	if err := jsonutils.SafeUnmarshal(requestBody, &requestData); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	// 更新model字段
	requestData["model"] = newModel

	// 重新序列化
	updatedBody, err := jsonutils.SafeMarshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated request body: %w", err)
	}

	return updatedBody, nil
}
