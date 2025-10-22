package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/tagging"
	"claude-code-codex-companion/internal/utils"

	"github.com/gin-gonic/gin"
)

// RetryBehavior 定义重试行为
type RetryBehavior int

const (
	RetryBehaviorReturnError    RetryBehavior = 0 // 立刻返回错误
	RetryBehaviorRetryEndpoint  RetryBehavior = 1 // 在当前端点重试
	RetryBehaviorSwitchEndpoint RetryBehavior = 2 // 切换到下一个端点
)

// EndpointFilterResult 和相关类型定义不再需要，因为现在直接在尝试时处理被拉黑端点

// MaxEndpointRetries 单个端点最大重试次数
const MaxEndpointRetries = 2

// shouldSkipHealthRecord 根据错误类型和配置决定是否跳过健康统计记录
func (s *Server) shouldSkipHealthRecord(errorCategory ErrorCategory) bool {
	// 如果全局拉黑功能被禁用，跳过所有健康统计
	if !s.config.Blacklist.Enabled {
		return true
	}

	// 如果自动拉黑被禁用，跳过所有健康统计
	if !s.config.Blacklist.AutoBlacklist {
		return true
	}

	switch errorCategory {
	case ErrorCategoryBusinessError:
		return s.config.Blacklist.BusinessErrorSafe
	case ErrorCategoryConfigError:
		return s.config.Blacklist.ConfigErrorSafe
	case ErrorCategoryServerError:
		return s.config.Blacklist.ServerErrorSafe
	default:
		return false
	}
}

// tryProxyRequestWithRetry 尝试向端点发送请求，支持单端点重试
func (s *Server) tryProxyRequestWithRetry(c *gin.Context, ep *endpoint.Endpoint, requestBody []byte, requestID string, startTime time.Time, path string, taggedRequest *tagging.TaggedRequest, globalAttemptNumber int) (success bool, shouldTryNextEndpoint bool) {
	// 检查端点是否被拉黑，如果是则记录虚拟日志并跳过
	if !ep.IsAvailable() {
		duration := time.Since(startTime)
		blacklistReason := ep.GetBlacklistReason()
		var errorMsg string
		var causingRequestIDs []string

		if blacklistReason != nil {
			causingRequestIDs = blacklistReason.CausingRequestIDs
			errorMsg = fmt.Sprintf("Endpoint blacklisted due to previous failures. Causing request IDs: %v. Original error: %s",
				causingRequestIDs, blacklistReason.ErrorSummary)
		} else {
			errorMsg = "Endpoint is blacklisted (no detailed reason available)"
		}

		// 记录被拉黑端点的虚拟请求日志
		s.logBlacklistedEndpointRequest(requestID, ep, path, requestBody, c, duration, errorMsg, causingRequestIDs, globalAttemptNumber, taggedRequest)

		// 立即尝试下一个端点
		s.logger.Debug(fmt.Sprintf("Endpoint %s is blacklisted, skipping to next endpoint", ep.Name))
		return false, true
	}

	for endpointAttempt := 1; endpointAttempt <= MaxEndpointRetries; endpointAttempt++ {
		currentGlobalAttempt := globalAttemptNumber + endpointAttempt - 1
		s.logger.Debug(fmt.Sprintf("Trying endpoint %s (endpoint attempt %d/%d, global attempt %d)", ep.Name, endpointAttempt, MaxEndpointRetries, currentGlobalAttempt))

		success, shouldRetryAnywhere, responseTime, firstByteTime := s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime, taggedRequest, currentGlobalAttempt)
		if success {
			// 检查是否应该跳过健康统计记录
			skipHealthRecord, _ := c.Get("skip_health_record")
			if skipHealthRecord != true {
				s.endpointManager.RecordRequest(ep.ID, true, requestID, firstByteTime, responseTime)
			}

			// 尝试提取基准信息用于健康检查
			if len(requestBody) > 0 {
				extracted := s.healthChecker.GetExtractor().ExtractFromRequest(requestBody, c.Request.Header)
				if extracted {
					s.logger.Info("Successfully updated health check baseline info from request")
				}
			}

			s.logger.Debug(fmt.Sprintf("Request succeeded on endpoint %s (endpoint attempt %d/%d)", ep.Name, endpointAttempt, MaxEndpointRetries))
			return true, false
		}

		// 记录失败，但检查是否为 count_tokens 请求，如果是则不计入健康统计
		skipHealthRecord, _ := c.Get("skip_health_record")
		isCountTokensRequest := strings.Contains(path, "/count_tokens")

		// 获取错误分类以决定是否跳过健康统计
		var lastError error
		var lastStatusCode int
		if errInterface, exists := c.Get("last_error"); exists {
			if err, ok := errInterface.(error); ok {
				lastError = err
			}
		}
		if statusInterface, exists := c.Get("last_status_code"); exists {
			if status, ok := statusInterface.(int); ok {
				lastStatusCode = status
			}
		}
		errorCategory := s.categorizeError(lastError, lastStatusCode)
		shouldSkipByConfig := s.shouldSkipHealthRecord(errorCategory)

		shouldSkip := (skipHealthRecord == true) || isCountTokensRequest || shouldSkipByConfig
		if !shouldSkip {
			s.endpointManager.RecordRequest(ep.ID, false, requestID, 0, responseTime)
		}

		// 如果明确指示不应重试任何地方，直接返回
		if !shouldRetryAnywhere {
			s.logger.Debug(fmt.Sprintf("Endpoint %s indicated no retry should be attempted", ep.Name))
			return false, false
		}

		// 从context中获取最后一次的错误信息和状态码（如果有的话）
		// 重用之前声明的 lastError 和 lastStatusCode 变量

		// 根据错误类型确定重试行为
		retryBehavior := s.determineRetryBehaviorFromError(lastError, lastStatusCode, endpointAttempt)

		switch retryBehavior {
		case RetryBehaviorReturnError:
			s.logger.Debug(fmt.Sprintf("Endpoint %s: RetryBehaviorReturnError - stopping all retries", ep.Name))
			return false, false

		case RetryBehaviorRetryEndpoint:
			if endpointAttempt < MaxEndpointRetries {
				s.logger.Debug(fmt.Sprintf("Endpoint %s: RetryBehaviorRetryEndpoint - retrying same endpoint (attempt %d/%d)", ep.Name, endpointAttempt+1, MaxEndpointRetries))
				// 重新构建请求体，继续循环
				s.rebuildRequestBody(c, requestBody)
				continue
			} else {
				s.logger.Debug(fmt.Sprintf("Endpoint %s: Max retries reached, switching to next endpoint", ep.Name))
				return false, true
			}

		case RetryBehaviorSwitchEndpoint:
			s.logger.Debug(fmt.Sprintf("Endpoint %s: RetryBehaviorSwitchEndpoint - switching to next endpoint", ep.Name))
			return false, true
		}
	}

	// 如果所有重试都失败了，切换到下一个端点
	s.logger.Debug(fmt.Sprintf("All %d attempts failed on endpoint %s, switching to next endpoint", MaxEndpointRetries, ep.Name))
	return false, true
}

