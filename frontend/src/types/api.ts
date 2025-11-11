// CCCC 桌面应用 API TypeScript 类型定义
// 与 Go 后端数据结构 100% 匹配

// =============================================================================
// 核心数据类型
// =============================================================================

// 端点数据结构
export interface Endpoint {
  id: string
  name: string
  url_anthropic?: string
  url_openai?: string
  endpoint_type: string
  auth_type: string
  auth_value: string
  enabled: boolean
  priority: number
  tags: string[]
  status: string // "healthy" | "unhealthy"
  response_time: number // 毫秒
  last_check?: string // ISO 8601 时间
  created_at: string
  updated_at: string
  model_rewrite?: ModelRewrite
  parameter_overrides?: Record<string, string>
  target_model?: string
  // 学习信息（运行时学习，部分持久化）
  openai_preference?: "auto" | "responses" | "chat_completions" // OpenAI格式偏好（持久化）
  supports_responses?: boolean // 是否支持 /responses API（持久化）
}

// 模型重写配置
export interface ModelRewrite {
  enabled: boolean
  target_model?: string
  rules?: ModelRewriteRule[]
}

export interface ModelRewriteRule {
  source_pattern: string // "claude-*"
  target_model: string   // "glm-4.6"
}

// 日志数据结构
export interface LogEntry {
  timestamp: string // "2025-10-29 15:04:05"
  level: LogLevel // "info" | "error" | "warn" | "debug"
  message: string
  requestId?: string
  clientType?: ClientType
  endpointId?: string
  model?: string
  status?: RequestStatus
  responseTime?: number // 毫秒
  requestSize?: number // 字节
  responseSize?: number // 字节
}

// 日志级别
export type LogLevel = "info" | "error" | "warn" | "debug"

// 客户端类型
export type ClientType = "claude-code" | "codex" | "openai" | "universal"

// 请求状态
export type RequestStatus = "success" | "failure"

// 服务器状态
export interface ServerStatus {
  running: boolean
  host: string
  port: string | number
  endpoints_total?: number
  endpoints_healthy?: number
  mode?: string
  architecture?: string
  http_server?: string
  api_communication?: string
  config_path?: string
  uptime?: string
}

// 统计信息
export interface SystemStats {
  uptime: string
  requests_total: number
  requests_successful: number
  requests_failed: number
  endpoints_total: number
  endpoints_healthy: number
  running: boolean
  last_updated: string
  architecture: string
}

// 请求趋势数据点
export interface TrendDataPoint {
  time: string // ISO 8601
  requests: number
  successes: number
  failures: number
}

// 请求趋势响应
export interface RequestTrends {
  timeRange: string
  data: TrendDataPoint[]
  totalRequests: number
  totalSuccesses: number
  totalFailures: number
  successRate: number
  message: string
}

// 端点统计
export interface EndpointStats {
  name: string
  requests: number
  success_rate: number
  avg_response_time: number
  status: string
  enabled: boolean
  api_type: string
}

// 系统信息
export interface SystemInfo {
  platform: string
  architecture: string
  go_version: string
  wails_version: string
  app_version: string
  uptime: string
  api_communication: string
  http_server: string
  config_path: string
}

// 进程绑定信息
export interface ProcessBindingInfo {
  pid: number
  port: number
  start_time: string
  last_active: string
  status: string
  is_primary: boolean
  database_path: string
  app_instance: string
}

// =============================================================================
// API 响应类型
// =============================================================================

// 标准成功响应
export interface APIResponse<T = any> {
  success: true
  data: T
  message?: string
}

// 操作结果响应
export interface OperationResult {
  success: boolean
  message: string
  id?: string
  endpoint_name?: string
  rows_affected?: number
}

// 错误响应
export interface ErrorResponse {
  success: false
  message: string
  error?: string
}

// 统一 API 响应类型
export type APIResult<T = any> = APIResponse<T> | OperationResult | ErrorResponse

// 端点测试结果
export interface EndpointTestResult {
  success: boolean
  message: string
  response_time: number
  endpoint_id: string
  status: string
  error?: string
}

// 批量测试结果
export interface BatchTestResult {
  results: EndpointTestResult[]
  total: number
  success_count: number
  message: string
}

