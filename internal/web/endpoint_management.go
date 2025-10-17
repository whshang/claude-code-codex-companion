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

func (s *AdminServer) handleCopyEndpoint(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

func (s *AdminServer) handleResetEndpointStatus(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

func (s *AdminServer) handleResetAllEndpointsStatus(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
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

	// 保存到配置文件
	if err := config.SaveConfig(s.config, s.configFilePath); err != nil {
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

	// 保存到配置文件
	if err := config.SaveConfig(s.config, s.configFilePath); err != nil {
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