package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"
)

// ProcessBindingInfo 进程绑定信息
type ProcessBindingInfo struct {
	PID           int    `json:"pid"`
	Port          int    `json:"port"`
	StartTime     string `json:"start_time"`
	LastActive     string `json:"last_active"`
	Status        string `json:"status"`
	IsPrimary     bool   `json:"is_primary"`
	DatabasePath  string `json:"database_path"`
	AppInstance   string `json:"app_instance"`
}

// BindingManager 绑定管理器
type BindingManager struct {
	ctx        context.Context
	mutex      sync.RWMutex
	bindingFile string
	info       ProcessBindingInfo
	app        *App // 添加应用引用用于日志记录
}

// NewBindingManager 创建绑定管理器
func NewBindingManager(ctx context.Context, app *App) *BindingManager {
	return &BindingManager{
		ctx:        ctx,
		mutex:      sync.RWMutex{},
		bindingFile: "./.binding-info.json",
		info: ProcessBindingInfo{
			Status:      "initializing",
			IsPrimary:   true,
		},
		app: app, // 设置应用引用
	}
}

// InitializeBinding 初始化绑定
func (bm *BindingManager) InitializeBinding() error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// 获取进程ID
	pid := os.Getpid()

	// 生成唯一的应用实例ID
	appInstance := fmt.Sprintf("cccc-proxy-%d-%s", pid, time.Now().Format("20060102-150405"))

	// 设置绑定信息
	bm.info = ProcessBindingInfo{
		PID:          pid,
		Port:        34115, // Wails默认端口
	StartTime:    time.Now().Format("2006-01-02 15:04:05"),
	LastActive:   time.Now().Format("2006-01-02 15:04:05"),
		Status:       "active",
		IsPrimary:    true,
		DatabasePath: "./cccc-proxy.db",
		AppInstance:  appInstance,
	}

	// 检查是否有其他实例在运行
	bm.checkAndCleanupOtherInstances()

	// 保存绑定信息
	if err := bm.saveBindingInfo(); err != nil {
		return fmt.Errorf("failed to save binding info: %w", err)
	}

	// 启动HTTP服务器提供绑定信息
	go bm.startBindingServer()

	// 定期更新活动时间
	go bm.updateHeartbeat()

	runtime.LogInfo(bm.ctx, fmt.Sprintf("✅ Process binding initialized - PID: %d, Instance: %s", pid, appInstance))
	if bm.app != nil {
		bm.app.addLog("info", fmt.Sprintf("进程绑定初始化完成 - PID: %d, 实例: %s", pid, appInstance))
	}

	return nil
}

// checkAndCleanupOtherInstances 检查并清理其他实例
func (bm *BindingManager) checkAndCleanupOtherInstances() {
	// 这里可以实现检查其他实例的逻辑
	// 例如：检查锁定文件、端口占用等

	// 简单的端口检查
	if bm.isPortTaken(34115) {
		runtime.LogWarning(bm.ctx, "Port 34115 is already in use by another process")
		bm.info.Status = "port_conflict"
		bm.info.IsPrimary = false
	} else {
		bm.info.Status = "primary"
		bm.info.IsPrimary = true
	}
}

// isPortTaken 检查端口是否被占用
func (bm *BindingManager) isPortTaken(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// saveBindingInfo 保存绑定信息到文件
func (bm *BindingManager) saveBindingInfo() error {
	data, err := json.MarshalIndent(bm.info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(bm.bindingFile, data, 0644)
}

// loadBindingInfo 从文件加载绑定信息
func (bm *BindingManager) loadBindingInfo() error {
	data, err := os.ReadFile(bm.bindingFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}
	return json.Unmarshal(data, &bm.info)
}

// startBindingServer 启动绑定信息HTTP服务器
func (bm *BindingManager) startBindingServer() {
	http.HandleFunc("/binding-info", bm.handleBindingInfo)
	http.HandleFunc("/health", bm.handleHealth)

	// 尝试在备用端口启动
	server := &http.Server{
		Addr:    ":8081", // 使用8081端口提供绑定信息
	Handler: nil,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			runtime.LogError(bm.ctx, fmt.Sprintf("Binding server error: %v", err))
		}
	}()

	runtime.LogInfo(bm.ctx, "✅ Binding server started on port 8081")
	if bm.app != nil {
		bm.app.addLog("info", "绑定信息服务器启动 - 端口: 8081")
	}
}

// handleBindingInfo 处理绑定信息请求
func (bm *BindingManager) handleBindingInfo(w http.ResponseWriter, r *http.Request) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"binding": bm.info,
		"timestamp": time.Now().Format("2006-01-02T15:04:05Z"),
	}

	json.NewEncoder(w).Encode(response)
}

// handleHealth 处理健康检查
func (bm *BindingManager) handleHealth(w http.ResponseWriter, r *http.Request) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 计算运行时间
	startTime, _ := time.Parse("2006-01-02 15:04:05", bm.info.StartTime)
	uptimeSeconds := time.Since(startTime).Seconds()

	health := map[string]interface{}{
		"status": bm.info.Status,
		"is_primary": bm.info.IsPrimary,
		"pid": bm.info.PID,
		"port": bm.info.Port,
		"uptime_seconds": uptimeSeconds,
	"timestamp": time.Now().Format("2006-01-02T15:04:05Z"),
	}

	json.NewEncoder(w).Encode(health)
}

// updateHeartbeat 更新心跳
func (bm *BindingManager) updateHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		bm.mutex.Lock()
		bm.info.LastActive = time.Now().Format("2006-01-02 15:04:05")
		bm.saveBindingInfo()
		bm.mutex.Unlock()
	}
}

// GetBindingInfo 获取绑定信息
func (bm *BindingManager) GetBindingInfo() ProcessBindingInfo {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	return bm.info
}


// 在app.go中添加绑定管理器
var bindingManager *BindingManager

// InitializeBindingManager 初始化绑定管理器
func (a *App) InitializeBindingManager() error {
	bindingManager = NewBindingManager(a.ctx, a)
	return bindingManager.InitializeBinding()
}

// GetBindingInfo 获取绑定信息
func (a *App) GetBindingInfo() ProcessBindingInfo {
	if bindingManager == nil {
		return ProcessBindingInfo{
			Status: "not_initialized",
		}
	}
	return bindingManager.GetBindingInfo()
}