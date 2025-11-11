import { useState, useEffect, useMemo } from 'react'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  Search,
  Filter,
  RefreshCw,
  Trash2,
  Eye,
  Copy,
  AlertCircle,
  CheckCircle,
  XCircle,
  Clock,
  Bot,
  Code,
  Zap,
  Database,
  ArrowRight,
  ArrowLeft,
  Settings,
  MoreHorizontal,
  Globe,
  Server,
  Cpu,
  Wifi,
  WifiOff,
} from 'lucide-react'
import { toast } from 'sonner'
import { wailsAPI } from '@/lib/wails-api'

// 日志类型定义
interface RequestLog {
  id: string
  timestamp: string
  request_id: string
  endpoint: string
  method: string
  path: string
  status_code: number
  duration_ms: number
  attempt_number: number
  request_headers: Record<string, string>
  request_body: string | null
  response_headers: Record<string, string>
  response_body: string | null
  original_request_headers?: Record<string, string>
  original_request_body?: string | null
  original_request_url?: string
  final_request_headers?: Record<string, string>
  final_request_body?: string | null
  original_response_headers?: Record<string, string>
  original_response_body?: string | null
  final_response_headers?: Record<string, string>
  final_response_body?: string | null
  request_body_size: number
  response_body_size: number
  is_streaming: boolean
  error?: string
  model?: string
  original_model?: string
  rewritten_model?: string
  model_rewrite_applied?: boolean
  tags?: string[]
  content_type_override?: string
  session_id?: string
  thinking_enabled?: boolean
  thinking_budget_tokens?: number
  final_request_url?: string
  client_type?: 'claude-code' | 'codex' | 'unknown'
  request_format?: 'anthropic' | 'openai' | 'unknown'
  target_format?: string
  format_converted?: boolean
  endpoint_blacklist_reason?: string
}

interface LogFilters {
  search: string
  client_type: string
  status_range: string
  streaming_only: boolean
  failed_only: boolean
  has_error: boolean
  model?: string
  endpoint?: string
  with_thinking?: boolean
}

// 列配置接口
interface ColumnConfig {
  key: keyof RequestLog | 'actions'
  label: string
  visible: boolean
  priority: number // 数值越大优先级越高
  minWidth?: string
  responsive?: boolean // 是否在小屏幕上隐藏
}

// 默认列配置
const defaultColumns: ColumnConfig[] = [
  { key: 'timestamp', label: '时间', visible: true, priority: 1, minWidth: '80px' },
  { key: 'request_id', label: '请求ID', visible: true, priority: 2, minWidth: '120px' },
  { key: 'client_type', label: '客户端', visible: true, priority: 3, minWidth: '100px' },
  { key: 'status_code', label: '状态', visible: true, priority: 4, minWidth: '80px' },
  { key: 'endpoint', label: '端点', visible: true, priority: 5, minWidth: '200px', responsive: true },
  { key: 'model', label: '模型', visible: true, priority: 6, minWidth: '120px', responsive: true },
  { key: 'duration_ms', label: '耗时', visible: true, priority: 7, minWidth: '80px' },
  { key: 'thinking_enabled', label: '思考', visible: true, priority: 8, minWidth: '60px', responsive: true },
  { key: 'attempt_number', label: '重试', visible: false, priority: 9, minWidth: '60px', responsive: true },
  { key: 'request_body_size', label: '请求大小', visible: false, priority: 10, minWidth: '80px', responsive: true },
  { key: 'response_body_size', label: '响应大小', visible: false, priority: 11, minWidth: '80px', responsive: true },
  { key: 'is_streaming', label: '流式', visible: false, priority: 12, minWidth: '60px', responsive: true },
  { key: 'actions', label: '操作', visible: true, priority: 13, minWidth: '80px' },
]


