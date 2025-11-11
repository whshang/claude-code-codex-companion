package httpclient

import (
	"net/http"
	"sync"
)

// Manager 全局HTTP客户端管理器
type Manager struct {
	factory     *Factory
	proxyClient *http.Client
	healthClient *http.Client
	mutex       sync.RWMutex
}

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// GetManager 获取全局客户端管理器
func GetManager() *Manager {
	managerOnce.Do(func() {
		globalManager = &Manager{
			factory: NewFactory(),
		}
	})
	return globalManager
}

// InitClients 初始化全局客户端
func (m *Manager) InitClients(proxyTimeouts, healthTimeouts TimeoutConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.proxyClient = m.factory.CreateProxyClient(proxyTimeouts)
	m.healthClient = m.factory.CreateHealthClient(healthTimeouts)
}

// GetProxyClient 获取代理客户端
func (m *Manager) GetProxyClient() *http.Client {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.proxyClient == nil {
		// 使用默认配置初始化
		m.mutex.RUnlock()
		m.InitClients(
			m.factory.defaultConfigs[ClientTypeProxy].Timeouts,
			m.factory.defaultConfigs[ClientTypeHealth].Timeouts,
		)
		m.mutex.RLock()
	}
	return m.proxyClient
}

// GetHealthClient 获取健康检查客户端
func (m *Manager) GetHealthClient() *http.Client {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.healthClient == nil {
		// 使用默认配置初始化
		m.mutex.RUnlock()
		m.InitClients(
			m.factory.defaultConfigs[ClientTypeProxy].Timeouts,
			m.factory.defaultConfigs[ClientTypeHealth].Timeouts,
		)
		m.mutex.RLock()
	}
	return m.healthClient
}

// CreateEndpointClient 创建端点专用客户端
func (m *Manager) CreateEndpointClient(config ClientConfig) (*http.Client, error) {
	return m.factory.CreateClient(config)
}

// 全局便捷函数

// InitHTTPClients 初始化全局HTTP客户端
func InitHTTPClients(proxyTimeouts, healthTimeouts TimeoutConfig) {
	GetManager().InitClients(proxyTimeouts, healthTimeouts)
}

// GetProxyClient 获取全局代理客户端
func GetProxyClient() *http.Client {
	return GetManager().GetProxyClient()
}

// GetHealthClient 获取全局健康检查客户端
func GetHealthClient() *http.Client {
	return GetManager().GetHealthClient()
}