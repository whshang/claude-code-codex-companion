package proxy

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/utils"
	"claude-code-codex-companion/internal/validator"
	"github.com/gin-gonic/gin"
)

// streaming.go: 流式处理模块
// 负责处理所有与 SSE (Server-Sent Events) 相关的流式响应逻辑。
//
// 目标：
// - 包含 handleStreamingResponse 函数及其所有辅助函数。
// - 集中处理流式响应的格式转换（如 OpenAI SSE -> Anthropic SSE）。
// - 管理流式响应的生命周期，包括头部设置、数据刷新和连接关闭。

// handleStreamingResponse 处理流式响应
func (s *Server) handleStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	req *http.Request,
	ep *endpoint.Endpoint,
	requestID string,
	path string,
	inboundPath string,
	requestBody []byte,
	finalRequestBody []byte,
	originalModel string,
	rewrittenModel string,
	tags []string,
	endpointRequestFormat string,
	actualEndpointFormat string,
	formatDetection *utils.FormatDetectionResult,
	actuallyUsingOpenAIURL bool,
	isCountTokensRequest bool,
	endpointStartTime time.Time,
	attemptNumber int,
	clientRequestFormat string,
	conversionStages *[]string,
	firstByteTime time.Duration,
) (bool, bool, time.Duration, time.Duration) {
	contentEncoding := resp.Header.Get("Content-Encoding")
	var reader io.Reader = resp.Body
	var gzipReader *gzip.Reader
	if s.validator.IsGzipContent(contentEncoding) {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			duration := time.Since(endpointStartTime)
			errMsg := fmt.Errorf("failed to init gzip reader: %w", err)
			if conversionStages != nil {
				setConversionContext(c, *conversionStages)
			}
			s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, nil, duration, errMsg, true, tags, "", originalModel, rewrittenModel, attemptNumber, ep.GetURLForFormat(endpointRequestFormat))
			c.Set("last_error", errMsg)
			c.Set("last_status_code", resp.StatusCode)
			return false, false, duration, 0
		}
		gzipReader = gz
		reader = gz
	}
	if gzipReader != nil {
		defer gzipReader.Close()
	}

	originalCapture := newLimitedBuffer(responseCaptureLimit)
	reader = io.TeeReader(reader, originalCapture)

	isCodexClient := formatDetection != nil && formatDetection.ClientType == utils.ClientCodex
	if isCodexClient {
		addConversionStage(conversionStages, "response:*->responses")
	}

	validationEndpointType := ep.EndpointType
	if actualEndpointFormat != "" {
		validationEndpointType = actualEndpointFormat
	}

	// 复制上游响应头（除去长度与编码）
	c.Status(resp.StatusCode)
	for key, values := range resp.Header {
		keyLower := strings.ToLower(key)
		switch keyLower {
		case "content-length", "content-encoding":
			continue
		case "content-type":
			continue
		default:
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Content-Length", "")
	c.Header("Content-Encoding", "")

	if ep.ShouldMonitorRateLimit() {
		if err := s.processRateLimitHeaders(ep, resp.Header, requestID); err != nil {
			s.logger.Error("Failed to process rate limit headers", err)
		}
	}

	captureWriter := newTeeCaptureWriter(c.Writer, responseCaptureLimit)
	outWriter := io.Writer(captureWriter)
	var streamErr error

	// 根据客户端类型和上游格式决定是否需要流式转换
	actualEndpointFormat, streamErr = s.handleStreamingConversion(formatDetection, actualEndpointFormat, reader, outWriter, ep)

	if streamErr != nil {
		duration := time.Since(endpointStartTime)
		errMsg := fmt.Errorf("streaming response failed: %w", streamErr)
		if conversionStages != nil {
			setConversionContext(c, *conversionStages)
		}
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, originalCapture.Bytes(), duration, errMsg, true, tags, "", originalModel, rewrittenModel, attemptNumber, ep.GetURLForFormat(endpointRequestFormat))
		c.Set("last_error", errMsg)
		c.Set("last_status_code", resp.StatusCode)
		return false, false, duration, 0
	}

	if actualEndpointFormat != "" {
		validationEndpointType = actualEndpointFormat
	}

	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	finalSample := captureWriter.Captured()
	originalSample := originalCapture.Bytes()

	if len(finalSample) >= responseCaptureLimit {
		s.logger.Debug("Streaming response capture truncated", map[string]interface{}{
			"endpoint":   ep.Name,
			"request_id": requestID,
		})
	}

	if len(finalSample) < responseCaptureLimit && len(finalSample) > 0 {
		if err := s.validator.ValidateResponseWithPath(finalSample, true, validationEndpointType, path, ep.GetURLForFormat(endpointRequestFormat)); err != nil {
			shouldSkip := validator.IsBusinessError(err) && s.config.Blacklist.BusinessErrorSafe
			if shouldSkip {
				s.logger.Info(fmt.Sprintf("Streaming response validation returned business error for endpoint %s: %v", ep.Name, err))
			} else {
				s.logger.Info(fmt.Sprintf("Streaming response validation failed for endpoint %s, trying next endpoint: %v", ep.Name, err))
			}
			duration := time.Since(endpointStartTime)
			if conversionStages != nil {
				setConversionContext(c, *conversionStages)
			}
			s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, finalSample, duration, err, true, tags, "", originalModel, rewrittenModel, attemptNumber, ep.GetURLForFormat(endpointRequestFormat))
			c.Set("last_error", err)
			c.Set("last_status_code", resp.StatusCode)
			if shouldSkip {
				return false, true, duration, 0
			}
			return false, true, duration, 0
		}
	}

	overrideInfo := ""
	if len(finalSample) > 0 {
		if _, info := s.validator.SmartDetectContentType(finalSample, "text/event-stream; charset=utf-8", resp.StatusCode); info != "" {
			overrideInfo = info
		}
	}

	duration := time.Since(endpointStartTime)
	if conversionStages != nil {
		setConversionContext(c, *conversionStages)
	}
	updateSupportsResponsesContext(c, ep)
	requestLog := s.logger.CreateRequestLog(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path)
	requestLog.RequestBodySize = len(requestBody)
	requestLog.Tags = tags
	requestLog.ContentTypeOverride = overrideInfo
	requestLog.AttemptNumber = attemptNumber
	requestLog.IsStreaming = true
	requestLog.WasStreaming = true

	if thinkingInfo, exists := c.Get("thinking_info"); exists {
		if info, ok := thinkingInfo.(*utils.ThinkingInfo); ok && info != nil {
			requestLog.ThinkingEnabled = info.Enabled
			requestLog.ThinkingBudgetTokens = info.BudgetTokens
		}
	}

	if formatDetection != nil {
		requestLog.ClientType = string(formatDetection.ClientType)
		requestLog.RequestFormat = string(formatDetection.Format)
		requestLog.TargetFormat = ep.EndpointType
		requestLog.FormatConverted = isCodexClient
		requestLog.DetectionConfidence = formatDetection.Confidence
		requestLog.DetectedBy = formatDetection.DetectedBy
	}
	if conversionStages != nil && len(*conversionStages) > 0 {
		requestLog.FormatConverted = true
		requestLog.ConversionPath = strings.Join(*conversionStages, conversionStageSeparator)
	}
	requestLog.SupportsResponsesFlag = getSupportsResponsesFlag(ep)
	requestLog.OriginalRequestURL = c.Request.URL.String()
	requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
	if len(requestBody) > 0 {
		if s.config.Logging.LogRequestBody != "none" {
			preview, _, _ := buildBodySnapshot(requestBody)
			requestLog.OriginalRequestBody = preview
		}
	}

	if req != nil {
		requestLog.FinalRequestURL = req.URL.String()
		requestLog.FinalRequestHeaders = utils.HeadersToMap(req.Header)
	} else {
		// For streaming responses, construct URL from endpoint info
		requestLog.FinalRequestURL = ep.GetURLForFormat(endpointRequestFormat)
		requestLog.FinalRequestHeaders = make(map[string]string)
	}
	if len(finalRequestBody) > 0 {
		preview, hash, truncated := buildBodySnapshot(finalRequestBody)
		if s.config.Logging.LogRequestBody != "none" {
			requestLog.FinalRequestBody = preview
		}
		requestLog.RequestBody = requestLog.FinalRequestBody
		requestLog.RequestBodyHash = hash
		requestLog.RequestBodyTruncated = truncated
		requestLog.RequestBodySize = len(finalRequestBody)
	} else if len(requestBody) > 0 {
		preview, hash, truncated := buildBodySnapshot(requestBody)
		if s.config.Logging.LogRequestBody != "none" && requestLog.OriginalRequestBody == "" {
			requestLog.OriginalRequestBody = preview
		}
		if requestLog.RequestBody == "" {
			requestLog.RequestBody = requestLog.OriginalRequestBody
		}
		requestLog.RequestBodyHash = hash
		requestLog.RequestBodyTruncated = truncated
	}

	requestLog.OriginalResponseHeaders = utils.HeadersToMap(resp.Header)
	if len(originalSample) > 0 && s.config.Logging.LogResponseBody != "none" {
		preview, _, _ := buildBodySnapshot(originalSample)
		requestLog.OriginalResponseBody = preview
	}

	finalHeaders := make(map[string]string)
	for key := range resp.Header {
		values := c.Writer.Header().Values(key)
		if len(values) > 0 {
			finalHeaders[key] = values[0]
		}
	}
	requestLog.FinalResponseHeaders = finalHeaders
	if len(finalSample) > 0 && s.config.Logging.LogResponseBody != "none" {
		preview, _, _ := buildBodySnapshot(finalSample)
		requestLog.FinalResponseBody = preview
	}

	requestLog.RequestHeaders = requestLog.FinalRequestHeaders
	if requestLog.RequestBody == "" {
		requestLog.RequestBody = requestLog.OriginalRequestBody
	}
	requestLog.ResponseHeaders = requestLog.FinalResponseHeaders
	if requestLog.ResponseBody == "" {
		requestLog.ResponseBody = requestLog.FinalResponseBody
	}

	if len(requestBody) > 0 {
		extractedModel := utils.ExtractModelFromRequestBody(string(requestBody))
		if originalModel != "" {
			requestLog.Model = originalModel
			requestLog.OriginalModel = originalModel
		} else {
			requestLog.Model = extractedModel
			requestLog.OriginalModel = extractedModel
		}
		if rewrittenModel != "" {
			requestLog.RewrittenModel = rewrittenModel
			requestLog.ModelRewriteApplied = rewrittenModel != requestLog.OriginalModel
		}
		requestLog.SessionID = utils.ExtractSessionIDFromRequestBody(string(requestBody))
	}

	s.logger.UpdateRequestLog(requestLog, req, resp, finalSample, duration, nil)
	s.logger.LogRequest(requestLog)

	if ep.EndpointType == "openai" && inboundPath == "/responses" && ep.NativeCodexFormat == nil {
		// 基于上游原始样本判断是否为原生 Codex /responses 流
		isResponsesNative := bytes.Contains(originalSample, []byte("response.output_text.delta")) ||
			bytes.Contains(originalSample, []byte("response.created")) ||
			bytes.Contains(originalSample, []byte("\"type\":\"response.completed\""))
		if isResponsesNative {
			trueValue := true
			ep.NativeCodexFormat = &trueValue
			updateSupportsResponsesContext(c, ep)
			s.logger.Info("Auto-detected: endpoint natively supports Codex format", map[string]interface{}{
				"endpoint": ep.Name,
			})
			if ep.OpenAIPreference == "" || ep.OpenAIPreference == "auto" {
				ep.OpenAIPreference = "responses"
				s.PersistEndpointLearning(ep)
			}
		}
	}

	if formatDetection != nil && formatDetection.ClientType == utils.ClientCodex && ep.EndpointType == "openai" {
		if inboundPath == "/responses" {
			// 使用原始样本判断是否原生Codex支持
			isResponsesNative := bytes.Contains(originalSample, []byte("response.output_text.delta")) ||
				bytes.Contains(originalSample, []byte("response.created")) ||
				bytes.Contains(originalSample, []byte("\"type\":\"response.completed\""))
			if isResponsesNative {
				s.updateEndpointCodexSupport(ep, true)
			}
		}
	} else if formatDetection != nil && formatDetection.ClientType == utils.ClientClaudeCode && ep.EndpointType == "anthropic" {
		s.updateEndpointCodexSupport(ep, false)
	}

	if isCountTokensRequest {
		ep.MarkCountTokensSupport(true)
	}

	c.Set("last_error", nil)
	c.Set("last_status_code", resp.StatusCode)

	return true, false, duration, firstByteTime
}

