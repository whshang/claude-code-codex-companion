// Wails 统一架构 API - 直接使用Go方法，无HTTP服务器
import type {
  DesktopAPI,
  ServerStatus,
  Endpoint,
  CreateEndpointParams,
  UpdateEndpointParams,
  OperationResult,
  EndpointTestResult,
  BatchTestResult,
  LogQueryParams,
  LogQueryResponse,
  SystemStats,
  RequestTrends,
  EndpointStats,
  SystemInfo,
  ConfigData,
  OpenURLResult,
  ProcessBindingInfo,
  APIResult,
} from '../types/api'

// 导入Wails自动生成的绑定
import * as AppBindings from '../../wailsjs/go/main/App'

// 重新导出类型以保持向后兼容
export type {
  DesktopAPI,
  ServerStatus,
  Endpoint,
  CreateEndpointParams,
  UpdateEndpointParams,
  OperationResult,
  EndpointTestResult,
  BatchTestResult,
  LogQueryParams,
  LogQueryResponse,
  SystemStats,
  RequestTrends,
  EndpointStats,
  SystemInfo,
  ConfigData,
  OpenURLResult,
  ProcessBindingInfo,
  APIResult,
}


// 检查Wails API是否可用的辅助函数
function checkWailsAPI(): void {
  if (!window.go?.main?.App) {
    throw new Error('Wails API not available - 请在桌面应用中使用')
  }
}

// 等待Wails API初始化的辅助函数
// 增加重试次数和超时时间，确保刷新后也能正常工作
async function waitForWailsAPI(maxRetries: number = 30, delay: number = 200): Promise<void> {
  for (let i = 0; i < maxRetries; i++) {
    if (window.go?.main?.App) {
      console.log(`Wails API initialized after ${i} retries (${i * delay}ms)`)
      return
    }
    await new Promise(resolve => setTimeout(resolve, delay))
  }
  throw new Error('Wails API not available - 请在桌面应用中使用')
}

