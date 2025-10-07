package proxy

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/tagging"
	"claude-code-codex-companion/internal/utils"
	"claude-code-codex-companion/internal/validator"

	"github.com/gin-gonic/gin"
)

func (s *Server) proxyToEndpoint(c *gin.Context, ep *endpoint.Endpoint, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest, attemptNumber int) (bool, bool) {
	// 检查是否为 count_tokens 请求到 OpenAI 端点
	isCountTokensRequest := strings.Contains(path, "/count_tokens")
	isOpenAIEndpoint := ep.EndpointType == "openai"

	// OpenAI 端点不支持 count_tokens，立即尝试下一个端点
	if isCountTokensRequest && isOpenAIEndpoint {
		s.logger.Debug(fmt.Sprintf("Skipping count_tokens request on OpenAI endpoint %s", ep.Name))
		// 标记这次尝试为特殊情况，不记录健康统计，不记录日志（除非所有端点都因此失败）
		c.Set("skip_health_record", true)
		c.Set("skip_logging", true)
		c.Set("count_tokens_openai_skip", true)
		c.Set("last_error", fmt.Errorf("count_tokens not supported on OpenAI endpoint"))
		c.Set("last_status_code", http.StatusNotFound)
		return false, true // 立即尝试下一个端点
	}
	// 为这个端点记录独立的开始时间
	endpointStartTime := time.Now()
	// 记录入站原始路径，与实际请求路径区分
	inboundPath := path
	effectivePath := path

	// 获取请求格式以选择对应的URL
	var formatDetection *utils.FormatDetectionResult
	if detection, exists := c.Get("format_detection"); exists {
		if det, ok := detection.(*utils.FormatDetectionResult); ok {
			formatDetection = det
		}
	}

	clientRequestFormat := ""
	if formatDetection != nil {
		clientRequestFormat = string(formatDetection.Format)
	}
	endpointRequestFormat := clientRequestFormat

	targetURL := ep.GetFullURLWithFormat(effectivePath, endpointRequestFormat)

	// Extract tags from taggedRequest
	var tags []string
	if taggedRequest != nil {
		tags = taggedRequest.Tags
	}

	// 创建HTTP请求用于模型重写处理
	tempReq, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		s.logger.Error("Failed to create request", err)
		// 记录创建请求失败的日志
		duration := time.Since(endpointStartTime)
		createRequestError := fmt.Sprintf("Failed to create request: %v", err)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, requestBody, c, nil, nil, nil, duration, fmt.Errorf(createRequestError), false, tags, "", "", "", attemptNumber)
		// 设置错误信息到context中
		c.Set("last_error", fmt.Errorf(createRequestError))
		c.Set("last_status_code", 0)
		return false, false
	}

	// 获取客户端类型
	var clientType string
	if detection, exists := c.Get("format_detection"); exists {
		if det, ok := detection.(*utils.FormatDetectionResult); ok {
			clientType = string(det.ClientType)
		}
	}

	// 应用模型重写（如果配置了）
	originalModel, rewrittenModel, err := s.modelRewriter.RewriteRequestWithTags(tempReq, ep.ModelRewrite, ep.Tags, clientType)
	if err != nil {
		s.logger.Error("Model rewrite failed", err)
		// 记录模型重写失败的日志
		duration := time.Since(endpointStartTime)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, requestBody, c, nil, nil, nil, duration, err, false, tags, "", "", "", attemptNumber)
		// 设置错误信息到context中
		c.Set("last_error", err)
		c.Set("last_status_code", 0)
		return false, false
	}

	// 如果进行了模型重写，获取重写后的请求体
	var finalRequestBody []byte
	if originalModel != "" && rewrittenModel != "" {
		finalRequestBody, err = io.ReadAll(tempReq.Body)
		if err != nil {
			s.logger.Error("Failed to read rewritten request body", err)
			duration := time.Since(endpointStartTime)
			s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, nil, nil, nil, duration, err, false, tags, "", originalModel, rewrittenModel, attemptNumber)
			// 设置错误信息到context中
			c.Set("last_error", err)
			c.Set("last_status_code", 0)
			return false, false
		}
	} else {
		finalRequestBody = requestBody // 使用原始请求体
	}

	// 格式转换（在模型重写之后）
	// 关键修复：只有当请求格式与端点格式不匹配时才需要转换
	var conversionContext *conversion.ConversionContext
	// 判断是否需要格式转换
	// 关键修复：根据实际使用的URL来判断端点格式，而不是endpoint_type
	needsConversion := false
	actualEndpointFormat := ""

	// 用于决定响应是否需要转换回客户端格式
	shouldConvertAnthropicResponseToOpenAI := false

	if formatDetection != nil && formatDetection.Format != utils.FormatUnknown {
		// 确定实际使用的URL类型
		// 更准确的URL类型判断逻辑
		if clientRequestFormat == "anthropic" && ep.URLAnthropic != "" {
			// Anthropic请求且有Anthropic URL → 使用Anthropic格式
			actualEndpointFormat = "anthropic"
		} else if clientRequestFormat == "openai" && ep.URLOpenAI != "" {
			// OpenAI请求且有OpenAI URL → 使用OpenAI格式
			actualEndpointFormat = "openai"
		} else if ep.URLAnthropic != "" {
			// 优先使用Anthropic URL
			actualEndpointFormat = "anthropic"
		} else if ep.URLOpenAI != "" {
			// 使用OpenAI URL
			actualEndpointFormat = "openai"
		} else {
			// 不应该到达这里，作为保险
			actualEndpointFormat = "openai"
		}

		// 判断是否需要格式转换
		requestIsAnthropic := (formatDetection.Format == utils.FormatAnthropic)
		requestIsOpenAI := (formatDetection.Format == utils.FormatOpenAI)
		endpointIsOpenAI := (actualEndpointFormat == "openai")

		// Anthropic格式请求 + OpenAI URL = 需要转换
		if requestIsAnthropic && endpointIsOpenAI {
			needsConversion = true
			endpointRequestFormat = "openai"
		}

		// OpenAI格式请求 + Anthropic URL = 需要转换（用于仅配置 Anthropic URL 时兼容 OpenAI 客户端）
		if requestIsOpenAI && actualEndpointFormat == "anthropic" {
			needsConversion = true
			shouldConvertAnthropicResponseToOpenAI = true
			endpointRequestFormat = "anthropic"
		}

		s.logger.Debug("Format conversion decision", map[string]interface{}{
			"request_format":         formatDetection.Format,
			"actual_endpoint_format": actualEndpointFormat,
			"needs_conversion":       needsConversion,
			"detection_confidence":   formatDetection.Confidence,
		})
	} else {
		// 没有格式检测结果，使用默认逻辑
		needsConversion = (ep.EndpointType == "openai")
	}

	actuallyUsingOpenAIURL := (actualEndpointFormat == "openai")

	if needsConversion {
		s.logger.Info(fmt.Sprintf("Starting request conversion for endpoint type: %s", ep.EndpointType))

		var convertedBody []byte
		var ctx *conversion.ConversionContext
		var err error

		if shouldConvertAnthropicResponseToOpenAI {
			// OpenAI → Anthropic 请求转换
			convertedBody, err = conversion.ConvertOpenAIRequestJSONToAnthropic(finalRequestBody)
			if err != nil {
				s.logger.Error("OpenAI to Anthropic conversion failed", err)
				duration := time.Since(endpointStartTime)
				s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, nil, nil, nil, duration, err, false, tags, "", originalModel, rewrittenModel, attemptNumber)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Request format conversion failed", "details": err.Error()})
				c.Set("last_error", err)
				c.Set("last_status_code", http.StatusBadRequest)
				return false, false
			}
			conversionContext = nil // 响应由自定义路径转换
		} else {
			// Anthropic → OpenAI 请求转换
			reqConverter := conversion.NewRequestConverter(s.logger)
			endpointInfo := &conversion.EndpointInfo{
				Type:               ep.EndpointType,
				MaxTokensFieldName: ep.MaxTokensFieldName,
			}

			convertedBody, ctx, err = reqConverter.Convert(finalRequestBody, endpointInfo)
			if err != nil {
				s.logger.Error("Request format conversion failed", err)
				duration := time.Since(endpointStartTime)
				s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, nil, nil, nil, duration, err, false, tags, "", originalModel, rewrittenModel, attemptNumber)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Request format conversion failed", "details": err.Error()})
				c.Set("last_error", err)
				c.Set("last_status_code", http.StatusBadRequest)
				return false, false
			}
			conversionContext = ctx
		}

		finalRequestBody = convertedBody

		// 如果进行了格式转换，也需要转换路径
		if clientRequestFormat == "anthropic" && actualEndpointFormat == "openai" {
			// Anthropic → OpenAI: /v1/messages → /v1/chat/completions
			if effectivePath == "/v1/messages" {
				effectivePath = "/v1/chat/completions"
				s.logger.Debug("Path converted for format conversion", map[string]interface{}{
					"original_path":  "/v1/messages",
					"converted_path": "/v1/chat/completions",
					"conversion":     "anthropic_to_openai",
				})
			}
		} else if shouldConvertAnthropicResponseToOpenAI {
			// OpenAI → Anthropic: /v1/chat/completions → /v1/messages
			if effectivePath == "/v1/chat/completions" {
				effectivePath = "/v1/messages"
				s.logger.Debug("Path converted for format conversion", map[string]interface{}{
					"original_path":  "/v1/chat/completions",
					"converted_path": "/v1/messages",
					"conversion":     "openai_to_anthropic",
				})
			}
		}

		s.logger.Debug("Request format converted successfully", map[string]interface{}{
			"endpoint_type":  ep.EndpointType,
			"original_size":  len(requestBody),
			"converted_size": len(convertedBody),
		})

		// 重新构建targetURL，因为路径可能已经改变
		targetURL = ep.GetFullURLWithFormat(effectivePath, endpointRequestFormat)
	} else {
		s.logger.Debug("Skipping format conversion (not needed)", map[string]interface{}{
			"request_format": func() string {
				if formatDetection != nil {
					return string(formatDetection.Format)
				}
				return "unknown"
			}(),
			"endpoint_type": ep.EndpointType,
		})
	}

	if shouldConvertAnthropicResponseToOpenAI {
		actuallyUsingOpenAIURL = true
	}

	// Codex /responses 格式转换为 OpenAI /chat/completions 格式
	// 智能自适应逻辑：
	// 1. 如果配置了 openai_preference：
	//    - "responses": 优先尝试原生 /responses，失败后转换 /chat/completions
	//    - "chat_completions": 直接使用 /chat/completions
	//    - "auto": 自动探测（默认行为）
	// 2. 自动探测逻辑：
	//    - NativeCodexFormat == nil: 未探测，首次请求使用原生格式，收到400后自动重试
	//    - NativeCodexFormat == true: 端点支持原生 Codex 格式，跳过转换
	//    - NativeCodexFormat == false: 端点需要 OpenAI 格式，执行转换

	codexNeedsConversion := false
	// 关键修复：检查实际使用的URL类型，而不是endpoint_type
	// 当请求格式是openai且路径是/responses时，需要考虑Codex转换
	if actuallyUsingOpenAIURL && inboundPath == "/responses" {
		// 优先使用配置的偏好设置
		preference := ep.OpenAIPreference
		if preference == "" {
			preference = "auto" // 默认为自动探测
		}

		switch preference {
		case "chat_completions":
			// 直接使用 /chat/completions 格式
			s.logger.Debug("Using chat_completions format (configured preference)", map[string]interface{}{
				"endpoint":   ep.Name,
				"preference": preference,
			})
			codexNeedsConversion = true

		case "responses":
			// 优先尝试原生 /responses 格式
			if ep.NativeCodexFormat == nil || *ep.NativeCodexFormat {
				s.logger.Debug("Trying native /responses format (configured preference)", map[string]interface{}{
					"endpoint":   ep.Name,
					"preference": preference,
				})
				codexNeedsConversion = false
			} else {
				s.logger.Debug("Converting to /chat/completions (preference=responses but native format failed)", map[string]interface{}{
					"endpoint":   ep.Name,
					"preference": preference,
				})
				codexNeedsConversion = true
			}

		case "auto":
			// 自动探测逻辑（原有逻辑）
			if ep.NativeCodexFormat == nil {
				// 首次请求，使用原生格式尝试（收到400后会自动转换并重试）
				s.logger.Info("First /responses request to endpoint, trying native Codex format (auto)", map[string]interface{}{
					"endpoint": ep.Name,
				})
				codexNeedsConversion = false
			} else if *ep.NativeCodexFormat {
				// 已探测：支持原生 Codex 格式
				s.logger.Debug("Using native Codex format (auto detected)", map[string]interface{}{
					"endpoint": ep.Name,
				})
				codexNeedsConversion = false
			} else {
				// 已探测：需要转换为 OpenAI 格式
				s.logger.Debug("Converting to OpenAI format (auto detected)", map[string]interface{}{
					"endpoint": ep.Name,
				})
				codexNeedsConversion = true
			}

		default:
			// 未知配置，使用自动探测
			s.logger.Info("Unknown openai_preference, using auto detection", map[string]interface{}{
				"endpoint":   ep.Name,
				"preference": preference,
			})
			if ep.NativeCodexFormat == nil {
				codexNeedsConversion = false
			} else if *ep.NativeCodexFormat {
				codexNeedsConversion = false
			} else {
				codexNeedsConversion = true
			}
		}
	}

	if codexNeedsConversion {
		// 将 Codex 格式转换为 OpenAI Chat Completions，并切换路径到 /chat/completions
		// 大多数 OpenAI 兼容端点（包括 88code）不支持 /responses
		if inboundPath == "/responses" {
			effectivePath = "/chat/completions"
			targetURL = ep.GetFullURLWithFormat(effectivePath, endpointRequestFormat)
		}
		convertedBody, err := s.convertCodexToOpenAI(finalRequestBody)
		if err != nil {
			s.logger.Debug("Failed to convert Codex format to OpenAI", map[string]interface{}{
				"error": err.Error(),
			})
			// 不返回错误，继续使用原始请求体
		} else if convertedBody != nil {
			finalRequestBody = convertedBody
			s.logger.Info("Codex format converted to OpenAI format", map[string]interface{}{
				"path": effectivePath,
			})

			// 调试：输出转换后的请求体（截断到前500字符）
			bodyPreview := string(convertedBody)
			if len(bodyPreview) > 500 {
				bodyPreview = bodyPreview[:500] + "..."
			}
			s.logger.Debug("Converted Codex request body", map[string]interface{}{
				"body": bodyPreview,
			})
		}
	}

	// OpenAI user 参数长度限制 hack（在格式转换之后，参数覆盖之前）
	if ep.EndpointType == "openai" {
		hackedBody, err := s.applyOpenAIUserLengthHack(finalRequestBody)
		if err != nil {
			s.logger.Debug("Failed to apply OpenAI user length hack", map[string]interface{}{
				"error": err.Error(),
			})
			// 不返回错误，继续使用原始请求体
		} else if hackedBody != nil {
			finalRequestBody = hackedBody
			s.logger.Debug("OpenAI user parameter length hack applied")
		}

		// GPT-5 模型特殊处理 hack
		// 只有当最终模型（重写后）仍然是 GPT-5 时才应用 hack
		// 如果模型被重写成其他模型（如 qwen3-coder），则跳过 hack
		finalModel := rewrittenModel
		if finalModel == "" {
			finalModel = originalModel
		}
		shouldApplyGPT5Hack := finalModel == "" || strings.Contains(strings.ToLower(finalModel), "gpt-5")

		if shouldApplyGPT5Hack {
			gpt5HackedBody, err := s.applyGPT5ModelHack(finalRequestBody)
			if err != nil {
				s.logger.Debug("Failed to apply GPT-5 model hack", map[string]interface{}{
					"error": err.Error(),
				})
				// 不返回错误，继续使用原始请求体
			} else if gpt5HackedBody != nil {
				finalRequestBody = gpt5HackedBody
				s.logger.Debug("GPT-5 model hack applied")
			}
		} else {
			s.logger.Debug("Skipping GPT-5 hack (model was rewritten)", map[string]interface{}{
				"original_model": originalModel,
				"final_model":    finalModel,
			})
		}
	}

	// 自动移除不支持的参数（基于模型名称智能检测）
	// 自动移除该端点已学习到的不支持参数
	if cleanedBody, wasModified := s.autoRemoveUnsupportedParams(finalRequestBody, ep); wasModified {
		finalRequestBody = cleanedBody
		modelForCheck := rewrittenModel
		if modelForCheck == "" {
			modelForCheck = originalModel
		}
		s.logger.Info("Auto-removed unsupported parameters based on endpoint learning", map[string]interface{}{
			"model":    modelForCheck,
			"endpoint": ep.Name,
		})
	}

	// 应用请求参数覆盖规则（在格式转换之后，创建HTTP请求之前）
	if parameterOverrides := ep.GetParameterOverrides(); parameterOverrides != nil && len(parameterOverrides) > 0 {
		overriddenBody, err := s.applyParameterOverrides(finalRequestBody, parameterOverrides)
		if err != nil {
			s.logger.Debug("Failed to apply parameter overrides", map[string]interface{}{
				"error": err.Error(),
			})
			// 不返回错误，继续使用原始请求体
		} else {
			finalRequestBody = overriddenBody
			s.logger.Info("Request parameter overrides applied", map[string]interface{}{
				"endpoint":        ep.Name,
				"overrides_count": len(parameterOverrides),
			})
		}
	}

	// 创建最终的HTTP请求
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(finalRequestBody))
	if err != nil {
		s.logger.Error("Failed to create final request", err)
		// 记录创建请求失败的日志
		duration := time.Since(endpointStartTime)
		createRequestError := fmt.Sprintf("Failed to create final request: %v", err)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, nil, nil, nil, duration, fmt.Errorf(createRequestError), false, tags, "", originalModel, rewrittenModel, attemptNumber)
		// 设置错误信息到context中
		c.Set("last_error", fmt.Errorf(createRequestError))
		c.Set("last_status_code", 0)
		return false, false
	}

	for key, values := range c.Request.Header {
		if key == "Authorization" {
			continue
		}
		// 如果已经转换 /responses -> /chat/completions，移除 responses 特有的请求头
		if codexNeedsConversion && inboundPath == "/responses" && effectivePath == "/chat/completions" {
			if key == "Openai-Beta" || key == "openai-beta" {
				s.logger.Debug("Removing /responses-specific header when converting to /chat/completions", map[string]interface{}{
					"header": key,
				})
				continue
			}
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 智能认证头设置：支持自动检测和配置两种方式
	authHeaderType := ep.AuthType

	// 如果已经通过运行时检测到有效的认证方式，优先使用
	ep.AuthHeaderMutex.RLock()
	detectedAuthHeader := ep.DetectedAuthHeader
	ep.AuthHeaderMutex.RUnlock()

	if detectedAuthHeader != "" {
		// 使用已检测到的有效认证方式
		authHeaderType = detectedAuthHeader
	}

	// 根据认证类型设置认证头部
	// 支持: api_key (x-api-key), auth_token/bearer/其他 (Authorization), auto (自动尝试)
	trimmedAuthValue := strings.TrimSpace(ep.AuthValue)
	bearerValue := ""
	if trimmedAuthValue != "" {
		bearerValue = "Bearer " + trimmedAuthValue
	}

	if authHeaderType == "api_key" || authHeaderType == "x-api-key" {
		if ep.AuthValue != "" {
			req.Header.Set("x-api-key", ep.AuthValue)
		}
		// 同步设置 Authorization，部分第三方兼容端要求同时存在
		if req.Header.Get("Authorization") == "" && bearerValue != "" {
			req.Header.Set("Authorization", bearerValue)
		}
	} else if authHeaderType == "auto" || authHeaderType == "" {
		// 自动模式：先尝试 Authorization Bearer (更常用)
		authHeader, err := ep.GetAuthHeaderWithRefreshCallback(s.config.Timeouts.ToProxyTimeoutConfig(), s.createOAuthTokenRefreshCallback())
		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to get auth header: %v", err), err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
			c.Set("last_error", err)
			c.Set("last_status_code", http.StatusUnauthorized)
			return false, false
		}
		req.Header.Set("Authorization", authHeader)
		// 标记：首次尝试使用 Authorization
		c.Set("auth_method_tried", "Authorization")
	} else {
		// auth_token, bearer, 或其他 -> 使用 Authorization
		authHeader, err := ep.GetAuthHeaderWithRefreshCallback(s.config.Timeouts.ToProxyTimeoutConfig(), s.createOAuthTokenRefreshCallback())
		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to get auth header: %v", err), err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
			c.Set("last_error", err)
			c.Set("last_status_code", http.StatusUnauthorized)
			return false, false
		}
		req.Header.Set("Authorization", authHeader)
	}

	// Special OAuth header hack for api.anthropic.com with OAuth tokens
	if strings.Contains(ep.GetURLForFormat(endpointRequestFormat), "api.anthropic.com") && ep.AuthType == "auth_token" && strings.HasPrefix(ep.AuthValue, "sk-ant-oat01") {
		if existingBeta := req.Header.Get("Anthropic-Beta"); existingBeta != "" {
			// Prepend oauth-2025-04-20 to existing Anthropic-Beta header
			req.Header.Set("Anthropic-Beta", "oauth-2025-04-20,"+existingBeta)
		} else {
			// Set oauth-2025-04-20 as the only value if no existing header
			req.Header.Set("Anthropic-Beta", "oauth-2025-04-20")
		}
	}

	// 根据格式补全必需的头信息
	if endpointRequestFormat == "anthropic" {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if req.Header.Get("anthropic-version") == "" {
			req.Header.Set("anthropic-version", "2023-06-01")
		}
		if ep.AuthValue != "" {
			req.Header.Set("x-api-key", ep.AuthValue)
		}
		if bearerValue != "" {
			req.Header.Set("Authorization", bearerValue)
		}
	} else if endpointRequestFormat == "openai" {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if req.Header.Get("Authorization") == "" {
			if authHeader, err := ep.GetAuthHeaderWithRefreshCallback(s.config.Timeouts.ToProxyTimeoutConfig(), s.createOAuthTokenRefreshCallback()); err == nil && authHeader != "" {
				if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
					authHeader = "Bearer " + strings.TrimSpace(authHeader)
				}
				req.Header.Set("Authorization", authHeader)
			} else if err == nil && bearerValue != "" {
				req.Header.Set("Authorization", bearerValue)
			}
		}
	}

	// 应用HTTP Header覆盖规则（在所有其他header处理之后）
	if headerOverrides := ep.GetHeaderOverrides(); headerOverrides != nil && len(headerOverrides) > 0 {
		for headerName, headerValue := range headerOverrides {
			if headerValue == "" {
				// 空值表示删除header
				req.Header.Del(headerName)
				s.logger.Debug(fmt.Sprintf("Header override: deleted header %s for endpoint %s", headerName, ep.Name))
			} else {
				// 非空值表示设置header
				req.Header.Set(headerName, headerValue)
				s.logger.Debug(fmt.Sprintf("Header override: set header %s = [REDACTED] for endpoint %s", headerName, ep.Name))
			}
		}
	}

	if c.Request.URL.RawQuery != "" {
		req.URL.RawQuery = c.Request.URL.RawQuery
	}

	// 为这个端点创建支持代理的HTTP客户端
	client, err := ep.CreateProxyClient(s.config.Timeouts.ToProxyTimeoutConfig())
	if err != nil {
		s.logger.Error("Failed to create proxy client for endpoint", err)
		duration := time.Since(endpointStartTime)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, nil, nil, duration, err, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		// 设置错误信息到context中
		c.Set("last_error", err)
		c.Set("last_status_code", 0)
		return false, true
	}

	resp, err := client.Do(req)
	if err != nil {
		// 如果是首次对 OpenAI 端点的 /responses 请求发生网络级错误（如 EOF），视作不支持 responses，转换并改用 /chat/completions 重试
		if ep.EndpointType == "openai" && inboundPath == "/responses" && ep.NativeCodexFormat == nil {
			s.logger.Info("Network error on first /responses request - converting to OpenAI format and retrying /chat/completions", map[string]interface{}{
				"endpoint": ep.Name,
				"error":    err.Error(),
			})
			falseValue := false
			ep.NativeCodexFormat = &falseValue
			if convertedBody, convertErr := s.convertCodexToOpenAI(requestBody); convertErr == nil && convertedBody != nil {
				// 递归重试到 /chat/completions
				return s.proxyToEndpoint(c, ep, "/chat/completions", convertedBody, requestID, startTime, taggedRequest, attemptNumber)
			}
			// 转换失败则继续按原逻辑记录并交给上层重试其他端点
		}

		duration := time.Since(endpointStartTime)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, nil, nil, duration, err, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		// 设置错误信息到context中，供重试逻辑使用
		c.Set("last_error", err)
		c.Set("last_status_code", 0) // 网络错误，没有状态码
		return false, true
	}
	defer resp.Body.Close()

	// 🔐 智能认证头检测与切换
	// 如果是认证失败(401/403)且auth_type为空或auto，标记切换认证方式后重试
	if (resp.StatusCode == 401 || resp.StatusCode == 403) &&
		(ep.AuthType == "" || ep.AuthType == "auto") {

		authRetryKey := fmt.Sprintf("auth_retry_attempted_%s", ep.ID)
		authMethodTried, _ := c.Get("auth_method_tried")

		if _, alreadyRetried := c.Get(authRetryKey); !alreadyRetried && authMethodTried == "Authorization" {
			s.logger.Info(fmt.Sprintf("Authentication failed with Authorization header for endpoint %s, will retry with x-api-key", ep.Name))

			// 标记已尝试切换认证方式，下次重试将使用x-api-key
			c.Set(authRetryKey, true)

			// 记住使用 x-api-key (下次proxyToEndpoint调用时生效)
			ep.AuthHeaderMutex.Lock()
			ep.DetectedAuthHeader = "api_key"
			ep.AuthHeaderMutex.Unlock()
			
			// 🎓 持久化学习结果：保存切换后的认证方式
			// 注意：这里只是标记，实际持久化会在重试成功后进行
			s.logger.Debug(fmt.Sprintf("Marked endpoint %s to use x-api-key, will persist if retry succeeds", ep.Name))

			// 返回失败但需要重试，让endpoint_management.go的重试逻辑处理
			duration := time.Since(endpointStartTime)
			body, _ := io.ReadAll(resp.Body)
			contentEncoding := resp.Header.Get("Content-Encoding")
			decompressedBody, err := s.validator.GetDecompressedBody(body, contentEncoding)
			if err != nil {
				decompressedBody = body
			}

			s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
			c.Set("last_error", fmt.Errorf("authentication failed, switching to x-api-key"))
			c.Set("last_status_code", resp.StatusCode)
			return false, true // 失败，但请重试同一端点
		}
	}

	// 检查认证失败情况，如果是OAuth端点且有refresh_token，先尝试刷新token
	if (resp.StatusCode == 401 || resp.StatusCode == 403) &&
		ep.AuthType == "oauth" &&
		ep.OAuthConfig != nil &&
		ep.OAuthConfig.RefreshToken != "" {

		// 检查是否已经因为这个端点的认证问题刷新过token
		refreshKey := fmt.Sprintf("oauth_refresh_attempted_%s", ep.ID)
		if _, alreadyRefreshed := c.Get(refreshKey); !alreadyRefreshed {
			s.logger.Info(fmt.Sprintf("Authentication failed (HTTP %d) for OAuth endpoint %s, attempting token refresh", resp.StatusCode, ep.Name))

			// 标记我们已经为这个端点尝试过token刷新，避免无限循环
			c.Set(refreshKey, true)

			// 尝试刷新token
			if refreshErr := ep.RefreshOAuthTokenWithCallback(s.config.Timeouts.ToProxyTimeoutConfig(), s.createOAuthTokenRefreshCallback()); refreshErr != nil {
				s.logger.Error(fmt.Sprintf("Failed to refresh OAuth token for endpoint %s: %v", ep.Name, refreshErr), refreshErr)

				// 刷新失败，读取响应体用于日志记录
				duration := time.Since(endpointStartTime)
				body, _ := io.ReadAll(resp.Body)
				contentEncoding := resp.Header.Get("Content-Encoding")
				decompressedBody, err := s.validator.GetDecompressedBody(body, contentEncoding)
				if err != nil {
					decompressedBody = body // 如果解压失败，使用原始数据
				}

				s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
				// 设置错误信息到context中
				c.Set("last_error", fmt.Errorf("OAuth token refresh failed: %v", refreshErr))
				c.Set("last_status_code", resp.StatusCode)
				return false, true
			} else {
				s.logger.Info(fmt.Sprintf("OAuth token refreshed successfully for endpoint %s, retrying request", ep.Name))

				// 关闭原始响应体
				resp.Body.Close()

				// Token刷新成功，递归重试相同的endpoint（重新走完整的请求流程）
				return s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime, taggedRequest, attemptNumber)
			}
		} else {
			s.logger.Debug(fmt.Sprintf("OAuth token refresh already attempted for endpoint %s in this request, not retrying", ep.Name))
		}
	}

	// 只有2xx状态码才认为是成功，其他所有状态码都尝试下一个端点
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		duration := time.Since(endpointStartTime)
		body, _ := io.ReadAll(resp.Body)

		// 解压响应体用于日志记录
		contentEncoding := resp.Header.Get("Content-Encoding")
		decompressedBody, err := s.validator.GetDecompressedBody(body, contentEncoding)
		if err != nil {
			decompressedBody = body // 如果解压失败，使用原始数据
		}

		// 🔍 优先级1: Codex 格式自动探测
		// 如果是首个 /responses 请求且返回 4xx/5xx（排除 401/403 认证类），
		// 视为端点不支持原生 Codex /responses：转换为 OpenAI 格式并改走 /chat/completions 重试
		// 关键：此逻辑必须在业务错误检测之前，因为404等错误也可能返回{"error":{...}}格式
		if (resp.StatusCode >= 400 && resp.StatusCode < 600 && resp.StatusCode != 401 && resp.StatusCode != 403) &&
			actuallyUsingOpenAIURL &&
			inboundPath == "/responses" &&
			ep.NativeCodexFormat == nil {

			s.logger.Info("Received error on first /responses request - endpoint requires OpenAI format", map[string]interface{}{
				"endpoint":    ep.Name,
				"status_code": resp.StatusCode,
			})

			// 标记此次尝试应跳过健康统计（这是正常的探测行为，不应该导致端点被拉黑）
			c.Set("skip_health_record", true)

			// 标记该端点不支持原生 Codex 格式，需要转换
			falseValue := false
			ep.NativeCodexFormat = &falseValue
			
			// 🎓 持久化学习结果：标记端点需要使用 chat_completions 格式
			if ep.OpenAIPreference == "" || ep.OpenAIPreference == "auto" {
				ep.OpenAIPreference = "chat_completions"
				s.logger.Debug(fmt.Sprintf("Marked endpoint %s to use chat_completions format, will persist if retry succeeds", ep.Name))
			}

			// 转换 Codex 格式到 OpenAI 格式
			convertedBody, convertErr := s.convertCodexToOpenAI(requestBody)
			if convertErr != nil {
				s.logger.Error("Failed to convert Codex format to OpenAI for retry", convertErr)
				// 转换失败，记录日志并尝试下一个端点
				s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
				c.Set("last_error", fmt.Errorf("format conversion failed: %v", convertErr))
				c.Set("last_status_code", resp.StatusCode)
				return false, true
			}

			s.logger.Info("Auto-converted to OpenAI format, retrying request", map[string]interface{}{
				"endpoint": ep.Name,
			})

			// 关闭原响应
			resp.Body.Close()

			// 用转换后的请求体重试（递归调用，会使用新的 NativeCodexFormat 配置）
			// 同时切换到 /chat/completions 路径
			// 注意：持久化会在重试成功后的 success 分支执行
			return s.proxyToEndpoint(c, ep, "/chat/completions", convertedBody, requestID, startTime, taggedRequest, attemptNumber)
		}

		// 优先级2: 检查是否为业务错误（端点正常返回了错误信息）
		var jsonResp map[string]interface{}
		if json.Unmarshal(decompressedBody, &jsonResp) == nil {
			if _, hasError := jsonResp["error"]; hasError {
				// 这是业务错误，根据配置决定是否触发端点黑名单
				shouldSkip := s.config.Blacklist.BusinessErrorSafe
				errorType := "business error"

				s.logger.Info(fmt.Sprintf("Endpoint %s returned %s with status %d (blacklist_safe=%v)", ep.Name, errorType, resp.StatusCode, shouldSkip))
				s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)

				// 根据配置决定是否跳过健康统计
				if shouldSkip {
					c.Set("skip_health_record", true)
				}

				// 设置错误信息到context中
				c.Set("last_error", fmt.Errorf("%s: status %d", errorType, resp.StatusCode))
				c.Set("last_status_code", resp.StatusCode)
				return false, true // 尝试下一个endpoint
			}
		}

		// 🎓 自动学习不支持的参数 - 基于400错误分析并重试
		if resp.StatusCode == 400 {
			// 记录学习前的参数列表长度
			paramCountBefore := len(ep.GetLearnedUnsupportedParams())

			// 尝试从错误中学习不支持的参数
			s.learnUnsupportedParamsFromError(decompressedBody, ep, finalRequestBody)

			// 如果学习到了新参数，移除它们并立即重试
			paramCountAfter := len(ep.GetLearnedUnsupportedParams())
			if paramCountAfter > paramCountBefore {
				s.logger.Info("Learned new unsupported parameters, retrying with clean request", map[string]interface{}{
					"endpoint":      ep.Name,
					"learned_count": paramCountAfter - paramCountBefore,
				})

				// 移除已学习的不支持参数
				cleanedBody, wasModified := s.autoRemoveUnsupportedParams(finalRequestBody, ep)
				if wasModified {
					// 使用清理后的请求体递归重试当前端点
					s.logger.Debug("Retrying request after removing learned unsupported parameters")
					return s.proxyToEndpoint(c, ep, path, cleanedBody, requestID, startTime, taggedRequest, attemptNumber)
				}
			}
		}

		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		s.logger.Debug(fmt.Sprintf("HTTP error %d from endpoint %s, trying next endpoint", resp.StatusCode, ep.Name))
		// 设置状态码到context中，供重试逻辑使用
		c.Set("last_error", nil)
		c.Set("last_status_code", resp.StatusCode)
		return false, true
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", err)
		// 记录读取响应体失败的日志
		duration := time.Since(endpointStartTime)
		readError := fmt.Sprintf("Failed to read response body: %v", err)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, nil, duration, fmt.Errorf(readError), s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		// 设置错误信息到context中
		c.Set("last_error", fmt.Errorf(readError))
		c.Set("last_status_code", resp.StatusCode)
		return false, false
	}

	// 解压响应体仅用于日志记录和验证
	contentEncoding := resp.Header.Get("Content-Encoding")
	decompressedBody, err := s.validator.GetDecompressedBody(responseBody, contentEncoding)
	if err != nil {
		s.logger.Error("Failed to decompress response body", err)
		// 记录解压响应体失败的日志
		duration := time.Since(endpointStartTime)
		decompressError := fmt.Sprintf("Failed to decompress response body: %v", err)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, responseBody, duration, fmt.Errorf(decompressError), s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		// 设置错误信息到context中
		c.Set("last_error", fmt.Errorf(decompressError))
		c.Set("last_status_code", resp.StatusCode)
		return false, false
	}

	contentTypeOverrideFromConversion := ""

	// 智能检测内容类型并自动覆盖
	currentContentType := resp.Header.Get("Content-Type")
	newContentType, overrideInfo := s.validator.SmartDetectContentType(decompressedBody, currentContentType, resp.StatusCode)

	// 确定最终的Content-Type和是否为流式响应
	finalContentType := currentContentType
	if newContentType != "" {
		finalContentType = newContentType
		s.logger.Info(fmt.Sprintf("Auto-detected content type mismatch for endpoint %s: %s", ep.Name, overrideInfo))
	}
	if contentTypeOverrideFromConversion != "" {
		finalContentType = contentTypeOverrideFromConversion
	}

	// 判断是否为流式响应（基于最终的Content-Type）
	isStreaming := strings.Contains(strings.ToLower(finalContentType), "text/event-stream")

	// 添加调试日志
	if len(decompressedBody) > 0 && len(decompressedBody) < 500 {
		s.logger.Debug(fmt.Sprintf("Response from %s - ContentType: %s, IsStreaming: %v, BodyPreview: %s",
			ep.Name, finalContentType, isStreaming, string(decompressedBody)))
	} else if len(decompressedBody) > 0 {
		s.logger.Debug(fmt.Sprintf("Response from %s - ContentType: %s, IsStreaming: %v, BodySize: %d, BodyPreview: %s...",
			ep.Name, finalContentType, isStreaming, len(decompressedBody), string(decompressedBody[:200])))
	}

	// 复制响应头，但跳过可能需要重新计算的头部
	for key, values := range resp.Header {
		keyLower := strings.ToLower(key)
		if keyLower == "content-type" {
			c.Header(key, finalContentType)
		} else if keyLower == "content-length" || keyLower == "content-encoding" {
			// 这些头部会在后面根据最终响应体重新设置
			continue
		} else {
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}

	// 监控Anthropic rate limit headers
	if ep.ShouldMonitorRateLimit() {
		if err := s.processRateLimitHeaders(ep, resp.Header, requestID); err != nil {
			s.logger.Error("Failed to process rate limit headers", err)
		}
	}

	// 严格 Anthropic 格式验证已永久启用
	// 关键修复：使用actualEndpointFormat进行验证，而不是ep.EndpointType
	validationEndpointType := ep.EndpointType
	if actualEndpointFormat != "" {
		validationEndpointType = actualEndpointFormat
	}
	if err := s.validator.ValidateResponseWithPath(decompressedBody, isStreaming, validationEndpointType, path, ep.GetURLForFormat(endpointRequestFormat)); err != nil {
		// 检查是否为业务错误（根据配置决定是否触发端点黑名单）
		if validator.IsBusinessError(err) {
			shouldSkip := s.config.Blacklist.BusinessErrorSafe
			errorType := "business error"

			s.logger.Info(fmt.Sprintf("Endpoint %s returned %s (not endpoint failure): %v (blacklist_safe=%v)", ep.Name, errorType, err, shouldSkip))
			duration := time.Since(endpointStartTime)
			errorLog := fmt.Sprintf("%s: %v", errorType, err)
			s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, fmt.Errorf(errorLog), isStreaming, tags, "", originalModel, rewrittenModel, attemptNumber)

			// 根据配置决定是否跳过健康统计
			if shouldSkip {
				c.Set("skip_health_record", true)
			}

			// 设置错误信息到context中
			c.Set("last_error", fmt.Errorf(errorLog))
			c.Set("last_status_code", resp.StatusCode)
			return false, true // 尝试下一个endpoint
		}

		// 如果是usage统计验证失败，尝试下一个endpoint
		if strings.Contains(err.Error(), "invalid usage stats") {
			s.logger.Info(fmt.Sprintf("Usage validation failed for endpoint %s: %v", ep.Name, err))
			duration := time.Since(endpointStartTime)
			errorLog := fmt.Sprintf("Usage validation failed: %v", err)
			s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, append(decompressedBody, []byte(errorLog)...), duration, fmt.Errorf(errorLog), s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
			// 设置错误信息到context中
			c.Set("last_error", fmt.Errorf(errorLog))
			c.Set("last_status_code", resp.StatusCode)
			return false, true // 验证失败，尝试下一个endpoint
		}

		// 如果是SSE流不完整的验证失败，检查配置决定是否触发端点黑名单
		if strings.Contains(err.Error(), "incomplete SSE stream") || strings.Contains(err.Error(), "missing message_stop") || strings.Contains(err.Error(), "missing [DONE]") || strings.Contains(err.Error(), "missing finish_reason") {
			shouldSkip := s.config.Blacklist.SSEValidationSafe
			errorType := "SSE validation error"

			s.logger.Info(fmt.Sprintf("Endpoint %s returned %s: %v (blacklist_safe=%v)", ep.Name, errorType, err, shouldSkip))
			duration := time.Since(endpointStartTime)
			errorLog := fmt.Sprintf("%s: %v", errorType, err)
			s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, append(decompressedBody, []byte(errorLog)...), duration, fmt.Errorf(errorLog), s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)

			// 根据配置决定是否跳过健康统计
			if shouldSkip {
				c.Set("skip_health_record", true)
			}

			// 设置错误信息到context中
			c.Set("last_error", fmt.Errorf(errorLog))
			c.Set("last_status_code", resp.StatusCode)
			return false, true // 验证失败，尝试下一个endpoint
		}

		// 验证失败，尝试下一个端点
		s.logger.Info(fmt.Sprintf("Response validation failed for endpoint %s, trying next endpoint: %v", ep.Name, err))
		duration := time.Since(endpointStartTime)
		validationError := fmt.Sprintf("Response validation failed: %v", err)
		s.logSimpleRequest(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, fmt.Errorf(validationError), isStreaming, tags, "", originalModel, rewrittenModel, attemptNumber)
		// 设置错误信息到context中
		c.Set("last_error", fmt.Errorf(validationError))
		c.Set("last_status_code", resp.StatusCode)
		return false, true // 验证失败，尝试下一个endpoint
	}

	c.Status(resp.StatusCode)

	// 格式转换（在模型重写之前）
	// 注意：conversionContext 仅在 Anthropic → OpenAI 请求转换时设置
	// 此时不需要 OpenAI → Anthropic 响应转换，因为已经有专门的转换逻辑
	convertedResponseBody := decompressedBody
	if conversionContext != nil {
		s.logger.Info("conversionContext is set but OpenAI to Anthropic response conversion is deprecated")
		// 在新架构中，这个分支不应该被执行
		// 如果执行到这里，说明代码逻辑可能有问题
	}

	if shouldConvertAnthropicResponseToOpenAI && len(decompressedBody) > 0 {
		var converted []byte
		var err error

		if isStreaming {
			converted, err = conversion.ConvertAnthropicSSEToOpenAI(decompressedBody)
		} else {
			converted, err = conversion.ConvertAnthropicResponseJSONToOpenAI(decompressedBody)
		}

		if err != nil {
			s.logger.Error("Failed to convert Anthropic response to OpenAI format", err)
		} else {
			convertedResponseBody = converted
			actuallyUsingOpenAIURL = true

			// 确保响应头与新的数据格式匹配
			c.Header("Content-Encoding", "")
			if !isStreaming {
				c.Header("Content-Length", fmt.Sprintf("%d", len(convertedResponseBody)))
			} else {
				c.Header("Content-Length", "")
			}

			if isStreaming {
				contentTypeOverrideFromConversion = "text/event-stream; charset=utf-8"
			} else {
				contentTypeOverrideFromConversion = "application/json"
			}
		}
	}

	// 应用响应模型重写（如果进行了请求模型重写）
	finalResponseBody := convertedResponseBody
	if originalModel != "" && rewrittenModel != "" {
		rewrittenResponseBody, err := s.modelRewriter.RewriteResponse(convertedResponseBody, originalModel, rewrittenModel)
		if err != nil {
			s.logger.Error("Failed to rewrite response model", err)
			// 如果响应重写失败，使用转换后的响应体，不中断请求
		} else if len(rewrittenResponseBody) > 0 && !bytes.Equal(rewrittenResponseBody, convertedResponseBody) {
			// 如果响应重写成功且内容发生了变化，发送重写后的未压缩响应
			// 并移除Content-Encoding头（因为我们发送的是未压缩数据）
			c.Header("Content-Encoding", "")
			c.Header("Content-Length", fmt.Sprintf("%d", len(rewrittenResponseBody)))
			finalResponseBody = rewrittenResponseBody
		} else {
			// 如果没有重写或重写后内容没变化，使用转换后的响应体
			finalResponseBody = convertedResponseBody
		}
	} else if conversionContext != nil {
		// 只有格式转换没有模型重写的情况
		finalResponseBody = convertedResponseBody
	}

	// 设置正确的响应头部
	if conversionContext != nil || (originalModel != "" && rewrittenModel != "") {
		// 如果进行了转换或模型重写，需要重新设置头部
		// 移除压缩编码（因为我们发送的是解压后的数据）
		c.Header("Content-Encoding", "")
		// 设置正确的内容长度
		c.Header("Content-Length", fmt.Sprintf("%d", len(finalResponseBody)))
	}

	// 如果是流式响应，确保设置正确的SSE头部
	if isStreaming {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no") // 防止中间层缓冲
		// 移除Content-Length头部（SSE不应该设置这个）
		c.Header("Content-Length", "")

		// Codex /responses API 格式转换
		// Codex 客户端期望 Responses API 的 SSE 事件格式（type: response.created/response.output_text.delta/response.completed）
		// 而不是 Chat Completions 的格式（object: chat.completion.chunk）
		formatDetection, _ := c.Get("format_detection")
		isCodexClient := false
		if fd, ok := formatDetection.(*utils.FormatDetectionResult); ok {
			isCodexClient = (fd.ClientType == utils.ClientCodex)
		}

		// 关键修复：检查actuallyUsingOpenAIURL而不是ep.EndpointType
		if actuallyUsingOpenAIURL && isCodexClient {
			s.logger.Info("Converting chat completions SSE to Responses API format for Codex", map[string]interface{}{
				"actual_endpoint_format": actualEndpointFormat,
				"client_type":            "codex",
				"path":                   path,
			})
			finalResponseBody = s.convertChatCompletionsToResponsesSSE(finalResponseBody)
		}
	} else {
		// 非流式响应的Codex格式转换
		formatDetection, _ := c.Get("format_detection")
		isCodexClient := false
		if fd, ok := formatDetection.(*utils.FormatDetectionResult); ok {
			isCodexClient = (fd.ClientType == utils.ClientCodex)
		}

		// 如果是Codex客户端且使用OpenAI URL，需要转换非流式响应
		if actuallyUsingOpenAIURL && isCodexClient {
			s.logger.Info("Converting chat completion to Responses API format for Codex", map[string]interface{}{
				"actual_endpoint_format": actualEndpointFormat,
				"client_type":            "codex",
				"path":                   path,
			})
			if converted, err := s.convertChatCompletionToResponse(finalResponseBody); err == nil {
				finalResponseBody = converted
				// 更新Content-Length
				c.Header("Content-Length", fmt.Sprintf("%d", len(finalResponseBody)))
			} else {
				s.logger.Error("Failed to convert response format for Codex", err)
			}
		}
	}

	// 发送最终响应体给客户端
	c.Writer.Write(finalResponseBody)

	// 清除错误信息（成功情况）
	c.Set("last_error", nil)
	c.Set("last_status_code", resp.StatusCode)

	duration := time.Since(endpointStartTime)
	// 创建日志条目，记录修改前后的完整数据
	requestLog := s.logger.CreateRequestLog(requestID, ep.GetURLForFormat(endpointRequestFormat), c.Request.Method, path)
	requestLog.RequestBodySize = len(requestBody)
	requestLog.Tags = tags
	requestLog.ContentTypeOverride = overrideInfo
	requestLog.AttemptNumber = attemptNumber

	// 设置 thinking 信息
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
			requestLog.TargetFormat = ep.EndpointType
			requestLog.FormatConverted = (conversionContext != nil)
			requestLog.DetectionConfidence = detection.Confidence
			requestLog.DetectedBy = detection.DetectedBy
		}
	}

	// 记录原始客户端请求数据
	requestLog.OriginalRequestURL = c.Request.URL.String()
	requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
	if len(requestBody) > 0 {
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.OriginalRequestBody = utils.TruncateBody(string(requestBody), 1024)
			} else {
				requestLog.OriginalRequestBody = string(requestBody)
			}
		}
	}

	// 记录最终发送给上游的请求数据
	requestLog.FinalRequestURL = req.URL.String()
	requestLog.FinalRequestHeaders = utils.HeadersToMap(req.Header)
	if len(finalRequestBody) > 0 {
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.FinalRequestBody = utils.TruncateBody(string(finalRequestBody), 1024)
			} else {
				requestLog.FinalRequestBody = string(finalRequestBody)
			}
		}
	}

	// 记录上游原始响应数据
	requestLog.OriginalResponseHeaders = utils.HeadersToMap(resp.Header)
	if len(decompressedBody) > 0 {
		if s.config.Logging.LogResponseBody != "none" {
			if s.config.Logging.LogResponseBody == "truncated" {
				requestLog.OriginalResponseBody = utils.TruncateBody(string(decompressedBody), 1024)
			} else {
				requestLog.OriginalResponseBody = string(decompressedBody)
			}
		}
	}

	// 记录最终发送给客户端的响应数据
	finalHeaders := make(map[string]string)
	for key := range resp.Header {
		values := c.Writer.Header().Values(key)
		if len(values) > 0 {
			finalHeaders[key] = values[0]
		}
	}
	requestLog.FinalResponseHeaders = finalHeaders
	if len(finalResponseBody) > 0 {
		if s.config.Logging.LogResponseBody != "none" {
			if s.config.Logging.LogResponseBody == "truncated" {
				requestLog.FinalResponseBody = utils.TruncateBody(string(finalResponseBody), 1024)
			} else {
				requestLog.FinalResponseBody = string(finalResponseBody)
			}
		}
	}

	// 动态API格式学习 - 根据成功响应更新端点格式偏好
	if formatDetection != nil && formatDetection.ClientType == utils.ClientCodex && ep.EndpointType == "openai" {
		// 只有当 /responses 路径成功时，才标记端点支持原生 Codex 格式
		// /chat/completions 成功不代表支持 /responses
		if inboundPath == "/responses" {
			s.updateEndpointCodexSupport(ep, true)
		}
	} else if formatDetection != nil && formatDetection.ClientType == utils.ClientClaudeCode && ep.EndpointType == "anthropic" {
		// 检测到Claude Code请求成功通过Anthropic端点，确认端点支持
		s.updateEndpointCodexSupport(ep, false)
	}

	// 设置兼容性字段
	requestLog.RequestHeaders = requestLog.FinalRequestHeaders
	requestLog.RequestBody = requestLog.OriginalRequestBody
	requestLog.ResponseHeaders = requestLog.OriginalResponseHeaders
	requestLog.ResponseBody = requestLog.OriginalResponseBody

	// 设置模型信息
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

		// 提取 Session ID
		requestLog.SessionID = utils.ExtractSessionIDFromRequestBody(string(requestBody))
	}

	// 更新基本字段
	s.logger.UpdateRequestLog(requestLog, req, resp, decompressedBody, duration, nil)
	requestLog.IsStreaming = isStreaming
	s.logger.LogRequest(requestLog)

	// 🔍 自动探测成功：如果是首次 /responses 请求且成功，标记为支持原生 Codex 格式
	if ep.EndpointType == "openai" && inboundPath == "/responses" && ep.NativeCodexFormat == nil {
		trueValue := true
		ep.NativeCodexFormat = &trueValue
		s.logger.Info("Auto-detected: endpoint natively supports Codex format", map[string]interface{}{
			"endpoint": ep.Name,
		})
		
		// 🎓 持久化学习结果：标记 OpenAI 格式偏好为 responses
		if ep.OpenAIPreference == "" || ep.OpenAIPreference == "auto" {
			ep.OpenAIPreference = "responses"
			s.PersistEndpointLearning(ep)
		}
	}

	// 🔐 记录成功的认证方式（用于自动模式）
	if ep.AuthType == "" || ep.AuthType == "auto" {
		ep.AuthHeaderMutex.RLock()
		currentDetected := ep.DetectedAuthHeader
		ep.AuthHeaderMutex.RUnlock()

		if currentDetected == "" {
			// 第一次成功，记住使用的认证方式
			authMethodTried, _ := c.Get("auth_method_tried")
			if authMethodTried == "Authorization" {
				ep.AuthHeaderMutex.Lock()
				ep.DetectedAuthHeader = "auth_token"
				ep.AuthHeaderMutex.Unlock()
				s.logger.Info(fmt.Sprintf("Auto-detected: endpoint %s works with Authorization header", ep.Name))
				
				// 🎓 持久化学习结果：保存认证方式
				s.PersistEndpointLearning(ep)
			}
		}
	}

	return true, false
}

