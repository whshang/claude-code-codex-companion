package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/conversion"
	"claude-code-codex-companion/internal/database"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/health"
	logger "claude-code-codex-companion/internal/logger"
	"claude-code-codex-companion/internal/modelrewrite"
	"claude-code-codex-companion/internal/utils"
)

const (
	defaultProxyHost = "127.0.0.1"
	defaultProxyPort = 8080
)

// 进程绑定管理器 - 使用Wails自动生成的BindingManager

// 日志条目结构
type LogEntry struct {
	Timestamp    string `json:"timestamp"`
	Level        string `json:"level"`
	Message      string `json:"message"`
	RequestID    string `json:"requestId,omitempty"`
	ClientType   string `json:"clientType,omitempty"`
	EndpointID   string `json:"endpointId,omitempty"`
	Model        string `json:"model,omitempty"`
	Status       string `json:"status,omitempty"`
	ResponseTime int    `json:"responseTime,omitempty"`
	RequestSize  int    `json:"requestSize,omitempty"`
	ResponseSize int    `json:"responseSize,omitempty"`
}

// App struct - 统一路由架构，无HTTP服务器
type App struct {
	ctx           context.Context
	mutex         sync.RWMutex
	running       bool
	dbManager     *database.Manager // 统一数据库管理器（3-DB架构：main.db/logs.db/statistics.db）
	db            *sql.DB
	configPath    string
	config        map[string]interface{} // 配置缓存
	logs          []LogEntry             // 内存日志存储
	requestLogger *logger.Logger
	modelRewriter *modelrewrite.Rewriter
	healthChecker *health.Checker

	proxyHost      string
	proxyPort      int
	configuredHost string
	configuredPort int
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		proxyHost:      defaultProxyHost,
		proxyPort:      defaultProxyPort,
		configuredHost: defaultProxyHost,
		configuredPort: defaultProxyPort,
	}
}

func parsePortValue(value interface{}) int {
	port := defaultProxyPort

	switch v := value.(type) {
	case float64:
		port = int(v)
	case float32:
		port = int(v)
	case int:
		port = v
	case int32:
		port = int(v)
	case int64:
		port = int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			port = int(parsed)
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			break
		}
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			port = parsed
		}
	}

	if port <= 0 || port > 65535 {
		port = defaultProxyPort
	}

	return port
}

func normalizeHostValue(value interface{}) string {
	if value == nil {
		return defaultProxyHost
	}

	if hostStr, ok := value.(string); ok {
		if trimmed := strings.TrimSpace(hostStr); trimmed != "" {
			return trimmed
		}
	}

	return defaultProxyHost
}

// applyServerAddressNoLock assumes caller already holds the mutex.
func (a *App) applyServerAddressNoLock(server map[string]interface{}) {
	host := defaultProxyHost
	port := defaultProxyPort

	if server != nil {
		if hostVal, exists := server["host"]; exists {
			host = normalizeHostValue(hostVal)
		}
		if portVal, exists := server["port"]; exists {
			port = parsePortValue(portVal)
		}

		server["host"] = host
		server["port"] = port
	}

	a.configuredHost = host
	a.configuredPort = port
}

func (a *App) syncActualAddressNoLock() {
	a.proxyHost = a.configuredHost
	a.proxyPort = a.configuredPort
}

func (a *App) getEffectiveProxyAddress() (string, int) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	host := strings.TrimSpace(a.proxyHost)
	if host == "" {
		host = defaultProxyHost
	}

	port := a.proxyPort
	if port <= 0 || port > 65535 {
		port = defaultProxyPort
	}

	return host, port
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.running = false

	// 设置配置文件路径到用户目录
	if configPath, err := a.getConfigPath(); err == nil {
		a.configPath = configPath
	} else {
		a.configPath = "./config.json" // 回退到默认路径
	}

	a.logs = []LogEntry{} // 初始化日志存储

	// 初始化绑定管理器 - 使用Wails自动生成的代码
	if err := a.InitializeBindingManager(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to initialize binding manager: %v", err))
		a.addLog("error", fmt.Sprintf("绑定管理器初始化失败: %v", err))
	} else {
		bindingInfo := a.GetBindingInfo()
		runtime.LogInfo(a.ctx, fmt.Sprintf("✅ Process binding initialized - PID: %d, Port: %d", bindingInfo.PID, bindingInfo.Port))
		a.addLog("info", fmt.Sprintf("进程绑定初始化成功 - PID: %d, 实例: %s", bindingInfo.PID, bindingInfo.AppInstance))
	}

	// 添加启动日志
	a.addLog("info", "统一路由架构已启动")
	a.addLog("info", "前端通过Go API与后端通信")

	// 初始化统一数据库管理器
	if err := a.initDatabaseManager(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to initialize database manager: %v", err))
		a.addLog("error", fmt.Sprintf("数据库管理器初始化失败: %v", err))
	} else {
		runtime.LogInfo(a.ctx, "Database manager initialized successfully")
		a.addLog("info", "数据库管理器初始化成功")
	}

	// 初始化数据库
	if err := a.initDatabase(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to initialize database: %v", err))
		a.addLog("error", fmt.Sprintf("数据库初始化失败: %v", err))
	} else {
		runtime.LogInfo(a.ctx, "Database initialized successfully")
		a.addLog("info", "数据库初始化成功")
	}

	// 初始化请求日志记录器
	if err := a.initRequestLogger(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to initialize request logger: %v", err))
		a.addLog("error", fmt.Sprintf("请求日志记录器初始化失败: %v", err))
	} else {
		runtime.LogInfo(a.ctx, "Request logger initialized successfully")
		a.addLog("info", "请求日志记录器初始化成功")
	}

	// 初始化模型重写器与健康检查器
	if err := a.initModelRewriterAndHealthChecker(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to initialize health checker: %v", err))
		a.addLog("error", fmt.Sprintf("健康检查器初始化失败: %v", err))
	} else {
		runtime.LogInfo(a.ctx, "Health checker initialized successfully")
		a.addLog("info", "健康检查器初始化成功")
	}

	runtime.LogInfo(a.ctx, "CCCC Desktop App startup completed")
	runtime.LogInfo(a.ctx, "✅ 统一路由架构已启用 - 无HTTP服务器冲突")
	runtime.LogInfo(a.ctx, "✅ 前端将通过Go API与后端通信")

	a.addLog("info", "CCCC Desktop App 启动完成")
	a.addLog("info", "✅ 统一路由架构已启用 - 无HTTP服务器冲突")
	a.addLog("info", "✅ 前端将通过Go API与后端通信")
}

// addLog 添加日志条目
func (a *App) addLog(level, message string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Level:     level,
		Message:   message,
	}

	a.logs = append(a.logs, entry)

	// 保持日志数量在合理范围内（最多1000条）
	if len(a.logs) > 1000 {
		a.logs = a.logs[1:]
	}
}

func (a *App) initRequestLogger() error {
	if a.requestLogger != nil {
		return nil
	}

	if a.dbManager == nil {
		return fmt.Errorf("database manager not initialized")
	}

	// 使用统一数据库管理器获取日志数据库目录
	logsDBPath := a.dbManager.GetLogsDBPath()
	logDir := filepath.Dir(logsDBPath)

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	config := logger.LogConfig{
		Level:           "info",
		LogRequestTypes: "all",
		LogRequestBody:  "truncated",
		LogResponseBody: "truncated",
		LogDirectory:    logDir,
	}

	l, err := logger.NewLogger(config)
	if err != nil {
		return err
	}

	a.requestLogger = l
	runtime.LogInfo(a.ctx, fmt.Sprintf("Request logger initialized with log directory: %s", logDir))
	a.addLog("info", fmt.Sprintf("请求日志记录器初始化完成，日志目录: %s", logDir))

	return nil
}

func (a *App) initModelRewriterAndHealthChecker() error {
	if a.requestLogger == nil {
		if err := a.initRequestLogger(); err != nil {
			return err
		}
	}

	if a.modelRewriter == nil && a.requestLogger != nil {
		a.modelRewriter = modelrewrite.NewRewriter(*a.requestLogger)
	}

	if a.healthChecker == nil {
		timeoutCfg := defaultTimeoutConfig()

		// 从配置中获取默认模型
		defaultModel := "claude-sonnet-4-20250929"
		if a.config != nil {
			if server, ok := a.config["server"].(map[string]interface{}); ok {
				if dm, ok := server["default_model"].(string); ok && dm != "" {
					defaultModel = dm
				}
			}
		}

		a.healthChecker = health.NewChecker(timeoutCfg.ToHealthCheckTimeoutConfig(), a.modelRewriter, defaultModel)
	}

	return nil
}

// getConfigPath 获取配置文件的绝对路径
func (a *App) getConfigPath() (string, error) {
	// 获取用户配置目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// 创建应用数据目录
	appDataDir := filepath.Join(homeDir, ".cccc-proxy")
	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create app data directory: %w", err)
	}

	// 返回配置文件的完整路径
	return filepath.Join(appDataDir, "config.json"), nil
}

// initDatabaseManager 初始化统一数据库管理器（3-DB架构）
func (a *App) initDatabaseManager() error {
	// 初始化全局数据库管理器（使用默认配置）
	if err := database.InitializeGlobalManager(nil); err != nil {
		return fmt.Errorf("failed to initialize global database manager: %w", err)
	}

	dbManager, err := database.GetGlobalManager()
	if err != nil {
		return fmt.Errorf("failed to get global database manager: %w", err)
	}

	a.dbManager = dbManager

	// 打印数据库路径信息
	info := dbManager.GetInfo()
	runtime.LogInfo(a.ctx, fmt.Sprintf("Database manager initialized (3-DB architecture): %+v", info))
	a.addLog("info", fmt.Sprintf("统一数据库管理器初始化完成（3-DB架构）"))
	a.addLog("info", fmt.Sprintf("主数据库: %s", info["main_db_path"]))
	a.addLog("info", fmt.Sprintf("日志数据库: %s", info["logs_db_path"]))
	a.addLog("info", fmt.Sprintf("统计数据库: %s", info["statistics_db_path"]))

	return nil
}

// initDatabase 初始化SQLite数据库 - 使用简化架构
func (a *App) initDatabase() error {
	if a.dbManager == nil {
		return fmt.Errorf("database manager not initialized")
	}

	// 使用简化数据库管理器获取数据库连接
	db, err := a.dbManager.GetMainDB()
	if err != nil {
		return fmt.Errorf("failed to get database from manager: %w", err)
	}

	// 确保请求日志表包含最新字段
	if err := a.ensureRequestLogsSchema(db); err != nil {
		return fmt.Errorf("failed to ensure request logs schema: %w", err)
	}

	// 打印数据库路径信息
	mainDBPath := a.dbManager.GetMainDBPath()
	runtime.LogInfo(a.ctx, fmt.Sprintf("Main database path: %s", mainDBPath))
	a.addLog("info", fmt.Sprintf("主数据库路径: %s", mainDBPath))

	// 表结构已在Manager.initialize中创建（分库架构：main.db存储端点配置）
	// 这里只需要设置数据库连接
	a.db = db

	return nil
}

// OnDomReady is called after the DOM has finished loading
func (a *App) OnDomReady(ctx context.Context) {
	runtime.LogInfo(a.ctx, "DOM ready event triggered")
	host, port := a.getEffectiveProxyAddress()
	runtime.LogInfo(a.ctx, fmt.Sprintf("✅ 统一路由架构已启动 - 启动HTTP代理服务器 (%s:%d)", host, port))

	// 启动HTTP代理服务器供Claude Code使用
	go a.startProxyServer()

	a.running = true
}

// OnBeforeClose is called when the application is about to quit
func (a *App) OnBeforeClose(ctx context.Context) (prevent bool) {
	a.cleanup()
	return false
}

// OnShutdown is called when the application is shutting down
func (a *App) OnShutdown(ctx context.Context) {
	a.cleanup()
}

// startProxyServer 启动HTTP代理服务器供Claude Code使用
func (a *App) startProxyServer() {
	runtime.LogInfo(a.ctx, "启动HTTP代理服务器...")

	// 检查数据库是否可用
	if a.db == nil {
		runtime.LogError(a.ctx, "数据库不可用，无法启动代理服务器")
		return
	}

	a.mutex.Lock()
	var serverConfig map[string]interface{}
	if a.config != nil {
		if cfg, ok := a.config["server"].(map[string]interface{}); ok {
			serverConfig = cfg
		}
	}
	a.applyServerAddressNoLock(serverConfig)
	a.syncActualAddressNoLock()
	host := a.proxyHost
	port := a.proxyPort
	a.mutex.Unlock()

	addr := fmt.Sprintf("%s:%d", host, port)

	// 创建一个简单的HTTP服务器
	mux := http.NewServeMux()

	// 添加CORS头
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 设置CORS头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 处理API请求
		if r.URL.Path == "/v1/messages" || r.URL.Path == "/chat/completions" || r.URL.Path == "/responses" {
			a.handleProxyRequest(w, r)
			return
		}

		// 健康检查端点
		if r.URL.Path == "/health" || r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status": "healthy", "service": "cccc-proxy", "host": "%s", "port": %d}`, host, port)
			return
		}

		// 其他路径返回404
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error": "Not found"}`)
	})

	// 启动HTTP服务器
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("HTTP代理服务器启动在 http://%s:%d", host, port))

	// 启动服务器
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		runtime.LogError(a.ctx, fmt.Sprintf("HTTP服务器启动失败: %v", err))
	}
}