// 日志查询参数
export interface LogQueryParams {
  page?: string | number
  limit?: string | number
  level?: LogLevel
  endpoint_id?: string
  failed_only?: boolean
  cleanup?: number // 清理N天前的日志
  export?: boolean // 导出CSV格式
}

// 日志查询响应
export interface LogQueryResponse {
  logs: LogEntry[]
  total: number
  page: number
  limit: number
  message: string
  export?: boolean
}

// 配置数据
export interface ConfigData {
  server: ServerConfig
  logging: LoggingConfig
  blacklist: BlacklistConfig
  architecture: string
  endpoints?: Endpoint[]
}

// 服务器配置
export interface ServerConfig {
  host: string
  port: string | number
  auto_sort_endpoints?: boolean
}

// 日志配置
export interface LoggingConfig {
  level: string
}

// 黑名单配置
export interface BlacklistConfig {
  enabled: boolean
}

// =============================================================================
// 端点创建和更新类型
// =============================================================================

// 端点创建参数
export interface CreateEndpointParams {
  name: string
  url_anthropic?: string
  url_openai?: string
  endpoint_type?: string
  auth_type: string
  auth_value: string
  enabled?: boolean
  priority?: number
  tags?: string[]
  model_rewrite?: ModelRewrite
  parameter_overrides?: Record<string, string>
}

// 端点更新参数 (部分更新)
export interface UpdateEndpointParams {
  name?: string
  url_anthropic?: string
  url_openai?: string
  endpoint_type?: string
  auth_type?: string
  auth_value?: string
  enabled?: boolean
  priority?: number
  tags?: string[]
  model_rewrite?: ModelRewrite
  parameter_overrides?: Record<string, string>
}

// URL打开结果
export interface OpenURLResult {
  success: boolean
  message: string
}

// =============================================================================
// Wails API 接口定义
// =============================================================================

// 主 API 接口
export interface DesktopAPI {
  // 服务器管理
  GetServerStatus(): Promise<ServerStatus>
  RestartServer(): Promise<string>

  // 端点管理
  GetEndpoints(): Promise<Endpoint[]>
  CreateEndpoint(endpointData: CreateEndpointParams): Promise<OperationResult>
  UpdateEndpoint(id: string, endpointData: UpdateEndpointParams): Promise<OperationResult>
  DeleteEndpoint(id: string): Promise<OperationResult>
  TestEndpoint(id: string): Promise<EndpointTestResult>
  TestAllEndpoints(): Promise<BatchTestResult>

  // 日志管理
  GetLogs(params: LogQueryParams): Promise<LogQueryResponse>

  // 统计信息
  GetStats(): Promise<any>
  GetRequestTrends(timeRange: string): Promise<any>
  GetEndpointStats(): Promise<any>
  GetSystemInfo(): Promise<any>

  // 配置管理
  LoadConfig(): Promise<any>
  SaveConfig(config: any): Promise<OperationResult>
  GetConfigPath(): Promise<string>

  // 系统功能
  OpenURL(url: string): Promise<OpenURLResult>
  OpenConfigDirectory(): Promise<void>
  Greet(name: string): Promise<string>
  GetVersionInfo(): Promise<string>

  // 进程绑定
  GetBindingInfo(): Promise<ProcessBindingInfo>
}

// =============================================================================
// 全局类型扩展
// =============================================================================

declare global {
  interface Window {
    go?: {
      main: {
        App: {
          // 服务器管理
          GetServerStatus(): Promise<ServerStatus>
          RestartServer(): Promise<string>

          // 端点管理
          GetEndpoints(): Promise<any[]>
          CreateEndpoint(endpointData: any): Promise<any>
          UpdateEndpoint(id: string, endpointData: any): Promise<any>
          DeleteEndpoint(id: string): Promise<any>
          TestEndpoint(id: string): Promise<any>
          TestAllEndpoints(): Promise<any>

          // 日志管理
          GetLogs(params: any): Promise<any>

          // 统计信息
          GetStats(): Promise<any>
          GetRequestTrends(timeRange: string): Promise<any>
          GetEndpointStats(): Promise<any>
          GetSystemInfo(): Promise<any>

          // 配置管理
          LoadConfig(): Promise<any>
          SaveConfig(config: any): Promise<any>
          GetConfigPath(): Promise<string>

          // 系统功能
          OpenURL(url: string): Promise<any>
          OpenConfigDirectory(): Promise<void>
          Greet(name: string): Promise<string>
          GetVersionInfo(): Promise<string>

          // 进程绑定
          GetBindingInfo(): Promise<ProcessBindingInfo>
        }
      }
    }
  }
}