// applyParameterOverrides 应用请求参数覆盖规则
// autoRemoveUnsupportedParams 基于端点学习到的信息自动移除不支持的参数
func (s *Server) autoRemoveUnsupportedParams(requestBody []byte, ep *endpoint.Endpoint) ([]byte, bool) {
	// 获取端点学习到的不支持参数列表
	unsupportedParams := ep.GetLearnedUnsupportedParams()
	if len(unsupportedParams) == 0 {
		return requestBody, false
	}

	// 解析请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		return requestBody, false
	}

	// 移除学习到的不支持参数
	modified := false
	for _, param := range unsupportedParams {
		if _, exists := requestData[param]; exists {
			delete(requestData, param)
			modified = true
			s.logger.Debug(fmt.Sprintf("Auto-removed '%s' parameter (learned from previous failures)", param))
		}
	}

	if !modified {
		return requestBody, false
	}

	// 重新序列化
	modifiedBody, err := json.Marshal(requestData)
	if err != nil {
		return requestBody, false
	}

	return modifiedBody, true
}

func (s *Server) applyParameterOverrides(requestBody []byte, parameterOverrides map[string]string) ([]byte, error) {
	if len(parameterOverrides) == 0 {
		return requestBody, nil
	}

	// 解析JSON请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		// 如果解析失败，记录日志但不返回错误，使用原始请求体
		s.logger.Debug("Failed to parse request body as JSON for parameter override, using original body", map[string]interface{}{
			"error": err.Error(),
		})
		return requestBody, nil
	}

	// 应用参数覆盖规则
	modified := false
	for paramName, paramValue := range parameterOverrides {
		if paramValue == "" {
			// 空值表示删除参数
			if _, exists := requestData[paramName]; exists {
				delete(requestData, paramName)
				modified = true
				s.logger.Debug(fmt.Sprintf("Parameter override: deleted parameter %s", paramName))
			}
		} else {
			// 非空值表示设置参数
			// 尝试解析参数值为适当的类型
			var parsedValue interface{}
			if err := json.Unmarshal([]byte(paramValue), &parsedValue); err != nil {
				// 如果JSON解析失败，作为字符串处理
				parsedValue = paramValue
			}
			requestData[paramName] = parsedValue
			modified = true
			s.logger.Debug(fmt.Sprintf("Parameter override: set parameter %s = %v", paramName, parsedValue))
		}
	}

	// 如果没有修改，返回原始请求体
	if !modified {
		return requestBody, nil
	}

	// 重新序列化为JSON
	modifiedBody, err := json.Marshal(requestData)
	if err != nil {
		s.logger.Error("Failed to marshal modified request body", err)
		return requestBody, nil // 返回原始请求体
	}

	return modifiedBody, nil
}

