package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-code-codex-companion/internal/endpoint"
	jsonutils "claude-code-codex-companion/internal/common/json"
	"github.com/gin-gonic/gin"
)

// models.go: 模型列表模块
// 负责处理 /v1/models 端点的逻辑。
//
// 目标：
// - 包含 handleModelsList 函数及其所有辅助函数。
// - 负责从不同的上游端点获取模型列表。
// - 将不同格式的模型列表响应聚合并转换为客户端期望的格式。

func (s *Server) handleModelsList(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = fmt.Sprintf("models-%d", time.Now().UnixNano())
		c.Set("request_id", requestID)
	}

	path := c.Request.URL.Path
	s.logger.Info("📋 Models list request", map[string]interface{}{
		"request_id": requestID,
		"path":       path,
	})

	// 检测客户端格式
	clientFormat := s.detectModelsClientFormat(c)

	// 选择合适的端点
	ep, err := s.selectEndpointForModels(clientFormat)
	if err != nil {
		s.logger.Error("Failed to select endpoint for models", err, map[string]interface{}{
			"request_id":    requestID,
			"client_format": clientFormat,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"type":    "service_unavailable",
				"message": "No suitable endpoint available for models list",
			},
		})
		return
	}

	// 构建上游请求
	upstreamURL := s.buildModelsUpstreamURL(ep, clientFormat, path)

	// 创建上游请求
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", upstreamURL, nil)
	if err != nil {
		s.logger.Error("Failed to create upstream request", err, map[string]interface{}{
			"request_id": requestID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"type":    "internal_error",
				"message": "Failed to create upstream request",
			},
		})
		return
	}

	// 设置认证头
	authHeader, err := ep.GetAuthHeader()
	if err != nil {
		s.logger.Error("Failed to get auth header", err, map[string]interface{}{
			"request_id": requestID,
			"endpoint":   ep.Name,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"type":    "auth_error",
				"message": "Failed to get authentication header",
			},
		})
		return
	}

	// 根据客户端格式设置认证
	s.setModelsAuthHeader(req, clientFormat, authHeader)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Upstream request failed", err, map[string]interface{}{
			"request_id": requestID,
			"endpoint":   ep.Name,
			"url":        upstreamURL,
		})
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"type":    "upstream_error",
				"message": "Failed to fetch models from upstream",
			},
		})
		return
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read upstream response", err, map[string]interface{}{
			"request_id": requestID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"type":    "response_error",
				"message": "Failed to read upstream response",
			},
		})
		return
	}

	// 如果上游返回错误，直接返回
	if resp.StatusCode >= 400 {
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	// 转换响应格式（如果需要）
	convertedBody, err := s.convertModelsResponse(body, clientFormat, ep.EndpointType)
	if err != nil {
		s.logger.Error("Failed to convert models response", err, map[string]interface{}{
			"request_id":    requestID,
			"client_format": clientFormat,
			"endpoint_type": ep.EndpointType,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"type":    "conversion_error",
				"message": "Failed to convert models response format",
			},
		})
		return
	}

	// 设置响应头并返回
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", convertedBody)

	s.logger.Info("📋 Models list completed", map[string]interface{}{
		"request_id":    requestID,
		"endpoint":      ep.Name,
		"client_format": clientFormat,
		"status_code":   resp.StatusCode,
	})
}

// detectModelsClientFormat 检测模型列表请求的客户端格式
func (s *Server) detectModelsClientFormat(c *gin.Context) string {
	path := c.Request.URL.Path
	authHeader := c.GetHeader("Authorization")
	apiKey := c.GetHeader("x-api-key")
	apiKeyParam := c.Query("key")

	// 根据路径和认证方式判断
	if strings.Contains(path, "/v1beta/models") {
		return "gemini"
	}

	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		return "openai"
	}

	if apiKey != "" {
		return "anthropic"
	}

	if apiKeyParam != "" {
		return "gemini"
	}

	// 默认返回openai格式
	return "openai"
}

// selectEndpointForModels 为模型列表选择合适的端点
func (s *Server) selectEndpointForModels(clientFormat string) (*endpoint.Endpoint, error) {
	endpoints := s.endpointManager.GetAllEndpoints()

	// 优先选择支持相应格式的端点
	for _, ep := range endpoints {
		if !ep.Enabled {
			continue
		}

		switch clientFormat {
		case "openai":
			if ep.URLOpenAI != "" {
				return ep, nil
			}
		case "anthropic":
			if ep.URLAnthropic != "" {
				return ep, nil
			}
		case "gemini":
			if ep.URLGemini != "" {
				return ep, nil
			}
		}
	}

	return nil, fmt.Errorf("no suitable endpoint found for format: %s", clientFormat)
}

// buildModelsUpstreamURL 构建上游模型列表URL
func (s *Server) buildModelsUpstreamURL(ep *endpoint.Endpoint, clientFormat, path string) string {
	switch clientFormat {
	case "openai":
		return ep.URLOpenAI + "/v1/models"
	case "anthropic":
		return ep.URLAnthropic + "/v1/models"
	case "gemini":
		return ep.URLGemini + path
	default:
		return ep.URLOpenAI + "/v1/models"
	}
}