// ErrorCategory 错误类别
type ErrorCategory int

const (
	ErrorCategoryClientError          ErrorCategory = 0 // 4xx错误，直接切换端点
	ErrorCategoryServerError          ErrorCategory = 1 // 5xx错误，原地重试后切换端点
	ErrorCategoryNetworkError         ErrorCategory = 2 // 网络错误，应该重试
	ErrorCategoryUsageValidationError ErrorCategory = 3 // Usage验证错误，原地重试
	ErrorCategorySSEValidationError   ErrorCategory = 4 // SSE流不完整验证错误，原地重试
	ErrorCategoryOtherValidationError ErrorCategory = 5 // 其他验证错误，切换端点
	ErrorCategoryResponseTimeoutError ErrorCategory = 6 // 响应超时错误，切换端点
	ErrorCategoryBusinessError        ErrorCategory = 7 // 业务错误，根据配置决定
	ErrorCategoryConfigError          ErrorCategory = 8 // 配置错误，根据配置决定
)

// determineRetryBehaviorFromError 根据错误信息确定重试行为
func (s *Server) determineRetryBehaviorFromError(err error, statusCode int, currentAttempt int) RetryBehavior {
	if err == nil && statusCode >= 200 && statusCode < 300 {
		// 成功情况，不需要重试
		return RetryBehaviorReturnError
	}

	if ue, ok := err.(*upstreamError); ok {
		switch ue.action {
		case "retry_endpoint":
			if ue.maxRetries > 0 && currentAttempt >= ue.maxRetries {
				return RetryBehaviorSwitchEndpoint
			}
			if currentAttempt < MaxEndpointRetries {
				return RetryBehaviorRetryEndpoint
			}
			return RetryBehaviorSwitchEndpoint
		case "switch_endpoint":
			return RetryBehaviorSwitchEndpoint
		default:
			return RetryBehaviorSwitchEndpoint
		}
	}

	errorCategory := s.categorizeError(err, statusCode)

	switch errorCategory {
	case ErrorCategoryClientError:
		// 客户端错误（4xx状态码），直接尝试下一个端点
		// 修改逻辑：4xx错误现在直接切换端点，避免因提供商不正确返回4xx导致停下
		return RetryBehaviorSwitchEndpoint

	case ErrorCategoryNetworkError:
		// 网络错误（连接失败、超时等），在同一端点重试
		if currentAttempt < MaxEndpointRetries {
			return RetryBehaviorRetryEndpoint
		}
		return RetryBehaviorSwitchEndpoint

	case ErrorCategoryServerError:
		// 服务器错误（5xx状态码），在同一端点重试
		if currentAttempt < MaxEndpointRetries {
			return RetryBehaviorRetryEndpoint
		}
		return RetryBehaviorSwitchEndpoint

	case ErrorCategoryUsageValidationError:
		// Usage验证失败，原地重试
		if currentAttempt < MaxEndpointRetries {
			return RetryBehaviorRetryEndpoint
		}
		return RetryBehaviorSwitchEndpoint

	case ErrorCategorySSEValidationError:
		// SSE流不完整验证失败，原地重试
		if currentAttempt < MaxEndpointRetries {
			return RetryBehaviorRetryEndpoint
		}
		return RetryBehaviorSwitchEndpoint

	case ErrorCategoryOtherValidationError:
		// 其他验证错误，切换端点
		return RetryBehaviorSwitchEndpoint

	case ErrorCategoryResponseTimeoutError:
		// 响应超时错误，切换端点
		return RetryBehaviorSwitchEndpoint

	default:
		// 未知错误，在同一端点重试
		if currentAttempt < MaxEndpointRetries {
			return RetryBehaviorRetryEndpoint
		}
		return RetryBehaviorSwitchEndpoint
	}
}