export function Logs() {
  const [logs, setLogs] = useState<RequestLog[]>([])
  const [loading, setLoading] = useState(false)
  const [failedOnly, setFailedOnly] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [totalLogs, setTotalLogs] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [selectedLog, setSelectedLog] = useState<RequestLog | null>(null)
  const [showDetails, setShowDetails] = useState(false)
  const [filters, setFilters] = useState<LogFilters>({
    search: '',
    client_type: 'all',
    status_range: 'all',
    streaming_only: false,
    failed_only: false,
    has_error: false,
    with_thinking: false,
  })
  const [columns, setColumns] = useState<ColumnConfig[]>(defaultColumns)
  const [isMobile, setIsMobile] = useState(false)
  const [debugMode, setDebugMode] = useState(false)
  const [debugInfo, setDebugInfo] = useState<string>('')

  // 本地存储键
  const COLUMN_CONFIG_KEY = 'logs_table_column_config'

  // 从本地存储加载列配置
  const loadColumnConfig = () => {
    try {
      const saved = localStorage.getItem(COLUMN_CONFIG_KEY)
      if (saved) {
        const savedColumns = JSON.parse(saved)
        // 合并默认配置和保存的配置，确保新列不会丢失
        return defaultColumns.map(defaultCol => {
          const savedCol = savedColumns.find((s: ColumnConfig) => s.key === defaultCol.key)
          return savedCol ? { ...defaultCol, ...savedCol } : defaultCol
        })
      }
    } catch (error) {
      console.warn('Failed to load column config:', error)
    }
    return defaultColumns
  }

  // 防抖保存函数
  let saveTimeout: NodeJS.Timeout | null = null
  const debouncedSaveColumnConfig = (newColumns: ColumnConfig[]) => {
    if (saveTimeout) {
      clearTimeout(saveTimeout)
    }
    saveTimeout = setTimeout(() => {
      try {
        localStorage.setItem(COLUMN_CONFIG_KEY, JSON.stringify(newColumns))
      } catch (error) {
        console.warn('Failed to save column config:', error)
      }
    }, 300) // 300ms 防抖
  }

  // 立即保存列配置（用于重置等需要立即保存的场景）
  const saveColumnConfig = (newColumns: ColumnConfig[]) => {
    try {
      localStorage.setItem(COLUMN_CONFIG_KEY, JSON.stringify(newColumns))
    } catch (error) {
      console.warn('Failed to save column config:', error)
    }
  }

  // 初始化列配置
  useEffect(() => {
    const savedColumns = loadColumnConfig()
    setColumns(savedColumns)
  }, [])

  // 响应式检测
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768)
    }
    checkMobile()
    window.addEventListener('resize', checkMobile)
    return () => window.removeEventListener('resize', checkMobile)
  }, [])

  // 根据屏幕大小和优先级过滤列
  const visibleColumns = useMemo(() => {
    return columns
      .filter(col => {
        if (isMobile && col.responsive) return false
        return col.visible
      })
      .sort((a, b) => a.priority - b.priority)
  }, [columns, isMobile])

  // 切换列可见性
  const toggleColumn = (key: string) => {
    const newColumns = columns.map(col =>
      col.key === key ? { ...col, visible: !col.visible } : col
    )
    setColumns(newColumns)
    debouncedSaveColumnConfig(newColumns)
  }

  // 重置列配置
  const resetColumnConfig = () => {
    setColumns(defaultColumns)
    saveColumnConfig(defaultColumns)
    toast.success('列配置已重置为默认')
  }

  // 加载日志数据
  const loadLogs = async (page = 1) => {
    setLoading(true)
    try {
      const params = {
        page: page.toString(),
        limit: '20',
        search: filters.search,
        client_type: filters.client_type,
        status_range: filters.status_range,
        streaming_only: filters.streaming_only,
        failed_only: failedOnly,
        has_error: filters.has_error,
        with_thinking: filters.with_thinking,
      }

      const debugText = `请求参数: ${JSON.stringify(params, null, 2)}`
      setDebugInfo(debugText)

      // 调用真实API获取日志
      const response = await wailsAPI.GetLogs(params) as any

      const responseDebug = `响应数据: ${JSON.stringify(response, null, 2)}`
      setDebugInfo(prev => prev + '\n\n' + responseDebug)

      setLogs(response.logs || [])
      setTotalLogs(response.total || 0)
      setTotalPages(Math.ceil((response.total || 0) / 20))
      setCurrentPage(page)

      if (!response.logs || response.logs.length === 0) {
        setDebugInfo(prev => prev + '\n\n⚠️ 警告: 日志数组为空!')
      }
    } catch (error) {
      const errorDebug = `请求失败: ${error instanceof Error ? error.message : String(error)}`
      setDebugInfo(prev => prev + '\n\n' + errorDebug)
      toast.error('加载日志失败')
    } finally {
      setLoading(false)
    }
  }

  // 自动刷新
  useEffect(() => {
    if (!autoRefresh) return

    const interval = setInterval(() => {
      loadLogs(currentPage)
    }, 5000) // 5秒刷新一次

    return () => clearInterval(interval)
  }, [autoRefresh, currentPage, failedOnly])

  // 监听过滤器和失败选项变化
  useEffect(() => {
    loadLogs(1) // 当过滤器变化时，回到第一页
  }, [filters, failedOnly])

  // 初始加载
  useEffect(() => {
    loadLogs()
  }, [])

  // 复制请求ID
  const copyRequestId = (requestId: string) => {
    navigator.clipboard.writeText(requestId)
    toast.success('请求ID已复制到剪贴板')
  }

  // 获取客户端类型文本
  const getClientTypeText = (clientType: string, requestFormat?: string) => {
    const displayText = clientType === 'claude-code' ? 'claude code' : clientType.toLowerCase()
    const title = requestFormat ? `${displayText} (${requestFormat})` : displayText
    
    return (
      <span className="text-sm" title={title}>
        {displayText}
      </span>
    )
  }

  // 获取状态徽章
  const getStatusBadge = (statusCode: number) => {
    if (statusCode >= 200 && statusCode < 300) {
      return (
        <Badge variant="default" className="bg-green-500">
          {statusCode}
        </Badge>
      )
    } else {
      return (
        <Badge variant="destructive">
          {statusCode}
        </Badge>
      )
    }
  }

  // 获取重试徽章
  const getRetryBadge = (attemptNumber: number) => {
    if (attemptNumber <= 1) return null

    const retryCount = attemptNumber - 1
    return (
      <Badge variant="secondary" className="bg-orange-500 text-white">
        #{retryCount}
      </Badge>
    )
  }

  // 获取思考模式徽章
  const getThinkingBadge = (enabled: boolean, tokens?: number) => {
    if (!enabled) return <span className="text-muted-foreground">-</span>

    if (!tokens) return <Badge className="bg-green-500">T</Badge>

    let color = 'bg-green-500'
    if (tokens > 32768) color = 'bg-red-500'
    else if (tokens > 16384) color = 'bg-yellow-500'
    else if (tokens > 4096) color = 'bg-orange-500'

    return (
      <Badge className={color} title={`Thinking: ${tokens} tokens`}>
        T
      </Badge>
    )
  }

  // 格式化文件大小
  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + sizes[i]
  }

  // 格式化时间
  const formatTime = (timestamp: string) => {
    return format(new Date(timestamp), 'MM/dd HH:mm:ss', { locale: zhCN })
  }

  const hasHeaders = (headers?: Record<string, string>) =>
    !!headers && Object.keys(headers).length > 0

  const formatHeaders = (headers?: Record<string, string>) => {
    if (!headers || Object.keys(headers).length === 0) {
      return '（空）'
    }
    return Object.entries(headers)
      .map(([key, value]) => `${key}: ${value}`)
      .join('\n')
  }

  const pickNonEmptyBody = (...bodies: Array<string | null | undefined>) => {
    for (const body of bodies) {
      if (body === null || body === undefined) continue
      if (typeof body === 'string' && body.trim() === '') continue
      return body
    }
    return undefined
  }

  const formatBody = (body: string | null | undefined) => {
    if (body === null || body === undefined) {
      return '（空）'
    }
    if (typeof body === 'string') {
      if (body.trim() === '') {
        return '（空）'
      }

      // 如果是JSON字符串，尝试格式化展示
      try {
        const parsed = JSON.parse(body)
        return JSON.stringify(parsed, null, 2)
      } catch {
        return body
      }
    }

    try {
      return JSON.stringify(body, null, 2)
    } catch {
      return String(body)
    }
  }

  const getOriginalRequestHeadersText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    const source = hasHeaders(log.original_request_headers)
      ? log.original_request_headers
      : log.request_headers
    return formatHeaders(source)
  }

  const getFinalRequestHeadersText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    const source = hasHeaders(log.final_request_headers)
      ? log.final_request_headers
      : log.request_headers
    return formatHeaders(source)
  }

  const getOriginalRequestBodyText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    return formatBody(
      pickNonEmptyBody(
        log.original_request_body,
        log.request_body,
        log.final_request_body,
      ),
    )
  }

  const getFinalRequestBodyText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    return formatBody(
      pickNonEmptyBody(
        log.final_request_body,
        log.request_body,
        log.original_request_body,
      ),
    )
  }

  const getOriginalResponseHeadersText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    const source = hasHeaders(log.original_response_headers)
      ? log.original_response_headers
      : log.response_headers
    return formatHeaders(source)
  }

  const getFinalResponseHeadersText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    const source = hasHeaders(log.final_response_headers)
      ? log.final_response_headers
      : log.response_headers
    return formatHeaders(source)
  }

  const getOriginalResponseBodyText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    return formatBody(
      pickNonEmptyBody(
        log.original_response_body,
        log.response_body,
        log.final_response_body,
      ),
    )
  }

  const getFinalResponseBodyText = (log: RequestLog | null) => {
    if (!log) return '（空）'
    return formatBody(
      pickNonEmptyBody(
        log.final_response_body,
        log.response_body,
        log.original_response_body,
      ),
    )
  }

  // 渲染表格单元格
  const renderCell = (log: RequestLog, column: ColumnConfig) => {
    switch (column.key) {
      case 'timestamp':
        return (
          <div className="text-xs font-medium">
            {formatTime(log.timestamp)}
          </div>
        )

      case 'request_id':
        return (
          <div className="flex items-center gap-1">
            <span className="text-xs font-mono max-w-[80px] truncate" title={log.request_id}>
              {log.request_id}
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="h-6 w-6 p-0 hover:bg-muted"
              onClick={() => copyRequestId(log.request_id)}
            >
              <Copy className="w-3 h-3" />
            </Button>
          </div>
        )

      case 'client_type':
        return (
          <div className="flex items-center gap-1">
            {getClientTypeText(log.client_type || 'unknown', log.request_format)}
          </div>
        )

      case 'status_code':
        const statusMessage = log.error || log.endpoint_blacklist_reason || (log.status_code >= 200 && log.status_code < 300 ? '请求成功' : '请求失败')
        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <div className="flex items-center gap-1 cursor-help">
                {getStatusBadge(log.status_code)}
                {log.session_id && (
                  <Badge variant="outline" className="text-xs" title={`Session ID: ${log.session_id}`}>
                    SID
                  </Badge>
                )}
              </div>
            </TooltipTrigger>
            <TooltipContent>
              <p>{statusMessage}</p>
            </TooltipContent>
          </Tooltip>
        )

      case 'endpoint':
        const endpointUrl = log.final_request_url || log.endpoint
        const displayUrl = endpointUrl.length > 30 ? endpointUrl.substring(0, 30) + '...' : endpointUrl

        return (
          <div className="max-w-[150px]">
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="flex items-center gap-1">
                  <code className="text-xs bg-muted px-1 py-0.5 rounded truncate cursor-help">
                    {displayUrl}
                  </code>
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <p>{endpointUrl}</p>
              </TooltipContent>
            </Tooltip>
          </div>
        )

      case 'model':
        return (
          <div className="text-xs">
            {log.model ? (
              <span className={log.model_rewrite_applied ? 'text-orange-600' : ''} title={log.model_rewrite_applied ? `重写为: ${log.rewritten_model}` : ''}>
                {log.model}
              </span>
            ) : (
              <span className="text-muted-foreground">-</span>
            )}
          </div>
        )

      case 'duration_ms':
        return (
          <div className="text-xs">
            <span className={log.duration_ms > 5000 ? 'text-orange-600' : log.duration_ms > 10000 ? 'text-red-600' : ''}>
              {log.duration_ms}ms
            </span>
          </div>
        )

      case 'thinking_enabled':
        return getThinkingBadge(log.thinking_enabled || false, log.thinking_budget_tokens)

      case 'attempt_number':
        return getRetryBadge(log.attempt_number)

      case 'request_body_size':
        return (
          <div className="text-xs text-muted-foreground">
            {formatFileSize(log.request_body_size)}
          </div>
        )

      case 'response_body_size':
        return (
          <div className="text-xs text-muted-foreground">
            {formatFileSize(log.response_body_size)}
          </div>
        )

      case 'is_streaming':
        return (
          <div className="text-xs">
            {log.is_streaming ? (
              <Badge variant="default" className="text-xs bg-green-500">
                SSE
              </Badge>
            ) : (
              <Badge variant="secondary" className="text-xs">
                JSON
              </Badge>
            )}
          </div>
        )

      case 'actions':
        return (
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setSelectedLog(log)
              setShowDetails(true)
            }}
            className="h-7 w-7 p-0"
          >
            <Eye className="w-3 h-3" />
          </Button>
        )

      default:
        return null
    }
  }

  // 清理日志
  const cleanupLogs = async (days: number) => {
    try {
      const response = await wailsAPI.GetLogs({ cleanup: days }) as any
      if (response.success) {
        if (days === 0) {
          toast.success(`已清理所有日志，共 ${response.rows_affected || 0} 条记录`)
        } else {
          toast.success(`已清理 ${days} 天前的日志，共 ${response.rows_affected || 0} 条记录`)
        }
        // 清理成功后刷新到第1页，因为当前页可能已经没有数据了
        loadLogs(1)
      } else {
        toast.error(response.error || '清理日志失败')
      }
    } catch (error) {
      console.error('Failed to cleanup logs:', error)
      toast.error('清理日志失败')
    }
  }

  
  return (
    <div className="h-full flex flex-col space-y-4">
      {/* 页面标题和操作按钮 */}
      <div className="flex items-center justify-between shrink-0">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">请求日志</h1>
          <p className="text-muted-foreground">
            查看和分析API请求日志
          </p>
        </div>
        <div className="flex gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="text-destructive">
                <Trash2 className="w-4 h-4 mr-2" />
                清理日志
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <div className="px-2 py-1.5 border-b">
                <p className="text-xs text-muted-foreground flex items-center gap-1">
                  <AlertCircle className="w-3 h-3 text-orange-500" />
                  选择清理范围（不可撤销）
                </p>
              </div>
              <DropdownMenuItem
                onClick={() => cleanupLogs(1)}
                className="text-red-600 focus:text-red-600"
              >
                <div className="flex items-center gap-2">
                  <Clock className="w-4 h-4" />
                  <div>
                    <div className="font-medium">清除 1 天前的日志</div>
                    <div className="text-xs text-muted-foreground">保留最近 24 小时的记录</div>
                  </div>
                </div>
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => cleanupLogs(7)}
                className="text-red-600 focus:text-red-600"
              >
                <div className="flex items-center gap-2">
                  <Clock className="w-4 h-4" />
                  <div>
                    <div className="font-medium">清除 7 天前的日志</div>
                    <div className="text-xs text-muted-foreground">保留最近一周的记录</div>
                  </div>
                </div>
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => cleanupLogs(30)}
                className="text-red-600 focus:text-red-600"
              >
                <div className="flex items-center gap-2">
                  <Clock className="w-4 h-4" />
                  <div>
                    <div className="font-medium">清除 30 天前的日志</div>
                    <div className="text-xs text-muted-foreground">保留最近一个月的记录</div>
                  </div>
                </div>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => cleanupLogs(0)}
                className="text-red-600 bg-red-50 focus:text-red-600 focus:bg-red-50"
              >
                <div className="flex items-center gap-2">
                  <Trash2 className="w-4 h-4" />
                  <div>
                    <div className="font-medium">清除所有日志</div>
                    <div className="text-xs text-muted-foreground">删除所有历史记录</div>
                  </div>
                </div>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <Button onClick={() => loadLogs(currentPage)} disabled={loading}>
            <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
            刷新
          </Button>

          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-2 border rounded-md px-3 py-1.5 transition-colors ${
              autoRefresh ? 'bg-primary/10 border-primary/20' : 'bg-muted/30'
            }`}
          >
            <RefreshCw className={`w-4 h-4 ${autoRefresh ? 'text-primary animate-spin' : 'text-muted-foreground'}`} />
            <span className={`text-sm ${autoRefresh ? 'text-primary font-medium' : 'text-muted-foreground'}`}>
              自动刷新
            </span>
            <div
              className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                autoRefresh ? 'bg-primary' : 'bg-gray-300'
              }`}
            >
              <span
                className={`inline-block h-3 w-3 transform rounded-full bg-white transition-transform ${
                  autoRefresh ? 'translate-x-5' : 'translate-x-0.5'
                }`}
              />
            </div>
          </button>

          <Button
            variant={debugMode ? "default" : "outline"}
            onClick={() => setDebugMode(!debugMode)}
            className="text-xs"
          >
            <Settings className="w-4 h-4 mr-1" />
            调试
          </Button>
        </div>
      </div>

      {/* 过滤器 */}
      <div className="shrink-0 p-4 bg-card border rounded-lg">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="space-y-2">
              <Label>搜索</Label>
              <div className="relative">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="搜索请求ID、端点、模型..."
                  value={filters.search}
                  onChange={(e) => setFilters(prev => ({ ...prev, search: e.target.value }))}
                  className="pl-8"
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label>客户端类型</Label>
              <Select value={filters.client_type} onValueChange={(value: string) =>
                setFilters(prev => ({ ...prev, client_type: value }))
              }>
                <SelectTrigger>
                  <SelectValue placeholder="选择客户端类型" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="claude-code">Claude Code</SelectItem>
                  <SelectItem value="codex">Codex</SelectItem>
                  <SelectItem value="unknown">未知</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>状态码范围</Label>
              <Select value={filters.status_range} onValueChange={(value: string) =>
                setFilters(prev => ({ ...prev, status_range: value }))
              }>
                <SelectTrigger>
                  <SelectValue placeholder="选择状态范围" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="2xx">成功 (2xx)</SelectItem>
                  <SelectItem value="4xx">客户端错误 (4xx)</SelectItem>
                  <SelectItem value="5xx">服务器错误 (5xx)</SelectItem>
                  <SelectItem value="error">错误请求 (≥400)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>响应类型</Label>
              <Select value={filters.streaming_only ? 'streaming' : filters.model ? 'has-model' : 'all'} onValueChange={(value: string) =>
                setFilters(prev => ({
                  ...prev,
                  streaming_only: value === 'streaming',
                  model: value === 'has-model' ? 'any' : undefined
                }))
              }>
                <SelectTrigger>
                  <SelectValue placeholder="选择响应类型" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="streaming">仅流式响应</SelectItem>
                  <SelectItem value="has-model">有模型重写</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* 选项行 */}
          <div className="flex items-center gap-4 pt-2">
            <div className="flex items-center space-x-2">
              <Checkbox
                id="failed-only"
                checked={failedOnly}
                onCheckedChange={(checked: boolean) => setFailedOnly(checked)}
              />
              <Label htmlFor="failed-only" className="text-sm text-red-600">
                仅显示失败请求
              </Label>
            </div>
            <div className="flex items-center space-x-2">
              <Checkbox
                id="with-thinking"
                checked={filters.with_thinking}
                onCheckedChange={(checked: boolean) =>
                  setFilters(prev => ({ ...prev, with_thinking: checked }))
                }
              />
              <Label htmlFor="with-thinking" className="text-sm">启用思考</Label>
            </div>
          </div>
      </div>

      {/* 调试面板 */}
      {debugMode && (
        <div className="shrink-0 max-h-48 p-4 bg-card border rounded-lg">
          <div className="pb-2">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-mono">调试信息</h3>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  navigator.clipboard.writeText(debugInfo)
                  toast.success('调试信息已复制到剪贴板')
                }}
              >
                <Copy className="w-3 h-3 mr-1" />
                复制
              </Button>
            </div>
          </div>
          <div className="pt-0">
            <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-32 whitespace-pre-wrap font-mono">
              {debugInfo || '暂无调试信息，请尝试加载日志'}
            </pre>
          </div>
        </div>
      )}

      {/* 日志表格 */}
      <div className="flex-1 flex flex-col bg-card border rounded-lg">
        <div className="shrink-0 p-4 border-b">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <span>日志列表</span>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Settings className="w-4 h-4 mr-2" />
                    列配置
                    <Badge variant="secondary" className="ml-2 text-xs">
                      自动保存
                    </Badge>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56">
                  <div className="px-2 py-1.5 border-b">
                    <p className="text-xs text-muted-foreground flex items-center gap-1">
                      <CheckCircle className="w-3 h-3 text-green-500" />
                      配置自动保存，下次访问时生效
                    </p>
                  </div>
                  <div className="max-h-48 overflow-y-auto">
                    {columns.map((column) => (
                      <DropdownMenuCheckboxItem
                        key={column.key}
                        checked={column.visible}
                        onCheckedChange={() => toggleColumn(column.key)}
                        disabled={column.key === 'actions'} // 操作列不能隐藏
                      >
                        <div className="flex items-center gap-2">
                          <span>{column.label}</span>
                          {column.responsive && (
                            <Badge variant="secondary" className="text-xs">
                              响应式
                            </Badge>
                          )}
                        </div>
                      </DropdownMenuCheckboxItem>
                    ))}
                  </div>
                  <DropdownMenuCheckboxItem
                    checked={false}
                    onCheckedChange={resetColumnConfig}
                    className="border-t"
                  >
                    <div className="flex items-center gap-2 text-red-600">
                      <RefreshCw className="w-4 h-4" />
                      <span>重置为默认</span>
                    </div>
                  </DropdownMenuCheckboxItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            <div className="flex items-center gap-4">
              <span className="text-sm font-normal text-muted-foreground">
                共 {totalLogs} 条记录
              </span>
              {isMobile && (
                <Badge variant="outline" className="text-xs">
                  {visibleColumns.length} 列
                </Badge>
              )}
            </div>
          </div>
        </div>
        <div className="flex-1 flex flex-col overflow-hidden">
          {loading ? (
            <div className="flex items-center justify-center h-64">
              <RefreshCw className="w-6 h-6 animate-spin" />
              <span className="ml-2">加载中...</span>
            </div>
          ) : logs.length === 0 ? (
            <div className="flex items-center justify-center h-64 text-muted-foreground">
              <Database className="w-6 h-6 mr-2" />
              暂无日志记录
            </div>
          ) : (
            <>
              <div className="flex-1 flex flex-col overflow-hidden">
                {/* 桌面端表格视图 */}
                {!isMobile && (
                <div className="flex-1 overflow-auto">
                <Table>
                        <TableHeader>
                          <TableRow>
                          {visibleColumns.map((column) => (
                            <TableHead
                              key={column.key}
                              className={`${column.minWidth ? `min-w-[${column.minWidth}]` : ''}`}
                            >
                              <div className="flex items-center gap-2">
                                {column.label}
                              </div>
                            </TableHead>
                          ))}
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {logs.map((log) => (
                          <TableRow key={log.id} className="hover:bg-muted/50">
                            {visibleColumns.map((column) => (
                              <TableCell
                                key={column.key}
                                className={`${column.minWidth ? `min-w-[${column.minWidth}]` : ''}`}
                              >
                                {renderCell(log, column)}
                              </TableCell>
                            ))}
                          </TableRow>
                        ))}
                      </TableBody>
                </Table>
              </div>
                )}
              </div>

              {/* 移动端卡片视图 */}
              {isMobile && (
                <div className="space-y-3 flex-1 overflow-y-auto">
                  {logs.map((log) => (
                    <Card key={log.id} className="p-4">
                      <div className="space-y-3">
                        {/* 头部：时间和请求ID */}
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <span className="text-xs font-medium text-muted-foreground">
                              {formatTime(log.timestamp)}
                            </span>
                            {getStatusBadge(log.status_code)}
                          </div>
                          <div className="flex items-center gap-1">
                            <span className="text-xs font-mono max-w-[60px] truncate" title={log.request_id}>
                              {log.request_id}
                            </span>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-6 w-6 p-0"
                              onClick={() => copyRequestId(log.request_id)}
                            >
                              <Copy className="w-3 h-3" />
                            </Button>
                          </div>
                        </div>

                        {/* 客户端和模型信息 */}
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            {getClientTypeText(log.client_type || 'unknown', log.request_format)}
                          </div>
                          <div className="flex items-center gap-2">
                            {getThinkingBadge(log.thinking_enabled || false, log.thinking_budget_tokens)}
                            {getRetryBadge(log.attempt_number)}
                          </div>
                        </div>

                        {/* 端点和模型 */}
                        <div className="space-y-2">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <div className="flex items-center gap-1">
                                <code className="text-xs bg-muted px-1 py-0.5 rounded truncate flex-1 cursor-help">
                                  {(log.final_request_url || log.endpoint)?.length > 40
                                    ? (log.final_request_url || log.endpoint)?.substring(0, 40) + '...'
                                    : (log.final_request_url || log.endpoint)
                                  }
                                </code>
                              </div>
                            </TooltipTrigger>
                            <TooltipContent>
                              <p>{log.final_request_url || log.endpoint}</p>
                            </TooltipContent>
                          </Tooltip>
                          {log.model && (
                            <div className="flex items-center gap-2">
                              <Bot className="w-3 h-3 text-muted-foreground" />
                              <span className={`text-xs ${log.model_rewrite_applied ? 'text-orange-600' : ''}`}>
                                {log.model}
                              </span>
                            </div>
                          )}
                        </div>

                        {/* 错误信息 */}
                        {(log.error || log.endpoint_blacklist_reason) && (
                          <div className="flex items-center gap-2 text-xs text-red-600 bg-red-50 p-2 rounded">
                            <WifiOff className="w-3 h-3" />
                            <span className="truncate">{log.endpoint_blacklist_reason || log.error}</span>
                          </div>
                        )}

                        {/* 底部：耗时和操作 */}
                        <div className="flex items-center justify-between pt-2 border-t">
                          <div className="flex items-center gap-3 text-xs text-muted-foreground">
                            <div className="flex items-center gap-1">
                              <Clock className="w-3 h-3" />
                              <span className={log.duration_ms > 5000 ? 'text-orange-600' : log.duration_ms > 10000 ? 'text-red-600' : ''}>
                                {log.duration_ms}ms
                              </span>
                            </div>
                            {log.is_streaming ? (
                              <Badge variant="default" className="text-xs bg-green-500">
                                <Wifi className="w-3 h-3 mr-1" />
                                SSE
                              </Badge>
                            ) : (
                              <Badge variant="secondary" className="text-xs">
                                <WifiOff className="w-3 h-3 mr-1" />
                                JSON
                              </Badge>
                            )}
                          </div>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setSelectedLog(log)}
                            className="h-7 px-2"
                          >
                            <Eye className="w-3 h-3 mr-1" />
                            详情
                          </Button>
                        </div>
                      </div>
                    </Card>
                  ))}
                </div>
              )}
            </>
          )}

          {/* 分页 */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between space-x-2 py-4">
              <div className="text-sm text-muted-foreground">
                第 {currentPage} / {totalPages} 页，共 {totalLogs} 条记录，每页显示 20 条
              </div>
              <div className="flex items-center space-x-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => loadLogs(currentPage - 1)}
                  disabled={!currentPage || currentPage === 1 || loading}
                >
                  <ArrowLeft className="w-4 h-4 mr-1" />
                  上一页
                </Button>

                <div className="flex items-center space-x-1">
                  {/* 生成页码 */}
                  {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                    let pageNum
                    if (totalPages <= 5) {
                      pageNum = i + 1
                    } else if (currentPage <= 3) {
                      pageNum = i + 1
                    } else if (currentPage >= totalPages - 2) {
                      pageNum = totalPages - 4 + i
                    } else {
                      pageNum = currentPage - 2 + i
                    }

                    return (
                      <Button
                        key={pageNum}
                        variant={currentPage === pageNum ? "default" : "outline"}
                        size="sm"
                        onClick={() => loadLogs(pageNum)}
                        disabled={loading}
                      >
                        {pageNum}
                      </Button>
                    )
                  })}
                </div>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => loadLogs(currentPage + 1)}
                  disabled={!currentPage || currentPage === totalPages || loading}
                >
                  下一页
                  <ArrowRight className="w-4 h-4 ml-1" />
                </Button>
              </div>
            </div>
          )}
          </div>
          </div>

      {/* 详情弹窗（移动端和桌面端共用） */}
      <Dialog open={showDetails} onOpenChange={setShowDetails}>
        <DialogContent className="max-w-4xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>日志详情</DialogTitle>
            <DialogDescription>
              请求ID: {selectedLog?.request_id}
            </DialogDescription>
          </DialogHeader>
          {selectedLog && (
            <div className="space-y-4">
              {/* 基本信息 */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-sm font-medium">时间</Label>
                  <p className="text-sm">{formatTime(selectedLog.timestamp)}</p>
                </div>
                <div>
                  <Label className="text-sm font-medium">状态码</Label>
                  <p className="text-sm">{selectedLog.status_code}</p>
                </div>
                <div>
                  <Label className="text-sm font-medium">耗时</Label>
                  <p className="text-sm">{selectedLog.duration_ms}ms</p>
                </div>
                <div>
                  <Label className="text-sm font-medium">客户端类型</Label>
                  <p className="text-sm">{selectedLog.client_type || 'unknown'}</p>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {selectedLog.original_request_url && (
                  <div>
                    <Label className="text-sm font-medium">客户端请求URL</Label>
                    <p className="text-sm font-mono bg-muted p-2 rounded break-all">
                      {selectedLog.original_request_url}
                    </p>
                  </div>
                )}
                <div>
                  <Label className="text-sm font-medium">代理请求URL</Label>
                  <p className="text-sm font-mono bg-muted p-2 rounded break-all">
                    {selectedLog.final_request_url || selectedLog.endpoint}
                  </p>
                </div>
              </div>

              {selectedLog.error && (
                <div>
                  <Label className="text-sm font-medium text-red-600">错误信息</Label>
                  <p className="text-sm text-red-600 bg-red-50 p-2 rounded">
                    {selectedLog.error}
                  </p>
                </div>
              )}

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-sm font-medium">请求头（原始）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-48 whitespace-pre-wrap break-words">
                    {getOriginalRequestHeadersText(selectedLog)}
                  </pre>
                </div>
                <div>
                  <Label className="text-sm font-medium">请求头（最终）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-48 whitespace-pre-wrap break-words">
                    {getFinalRequestHeadersText(selectedLog)}
                  </pre>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-sm font-medium">请求体（原始）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-words">
                    {getOriginalRequestBodyText(selectedLog)}
                  </pre>
                </div>
                <div>
                  <Label className="text-sm font-medium">请求体（最终）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-words">
                    {getFinalRequestBodyText(selectedLog)}
                  </pre>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-sm font-medium">响应头（原始）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-48 whitespace-pre-wrap break-words">
                    {getOriginalResponseHeadersText(selectedLog)}
                  </pre>
                </div>
                <div>
                  <Label className="text-sm font-medium">响应头（最终）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-48 whitespace-pre-wrap break-words">
                    {getFinalResponseHeadersText(selectedLog)}
                  </pre>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-sm font-medium">响应体（原始）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-words">
                    {getOriginalResponseBodyText(selectedLog)}
                  </pre>
                </div>
                <div>
                  <Label className="text-sm font-medium">响应体（最终）</Label>
                  <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-words">
                    {getFinalResponseBodyText(selectedLog)}
                  </pre>
                </div>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