// =============================================================================
// 工具类型
// =============================================================================

// 提取数组元素类型
type ArrayElement<T> = T extends (infer U)[] ? U : never

// API 响应包装器
export type WrapAPIResponse<T> = Promise<{
  success: boolean
  data?: T
  message?: string
  error?: string
}>

// 错误类型
export class CCCCError extends Error {
  public code?: string
  public details?: any

  constructor(message: string, code?: string, details?: any) {
    super(message)
    this.name = 'CCCCError'
    this.code = code
    this.details = details
  }
}

// 时间范围类型
export type TimeRange = "1h" | "24h" | "7d" | "30d"

// 端点类型
export type EndpointType = "anthropic" | "openai" | "universal" | "gemini" | "unknown"

// 认证类型
export type AuthType = "none" | "api_key" | "auth_token" | "oauth" | "auto"

// 排序字段
export type EndpointSortField = "name" | "priority" | "status" | "response_time" | "created_at" | "updated_at"

// 排序方向
export type SortDirection = "asc" | "desc"

// 查询过滤器
export interface EndpointFilter {
  enabled?: boolean
  status?: string
  endpoint_type?: EndpointType
  auth_type?: AuthType
  tags?: string[]
  search?: string // 搜索名称和URL
}

// 分页参数
export interface PaginationParams {
  page: number
  limit: number
}

// 排序参数
export interface SortParams {
  field: EndpointSortField
  direction: SortDirection
}

// 端点查询参数
export interface EndpointQueryParams extends PaginationParams {
  filter?: EndpointFilter
  sort?: SortParams
}

// =============================================================================
// 常量定义
// =============================================================================

// 默认值
export const DEFAULTS = {
  PAGE_SIZE: 20,
  MAX_PAGE_SIZE: 100,
  LOG_RETENTION_DAYS: 30,
  ENDPOINT_TIMEOUT: 30000, // 30秒
  REQUEST_TIMEOUT: 60000,   // 60秒
} as const

// API 错误代码
export const API_ERROR_CODES = {
  DATABASE_ERROR: 'DATABASE_ERROR',
  VALIDATION_ERROR: 'VALIDATION_ERROR',
  NOT_FOUND: 'NOT_FOUND',
  PERMISSION_DENIED: 'PERMISSION_DENIED',
  NETWORK_ERROR: 'NETWORK_ERROR',
  TIMEOUT_ERROR: 'TIMEOUT_ERROR',
  UNKNOWN_ERROR: 'UNKNOWN_ERROR',
} as const

// 时间范围选项
export const TIME_RANGE_OPTIONS = [
  { value: '1h', label: '1小时' },
  { value: '24h', label: '24小时' },
  { value: '7d', label: '7天' },
  { value: '30d', label: '30天' },
] as const

// 日志级别选项
export const LOG_LEVEL_OPTIONS = [
  { value: 'debug', label: '调试', color: '#6b7280' },
  { value: 'info', label: '信息', color: '#3b82f6' },
  { value: 'warn', label: '警告', color: '#f59e0b' },
  { value: 'error', label: '错误', color: '#ef4444' },
] as const

// 端点类型选项
export const ENDPOINT_TYPE_OPTIONS = [
  { value: 'anthropic', label: 'Anthropic', icon: '🤖' },
  { value: 'openai', label: 'OpenAI', icon: '🧠' },
  { value: 'universal', label: '通用', icon: '🌐' },
  { value: 'gemini', label: 'Gemini', icon: '💎' },
] as const

// 认证类型选项
export const AUTH_TYPE_OPTIONS = [
  { value: 'none', label: '无认证' },
  { value: 'api_key', label: 'API Key' },
  { value: 'auth_token', label: 'Bearer Token' },
  { value: 'oauth', label: 'OAuth' },
  { value: 'auto', label: '自动检测' },
] as const

export default {}