// applyOpenAIUserLengthHack 应用 OpenAI user 参数长度限制 hack
func (s *Server) applyOpenAIUserLengthHack(requestBody []byte) ([]byte, error) {
	// 解析JSON请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		// 如果解析失败，记录日志但不返回错误，使用原始请求体
		s.logger.Debug("Failed to parse request body as JSON for OpenAI user hack, using original body", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, nil
	}

	// 检查是否存在 user 参数
	userValue, exists := requestData["user"]
	if !exists {
		return nil, nil // 没有 user 参数，无需处理
	}

	// 转换为字符串
	userStr, ok := userValue.(string)
	if !ok {
		return nil, nil // user 参数不是字符串，无需处理
	}

	// 检查长度（以字节为单位）
	if len(userStr) <= 64 {
		return nil, nil // 长度在限制内，无需处理
	}

	// 生成 hash
	hasher := md5.New()
	hasher.Write([]byte(userStr))
	hashBytes := hasher.Sum(nil)
	hashStr := hex.EncodeToString(hashBytes)

	// 添加前缀标识
	hashedUser := "hashed-" + hashStr

	// 更新请求数据
	requestData["user"] = hashedUser

	s.logger.Info("OpenAI user parameter hashed due to length limit", map[string]interface{}{
		"original_length": len(userStr),
		"hashed_length":   len(hashedUser),
		"original_user":   userStr[:min(32, len(userStr))] + "...", // 只记录前32个字符用于调试
	})

	// 重新序列化为JSON
	modifiedBody, err := json.Marshal(requestData)
	if err != nil {
		s.logger.Error("Failed to marshal request body after user hash", err)
		return nil, err
	}

	return modifiedBody, nil
}