// handleProxyRequest 处理代理请求
func (a *App) handleProxyRequest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	runtime.LogInfo(a.ctx, fmt.Sprintf("收到代理请求: %s %s", r.Method, r.URL.Path))

	// 获取可用的端点
	endpoints, err := a.getAvailableEndpoints()
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("获取端点失败: %v", err))
		writeJSONError(w, http.StatusInternalServerError, "endpoint_query_failed", "Failed to get endpoints")
		return
	}

	if len(endpoints) == 0 {
		runtime.LogError(a.ctx, "没有可用的端点")
		writeJSONError(w, http.StatusServiceUnavailable, "no_available_endpoints", "No available endpoints")
		return
	}

	formatDetection := utils.DetectRequestFormat(r.URL.Path, body)
	if a.requestLogger == nil {
		if err := a.initRequestLogger(); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("初始化日志记录器失败: %v", err))
		}
	}

	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	originalRequestHeaders := headersToMap(r.Header, true)
	originalRequestURL := r.URL.String()
	originalRequestBody := string(body)
	originalRequestBodyPreview, originalRequestBodyTruncated := truncateStringForLog(originalRequestBody, healthLogPreviewLimit)
	requestBodySize := len(body)

	clientToken := a.extractClientToken(r)
	unauthorized := true
	var lastError error
	var lastStatus int
	var lastBody []byte

	clientType := "unknown"
	requestFormat := "unknown"
	detectedBy := ""
	detectionConfidence := 0.0
	if formatDetection != nil {
		clientType = normalizeClientType(formatDetection.ClientType)
		requestFormat = normalizeRequestFormat(formatDetection.Format)
		detectedBy = formatDetection.DetectedBy
		detectionConfidence = formatDetection.Confidence
	}

	attemptNumber := 1

	for _, endpoint := range endpoints {
		attemptStart := time.Now()

		targetURL, err := a.buildTargetURL(&endpoint, r.URL.Path, r.URL.RawQuery)
		if err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("构建目标URL失败 (%s): %v", endpoint.Name, err))
			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               endpoint.Name,
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             http.StatusBadGateway,
				DurationMs:             time.Since(attemptStart).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        map[string]string{},
				ResponseBody:           "",
				ResponseBodyTruncated:  false,
				ResponseBodySize:       0,
				IsStreaming:            false,
				Error:                  err.Error(),
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        targetURL,
				FinalRequestHeaders:    cloneStringMap(originalRequestHeaders),
				FinalRequestBody:       originalRequestBodyPreview,
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				Tags:                   append([]string{}, endpoint.Tags...),
				EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
			})
			lastError = err
			attemptNumber++
			continue
		}

		bodyForEndpoint := append([]byte(nil), body...)
		bodyForEndpoint, originalModel, rewrittenModel, rewriteApplied, rewriteErr := a.applyModelRewrite(bodyForEndpoint, &endpoint, clientType, r.Header)
		if rewriteErr != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("模型重写失败 (%s): %v", endpoint.Name, rewriteErr))
		}
		finalRequestBodyPreview, _ := truncateStringForLog(string(bodyForEndpoint), healthLogPreviewLimit)

		mappedToken, ok := a.validateAndMapToken(clientToken, &endpoint)
		if !ok {
			runtime.LogDebug(a.ctx, fmt.Sprintf("Token验证未通过，跳过端点 %s (provided=%s)", endpoint.Name, maskToken(clientToken)))
			attemptNumber++
			continue
		}

		unauthorized = false
		if mappedToken != "" {
			runtime.LogInfo(a.ctx, fmt.Sprintf("Token validated for endpoint %s, using mapped credential %s", endpoint.Name, maskToken(mappedToken)))
		} else {
			runtime.LogInfo(a.ctx, fmt.Sprintf("Token validation passed for endpoint %s (no credential forwarding required)", endpoint.Name))
		}

		finalRequestHeaders := buildFinalRequestHeaders(r.Header, &endpoint, mappedToken)

		resp, err := a.forwardRequest(r, bodyForEndpoint, targetURL, endpoint, mappedToken)
		if err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("请求发送失败: %s -> %s (%s): %v", r.URL.Path, targetURL, endpoint.Name, err))
			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               endpoint.Name,
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             http.StatusBadGateway,
				DurationMs:             time.Since(attemptStart).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        map[string]string{},
				ResponseBody:           "",
				ResponseBodyTruncated:  false,
				ResponseBodySize:       0,
				IsStreaming:            false,
				Error:                  err.Error(),
				Model:                  chooseLoggedModel(originalModel, rewrittenModel),
				OriginalModel:          originalModel,
				RewrittenModel:         rewrittenModel,
				ModelRewriteApplied:    rewriteApplied,
				Tags:                   append([]string{}, endpoint.Tags...),
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        targetURL,
				FinalRequestHeaders:    cloneStringMap(finalRequestHeaders),
				FinalRequestBody:       finalRequestBodyPreview,
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				FormatConverted:        rewriteApplied,
				EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
			})
			lastError = err
			lastStatus = http.StatusBadGateway
			attemptNumber++
			continue
		}

		if resp == nil {
			lastError = fmt.Errorf("empty response returned from endpoint %s", endpoint.Name)
			lastStatus = http.StatusBadGateway
			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               endpoint.Name,
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             http.StatusBadGateway,
				DurationMs:             time.Since(attemptStart).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        map[string]string{},
				ResponseBody:           "",
				ResponseBodyTruncated:  false,
				ResponseBodySize:       0,
				IsStreaming:            false,
				Error:                  "empty response",
				Model:                  chooseLoggedModel(originalModel, rewrittenModel),
				OriginalModel:          originalModel,
				RewrittenModel:         rewrittenModel,
				ModelRewriteApplied:    rewriteApplied,
				Tags:                   append([]string{}, endpoint.Tags...),
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        targetURL,
				FinalRequestHeaders:    cloneStringMap(finalRequestHeaders),
				FinalRequestBody:       finalRequestBodyPreview,
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				FormatConverted:        rewriteApplied,
				EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
			})
			attemptNumber++
			continue
		}

		responseHeadersMap := headersToMap(resp.Header, false)

        if resp.StatusCode >= http.StatusInternalServerError {
			bodyCopy, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			runtime.LogWarning(a.ctx, fmt.Sprintf("端点 %s 返回 %d，尝试下一端点", endpoint.Name, resp.StatusCode))
			lastStatus = resp.StatusCode
			lastBody = bodyCopy

			responseBodyPreview, responseBodyTruncated := truncateStringForLog(string(bodyCopy), healthLogPreviewLimit)
			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               endpoint.Name,
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             resp.StatusCode,
				DurationMs:             time.Since(attemptStart).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        cloneStringMap(responseHeadersMap),
				ResponseBody:           responseBodyPreview,
				ResponseBodyTruncated:  responseBodyTruncated,
				ResponseBodySize:       len(bodyCopy),
				IsStreaming:            strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream"),
				Error:                  fmt.Sprintf("upstream returned %d", resp.StatusCode),
				Model:                  chooseLoggedModel(originalModel, rewrittenModel),
				OriginalModel:          originalModel,
				RewrittenModel:         rewrittenModel,
				ModelRewriteApplied:    rewriteApplied,
				Tags:                   append([]string{}, endpoint.Tags...),
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        targetURL,
				FinalRequestHeaders:    cloneStringMap(finalRequestHeaders),
				FinalRequestBody:       finalRequestBodyPreview,
				FinalResponseHeaders:   cloneStringMap(responseHeadersMap),
				FinalResponseBody:      responseBodyPreview,
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				FormatConverted:        rewriteApplied,
				EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
			})

			attemptNumber++
			continue
		}

        // 扩大回退策略到 4xx：对客户端错误也尝试下一端点（提高对不同上游兼容性，含 OpenAI 常见 400/401/403/404/422/429 等）
        if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError {
            bodyCopy, _ := io.ReadAll(resp.Body)
            resp.Body.Close()
            runtime.LogWarning(a.ctx, fmt.Sprintf("端点 %s 返回客户端错误 %d，尝试下一端点", endpoint.Name, resp.StatusCode))
            lastStatus = resp.StatusCode
            lastBody = bodyCopy

            responseHeadersMap := headersToMap(resp.Header, false)
            responseBodyPreview, responseBodyTruncated := truncateStringForLog(string(bodyCopy), healthLogPreviewLimit)
            a.logProxyRequest(&logger.RequestLog{
                Timestamp:              time.Now(),
                RequestID:              requestID,
                Endpoint:               endpoint.Name,
                Method:                 r.Method,
                Path:                   r.URL.Path,
                StatusCode:             resp.StatusCode,
                DurationMs:             time.Since(attemptStart).Milliseconds(),
                AttemptNumber:          attemptNumber,
                RequestHeaders:         cloneStringMap(originalRequestHeaders),
                RequestBody:            originalRequestBodyPreview,
                RequestBodyTruncated:   originalRequestBodyTruncated,
                RequestBodySize:        requestBodySize,
                ResponseHeaders:        cloneStringMap(responseHeadersMap),
                ResponseBody:           responseBodyPreview,
                ResponseBodyTruncated:  responseBodyTruncated,
                ResponseBodySize:       len(bodyCopy),
                IsStreaming:            strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream"),
                Error:                  fmt.Sprintf("upstream returned %d", resp.StatusCode),
                Model:                  chooseLoggedModel(originalModel, rewrittenModel),
                OriginalModel:          originalModel,
                RewrittenModel:         rewrittenModel,
                ModelRewriteApplied:    rewriteApplied,
                Tags:                   append([]string{}, endpoint.Tags...),
                OriginalRequestURL:     originalRequestURL,
                OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
                OriginalRequestBody:    originalRequestBodyPreview,
                FinalRequestURL:        targetURL,
                FinalRequestHeaders:    cloneStringMap(finalRequestHeaders),
                FinalRequestBody:       finalRequestBodyPreview,
                FinalResponseHeaders:   cloneStringMap(responseHeadersMap),
                FinalResponseBody:      responseBodyPreview,
                ClientType:             clientType,
                RequestFormat:          requestFormat,
                DetectionConfidence:    detectionConfidence,
                DetectedBy:             detectedBy,
                FormatConverted:        rewriteApplied,
                EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
            })

            attemptNumber++
            continue
        }

		isStreaming := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")

		if isStreaming {
			// 读取流式响应体（用于模型重写）
			streamBody, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				runtime.LogError(a.ctx, fmt.Sprintf("读取流式响应失败: %s -> %s (%s): %v", r.URL.Path, targetURL, endpoint.Name, readErr))
				lastError = readErr
				lastStatus = http.StatusBadGateway
				attemptNumber++
				continue
			}

			// 🔥 GZIP DECOMPRESSION: 检查并解压 gzip
			if len(streamBody) > 2 && streamBody[0] == 0x1f && streamBody[1] == 0x8b {
				runtime.LogInfo(a.ctx, "Detected gzip compressed response, decompressing...")
				gzReader, gzErr := gzip.NewReader(bytes.NewReader(streamBody))
				if gzErr == nil {
					decompressed, gzErr := io.ReadAll(gzReader)
					gzReader.Close()
					if gzErr == nil {
						streamBody = decompressed
						runtime.LogInfo(a.ctx, "✅ Gzip decompression successful")
					}
				}
			}

			// 🔥 FORMAT CONVERSION (SSE): OpenAI SSE → Anthropic SSE
			needsFormatConversion := endpoint.URLAnthropic == "" && endpoint.URLOpenAI != "" && requestFormat == "anthropic"
			runtime.LogInfo(a.ctx, fmt.Sprintf("🔍 SSE Conv check: URLAnthropic=%q URLOpenAI=%q requestFormat=%q needs=%v", 
				endpoint.URLAnthropic, endpoint.URLOpenAI, requestFormat, needsFormatConversion))
			
			if needsFormatConversion {
				runtime.LogInfo(a.ctx, fmt.Sprintf("🔄 Converting OpenAI SSE to Anthropic SSE for endpoint %s (body length: %d)", endpoint.Name, len(streamBody)))
				
				// 使用 conversion 包的流式转换函数
				reader := bytes.NewReader(streamBody)
				var buf bytes.Buffer
				convErr := conversion.StreamOpenAISSEToAnthropic(reader, &buf)
				if convErr == nil {
					streamBody = buf.Bytes()
					runtime.LogInfo(a.ctx, fmt.Sprintf("✅ SSE format conversion successful, new length: %d", len(streamBody)))
				} else {
					runtime.LogError(a.ctx, fmt.Sprintf("❌ SSE format conversion failed: %v", convErr))
				}
			}

			// 应用模型重写（SSE 格式）
			if rewriteApplied && a.modelRewriter != nil && originalModel != "" && rewrittenModel != "" {
				if rewrittenBody, err := a.modelRewriter.RewriteResponse(streamBody, originalModel, rewrittenModel); err == nil {
					streamBody = rewrittenBody
					runtime.LogDebug(a.ctx, fmt.Sprintf("流式响应模型重写成功: %s -> %s", rewrittenModel, originalModel))
				} else {
					runtime.LogWarning(a.ctx, fmt.Sprintf("流式响应模型重写失败: %v", err))
				}
			}

			// SSE格式中空text是正常的（在content_block_start中），不需要修复

			// 发送响应
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(streamBody)))
			w.WriteHeader(resp.StatusCode)
			w.Write(streamBody)

			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               endpoint.Name,
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             resp.StatusCode,
				DurationMs:             time.Since(attemptStart).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        cloneStringMap(responseHeadersMap),
				ResponseBody:           "",
				ResponseBodyTruncated:  false,
				ResponseBodySize:       0,
				IsStreaming:            true,
				Model:                  chooseLoggedModel(originalModel, rewrittenModel),
				OriginalModel:          originalModel,
				RewrittenModel:         rewrittenModel,
				ModelRewriteApplied:    rewriteApplied,
				Tags:                   append([]string{}, endpoint.Tags...),
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        targetURL,
				FinalRequestHeaders:    cloneStringMap(finalRequestHeaders),
				FinalRequestBody:       finalRequestBodyPreview,
				FinalResponseHeaders:   cloneStringMap(responseHeadersMap),
				FinalResponseBody:      "",
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				FormatConverted:        rewriteApplied,
				EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
			})

			duration := time.Since(startTime).Milliseconds()
			runtime.LogInfo(a.ctx, fmt.Sprintf("请求成功: %s -> %s (%dms)", r.URL.Path, targetURL, duration))
			return
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastError = readErr
			lastStatus = http.StatusBadGateway
			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               endpoint.Name,
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             http.StatusBadGateway,
				DurationMs:             time.Since(attemptStart).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        cloneStringMap(responseHeadersMap),
				ResponseBody:           "",
				ResponseBodyTruncated:  false,
				ResponseBodySize:       0,
				IsStreaming:            false,
				Error:                  readErr.Error(),
				Model:                  chooseLoggedModel(originalModel, rewrittenModel),
				OriginalModel:          originalModel,
				RewrittenModel:         rewrittenModel,
				ModelRewriteApplied:    rewriteApplied,
				Tags:                   append([]string{}, endpoint.Tags...),
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        targetURL,
				FinalRequestHeaders:    cloneStringMap(finalRequestHeaders),
				FinalRequestBody:       finalRequestBodyPreview,
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				FormatConverted:        rewriteApplied,
				EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
			})
			attemptNumber++
			continue
		}

		// 🔥 GZIP DECOMPRESSION: 检查并解压 gzip
		if len(respBody) > 2 && respBody[0] == 0x1f && respBody[1] == 0x8b {
			runtime.LogInfo(a.ctx, "Detected gzip compressed response, decompressing...")
			gzReader, gzErr := gzip.NewReader(bytes.NewReader(respBody))
			if gzErr == nil {
				decompressed, gzErr := io.ReadAll(gzReader)
				gzReader.Close()
				if gzErr == nil {
					respBody = decompressed
					runtime.LogInfo(a.ctx, "✅ Gzip decompression successful")
				}
			}
		}

		if rewriteApplied && a.modelRewriter != nil && originalModel != "" && rewrittenModel != "" {
			if rewrittenBody, err := a.modelRewriter.RewriteResponse(respBody, originalModel, rewrittenModel); err == nil {
				respBody = rewrittenBody
			}
		}

		// 🔥 FORMAT CONVERSION: OpenAI → Anthropic
		runtime.LogInfo(a.ctx, fmt.Sprintf("🔍 Non-streaming format check: endpoint=%s, requestFormat=%q, URLAnth=%q, URLOpenAI=%q", 
			endpoint.Name, requestFormat, endpoint.URLAnthropic, endpoint.URLOpenAI))
		
		if requestFormat == "anthropic" && endpoint.URLAnthropic == "" && endpoint.URLOpenAI != "" {
			// 检测响应格式
			var testResp map[string]interface{}
			if json.Unmarshal(respBody, &testResp) == nil {
				runtime.LogInfo(a.ctx, fmt.Sprintf("🔍 Response has keys: %v", getKeys(testResp)))
				// 如果是 OpenAI 格式（有 choices 字段），转换为 Anthropic
				if _, hasChoices := testResp["choices"]; hasChoices {
					runtime.LogInfo(a.ctx, fmt.Sprintf("🔄 Converting OpenAI response to Anthropic format for endpoint %s", endpoint.Name))
					convertedBody, convErr := conversion.ConvertChatResponseJSONToAnthropic(respBody)
					if convErr == nil {
						respBody = convertedBody
						runtime.LogInfo(a.ctx, "✅ Response format conversion successful")
					} else {
						runtime.LogError(a.ctx, fmt.Sprintf("❌ Response format conversion failed: %v", convErr))
					}
				} else {
					runtime.LogInfo(a.ctx, "ℹ️ Response already in Anthropic format (no choices field)")
				}
			}
		} else {
			runtime.LogInfo(a.ctx, "ℹ️ Format conversion skipped (conditions not met)")
		}

		// 🔥 RESPONSE VALIDATION: 修复不完整的 Anthropic 响应
		if requestFormat == "anthropic" {
			var anthResp map[string]interface{}
			if err := json.Unmarshal(respBody, &anthResp); err == nil {
				// 检查是否是 Anthropic 格式
				if anthResp["type"] == "message" {
					if content, ok := anthResp["content"].([]interface{}); ok {
						fixed := false
						for i, block := range content {
							if blockMap, ok := block.(map[string]interface{}); ok {
								if blockMap["type"] == "text" {
									// 检查 text 字段是否存在且非空
									textVal, hasText := blockMap["text"]
									if !hasText || textVal == "" {
										// 修复：添加占位符文本，避免 Claude Code 报错
										blockMap["text"] = "[Empty response from upstream]"
										content[i] = blockMap
										fixed = true
										runtime.LogInfo(a.ctx, fmt.Sprintf("🔧 Fixed empty text field in content block %d", i))
									}
								}
							}
						}
						if fixed {
							anthResp["content"] = content
							if fixedBody, err := json.Marshal(anthResp); err == nil {
								respBody = fixedBody
								runtime.LogInfo(a.ctx, "✅ Response validation: fixed empty Anthropic response")
							}
						}
					}
				}
			}
		}

		for key, values := range resp.Header {
			if strings.EqualFold(key, "Content-Length") {
				continue
			}
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)

		responseBodyPreview, responseBodyTruncated := truncateStringForLog(string(respBody), healthLogPreviewLimit)
		a.logProxyRequest(&logger.RequestLog{
			Timestamp:              time.Now(),
			RequestID:              requestID,
			Endpoint:               endpoint.Name,
			Method:                 r.Method,
			Path:                   r.URL.Path,
			StatusCode:             resp.StatusCode,
			DurationMs:             time.Since(attemptStart).Milliseconds(),
			AttemptNumber:          attemptNumber,
			RequestHeaders:         cloneStringMap(originalRequestHeaders),
			RequestBody:            originalRequestBodyPreview,
			RequestBodyTruncated:   originalRequestBodyTruncated,
			RequestBodySize:        requestBodySize,
			ResponseHeaders:        cloneStringMap(responseHeadersMap),
			ResponseBody:           responseBodyPreview,
			ResponseBodyTruncated:  responseBodyTruncated,
			ResponseBodySize:       len(respBody),
			IsStreaming:            false,
			Model:                  chooseLoggedModel(originalModel, rewrittenModel),
			OriginalModel:          originalModel,
			RewrittenModel:         rewrittenModel,
			ModelRewriteApplied:    rewriteApplied,
			Tags:                   append([]string{}, endpoint.Tags...),
			OriginalRequestURL:     originalRequestURL,
			OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
			OriginalRequestBody:    originalRequestBodyPreview,
			FinalRequestURL:        targetURL,
			FinalRequestHeaders:    cloneStringMap(finalRequestHeaders),
			FinalRequestBody:       finalRequestBodyPreview,
			FinalResponseHeaders:   cloneStringMap(responseHeadersMap),
			FinalResponseBody:      responseBodyPreview,
			ClientType:             clientType,
			RequestFormat:          requestFormat,
			DetectionConfidence:    detectionConfidence,
			DetectedBy:             detectedBy,
			FormatConverted:        rewriteApplied,
			EndpointResponseTime:   time.Since(attemptStart).Milliseconds(),
		})

		duration := time.Since(startTime).Milliseconds()
		runtime.LogInfo(a.ctx, fmt.Sprintf("请求成功: %s -> %s (%dms)", r.URL.Path, targetURL, duration))
		return
	}

	if unauthorized {
		runtime.LogInfo(a.ctx, fmt.Sprintf("Token validation failed for all endpoints (provided=%s)", maskToken(clientToken)))
		a.logProxyRequest(&logger.RequestLog{
			Timestamp:              time.Now(),
			RequestID:              requestID,
			Endpoint:               "authorization",
			Method:                 r.Method,
			Path:                   r.URL.Path,
			StatusCode:             http.StatusUnauthorized,
			DurationMs:             time.Since(startTime).Milliseconds(),
			AttemptNumber:          attemptNumber,
			RequestHeaders:         cloneStringMap(originalRequestHeaders),
			RequestBody:            originalRequestBodyPreview,
			RequestBodyTruncated:   originalRequestBodyTruncated,
			RequestBodySize:        requestBodySize,
			ResponseHeaders:        map[string]string{},
			ResponseBody:           "",
			ResponseBodyTruncated:  false,
			ResponseBodySize:       0,
			IsStreaming:            false,
			Error:                  "Invalid or unauthorized token",
			OriginalRequestURL:     originalRequestURL,
			OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
			OriginalRequestBody:    originalRequestBodyPreview,
			FinalRequestURL:        "",
			FinalRequestHeaders:    map[string]string{},
			FinalRequestBody:       originalRequestBodyPreview,
			ClientType:             clientType,
			RequestFormat:          requestFormat,
			DetectionConfidence:    detectionConfidence,
			DetectedBy:             detectedBy,
			FormatConverted:        false,
			EndpointResponseTime:   time.Since(startTime).Milliseconds(),
		})
		writeJSONError(w, http.StatusUnauthorized, "unauthorized_token", "Invalid or unauthorized token")
		return
	}

	if lastStatus != 0 {
		runtime.LogError(a.ctx, fmt.Sprintf("所有端点返回服务器错误，最后状态码: %d", lastStatus))
		if len(lastBody) > 0 {
			w.WriteHeader(lastStatus)
			w.Write(lastBody)
			responseBodyPreview, responseBodyTruncated := truncateStringForLog(string(lastBody), healthLogPreviewLimit)
			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               "fallback",
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             lastStatus,
				DurationMs:             time.Since(startTime).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        map[string]string{},
				ResponseBody:           responseBodyPreview,
				ResponseBodyTruncated:  responseBodyTruncated,
				ResponseBodySize:       len(lastBody),
				IsStreaming:            false,
				Error:                  fmt.Sprintf("All endpoints returned %d", lastStatus),
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        "",
				FinalRequestHeaders:    map[string]string{},
				FinalRequestBody:       originalRequestBodyPreview,
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				FormatConverted:        false,
				EndpointResponseTime:   time.Since(startTime).Milliseconds(),
			})
		} else {
			writeJSONError(w, lastStatus, "upstream_error", "All upstream endpoints returned errors")
			a.logProxyRequest(&logger.RequestLog{
				Timestamp:              time.Now(),
				RequestID:              requestID,
				Endpoint:               "fallback",
				Method:                 r.Method,
				Path:                   r.URL.Path,
				StatusCode:             lastStatus,
				DurationMs:             time.Since(startTime).Milliseconds(),
				AttemptNumber:          attemptNumber,
				RequestHeaders:         cloneStringMap(originalRequestHeaders),
				RequestBody:            originalRequestBodyPreview,
				RequestBodyTruncated:   originalRequestBodyTruncated,
				RequestBodySize:        requestBodySize,
				ResponseHeaders:        map[string]string{},
				ResponseBody:           "",
				ResponseBodyTruncated:  false,
				ResponseBodySize:       0,
				IsStreaming:            false,
				Error:                  "All upstream endpoints returned errors",
				OriginalRequestURL:     originalRequestURL,
				OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
				OriginalRequestBody:    originalRequestBodyPreview,
				FinalRequestURL:        "",
				FinalRequestHeaders:    map[string]string{},
				FinalRequestBody:       originalRequestBodyPreview,
				ClientType:             clientType,
				RequestFormat:          requestFormat,
				DetectionConfidence:    detectionConfidence,
				DetectedBy:             detectedBy,
				FormatConverted:        false,
				EndpointResponseTime:   time.Since(startTime).Milliseconds(),
			})
		}
		return
	}

	if lastError != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("所有端点请求失败: %v", lastError))
		a.logProxyRequest(&logger.RequestLog{
			Timestamp:              time.Now(),
			RequestID:              requestID,
			Endpoint:               "fallback",
			Method:                 r.Method,
			Path:                   r.URL.Path,
			StatusCode:             http.StatusBadGateway,
			DurationMs:             time.Since(startTime).Milliseconds(),
			AttemptNumber:          attemptNumber,
			RequestHeaders:         cloneStringMap(originalRequestHeaders),
			RequestBody:            originalRequestBodyPreview,
			RequestBodyTruncated:   originalRequestBodyTruncated,
			RequestBodySize:        requestBodySize,
			ResponseHeaders:        map[string]string{},
			ResponseBody:           "",
			ResponseBodyTruncated:  false,
			ResponseBodySize:       0,
			IsStreaming:            false,
			Error:                  lastError.Error(),
			OriginalRequestURL:     originalRequestURL,
			OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
			OriginalRequestBody:    originalRequestBodyPreview,
			FinalRequestURL:        "",
			FinalRequestHeaders:    map[string]string{},
			FinalRequestBody:       originalRequestBodyPreview,
			ClientType:             clientType,
			RequestFormat:          requestFormat,
			DetectionConfidence:    detectionConfidence,
			DetectedBy:             detectedBy,
			FormatConverted:        false,
			EndpointResponseTime:   time.Since(startTime).Milliseconds(),
		})
		writeJSONError(w, http.StatusBadGateway, "proxy_forward_failed", lastError.Error())
		return
	}

	runtime.LogError(a.ctx, "没有可用端点处理请求")
	a.logProxyRequest(&logger.RequestLog{
		Timestamp:              time.Now(),
		RequestID:              requestID,
		Endpoint:               "fallback",
		Method:                 r.Method,
		Path:                   r.URL.Path,
		StatusCode:             http.StatusServiceUnavailable,
		DurationMs:             time.Since(startTime).Milliseconds(),
		AttemptNumber:          attemptNumber,
		RequestHeaders:         cloneStringMap(originalRequestHeaders),
		RequestBody:            originalRequestBodyPreview,
		RequestBodyTruncated:   originalRequestBodyTruncated,
		RequestBodySize:        requestBodySize,
		ResponseHeaders:        map[string]string{},
		ResponseBody:           "",
		ResponseBodyTruncated:  false,
		ResponseBodySize:       0,
		IsStreaming:            false,
		Error:                  "No available endpoints",
		OriginalRequestURL:     originalRequestURL,
		OriginalRequestHeaders: cloneStringMap(originalRequestHeaders),
		OriginalRequestBody:    originalRequestBodyPreview,
		FinalRequestURL:        "",
		FinalRequestHeaders:    map[string]string{},
		FinalRequestBody:       originalRequestBodyPreview,
		ClientType:             clientType,
		RequestFormat:          requestFormat,
		DetectionConfidence:    detectionConfidence,
		DetectedBy:             detectedBy,
		FormatConverted:        false,
		EndpointResponseTime:   time.Since(startTime).Milliseconds(),
	})
	writeJSONError(w, http.StatusServiceUnavailable, "no_available_endpoints", "No available endpoints")
}