// categorizeError 对错误进行分类
func (s *Server) categorizeError(err error, statusCode int) ErrorCategory {
	if err == nil {
		// 基于HTTP状态码判断（初步判断）
		if statusCode >= 400 && statusCode < 500 {
			return ErrorCategoryClientError
		} else if statusCode >= 500 {
			return ErrorCategoryServerError
		}
		return ErrorCategoryClientError
	}

	errStr := err.Error()

	// 🔍 优先检测服务端异常标志（即使返回 4xx 状态码）
	// 常见的服务端内部错误特征：NPE、空指针、内部异常等
	if strings.Contains(errStr, "is null") ||
		strings.Contains(errStr, "Cannot invoke") ||
		strings.Contains(errStr, "NullPointerException") ||
		strings.Contains(errStr, "null pointer") ||
		strings.Contains(errStr, "Internal Server Error") ||
		strings.Contains(errStr, "internal error") ||
		strings.Contains(errStr, "500") {
		return ErrorCategoryServerError
	}

	// 业务错误（根据配置决定是否拉黑）
	if strings.Contains(errStr, "business error:") ||
		strings.Contains(errStr, "API error:") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "quota exceeded") ||
		strings.Contains(errStr, "invalid request") {
		return ErrorCategoryBusinessError
	}

	// 配置错误（根据配置决定是否拉黑）
	if strings.Contains(errStr, "Request format conversion failed") ||
		strings.Contains(errStr, "Authentication failed") ||
		strings.Contains(errStr, "Failed to create request") ||
		strings.Contains(errStr, "Failed to create final request") ||
		strings.Contains(errStr, "Failed to read rewritten request body") ||
		strings.Contains(errStr, "Failed to decompress response body") {
		return ErrorCategoryConfigError
	}

	// Usage验证错误（原地重试）
	if strings.Contains(errStr, "Usage validation failed") ||
		strings.Contains(errStr, "invalid usage stats") {
		return ErrorCategoryUsageValidationError
	}

	// SSE流不完整验证错误（原地重试）
	if strings.Contains(errStr, "Incomplete SSE stream") ||
		strings.Contains(errStr, "incomplete SSE stream") ||
		strings.Contains(errStr, "missing message_stop") ||
		strings.Contains(errStr, "missing [DONE]") ||
		strings.Contains(errStr, "missing finish_reason") {
		return ErrorCategorySSEValidationError
	}

	// 其他验证错误（切换端点）
	if strings.Contains(errStr, "validation failed") ||
		strings.Contains(errStr, "Response format conversion failed") {
		return ErrorCategoryOtherValidationError
	}

	// 响应读取超时（切换端点）- 特殊处理
	if strings.Contains(errStr, "Failed to read response body") {
		return ErrorCategoryResponseTimeoutError
	}

	// 网络错误（应该重试）
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "Failed to create proxy client") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "dial tcp") {
		return ErrorCategoryNetworkError
	}

	// 默认为服务器错误（可以重试）
	return ErrorCategoryServerError
}

