package web

import (
	"fmt"
	"net/http"

	"claude-code-codex-companion/internal/config"

	"github.com/gin-gonic/gin"
)

const disabledError = "This feature is disabled due to a major architectural refactoring. Endpoint management is now done directly in the config.yaml file."

func (s *AdminServer) handleToggleEndpoint(c *gin.Context) {
	endpointID := c.Param("id")
	if endpointID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint ID is required"})
		return
	}

	// 解析请求体
	var toggleReq struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&toggleReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 更新配置文件
	err := s.updateEndpointInConfig(endpointID, func(cfg *config.EndpointConfig) error {
		cfg.Enabled = toggleReq.Enabled
		return nil
	})

	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to toggle endpoint '%s'", endpointID), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after toggle", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}

		if err := s.hotUpdateHandler.HotUpdateConfig(newConfig); err != nil {
			s.logger.Error("Failed to hot update configuration", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}
	}

	actionText := "禁用"
	if toggleReq.Enabled {
		actionText = "启用"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("端点 '%s' 已%s", endpointID, actionText),
		"enabled": toggleReq.Enabled,
	})
}

// handleCopyEndpoint creates a copy of an existing endpoint with a new name.
func (s *AdminServer) handleCopyEndpoint(c *gin.Context) {
	endpointID := c.Param("id")
	if endpointID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint ID is required"})
		return
	}

	// 解析请求体
	var copyReq struct {
		NewName string `json:"new_name"`
	}

	if err := c.ShouldBindJSON(&copyReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	if copyReq.NewName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new_name is required"})
		return
	}

	// 查找源端点
	var sourceEndpoint *config.EndpointConfig
	for i := range s.config.Endpoints {
		if s.config.Endpoints[i].Name == endpointID {
			sourceEndpoint = &s.config.Endpoints[i]
			break
		}
	}

	if sourceEndpoint == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("endpoint '%s' not found", endpointID)})
		return
	}

	// 检查新名称是否已存在
	for i := range s.config.Endpoints {
		if s.config.Endpoints[i].Name == copyReq.NewName {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("endpoint '%s' already exists", copyReq.NewName)})
			return
		}
	}

	// 创建副本
	newEndpoint := *sourceEndpoint
	newEndpoint.Name = copyReq.NewName
	// 将新端点的优先级设置为最后
	newEndpoint.Priority = len(s.config.Endpoints) + 1

	// 添加到配置
	s.config.Endpoints = append(s.config.Endpoints, newEndpoint)

	// 保存到配置文件（用户操作：立即写入）
	if err := s.saveConfigImmediately(); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to save config after copying endpoint '%s'", endpointID), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save configuration: %v", err)})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after copy", err)
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
		"message": fmt.Sprintf("端点 '%s' 已复制为 '%s'", endpointID, copyReq.NewName),
	})
}

// handleResetEndpointStatus resets the status of a specific endpoint using the existing manager method.
func (s *AdminServer) handleResetEndpointStatus(c *gin.Context) {
	endpointID := c.Param("id")
	if endpointID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint ID is required"})
		return
	}

	// 使用已有的端点管理器方法重置状态
	if err := s.endpointManager.ResetEndpointStatus(endpointID); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to reset endpoint status '%s'", endpointID), err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("端点 '%s' 状态已重置", endpointID),
	})
}

// handleResetAllEndpointsStatus resets the status of all endpoints.
func (s *AdminServer) handleResetAllEndpointsStatus(c *gin.Context) {
	allEndpoints := s.endpointManager.GetAllEndpoints()
	resetCount := 0
	var errors []string

	for _, ep := range allEndpoints {
		if err := s.endpointManager.ResetEndpointStatus(ep.Name); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", ep.Name, err))
		} else {
			resetCount++
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusPartialContent, gin.H{
			"message": fmt.Sprintf("重置了 %d 个端点，%d 个失败", resetCount, len(errors)),
			"errors":  errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("成功重置了 %d 个端点的状态", resetCount),
	})
}

func (s *AdminServer) handleReorderEndpoints(c *gin.Context) {
	// 解析请求体
	var reorderReq struct {
		OrderedNames []string `json:"ordered_names"`
	}

	if err := c.ShouldBindJSON(&reorderReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	if len(reorderReq.OrderedNames) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ordered_names cannot be empty"})
		return
	}

	// 创建名称到新优先级的映射
	nameToPriority := make(map[string]int)
	for i, name := range reorderReq.OrderedNames {
		nameToPriority[name] = i + 1 // 优先级从1开始
	}

	// 更新配置文件中所有端点的优先级
	for i := range s.config.Endpoints {
		if newPriority, exists := nameToPriority[s.config.Endpoints[i].Name]; exists {
			s.config.Endpoints[i].Priority = newPriority
		}
	}

	// 保存到配置文件（用户操作：立即写入）
	if err := s.saveConfigImmediately(); err != nil {
		s.logger.Error("Failed to save config after reorder", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save configuration: %v", err)})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after reorder", err)
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
		"message": "端点排序已更新",
	})
}

func (s *AdminServer) handleSortEndpoints(c *gin.Context) {
	// 获取所有端点
	allEndpoints := s.endpointManager.GetAllEndpoints()
	if len(allEndpoints) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "没有端点需要排序"})
		return
	}

	// 简化排序逻辑：只按 启用状态 > 可用性 > 响应速度
	type endpointInfo struct {
		name         string
		enabled      bool
		available    bool
		responseTime int64 // 毫秒
	}

	infos := make([]endpointInfo, 0, len(allEndpoints))
	for _, ep := range allEndpoints {
		responseTime := ep.GetLastResponseTime().Milliseconds()
		infos = append(infos, endpointInfo{
			name:         ep.Name,
			enabled:      ep.Enabled,
			available:    ep.IsAvailable(),
			responseTime: responseTime,
		})
	}

	// 排序：启用 > 可用 > 速度快
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			shouldSwap := false

			// 1. 启用的优先
			if infos[i].enabled != infos[j].enabled {
				shouldSwap = !infos[i].enabled && infos[j].enabled
			} else if infos[i].available != infos[j].available {
				// 2. 可用的优先
				shouldSwap = !infos[i].available && infos[j].available
			} else if infos[i].responseTime > 0 && infos[j].responseTime > 0 {
				// 3. 响应快的优先
				shouldSwap = infos[i].responseTime > infos[j].responseTime
			}

			if shouldSwap {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	// 创建名称到新优先级的映射
	nameToPriority := make(map[string]int)
	for i, info := range infos {
		nameToPriority[info.name] = i + 1 // 优先级从1开始
	}

	// 更新配置文件中所有端点的优先级
	for i := range s.config.Endpoints {
		if newPriority, exists := nameToPriority[s.config.Endpoints[i].Name]; exists {
			s.config.Endpoints[i].Priority = newPriority
		}
	}

	// 保存到配置文件（用户操作：立即写入）
	if err := s.saveConfigImmediately(); err != nil {
		s.logger.Error("Failed to save config after sort", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save configuration: %v", err)})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after sort", err)
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
		"message": "端点已按可用性和速度排序完成",
	})
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// refreshDynamicSorterEndpoints is a stub for a deprecated feature.
func (s *AdminServer) refreshDynamicSorterEndpoints() error {
	return fmt.Errorf(disabledError)
}

// hotUpdateEndpointsWithRetry is a stub for a deprecated feature.
func (s *AdminServer) hotUpdateEndpointsWithRetry(endpoints []interface{}, maxRetries int) error {
	return fmt.Errorf(disabledError)
}