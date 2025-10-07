package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/tagging"
	"claude-code-codex-companion/internal/utils"

	"github.com/gin-gonic/gin"
)

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
		
		// 根据配置记录请求体内容
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.OriginalRequestBody = utils.TruncateBody(string(requestBody), 1024)
			} else {
				requestLog.OriginalRequestBody = string(requestBody)
			}
			// 同时设置RequestBody字段用于向后兼容
			requestLog.RequestBody = requestLog.OriginalRequestBody
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
func (s *Server) logSimpleRequest(requestID, endpoint, method, path string, originalRequestBody []byte, finalRequestBody []byte, c *gin.Context, req *http.Request, resp *http.Response, responseBody []byte, duration time.Duration, err error, isStreaming bool, tags []string, contentTypeOverride string, originalModel, rewrittenModel string, attemptNumber int) {
	requestLog := s.logger.CreateRequestLog(requestID, endpoint, method, path)
	requestLog.RequestBodySize = len(originalRequestBody)
	requestLog.Tags = tags
	requestLog.ContentTypeOverride = contentTypeOverride
	requestLog.AttemptNumber = attemptNumber
	
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
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.OriginalRequestBody = utils.TruncateBody(string(originalRequestBody), 1024)
				requestLog.RequestBody = requestLog.OriginalRequestBody
			} else {
				requestLog.OriginalRequestBody = string(originalRequestBody)
				requestLog.RequestBody = requestLog.OriginalRequestBody
			}
		}
	}
	
	// 记录最终请求体（如果不同于原始请求体）
	if len(finalRequestBody) > 0 && !bytes.Equal(originalRequestBody, finalRequestBody) {
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.FinalRequestBody = utils.TruncateBody(string(finalRequestBody), 1024)
			} else {
				requestLog.FinalRequestBody = string(finalRequestBody)
			}
		}
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
				
				if s.config.Logging.LogRequestBody != "none" {
					if s.config.Logging.LogRequestBody == "truncated" {
						requestLog.FinalRequestBody = utils.TruncateBody(string(finalBody), 1024)
					} else {
						requestLog.FinalRequestBody = string(finalBody)
					}
				}
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
			if s.config.Logging.LogResponseBody != "none" {
				if s.config.Logging.LogResponseBody == "truncated" {
					requestLog.OriginalResponseBody = utils.TruncateBody(string(responseBody), 1024)
					requestLog.ResponseBody = requestLog.OriginalResponseBody
				} else {
					requestLog.OriginalResponseBody = string(responseBody)
					requestLog.ResponseBody = requestLog.OriginalResponseBody
				}
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
	s.logger.LogRequest(requestLog)
}

// logBlacklistedEndpointRequest 记录对被拉黑端点的请求日志
func (s *Server) logBlacklistedEndpointRequest(requestID string, ep *endpoint.Endpoint, path string, requestBody []byte, c *gin.Context, duration time.Duration, errorMsg string, causingRequestIDs []string, attemptNumber int, taggedRequest *tagging.TaggedRequest) {
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
	
	// 设置请求标签
	if taggedRequest != nil {
		requestLog.Tags = taggedRequest.Tags
	}
	
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
		
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.OriginalRequestBody = utils.TruncateBody(string(requestBody), 1024)
			} else {
				requestLog.OriginalRequestBody = string(requestBody)
			}
			requestLog.RequestBody = requestLog.OriginalRequestBody
		}
	}
	
	s.logger.LogRequest(requestLog)
}