// setModelsAuthHeader 设置模型列表请求的认证头
func (s *Server) setModelsAuthHeader(req *http.Request, clientFormat, authHeader string) {
	switch clientFormat {
	case "openai":
		req.Header.Set("Authorization", authHeader)
	case "anthropic":
		if strings.HasPrefix(authHeader, "Bearer ") {
			req.Header.Set("x-api-key", strings.TrimPrefix(authHeader, "Bearer "))
		} else {
			req.Header.Set("x-api-key", authHeader)
		}
	case "gemini":
		if strings.HasPrefix(authHeader, "Bearer ") {
			req.Header.Set("x-goog-api-key", strings.TrimPrefix(authHeader, "Bearer "))
		} else {
			req.Header.Set("x-goog-api-key", authHeader)
		}
	}
}

// convertModelsResponse 转换模型列表响应格式
func (s *Server) convertModelsResponse(body []byte, clientFormat, endpointType string) ([]byte, error) {
	// 如果客户端格式和端点类型相同，无需转换
	if clientFormat == endpointType {
		return body, nil
	}

	// 解析原始响应
	var originalResp map[string]interface{}
	if err := jsonutils.SafeUnmarshal(body, &originalResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 根据需要进行格式转换
	switch clientFormat {
	case "openai":
		return s.convertToOpenAIModelsFormat(originalResp, endpointType)
	case "anthropic":
		return s.convertToAnthropicModelsFormat(originalResp, endpointType)
	case "gemini":
		return s.convertToGeminiModelsFormat(originalResp, endpointType)
	default:
		return body, nil
	}
}

// convertToOpenAIModelsFormat 转换为OpenAI格式的模型列表
func (s *Server) convertToOpenAIModelsFormat(resp map[string]interface{}, sourceFormat string) ([]byte, error) {
	var models []map[string]interface{}

	switch sourceFormat {
	case "anthropic":
		// Anthropic格式通常返回字符串数组
		if data, ok := resp["data"].([]interface{}); ok {
			for _, item := range data {
				if modelID, ok := item.(string); ok {
					models = append(models, map[string]interface{}{
						"id":       modelID,
						"object":   "model",
						"created":  time.Now().Unix(),
						"owned_by": "anthropic",
					})
				}
			}
		}
	case "gemini":
		// Gemini格式的模型列表
		if modelsData, ok := resp["models"].([]interface{}); ok {
			for _, item := range modelsData {
				if modelData, ok := item.(map[string]interface{}); ok {
					if name, ok := modelData["name"].(string); ok {
						models = append(models, map[string]interface{}{
							"id":       name,
							"object":   "model",
							"created":  time.Now().Unix(),
							"owned_by": "google",
						})
					}
				}
			}
		}
	}

	result := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	return jsonutils.SafeMarshal(result)
}

// convertToAnthropicModelsFormat 转换为Anthropic格式的模型列表
func (s *Server) convertToAnthropicModelsFormat(resp map[string]interface{}, sourceFormat string) ([]byte, error) {
	var models []string

	switch sourceFormat {
	case "openai":
		// OpenAI格式转换为Anthropic格式
		if data, ok := resp["data"].([]interface{}); ok {
			for _, item := range data {
				if modelData, ok := item.(map[string]interface{}); ok {
					if id, ok := modelData["id"].(string); ok {
						models = append(models, id)
					}
				}
			}
		}
	case "gemini":
		// Gemini格式转换为Anthropic格式
		if modelsData, ok := resp["models"].([]interface{}); ok {
			for _, item := range modelsData {
				if modelData, ok := item.(map[string]interface{}); ok {
					if name, ok := modelData["name"].(string); ok {
						models = append(models, name)
					}
				}
			}
		}
	}

	result := map[string]interface{}{
		"data": models,
	}

	return jsonutils.SafeMarshal(result)
}

// convertToGeminiModelsFormat 转换为Gemini格式的模型列表
func (s *Server) convertToGeminiModelsFormat(resp map[string]interface{}, sourceFormat string) ([]byte, error) {
	var models []map[string]interface{}

	switch sourceFormat {
	case "openai":
		// OpenAI格式转换为Gemini格式
		if data, ok := resp["data"].([]interface{}); ok {
			for _, item := range data {
				if modelData, ok := item.(map[string]interface{}); ok {
					if id, ok := modelData["id"].(string); ok {
						models = append(models, map[string]interface{}{
							"name":                       id,
							"version":                    "001",
							"displayName":                id,
							"description":                fmt.Sprintf("Model %s", id),
							"inputTokenLimit":            32768,
							"outputTokenLimit":           8192,
							"supportedGenerationMethods": []string{"generateContent"},
						})
					}
				}
			}
		}
	case "anthropic":
		// Anthropic格式转换为Gemini格式
		if data, ok := resp["data"].([]interface{}); ok {
			for _, item := range data {
				if modelID, ok := item.(string); ok {
					models = append(models, map[string]interface{}{
						"name":                       modelID,
						"version":                    "001",
						"displayName":                modelID,
						"description":                fmt.Sprintf("Model %s", modelID),
						"inputTokenLimit":            32768,
						"outputTokenLimit":           8192,
						"supportedGenerationMethods": []string{"generateContent"},
					})
				}
			}
		}
	}

	result := map[string]interface{}{
		"models": models,
	}

	return jsonutils.SafeMarshal(result)
}