// determineRetryBehavior 根据当前情况确定重试行为（保持向后兼容）
func (s *Server) determineRetryBehavior(c *gin.Context, ep *endpoint.Endpoint, currentAttempt int) RetryBehavior {
	// 临时实现：默认在同一端点重试，最后一次尝试时切换端点
	if currentAttempt < MaxEndpointRetries {
		return RetryBehaviorRetryEndpoint
	}
	return RetryBehaviorSwitchEndpoint
}

// tryProxyRequest attempts to proxy the request to the given endpoint (保持向后兼容)
func (s *Server) tryProxyRequest(c *gin.Context, ep *endpoint.Endpoint, requestBody []byte, requestID string, startTime time.Time, path string, taggedRequest *tagging.TaggedRequest, attemptNumber int) (success, shouldRetry bool) {
	return s.tryProxyRequestWithRetry(c, ep, requestBody, requestID, startTime, path, taggedRequest, attemptNumber)
}

// tryEndpointList 尝试端点列表，返回(成功, 尝试次数)
func (s *Server) tryEndpointList(c *gin.Context, endpoints []utils.EndpointSorter, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest, phase string, startingAttemptNumber int) (bool, int) {
	totalAttempts := 0

	for _, epInterface := range endpoints {
		ep := epInterface.(*endpoint.Endpoint)
		currentGlobalAttempt := startingAttemptNumber + totalAttempts
		s.logger.Debug(fmt.Sprintf("%s: Attempting endpoint %s (starting from global attempt #%d)", phase, ep.Name, currentGlobalAttempt))

		success, shouldTryNextEndpoint := s.tryProxyRequestWithRetry(c, ep, requestBody, requestID, startTime, path, taggedRequest, currentGlobalAttempt)

		// 更新总尝试次数（包括该端点的所有重试）
		totalAttempts += MaxEndpointRetries

		if success {
			s.logger.Debug(fmt.Sprintf("%s: Request succeeded on endpoint %s", phase, ep.Name))
			return true, totalAttempts
		}

		if !shouldTryNextEndpoint {
			s.logger.Debug("Endpoint indicated no retry should be attempted, stopping fallback")
			break
		}

		s.logger.Debug(fmt.Sprintf("%s: All attempts failed on endpoint %s, trying next endpoint", phase, ep.Name))

		// 重新构建请求体
		s.rebuildRequestBody(c, requestBody)
	}

	return false, totalAttempts
}

// filterAndSortEndpoints 过滤并排序端点（包括被拉黑端点，用于在实际轮到时记录虚拟日志）
func (s *Server) filterAndSortEndpoints(allEndpoints []*endpoint.Endpoint, failedEndpoint *endpoint.Endpoint, filterFunc func(*endpoint.Endpoint) bool) []utils.EndpointSorter {
	var filtered []*endpoint.Endpoint

	for _, ep := range allEndpoints {
		// 跳过已失败的endpoint
		if ep.ID == failedEndpoint.ID {
			continue
		}
		// 跳过禁用的端点，但允许被拉黑端点进入候选列表（用于记录虚拟日志）
		if !ep.Enabled {
			continue
		}

		if filterFunc(ep) {
			filtered = append(filtered, ep)
		}
	}

	// 转换为接口类型并排序
	sorter := make([]utils.EndpointSorter, len(filtered))
	for i, ep := range filtered {
		sorter[i] = ep
	}
	utils.SortEndpointsByPriority(sorter)

	return sorter
}

// endpointContainsAllTags 检查endpoint的标签是否包含请求的所有标签
func (s *Server) endpointContainsAllTags(endpointTags, requestTags []string) bool {
	if len(requestTags) == 0 {
		return true // 无标签请求总是匹配
	}

	// 将endpoint的标签转换为map以便快速查找
	tagSet := make(map[string]bool)
	for _, tag := range endpointTags {
		tagSet[tag] = true
	}

	// 检查是否包含所有请求的标签
	for _, reqTag := range requestTags {
		if !tagSet[reqTag] {
			return false
		}
	}
	return true
}