// applyGPT5ModelHack 应用 GPT-5 模型特殊处理 hack
// 如果模型名包含 "gpt5" 且端点是 OpenAI 类型，则：
// 1. 如果 temperature 不是 1 则将其改为 1
// 2. 如果包含 max_tokens 字段，则将其改名为 max_completion_tokens
func (s *Server) applyGPT5ModelHack(requestBody []byte) ([]byte, error) {
	// 解析JSON请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		// 如果解析失败，记录日志但不返回错误，使用原始请求体
		s.logger.Debug("Failed to parse request body as JSON for GPT-5 hack, using original body", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, nil
	}

	// 检查是否为 GPT-5 模型
	modelValue, exists := requestData["model"]
	if !exists {
		return nil, nil // 没有 model 参数，无需处理
	}

	modelStr, ok := modelValue.(string)
	if !ok {
		return nil, nil // model 参数不是字符串，无需处理
	}

	// 检查模型名是否包含 "gpt-5"（不区分大小写）
	if !strings.Contains(strings.ToLower(modelStr), "gpt-5") {
		return nil, nil // 不是 GPT-5 模型，无需处理
	}

	modified := false
	var hackDetails []string

	// 1. 检查并修改 temperature
	if tempValue, exists := requestData["temperature"]; exists {
		if temp, ok := tempValue.(float64); ok && temp != 1.0 {
			requestData["temperature"] = 1.0
			modified = true
			hackDetails = append(hackDetails, fmt.Sprintf("temperature: %.3f → 1.0", temp))
		}
	} else {
		// 如果没有 temperature，设置为 1.0
		requestData["temperature"] = 1.0
		modified = true
		hackDetails = append(hackDetails, "temperature: not set → 1.0")
	}

	// 2. 检查并重命名 max_tokens 为 max_completion_tokens
	if maxTokensValue, exists := requestData["max_tokens"]; exists {
		// 将 max_tokens 改名为 max_completion_tokens
		requestData["max_completion_tokens"] = maxTokensValue
		delete(requestData, "max_tokens")
		modified = true
		hackDetails = append(hackDetails, fmt.Sprintf("max_tokens → max_completion_tokens: %v", maxTokensValue))
	}

	// 如果没有修改，返回 nil
	if !modified {
		return nil, nil
	}

	s.logger.Info("GPT-5 model hack applied", map[string]interface{}{
		"model":   modelStr,
		"changes": hackDetails,
	})

	// 重新序列化为JSON
	modifiedBody, err := json.Marshal(requestData)
	if err != nil {
		s.logger.Error("Failed to marshal request body after GPT-5 hack", err)
		return nil, err
	}

	return modifiedBody, nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// processRateLimitHeaders 处理Anthropic rate limit headers
func (s *Server) processRateLimitHeaders(ep *endpoint.Endpoint, headers http.Header, requestID string) error {
	resetHeader := headers.Get("Anthropic-Ratelimit-Unified-Reset")
	statusHeader := headers.Get("Anthropic-Ratelimit-Unified-Status")

	// 转换reset为int64
	var resetValue *int64
	if resetHeader != "" {
		if parsed, err := strconv.ParseInt(resetHeader, 10, 64); err == nil {
			resetValue = &parsed
		} else {
			s.logger.Debug("Failed to parse Anthropic-Ratelimit-Unified-Reset header", map[string]interface{}{
				"value":      resetHeader,
				"error":      err.Error(),
				"endpoint":   ep.Name,
				"request_id": requestID,
			})
		}
	}

	var statusValue *string
	if statusHeader != "" {
		statusValue = &statusHeader
	}

	// 更新endpoint状态
	changed, err := ep.UpdateRateLimitState(resetValue, statusValue)
	if err != nil {
		return err
	}

	// 如果状态发生变化，持久化到配置文件
	if changed {
		s.logger.Info("Rate limit state changed, persisting to config", map[string]interface{}{
			"endpoint":   ep.Name,
			"reset":      resetValue,
			"status":     statusValue,
			"request_id": requestID,
		})

		// 持久化到配置文件
		if err := s.persistRateLimitState(ep.ID, resetValue, statusValue); err != nil {
			s.logger.Error("Failed to persist rate limit state", err)
			return err
		}
	}

	// 检查增强保护：如果启用了增强保护且状态为allowed_warning，则禁用端点
	if ep.ShouldDisableOnAllowedWarning() && ep.IsAvailable() {
		s.logger.Info("Enhanced protection triggered: disabling endpoint due to allowed_warning status", map[string]interface{}{
			"endpoint":            ep.Name,
			"status":              statusValue,
			"enhanced_protection": true,
			"request_id":          requestID,
		})
		ep.MarkInactive()
	}

	return nil
}

// convertChatCompletionsToResponsesSSE 将 OpenAI /chat/completions SSE 格式转换为 /responses API 格式
// Codex 客户端使用 /responses API，期望的事件格式为：
//   - {"type": "response.created", "response": {...}}
//   - {"type": "response.output_text.delta", "delta": "..."}
//   - {"type": "response.completed", "response": {...}}
func (s *Server) convertChatCompletionsToResponsesSSE(body []byte) []byte {
	bodyStr := string(body)
	lines := strings.Split(bodyStr, "\n")

	var convertedLines []string
	responseID := ""
	model := ""
	created := int64(0)
	hasStarted := false

	for _, line := range lines {
		// SSE 格式：data: {...}
		if !strings.HasPrefix(line, "data: ") {
			convertedLines = append(convertedLines, line)
			continue
		}

		dataStr := strings.TrimPrefix(line, "data: ")
		dataStr = strings.TrimSpace(dataStr)

		// 跳过 [DONE] 标记，稍后添加 response.completed
		if dataStr == "[DONE]" {
			continue
		}

		// 解析 JSON
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			convertedLines = append(convertedLines, line)
			continue
		}

		// 提取基本信息
		if id, ok := chunk["id"].(string); ok && responseID == "" {
			responseID = id
		}
		if m, ok := chunk["model"].(string); ok && model == "" {
			model = m
		}
		if c, ok := chunk["created"].(float64); ok && created == 0 {
			created = int64(c)
		}

		// 获取 choices 数组
		choices, ok := chunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}

		choice := choices[0].(map[string]interface{})
		delta, hasDelta := choice["delta"].(map[string]interface{})
		finishReason, _ := choice["finish_reason"].(string)

		// 第一个事件：response.created
		if !hasStarted {
			hasStarted = true
			event := map[string]interface{}{
				"type": "response.created",
				"response": map[string]interface{}{
					"id":      responseID,
					"object":  "response",
					"created": created,
					"model":   model,
					"status":  "in_progress",
				},
			}
			eventJSON, _ := json.Marshal(event)
			convertedLines = append(convertedLines, "data: "+string(eventJSON))
			convertedLines = append(convertedLines, "")
		}

		// 内容增量事件：response.output_text.delta
		if hasDelta {
			if role, hasRole := delta["role"]; hasRole && role != "" {
				// 角色变化，忽略或处理
				_ = role
			}

			if content, hasContent := delta["content"].(string); hasContent && content != "" {
				event := map[string]interface{}{
					"type":        "response.output_text.delta",
					"delta":       content,
					"response_id": responseID,
				}
				eventJSON, _ := json.Marshal(event)
				convertedLines = append(convertedLines, "data: "+string(eventJSON))
				convertedLines = append(convertedLines, "")
			}
		}

		// 结束事件：response.completed
		if finishReason != "" {
			event := map[string]interface{}{
				"type": "response.completed",
				"response": map[string]interface{}{
					"id":            responseID,
					"object":        "response",
					"created":       created,
					"model":         model,
					"status":        "completed",
					"finish_reason": finishReason,
				},
			}
			eventJSON, _ := json.Marshal(event)
			convertedLines = append(convertedLines, "data: "+string(eventJSON))
			convertedLines = append(convertedLines, "")
		}
	}

	result := strings.Join(convertedLines, "\n")

	s.logger.Debug("Converted chat completions SSE to Responses API format", map[string]interface{}{
		"original_size":  len(body),
		"converted_size": len(result),
		"response_id":    responseID,
	})

	return []byte(result)
}