// handleStreamingConversion 根据客户端类型和上游格式决定流式转换策略
func (s *Server) handleStreamingConversion(formatDetection *utils.FormatDetectionResult, upstreamFormat string, reader io.Reader, writer io.Writer, ep *endpoint.Endpoint) (string, error) {
	if formatDetection == nil {
		// 无格式检测信息，直接透传
		_, err := io.Copy(writer, reader)
		return upstreamFormat, err
	}

	clientType := formatDetection.ClientType
	expectedFormat := s.getExpectedFormatForClient(clientType)

	// 如果客户端期望格式与上游格式匹配，无需转换
	if expectedFormat == upstreamFormat || expectedFormat == "" {
		_, err := io.Copy(writer, reader)
		return upstreamFormat, err
	}

	// 需要格式转换
	if err := s.convertStreamingResponse(expectedFormat, upstreamFormat, reader, writer, ep); err != nil {
		return upstreamFormat, err
	}
	return expectedFormat, nil
}

// getExpectedFormatForClient 根据客户端类型返回期望的上游格式
func (s *Server) getExpectedFormatForClient(clientType utils.ClientType) string {
	switch clientType {
	case utils.ClientCodex:
		// Codex 客户端期望 Responses API 格式，对应 openai 端点
		return "openai"
	case utils.ClientClaudeCode:
		// Claude Code 客户端期望 Anthropic 格式，对应 anthropic 端点
		return "anthropic"
	case utils.ClientGemini:
		// Gemini 客户端期望 Gemini 格式，对应 gemini 端点
		return "gemini"
	default:
		return ""
	}
}