// buildTargetURL 根据请求路径选择端点基础URL并拼接完整目标URL
func (a *App) buildTargetURL(endpoint *config.EndpointConfig, requestPath string, rawQuery string) (string, error) {
	if endpoint == nil {
		return "", fmt.Errorf("endpoint is nil")
	}

	reqPath := strings.TrimSpace(requestPath)
	if reqPath == "" || reqPath == "/" {
		return "", fmt.Errorf("invalid request path: %s", requestPath)
	}
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}

	var base string
	switch {
	case strings.HasPrefix(reqPath, "/v1/messages"):
		if endpoint.URLAnthropic != "" {
			base = endpoint.URLAnthropic
		} else {
			// 🔥 PATH CONVERSION: 端点只有 OpenAI URL，需要转换路径
			base = endpoint.URLOpenAI
			reqPath = strings.Replace(reqPath, "/v1/messages", "/v1/chat/completions", 1)
			runtime.LogInfo(a.ctx, fmt.Sprintf("🚨 PATH CONVERTED: /v1/messages -> /v1/chat/completions for OpenAI-only endpoint %s", endpoint.Name))
		}
	case strings.Contains(reqPath, "/chat/completions") || strings.Contains(reqPath, "/responses"):
		if endpoint.URLOpenAI != "" {
			base = endpoint.URLOpenAI
		} else {
			base = endpoint.URLAnthropic
		}
	default:
		if endpoint.URLAnthropic != "" {
			base = endpoint.URLAnthropic
		} else {
			base = endpoint.URLOpenAI
		}
	}

	if base == "" {
		return "", fmt.Errorf("no base URL configured for request path %s", reqPath)
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint URL %s: %w", base, err)
	}

	cleanPath := reqPath
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	basePath := baseURL.Path
	if basePath == "" || basePath == "/" {
		baseURL.Path = cleanPath
	} else {
		combined := pathpkg.Join(strings.TrimSuffix(basePath, "/"), strings.TrimPrefix(cleanPath, "/"))
		if !strings.HasPrefix(combined, "/") {
			combined = "/" + combined
		}
		baseURL.Path = combined
	}

	if rawQuery != "" {
		if baseURL.RawQuery != "" {
			baseURL.RawQuery = baseURL.RawQuery + "&" + rawQuery
		} else {
			baseURL.RawQuery = rawQuery
		}
	}

	return baseURL.String(), nil
}

// getAvailableEndpoints 获取可用的端点
func (a *App) getAvailableEndpoints() ([]config.EndpointConfig, error) {
	query := `
		SELECT name,
		       url_anthropic,
			   url_openai,
			   endpoint_type,
			   auth_type,
			   auth_value,
			   enabled,
			   priority,
			   tags,
			   model_rewrite_enabled,
			   target_model,
			   model_rewrite_rules
		FROM endpoints
		WHERE enabled = 1
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := a.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []config.EndpointConfig
	for rows.Next() {
		var (
			name, urlAnthropic, urlOpenai, endpointType, authType, authValue sql.NullString
			enabled                                                          sql.NullBool
			priority                                                         sql.NullInt64
			tagsJSON                                                         sql.NullString
			modelRewriteEnabled                                              sql.NullBool
			targetModel                                                      sql.NullString
			modelRewriteRules                                                sql.NullString
		)

		if err := rows.Scan(
			&name,
			&urlAnthropic,
			&urlOpenai,
			&endpointType,
			&authType,
			&authValue,
			&enabled,
			&priority,
			&tagsJSON,
			&modelRewriteEnabled,
			&targetModel,
			&modelRewriteRules,
		); err != nil {
			continue
		}

		if !enabled.Valid || !enabled.Bool {
			continue
		}

		endpoint := config.EndpointConfig{
			Name:         name.String,
			URLAnthropic: urlAnthropic.String,
			URLOpenAI:    urlOpenai.String,
			AuthType:     authType.String,
			AuthValue:    authValue.String,
			Enabled:      enabled.Bool,
			Priority:     int(priority.Int64),
		}

		if tagsJSON.Valid && strings.TrimSpace(tagsJSON.String) != "" {
			var parsedTags []string
			if err := json.Unmarshal([]byte(tagsJSON.String), &parsedTags); err == nil {
				endpoint.Tags = parsedTags
			}
		}

		if modelRewriteCfg, err := buildModelRewriteConfigFromRow(modelRewriteEnabled, targetModel, modelRewriteRules); err == nil && modelRewriteCfg != nil {
			endpoint.ModelRewrite = modelRewriteCfg
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

// TokenMapping 定义Token映射结构
type TokenMapping struct {
	InputToken  string `json:"input_token"`  // 用户输入的任意token
	OutputToken string `json:"output_token"` // 实际转发给上游端点的token
	EndpointID  string `json:"endpoint_id"`  // 目标端点ID（可选，为空则适用于所有端点）
	Description string `json:"description"`  // 映射描述
}

// getTokenMappings 获取Token映射配置
func (a *App) getTokenMappings() []TokenMapping {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	var mappings []TokenMapping

	// 从配置中获取Token映射
	if a.config != nil {
		if server, ok := a.config["server"].(map[string]interface{}); ok {
			if mappingsData, ok := server["token_mappings"].([]interface{}); ok {
				for _, mappingData := range mappingsData {
					if mapping, ok := mappingData.(map[string]interface{}); ok {
						tokenMapping := TokenMapping{
							InputToken:  getStringValue(mapping["input_token"]),
							OutputToken: getStringValue(mapping["output_token"]),
							EndpointID:  getStringValue(mapping["endpoint_id"]),
							Description: getStringValue(mapping["description"]),
						}
						if tokenMapping.InputToken != "" && tokenMapping.OutputToken != "" {
							mappings = append(mappings, tokenMapping)
						}
					}
				}
			}
		}
	}

	return mappings
}

// getClaudeCodeAuthToken 获取Claude Code认证token
func (a *App) getClaudeCodeAuthToken() string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 从配置中获取Claude Code认证token
	if a.config != nil {
		if server, ok := a.config["server"].(map[string]interface{}); ok {
			if token, ok := server["claude_code_auth_token"].(string); ok && token != "" {
				return token
			}
		}
	}

	// 如果配置中没有，尝试从环境变量获取
	if envToken := os.Getenv("CLAUDE_CODE_AUTH_TOKEN"); envToken != "" {
		return envToken
	}

	// 如果都没有，返回空字符串（将使用默认值"hello"）
	return ""
}

// validateAndMapToken 验证并映射用户Token到目标端点Token
func (a *App) validateAndMapToken(inputToken string, endpoint *config.EndpointConfig) (string, bool) {
	if endpoint == nil {
		return "", false
	}

	authType := strings.ToLower(strings.TrimSpace(endpoint.AuthType))
	expected := strings.TrimSpace(endpoint.AuthValue)

	// 无需验证的场景：无认证、OAuth等由服务端处理的方式
	if authType == "" || authType == "none" || authType == "oauth" {
		return "", true
	}

	// 任意Token模式直接放行（用于开发或调试）
	if a.isArbitraryTokenModeEnabled() {
		if expected != "" {
			return expected, true
		}
		return "", true
	}

	token := strings.TrimSpace(inputToken)

	allowed := make(map[string]string)

	if expected != "" {
		allowed[expected] = expected
	}

	globalToken := strings.TrimSpace(a.getClaudeCodeAuthToken())
	if globalToken != "" && expected != "" {
		allowed[globalToken] = expected
	} else if globalToken == "" && expected != "" {
		// 默认兼容hello占位令牌（用于未配置专用token的场景）
		allowed["hello"] = expected
	}

	for _, mapping := range a.getTokenMappings() {
		input := strings.TrimSpace(mapping.InputToken)
		output := strings.TrimSpace(mapping.OutputToken)
		if input == "" || output == "" {
			continue
		}
		if mapping.EndpointID != "" && !strings.EqualFold(mapping.EndpointID, endpoint.Name) {
			continue
		}
		allowed[input] = output
	}

	if token == "" {
		token = globalToken
		token = strings.TrimSpace(token)
	}

	if token == "" && expected != "" {
		token = "hello"
	}

	if token == "" {
		return "", false
	}

	if mapped, ok := allowed[token]; ok && mapped != "" {
		return mapped, true
	}

	return "", false
}

// isArbitraryTokenModeEnabled 检查是否启用任意Token模式
func (a *App) isArbitraryTokenModeEnabled() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if a.config != nil {
		if server, ok := a.config["server"].(map[string]interface{}); ok {
			if enabled, ok := server["arbitrary_token_mode"].(bool); ok {
				return enabled
			}
		}
	}

	// 默认从环境变量读取
	return os.Getenv("ARBITRARY_TOKEN_MODE") == "true"
}

// setClaudeCodeAuthToken 设置Claude Code认证token
func (a *App) setClaudeCodeAuthToken(token string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.config == nil {
		a.config = make(map[string]interface{})
	}

	server, ok := a.config["server"].(map[string]interface{})
	if !ok {
		server = make(map[string]interface{})
		a.config["server"] = server
	}

	server["claude_code_auth_token"] = token

	// 保存配置到文件
	return a.saveConfig()
}

// saveConfig 保存配置到文件
func (a *App) saveConfig() error {
	configPath := filepath.Join(os.Getenv("HOME"), ".cccc-proxy", "config.json")

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// 将配置写入文件
	configData, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, configData, 0644)
}

// applyModelRewrite 根据端点配置执行模型重写
func (a *App) applyModelRewrite(body []byte, endpoint *config.EndpointConfig, clientType string, headers http.Header) ([]byte, string, string, bool, error) {
	if a.modelRewriter == nil || endpoint == nil {
		return body, "", "", false, nil
	}

	reqClone, err := http.NewRequest(http.MethodPost, "http://localhost/model-rewrite", bytes.NewReader(body))
	if err != nil {
		return body, "", "", false, err
	}
	if headers != nil {
		reqClone.Header = headers.Clone()
	}

	originalModel, rewrittenModel, err := a.modelRewriter.RewriteRequestWithTags(reqClone, endpoint.ModelRewrite, endpoint.Tags, clientType)
	if err != nil {
		return body, "", "", false, err
	}
	if originalModel == "" || rewrittenModel == "" {
		return body, "", "", false, nil
	}

	rewrittenBody, err := io.ReadAll(reqClone.Body)
	reqClone.Body.Close()
	if err != nil {
		return body, "", "", false, err
	}

	return rewrittenBody, originalModel, rewrittenModel, true, nil
}

// chooseLoggedModel 返回应记录的模型名称
func chooseLoggedModel(originalModel, rewrittenModel string) string {
	if strings.TrimSpace(rewrittenModel) != "" {
		return rewrittenModel
	}
	return originalModel
}

// logProxyRequest 统一写入请求日志
func (a *App) logProxyRequest(entry *logger.RequestLog) {
	if entry == nil {
		return
	}

	if a.requestLogger == nil {
		if err := a.initRequestLogger(); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("无法初始化请求日志记录器: %v", err))
			return
		}
	}

	if a.requestLogger != nil {
		a.requestLogger.LogRequest(entry)
	}
}

// headersToMap 将HTTP头转换为map
func headersToMap(h http.Header, maskSensitive bool) map[string]string {
	if len(h) == 0 {
		return map[string]string{}
	}

	result := make(map[string]string, len(h))
	for key, values := range h {
		joined := strings.Join(values, ",")
		if maskSensitive {
			joined = maskHeaderValue(key, joined)
		}
		result[key] = joined
	}
	return result
}

// buildFinalRequestHeaders 构建发往上游的请求头（敏感字段已脱敏）
func buildFinalRequestHeaders(original http.Header, endpoint *config.EndpointConfig, mappedToken string) map[string]string {
	headers := headersToMap(original, true)

	// 移除原有认证信息
	for key := range headers {
		lower := strings.ToLower(key)
		if lower == "authorization" || lower == "x-api-key" {
			delete(headers, key)
		}
	}

	token := strings.TrimSpace(mappedToken)
	if token == "" && endpoint != nil {
		token = strings.TrimSpace(endpoint.AuthValue)
	}

	if endpoint != nil {
		switch strings.ToLower(strings.TrimSpace(endpoint.AuthType)) {
		case "api_key":
			if token != "" {
				headers["X-API-Key"] = maskToken(token)
			}
		case "auth_token", "auto":
			if token != "" {
				headers["Authorization"] = maskHeaderValue("Authorization", "Bearer "+token)
			}
		}
	}

	return headers
}

// maskHeaderValue 对敏感头部进行脱敏
func maskHeaderValue(key, value string) string {
	switch strings.ToLower(key) {
	case "authorization":
		parts := strings.Fields(value)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[0] + " " + maskToken(parts[1])
		}
		return maskToken(value)
	case "x-api-key":
		return maskToken(value)
	default:
		return value
	}
}

// normalizeClientType 将内部客户端类型转换为统一字符串
func normalizeClientType(ct utils.ClientType) string {
	switch ct {
	case utils.ClientClaudeCode:
		return "claude_code"
	case utils.ClientCodex:
		return "codex"
	case utils.ClientGemini:
		return "gemini"
	default:
		return "unknown"
	}
}

// normalizeRequestFormat 统一请求格式标识
func normalizeRequestFormat(f utils.RequestFormat) string {
	switch f {
	case utils.FormatAnthropic:
		return "anthropic"
	case utils.FormatOpenAI:
		return "openai"
	default:
		return "unknown"
	}
}

// forwardRequest 转发请求到目标端点
func (a *App) forwardRequest(originalReq *http.Request, body []byte, targetURL string, endpoint config.EndpointConfig, upstreamToken string) (*http.Response, error) {
	// 解析目标URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("解析目标URL失败: %v", err))
		return nil, err
	}

	// 创建新请求
	req, err := http.NewRequest(originalReq.Method, parsedURL.String(), bytes.NewReader(body))
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("创建新请求失败: %v", err))
		return nil, err
	}

	// 复制所有请求头，跳过认证相关字段，后续将使用经过验证的凭据
	for key, values := range originalReq.Header {
		if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "X-API-Key") {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	effectiveToken := strings.TrimSpace(upstreamToken)
	if effectiveToken == "" {
		effectiveToken = strings.TrimSpace(endpoint.AuthValue)
	}

	switch strings.ToLower(strings.TrimSpace(endpoint.AuthType)) {
	case "api_key":
		if effectiveToken != "" {
			req.Header.Set("x-api-key", effectiveToken)
			req.Header.Del("Authorization")
			runtime.LogInfo(a.ctx, fmt.Sprintf("使用端点API Key认证: %s", maskToken(effectiveToken)))
		} else {
			runtime.LogInfo(a.ctx, "端点API Key未配置，请求将使用原始头部")
		}
	case "auth_token", "auto":
		if effectiveToken != "" {
			req.Header.Set("Authorization", "Bearer "+effectiveToken)
			req.Header.Del("x-api-key")
			runtime.LogInfo(a.ctx, fmt.Sprintf("使用端点Bearer Token认证: %s", maskToken(effectiveToken)))
		} else {
			runtime.LogInfo(a.ctx, "端点Bearer Token未配置，请求将使用原始头部")
		}
	default:
		if effectiveToken != "" {
			req.Header.Set("Authorization", effectiveToken)
			runtime.LogInfo(a.ctx, fmt.Sprintf("使用端点自定义认证: %s", maskToken(effectiveToken)))
		} else {
			runtime.LogInfo(a.ctx, "端点未配置认证信息，使用原始请求头")
		}
	}

	// 发送请求
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("发送请求失败: %v", err))
		return nil, err
	}

	return resp, nil
}

// getKeys 获取map的所有key
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// writeJSONError 输出统一的JSON错误响应
func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	if message == "" {
		message = http.StatusText(status)
	}
	payload := map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	respBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, message, status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBytes)
}

// cleanup 清理资源
func (a *App) cleanup() {
	if a.dbManager != nil {
		a.dbManager.Close()
	}
	runtime.LogInfo(a.ctx, "✅ 统一路由架构已关闭")
}

// GetServerStatus 获取服务器状态
func (a *App) GetServerStatus() map[string]interface{} {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	host := strings.TrimSpace(a.proxyHost)
	if host == "" {
		host = defaultProxyHost
	}

	port := a.proxyPort
	if port <= 0 || port > 65535 {
		port = defaultProxyPort
	}

	configuredHost := strings.TrimSpace(a.configuredHost)
	if configuredHost == "" {
		configuredHost = defaultProxyHost
	}

	configuredPort := a.configuredPort
	if configuredPort <= 0 || configuredPort > 65535 {
		configuredPort = defaultProxyPort
	}

	status := map[string]interface{}{
		"running":           a.running,
		"host":              host,
		"port":              port,
		"configured_host":   configuredHost,
		"configured_port":   configuredPort,
		"endpoints_total":   0,
		"endpoints_healthy": 0,
		"mode":              "desktop (统一路由)",
		"architecture":      "unified_wails",
		"http_server":       "embedded",
		"api_communication": "go_methods_only",
		"config_path":       a.configPath,
	}

	if a.running {
		status["uptime"] = "运行中 (统一架构)"
	}

	return status
}

// RestartServer 重启服务
func (a *App) RestartServer() string {
	runtime.LogInfo(a.ctx, "Restarting unified architecture services")

	a.mutex.Lock()
	a.running = false
	a.mutex.Unlock()

	time.Sleep(100 * time.Millisecond)

	a.mutex.Lock()
	a.running = true
	a.mutex.Unlock()

	runtime.LogInfo(a.ctx, "✅ 统一架构服务重启成功")
	return "统一架构服务重启成功 (无HTTP服务器冲突)"
}

// GetVersionInfo 获取版本信息
func (a *App) GetVersionInfo() string {
	return fmt.Sprintf("1.0.0 - %s", getCurrentTimestamp())
}

// Greet 返回问候语
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, Welcome to CCCC Proxy Desktop with Unified Architecture!", name)
}

// GetEndpoints 返回端点列表
func (a *App) GetEndpoints() map[string]interface{} {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	runtime.LogInfo(a.ctx, "GetEndpoints: 开始获取端点列表")

	if a.db == nil {
		runtime.LogError(a.ctx, "GetEndpoints: 数据库为空")
		return map[string]interface{}{
			"success": false,
			"error":   "数据库不可用",
			"data":    []interface{}{},
		}
	}

	query := `
		SELECT id, name, url_anthropic, url_openai, endpoint_type, auth_type, auth_value,
			   enabled, priority, tags, status, response_time, last_check, created_at, updated_at,
			   model_rewrite_enabled, target_model, parameter_overrides, model_rewrite_rules
		FROM endpoints
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := a.db.Query(query)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to query endpoints: %v", err))
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("查询端点失败: %v", err),
			"data":    []interface{}{},
		}
	}
	defer rows.Close()

	var endpoints []interface{}
	for rows.Next() {
		var (
			id, name, urlAnthropic, urlOpenai, endpointType, authType, authValue sql.NullString
			enabled                                                              sql.NullBool
			priority                                                             sql.NullInt64
			tagsJSON, status, lastCheck, createdAt, updatedAt                    sql.NullString
			targetModel, parameterOverridesJSON, modelRewriteRulesJSON           sql.NullString
			responseTime                                                         sql.NullInt64
			modelRewriteEnabled                                                  sql.NullBool
		)

		if err := rows.Scan(
			&id,
			&name,
			&urlAnthropic,
			&urlOpenai,
			&endpointType,
			&authType,
			&authValue,
			&enabled,
			&priority,
			&tagsJSON,
			&status,
			&responseTime,
			&lastCheck,
			&createdAt,
			&updatedAt,
			&modelRewriteEnabled,
			&targetModel,
			&parameterOverridesJSON,
			&modelRewriteRulesJSON,
		); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("Failed to scan endpoint row: %v", err))
			continue
		}

		enabledValue := true
		if enabled.Valid {
			enabledValue = enabled.Bool
		}

		tags := decodeStringSlice(tagsJSON)
		parameterOverrides := decodeStringMap(parameterOverridesJSON)
		modelRewrite := buildModelRewriteMap(modelRewriteEnabled, targetModel, modelRewriteRulesJSON)

		endpoint := map[string]interface{}{
			"id":            id.String,
			"name":          name.String,
			"url_anthropic": urlAnthropic.String,
			"url_openai":    urlOpenai.String,
			"endpoint_type": endpointType.String,
			"auth_type":     authType.String,
			"auth_value":    authValue.String,
			"enabled":       enabledValue,
			"priority":      int(priority.Int64),
			"tags":          tags,
			"status":        status.String,
			"response_time": int(responseTime.Int64),
			"last_check":    lastCheck.String,
			"created_at":    createdAt.String,
			"updated_at":    updatedAt.String,
		}

		if len(parameterOverrides) > 0 {
			endpoint["parameter_overrides"] = parameterOverrides
		}
		if modelRewrite != nil {
			endpoint["model_rewrite"] = modelRewrite
		}
		if target := strings.TrimSpace(targetModel.String); target != "" {
			endpoint["target_model"] = target
		}

		endpoints = append(endpoints, endpoint)
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("GetEndpoints: 完成，获取到 %d 个端点", len(endpoints)))

	// 添加详细的端点信息日志
	for i, endpoint := range endpoints {
		if ep, ok := endpoint.(map[string]interface{}); ok {
			runtime.LogInfo(a.ctx, fmt.Sprintf("Endpoint[%d]: ID=%s, Name=%s, TargetModel=%v",
				i, ep["id"], ep["name"], ep["target_model"]))
		}
	}

	return map[string]interface{}{
		"success": true,
		"data":    endpoints,
	}
}

