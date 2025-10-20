package web

import (
	"fmt"
	"net/http"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/endpoint"

	"github.com/gin-gonic/gin"
)

// Endpoint is an alias for endpoint.Endpoint for internal use
type Endpoint = endpoint.Endpoint

// handleGetEndpoints returns the list of all endpoints
func (s *AdminServer) handleGetEndpoints(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	c.JSON(http.StatusOK, gin.H{
		"endpoints": endpoints,
	})
}

// handleUpdateEndpoints is a stub for a deprecated feature.
func (s *AdminServer) handleUpdateEndpoints(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

// handleCreateEndpoint creates a new endpoint in the configuration file.
func (s *AdminServer) handleCreateEndpoint(c *gin.Context) {
	// 解析请求体
	var createReq config.EndpointConfig

	if err := c.ShouldBindJSON(&createReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 验证必填字段
	if createReq.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint name is required"})
		return
	}

	// 添加调试日志
	s.logger.Debug(fmt.Sprintf("Creating endpoint '%s': URLAnthropic='%s', URLOpenAI='%s'",
		createReq.Name, createReq.URLAnthropic, createReq.URLOpenAI))

	if createReq.URLAnthropic == "" && createReq.URLOpenAI == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "at least one URL (url_anthropic or url_openai) is required",
			"debug": fmt.Sprintf("Received: url_anthropic='%s', url_openai='%s'",
				createReq.URLAnthropic, createReq.URLOpenAI),
		})
		return
	}

	// 检查端点名称是否已存在
	for i := range s.config.Endpoints {
		if s.config.Endpoints[i].Name == createReq.Name {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("endpoint '%s' already exists", createReq.Name)})
			return
		}
	}

	// 设置默认优先级（放到列表末尾）
	if createReq.Priority == 0 {
		createReq.Priority = len(s.config.Endpoints) + 1
	}

	// 设置默认值
	if createReq.AuthType == "" {
		createReq.AuthType = "auto"
	}

	// 添加到配置
	s.config.Endpoints = append(s.config.Endpoints, createReq)

	// 保存到配置文件（用户操作：立即写入）
	if err := s.saveConfigImmediately(); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to save config after creating endpoint '%s'", createReq.Name), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save configuration: %v", err)})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after create", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}

		if err := s.hotUpdateHandler.HotUpdateConfig(newConfig); err != nil {
			s.logger.Error("Failed to hot update configuration", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("端点 '%s' 创建成功", createReq.Name),
	})
}

// handleUpdateEndpoint updates a specific endpoint's configuration
func (s *AdminServer) handleUpdateEndpoint(c *gin.Context) {
	endpointID := c.Param("id")
	if endpointID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint ID is required"})
		return
	}

	// 解析完整的端点配置
	var updateReq config.EndpointConfig

	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 查找现有端点配置
	foundIndex := -1
	for i := range s.config.Endpoints {
		if s.config.Endpoints[i].Name == endpointID {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("endpoint '%s' not found", endpointID)})
		return
	}

	// 获取旧配置作为基础
	oldConfig := s.config.Endpoints[foundIndex]

	// 如果请求中的 name 为空，使用旧名称
	if updateReq.Name == "" {
		updateReq.Name = oldConfig.Name
	}

	// 如果改名了，检查新名称是否已存在
	if updateReq.Name != endpointID {
		for i := range s.config.Endpoints {
			if s.config.Endpoints[i].Name == updateReq.Name {
				c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("endpoint '%s' already exists", updateReq.Name)})
				return
			}
		}
	}

	// 智能合并：如果新值为空，保留旧值（URL字段除外，因为前端可能明确要清空）
	// URL字段特殊处理：前端会明确发送空字符串来清空，所以我们尊重前端的意图
	// 但如果两个URL都是空，这是错误的，需要保留至少一个
	finalURLAnthropic := updateReq.URLAnthropic
	finalURLOpenAI := updateReq.URLOpenAI

	// 如果新配置的两个URL都为空，保留旧的URL配置
	if finalURLAnthropic == "" && finalURLOpenAI == "" {
		finalURLAnthropic = oldConfig.URLAnthropic
		finalURLOpenAI = oldConfig.URLOpenAI
	}

	// 保留优先级（如果请求中没有指定）
	if updateReq.Priority == 0 {
		updateReq.Priority = oldConfig.Priority
	}

	// 保留其他关键字段（如果为空）
	if updateReq.AuthType == "" {
		updateReq.AuthType = oldConfig.AuthType
	}
	if updateReq.AuthValue == "" {
		updateReq.AuthValue = oldConfig.AuthValue
	}

	// 应用合并后的URL
	updateReq.URLAnthropic = finalURLAnthropic
	updateReq.URLOpenAI = finalURLOpenAI

	// 完全替换端点配置
	s.config.Endpoints[foundIndex] = updateReq

	// 保存到配置文件（用户操作：立即写入）
	if err := s.saveConfigImmediately(); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to save config after updating endpoint '%s'", endpointID), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save configuration: %v", err)})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after update", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}

		if err := s.hotUpdateHandler.HotUpdateConfig(newConfig); err != nil {
			s.logger.Error("Failed to hot update configuration", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}
	}

	// 返回更新后的端点信息
	var updatedEndpoint *Endpoint
	allEndpointsAfterUpdate := s.endpointManager.GetAllEndpoints()
	for _, ep := range allEndpointsAfterUpdate {
		if ep.Name == updateReq.Name { // 使用新名称查找
			updatedEndpoint = ep
			break
		}
	}

	if updatedEndpoint == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "endpoint updated but not found after reload"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"endpoint": updatedEndpoint,
		"message":  fmt.Sprintf("端点 '%s' 更新成功", updateReq.Name),
	})
}

// updateEndpointInConfig 安全地更新指定端点的配置并持久化到config.yaml
func (s *AdminServer) updateEndpointInConfig(endpointName string, updateFunc func(*config.EndpointConfig) error) error {
	// 查找对应的端点配置
	for i := range s.config.Endpoints {
		if s.config.Endpoints[i].Name == endpointName {
			// 应用更新函数
			if err := updateFunc(&s.config.Endpoints[i]); err != nil {
				return err
			}

			// 保存到配置文件（用户操作：立即写入）
			return s.saveConfigImmediately()
		}
	}

	return fmt.Errorf("endpoint not found: %s", endpointName)
}

// handleDeleteEndpoint deletes an endpoint from the configuration file.
func (s *AdminServer) handleDeleteEndpoint(c *gin.Context) {
	endpointID := c.Param("id")
	if endpointID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint ID is required"})
		return
	}

	// 查找端点索引
	foundIndex := -1
	for i := range s.config.Endpoints {
		if s.config.Endpoints[i].Name == endpointID {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("endpoint '%s' not found", endpointID)})
		return
	}

	// 从配置中删除端点
	s.config.Endpoints = append(s.config.Endpoints[:foundIndex], s.config.Endpoints[foundIndex+1:]...)

	// 保存到配置文件（用户操作：立即写入）
	if err := s.saveConfigImmediately(); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to save config after deleting endpoint '%s'", endpointID), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save configuration: %v", err)})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after delete", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}

		if err := s.hotUpdateHandler.HotUpdateConfig(newConfig); err != nil {
			s.logger.Error("Failed to hot update configuration", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("端点 '%s' 已删除", endpointID),
	})
}

// normalizeOpenAIPreference is a helper for a deprecated feature.
func normalizeOpenAIPreference(value string) (string, error) {
	// No-op, kept for compatibility if anything still calls it.
	return "auto", nil
}