package web

import (
	"fmt"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/i18n"
	"claude-code-codex-companion/internal/logger"
	"claude-code-codex-companion/internal/tagging"
)

// RunEndpointBatchTest 在非HTTP上下文中执行批量端点连通性测试，返回测试结果。
// configPath 用于日志输出信息，可传空字符串。
func RunEndpointBatchTest(cfg *config.Config, configPath string) ([]*BatchTestResult, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is nil")
	}

	// 初始化日志器
	logCfg := logger.LogConfig{
		Level:           cfg.Logging.Level,
		LogRequestTypes: cfg.Logging.LogRequestTypes,
		LogRequestBody:  cfg.Logging.LogRequestBody,
		LogResponseBody: cfg.Logging.LogResponseBody,
		LogDirectory:    cfg.Logging.LogDirectory,
	}

	appLogger, err := logger.NewLogger(logCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	cleanup := func() error {
		if appLogger != nil {
			return appLogger.Close()
		}
		return nil
	}

	// 初始化 Tagging
	taggingManager := tagging.NewManager()
	if err := taggingManager.Initialize(&cfg.Tagging); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to initialize tagging manager: %w", err)
	}

	// 初始化 Endpoint Manager
	endpointManager, err := endpoint.NewManager(cfg)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to initialize endpoint manager: %w", err)
	}

	// 初始化 I18n（按配置，如果失败则降级为禁用）
	i18nManager, err := i18n.NewManager(&i18n.Config{
		Enabled:         cfg.I18n.Enabled,
		DefaultLanguage: i18n.Language(cfg.I18n.DefaultLanguage),
		LocalesPath:     cfg.I18n.LocalesPath,
	})
	if err != nil {
		// 日志记录后继续（i18n 仅用于界面）
		appLogger.Error("failed to initialize i18n manager", err)
		i18nManager, _ = i18n.NewManager(&i18n.Config{Enabled: false})
	}

	adminServer := NewAdminServer(cfg, endpointManager, taggingManager, appLogger, configPath, "cli", i18nManager)

	results := adminServer.testAllEndpoints()
	return results, cleanup, nil
}
