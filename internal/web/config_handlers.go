package web

import (
	"fmt"
	"net/http"
	"net/url"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/modelrewrite"
	"claude-code-codex-companion/internal/utils"

	"github.com/gin-gonic/gin"
)

// handleGetConfig 获取当前配置
func (s *AdminServer) handleGetConfig(c *gin.Context) {
	// 返回当前配置，但隐藏敏感信息
	configCopy := *s.config
	
	// 隐藏认证信息的敏感部分
	// 直接返回配置，不掩码认证值
	
	c.JSON(http.StatusOK, gin.H{
		"config": configCopy,
	})
}

// handleHotUpdateConfig 热更新配置
func (s *AdminServer) handleHotUpdateConfig(c *gin.Context) {
	var request struct {
		Config config.Config `json:"config"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format: " + err.Error(),
		})
		return
	}

	// 验证新配置
	newConfig := request.Config
	if err := s.validateConfigUpdate(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Configuration validation failed: " + err.Error(),
		})
		return
	}

	// 保存配置到文件
	if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save configuration file: " + err.Error(),
		})
		return
	}

	// 如果有热更新处理器，执行热更新
	if s.hotUpdateHandler != nil {
		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			// 热更新失败，记录错误但不回滚文件（文件已保存成功）
			s.logger.Error("Hot update failed, configuration file saved but runtime not updated", err)
			c.JSON(http.StatusPartialContent, gin.H{
				"warning": "Configuration file saved successfully, but hot update failed: " + err.Error(),
				"message": "Server restart may be required for some changes to take effect",
			})
			return
		}
	}

	// 更新本地配置引用
	s.config = &newConfig

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully via hot update",
	})
}

// validateConfigUpdate validates the configuration update using unified validation
func (s *AdminServer) validateConfigUpdate(newConfig *config.Config) error {
	// 使用统一的服务器配置验证
	if err := utils.ValidateServerConfig(newConfig.Server.Host, newConfig.Server.Port); err != nil {
		return err
	}

	// 转换为接口类型进行统一验证
	validator := utils.NewEndpointConfigValidator()
	endpointInterfaces := make([]utils.EndpointConfig, len(newConfig.Endpoints))
	for i, ep := range newConfig.Endpoints {
		endpointInterfaces[i] = ep
	}

	return validator.ValidateEndpoints(endpointInterfaces)
}

// handleUpdateEndpointModelRewrite 更新端点模型重写配置
func (s *AdminServer) handleUpdateEndpointModelRewrite(c *gin.Context) {
	encodedEndpointName := c.Param("id")
	endpointName, err := url.PathUnescape(encodedEndpointName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endpoint name encoding"})
		return
	}

	var request config.ModelRewriteConfig
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 验证重写配置
	if err := config.ValidateModelRewriteConfig(&request, fmt.Sprintf("endpoint '%s'", endpointName)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model rewrite config: " + err.Error()})
		return
	}

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints
	found := false

	for i, ep := range currentEndpoints {
		if ep.Name == endpointName {
			// 始终保存模型重写配置对象，即使是禁用状态或空规则
			// 这样前端编辑时可以正确显示禁用状态，而不是null
			currentEndpoints[i].ModelRewrite = &request
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 使用热更新机制
	if s.hotUpdateHandler != nil {
		// 创建新配置，只更新端点部分
		newConfig := *s.config
		newConfig.Endpoints = currentEndpoints

		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update model rewrite config: " + err.Error(),
			})
			return
		}

		// 保存配置到文件
		if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
			s.logger.Error("Failed to save configuration file after model rewrite config update", err)
			// 不返回错误，因为内存更新已成功
		}

		// 更新本地配置引用
		s.config = &newConfig

		c.JSON(http.StatusOK, gin.H{
			"message": "Model rewrite configuration updated successfully",
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Hot update is not available",
		})
	}
}

// handleTestModelRewrite 测试模型重写规则
func (s *AdminServer) handleTestModelRewrite(c *gin.Context) {
	encodedEndpointName := c.Param("id")
	endpointName, err := url.PathUnescape(encodedEndpointName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endpoint name encoding"})
		return
	}

	var request struct {
		TestModel string `json:"test_model"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	if request.TestModel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test_model is required"})
		return
	}

	// 查找端点
	var targetEndpoint *config.EndpointConfig
	for _, ep := range s.config.Endpoints {
		if ep.Name == endpointName {
			targetEndpoint = &ep
			break
		}
	}

	if targetEndpoint == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	if targetEndpoint.ModelRewrite == nil || !targetEndpoint.ModelRewrite.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"original_model":  request.TestModel,
			"rewritten_model": request.TestModel,
			"matched_rule":    "",
			"rewrite_applied": false,
		})
		return
	}

	// 创建临时重写器进行测试
	rewriter := modelrewrite.NewRewriter(*s.logger)
	rewrittenModel, matchedRule, matched := rewriter.TestRewriteRule(request.TestModel, targetEndpoint.ModelRewrite.Rules)

	c.JSON(http.StatusOK, gin.H{
		"original_model":  request.TestModel,
		"rewritten_model": rewrittenModel,
		"matched_rule":    matchedRule,
		"rewrite_applied": matched,
	})
}