// convertStreamingResponse 执行流式响应格式转换
func (s *Server) convertStreamingResponse(targetFormat, sourceFormat string, reader io.Reader, writer io.Writer, ep *endpoint.Endpoint) error {
	conversionKey := sourceFormat + "_to_" + targetFormat

	// 记录流式转换操作
	s.logger.Info("🔄 Streaming format conversion", map[string]interface{}{
		"conversion":    conversionKey,
		"source_format": sourceFormat,
		"target_format": targetFormat,
		"endpoint":      ep.Name,
	})

	switch conversionKey {
	case "openai_to_openai":
		// OpenAI Chat Completions SSE → OpenAI Responses SSE (Codex)
		if s.conversionManager != nil {
			_, err := s.conversionManager.ConvertStream(
				"chat_sse_to_responses",
				ep.Name,
				reader,
				writer,
				conversion.StreamChatCompletionsToResponses, // unified
				nil,
			)
			return err
		}
		return conversion.StreamChatCompletionsToResponses(reader, writer)

	case "anthropic_to_openai":
		// Anthropic SSE → OpenAI Chat Completions SSE
		return conversion.StreamAnthropicSSEToOpenAI(reader, writer)

	case "openai_to_anthropic":
		// OpenAI Chat Completions SSE → Anthropic SSE
		return conversion.StreamOpenAISSEToAnthropic(reader, writer)

	case "gemini_to_openai":
		// Gemini SSE → OpenAI Chat Completions SSE
		return conversion.StreamGeminiSSEToOpenAI(reader, writer)

	case "gemini_to_anthropic":
		// Gemini SSE → Anthropic SSE
		return conversion.StreamGeminiSSEToAnthropic(reader, writer)

	case "openai_to_gemini":
		// OpenAI Chat Completions SSE → Gemini SSE
		// TODO: Implement StreamOpenAIToGemini
		return fmt.Errorf("streaming conversion from OpenAI to Gemini not yet implemented")

	case "anthropic_to_gemini":
		// Anthropic SSE → Gemini SSE
		// TODO: Implement StreamAnthropicToGemini
		return fmt.Errorf("streaming conversion from Anthropic to Gemini not yet implemented")

	default:
		// 不支持的转换组合，返回错误
		return fmt.Errorf("unsupported streaming conversion: %s to %s", sourceFormat, targetFormat)
	}
}
