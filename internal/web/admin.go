package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/i18n"
	"claude-code-codex-companion/internal/logger"
	"claude-code-codex-companion/internal/security"
	"claude-code-codex-companion/internal/tagging"
	"claude-code-codex-companion/internal/webres"

	"github.com/gin-gonic/gin"
)

// HotUpdateHandler defines the interface for hot config updates
type HotUpdateHandler interface {
	HotUpdateConfig(newConfig *config.Config) error
	GetConfigPersister() *config.ConfigPersister
}

// PersistenceHandler defines the interface for persisting learned endpoint configuration
type PersistenceHandler interface {
	PersistEndpointLearning(ep *endpoint.Endpoint)
}

type AdminServer struct {
	config             *config.Config
	endpointManager    *endpoint.Manager
	taggingManager     *tagging.Manager
	logger             *logger.Logger
	configFilePath     string
	hotUpdateHandler   HotUpdateHandler
	persistenceHandler PersistenceHandler
	version            string
	i18nManager        *i18n.Manager
	csrfManager        *security.CSRFManager
	startTime          time.Time
	conversionManager  *conversion.ConversionManager

	// 动态排序器（用于自动调整端点优先级）
	dynamicSorter interface {
		ForceUpdate()
	}
}

func NewAdminServer(cfg *config.Config, endpointManager *endpoint.Manager, taggingManager *tagging.Manager, log *logger.Logger, configFilePath string, version string, i18nManager *i18n.Manager) *AdminServer {
	return &AdminServer{
		config:          cfg,
		endpointManager: endpointManager,
		taggingManager:  taggingManager,
		logger:          log,
		configFilePath:  configFilePath,
		version:         version,
		i18nManager:     i18nManager,
		csrfManager:     security.NewCSRFManager(),
		startTime:       time.Now(),
	}
}

// SetHotUpdateHandler sets the hot update handler
func (s *AdminServer) SetHotUpdateHandler(handler HotUpdateHandler) {
	s.hotUpdateHandler = handler
}

// SetPersistenceCallbacks sets the persistence handler for learned configuration
func (s *AdminServer) SetPersistenceCallbacks(handler PersistenceHandler) {
	s.persistenceHandler = handler
}

// SetDynamicSorter sets the dynamic endpoint sorter
func (s *AdminServer) SetDynamicSorter(dynamicSorter interface {
	ForceUpdate()
}) {
	s.dynamicSorter = dynamicSorter
}

// saveConfigImmediately 立即保存配置（用于用户手动操作）
func (s *AdminServer) saveConfigImmediately() error {
	// 优先使用 ConfigPersister 的 FlushNow
	if s.hotUpdateHandler != nil {
		persister := s.hotUpdateHandler.GetConfigPersister()
		if persister != nil {
			// 确保持久化器使用最新的配置指针
			persister.UpdateConfig(s.config)
			return persister.FlushNow()
		}
	}
	// 降级到直接保存
	return config.SaveConfig(s.config, s.configFilePath)
}

// PersistAuthType 持久化端点的认证类型（通过 PersistenceHandler）
func (s *AdminServer) PersistAuthType(ep *endpoint.Endpoint, authType string) {
	if s.persistenceHandler != nil {
		// 更新内存中的端点配置
		ep.AuthType = authType
		// 调用持久化处理器
		s.persistenceHandler.PersistEndpointLearning(ep)
	}
}

// PersistOpenAIPreference 持久化端点的OpenAI格式偏好（通过 PersistenceHandler）
func (s *AdminServer) PersistOpenAIPreference(ep *endpoint.Endpoint, preference string) {
	if s.persistenceHandler != nil {
		// 更新内存中的端点配置
		ep.OpenAIPreference = preference
		// 调用持久化处理器
		s.persistenceHandler.PersistEndpointLearning(ep)
	}
}

func (s *AdminServer) handleGetConversionMode(c *gin.Context) {
	if s.conversionManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversion manager not configured"})
		return
	}

	mode := s.conversionManager.GetConfiguredMode()
	effective := s.conversionManager.GetEffectiveMode()

	c.JSON(http.StatusOK, gin.H{
		"mode":                 string(mode),
		"effective_mode":       string(effective),
		"failback_threshold":   s.config.Conversion.FailbackThreshold,
		"validate_mode_switch": s.config.Conversion.ValidateModeSwitch,
	})
}