// CreateEndpoint 创建新端点
func (a *App) CreateEndpoint(endpointData map[string]interface{}) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	runtime.LogInfo(a.ctx, "CreateEndpoint called")

	if a.db == nil {
		runtime.LogError(a.ctx, "Database not available")
		return map[string]interface{}{
			"success": false,
			"message": "数据库不可用",
		}
	}

	// 生成端点ID（使用UUID避免冲突）
	endpointID := fmt.Sprintf("endpoint_%s", uuid.NewString())

	name := strings.TrimSpace(getStringFromMap(endpointData, "name"))
	if name == "" {
		return map[string]interface{}{
			"success": false,
			"message": "端点名称不能为空",
		}
	}

	urlAnthropic := strings.TrimSpace(getStringFromMap(endpointData, "url_anthropic"))
	urlOpenai := strings.TrimSpace(getStringFromMap(endpointData, "url_openai"))

	if urlAnthropic == "" && urlOpenai == "" {
		return map[string]interface{}{
			"success": false,
			"message": "至少需要配置一个URL",
		}
	}

	endpointType := strings.TrimSpace(getStringFromMap(endpointData, "endpoint_type"))
	if endpointType == "" {
		endpointType = deduceEndpointType(urlAnthropic, urlOpenai)
	}

	authType := strings.TrimSpace(getStringFromMap(endpointData, "auth_type"))
	if authType == "" {
		authType = "none"
	}

	authValue := strings.TrimSpace(getStringFromMap(endpointData, "auth_value"))

	enabled := extractBool(endpointData["enabled"], true)
	priority := extractPriority(endpointData["priority"])

	tagsJSON := "[]"
	if rawTags, exists := endpointData["tags"]; exists {
		if serialised, err := serialiseStringSlice(rawTags, "[]"); err == nil {
			tagsJSON = serialised
		} else {
			runtime.LogWarning(a.ctx, fmt.Sprintf("Invalid tags value for endpoint %s: %v", name, err))
		}
	}

	parameterOverridesJSON := "{}"
	if rawOverrides, exists := endpointData["parameter_overrides"]; exists {
		if serialised, err := serialiseStringMap(rawOverrides, "{}"); err == nil {
			parameterOverridesJSON = serialised
		} else {
			runtime.LogWarning(a.ctx, fmt.Sprintf("Invalid parameter_overrides for endpoint %s: %v", name, err))
		}
	}

	modelRewritePayload, err := extractModelRewritePayload(endpointData["model_rewrite"])
	if err != nil {
		runtime.LogWarning(a.ctx, fmt.Sprintf("Invalid model_rewrite for endpoint %s: %v", name, err))
		modelRewritePayload = defaultModelRewritePayload()
	}

	createdAt := getCurrentTimestamp()

	runtime.LogInfo(a.ctx, fmt.Sprintf(
		"Creating endpoint: ID=%s, Name=%s, AnthropicURL=%s, OpenAIURL=%s, Type=%s",
		endpointID, name, urlAnthropic, urlOpenai, endpointType,
	))

	result, err := a.db.Exec(`
		INSERT INTO endpoints (
			id, name, url_anthropic, url_openai, endpoint_type, auth_type, auth_value,
			enabled, priority, tags, status, response_time, last_check, created_at, updated_at,
			model_rewrite_enabled, target_model, parameter_overrides, model_rewrite_rules
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		endpointID,
		name,
		urlAnthropic,
		urlOpenai,
		endpointType,
		authType,
		authValue,
		enabled,
		priority,
		tagsJSON,
		"healthy",
		0,
		"",
		createdAt,
		createdAt,
		modelRewritePayload.Enabled,
		modelRewritePayload.TargetModel,
		parameterOverridesJSON,
		modelRewritePayload.RulesJSON,
	)

	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to create endpoint %s: %v", name, err))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("创建端点失败: %v", err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": "创建端点后验证失败",
		}
	}

	if rowsAffected == 0 {
		return map[string]interface{}{
			"success": false,
			"message": "端点创建失败：没有插入任何记录",
		}
	}

	a.addLog("info", fmt.Sprintf("端点 '%s' (ID: %s) 已成功创建", name, endpointID))

	return map[string]interface{}{
		"success":       true,
		"message":       fmt.Sprintf("端点 '%s' 创建成功", name),
		"id":            endpointID,
		"endpoint_name": name,
		"rows_affected": rowsAffected,
	}
}

// UpdateEndpoint 更新端点
func (a *App) UpdateEndpoint(id string, endpointData map[string]interface{}) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.db == nil {
		runtime.LogError(a.ctx, "Database not available")
		return map[string]interface{}{
			"success": false,
			"message": "数据库不可用",
		}
	}

	// 构建动态更新SQL
	setParts := []string{}
	args := []interface{}{}

	if rawName, exists := endpointData["name"]; exists {
		if name, ok := rawName.(string); ok && strings.TrimSpace(name) != "" {
			setParts = append(setParts, "name = ?")
			args = append(args, strings.TrimSpace(name))
		}
	}

	if rawURL, exists := endpointData["url_anthropic"]; exists {
		if value, ok := rawURL.(string); ok {
			setParts = append(setParts, "url_anthropic = ?")
			args = append(args, strings.TrimSpace(value))
		}
	}

	if rawURL, exists := endpointData["url_openai"]; exists {
		if value, ok := rawURL.(string); ok {
			setParts = append(setParts, "url_openai = ?")
			args = append(args, strings.TrimSpace(value))
		}
	}

	if rawType, exists := endpointData["endpoint_type"]; exists {
		if value, ok := rawType.(string); ok && strings.TrimSpace(value) != "" {
			setParts = append(setParts, "endpoint_type = ?")
			args = append(args, strings.TrimSpace(value))
		}
	}

	if rawAuthType, exists := endpointData["auth_type"]; exists {
		if value, ok := rawAuthType.(string); ok && strings.TrimSpace(value) != "" {
			setParts = append(setParts, "auth_type = ?")
			args = append(args, strings.TrimSpace(value))
		}
	}

	if rawAuthValue, exists := endpointData["auth_value"]; exists {
		if value, ok := rawAuthValue.(string); ok {
			setParts = append(setParts, "auth_value = ?")
			args = append(args, strings.TrimSpace(value))
		}
	}

	if rawEnabled, exists := endpointData["enabled"]; exists {
		setParts = append(setParts, "enabled = ?")
		args = append(args, extractBool(rawEnabled, true))
	}

	if rawPriority, exists := endpointData["priority"]; exists {
		setParts = append(setParts, "priority = ?")
		args = append(args, extractPriority(rawPriority))
	}

	if rawTags, exists := endpointData["tags"]; exists {
		if serialised, err := serialiseStringSlice(rawTags, "[]"); err == nil {
			setParts = append(setParts, "tags = ?")
			args = append(args, serialised)
		} else {
			runtime.LogWarning(a.ctx, fmt.Sprintf("Invalid tags update for endpoint %s: %v", id, err))
		}
	}

	if rawOverrides, exists := endpointData["parameter_overrides"]; exists {
		if serialised, err := serialiseStringMap(rawOverrides, "{}"); err == nil {
			setParts = append(setParts, "parameter_overrides = ?")
			args = append(args, serialised)
		} else {
			runtime.LogWarning(a.ctx, fmt.Sprintf("Invalid parameter_overrides update for endpoint %s: %v", id, err))
		}
	}

	// 检查是否有model_rewrite更新，如果有，target_model更新应该在model_rewrite处理中
	hasModelRewriteUpdate := false
	if rawModelRewrite, exists := endpointData["model_rewrite"]; exists {
		payload, err := extractModelRewritePayload(rawModelRewrite)
		if err != nil {
			runtime.LogWarning(a.ctx, fmt.Sprintf("Invalid model_rewrite update for endpoint %s: %v", id, err))
		} else {
			hasModelRewriteUpdate = true
			setParts = append(setParts, "model_rewrite_enabled = ?")
			args = append(args, payload.Enabled)
			setParts = append(setParts, "target_model = ?")
			args = append(args, payload.TargetModel)
			setParts = append(setParts, "model_rewrite_rules = ?")
			args = append(args, payload.RulesJSON)
			runtime.LogInfo(a.ctx, fmt.Sprintf("Adding model_rewrite update with target_model: '%s'", payload.TargetModel))
		}
	}

	// 只有在没有model_rewrite更新时才处理单独的target_model更新
	if !hasModelRewriteUpdate {
		if rawTargetModel, exists := endpointData["target_model"]; exists {
			if value, ok := rawTargetModel.(string); ok {
				setParts = append(setParts, "target_model = ?")
				args = append(args, strings.TrimSpace(value))
				runtime.LogInfo(a.ctx, fmt.Sprintf("Adding standalone target_model update: '%s'", strings.TrimSpace(value)))
			}
		}
	}

	if len(setParts) == 0 {
		return map[string]interface{}{
			"success": false,
			"message": "没有提供要更新的字段",
		}
	}

	// 添加更新时间和ID
	setParts = append(setParts, "updated_at = ?")
	args = append(args, getCurrentTimestamp())
	args = append(args, id)

	sql := fmt.Sprintf("UPDATE endpoints SET %s WHERE id = ?", strings.Join(setParts, ", "))

	// 添加详细的调试信息
	runtime.LogInfo(a.ctx, fmt.Sprintf("Update SQL: %s", sql))
	runtime.LogInfo(a.ctx, fmt.Sprintf("Update Args: %v", args))

	result, err := a.db.Exec(sql, args...)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to update endpoint %s: %v", id, err))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("更新端点失败: %v", err),
		}
	}

	// 检查影响的行数
	rowsAffected, _ := result.RowsAffected()
	runtime.LogInfo(a.ctx, fmt.Sprintf("Update affected %d rows", rowsAffected))

	if rowsAffected == 0 {
		runtime.LogWarning(a.ctx, fmt.Sprintf("No rows affected when updating endpoint %s", id))
		return map[string]interface{}{
			"success": false,
			"message": "没有找到要更新的端点",
		}
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Successfully updated endpoint: %s", id))
	a.addLog("info", fmt.Sprintf("端点 %s 已更新", id))

	return map[string]interface{}{
		"success": true,
		"message": "端点更新成功 (通过Go API)",
	}
}

// DeleteEndpoint 删除端点
func (a *App) DeleteEndpoint(id string) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	runtime.LogInfo(a.ctx, fmt.Sprintf("DeleteEndpoint called with ID: %s", id))

	if a.db == nil {
		runtime.LogError(a.ctx, "Database not available")
		return map[string]interface{}{
			"success": false,
			"message": "数据库不可用",
		}
	}

	// 检查ID是否为空
	if strings.TrimSpace(id) == "" {
		runtime.LogError(a.ctx, "Empty endpoint ID provided")
		return map[string]interface{}{
			"success": false,
			"message": "端点ID不能为空",
		}
	}

	// 先检查端点是否存在，同时获取端点名称用于日志
	var existingID, endpointName sql.NullString
	err := a.db.QueryRow("SELECT id, name FROM endpoints WHERE id = ?", id).Scan(&existingID, &endpointName)
	if err != nil {
		if err == sql.ErrNoRows {
			runtime.LogError(a.ctx, fmt.Sprintf("Endpoint not found with ID: %s", id))
			return map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("端点不存在: %s", id),
			}
		}
		runtime.LogError(a.ctx, fmt.Sprintf("Error checking endpoint existence: %v", err))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("检查端点失败: %v", err),
		}
	}

	// 执行删除操作
	result, err := a.db.Exec("DELETE FROM endpoints WHERE id = ?", id)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to delete endpoint %s: %v", id, err))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("删除端点失败: %v", err),
		}
	}

	// 检查是否真的删除了记录
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Error getting rows affected for endpoint %s: %v", id, err))
		return map[string]interface{}{
			"success": false,
			"message": "获取删除结果失败",
		}
	}

	if rowsAffected == 0 {
		runtime.LogError(a.ctx, fmt.Sprintf("No rows deleted for endpoint ID: %s", id))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("没有删除任何记录，端点ID: %s", id),
		}
	}

	endpointNameStr := "未知端点"
	if endpointName.Valid && endpointName.String != "" {
		endpointNameStr = endpointName.String
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Successfully deleted endpoint '%s' with ID: %s (rows affected: %d)", endpointNameStr, id, rowsAffected))

	// 添加删除操作的日志记录
	a.addLog("info", fmt.Sprintf("端点 '%s' (ID: %s) 已成功删除", endpointNameStr, id))

	return map[string]interface{}{
		"success":       true,
		"message":       fmt.Sprintf("端点 '%s' 删除成功", endpointNameStr),
		"endpoint_name": endpointNameStr,
		"rows_affected": rowsAffected,
	}
}

// TestEndpoint 测试端点
func (a *App) TestEndpoint(id string) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.db == nil {
		return map[string]interface{}{
			"success":     false,
			"message":     "数据库不可用",
			"endpoint_id": id,
		}
	}

	// 重新加载配置以确保获取最新的默认模型设置
	if a.config == nil {
		a.LoadConfig()
	}

	// 重置健康检查器以使用最新的默认模型
	a.healthChecker = nil
	if err := a.initModelRewriterAndHealthChecker(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to initialize health checker: %v", err))
		return map[string]interface{}{
			"success":     false,
			"message":     fmt.Sprintf("初始化健康检查器失败: %v", err),
			"endpoint_id": id,
		}
	}

	var (
		name, urlAnthropic, urlOpenai, endpointType, authType, authValue, tagsJSON sql.NullString
		enabled                                                                    sql.NullBool
		priority                                                                   sql.NullInt64
		modelRewriteEnabled                                                        sql.NullBool
		targetModel, parameterOverridesJSON, modelRewriteRulesJSON                 sql.NullString
	)

	err := a.db.QueryRow(`
		SELECT name, url_anthropic, url_openai, endpoint_type, auth_type, auth_value,
		       enabled, priority, tags, model_rewrite_enabled, target_model,
		       parameter_overrides, model_rewrite_rules
		FROM endpoints
		WHERE id = ?
	`, id).Scan(
		&name,
		&urlAnthropic,
		&urlOpenai,
		&endpointType,
		&authType,
		&authValue,
		&enabled,
		&priority,
		&tagsJSON,
		&modelRewriteEnabled,
		&targetModel,
		&parameterOverridesJSON,
		&modelRewriteRulesJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{
				"success":     false,
				"message":     fmt.Sprintf("端点 %s 不存在", id),
				"endpoint_id": id,
			}
		}
		return map[string]interface{}{
			"success":     false,
			"message":     fmt.Sprintf("查询端点失败: %v", err),
			"endpoint_id": id,
		}
	}

	nameStr := strings.TrimSpace(name.String)
	if nameStr == "" {
		nameStr = id
	}

	enabledValue := true
	if enabled.Valid {
		enabledValue = enabled.Bool
	}

	priorityValue := int(priority.Int64)
	if !priority.Valid || priorityValue <= 0 {
		priorityValue = 10
	}

	endpointTags := decodeStringSlice(tagsJSON)

	modelRewriteCfg, mrErr := buildModelRewriteConfigFromRow(modelRewriteEnabled, targetModel, modelRewriteRulesJSON)
	if mrErr != nil {
		runtime.LogWarning(a.ctx, fmt.Sprintf("Failed to parse model rewrite config for endpoint %s: %v", id, mrErr))
	}

	cfg := config.EndpointConfig{
		Name:         nameStr,
		URLAnthropic: strings.TrimSpace(urlAnthropic.String),
		URLOpenAI:    strings.TrimSpace(urlOpenai.String),
		AuthType:     normalizeAuthType(authType.String),
		AuthValue:    strings.TrimSpace(authValue.String),
		Enabled:      enabledValue,
		Priority:     priorityValue,
		Tags:         endpointTags,
	}

	if modelRewriteCfg != nil {
		cfg.ModelRewrite = modelRewriteCfg
	}

	testEndpoint := endpoint.NewEndpoint(cfg)
	testEndpoint.ID = id
	testEndpoint.Enabled = enabledValue
	testEndpoint.Tags = endpointTags
	testEndpoint.AuthValue = cfg.AuthValue
	testEndpoint.AuthType = cfg.AuthType

	if endpointTypeStr := strings.TrimSpace(endpointType.String); endpointTypeStr != "" {
		testEndpoint.EndpointType = endpointTypeStr
	}

	if modelRewriteCfg != nil {
		testEndpoint.ModelRewrite = modelRewriteCfg
	}

	if parameterOverrides := decodeStringMap(parameterOverridesJSON); len(parameterOverrides) > 0 {
		testEndpoint.ParameterOverrides = parameterOverrides
	}

	result, checkErr := a.healthChecker.CheckEndpointWithDetails(testEndpoint)
	if result == nil {
		result = &health.HealthCheckResult{}
	}

	testURLUsed := strings.TrimSpace(result.URL)
	if testURLUsed == "" {
		testURLUsed = firstNonEmpty(cfg.URLAnthropic, cfg.URLOpenAI)
	}

	responseTime := int(result.Duration.Milliseconds())
	if responseTime < 0 {
		responseTime = 0
	}

	statusValue := "healthy"
	message := fmt.Sprintf("端点 %s 测试成功", nameStr)
	errorMessage := ""
	if checkErr != nil {
		statusValue = "unhealthy"
		message = fmt.Sprintf("端点 %s 测试失败", nameStr)
		errorMessage = checkErr.Error()
	}

	now := getCurrentTimestamp()
	if _, updateErr := a.db.Exec(`
		UPDATE endpoints
		SET status = ?, response_time = ?, last_check = ?, updated_at = ?
		WHERE id = ?
	`, statusValue, responseTime, now, now, id); updateErr != nil {
		runtime.LogWarning(a.ctx, fmt.Sprintf("Failed to update endpoint status for %s: %v", id, updateErr))
	}

	requestID, _ := a.logEndpointTestResult(testEndpoint, result, checkErr, testURLUsed)

	requestPreview := truncateForResponse(result.RequestBody)
	responsePreview := truncateForResponse(result.ResponseBody)

	responseData := map[string]interface{}{
		"success":          checkErr == nil,
		"message":          message,
		"endpoint_id":      id,
		"endpoint_name":    nameStr,
		"status":           statusValue,
		"response_time":    responseTime,
		"status_code":      result.StatusCode,
		"url":              testURLUsed,
		"request_preview":  requestPreview,
		"response_preview": responsePreview,
		"timestamp":        now,
	}

	if requestID != "" {
		responseData["request_id"] = requestID
	}
	if len(result.RequestHeaders) > 0 {
		responseData["request_headers"] = result.RequestHeaders
	}
	if len(result.ResponseHeaders) > 0 {
		responseData["response_headers"] = result.ResponseHeaders
	}
	if result.Model != "" {
		responseData["model"] = result.Model
	}
	if checkErr != nil {
		responseData["error"] = errorMessage
		a.addLog("warn", fmt.Sprintf("端点 '%s' (ID: %s) 测试失败: %s，响应时间: %dms", nameStr, id, errorMessage, responseTime))
	} else {
		a.addLog("info", fmt.Sprintf("端点 '%s' (ID: %s) 测试成功，响应时间: %dms", nameStr, id, responseTime))
	}

	return responseData
}

// TestAllEndpoints 测试所有端点
func (a *App) TestAllEndpoints() map[string]interface{} {
	runtime.LogInfo(a.ctx, "=== TestAllEndpoints 函数开始执行 ===")
	runtime.LogInfo(a.ctx, "Testing all endpoints via Go API (统一架构)")

	if a.db == nil {
		runtime.LogError(a.ctx, "TestAllEndpoints: 数据库不可用")
		return map[string]interface{}{
			"results":       []interface{}{},
			"total":         0,
			"success_count": 0,
			"message":       "批量测试失败：数据库不可用",
			"success":       false,
		}
	}

	rows, err := a.db.Query(`
		SELECT id, name
		FROM endpoints
		ORDER BY priority DESC, created_at ASC
	`)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("TestAllEndpoints: 查询端点列表失败: %v", err))
		return map[string]interface{}{
			"results":       []interface{}{},
			"total":         0,
			"success_count": 0,
			"message":       fmt.Sprintf("批量测试失败：查询端点列表失败(%v)", err),
			"success":       false,
		}
	}
	defer rows.Close()

	type endpointRef struct {
		ID   string
		Name string
	}

	var endpointRefs []endpointRef
	for rows.Next() {
		var ref endpointRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("TestAllEndpoints: 读取端点信息失败: %v", err))
			continue
		}
		endpointRefs = append(endpointRefs, ref)
	}

	if err := rows.Err(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("TestAllEndpoints: 遍历端点行失败: %v", err))
	}

	// 添加批量测试开始的日志记录
	a.addLog("info", fmt.Sprintf("开始批量测试 %d 个端点", len(endpointRefs)))

	results := make([]interface{}, 0, len(endpointRefs))
	successCount := 0

	for idx, ref := range endpointRefs {
		if ref.Name == "" {
			runtime.LogInfo(a.ctx, fmt.Sprintf("Testing endpoint %d: ID=%s", idx, ref.ID))
		} else {
			runtime.LogInfo(a.ctx, fmt.Sprintf("Testing endpoint %d: ID=%s, Name=%s", idx, ref.ID, ref.Name))
		}

		result := a.TestEndpoint(ref.ID)
		results = append(results, result)

		if success, ok := result["success"].(bool); ok && success {
			successCount++
		}

		runtime.LogInfo(a.ctx, fmt.Sprintf("Endpoint %d test result: success=%v", idx, result["success"]))
	}

	a.addLog("info", fmt.Sprintf("批量测试完成，成功: %d/%d", successCount, len(results)))
	runtime.LogInfo(a.ctx, fmt.Sprintf("TestAllEndpoints completed: success_count=%d, total=%d", successCount, len(results)))

	return map[string]interface{}{
		"results":       results,
		"total":         len(results),
		"success_count": successCount,
		"message":       fmt.Sprintf("批量测试完成，成功: %d/%d", successCount, len(results)),
		"success":       true,
	}
}

// GetStats 返回统计信息
func (a *App) GetStats() map[string]interface{} {
	endpoints := a.GetEndpoints()
	endpointsTotal := len(endpoints)
	endpointsHealthy := 0

	for _, epInterface := range endpoints {
		if ep, ok := epInterface.(map[string]interface{}); ok {
			if enabled, ok := ep["enabled"].(bool); ok && enabled {
				if status, ok := ep["status"].(string); ok && status == "healthy" {
					endpointsHealthy++
				}
			}
		}
	}

	return map[string]interface{}{
		"uptime":              "运行中 (统一架构)",
		"requests_total":      0,
		"requests_successful": 0,
		"requests_failed":     0,
		"endpoints_total":     endpointsTotal,
		"endpoints_healthy":   endpointsHealthy,
		"running":             a.running,
		"last_updated":        getCurrentTimestamp(),
		"architecture":        "unified_wails_no_http_server",
	}
}

// GetConfigPath 获取配置文件路径
func (a *App) GetConfigPath() string {
	return a.configPath
}

// OpenConfigDirectory 打开配置目录
func (a *App) OpenConfigDirectory() {
	runtime.LogInfo(a.ctx, "Opening config directory via Go API (统一架构)")
}

// OpenURL 在默认浏览器中打开URL
func (a *App) OpenURL(url string) map[string]interface{} {
	runtime.LogInfo(a.ctx, fmt.Sprintf("Opening URL via Go API: %s", url))

	if url == "" {
		return map[string]interface{}{
			"success": false,
			"message": "URL不能为空",
		}
	}

	// 使用Wails的runtime.BrowserOpenURL方法打开URL
	// 注意：runtime.BrowserOpenURL没有返回值，如果有错误会panic
	runtime.BrowserOpenURL(a.ctx, url)

	runtime.LogInfo(a.ctx, fmt.Sprintf("Successfully opened URL: %s", url))

	return map[string]interface{}{
		"success": true,
		"message": "链接已打开",
	}
}

// LoadConfig 加载配置
func (a *App) LoadConfig() map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 默认配置
	defaultConfig := map[string]interface{}{
		"server": map[string]interface{}{
			"host":                defaultProxyHost,
			"port":                defaultProxyPort,
			"auto_sort_endpoints": false,
			"default_model":       "claude-sonnet-4-20250929",
		},
		"logging": map[string]interface{}{
			"level": "info",
		},
		"blacklist": map[string]interface{}{
			"enabled": false,
		},
		"debug": map[string]interface{}{
			"console_enabled": false,
		},
		"architecture": "unified_wails",
	}

	// 如果配置文件不存在，返回默认配置
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		runtime.LogInfo(a.ctx, fmt.Sprintf("Config file not found, using defaults: %s", a.configPath))
		// 暂时不加载端点数据，避免可能的死锁问题
		defaultConfig["endpoints"] = []interface{}{}
		if server, ok := defaultConfig["server"].(map[string]interface{}); ok {
			a.applyServerAddressNoLock(server)
		} else {
			a.applyServerAddressNoLock(nil)
		}
		if !a.running {
			a.syncActualAddressNoLock()
		}
		a.config = defaultConfig
		return defaultConfig
	}

	// 读取配置文件
	jsonData, err := os.ReadFile(a.configPath)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to read config file: %v", err))
		defaultConfig["endpoints"] = []interface{}{}
		if server, ok := defaultConfig["server"].(map[string]interface{}); ok {
			a.applyServerAddressNoLock(server)
		} else {
			a.applyServerAddressNoLock(nil)
		}
		if !a.running {
			a.syncActualAddressNoLock()
		}
		a.config = defaultConfig
		return defaultConfig
	}

	// 解析JSON配置
	var configData map[string]interface{}
	if err := json.Unmarshal(jsonData, &configData); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to parse config file: %v", err))
		defaultConfig["endpoints"] = []interface{}{}
		if server, ok := defaultConfig["server"].(map[string]interface{}); ok {
			a.applyServerAddressNoLock(server)
		} else {
			a.applyServerAddressNoLock(nil)
		}
		if !a.running {
			a.syncActualAddressNoLock()
		}
		a.config = defaultConfig
		return defaultConfig
	}

	// 合并默认配置和加载的配置，确保所有必要字段都存在
	if server, ok := configData["server"].(map[string]interface{}); ok {
		if defaultServer, ok := defaultConfig["server"].(map[string]interface{}); ok {
			for key, defaultValue := range defaultServer {
				if _, exists := server[key]; !exists {
					server[key] = defaultValue
				}
			}
		}
		a.applyServerAddressNoLock(server)
	} else {
		configData["server"] = defaultConfig["server"]
		if serverCfg, ok := configData["server"].(map[string]interface{}); ok {
			a.applyServerAddressNoLock(serverCfg)
		} else {
			a.applyServerAddressNoLock(nil)
		}
	}

	if !a.running {
		a.syncActualAddressNoLock()
	}

	// 暂时不加载端点数据，避免死锁问题
	configData["endpoints"] = []interface{}{}

	// 将配置保存到App结构体中
	a.config = configData

	runtime.LogInfo(a.ctx, fmt.Sprintf("Configuration loaded successfully from: %s", a.configPath))

	return configData
}

// SaveConfig 保存配置
func (a *App) SaveConfig(configData map[string]interface{}) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	runtime.LogInfo(a.ctx, "Saving config via Go API (统一架构)")

	// 确保配置目录存在
	configDir := filepath.Dir(a.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to create config directory: %v", err))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("创建配置目录失败: %v", err),
		}
	}

	if serverCfg, ok := configData["server"].(map[string]interface{}); ok {
		a.applyServerAddressNoLock(serverCfg)
	} else {
		a.applyServerAddressNoLock(nil)
	}
	if !a.running {
		a.syncActualAddressNoLock()
	}

	// 将配置数据序列化为JSON
	jsonData, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to marshal config: %v", err))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("配置序列化失败: %v", err),
		}
	}

	// 写入配置文件
	if err := os.WriteFile(a.configPath, jsonData, 0644); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to write config file: %v", err))
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("保存配置文件失败: %v", err),
		}
	}

	// 更新App结构体中的配置缓存
	a.config = configData

	runtime.LogInfo(a.ctx, fmt.Sprintf("Configuration saved successfully to: %s", a.configPath))

	return map[string]interface{}{
		"success": true,
		"message": "配置保存成功 (通过Go API)",
		"path":    a.configPath,
	}
}

// GetLogs 获取日志
func (a *App) GetLogs(params map[string]interface{}) map[string]interface{} {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 确保日志记录器已初始化
	if a.requestLogger == nil {
		if err := a.initRequestLogger(); err != nil {
			return map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("初始化日志记录器失败: %v", err),
			}
		}
	}

	// 解析参数
	page := 1
	limit := 20
	search := ""
	clientType := ""
	statusRange := ""
	streamingOnly := false
	failedOnly := false
	hasError := false
	model := ""
	withThinking := false
	cleanup := -1 // 默认值设为-1，表示不执行清理
	export := false

	// 解析字符串参数
	if p, ok := params["page"].(string); ok {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if l, ok := params["limit"].(string); ok {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if s, ok := params["search"].(string); ok {
		search = s
	}

	if ct, ok := params["client_type"].(string); ok {
		clientType = ct
	}

	if sr, ok := params["status_range"].(string); ok {
		statusRange = sr
	}

	if so, ok := params["streaming_only"].(bool); ok {
		streamingOnly = so
	}

	if fo, ok := params["failed_only"].(bool); ok {
		failedOnly = fo
	}

	if he, ok := params["has_error"].(bool); ok {
		hasError = he
	}

	if m, ok := params["model"].(string); ok {
		model = m
	}

	if wt, ok := params["with_thinking"].(bool); ok {
		withThinking = wt
	}

	if c, ok := params["cleanup"].(float64); ok {
		cleanup = int(c)
	}

	if ex, ok := params["export"].(bool); ok {
		export = ex
	}

	// 处理清理请求 - 从数据库清理旧日志
	if cleanup >= 0 { // 只有明确提供cleanup参数时才执行清理

		if cleanup == 0 {
			// 清除所有日志 - 使用logger的存储来清理
			if a.requestLogger == nil {
				return map[string]interface{}{
					"success": false,
					"error":   "日志记录器未初始化",
				}
			}

			// 使用logger的CleanupLogsByDays方法清理所有日志
			rowsAffected, err := a.requestLogger.CleanupLogsByDays(0)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("清理所有日志失败: %v", err),
				}
			}

			// 重置示例数据初始化标记，确保清理后不会重新插入示例数据
			_, err = a.db.Exec("DELETE FROM app_settings WHERE key = 'sample_logs_initialized'")
			if err != nil {
				runtime.LogWarning(a.ctx, fmt.Sprintf("Failed to reset sample logs flag: %v", err))
			}

			cleanupMsg := fmt.Sprintf("已清理所有日志，共 %d 条记录", rowsAffected)
			a.addLog("info", cleanupMsg)
			return map[string]interface{}{
				"success":       true,
				"rows_affected": rowsAffected,
				"message":       cleanupMsg,
			}
		} else {
			// 清除指定天数前的日志 - 使用logger的存储来清理
			if a.requestLogger == nil {
				return map[string]interface{}{
					"success": false,
					"error":   "日志记录器未初始化",
				}
			}

			rowsAffected, err := a.requestLogger.CleanupLogsByDays(cleanup)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("清理日志失败: %v", err),
				}
			}
			cleanupMsg := fmt.Sprintf("已清理 %d 天前的 %d 条日志", cleanup, rowsAffected)
			a.addLog("info", cleanupMsg)
			return map[string]interface{}{
				"success":       true,
				"rows_affected": rowsAffected,
				"message":       cleanupMsg,
			}
		}
	}

	// 构建SQL查询条件
	whereConditions := []string{}
	args := []interface{}{}

	// 搜索条件
	if search != "" {
		whereConditions = append(whereConditions, "(request_id LIKE ? OR endpoint LIKE ? OR model LIKE ? OR path LIKE ?)")
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	// 客户端类型过滤
	if clientType != "" && clientType != "all" {
		whereConditions = append(whereConditions, "client_type = ?")
		args = append(args, clientType)
	}

	// 状态码范围过滤
	if statusRange != "" && statusRange != "all" {
		switch statusRange {
		case "2xx":
			whereConditions = append(whereConditions, "status_code >= 200 AND status_code < 300")
		case "4xx":
			whereConditions = append(whereConditions, "status_code >= 400 AND status_code < 500")
		case "5xx":
			whereConditions = append(whereConditions, "status_code >= 500")
		case "error":
			whereConditions = append(whereConditions, "status_code >= 400")
		}
	}

	// 流式响应过滤
	if streamingOnly {
		whereConditions = append(whereConditions, "is_streaming = 1")
	}

	// 模型重写过滤
	if model == "any" {
		whereConditions = append(whereConditions, "model_rewrite_applied = 1")
	}

	// 错误过滤
	if failedOnly || hasError {
		whereConditions = append(whereConditions, "(status_code >= 400 OR error != '')")
	}

	// 思考模式过滤
	if withThinking {
		whereConditions = append(whereConditions, "thinking_enabled = 1")
	}

	// 使用日志记录器获取数据
	logs, total, err := a.requestLogger.GetLogs(limit, (page-1)*limit, failedOnly)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("查询日志失败: %v", err),
		}
	}

	// 应用过滤条件（由于GetLogs方法只支持基本的failedOnly过滤，我们需要在这里应用其他过滤条件）
	var filteredLogs []*logger.RequestLog
	if search != "" || clientType != "" || statusRange != "" || streamingOnly || hasError || model != "" || withThinking {
		filteredLogs = make([]*logger.RequestLog, 0)
		for _, log := range logs {
			// 搜索过滤
			if search != "" {
				searchLower := strings.ToLower(search)
				if !strings.Contains(strings.ToLower(log.RequestID), searchLower) &&
					!strings.Contains(strings.ToLower(log.Endpoint), searchLower) &&
					!strings.Contains(strings.ToLower(log.Path), searchLower) &&
					!strings.Contains(strings.ToLower(log.Model), searchLower) {
					continue
				}
			}

			// 客户端类型过滤
			if clientType != "" && clientType != "all" && log.ClientType != clientType {
				continue
			}

			// 状态码范围过滤
			if statusRange != "" && statusRange != "all" {
				switch statusRange {
				case "2xx":
					if log.StatusCode < 200 || log.StatusCode >= 300 {
						continue
					}
				case "4xx":
					if log.StatusCode < 400 || log.StatusCode >= 500 {
						continue
					}
				case "5xx":
					if log.StatusCode < 500 {
						continue
					}
				case "error":
					if log.StatusCode < 400 && log.Error == "" {
						continue
					}
				}
			}

			// 流式响应过滤
			if streamingOnly && !log.IsStreaming {
				continue
			}

			// 模型重写过滤
			if model == "any" && !log.ModelRewriteApplied {
				continue
			}

			// 错误过滤
			if hasError && log.StatusCode < 400 && log.Error == "" {
				continue
			}

			// 思考模式过滤
			if withThinking && !log.ThinkingEnabled {
				continue
			}

			filteredLogs = append(filteredLogs, log)
		}
	} else {
		filteredLogs = logs
	}

	// 🔴 CRITICAL: total 必须保持为数据库返回的真实总数（包含 failedOnly 等 DB 层过滤）
	// 禁止用页内过滤结果覆盖，否则前端分页总数会错误地显示为当前页大小（20）
	// 如果需要精确的过滤后总数，应该将过滤逻辑下沉到数据库层（logger.GetLogs）
	// 当前实现：DB 层过滤 failedOnly，内存层过滤其他条件，total 反映 DB 层结果

	// 转换日志数据为前端格式
	logEntries := []map[string]interface{}{}
	for _, log := range filteredLogs {
		logMap := map[string]interface{}{
			"id":                        strconv.Itoa(int(log.Timestamp.Unix())),
			"timestamp":                 a.formatTimestamp(log.Timestamp),
			"request_id":                log.RequestID,
			"endpoint":                  log.Endpoint,
			"method":                    log.Method,
			"path":                      log.Path,
			"status_code":               log.StatusCode,
			"duration_ms":               log.DurationMs,
			"attempt_number":            log.AttemptNumber,
			"request_body_size":         log.RequestBodySize,
			"response_body_size":        log.ResponseBodySize,
			"is_streaming":              log.IsStreaming,
			"model":                     log.Model,
			"original_model":            log.OriginalModel,
			"rewritten_model":           log.RewrittenModel,
			"model_rewrite_applied":     log.ModelRewriteApplied,
			"thinking_enabled":          log.ThinkingEnabled,
			"thinking_budget_tokens":    log.ThinkingBudgetTokens,
			"format_converted":          log.FormatConverted,
			"request_headers":           cloneStringMap(log.RequestHeaders),
			"response_headers":          cloneStringMap(log.ResponseHeaders),
			"request_body":              log.RequestBody,
			"response_body":             log.ResponseBody,
			"original_request_headers":  cloneStringMap(log.OriginalRequestHeaders),
			"original_request_body":     log.OriginalRequestBody,
			"final_request_headers":     cloneStringMap(log.FinalRequestHeaders),
			"final_request_body":        log.FinalRequestBody,
			"original_response_headers": cloneStringMap(log.OriginalResponseHeaders),
			"original_response_body":    log.OriginalResponseBody,
			"final_response_headers":    cloneStringMap(log.FinalResponseHeaders),
			"final_response_body":       log.FinalResponseBody,
			"original_request_url":      log.OriginalRequestURL,
		}

		// 处理可为空的字符串字段
		if log.Error != "" {
			logMap["error"] = log.Error
		}
		if log.FinalRequestURL != "" {
			logMap["final_request_url"] = log.FinalRequestURL
		}
		if log.ClientType != "" {
			logMap["client_type"] = log.ClientType
		} else {
			logMap["client_type"] = ""
		}
		if log.RequestFormat != "" {
			logMap["request_format"] = log.RequestFormat
		} else {
			logMap["request_format"] = ""
		}
		if log.TargetFormat != "" {
			logMap["target_format"] = log.TargetFormat
		} else {
			logMap["target_format"] = ""
		}
		if log.EndpointBlacklistReason != "" {
			logMap["endpoint_blacklist_reason"] = log.EndpointBlacklistReason
		}
		if log.SessionID != "" {
			logMap["session_id"] = log.SessionID
		}

		logEntries = append(logEntries, logMap)
	}

	if export {
		return map[string]interface{}{
			"success": true,
			"logs":    logEntries,
			"total":   total,
			"export":  true,
		}
	}

	return map[string]interface{}{
		"success": true,
		"logs":    logEntries,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"message": fmt.Sprintf("获取到 %d 条日志，第 %d 页，共 %d 条", len(logEntries), page, total),
	}
}

// GetSystemInfo 获取系统信息
func (a *App) GetSystemInfo() map[string]interface{} {
	return map[string]interface{}{
		"platform":          "Desktop Application (统一架构)",
		"architecture":      "Unified Wails (无HTTP服务器)",
		"go_version":        "1.23+",
		"wails_version":     "2.10+",
		"app_version":       "1.0.0",
		"uptime":            "运行中",
		"api_communication": "Go Methods Only",
		"http_server":       "Disabled (路由冲突已解决)",
		"config_path":       a.configPath,
	}
}

// GetEndpointStats 获取端点统计
func (a *App) GetEndpointStats() []interface{} {
	endpoints := a.GetEndpoints()
	result := make([]interface{}, 0, len(endpoints))

	for _, epInterface := range endpoints {
		ep, ok := epInterface.(map[string]interface{})
		if !ok {
			continue
		}

		stat := map[string]interface{}{
			"name":              ep["name"],
			"requests":          0,
			"success_rate":      100.0,
			"avg_response_time": 0,
			"status":            ep["status"],
			"enabled":           ep["enabled"],
			"api_type":          "Go Methods (统一架构)",
		}
		result = append(result, stat)
	}

	return result
}

// GetRequestTrends 获取请求趋势
func (a *App) GetRequestTrends(timeRange string) map[string]interface{} {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 根据时间范围确定数据点数量和间隔
	var dataPoints int
	var interval time.Duration

	switch timeRange {
	case "1h":
		dataPoints = 12 // 每5分钟一个点
		interval = 5 * time.Minute
	case "24h":
		dataPoints = 24 // 每小时一个点
		interval = 1 * time.Hour
	case "7d":
		dataPoints = 7 // 每天一个点
		interval = 24 * time.Hour
	case "30d":
		dataPoints = 30 // 每天一个点
		interval = 24 * time.Hour
	default:
		dataPoints = 12
		interval = 1 * time.Hour
	}

	// 生成趋势数据
	data := make([]interface{}, 0, dataPoints)
	now := time.Now()

	for i := dataPoints - 1; i >= 0; i-- {
		timePoint := now.Add(-time.Duration(i) * interval)

		// 从日志中统计这个时间段的请求
		requests := 0
		successes := 0
		failures := 0

		for _, log := range a.logs {
			// 只统计包含请求信息的日志
			if log.RequestID == "" {
				continue
			}

			// 解析日志时间
			logTime, err := time.Parse("2006-01-02 15:04:05", log.Timestamp)
			if err != nil {
				continue
			}

			// 检查是否在当前时间窗口
			if (logTime.Equal(timePoint) || logTime.After(timePoint)) &&
				(logTime.Before(timePoint.Add(interval)) || logTime.Equal(timePoint.Add(interval))) {
				requests++
				if log.Level == "error" {
					failures++
				} else {
					successes++
				}
			}
		}

		data = append(data, map[string]interface{}{
			"time":      timePoint.Format("2006-01-02T15:04:05Z"),
			"requests":  requests,
			"successes": successes,
			"failures":  failures,
		})
	}

	// 计算总计
	totalRequests := 0
	totalSuccesses := 0
	totalFailures := 0

	for _, point := range data {
		if pointMap, ok := point.(map[string]interface{}); ok {
			if req, ok := pointMap["requests"].(int); ok {
				totalRequests += req
			}
			if succ, ok := pointMap["successes"].(int); ok {
				totalSuccesses += succ
			}
			if fail, ok := pointMap["failures"].(int); ok {
				totalFailures += fail
			}
		}
	}

	successRate := 0.0
	if totalRequests > 0 {
		successRate = float64(totalSuccesses) / float64(totalRequests) * 100
	}

	return map[string]interface{}{
		"timeRange":      timeRange,
		"data":           data,
		"totalRequests":  totalRequests,
		"totalSuccesses": totalSuccesses,
		"totalFailures":  totalFailures,
		"successRate":    successRate,
		"message":        fmt.Sprintf("趋势数据 (%s) - 统一架构", timeRange),
	}
}

func (a *App) ensureEndpointSchema(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(endpoints)")
	if err != nil {
		return err
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		columns[name] = true
	}

	migrations := []struct {
		name string
		sql  string
	}{
		{"tags", "ALTER TABLE endpoints ADD COLUMN tags TEXT"},
		{"status", "ALTER TABLE endpoints ADD COLUMN status TEXT DEFAULT 'healthy'"},
		{"response_time", "ALTER TABLE endpoints ADD COLUMN response_time INTEGER DEFAULT 0"},
		{"last_check", "ALTER TABLE endpoints ADD COLUMN last_check TEXT"},
		{"created_at", "ALTER TABLE endpoints ADD COLUMN created_at TEXT"},
		{"updated_at", "ALTER TABLE endpoints ADD COLUMN updated_at TEXT"},
		{"model_rewrite_enabled", "ALTER TABLE endpoints ADD COLUMN model_rewrite_enabled BOOLEAN DEFAULT FALSE"},
		{"target_model", "ALTER TABLE endpoints ADD COLUMN target_model TEXT"},
		{"parameter_overrides", "ALTER TABLE endpoints ADD COLUMN parameter_overrides TEXT"},
		{"model_rewrite_rules", "ALTER TABLE endpoints ADD COLUMN model_rewrite_rules TEXT"},
	}

	for _, migration := range migrations {
		if !columns[migration.name] {
			if _, err := db.Exec(migration.sql); err != nil {
				lowerErr := strings.ToLower(err.Error())
				if strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "exists") {
					continue
				}
				return fmt.Errorf("failed to add column %s: %w", migration.name, err)
			}
		}
	}

	return nil
}

// ensureRequestLogsSchema 确保request_logs表存在并包含所有必要字段
func (a *App) ensureRequestLogsSchema(db *sql.DB) error {
	// 创建request_logs表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS request_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		request_id TEXT DEFAULT '',
		endpoint TEXT DEFAULT '',
		method TEXT DEFAULT '',
		path TEXT DEFAULT '',
		status_code INTEGER DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		attempt_number INTEGER DEFAULT 1,
		request_headers TEXT DEFAULT '{}',
		request_body TEXT DEFAULT '',
		request_body_size INTEGER DEFAULT 0,
		response_headers TEXT DEFAULT '{}',
		response_body TEXT DEFAULT '',
		response_body_size INTEGER DEFAULT 0,
		is_streaming INTEGER DEFAULT 0,
		was_streaming INTEGER DEFAULT 0,
		model TEXT DEFAULT '',
		error TEXT DEFAULT '',
		tags TEXT DEFAULT '[]',
		content_type_override TEXT DEFAULT '',
		session_id TEXT DEFAULT '',
		original_model TEXT DEFAULT '',
		rewritten_model TEXT DEFAULT '',
		model_rewrite_applied INTEGER DEFAULT 0,
		thinking_enabled INTEGER DEFAULT 0,
		thinking_budget_tokens INTEGER DEFAULT 0,
		original_request_url TEXT DEFAULT '',
		original_request_headers TEXT DEFAULT '{}',
		original_request_body TEXT DEFAULT '',
		original_response_headers TEXT DEFAULT '{}',
		original_response_body TEXT DEFAULT '',
		final_request_url TEXT DEFAULT '',
		final_request_headers TEXT DEFAULT '{}',
		final_request_body TEXT DEFAULT '',
		final_response_headers TEXT DEFAULT '{}',
		final_response_body TEXT DEFAULT '',
		request_body_hash TEXT DEFAULT '',
		response_body_hash TEXT DEFAULT '',
		request_body_truncated INTEGER DEFAULT 0,
		response_body_truncated INTEGER DEFAULT 0,
		conversion_path TEXT DEFAULT '',
		supports_responses_flag TEXT DEFAULT '',
		blacklist_causing_request_ids TEXT DEFAULT '[]',
		endpoint_blacklisted_at DATETIME,
		endpoint_blacklist_reason TEXT DEFAULT '',
		client_type TEXT DEFAULT '',
		request_format TEXT DEFAULT '',
		target_format TEXT DEFAULT '',
		format_converted INTEGER DEFAULT 0,
		detection_confidence REAL DEFAULT 0,
		detected_by TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create request_logs table: %w", err)
	}

	// 检查并补充缺失的列
	rows, err := db.Query("PRAGMA table_info(request_logs)")
	if err != nil {
		return fmt.Errorf("failed to inspect request_logs schema: %w", err)
	}
	defer rows.Close()

	existingColumns := map[string]bool{}
	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan request_logs schema: %w", err)
		}
		existingColumns[name] = true
	}

	requiredColumns := map[string]string{
		"request_id":                    "TEXT DEFAULT ''",
		"endpoint":                      "TEXT DEFAULT ''",
		"path":                          "TEXT DEFAULT ''",
		"duration_ms":                   "INTEGER DEFAULT 0",
		"attempt_number":                "INTEGER DEFAULT 1",
		"request_headers":               "TEXT DEFAULT '{}'",
		"request_body":                  "TEXT DEFAULT ''",
		"request_body_size":             "INTEGER DEFAULT 0",
		"response_headers":              "TEXT DEFAULT '{}'",
		"response_body":                 "TEXT DEFAULT ''",
		"response_body_size":            "INTEGER DEFAULT 0",
		"is_streaming":                  "INTEGER DEFAULT 0",
		"was_streaming":                 "INTEGER DEFAULT 0",
		"model":                         "TEXT DEFAULT ''",
		"error":                         "TEXT DEFAULT ''",
		"tags":                          "TEXT DEFAULT '[]'",
		"content_type_override":         "TEXT DEFAULT ''",
		"session_id":                    "TEXT DEFAULT ''",
		"original_model":                "TEXT DEFAULT ''",
		"rewritten_model":               "TEXT DEFAULT ''",
		"model_rewrite_applied":         "INTEGER DEFAULT 0",
		"thinking_enabled":              "INTEGER DEFAULT 0",
		"thinking_budget_tokens":        "INTEGER DEFAULT 0",
		"original_request_url":          "TEXT DEFAULT ''",
		"original_request_headers":      "TEXT DEFAULT '{}'",
		"original_request_body":         "TEXT DEFAULT ''",
		"original_response_headers":     "TEXT DEFAULT '{}'",
		"original_response_body":        "TEXT DEFAULT ''",
		"final_request_url":             "TEXT DEFAULT ''",
		"final_request_headers":         "TEXT DEFAULT '{}'",
		"final_request_body":            "TEXT DEFAULT ''",
		"final_response_headers":        "TEXT DEFAULT '{}'",
		"final_response_body":           "TEXT DEFAULT ''",
		"request_body_hash":             "TEXT DEFAULT ''",
		"response_body_hash":            "TEXT DEFAULT ''",
		"request_body_truncated":        "INTEGER DEFAULT 0",
		"response_body_truncated":       "INTEGER DEFAULT 0",
		"conversion_path":               "TEXT DEFAULT ''",
		"supports_responses_flag":       "TEXT DEFAULT ''",
		"blacklist_causing_request_ids": "TEXT DEFAULT '[]'",
		"endpoint_blacklisted_at":       "DATETIME",
		"endpoint_blacklist_reason":     "TEXT DEFAULT ''",
		"client_type":                   "TEXT DEFAULT ''",
		"request_format":                "TEXT DEFAULT ''",
		"target_format":                 "TEXT DEFAULT ''",
		"format_converted":              "INTEGER DEFAULT 0",
		"detection_confidence":          "REAL DEFAULT 0",
		"detected_by":                   "TEXT DEFAULT ''",
		"created_at":                    "DATETIME DEFAULT CURRENT_TIMESTAMP",
	}

	for column, definition := range requiredColumns {
		if !existingColumns[column] {
			alterSQL := fmt.Sprintf("ALTER TABLE request_logs ADD COLUMN %s %s", column, definition)
			if _, err := db.Exec(alterSQL); err != nil {
				return fmt.Errorf("failed to add column %s: %w", column, err)
			}
			runtime.LogInfo(a.ctx, fmt.Sprintf("Added missing column to request_logs: %s", column))
		}
	}

	// 创建索引以优化查询性能
	indexes := []struct {
		name string
		sql  string
	}{
		{"idx_timestamp", "CREATE INDEX IF NOT EXISTS idx_timestamp ON request_logs(timestamp)"},
		{"idx_request_id", "CREATE INDEX IF NOT EXISTS idx_request_id ON request_logs(request_id)"},
		{"idx_endpoint", "CREATE INDEX IF NOT EXISTS idx_endpoint ON request_logs(endpoint)"},
		{"idx_status_code", "CREATE INDEX IF NOT EXISTS idx_status_code ON request_logs(status_code)"},
		{"idx_client_type", "CREATE INDEX IF NOT EXISTS idx_client_type ON request_logs(client_type)"},
		{"idx_request_format", "CREATE INDEX IF NOT EXISTS idx_request_format ON request_logs(request_format)"},
		{"idx_format_converted", "CREATE INDEX IF NOT EXISTS idx_format_converted ON request_logs(format_converted)"},
	}

	for _, index := range indexes {
		if _, err := db.Exec(index.sql); err != nil {
			// 索引创建失败不应该阻止应用启动，只记录警告
			runtime.LogWarning(a.ctx, fmt.Sprintf("Failed to create index %s: %v", index.name, err))
		}
	}

	return nil
}

func (a *App) seedDefaultEndpoints() error {
	// 不再创建任何默认端点，用户需要手动添加
	runtime.LogInfo(a.ctx, "Skipping default endpoint seeding - no default endpoints will be created")
	a.addLog("info", "跳过默认端点创建 - 用户需要手动添加端点")
	return nil
}

// seedSampleLogs 创建示例日志数据用于演示
func (a *App) seedSampleLogs() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// 检查是否已经初始化过示例数据
	var initialized string
	err := a.db.QueryRow("SELECT value FROM app_settings WHERE key = 'sample_logs_initialized'").Scan(&initialized)
	if err == nil && initialized == "true" {
		return nil // 已经初始化过，不需要再次插入
	}

	// 检查是否已有示例数据（额外保险）
	var count int
	err = a.db.QueryRow("SELECT COUNT(*) FROM request_logs WHERE request_id LIKE 'req_demo_%'").Scan(&count)
	if err == nil && count > 0 {
		// 示例数据已存在，标记为已初始化
		_, err := a.db.Exec("INSERT OR REPLACE INTO app_settings (key, value) VALUES ('sample_logs_initialized', 'true')")
		if err != nil {
			runtime.LogWarning(a.ctx, fmt.Sprintf("Failed to mark sample logs as initialized: %v", err))
		}
		return nil
	}

	// 创建示例日志数据
	sampleLogs := []struct {
		timestamp               string
		requestID               string
		endpoint                string
		method                  string
		path                    string
		statusCode              int
		durationMs              int
		attemptNumber           int
		requestBodySize         int
		responseBodySize        int
		isStreaming             bool
		model                   string
		originalModel           string
		rewrittenModel          string
		modelRewriteApplied     bool
		thinkingEnabled         bool
		thinkingBudgetTokens    int
		finalRequestURL         string
		clientType              string
		requestFormat           string
		targetFormat            string
		formatConverted         bool
		sessionID               string
		error                   string
		endpointBlacklistReason string
	}{
		{
			timestamp:            time.Now().Add(-5 * time.Minute).Format("2006/01/02 15:04:05"),
			requestID:            "req_demo_001",
			endpoint:             "https://api.example.com/v1/messages",
			method:               "POST",
			path:                 "/v1/messages",
			statusCode:           200,
			durationMs:           1234,
			attemptNumber:        1,
			requestBodySize:      1024,
			responseBodySize:     2048,
			isStreaming:          true,
			model:                "claude-sonnet-4-20250514",
			originalModel:        "claude-sonnet-4-20250514",
			rewrittenModel:       "glm-4.6",
			modelRewriteApplied:  true,
			thinkingEnabled:      true,
			thinkingBudgetTokens: 8192,
			finalRequestURL:      "https://api.example.com/v1/messages",
			clientType:           "claude-code",
			requestFormat:        "anthropic",
			targetFormat:         "anthropic",
			formatConverted:      false,
			sessionID:            "session_001",
		},
		{
			timestamp:            time.Now().Add(-3 * time.Minute).Format("2006/01/02 15:04:05"),
			requestID:            "req_demo_002",
			endpoint:             "https://api.openai.com/v1/chat/completions",
			method:               "POST",
			path:                 "/v1/chat/completions",
			statusCode:           429,
			durationMs:           567,
			attemptNumber:        1,
			requestBodySize:      512,
			responseBodySize:     128,
			isStreaming:          false,
			model:                "gpt-4",
			originalModel:        "gpt-4",
			rewrittenModel:       "",
			modelRewriteApplied:  false,
			thinkingEnabled:      false,
			thinkingBudgetTokens: 0,
			finalRequestURL:      "https://api.openai.com/v1/chat/completions",
			clientType:           "codex",
			requestFormat:        "openai",
			targetFormat:         "openai",
			formatConverted:      false,
			sessionID:            "session_002",
			error:                "Rate limit exceeded",
		},
		{
			timestamp:            time.Now().Add(-1 * time.Minute).Format("2006/01/02 15:04:05"),
			requestID:            "req_demo_003",
			endpoint:             "https://api.example.com/v1/responses",
			method:               "POST",
			path:                 "/v1/responses",
			statusCode:           200,
			durationMs:           2890,
			attemptNumber:        1,
			requestBodySize:      2048,
			responseBodySize:     4096,
			isStreaming:          true,
			model:                "claude-3-5-sonnet-20241022",
			originalModel:        "claude-3-5-sonnet-20241022",
			rewrittenModel:       "claude-sonnet-4-20250514",
			modelRewriteApplied:  true,
			thinkingEnabled:      false,
			thinkingBudgetTokens: 0,
			finalRequestURL:      "https://api.example.com/v1/chat/completions",
			clientType:           "codex",
			requestFormat:        "openai",
			targetFormat:         "anthropic",
			formatConverted:      true,
			sessionID:            "session_003",
		},
	}

	for _, log := range sampleLogs {
		_, err := a.db.Exec(`
			INSERT INTO request_logs (
				timestamp, request_id, endpoint, method, path, status_code, duration_ms,
				attempt_number, request_body_size, response_body_size, is_streaming,
				model, original_model, rewritten_model, model_rewrite_applied,
				thinking_enabled, thinking_budget_tokens, final_request_url,
				client_type, request_format, target_format, format_converted,
				session_id, error, endpoint_blacklist_reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			log.timestamp, log.requestID, log.endpoint, log.method, log.path, log.statusCode, log.durationMs,
			log.attemptNumber, log.requestBodySize, log.responseBodySize, log.isStreaming,
			log.model, log.originalModel, log.rewrittenModel, log.modelRewriteApplied,
			log.thinkingEnabled, log.thinkingBudgetTokens, log.finalRequestURL,
			log.clientType, log.requestFormat, log.targetFormat, log.formatConverted,
			log.sessionID, log.error, log.endpointBlacklistReason,
		)
		if err != nil {
			return fmt.Errorf("failed to insert sample log: %w", err)
		}
	}

	// 标记示例数据已初始化
	_, err = a.db.Exec("INSERT OR REPLACE INTO app_settings (key, value) VALUES ('sample_logs_initialized', 'true')")
	if err != nil {
		runtime.LogWarning(a.ctx, fmt.Sprintf("Failed to mark sample logs as initialized: %v", err))
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Seeded %d sample log entries", len(sampleLogs)))
	a.addLog("info", fmt.Sprintf("已创建 %d 条示例日志记录", len(sampleLogs)))

	return nil
}