// 强类型 Wails API 客户端实现
class WailsAPI implements DesktopAPI {
  // 服务器管理
  async GetServerStatus(): Promise<ServerStatus> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetServerStatus()
  }

  async RestartServer(): Promise<string> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.RestartServer()
  }

  // 配置管理
  async GetConfigPath(): Promise<string> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetConfigPath()
  }

  async OpenConfigDirectory(): Promise<void> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.OpenConfigDirectory()
  }

  async OpenURL(url: string): Promise<OpenURLResult> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.OpenURL(url)
  }

  async Greet(name: string): Promise<string> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.Greet(name)
  }

  // 端点管理 - 统一架构，通过Go API
  async GetEndpoints(): Promise<Endpoint[]> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetEndpoints()
  }

  async CreateEndpoint(endpointData: CreateEndpointParams): Promise<OperationResult> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.CreateEndpoint(endpointData)
  }

  async UpdateEndpoint(id: string, endpointData: UpdateEndpointParams): Promise<OperationResult> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.UpdateEndpoint(id, endpointData)
  }

  async DeleteEndpoint(id: string): Promise<OperationResult> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.DeleteEndpoint(id)
  }

  async TestEndpoint(id: string): Promise<EndpointTestResult> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.TestEndpoint(id)
  }

  async TestAllEndpoints(): Promise<BatchTestResult> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.TestAllEndpoints()
  }

  // 日志管理 - 通过Go API
  async GetLogs(params: LogQueryParams): Promise<LogQueryResponse> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetLogs(params)
  }

  // 统计信息 - 通过Go API
  async GetStats(): Promise<any> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetStats()
  }

  // 配置管理 - 通过Go API
  async LoadConfig(): Promise<any> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.LoadConfig()
  }

  async SaveConfig(config: any): Promise<OperationResult> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.SaveConfig(config)
  }

  // 监控和统计 - 通过Go API
  async GetRequestTrends(timeRange: string): Promise<any> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetRequestTrends(timeRange)
  }

  async GetSystemInfo(): Promise<any> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetSystemInfo()
  }

  async GetEndpointStats(): Promise<any> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetEndpointStats()
  }

  // 进程绑定 - 通过Go API
  async GetBindingInfo(): Promise<ProcessBindingInfo> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetBindingInfo()
  }

  // Claude Code认证管理 - 通过Go API
  async GetClaudeCodeAuthToken(): Promise<string> {
    await ensureWailsAPIReady()
    return AppBindings.GetClaudeCodeAuthToken()
  }

  async SetClaudeCodeAuthToken(token: string): Promise<any> {
    await ensureWailsAPIReady()
    return AppBindings.SetClaudeCodeAuthToken(token)
  }

  // Token映射管理 - 通过Go API
  async GetTokenMappings(): Promise<any[]> {
    await ensureWailsAPIReady()
    return AppBindings.GetTokenMappings()
  }

  async SetTokenMappings(mappings: any[]): Promise<any> {
    await ensureWailsAPIReady()
    return AppBindings.SetTokenMappings(mappings)
  }

  // 任意Token模式管理 - 通过Go API
  async GetArbitraryTokenModeEnabled(): Promise<boolean> {
    await ensureWailsAPIReady()
    return AppBindings.GetArbitraryTokenModeEnabled()
  }

  async SetArbitraryTokenModeEnabled(enabled: boolean): Promise<any> {
    await ensureWailsAPIReady()
    return AppBindings.SetArbitraryTokenModeEnabled(enabled)
  }

  // 通用功能 - 通过Go API
  async GetVersionInfo(): Promise<string> {
    await ensureWailsAPIReady()
    checkWailsAPI()
    return window.go!.main.App.GetVersionInfo()
  }

  // 兼容性方法 - 保持向后兼容
  async createEndpointCompat(endpointData: any): Promise<any> {
    return this.CreateEndpoint(endpointData as CreateEndpointParams)
  }

  async updateEndpointCompat(id: string, endpointData: any): Promise<any> {
    return this.UpdateEndpoint(id, endpointData as UpdateEndpointParams)
  }

  async getLogsCompat(params: any): Promise<any> {
    return this.GetLogs(params as LogQueryParams)
  }

  async loadConfigCompat(): Promise<any> {
    return this.LoadConfig()
  }

  async saveConfigCompat(config: any): Promise<any> {
    return this.SaveConfig(config)
  }

  async getStatsCompat(): Promise<any> {
    return this.GetStats()
  }

  async getRequestTrendsCompat(timeRange: string): Promise<any> {
    return this.GetRequestTrends(timeRange)
  }

  async getSystemInfoCompat(): Promise<any> {
    return this.GetSystemInfo()
  }

  async getEndpointStatsCompat(): Promise<any> {
    return this.GetEndpointStats()
  }

  async openURLCompat(url: string): Promise<any> {
    return this.OpenURL(url)
  }
}

// 导出单例实例 - 统一架构
export const wailsAPI = new WailsAPI()

// 检查是否在Wails环境中运行
export function isWailsEnvironment(): boolean {
  return typeof window !== 'undefined' && window.go !== undefined
}

// 全局 API 就绪 Promise - 确保应用启动时 API 已初始化
let apiReadyPromise: Promise<void> | null = null

export function ensureWailsAPIReady(): Promise<void> {
  if (!apiReadyPromise) {
    apiReadyPromise = waitForWailsAPI(50, 200) // 最多等待 10 秒
      .then(() => {
        console.log('✅ Wails API is ready')
      })
      .catch((error) => {
        console.error('❌ Wails API initialization failed:', error)
        throw error
      })
  }
  return apiReadyPromise
}

// 统一架构：只在Wails桌面应用中使用，不需要Web环境兼容
if (!isWailsEnvironment()) {
  console.warn('CCCC Proxy - 此应用只能在Wails桌面环境中运行')
} else {
  // 应用启动时预加载 API
  ensureWailsAPIReady()
}