func (s *AdminServer) handleUpdateConversionMode(c *gin.Context) {
	if s.conversionManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversion manager not configured"})
		return
	}

	var req struct {
		Mode              string `json:"mode"`
		FailbackThreshold *int   `json:"failback_threshold"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	modeStr := strings.TrimSpace(req.Mode)
	if modeStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode is required"})
		return
	}

	failback := s.config.Conversion.FailbackThreshold
	if req.FailbackThreshold != nil {
		if *req.FailbackThreshold <= 0 || *req.FailbackThreshold > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failback_threshold must be between 1 and 100"})
			return
		}
		failback = *req.FailbackThreshold
	}

	managerCfg := conversion.ManagerConfig{
		Mode:              conversion.ConversionMode(strings.ToLower(modeStr)),
		FailbackThreshold: failback,
		ValidateSwitch:    s.config.Conversion.ValidateModeSwitch,
	}

	if err := s.conversionManager.ApplyConfig(managerCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新配置快照
	s.config.Conversion.AdapterMode = string(s.conversionManager.GetConfiguredMode())
	s.config.Conversion.FailbackThreshold = failback

	c.JSON(http.StatusOK, gin.H{
		"mode":           string(s.conversionManager.GetConfiguredMode()),
		"effective_mode": string(s.conversionManager.GetEffectiveMode()),
		"stats":          s.conversionManager.GetStats(),
	})
}

func (s *AdminServer) handleGetConversionStats(c *gin.Context) {
	if s.conversionManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversion manager not configured"})
		return
	}

	c.JSON(http.StatusOK, s.conversionManager.GetStats())
}

// renderHTML renders template with i18n support
func (s *AdminServer) renderHTML(c *gin.Context, templateName string, data map[string]interface{}) {
	// 调试日志
	if s.logger != nil {
		s.logger.Info("renderHTML called", map[string]interface{}{
			"template": templateName,
			"data":     fmt.Sprintf("%+v", data),
		})
	}

	// 添加recover来捕获任何panic
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Panic in renderHTML: %v", r)
			if s.logger != nil {
				s.logger.Error(errMsg, nil)
			}
			c.JSON(500, gin.H{"error": "Internal server error"})
		}
	}()

	// Always detect language fresh
	lang := s.i18nManager.GetDetector().DetectLanguage(c)
	i18n.SetLanguageToContext(c, lang)

	// If i18n is disabled or language is default, render normally
	if s.i18nManager == nil || !s.i18nManager.IsEnabled() || lang == s.i18nManager.GetDefaultLanguage() {
		if s.logger != nil {
			s.logger.Info("Rendering directly with c.HTML", map[string]interface{}{
				"template": templateName,
			})
		}

		// 在调用c.HTML之前添加更多调试信息
		if s.logger != nil {
			s.logger.Info("About to call c.HTML", map[string]interface{}{
				"template": templateName,
			})
		}

		c.HTML(200, templateName, data)

		if s.logger != nil {
			s.logger.Info("c.HTML call completed", map[string]interface{}{
				"template": templateName,
			})
		}
		return
	}

	// For non-default languages, we need to post-process
	// Create a custom writer that captures the output
	originalWriter := c.Writer
	captureWriter := &captureResponseWriter{ResponseWriter: originalWriter}
	c.Writer = captureWriter

	// Render template
	c.HTML(200, templateName, data)

	// Process the captured HTML through translator
	html := captureWriter.GetHTML()
	translator := s.i18nManager.GetTranslator()
	translatedHTML := translator.ProcessHTML(html, lang, s.i18nManager.GetTranslation)

	// Write the translated HTML to original writer
	c.Writer = originalWriter
	c.Writer.Write([]byte(translatedHTML))
}

// captureResponseWriter captures response for post-processing
type captureResponseWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *captureResponseWriter) GetHTML() string {
	return string(w.body)
}

// getBaseTemplateData returns common template data for all pages
func (s *AdminServer) getBaseTemplateData(c *gin.Context, currentPage string) map[string]interface{} {
	lang := s.i18nManager.GetDetector().DetectLanguage(c)

	// Build available languages data
	availableLanguages := make([]map[string]interface{}, 0)
	for _, availableLang := range s.i18nManager.GetAvailableLanguages() {
		langInfo := s.i18nManager.GetLanguageInfo(availableLang)
		availableLanguages = append(availableLanguages, map[string]interface{}{
			"code": string(availableLang),
			"flag": langInfo["flag"],
			"name": langInfo["name"],
		})
	}

	return map[string]interface{}{
		"Version":            s.version,
		"CurrentPage":        currentPage,
		"CurrentLanguage":    string(lang),
		"AvailableLanguages": availableLanguages,
	}
}

// mergeTemplateData merges base template data with page-specific data
func (s *AdminServer) mergeTemplateData(c *gin.Context, currentPage string, pageData map[string]interface{}) map[string]interface{} {
	baseData := s.getBaseTemplateData(c, currentPage)
	for key, value := range pageData {
		baseData[key] = value
	}
	return baseData
}

// calculateSuccessRate calculates success rate as a formatted percentage string
func calculateSuccessRate(successRequests, totalRequests int) string {
	if totalRequests == 0 {
		return "N/A"
	}
	rate := float64(successRequests) / float64(totalRequests) * 100.0
	return fmt.Sprintf("%.1f%%", rate)
}

// hotUpdateEndpoints is deprecated due to the removal of the dynamic endpoints list.
func (s *AdminServer) hotUpdateEndpoints(endpoints []interface{}) error {
	return fmt.Errorf("endpoint list hot-update is disabled due to architectural refactoring")
}

// validateEndpointsConfig is deprecated.
func (s *AdminServer) validateEndpointsConfig(endpoints []interface{}) error {
	return fmt.Errorf("endpoint list validation is disabled due to architectural refactoring")
}

// updateConfigWithRollback 执行配置更新，失败时自动回滚
func (s *AdminServer) updateConfigWithRollback(updateFunc func() error, rollbackFunc func() error) error {
	if err := updateFunc(); err != nil {
		return err
	}

	// 保存配置到文件
	if err := config.SaveConfig(s.config, s.configFilePath); err != nil {
		// 保存失败，尝试回滚
		if rollbackErr := rollbackFunc(); rollbackErr != nil {
			s.logger.Error("Failed to rollback after save error", rollbackErr)
		}
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	return nil
}

// RegisterRoutes 注册管理界面路由到指定的 router
func (s *AdminServer) RegisterRoutes(router *gin.Engine) {
	// 加载嵌入的模板
	if s.logger != nil {
		s.logger.Info("Loading embedded templates...", nil)
	}

	templates, err := webres.LoadTemplates()
	if err != nil {
		panic("Failed to load embedded templates: " + err.Error())
	}

	if s.logger != nil {
		s.logger.Info("Templates loaded successfully", map[string]interface{}{
			"template_count": len(templates.Templates()),
		})
	}

	// 注册国际化中间件（确保在其他路由配置之前）
	router.Use(s.i18nMiddleware())

	router.SetHTMLTemplate(templates)

	// 设置静态文件服务器（使用嵌入的文件系统）
	staticFS, err := webres.GetStaticFS()
	if err != nil {
		panic("Failed to get embedded static filesystem: " + err.Error())
	}
	router.StaticFS("/static", http.FS(staticFS))

	// 注册根目录帮助页面
	router.GET("/", s.handleHelpPage)

	// 注册健康检查路由（不需要认证）
	router.GET("/admin/health", s.handleHealthCheck)
	router.GET("/admin/health/diagnostics", s.handleDiagnostics)
	router.GET("/admin/health/database", s.handleDatabaseDiagnostics)

	// 注册页面路由
	router.GET("/admin/", s.handleDashboard)
	router.GET("/admin/endpoints", s.handleEndpointsPage)
	router.GET("/admin/taggers", s.handleTaggersPage)
	router.GET("/admin/logs", s.handleLogsPage)
	router.GET("/admin/settings", s.handleSettingsPage)

	// 健康检查路由（无需认证）
	router.GET("/admin/diagnostics", s.handleDiagnostics)

	// 注册 API 路由，添加UTF-8字符集中间件和CSRF防护
	api := router.Group("/admin/api")
	api.Use(s.utf8JsonMiddleware())     // 添加UTF-8中间件
	api.Use(s.csrfManager.Middleware()) // 添加CSRF防护
	{
		// CSRF token端点（GET请求，不需要CSRF验证）
		api.GET("/csrf-token", s.handleGetCSRFToken)

		api.GET("/endpoints", s.handleGetEndpoints)
		api.PUT("/endpoints", s.handleUpdateEndpoints)
		api.POST("/endpoints", s.handleCreateEndpoint)
		api.PUT("/endpoints/:id", s.handleUpdateEndpoint)
		api.PUT("/endpoints/:id/model-rewrite", s.handleUpdateEndpointModelRewrite)
		api.POST("/endpoints/:id/test-model-rewrite", s.handleTestModelRewrite)
		api.DELETE("/endpoints/:id", s.handleDeleteEndpoint)
		api.POST("/endpoints/:id/copy", s.handleCopyEndpoint)
		api.POST("/endpoints/:id/toggle", s.handleToggleEndpoint)
		api.POST("/endpoints/:id/reset-status", s.handleResetEndpointStatus)
		api.POST("/endpoints/reset-all-status", s.handleResetAllEndpointsStatus)
		api.POST("/endpoints/reorder", s.handleReorderEndpoints)
		api.POST("/endpoints/sort", s.handleSortEndpoints)
		api.POST("/endpoints/:id/test", s.handleTestEndpoint)
		api.POST("/endpoints/test-all", s.handleTestAllEndpoints)
		api.GET("/endpoints/test-all-stream", s.handleTestAllEndpointsStream)

		// 转换模式管理
		api.GET("/conversion/mode", s.handleGetConversionMode)
		api.PUT("/conversion/mode", s.handleUpdateConversionMode)
		api.GET("/conversion/stats", s.handleGetConversionStats)

		// 端点向导路由
		s.registerEndpointWizardRoutes(api)

		api.GET("/taggers", s.handleGetTaggers)
		api.POST("/taggers", s.handleCreateTagger)
		api.PUT("/taggers/:name", s.handleUpdateTagger)
		api.DELETE("/taggers/:name", s.handleDeleteTagger)
		api.GET("/tags", s.handleGetTags)

		api.GET("/logs", s.handleGetLogs)
		api.POST("/logs/cleanup", s.handleCleanupLogs)
		api.GET("/logs/stats", s.handleGetLogStats)
		api.GET("/logs/:request_id/export", s.handleExportDebugInfo)
		api.PUT("/config", s.handleHotUpdateConfig)
		api.GET("/config", s.handleGetConfig)
		api.GET("/settings", s.handleGetSettings)
		api.PUT("/settings", s.handleUpdateSettings)

		// 翻译API
		api.GET("/translations", s.handleGetTranslations)
	}
}

// utf8JsonMiddleware 确保所有JSON响应都包含UTF-8字符集声明
func (s *AdminServer) utf8JsonMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// 处理请求
		c.Next()

		// 如果响应是JSON，确保Content-Type包含UTF-8字符集
		contentType := c.Writer.Header().Get("Content-Type")
		if contentType == "application/json" {
			c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
	})
}

// i18nMiddleware provides internationalization support
func (s *AdminServer) i18nMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if s.i18nManager == nil || !s.i18nManager.IsEnabled() {
			c.Next()
			return
		}

		// Detect user's preferred language
		lang := s.i18nManager.GetDetector().DetectLanguage(c)
		i18n.SetLanguageToContext(c, lang)

		// Only apply translation for /admin/ pages
		if strings.HasPrefix(c.Request.URL.Path, "/admin/") &&
			!strings.HasPrefix(c.Request.URL.Path, "/admin/api/") {
			// Override HTML response to process translations
			originalWriter := c.Writer
			c.Writer = &translatingResponseWriter{
				ResponseWriter: originalWriter,
				lang:           lang,
				i18nManager:    s.i18nManager,
			}
		}

		c.Next()
	})
}

// translatingResponseWriter wraps gin.ResponseWriter to process translations
type translatingResponseWriter struct {
	gin.ResponseWriter
	lang        i18n.Language
	i18nManager *i18n.Manager
}

func (w *translatingResponseWriter) Write(data []byte) (int, error) {
	// Always process if it looks like HTML content
	html := string(data)
	if strings.Contains(html, "<!DOCTYPE html") || strings.Contains(html, "<html") {
		// Process translations
		translator := w.i18nManager.GetTranslator()
		translatedHTML := translator.ProcessHTML(html, w.lang, w.i18nManager.GetTranslation)
		return w.ResponseWriter.Write([]byte(translatedHTML))
	}

	return w.ResponseWriter.Write(data)
}

// handleGetCSRFToken generates and returns a new CSRF token
func (s *AdminServer) handleGetCSRFToken(c *gin.Context) {
	token := s.csrfManager.GenerateToken()
	if token == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate CSRF token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"csrf_token": token,
	})
}

// handleGetTranslations returns all available translations for client-side use
func (s *AdminServer) handleGetTranslations(c *gin.Context) {
	if s.i18nManager == nil || !s.i18nManager.IsEnabled() {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	// Get all translations from the manager
	allTranslations := s.i18nManager.GetAllTranslations()

	// Format the response for client consumption
	response := make(map[string]map[string]string)
	for lang, translations := range allTranslations {
		response[string(lang)] = translations
	}

	c.JSON(http.StatusOK, response)
}

// SetConversionManager sets the conversion manager for runtime mode control
func (s *AdminServer) SetConversionManager(manager *conversion.ConversionManager) {
	s.conversionManager = manager
}

func (s *AdminServer) handleDatabaseDiagnostics(c *gin.Context) {
	// 返回数据库诊断结果
	result := map[string]interface{}{
		"status": "success",
		"message": "Database diagnostics not implemented",
	}
	c.JSON(http.StatusOK, result)
}