// addEndpointTestLog 向 request_logs 表中添加端点测试记录
func (a *App) addEndpointTestLog(endpointID, endpointName, testURL string, success bool, responseTime int, errorMessage string) {
	if a.db == nil {
		return
	}

	// 生成唯一的请求ID
	requestID := fmt.Sprintf("test_%s_%d", endpointID, time.Now().UnixNano())

	// 确定状态码和路径
	statusCode := 200
	path := "/health-check"
	if !success {
		statusCode = 503 // Service Unavailable
	}

	// 解析URL获取路径
	if parsedURL, err := url.Parse(testURL); err == nil {
		path = parsedURL.Path
		if path == "" {
			path = "/"
		}
	}

	// 构建错误信息
	errorMsg := ""
	if !success && errorMessage != "" {
		errorMsg = errorMessage
	}

	// 插入到 request_logs 表
	_, err := a.db.Exec(`
		INSERT INTO request_logs (
			timestamp, request_id, endpoint, method, path, status_code, duration_ms,
			attempt_number, request_body_size, response_body_size, is_streaming,
			model, original_model, rewritten_model, model_rewrite_applied,
			thinking_enabled, thinking_budget_tokens, final_request_url,
			client_type, request_format, target_format, format_converted,
			session_id, error, endpoint_blacklist_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		getCurrentTimestamp(),              // timestamp
		requestID,                          // request_id
		testURL,                            // endpoint
		"GET",                              // method
		path,                               // path
		statusCode,                         // status_code
		responseTime,                       // duration_ms
		1,                                  // attempt_number
		0,                                  // request_body_size
		0,                                  // response_body_size
		false,                              // is_streaming
		"endpoint-test",                    // model
		"endpoint-test",                    // original_model
		"endpoint-test",                    // rewritten_model
		false,                              // model_rewrite_applied
		false,                              // thinking_enabled
		0,                                  // thinking_budget_tokens
		testURL,                            // final_request_url
		"cccc-desktop",                     // client_type
		"test",                             // request_format
		"test",                             // target_format
		false,                              // format_converted
		fmt.Sprintf("test_%s", endpointID), // session_id
		errorMsg,                           // error
		"",                                 // endpoint_blacklist_reason
	)

	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to insert endpoint test log: %v", err))
	}
}

const (
	healthLogPreviewLimit      = 2048
	healthResponsePreviewLimit = 512
)

func (a *App) logEndpointTestResult(ep *endpoint.Endpoint, result *health.HealthCheckResult, checkErr error, finalURL string) (string, *logger.RequestLog) {
	if a.requestLogger == nil || ep == nil {
		return "", nil
	}

	if result == nil {
		result = &health.HealthCheckResult{}
	}

	requestID := fmt.Sprintf("health-%s-%d", ep.ID, time.Now().UnixNano())

	reqHeaders := cloneStringMap(result.RequestHeaders)
	respHeaders := cloneStringMap(result.ResponseHeaders)

	requestBody := string(result.RequestBody)
	responseBody := string(result.ResponseBody)

	truncatedReq, reqTruncated := truncateStringForLog(requestBody, healthLogPreviewLimit)
	truncatedResp, respTruncated := truncateStringForLog(responseBody, healthLogPreviewLimit)

	path := finalURL
	if parsed, err := url.Parse(result.URL); err == nil && parsed != nil && parsed.Path != "" {
		path = parsed.Path
	}
	if path == "" {
		path = "/"
	}

	method := strings.ToUpper(strings.TrimSpace(result.Method))
	if method == "" {
		method = http.MethodPost
	}

	durationMs := result.Duration.Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}

	healthStatus := "healthy"
	if checkErr != nil {
		healthStatus = "unhealthy"
	}

	logEntry := &logger.RequestLog{
		Timestamp:             time.Now(),
		RequestID:             requestID,
		Endpoint:              ep.Name,
		Method:                method,
		Path:                  path,
		StatusCode:            result.StatusCode,
		DurationMs:            durationMs,
		RequestHeaders:        reqHeaders,
		ResponseHeaders:       respHeaders,
		RequestBody:           truncatedReq,
		ResponseBody:          truncatedResp,
		RequestBodyTruncated:  reqTruncated,
		ResponseBodyTruncated: respTruncated,
		RequestBodySize:       len(result.RequestBody),
		ResponseBodySize:      len(result.ResponseBody),
		IsStreaming:           strings.Contains(strings.ToLower(respHeaders["Content-Type"]), "text/event-stream"),
		Model:                 result.Model,
		Tags:                  append([]string{}, ep.Tags...),
		FinalRequestURL:       finalURL,
		FinalRequestHeaders:   cloneStringMap(reqHeaders),
		FinalRequestBody:      truncatedReq,
		FinalResponseHeaders:  cloneStringMap(respHeaders),
		FinalResponseBody:     truncatedResp,
		RequestFormat:         ep.EndpointType,
		TargetFormat:          ep.TargetFormat,
		FormatConverted:       ep.TargetFormat != "" && ep.TargetFormat != ep.EndpointType,
		ClientType:            ep.ClientType,
		EndpointHealthStatus:  healthStatus,
		EndpointResponseTime:  durationMs,
	}

	if checkErr != nil {
		logEntry.Error = checkErr.Error()
	}

	a.requestLogger.LogRequest(logEntry)
	return requestID, logEntry
}

func truncateStringForLog(value string, limit int) (string, bool) {
	if limit <= 0 || len(value) <= limit {
		return value, false
	}
	if limit > 3 {
		return value[:limit-3] + "...", true
	}
	return value[:limit], true
}

func truncateForResponse(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	text := string(body)
	if len(text) <= healthResponsePreviewLimit {
		return text
	}
	if healthResponsePreviewLimit > 3 {
		return text[:healthResponsePreviewLimit-3] + "..."
	}
	return text[:healthResponsePreviewLimit]
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func defaultTimeoutConfig() config.TimeoutConfig {
	return config.TimeoutConfig{
		TLSHandshake:       config.Default.Timeouts.TLSHandshake,
		ResponseHeader:     config.Default.Timeouts.ResponseHeader,
		IdleConnection:     config.Default.Timeouts.IdleConnection,
		HealthCheckTimeout: config.Default.Timeouts.HealthCheckTimeout,
		CheckInterval:      config.Default.Timeouts.CheckInterval,
		RecoveryThreshold:  config.Default.Timeouts.RecoveryThreshold,
	}
}

func normalizeAuthType(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "none"
	}
	return trimmed
}

func buildModelRewriteConfigFromRow(enabled sql.NullBool, target, rules sql.NullString) (*config.ModelRewriteConfig, error) {
	if !enabled.Valid && !target.Valid && (!rules.Valid || strings.TrimSpace(rules.String) == "") {
		return nil, nil
	}

	cfg := &config.ModelRewriteConfig{
		Enabled: enabled.Valid && enabled.Bool,
	}

	var parsed []config.ModelRewriteRule
	if rules.Valid && strings.TrimSpace(rules.String) != "" {
		if err := json.Unmarshal([]byte(rules.String), &parsed); err != nil {
			return nil, fmt.Errorf("解析模型重写规则失败: %w", err)
		}
	}

	if len(parsed) == 0 && target.Valid {
		if trimmed := strings.TrimSpace(target.String); trimmed != "" {
			parsed = []config.ModelRewriteRule{{SourcePattern: "*", TargetModel: trimmed}}
		}
	}

	if len(parsed) > 0 {
		cfg.Rules = parsed
	}

	return cfg, nil
}

// ----- 时间格式化函数 -----
// formatTimestamp 统一的时间格式化函数，使用本地时区
func formatTimestamp(t time.Time) string {
	return t.Format("2006/01/02 15:04:05")
}

// getCurrentTimestamp 获取当前时间戳（本地时区）
func getCurrentTimestamp() string {
	return formatTimestamp(time.Now())
}

// ----- 数据库与序列化辅助函数 -----

type modelRewriteRule struct {
	SourcePattern string `json:"source_pattern"`
	TargetModel   string `json:"target_model"`
}

type modelRewritePayload struct {
	Enabled     bool
	TargetModel string
	RulesJSON   string
}

func defaultModelRewritePayload() modelRewritePayload {
	return modelRewritePayload{
		Enabled:     false,
		TargetModel: "",
		RulesJSON:   "[]",
	}
}

func getStringFromMap(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func deduceEndpointType(urlAnthropic, urlOpenai string) string {
	hasAnthropic := strings.TrimSpace(urlAnthropic) != ""
	hasOpenAI := strings.TrimSpace(urlOpenai) != ""

	switch {
	case hasAnthropic && hasOpenAI:
		return "universal"
	case hasAnthropic:
		return "anthropic"
	case hasOpenAI:
		return "openai"
	default:
		return "unknown"
	}
}

func extractBool(raw interface{}, defaultValue bool) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case float32:
		return v != 0
	case int:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return defaultValue
		}
		if parsed, err := strconv.ParseBool(trimmed); err == nil {
			return parsed
		}
		if parsedInt, err := strconv.Atoi(trimmed); err == nil {
			return parsedInt != 0
		}
	}
	return defaultValue
}

func extractPriority(raw interface{}) int {
	priority := 1

	switch v := raw.(type) {
	case float64:
		priority = int(v)
	case float32:
		priority = int(v)
	case int:
		priority = v
	case int32:
		priority = int(v)
	case int64:
		priority = int(v)
	case string:
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			if parsed, err := strconv.Atoi(trimmed); err == nil {
				priority = parsed
			}
		}
	}

	if priority <= 0 {
		priority = 1
	}

	return priority
}

func parseStringSlice(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case []string:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result, nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("tag value %v is not a string", item)
			}
			if trimmed := strings.TrimSpace(str); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return []string{}, nil
		}
		parts := strings.Split(trimmed, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if value := strings.TrimSpace(part); value != "" {
				result = append(result, value)
			}
		}
		return result, nil
	case nil:
		return []string{}, nil
	default:
		return nil, fmt.Errorf("unsupported tag type %T", raw)
	}
}

func serialiseStringSlice(raw interface{}, emptyFallback string) (string, error) {
	tags, err := parseStringSlice(raw)
	if err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return emptyFallback, nil
	}
	payload, err := json.Marshal(tags)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeStringSlice(value sql.NullString) []string {
	if !value.Valid {
		return []string{}
	}
	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil {
		return tags
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseStringMap(raw interface{}) (map[string]string, error) {
	result := map[string]string{}

	switch v := raw.(type) {
	case map[string]string:
		for key, value := range v {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			result[trimmedKey] = strings.TrimSpace(value)
		}
	case map[string]interface{}:
		for key, value := range v {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			switch cast := value.(type) {
			case string:
				result[trimmedKey] = strings.TrimSpace(cast)
			case fmt.Stringer:
				result[trimmedKey] = strings.TrimSpace(cast.String())
			case nil:
				result[trimmedKey] = ""
			default:
				return nil, fmt.Errorf("parameter value for %s must be string", key)
			}
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return result, nil
		}
		if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
			return nil, err
		}
	case nil:
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported parameter type %T", raw)
	}

	return result, nil
}

func serialiseStringMap(raw interface{}, emptyFallback string) (string, error) {
	mapping, err := parseStringMap(raw)
	if err != nil {
		return "", err
	}
	if len(mapping) == 0 {
		return emptyFallback, nil
	}
	payload, err := json.Marshal(mapping)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeStringMap(value sql.NullString) map[string]string {
	result := map[string]string{}
	if !value.Valid {
		return result
	}
	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return result
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return map[string]string{}
	}
	cleaned := make(map[string]string, len(result))
	for key, val := range result {
		if trimmedKey := strings.TrimSpace(key); trimmedKey != "" {
			cleaned[trimmedKey] = strings.TrimSpace(val)
		}
	}
	return cleaned
}

func parseModelRewriteRules(raw interface{}) ([]modelRewriteRule, error) {
	switch v := raw.(type) {
	case []interface{}:
		rules := make([]modelRewriteRule, 0, len(v))
		for _, item := range v {
			rule, err := modelRewriteRuleFromInterface(item)
			if err != nil {
				return nil, err
			}
			rules = append(rules, rule)
		}
		return rules, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return []modelRewriteRule{}, nil
		}
		var rules []modelRewriteRule
		if err := json.Unmarshal([]byte(trimmed), &rules); err == nil {
			return rules, nil
		}
		parts := strings.Split(trimmed, "->")
		if len(parts) == 2 {
			return []modelRewriteRule{{
				SourcePattern: strings.TrimSpace(parts[0]),
				TargetModel:   strings.TrimSpace(parts[1]),
			}}, nil
		}
		return nil, fmt.Errorf("无法解析模型重写规则: %s", trimmed)
	case nil:
		return []modelRewriteRule{}, nil
	default:
		return nil, fmt.Errorf("unsupported model_rewrite rules type %T", raw)
	}
}

func modelRewriteRuleFromInterface(raw interface{}) (modelRewriteRule, error) {
	switch v := raw.(type) {
	case map[string]interface{}:
		rule := modelRewriteRule{}
		if src, ok := v["source_pattern"].(string); ok {
			rule.SourcePattern = strings.TrimSpace(src)
		}
		if tgt, ok := v["target_model"].(string); ok {
			rule.TargetModel = strings.TrimSpace(tgt)
		}
		if rule.TargetModel == "" {
			return rule, fmt.Errorf("model_rewrite 规则缺少 target_model")
		}
		if rule.SourcePattern == "" {
			rule.SourcePattern = "*"
		}
		return rule, nil
	case map[string]string:
		rule := modelRewriteRule{}
		if src, ok := v["source_pattern"]; ok {
			rule.SourcePattern = strings.TrimSpace(src)
		}
		if tgt, ok := v["target_model"]; ok {
			rule.TargetModel = strings.TrimSpace(tgt)
		}
		if rule.TargetModel == "" {
			return rule, fmt.Errorf("model_rewrite 规则缺少 target_model")
		}
		if rule.SourcePattern == "" {
			rule.SourcePattern = "*"
		}
		return rule, nil
	default:
		return modelRewriteRule{}, fmt.Errorf("unsupported model_rewrite rule type %T", raw)
	}
}

func extractModelRewritePayload(raw interface{}) (modelRewritePayload, error) {
	payload := defaultModelRewritePayload()

	if raw == nil {
		return payload, nil
	}

	switch typed := raw.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return payload, nil
		}
		temp := map[string]interface{}{}
		if err := json.Unmarshal([]byte(typed), &temp); err != nil {
			return payload, err
		}
		raw = temp
	}

	mrMap, ok := raw.(map[string]interface{})
	if !ok {
		return payload, fmt.Errorf("model_rewrite 必须是对象")
	}

	if enabledRaw, exists := mrMap["enabled"]; exists {
		payload.Enabled = extractBool(enabledRaw, false)
	}

	if targetRaw, exists := mrMap["target_model"]; exists {
		if str, ok := targetRaw.(string); ok {
			payload.TargetModel = strings.TrimSpace(str)
		}
	}

	if rulesRaw, exists := mrMap["rules"]; exists {
		rules, err := parseModelRewriteRules(rulesRaw)
		if err != nil {
			return payload, err
		}
		if len(rules) > 0 {
			bytes, err := json.Marshal(rules)
			if err != nil {
				return payload, err
			}
			payload.RulesJSON = string(bytes)
			if payload.TargetModel == "" {
				payload.TargetModel = strings.TrimSpace(rules[0].TargetModel)
			}
		}
	}

	if payload.RulesJSON == "" {
		payload.RulesJSON = "[]"
	}

	if payload.RulesJSON == "[]" && payload.TargetModel != "" && payload.Enabled {
		rules := []modelRewriteRule{{SourcePattern: "*", TargetModel: payload.TargetModel}}
		if bytes, err := json.Marshal(rules); err == nil {
			payload.RulesJSON = string(bytes)
		}
	}

	return payload, nil
}

func buildModelRewriteMap(enabled sql.NullBool, target sql.NullString, rules sql.NullString) map[string]interface{} {
	rewriteEnabled := enabled.Valid && enabled.Bool
	trimmedTarget := ""
	if target.Valid {
		trimmedTarget = strings.TrimSpace(target.String)
	}

	var parsedRules []modelRewriteRule
	if rules.Valid && strings.TrimSpace(rules.String) != "" {
		if err := json.Unmarshal([]byte(rules.String), &parsedRules); err != nil {
			parsedRules = nil
		}
	}

	if !rewriteEnabled && trimmedTarget == "" && len(parsedRules) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"enabled": rewriteEnabled,
	}
	if trimmedTarget != "" {
		payload["target_model"] = trimmedTarget
	}
	if len(parsedRules) > 0 {
		ruleList := make([]map[string]string, 0, len(parsedRules))
		for _, rule := range parsedRules {
			ruleList = append(ruleList, map[string]string{
				"source_pattern": rule.SourcePattern,
				"target_model":   rule.TargetModel,
			})
		}
		payload["rules"] = ruleList
	}
	return payload
}

// formatTimestamp 格式化时间戳为统一的 "YYYY/MM/DD HH:MM:SS" 格式
func (a *App) formatTimestamp(t time.Time) string {
	return t.Format("2006/01/02 15:04:05")
}

// GetBindingInfo - 使用Wails自动生成的代码

// ClearLogs 清除旧日志
func (a *App) ClearLogs(daysToKeep interface{}) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.db == nil {
		return map[string]interface{}{
			"success": false,
			"message": "数据库不可用",
		}
	}

	// 解析参数
	days := 7 // 默认保留7天
	if daysToKeep != nil {
		switch v := daysToKeep.(type) {
		case int:
			days = v
		case float64:
			days = int(v)
		case string:
			if d, err := strconv.Atoi(v); err == nil {
				days = d
			}
		}
	}

	if days < 0 {
		days = 0
	}

	// 计算截止日期
	cutoffDate := time.Now().AddDate(0, 0, -days)

	// 清除内存日志
	newLogs := make([]LogEntry, 0)
	for _, log := range a.logs {
		if logTime, err := time.Parse("2006-01-02 15:04:05", log.Timestamp); err == nil {
			if logTime.After(cutoffDate) {
				newLogs = append(newLogs, log)
			}
		}
	}
	a.logs = newLogs

	// 清除数据库日志
	result, err := a.db.Exec(`
		DELETE FROM request_logs
		WHERE timestamp < ?
	`, cutoffDate.Format("2006-01-02 15:04:05"))

	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("清除日志失败: %v", err),
		}
	}

	rowsAffected, _ := result.RowsAffected()
	a.addLog("info", fmt.Sprintf("已清除 %d 条超过 %d 天的日志记录", rowsAffected, days))

	return map[string]interface{}{
		"success":       true,
		"message":       fmt.Sprintf("日志清理完成，已清除超过 %d 天的记录", days),
		"rows_affected": rowsAffected,
	}
}

// ExportData 导出数据
func (a *App) ExportData(format string) map[string]interface{} {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if a.db == nil {
		return map[string]interface{}{
			"success": false,
			"message": "数据库不可用",
		}
	}

	switch strings.ToLower(format) {
	case "json":
		return a.exportToJSON()
	case "csv":
		return a.exportToCSV()
	default:
		return map[string]interface{}{
			"success": false,
			"message": "不支持的导出格式: " + format + " (支持: json, csv)",
		}
	}
}

// exportToJSON 导出为JSON格式
func (a *App) exportToJSON() map[string]interface{} {
	// 导出端点数据
	endpoints := a.GetEndpoints()
	endpointData, ok := endpoints["data"]
	if !ok {
		endpointData = []interface{}{}
	}

	// 导出日志数据（最近1000条）
	logs := a.GetLogs(map[string]interface{}{
		"page":  "1",
		"limit": "1000",
	})

	exportData := map[string]interface{}{
		"version":     "1.0",
		"export_time": time.Now().Format("2006-01-02 15:04:05"),
		"endpoints":   endpointData,
		"logs":        logs["logs"],
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("JSON序列化失败: %v", err),
		}
	}

	return map[string]interface{}{
		"success": true,
		"message": "数据导出成功 (JSON格式)",
		"data":    string(jsonData),
		"format":  "json",
	}
}

// exportToCSV 导出为CSV格式
func (a *App) exportToCSV() map[string]interface{} {
	var csvData strings.Builder

	// CSV写入器
	writer := csv.NewWriter(&csvData)

	// 写入端点数据
	writer.Write([]string{"Type", "ID", "Name", "Anthropic URL", "OpenAI URL", "Auth Type", "Enabled", "Priority", "Status"})
	endpoints := a.GetEndpoints()
	if endpointList, ok := endpoints["data"].([]interface{}); ok {
		for _, ep := range endpointList {
			if epMap, ok := ep.(map[string]interface{}); ok {
				writer.Write([]string{
					"endpoint",
					getStringValue(epMap["id"]),
					getStringValue(epMap["name"]),
					getStringValue(epMap["url_anthropic"]),
					getStringValue(epMap["url_openai"]),
					getStringValue(epMap["auth_type"]),
					fmt.Sprintf("%v", epMap["enabled"]),
					fmt.Sprintf("%v", epMap["priority"]),
					getStringValue(epMap["status"]),
				})
			}
		}
	}

	// 写入日志数据
	writer.Write([]string{"Type", "Timestamp", "Level", "Message", "Endpoint ID", "Response Time"})
	logs := a.GetLogs(map[string]interface{}{
		"page":  "1",
		"limit": "1000",
	})
	if logList, ok := logs["logs"].([]interface{}); ok {
		for _, log := range logList {
			if logMap, ok := log.(map[string]interface{}); ok {
				writer.Write([]string{
					"log",
					getStringValue(logMap["timestamp"]),
					getStringValue(logMap["level"]),
					getStringValue(logMap["message"]),
					getStringValue(logMap["endpointId"]),
					fmt.Sprintf("%v", logMap["responseTime"]),
				})
			}
		}
	}

	writer.Flush()

	return map[string]interface{}{
		"success": true,
		"message": "数据导出成功 (CSV格式)",
		"data":    csvData.String(),
		"format":  "csv",
	}
}

// ImportData 导入数据
func (a *App) ImportData(data string) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.db == nil {
		return map[string]interface{}{
			"success": false,
			"message": "数据库不可用",
		}
	}

	if strings.TrimSpace(data) == "" {
		return map[string]interface{}{
			"success": false,
			"message": "导入数据不能为空",
		}
	}

	var importData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &importData); err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("数据格式错误，不是有效的JSON: %v", err),
		}
	}

	version, _ := importData["version"].(string)
	if version == "" {
		version = "unknown"
	}

	a.addLog("info", fmt.Sprintf("开始导入数据，版本: %s", version))

	// 导入端点数据
	endpointsImported := 0
	if endpoints, ok := importData["endpoints"].([]interface{}); ok {
		for _, ep := range endpoints {
			if epMap, ok := ep.(map[string]interface{}); ok {
				// 清理导入的数据，移除ID和时间戳，让系统重新生成
				delete(epMap, "id")
				delete(epMap, "created_at")
				delete(epMap, "updated_at")

				result := a.CreateEndpoint(epMap)
				if success, ok := result["success"].(bool); ok && success {
					endpointsImported++
				}
			}
		}
	}

	a.addLog("info", fmt.Sprintf("数据导入完成，导入端点数量: %d", endpointsImported))

	return map[string]interface{}{
		"success":            true,
		"message":            fmt.Sprintf("数据导入成功，导入 %d 个端点", endpointsImported),
		"endpoints_imported": endpointsImported,
		"version":            version,
	}
}

// 辅助函数：获取字符串值
func getStringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// maskToken returns a masked version of the provided token for safe logging.
func maskToken(token string) string {
	token = strings.TrimSpace(token)
	length := len(token)
	if length == 0 {
		return ""
	}
	if length <= 4 {
		return strings.Repeat("*", length)
	}
	if length <= 8 {
		return token[:2] + strings.Repeat("*", length-4) + token[length-2:]
	}
	return token[:4] + strings.Repeat("*", length-8) + token[length-4:]
}

// extractClientToken retrieves the client-provided token from common headers.
func (a *App) extractClientToken(req *http.Request) string {
	if req == nil {
		return ""
	}

	authHeader := strings.TrimSpace(req.Header.Get("Authorization"))
	if authHeader != "" {
		if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
		return authHeader
	}

	if apiKey := strings.TrimSpace(req.Header.Get("x-api-key")); apiKey != "" {
		return apiKey
	}

	return ""
}

// GetClaudeCodeAuthToken 获取Claude Code认证token (Wails绑定)
func (a *App) GetClaudeCodeAuthToken() string {
	return a.getClaudeCodeAuthToken()
}

// SetClaudeCodeAuthToken 设置Claude Code认证token (Wails绑定)
func (a *App) SetClaudeCodeAuthToken(token string) map[string]interface{} {
	err := a.setClaudeCodeAuthToken(token)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
		"message": "Claude Code认证token已更新",
		"token":   token,
	}
}

// GetTokenMappings 获取Token映射配置 (Wails绑定)
func (a *App) GetTokenMappings() []TokenMapping {
	return a.getTokenMappings()
}

// SetTokenMappings 设置Token映射配置 (Wails绑定)
func (a *App) SetTokenMappings(mappings []TokenMapping) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 确保config存在
	if a.config == nil {
		a.config = make(map[string]interface{})
	}

	// 确保server配置存在
	if _, ok := a.config["server"]; !ok {
		a.config["server"] = make(map[string]interface{})
	}

	serverConfig := a.config["server"].(map[string]interface{})

	// 转换mappings为interface{}格式
	var mappingsData []interface{}
	for _, mapping := range mappings {
		mappingData := map[string]interface{}{
			"input_token":  mapping.InputToken,
			"output_token": mapping.OutputToken,
			"endpoint_id":  mapping.EndpointID,
			"description":  mapping.Description,
		}
		mappingsData = append(mappingsData, mappingData)
	}

	serverConfig["token_mappings"] = mappingsData

	// 保存配置到文件
	configPath := filepath.Join(os.Getenv("HOME"), ".cccc-proxy", "config.json")
	configData, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Token映射配置已更新，共 %d 条映射", len(mappings)))

	return map[string]interface{}{
		"success": true,
		"message": "Token映射配置已更新",
		"count":   len(mappings),
	}
}

// GetArbitraryTokenModeEnabled 获取任意Token模式状态 (Wails绑定)
func (a *App) GetArbitraryTokenModeEnabled() bool {
	return a.isArbitraryTokenModeEnabled()
}

// SetArbitraryTokenModeEnabled 设置任意Token模式状态 (Wails绑定)
func (a *App) SetArbitraryTokenModeEnabled(enabled bool) map[string]interface{} {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 确保config存在
	if a.config == nil {
		a.config = make(map[string]interface{})
	}

	// 确保server配置存在
	if _, ok := a.config["server"]; !ok {
		a.config["server"] = make(map[string]interface{})
	}

	serverConfig := a.config["server"].(map[string]interface{})
	serverConfig["arbitrary_token_mode"] = enabled

	// 保存配置到文件
	configPath := filepath.Join(os.Getenv("HOME"), ".cccc-proxy", "config.json")
	configData, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	mode := "禁用"
	if enabled {
		mode = "启用"
	}
	runtime.LogInfo(a.ctx, fmt.Sprintf("任意Token模式已%s", mode))

	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("任意Token模式已%s", mode),
		"enabled": enabled,
	}
}