// convertChatCompletionToResponse 将 OpenAI /chat/completions 非流式响应转换为 Codex /responses 格式
// OpenAI格式: {"id":"xxx","object":"chat.completion","created":123,"model":"xxx","choices":[{"index":0,"message":{"role":"assistant","content":"..."}}]}
// Codex格式: {"type":"response","id":"xxx","object":"response","created":123,"model":"xxx","choices":[{"index":0,"message":{"role":"assistant","content":"..."}}]}
func (s *Server) convertChatCompletionToResponse(body []byte) ([]byte, error) {
	var completion map[string]interface{}
	if err := json.Unmarshal(body, &completion); err != nil {
		s.logger.Error("Failed to parse chat completion response", err)
		return nil, err
	}

	// 检查是否是chat.completion对象
	if obj, ok := completion["object"].(string); !ok || obj != "chat.completion" {
		// 不是chat.completion，可能已经是正确格式或其他错误
		return body, nil
	}

	// 转换为Codex response格式
	response := map[string]interface{}{
		"type":    "response", // Codex响应的type字段
		"id":      completion["id"],
		"object":  "response", // 改为response对象
		"created": completion["created"],
		"model":   completion["model"],
		"choices": completion["choices"], // choices保持不变
	}

	// 可选字段
	if usage, ok := completion["usage"]; ok {
		response["usage"] = usage
	}
	if systemFingerprint, ok := completion["system_fingerprint"]; ok {
		response["system_fingerprint"] = systemFingerprint
	}

	converted, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("Failed to marshal Codex response", err)
		return nil, err
	}

	s.logger.Debug("Converted chat completion to Responses API format", map[string]interface{}{
		"original_object":  "chat.completion",
		"converted_object": "response",
		"original_size":    len(body),
		"converted_size":   len(converted),
	})

	return converted, nil
}