// filterEndpointsByFormat 根据请求格式过滤兼容的端点
func (s *Server) filterEndpointsByFormat(allEndpoints []*endpoint.Endpoint, requestFormat string) []*endpoint.Endpoint {
	if requestFormat == "" || requestFormat == "unknown" {
		// 格式未知时返回所有端点（保持向后兼容）
		return allEndpoints
	}

	filtered := make([]*endpoint.Endpoint, 0)
	for _, ep := range allEndpoints {
		if s.isEndpointCompatibleWithFormat(ep, requestFormat) {
			filtered = append(filtered, ep)
		}
	}

	return filtered
}

// isEndpointCompatibleWithFormat 判断端点是否与请求格式兼容
func (s *Server) isEndpointCompatibleWithFormat(ep *endpoint.Endpoint, requestFormat string) bool {
	if !ep.Enabled {
		return false
	}

	// 端点至少需要一个可用的上游URL
	hasAnyURL := ep.URLAnthropic != "" || ep.URLOpenAI != "" || ep.URLGemini != ""
	if !hasAnyURL {
		return false
	}

	switch requestFormat {
	case "", "unknown":
		// 未检测到格式时，保持向后兼容，允许所有具备URL的端点
		return true
	case "openai":
		// OpenAI 请求可直接命中 OpenAI URL，或通过格式转换使用 Anthropic/Gemini URL
		return ep.URLOpenAI != "" || ep.URLAnthropic != "" || ep.URLGemini != ""
	case "anthropic":
		// Anthropic 请求可直接命中 Anthropic URL，或通过格式转换使用 OpenAI/Gemini URL
		return ep.URLAnthropic != "" || ep.URLOpenAI != "" || ep.URLGemini != ""
	case "gemini":
		// Gemini 请求可直接命中 Gemini URL，或通过格式转换使用 OpenAI/Anthropic URL
		return ep.URLGemini != "" || ep.URLOpenAI != "" || ep.URLAnthropic != ""
	default:
		// 未知的新格式，允许具备任意上游URL的端点尝试处理
		return true
	}
}

