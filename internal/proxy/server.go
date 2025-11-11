package proxy

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/database"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/health"
	"claude-code-codex-companion/internal/logger"
	"claude-code-codex-companion/internal/modelrewrite"
	"claude-code-codex-companion/internal/statistics"
	"claude-code-codex-companion/internal/utils" // 新增：导入utils包
	"claude-code-codex-companion/internal/validator"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config          *config.Config
	endpointManager *endpoint.Manager
	logger          *logger.Logger
	validator       *validator.ResponseValidator
	healthChecker   *health.Checker
	modelRewriter   *modelrewrite.Rewriter // 新增：模型重写器
	router          *gin.Engine
	configFilePath  string
	configMutex     sync.Mutex // 新增：保护配置文件操作的互斥锁

	// Conversion manager for format adaptation
	conversionManager *conversion.ConversionManager

	// 动态端点排序器
	dynamicSorter *utils.DynamicEndpointSorter

	// 配置持久化管理器
	configPersister *config.ConfigPersister

	// 错误模式匹配器
	errorPatternMatcher *ErrorPatternMatcher
}

func NewServer(cfg *config.Config, configFilePath string, version string) (*Server, error) {
	// 获取全局数据库管理器
	dbManager, err := database.GetGlobalManager()
	if err != nil {
		return nil, fmt.Errorf("failed to get database manager: %w", err)
	}

    // 统一日志与统计目录：将配置中的日志目录覆盖为数据库管理器所在目录
    // 确保 Endpoint 统计与 GORM 日志使用同一目录（即 dataDir）
    cfg.Logging.LogDirectory = filepath.Dir(dbManager.GetLogsDBPath())

	// 使用统一数据库管理器的日志路径
    logConfig := logger.LogConfig{
		Level:           cfg.Logging.Level,
		LogRequestTypes: cfg.Logging.LogRequestTypes,
		LogRequestBody:  cfg.Logging.LogRequestBody,
		LogResponseBody: cfg.Logging.LogResponseBody,
        LogDirectory:    filepath.Dir(dbManager.GetLogsDBPath()),
		ExcludePaths:    cfg.Logging.ExcludePaths,
	}

	log, err := logger.NewLogger(logConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	endpointManager, err := endpoint.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize endpoint manager: %v", err)
	}
	responseValidator := validator.NewResponseValidator()

	// 初始化模型重写器
	modelRewriter := modelrewrite.NewRewriter(*log)

	// 初始化健康检查器（需要在模型重写器之后）
	healthChecker := health.NewChecker(cfg.Timeouts.ToHealthCheckTimeoutConfig(), modelRewriter, config.Default.HealthCheck.Model)

	manager := conversion.NewConversionManager(log, conversion.ManagerConfig{
		Mode:              conversion.ConversionMode(cfg.Conversion.AdapterMode),
		FailbackThreshold: cfg.Conversion.FailbackThreshold,
		ValidateSwitch:    cfg.Conversion.ValidateModeSwitch,
	})

	server := &Server{
		config:          cfg,
		endpointManager: endpointManager,
		logger:          log,
		validator:       responseValidator,
		healthChecker:   healthChecker,
		modelRewriter:   modelRewriter, // 新增：设置模型重写器
		configFilePath:  configFilePath,
	}

	// 初始化动态端点排序器
	server.dynamicSorter = utils.NewDynamicEndpointSorter()

	// 创建配置持久化管理器
	flushInterval := 30 * time.Second // 默认30秒
	maxDirtyTime := 5 * time.Minute   // 默认5分钟

	// 从配置中读取自定义值
	if cfg.Server.ConfigFlushInterval != "" {
		if duration, err := time.ParseDuration(cfg.Server.ConfigFlushInterval); err == nil {
			flushInterval = duration
		} else {
			log.Error("Invalid config_flush_interval, using default 30s", err)
		}
	}
	if cfg.Server.ConfigMaxDirtyTime != "" {
		if duration, err := time.ParseDuration(cfg.Server.ConfigMaxDirtyTime); err == nil {
			maxDirtyTime = duration
		} else {
			log.Error("Invalid config_max_dirty_time, using default 5m", err)
		}
	}

	persister := config.NewConfigPersister(cfg, configFilePath, &config.PersisterConfig{
		FlushInterval: flushInterval,
		MaxDirtyTime:  maxDirtyTime,
		BeforeWrite: func(c *config.Config) error {
			// 写入前验证
			if len(c.Endpoints) == 0 {
				return fmt.Errorf("configuration must have at least one endpoint")
			}
			return nil
		},
		AfterWrite: func(c *config.Config) error {
			// 写入后通知
			log.Info("✅ Configuration successfully persisted")
			return nil
		},
	})

	server.configPersister = persister

	// 启动持久化管理器
	persister.Start()

	server.conversionManager = manager

	// 初始化错误模式匹配器
	server.errorPatternMatcher = NewErrorPatternMatcher()

	// 设置动态排序器的持久化回调
	server.dynamicSorter.SetPersistCallback(func() error {
		return server.PersistEndpointPriorityChanges()
	})

	// 让端点管理器使用同一个健康检查器
	endpointManager.SetHealthChecker(healthChecker)

	server.setupRoutes()
	return server, nil
}