// convertCodexToOpenAI 将 Codex /responses 格式转换为 OpenAI /chat/completions 格式
// Codex 格式复杂，包含多个特殊字段：
//   - instructions: 系统提示（字符串）
//   - input: 消息数组（结构与 OpenAI messages 不同）
//   - include: 响应包含选项（Codex 特有）
//
// 转换策略：
//  1. 从 input 数组提取内容，转换为标准 OpenAI messages 格式
//  2. instructions 作为系统消息（如果存在）
//  3. 删除 Codex 特有字段（input, include 等）
func (s *Server) convertCodexToOpenAI(requestBody []byte) ([]byte, error) {
	// 解析请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		s.logger.Error("Failed to parse request body for Codex conversion", err)
		return nil, err
	}

	// 检查是否是 Codex 格式（至少要有 input 或 instructions 字段之一）
	_, hasInput := requestData["input"]
	_, hasInstructions := requestData["instructions"]

	if !hasInput && !hasInstructions {
		// 不是 Codex 格式，跳过转换
		return nil, nil
	}

	// 构建 OpenAI messages 数组
	messages := []map[string]interface{}{}

	// 1. 处理 instructions（作为 system 消息）
	if hasInstructions {
		if instructionsStr, ok := requestData["instructions"].(string); ok && instructionsStr != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": instructionsStr,
			})
		}
		delete(requestData, "instructions")
	}

	// 2. 处理 input 数组（转换为 user/assistant 消息）
	if hasInput {
		if inputArray, ok := requestData["input"].([]interface{}); ok {
			for _, item := range inputArray {
				if inputMsg, ok := item.(map[string]interface{}); ok {
					// 提取 role
					role, _ := inputMsg["role"].(string)
					if role == "" {
						role = "user" // 默认为 user
					}

					// 提取 content
					// Codex 的 content 是一个数组，包含 {text, type} 对象
					var contentStr string
					if contentArray, ok := inputMsg["content"].([]interface{}); ok {
						for _, contentItem := range contentArray {
							if contentObj, ok := contentItem.(map[string]interface{}); ok {
								if text, ok := contentObj["text"].(string); ok {
									contentStr += text
								}
							}
						}
					}

					if contentStr != "" {
						messages = append(messages, map[string]interface{}{
							"role":    role,
							"content": contentStr,
						})
					}
				}
			}
		}
		delete(requestData, "input")
	}

	// 如果没有成功转换出任何消息，添加一个默认的 user 消息
	if len(messages) == 0 {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": "Hello",
		})
	}

	// 设置 messages 字段
	requestData["messages"] = messages

	// 删除其他 Codex 特有字段
	delete(requestData, "include") // Codex 特有的响应选项

	// 保留以下字段（OpenAI 兼容）：
	// - tools: 工具定义数组（OpenAI 标准）
	// - tool_choice: 工具选择策略（OpenAI 标准）
	// - stream: 流式响应标志（OpenAI 标准）
	// - temperature, max_tokens 等参数（OpenAI 标准）

	// 注意：tools 字段在 Codex 和 OpenAI 中格式相同，可以直接保留
	// 不需要特殊处理，只需确保不被删除

	// 重新序列化为 JSON
	convertedBody, err := json.Marshal(requestData)
	if err != nil {
		s.logger.Error("Failed to marshal converted request body", err)
		return nil, err
	}

	s.logger.Debug("Codex to OpenAI conversion completed", map[string]interface{}{
		"messages_count": len(messages),
		"has_tools":      requestData["tools"] != nil,
		"has_stream":     requestData["stream"] != nil,
	})

	return convertedBody, nil
}

