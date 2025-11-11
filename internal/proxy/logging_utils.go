package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/utils"

	"github.com/gin-gonic/gin"
)

const logBodyPreviewLimit = 2048

func buildBodySnapshot(data []byte) (string, string, bool) {
	if len(data) == 0 {
		return "", "", false
	}
	sum := sha256.Sum256(data)
	preview := data
	truncated := false
	if len(preview) > logBodyPreviewLimit {
		preview = preview[:logBodyPreviewLimit]
		truncated = true
	}
	return string(preview), hex.EncodeToString(sum[:]), truncated
}

// sendFailureResponse 发送失败响应
func (s *Server) sendFailureResponse(c *gin.Context, requestID string, startTime time.Time, requestBody []byte, requestTags []string, attemptedCount int, errorMsg, errorType string) {
	duration := time.Since(startTime)
	requestLog := s.logger.CreateRequestLog(requestID, "failed", c.Request.Method, c.Param("path"))
	requestLog.DurationMs = duration.Nanoseconds() / 1000000
	requestLog.StatusCode = http.StatusBadGateway

	// 记录请求头信息
	if c.Request != nil {
		requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
		requestLog.RequestHeaders = requestLog.OriginalRequestHeaders

		// 记录请求URL
		requestLog.OriginalRequestURL = c.Request.URL.String()
	}

	// 记录请求体信息
	if len(requestBody) > 0 {
		requestLog.Model = utils.ExtractModelFromRequestBody(string(requestBody))
		requestLog.RequestBodySize = len(requestBody)

		// 提取 Session ID
		requestLog.SessionID = utils.ExtractSessionIDFromRequestBody(string(requestBody))

		preview, hash, truncated := buildBodySnapshot(requestBody)
		requestLog.RequestBodyHash = hash
		requestLog.RequestBodyTruncated = truncated

		// 根据配置记录请求体内容
		if s.config.Logging.LogRequestBody != "none" {
			requestLog.OriginalRequestBody = preview
			// 同时设置RequestBody字段用于向后兼容
			requestLog.RequestBody = preview
		}
	}

	// 添加被拉黑端点的详细信息
	allEndpoints := s.endpointManager.GetAllEndpoints()
	var blacklistedEndpoints []string
	var blacklistReasons []string

	for _, ep := range allEndpoints {
		if !ep.IsAvailable() {
			blacklistReason := ep.GetBlacklistReason()

			if blacklistReason != nil {
				blacklistedEndpoints = append(blacklistedEndpoints, ep.Name)
				blacklistReasons = append(blacklistReasons,
					fmt.Sprintf("caused by requests: %v",
						blacklistReason.CausingRequestIDs))
			}
		}
	}

	if len(blacklistedEndpoints) > 0 {
		errorMsg += fmt.Sprintf(". Blacklisted endpoints: %v. Reasons: %v",
			blacklistedEndpoints, blacklistReasons)
	}

	if hints := getUpstreamHints(c); len(hints) > 0 {
		hintMsg := strings.Join(hints, " ")
		if errorMsg != "" {
			errorMsg = fmt.Sprintf("%s. %s", strings.TrimRight(errorMsg, ". "), hintMsg)
		} else {
			errorMsg = hintMsg
		}
		if requestLog.ErrorDetails == nil {
			requestLog.ErrorDetails = map[string]interface{}{}
		}
		requestLog.ErrorDetails["upstream_hints"] = hints
		requestLog.ErrorCategory = "upstream"
		requestLog.LastRetryError = hintMsg
	}

	requestLog.Tags = requestTags
	requestLog.Error = errorMsg

	// 设置格式检测信息（即使失败也要记录）
	if formatDetection, exists := c.Get("format_detection"); exists {
		if detection, ok := formatDetection.(*utils.FormatDetectionResult); ok && detection != nil {
			requestLog.ClientType = string(detection.ClientType)
			requestLog.RequestFormat = string(detection.Format)
			requestLog.DetectionConfidence = detection.Confidence
			requestLog.DetectedBy = detection.DetectedBy
		}
	}

	s.logger.LogRequest(requestLog)
	s.sendProxyError(c, http.StatusBadGateway, errorType, requestLog.Error, requestID)
}