func (s *Server) setupRoutes() {
	gin.SetMode(gin.ReleaseMode)

	s.router = gin.New()
	s.router.Use(gin.Recovery())
	s.router.UseRawPath = true
	s.router.UnescapePathValues = false

	// 添加CORS中间件
	s.router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 为 API 端点添加日志中间件
	apiGroup := s.router.Group("/v1")
	apiGroup.Use(s.loggingMiddleware())
	{
		apiGroup.Any("/*path", s.handleProxy)
	}

	// 支持 Codex 的 /responses 路径
	s.router.Any("/responses", s.loggingMiddleware(), s.handleProxy)
	s.router.Any("/chat/completions", s.loggingMiddleware(), s.handleProxy)

	// 支持模型列表 API（由 handleProxy 内部特殊处理）
}

// Start starts the proxy server
func (s *Server) Start() error {
	// 🔥 VERSION CHECK: 确认代码已编译
	s.logger.Info("🚀🚀🚀 PROXY SERVER VERSION: PATH_CONVERSION_FIX_v2 🚀🚀🚀")
	
	// 根据配置启用动态排序
	if s.config.Server.AutoSortEndpoints {
		s.dynamicSorter.Enable()
		// 将端点转换为动态端点类型并设置引用
		dynamicEndpoints := make([]utils.DynamicEndpoint, 0)
		for _, ep := range s.endpointManager.GetAllEndpoints() {
			ep.SetDynamicSorter(s.dynamicSorter)
			dynamicEndpoints = append(dynamicEndpoints, ep)
		}
		s.dynamicSorter.SetEndpoints(dynamicEndpoints)
		s.logger.Info("✅ 启用动态端点排序功能")
	} else {
		s.logger.Info("ℹ️ 动态端点排序功能已禁用")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	s.logger.Info(fmt.Sprintf("Starting proxy server on %s:%d", s.config.Server.Host, s.config.Server.Port))
	return s.router.Run(addr)
}

func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

func (s *Server) GetEndpointManager() *endpoint.Manager {
	return s.endpointManager
}

func (s *Server) GetLogger() *logger.Logger {
	return s.logger
}

func (s *Server) GetHealthChecker() *health.Checker {
	return s.healthChecker
}

// HotUpdateConfig safely updates configuration without restarting the server
func (s *Server) HotUpdateConfig(newConfig *config.Config) error {
	// 验证新配置
	if err := s.validateConfigForHotUpdate(newConfig); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	s.logger.Info("Starting configuration hot update")

	// 更新端点配置
	if err := s.updateEndpoints(newConfig.Endpoints); err != nil {
		return fmt.Errorf("failed to update endpoints: %v", err)
	}

	// 更新日志配置（如果可能）
	if err := s.updateLoggingConfig(newConfig.Logging); err != nil {
		s.logger.Error("Failed to update logging config, continuing with endpoint updates", err)
	}

	// 更新验证器配置
	s.updateValidatorConfig(newConfig.Validation)

	// 更新黑名单配置
	s.updateBlacklistConfig(newConfig.Blacklist)

	// 更新内存中的配置（需要锁保护，因为可能与其他配置更新并发）
	s.configMutex.Lock()
	s.config = newConfig
	s.configMutex.Unlock()

	if s.configPersister != nil {
		s.configPersister.UpdateConfig(newConfig)
	}

	if s.conversionManager != nil {
		if err := s.conversionManager.ApplyConfig(conversion.ManagerConfig{
			Mode:              conversion.ConversionMode(newConfig.Conversion.AdapterMode),
			FailbackThreshold: newConfig.Conversion.FailbackThreshold,
			ValidateSwitch:    newConfig.Conversion.ValidateModeSwitch,
		}); err != nil {
			s.logger.Error("Failed to apply conversion configuration during hot update", err)
		}
	}

	s.logger.Info("Configuration hot update completed successfully")
	return nil
}

// validateConfigForHotUpdate validates the new configuration
func (s *Server) validateConfigForHotUpdate(newConfig *config.Config) error {
	// 检查是否尝试修改不可热更新的配置
	if newConfig.Server.Host != s.config.Server.Host {
		return fmt.Errorf("server host cannot be changed via hot update")
	}
	if newConfig.Server.Port != s.config.Server.Port {
		return fmt.Errorf("server port cannot be changed via hot update")
	}

	// 验证端点配置
	if len(newConfig.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be configured")
	}

	return nil
}

// updateEndpoints updates endpoint configuration
func (s *Server) updateEndpoints(newEndpoints []config.EndpointConfig) error {
	s.endpointManager.UpdateEndpoints(newEndpoints)

	// 如果动态排序已启用，需要更新dynamicSorter的endpoints列表
	if s.config.Server.AutoSortEndpoints && s.dynamicSorter != nil {
		dynamicEndpoints := make([]utils.DynamicEndpoint, 0)
		for _, ep := range s.endpointManager.GetAllEndpoints() {
			ep.SetDynamicSorter(s.dynamicSorter)
			dynamicEndpoints = append(dynamicEndpoints, ep)
		}
		s.dynamicSorter.SetEndpoints(dynamicEndpoints)
		s.logger.Info("🔄 动态排序器的端点列表已更新")
	}

	return nil
}

// updateLoggingConfig updates logging configuration if possible
func (s *Server) updateLoggingConfig(newLogging config.LoggingConfig) error {
	// 目前只能更新日志级别和记录策略，不能更换日志目录
	if newLogging.LogDirectory != s.config.Logging.LogDirectory {
		return fmt.Errorf("log directory cannot be changed via hot update")
	}

	// 可以安全更新的日志配置
	s.config.Logging.Level = newLogging.Level
	s.config.Logging.LogRequestTypes = newLogging.LogRequestTypes
	s.config.Logging.LogRequestBody = newLogging.LogRequestBody
	s.config.Logging.LogResponseBody = newLogging.LogResponseBody
	s.config.Logging.ExcludePaths = newLogging.ExcludePaths

	// 更新logger的配置
	s.logger.UpdateConfig(logger.LogConfig{
		Level:           newLogging.Level,
		LogRequestTypes: newLogging.LogRequestTypes,
		LogRequestBody:  newLogging.LogRequestBody,
		LogResponseBody: newLogging.LogResponseBody,
		LogDirectory:    newLogging.LogDirectory,
		ExcludePaths:    newLogging.ExcludePaths,
	})

	return nil
}

// updateValidatorConfig updates response validator configuration
func (s *Server) updateValidatorConfig(newValidation config.ValidationConfig) {
	s.validator = validator.NewResponseValidator()
	s.config.Validation = newValidation
}

// updateBlacklistConfig updates blacklist configuration
func (s *Server) updateBlacklistConfig(newBlacklist config.BlacklistConfig) {
	s.config.Blacklist = newBlacklist
}

// saveConfigToFile 将当前配置保存到文件（线程安全）
func (s *Server) saveConfigToFile() error {
	// 注意：这个方法假设调用者已经持有 configMutex
	// 如果 ConfigPersister 存在，使用它（立即写入）
	if s.configPersister != nil {
		return s.configPersister.FlushNow()
	}
	// 否则直接保存（兼容旧代码）
	return config.SaveConfig(s.config, s.configFilePath)
}

// GetConfigPersister 获取配置持久化管理器
func (s *Server) GetConfigPersister() *config.ConfigPersister {
	return s.configPersister
}

// Shutdown 优雅关闭服务器，确保所有待处理的配置被保存
func (s *Server) Shutdown() error {
	s.logger.Info("Shutting down server...")

	// 停止配置持久化管理器（会自动写入未保存的变更）
	if s.configPersister != nil {
		if err := s.configPersister.Stop(); err != nil {
			s.logger.Error("Failed to stop config persister", err)
			return err
		}
	}

	// 禁用动态排序器
	if s.dynamicSorter != nil {
		s.dynamicSorter.Disable()
	}

	s.logger.Info("Server shutdown complete")
	return nil
}

// updateEndpointConfig 安全地更新指定端点的配置并持久化
func (s *Server) updateEndpointConfig(endpointName string, updateFunc func(*config.EndpointConfig) error) error {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	// 查找对应的端点配置
	for i, cfgEndpoint := range s.config.Endpoints {
		if cfgEndpoint.Name == endpointName {
			// 应用更新函数
			if err := updateFunc(&s.config.Endpoints[i]); err != nil {
				return err
			}

			// 保存到配置文件
			return s.saveConfigToFile()
		}
	}

	return fmt.Errorf("endpoint not found: %s", endpointName)
}

// createOAuthTokenRefreshCallback 创建 OAuth token 刷新后的回调函数
func (s *Server) createOAuthTokenRefreshCallback() func(*endpoint.Endpoint) error {
	return func(ep *endpoint.Endpoint) error {
		// 使用统一的配置更新机制
		return s.updateEndpointConfig(ep.Name, func(cfg *config.EndpointConfig) error {
			cfg.OAuthConfig = ep.OAuthConfig
			return nil
		})
	}
}

// persistRateLimitState 持久化endpoint的rate limit状态到配置文件
func (s *Server) persistRateLimitState(endpointID string, reset *int64, status *string) error {
	// 首先根据endpoint ID找到对应的endpoint名称
	var endpointName string
	s.configMutex.Lock()
	for _, cfgEndpoint := range s.config.Endpoints {
		if statistics.GenerateEndpointID(cfgEndpoint.Name) == endpointID {
			endpointName = cfgEndpoint.Name
			break
		}
	}
	s.configMutex.Unlock()

	if endpointName == "" {
		return fmt.Errorf("endpoint with ID %s not found", endpointID)
	}

	// 使用统一的配置更新机制
	return s.updateEndpointConfig(endpointName, func(cfg *config.EndpointConfig) error {
		cfg.RateLimitReset = reset
		cfg.RateLimitStatus = status
		return nil
	})
}

// PersistEndpointPriorityChanges 持久化端点优先级更改到配置文件
// 注意：此方法由 DynamicSorter 调用，应标记为脏数据而非立即写入
func (s *Server) PersistEndpointPriorityChanges() error {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	// 获取所有端点并按优先级排序
	endpoints := s.endpointManager.GetAllEndpoints()

	// 创建端点名称到优先级的映射
	priorityMap := make(map[string]int)
	for _, ep := range endpoints {
		// 所有端点（无论启用或禁用）的优先级都需要持久化
		priorityMap[ep.Name] = ep.GetPriority()
	}

	// 更新配置中的端点优先级
	updated := false
	for i, cfgEndpoint := range s.config.Endpoints {
		if priority, exists := priorityMap[cfgEndpoint.Name]; exists {
			if s.config.Endpoints[i].Priority != priority {
				s.config.Endpoints[i].Priority = priority
				updated = true
				s.logger.Info(fmt.Sprintf("🔄 更新端点 '%s' 的优先级为 %d", cfgEndpoint.Name, priority))
			}
		}
	}

	// 如果有更新，标记为脏数据（由动态排序触发，不立即写入）
	if updated && s.configPersister != nil {
		s.configPersister.MarkDirty()
	}

	return nil
}

// PersistEndpointLearning 持久化端点学习到的配置
func (s *Server) PersistEndpointLearning(ep *endpoint.Endpoint) {
	// 线程安全：获取端点当前的学习状态
	ep.AuthHeaderMutex.RLock()
	detectedAuthHeader := ep.DetectedAuthHeader
	ep.AuthHeaderMutex.RUnlock()

	openAIPreference := ep.OpenAIPreference
	countTokensEnabled := ep.CountTokensEnabled
	supportsResponses := ep.NativeCodexFormat
	if supportsResponses == nil {
		ep.SupportsResponses = nil
	}

	// 调用 AdminServer 的持久化方法
	// 只有在学习到新信息时才持久化
	needsPersist := false

	// 1. 检查认证方式是否需要持久化
	if detectedAuthHeader != "" && (ep.AuthType == "" || ep.AuthType == "auto") {
		// 从检测到的头部类型推断认证类型
		var authType string
		if detectedAuthHeader == "api_key" || detectedAuthHeader == "x-api-key" {
			authType = "api_key"
		} else {
			authType = "auth_token"
		}

		// 只有当配置中的认证类型与检测到的不同时才更新
		if ep.AuthType != authType {
			s.logger.Info(fmt.Sprintf("🔐 Learning: Detected auth type '%s' for endpoint '%s'", authType, ep.Name), nil)

			// 使用统一的配置更新机制持久化认证类型
			if err := s.updateEndpointConfig(ep.Name, func(cfg *config.EndpointConfig) error {
				cfg.AuthType = authType
				return nil
			}); err != nil {
				s.logger.Error(fmt.Sprintf("Failed to persist auth type for endpoint '%s'", ep.Name), err)
			} else {
				s.logger.Info(fmt.Sprintf("✓ Persisted auth type '%s' for endpoint '%s'", authType, ep.Name), nil)
				needsPersist = true
			}
		}
	}

	// 2. 检查 OpenAI 格式偏好是否需要持久化
	if openAIPreference != "" && openAIPreference != "auto" {
		s.configMutex.Lock()
		configPreference := ""
		for i := range s.config.Endpoints {
			if s.config.Endpoints[i].Name == ep.Name {
				configPreference = s.config.Endpoints[i].OpenAIPreference
				break
			}
		}
		s.configMutex.Unlock()

		// 只有当配置中的偏好与当前学习到的不同时才更新
		if configPreference == "" || configPreference == "auto" || configPreference != openAIPreference {
			s.logger.Info(fmt.Sprintf("🔍 Learning: Detected OpenAI format preference '%s' for endpoint '%s'", openAIPreference, ep.Name), nil)

			// 使用统一的配置更新机制持久化 OpenAI 格式偏好
			if err := s.updateEndpointConfig(ep.Name, func(cfg *config.EndpointConfig) error {
				cfg.OpenAIPreference = openAIPreference
				return nil
			}); err != nil {
				s.logger.Error(fmt.Sprintf("Failed to persist OpenAI preference for endpoint '%s'", ep.Name), err)
			} else {
				s.logger.Info(fmt.Sprintf("✓ Persisted OpenAI preference '%s' for endpoint '%s'", openAIPreference, ep.Name), nil)
				needsPersist = true
			}
		}
	}

	// 3. 持久化对 /responses 支持的学习结果
	if supportsResponses != nil {
		var configSupportsValue bool
		configSupportsSet := false
		s.configMutex.Lock()
		for i := range s.config.Endpoints {
			if s.config.Endpoints[i].Name == ep.Name {
				if s.config.Endpoints[i].SupportsResponses != nil {
					configSupportsValue = *s.config.Endpoints[i].SupportsResponses
					configSupportsSet = true
				}
				break
			}
		}
		s.configMutex.Unlock()

		if !configSupportsSet || configSupportsValue != *supportsResponses {
			s.logger.Info(fmt.Sprintf("🧭 Learning: Detected supports_responses=%v for endpoint '%s'", *supportsResponses, ep.Name), nil)
			supported := *supportsResponses
			if err := s.updateEndpointConfig(ep.Name, func(cfg *config.EndpointConfig) error {
				ptr := new(bool)
				*ptr = supported
				cfg.SupportsResponses = ptr
				return nil
			}); err != nil {
				s.logger.Error(fmt.Sprintf("Failed to persist supports_responses for endpoint '%s'", ep.Name), err)
			} else {
				s.logger.Info(fmt.Sprintf("✓ Persisted supports_responses=%v for endpoint '%s'", supported, ep.Name), nil)
				needsPersist = true
				copyVal := supported
				ep.SupportsResponses = &copyVal
			}
		}
	}

	// 5. count_tokens 可用性（仅在需要时持久化）
	if !countTokensEnabled {
		if err := s.updateEndpointConfig(ep.Name, func(cfg *config.EndpointConfig) error {
			if cfg.CountTokensEnabled != nil && *cfg.CountTokensEnabled == countTokensEnabled {
				return nil
			}
			ptr := new(bool)
			*ptr = countTokensEnabled
			cfg.CountTokensEnabled = ptr
			return nil
		}); err != nil {
			s.logger.Error(fmt.Sprintf("Failed to persist count_tokens flag for endpoint '%s'", ep.Name), err)
		} else {
			s.logger.Info(fmt.Sprintf("✓ Persisted count_tokens_enabled=%v for endpoint '%s'", countTokensEnabled, ep.Name), nil)
			needsPersist = true
		}
	}

	if needsPersist {
		s.logger.Info(fmt.Sprintf("🎓 Successfully persisted learned configuration for endpoint '%s'", ep.Name), nil)
	}
}