// 动态更新端点的Codex支持状态
func (s *Server) updateEndpointCodexSupport(ep *endpoint.Endpoint, isCodex bool) {
	if ep == nil {
		return
	}

	// 使用端点的公共方法来安全地更新状态
	ep.UpdateNativeCodexSupport(isCodex)
	s.logger.Info(fmt.Sprintf("Updated endpoint %s native_codex_support to %v", ep.Name, isCodex))
}

// 🎓 从400错误响应中学习不支持的参数
func (s *Server) learnUnsupportedParamsFromError(errorBody []byte, ep *endpoint.Endpoint, requestBody []byte) {
	if ep == nil || len(errorBody) == 0 {
		return
	}

	// 解析错误消息
	var errorData map[string]interface{}
	if err := json.Unmarshal(errorBody, &errorData); err != nil {
		return // 无法解析为JSON,忽略
	}

	// 尝试从错误消息中提取参数名
	errorMsg := ""
	if msg, ok := errorData["message"].(string); ok {
		errorMsg = msg
	} else if err, ok := errorData["error"].(map[string]interface{}); ok {
		if msg, ok := err["message"].(string); ok {
			errorMsg = msg
		}
	} else if err, ok := errorData["error"].(string); ok {
		errorMsg = err
	}

	if errorMsg == "" {
		return
	}

	// 解析请求体以检查哪些参数存在
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		return
	}

	// 常见的不支持参数关键词模式
	unsupportedPatterns := []struct {
		keywords []string
		params   []string
	}{
		{
			keywords: []string{"tool", "function", "function_call", "tool_choice"},
			params:   []string{"tools", "tool_choice", "functions", "function_call"},
		},
		{
			keywords: []string{"unsupported", "not supported", "invalid parameter", "unexpected parameter"},
			params:   []string{}, // 将从错误消息中动态提取
		},
	}

	errorMsgLower := strings.ToLower(errorMsg)

	// 检查每个模式
	for _, pattern := range unsupportedPatterns {
		matched := false
		for _, keyword := range pattern.keywords {
			if strings.Contains(errorMsgLower, keyword) {
				matched = true
				break
			}
		}

		if matched {
			// 如果模式匹配，学习对应的参数
			if len(pattern.params) > 0 {
				for _, param := range pattern.params {
					if _, exists := requestData[param]; exists {
						ep.LearnUnsupportedParam(param)
						s.logger.Info("Learned unsupported parameter from API error", map[string]interface{}{
							"endpoint":  ep.Name,
							"parameter": param,
							"error_msg": errorMsg,
						})
					}
				}
			} else {
				// 尝试从错误消息中提取参数名
				// 匹配类似 "parameter 'xxx' is not supported" 或 "unsupported parameter: xxx"
				paramNameRegex := regexp.MustCompile(`parameter[\s'":]*([a-zA-Z_][a-zA-Z0-9_]*)`)
				matches := paramNameRegex.FindStringSubmatch(errorMsg)
				if len(matches) > 1 {
					paramName := matches[1]
					if _, exists := requestData[paramName]; exists {
						ep.LearnUnsupportedParam(paramName)
						s.logger.Info("Learned unsupported parameter from API error (regex)", map[string]interface{}{
							"endpoint":  ep.Name,
							"parameter": paramName,
							"error_msg": errorMsg,
						})
					}
				}
			}
		}
	}
}
