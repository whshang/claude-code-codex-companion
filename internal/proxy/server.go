package proxy

import (
	"fmt"
	"sync"
	"time"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/health"
	"claude-code-codex-companion/internal/i18n"
	"claude-code-codex-companion/internal/logger"
	"claude-code-codex-companion/internal/modelrewrite"
	"claude-code-codex-companion/internal/statistics"
	"claude-code-codex-companion/internal/tagging"
	"claude-code-codex-companion/internal/toolcall"
	"claude-code-codex-companion/internal/validator"
	"claude-code-codex-companion/internal/web"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config          *config.Config
	endpointManager *endpoint.Manager
	logger          *logger.Logger
	validator       *validator.ResponseValidator
	healthChecker   *health.Checker
	adminServer     *web.AdminServer
	taggingManager  *tagging.Manager       // 新增：tagging系统管理器
	modelRewriter   *modelrewrite.Rewriter // 新增：模型重写器
	i18nManager     *i18n.Manager          // 新增：国际化管理器
	router          *gin.Engine
	configFilePath  string
	configMutex     sync.Mutex // 新增：保护配置文件操作的互斥锁

	// Tool Calling enhancer (auto-enabled when tools provided by client)
	toolEnhancer *toolcall.Enhancer
}

func NewServer(cfg *config.Config, configFilePath string, version string) (*Server, error) {
	logConfig := logger.LogConfig{
		Level:           cfg.Logging.Level,
		LogRequestTypes: cfg.Logging.LogRequestTypes,
		LogRequestBody:  cfg.Logging.LogRequestBody,
		LogResponseBody: cfg.Logging.LogResponseBody,
		LogDirectory:    cfg.Logging.LogDirectory,
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

	// 初始化tagging系统
	taggingManager := tagging.NewManager()
	if err := taggingManager.Initialize(&cfg.Tagging); err != nil {
		return nil, fmt.Errorf("failed to initialize tagging system: %v", err)
	}

	// 初始化模型重写器
	modelRewriter := modelrewrite.NewRewriter(*log)

	// 初始化健康检查器（需要在模型重写器之后）
	healthChecker := health.NewChecker(cfg.Timeouts.ToHealthCheckTimeoutConfig(), modelRewriter)

	// 初始化国际化管理器
	i18nConfig := &i18n.Config{
		DefaultLanguage: i18n.Language(cfg.I18n.DefaultLanguage),
		LocalesPath:     cfg.I18n.LocalesPath,
		Enabled:         cfg.I18n.Enabled,
	}
	// 如果配置为空，使用默认配置
	if cfg.I18n.DefaultLanguage == "" {
		i18nConfig = i18n.DefaultConfig()
	}

	i18nManager, err := i18n.NewManager(i18nConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize i18n manager: %v", err)
	}

	// 创建管理界面服务器（永远启用）
	adminServer := web.NewAdminServer(cfg, endpointManager, taggingManager, log, configFilePath, version, i18nManager)

	server := &Server{
		config:          cfg,
		endpointManager: endpointManager,
		logger:          log,
		validator:       responseValidator,
		healthChecker:   healthChecker,
		adminServer:     adminServer,
		taggingManager:  taggingManager, // 新增：设置tagging管理器
		modelRewriter:   modelRewriter,  // 新增：设置模型重写器
		i18nManager:     i18nManager,    // 新增：设置国际化管理器
		configFilePath:  configFilePath,
	}

	// Initialize tool enhancer with sensible defaults (zero-config enable when tools present)
	// Use defaults from config.Default, with fallbacks
	// Zero-config defaults for tool calling enhancer
	ttl := 30 * time.Minute
	server.toolEnhancer = toolcall.NewEnhancer(toolcall.CacheConfig{
		MaxSize: 100,
		TTL:     ttl,
	})

	// 设置持久化回调，让AdminServer可以被Server调用
	adminServer.SetPersistenceCallbacks(server)

	// 设置热更新处理器
	adminServer.SetHotUpdateHandler(server)

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

	// 注册管理界面路由（不需要认证）
	s.adminServer.RegisterRoutes(s.router)

	// 为 API 端点添加日志中间件
	apiGroup := s.router.Group("/v1")
	apiGroup.Use(s.loggingMiddleware())
	{
		apiGroup.Any("/*path", s.handleProxy)
	}

	// 支持 Codex 的 /responses 路径
	s.router.Any("/responses", s.loggingMiddleware(), s.handleProxy)
	s.router.Any("/chat/completions", s.loggingMiddleware(), s.handleProxy)
}

func (s *Server) Start() error {
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

	// 更新内存中的配置（需要锁保护，因为可能与其他配置更新并发）
	s.configMutex.Lock()
	s.config = newConfig
	s.configMutex.Unlock()

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

	return nil
}

// updateValidatorConfig updates response validator configuration
func (s *Server) updateValidatorConfig(newValidation config.ValidationConfig) {
	s.validator = validator.NewResponseValidator()
	s.config.Validation = newValidation
}

// saveConfigToFile 将当前配置保存到文件（线程安全）
func (s *Server) saveConfigToFile() error {
	// 注意：这个方法假设调用者已经持有 configMutex
	return config.SaveConfig(s.config, s.configFilePath)
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

// PersistEndpointLearning 持久化端点学习结果到配置文件
// 这个方法会被 proxy_logic.go 在运行时学习成功后调用
func (s *Server) PersistEndpointLearning(ep *endpoint.Endpoint) {
	if ep == nil {
		return
	}

	// 线程安全：获取端点当前的学习状态
	ep.AuthHeaderMutex.RLock()
	detectedAuthHeader := ep.DetectedAuthHeader
	ep.AuthHeaderMutex.RUnlock()

	openAIPreference := ep.OpenAIPreference
	nativeToolSupport := ep.NativeToolSupport
	toolEnhMode := ep.ToolEnhancementMode
	countTokensEnabled := ep.CountTokensEnabled

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
		// 检查配置中是否已经有这个偏好设置
		s.configMutex.Lock()
		configPreference := ""
		for i := range s.config.Endpoints {
			if s.config.Endpoints[i].Name == ep.Name {
				configPreference = s.config.Endpoints[i].OpenAIPreference
				break
			}

			// 3. 持久化原生工具调用支持（当有学习结果或明确设置时）
			if nativeToolSupport != nil {
				s.logger.Info(fmt.Sprintf("🧩 Learning: Detected native tool support=%v for endpoint '%s'", *nativeToolSupport, ep.Name), nil)
				if err := s.updateEndpointConfig(ep.Name, func(cfg *config.EndpointConfig) error {
					cfg.NativeToolSupport = nativeToolSupport
					return nil
				}); err != nil {
					s.logger.Error(fmt.Sprintf("Failed to persist native tool support for endpoint '%s'", ep.Name), err)
				} else {
					needsPersist = true
				}
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

	// 4. count_tokens 可用性（仅在需要时持久化）
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

	// 5. 工具增强模式（如果被设置，持久化）
	if toolEnhMode != "" {
		if err := s.updateEndpointConfig(ep.Name, func(cfg *config.EndpointConfig) error {
			cfg.ToolEnhancementMode = toolEnhMode
			return nil
		}); err != nil {
			s.logger.Error(fmt.Sprintf("Failed to persist tool enhancement mode for endpoint '%s'", ep.Name), err)
		} else {
			s.logger.Info(fmt.Sprintf("✓ Persisted tool enhancement mode '%s' for endpoint '%s'", toolEnhMode, ep.Name), nil)
		}
	}
}