// logSimpleRequest creates and logs a simple request log entry for error cases
func (s *Server) logSimpleRequest(requestID, endpoint, method, path string, originalRequestBody []byte, finalRequestBody []byte, c *gin.Context, req *http.Request, resp *http.Response, responseBody []byte, duration time.Duration, err error, isStreaming bool, tags []string, contentTypeOverride string, originalModel, rewrittenModel string, attemptNumber int, targetURL string) {
	requestLog := s.logger.CreateRequestLog(requestID, endpoint, method, path)
	requestLog.RequestBodySize = len(originalRequestBody)
	requestLog.Tags = tags
	requestLog.ContentTypeOverride = contentTypeOverride
	requestLog.AttemptNumber = attemptNumber
	requestLog.IsStreaming = isStreaming
	requestLog.WasStreaming = isStreaming

	// 设置 thinking 信息
	if c != nil {
		if thinkingInfo, exists := c.Get("thinking_info"); exists {
			if info, ok := thinkingInfo.(*utils.ThinkingInfo); ok && info != nil {
				requestLog.ThinkingEnabled = info.Enabled
				requestLog.ThinkingBudgetTokens = info.BudgetTokens
			}
		}

		// 设置格式检测信息
		if formatDetection, exists := c.Get("format_detection"); exists {
			if detection, ok := formatDetection.(*utils.FormatDetectionResult); ok && detection != nil {
				requestLog.ClientType = string(detection.ClientType)
				requestLog.RequestFormat = string(detection.Format)
				requestLog.DetectionConfidence = detection.Confidence
				requestLog.DetectedBy = detection.DetectedBy
			}
		}
	}

	// 记录原始客户端请求数据
	if c != nil {
		requestLog.OriginalRequestURL = c.Request.URL.String()
		requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)

	}

	if len(originalRequestBody) > 0 {
		preview, hash, truncated := buildBodySnapshot(originalRequestBody)
		requestLog.RequestBodyHash = hash
		requestLog.RequestBodyTruncated = truncated
		if s.config.Logging.LogRequestBody != "none" {
			requestLog.OriginalRequestBody = preview
			requestLog.RequestBody = preview
		}
	}

	// 记录最终请求体（如果不同于原始请求体）
	if len(finalRequestBody) > 0 && !bytes.Equal(originalRequestBody, finalRequestBody) {
		preview, hash, truncated := buildBodySnapshot(finalRequestBody)
		if s.config.Logging.LogRequestBody != "none" {
			requestLog.FinalRequestBody = preview
		}
		requestLog.RequestBodyHash = hash
		requestLog.RequestBodyTruncated = truncated
	}

	// 设置最终请求数据（发送给上游的数据）
	if req != nil {
		requestLog.FinalRequestURL = req.URL.String()
		requestLog.FinalRequestHeaders = utils.HeadersToMap(req.Header)
		requestLog.RequestHeaders = requestLog.FinalRequestHeaders

		// 尝试读取最终请求体（如果有的话）
		if req.Body != nil {
			if finalBody, err := io.ReadAll(req.Body); err == nil && len(finalBody) > 0 {
				// 重新设置请求体供后续使用
				req.Body = io.NopCloser(bytes.NewReader(finalBody))

				preview, hash, truncated := buildBodySnapshot(finalBody)
				if s.config.Logging.LogRequestBody != "none" {
					requestLog.FinalRequestBody = preview
					requestLog.RequestBody = preview
				}
				requestLog.RequestBodyHash = hash
				requestLog.RequestBodyTruncated = truncated
			}
		}
	} else if c != nil {
		// 如果没有最终请求，使用原始请求数据作为兼容
		requestLog.RequestHeaders = requestLog.OriginalRequestHeaders
	}

	// 设置响应数据
	if resp != nil {
		requestLog.OriginalResponseHeaders = utils.HeadersToMap(resp.Header)
		requestLog.ResponseHeaders = requestLog.OriginalResponseHeaders
		if len(responseBody) > 0 {
			preview, hash, truncated := buildBodySnapshot(responseBody)
			if s.config.Logging.LogResponseBody != "none" {
				requestLog.OriginalResponseBody = preview
				requestLog.ResponseBody = preview
			}
			requestLog.ResponseBodyHash = hash
			requestLog.ResponseBodyTruncated = truncated
		}
	}

	if c != nil {
		if val, exists := c.Get("conversion_path"); exists {
			if cp, ok := val.(string); ok {
				requestLog.ConversionPath = cp
			}
		}
		if val, exists := c.Get("supports_responses_flag"); exists {
			if flag, ok := val.(string); ok {
				requestLog.SupportsResponsesFlag = flag
			}
		}
	}

	// 设置模型信息和 Session ID
	if len(originalRequestBody) > 0 {
		extractedModel := utils.ExtractModelFromRequestBody(string(originalRequestBody))
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

		// 提取 Session ID
		requestLog.SessionID = utils.ExtractSessionIDFromRequestBody(string(originalRequestBody))
	}

	// 更新并记录日志
	s.logger.UpdateRequestLog(requestLog, req, resp, responseBody, duration, err)
	requestLog.IsStreaming = isStreaming

	if ue, ok := err.(*upstreamError); ok {
		if requestLog.ErrorDetails == nil {
			requestLog.ErrorDetails = map[string]interface{}{}
		}
		requestLog.ErrorDetails["type"] = "upstream_error"
		requestLog.ErrorDetails["raw_message"] = ue.rawMessage
		if ue.endpoint != "" {
			requestLog.ErrorDetails["upstream_endpoint"] = ue.endpoint
		}
		if ue.model != "" {
			requestLog.ErrorDetails["upstream_model"] = ue.model
		}
		if ue.pattern != "" {
			requestLog.ErrorDetails["matched_pattern"] = ue.pattern
		}
		if ue.action != "" {
			requestLog.ErrorDetails["retry_action"] = ue.action
		}
		requestLog.ErrorCategory = "upstream"
		requestLog.LastRetryError = ue.rawMessage
	}

	s.logger.LogRequest(requestLog)
}