// fallbackToOtherEndpoints 当endpoint失败时，根据是否有tag决定fallback策略
func (s *Server) fallbackToOtherEndpoints(c *gin.Context, path string, requestBody []byte, requestID string, startTime time.Time, failedEndpoint *endpoint.Endpoint, taggedRequest *tagging.TaggedRequest) {
	// 记录失败的endpoint，但检查是否为 count_tokens 请求，如果是则不计入健康统计
	skipHealthRecord, _ := c.Get("skip_health_record")
	isCountTokensRequest := strings.Contains(path, "/count_tokens")
	shouldSkip := (skipHealthRecord == true) || isCountTokensRequest
	if !shouldSkip {
		s.endpointManager.RecordRequest(failedEndpoint.ID, false, requestID, 0, 0)
	}

	// 获取请求格式，用于过滤兼容的端点
	var requestFormat string
	if detection, exists := c.Get("format_detection"); exists {
		if det, ok := detection.(*utils.FormatDetectionResult); ok {
			requestFormat = string(det.Format)
		}
	}

	allEndpoints := s.endpointManager.GetAllEndpoints()

	// 根据请求格式过滤兼容的端点
	compatibleEndpoints := s.filterEndpointsByFormat(allEndpoints, requestFormat)
	if len(compatibleEndpoints) < len(allEndpoints) {
		s.logger.Debug(fmt.Sprintf("Filtered endpoints by format: %s, %d/%d endpoints compatible",
			requestFormat, len(compatibleEndpoints), len(allEndpoints)))
	}

	var requestTags []string
	if taggedRequest != nil {
		requestTags = taggedRequest.Tags
	}

	totalAttempted := MaxEndpointRetries // 包括最初失败的endpoint的所有重试

	if len(requestTags) > 0 {
		// 有标签请求：分两阶段尝试（只尝试格式兼容的端点）
		s.logger.Debug(fmt.Sprintf("Tagged request failed on %s, trying fallback with tags: %v and format: %s",
			failedEndpoint.Name, requestTags, requestFormat))

		// Phase 1：尝试有标签且匹配的端点（格式兼容）
		taggedEndpoints := s.filterAndSortEndpoints(compatibleEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
			return len(ep.Tags) > 0 && s.endpointContainsAllTags(ep.Tags, requestTags)
		})

		if len(taggedEndpoints) > 0 {
			s.logger.Debug(fmt.Sprintf("Phase 1: Trying %d tagged endpoints (format-compatible)", len(taggedEndpoints)))
			success, attemptedCount := s.tryEndpointList(c, taggedEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Phase 1", totalAttempted+1)
			if success {
				return
			}
			totalAttempted += attemptedCount
		}

		// Phase 2：尝试万用端点（格式兼容）
		universalEndpoints := s.filterAndSortEndpoints(compatibleEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
			return len(ep.Tags) == 0
		})

		if len(universalEndpoints) > 0 {
			s.logger.Debug(fmt.Sprintf("Phase 2: Trying %d universal endpoints", len(universalEndpoints)))
			success, attemptedCount := s.tryEndpointList(c, universalEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Phase 2", totalAttempted+1)
			if success {
				return
			}
			totalAttempted += attemptedCount
		}

		// 检查是否为 count_tokens 请求且所有失败都是因为 OpenAI 端点不支持
		isCountTokensRequest := strings.Contains(path, "/count_tokens")
		countTokensOpenAISkip, _ := c.Get("count_tokens_openai_skip")

		if isCountTokensRequest && countTokensOpenAISkip == true {
			// 所有端点都因为不支持 count_tokens 而跳过，提供估算结果
			s.respondWithEstimatedTokens(c, requestBody, requestID, requestTags)
			return
		}

		// 所有endpoint都失败了，发送错误响应但不记录额外日志（每个endpoint的失败已经记录过了）
		errorMsg := s.generateDetailedEndpointUnavailableMessage(requestID, requestTags)
		s.sendProxyError(c, http.StatusBadGateway, "all_endpoints_failed", errorMsg, requestID)

	} else {
		// 无标签请求：只尝试万用端点（格式兼容）
		s.logger.Debug(fmt.Sprintf("Untagged request failed, trying universal endpoints only (format: %s)", requestFormat))

		universalEndpoints := s.filterAndSortEndpoints(compatibleEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
			return len(ep.Tags) == 0
		})

		if len(universalEndpoints) == 0 {
			s.logger.Error(fmt.Sprintf("No format-compatible universal endpoints available for untagged request (format: %s)", requestFormat), nil)
			errorMsg := s.generateDetailedEndpointUnavailableMessage(requestID, requestTags)
			s.sendProxyError(c, http.StatusBadGateway, "no_universal_endpoints", errorMsg, requestID)
			return
		}

		s.logger.Debug(fmt.Sprintf("Trying %d universal endpoints for untagged request", len(universalEndpoints)))
		success, attemptedCount := s.tryEndpointList(c, universalEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Universal", totalAttempted+1)
		if success {
			return
		}
		totalAttempted += attemptedCount

		// 检查是否为 count_tokens 请求且所有失败都是因为 OpenAI 端点不支持
		isCountTokensRequest := strings.Contains(path, "/count_tokens")
		countTokensOpenAISkip, _ := c.Get("count_tokens_openai_skip")

		if isCountTokensRequest && countTokensOpenAISkip == true {
			// 所有端点都因为不支持 count_tokens 而跳过，提供估算
			s.respondWithEstimatedTokens(c, requestBody, requestID, nil)
			return
		}

		// 所有universal endpoint都失败了，发送错误响应但不记录额外日志（每个endpoint的失败已经记录过了）
		errorMsg := s.generateDetailedEndpointUnavailableMessage(requestID, requestTags)
		s.sendProxyError(c, http.StatusBadGateway, "all_universal_endpoints_failed", errorMsg, requestID)
	}
}

func (s *Server) respondWithEstimatedTokens(c *gin.Context, requestBody []byte, requestID string, tags []string) {
	estimate := utils.EstimateTokenCount(requestBody)
	payload := map[string]interface{}{
		"input_tokens":    estimate,
		"proxy_estimated": true,
		"detail":          "count_tokens handled locally because upstream endpoints do not support /count_tokens",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal fallback token count", err)
		s.sendProxyError(c, http.StatusInternalServerError, "count_tokens_fallback_failed", "Failed to generate fallback token count", requestID)
		return
	}

	s.logger.Info("Fallback count_tokens estimation", map[string]interface{}{
		"request_id": requestID,
		"estimate":   estimate,
		"tags":       tags,
	})

	c.Data(http.StatusOK, "application/json", body)
	c.Set("skip_health_record", true)
	c.Set("skip_logging", true)
	c.Set("last_error", nil)
	c.Set("last_status_code", http.StatusOK)
}