// logBlacklistedEndpointRequest 记录对被拉黑端点的请求日志
func (s *Server) logBlacklistedEndpointRequest(requestID string, ep *endpoint.Endpoint, path string, requestBody []byte, c *gin.Context, duration time.Duration, errorMsg string, causingRequestIDs []string, attemptNumber int) {
	requestLog := s.logger.CreateRequestLog(requestID, ep.GetURL(), c.Request.Method, path)
	requestLog.RequestBodySize = len(requestBody)
	requestLog.AttemptNumber = attemptNumber
	requestLog.DurationMs = duration.Nanoseconds() / 1000000
	requestLog.StatusCode = http.StatusServiceUnavailable
	requestLog.Error = errorMsg

	// 设置被拉黑端点相关信息
	requestLog.BlacklistCausingRequestIDs = causingRequestIDs

	// 获取失效原因信息（使用安全的访问器方法）
	blacklistReason := ep.GetBlacklistReason()
	if blacklistReason != nil {
		requestLog.EndpointBlacklistedAt = &blacklistReason.BlacklistedAt
		requestLog.EndpointBlacklistReason = blacklistReason.ErrorSummary
	}

	// Tagging system has been removed - no tags to set

	// 记录原始请求数据
	if c.Request != nil {
		requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
		requestLog.OriginalRequestURL = c.Request.URL.String()
		requestLog.RequestHeaders = requestLog.OriginalRequestHeaders
	}

	// 记录请求体
	if len(requestBody) > 0 {
		requestLog.Model = utils.ExtractModelFromRequestBody(string(requestBody))
		requestLog.SessionID = utils.ExtractSessionIDFromRequestBody(string(requestBody))
		requestLog.RequestBodySize = len(requestBody)
		preview, hash, truncated := buildBodySnapshot(requestBody)
		requestLog.RequestBodyHash = hash
		requestLog.RequestBodyTruncated = truncated

		if s.config.Logging.LogRequestBody != "none" {
			requestLog.OriginalRequestBody = preview
			requestLog.RequestBody = preview
		}
	}

	s.logger.LogRequest(requestLog)
}
