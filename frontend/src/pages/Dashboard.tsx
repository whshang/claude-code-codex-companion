import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Activity,
  Server,
  AlertTriangle,
  TrendingUp,
  TrendingDown,
  Clock,
  RefreshCw,
  CheckCircle,
  XCircle,
  Zap,
  Play,
  Pause,
  TestTube,
  ExternalLink,
  Settings,
  Download
} from 'lucide-react'
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend
} from 'recharts'
import { wailsAPI } from '@/lib/wails-api'
import { toast } from 'sonner'

interface SystemStats {
  totalRequests: number
  successRequests: number
  errorsCount: number
  successRate: number
  avgResponseTime: number
  requestsPerSecond: number
  systemUptime: number
  activeEndpoints: number
  totalEndpoints: number
}


interface EndpointStatus {
  name: string
  status: 'healthy' | 'unhealthy' | 'degraded'
  requests: number
  avgResponseTime: number
  successRate: number
  lastCheck: string
}

interface RequestTrend {
  timestamp: string
  requests: number
  errors: number
  avgLatency: number
}

interface RealtimeMetrics {
  requestHistory: Array<{ time: string; requests: number; errors: number }>
}

const timeRanges = [
  { value: '5m', label: '5分钟' },
  { value: '15m', label: '15分钟' },
  { value: '1h', label: '1小时' },
  { value: '6h', label: '6小时' },
  { value: '24h', label: '24小时' },
]

const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884D8']

export default function Dashboard() {
  const [stats, setStats] = useState<SystemStats | null>(null)
  const [endpoints, setEndpoints] = useState<EndpointStatus[]>([])
  const [realtimeMetrics, setRealtimeMetrics] = useState<RealtimeMetrics | null>(null)
  const [selectedTimeRange, setSelectedTimeRange] = useState('1h')
  const [loading, setLoading] = useState(true)
  const [lastUpdate, setLastUpdate] = useState(new Date())
  const [autoRefresh, setAutoRefresh] = useState(false) // 默认关闭自动刷新
  const [testingEndpoints, setTestingEndpoints] = useState<Set<string>>(new Set())
  const [refreshInterval, setRefreshInterval] = useState(30000) // 30秒默认刷新间隔
  const [isManualRefresh, setIsManualRefresh] = useState(false)
  const [appStartTime] = useState(new Date()) // 应用启动时间
  const [initialLoadFailed, setInitialLoadFailed] = useState(false) // 标记初始加载是否失败

  
  useEffect(() => {
    loadDashboardData()

    // 启动时如果没有数据，3秒后重试一次
    const retryTimeout = setTimeout(() => {
      if (!stats && !endpoints.length) {
        console.log('Retrying initial data load...')
        loadDashboardData(true)
      }
    }, 3000)

    let interval: NodeJS.Timeout
    if (autoRefresh && !isManualRefresh) {
      interval = setInterval(loadDashboardData, refreshInterval)
    }

    return () => {
      clearTimeout(retryTimeout)
      if (interval) clearInterval(interval)
    }
  }, [selectedTimeRange, autoRefresh, refreshInterval, isManualRefresh])

  const loadDashboardData = async (isManual = false) => {
    try {
      if (isManual) {
        setLoading(true)
        setIsManualRefresh(true)
        setInitialLoadFailed(false) // 手动重试时重置失败状态
      }

      // 智能刷新：避免过于频繁的请求
      const now = Date.now()
      const timeSinceLastUpdate = now - lastUpdate.getTime()
      if (!isManual && timeSinceLastUpdate < Math.min(refreshInterval, 10000)) {
        return
      }

      const [statsResponse, endpointsResponse, trendsResponse] = await Promise.all([
        wailsAPI.GetStats().catch((err: any) => {
          console.warn('Stats API failed:', err)
          return null // 返回null而不是抛出错误
        }),
        wailsAPI.GetEndpointStats().catch((err: any) => {
          console.warn('Endpoint stats API failed:', err)
          return null
        }),
        wailsAPI.GetRequestTrends(selectedTimeRange).catch((err: any) => {
          console.warn('Trends API failed:', err)
          return null
        })
      ])

      // 设置统计数据（只设置非空的数据）
      if (statsResponse) {
        setStats(statsResponse as SystemStats)
      }

      // 设置端点状态
      if (endpointsResponse && (endpointsResponse as any).stats) {
        setEndpoints((endpointsResponse as any).stats)
      }

      // 设置趋势数据
      if (trendsResponse && (trendsResponse as any).data) {
        const trends = (trendsResponse as any).data
        const requestHistory = trends.map((trend: any) => ({
          time: new Date(trend.timestamp).toLocaleTimeString('zh-CN', {
            hour: '2-digit',
            minute: '2-digit'
          }),
          requests: trend.requests,
          errors: trend.failed || 0
        }))

        setRealtimeMetrics({
          requestHistory
        })
      }

      setLastUpdate(new Date())
      setInitialLoadFailed(false) // 数据加载成功，重置失败状态
    } catch (error) {
      console.error('Failed to load dashboard data:', error)
      // 只有在初始加载且没有任何数据时才标记为失败
      if (!stats && !endpoints.length && !realtimeMetrics) {
        setInitialLoadFailed(true)
      }
      // 不再清空所有数据，只在完全无数据时显示状态
      if (!isManual && !stats && !endpoints.length) {
        // 静默失败，不显示toast避免打扰用户
      }
    } finally {
      setLoading(false)
      if (isManual) {
        setTimeout(() => setIsManualRefresh(false), 1000)
      }
    }
  }

  const formatUptime = (seconds?: number) => {
    // 如果没有提供运行时间，使用应用启动时间计算
    if (!seconds || seconds === 0) {
      const uptimeMs = Date.now() - appStartTime.getTime()
      seconds = Math.floor(uptimeMs / 1000)
    }

    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)

    if (days > 0) {
      return `${days}天 ${hours}小时`
    } else if (hours > 0) {
      return `${hours}小时 ${minutes}分钟`
    } else {
      return `${minutes}分钟`
    }
  }

  
  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'healthy':
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case 'unhealthy':
        return <XCircle className="h-4 w-4 text-red-500" />
      case 'degraded':
        return <AlertTriangle className="h-4 w-4 text-yellow-500" />
      default:
        return <Clock className="h-4 w-4 text-gray-500" />
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'healthy':
        return 'text-green-600 bg-green-50'
      case 'unhealthy':
        return 'text-red-600 bg-red-50'
      case 'degraded':
        return 'text-yellow-600 bg-yellow-50'
      default:
        return 'text-gray-600 bg-gray-50'
    }
  }

  const getTrendIcon = (current: number, previous: number) => {
    if (current > previous) {
      return <TrendingUp className="h-4 w-4 text-green-500" />
    } else if (current < previous) {
      return <TrendingDown className="h-4 w-4 text-red-500" />
    }
    return <div className="h-4 w-4" />
  }

  // 端点操作函数
  const testEndpoint = async (endpointName: string) => {
    setTestingEndpoints(prev => new Set(prev).add(endpointName))
    try {
      const result = await wailsAPI.TestEndpoint(endpointName)
      if (result.success) {
        toast.success(`端点 ${endpointName} 测试成功，响应时间: ${result.response_time}ms`)
        // 刷新数据
        loadDashboardData()
      } else {
        toast.error(`端点 ${endpointName} 测试失败: ${result.error}`)
      }
    } catch (error) {
      console.error('Test endpoint error:', error)
      toast.error(`端点 ${endpointName} 测试失败: ${error instanceof Error ? error.message : '未知错误'}`)
    } finally {
      setTestingEndpoints(prev => {
        const newSet = new Set(prev)
        newSet.delete(endpointName)
        return newSet
      })
    }
  }

  const toggleEndpoint = async (endpointName: string, enabled: boolean) => {
    try {
      await wailsAPI.UpdateEndpoint(endpointName, { enabled: !enabled })
      toast.success(`端点 ${endpointName} 已${!enabled ? '启用' : '禁用'}`)
      // 刷新数据
      loadDashboardData()
    } catch (error) {
      console.error('Toggle endpoint error:', error)
      toast.error(`端点操作失败: ${error instanceof Error ? error.message : '未知错误'}`)
    }
  }

  // 导出Dashboard数据
  const exportDashboardData = async () => {
    try {
      const exportData = {
        timestamp: new Date().toISOString(),
        timeRange: selectedTimeRange,
        stats: stats,
        endpoints: endpoints,
        trends: realtimeMetrics,
        lastUpdate: lastUpdate.toISOString()
      }

      // 格式化数据为CSV
      const csvContent = [
        '指标,数值',
        `端点总数,${stats?.totalEndpoints || 0}`,
        `活跃端点,${stats?.activeEndpoints || 0}`,
        `总请求数,${stats?.totalRequests || 0}`,
        `成功率,${stats?.successRate || 0}%`,
        `平均响应时间,${stats?.avgResponseTime || 0}ms`,
        `错误数,${stats?.errorsCount || 0}`,
        '',
        '端点状态',
        '端点名称,状态,请求数,响应时间,成功率,最后检查'
      ]

      // 添加端点数据
      if (endpoints && endpoints.length > 0) {
        endpoints.forEach(endpoint => {
          csvContent.push(
            `${endpoint.name},${endpoint.status},${endpoint.requests},${endpoint.avgResponseTime}ms,${endpoint.successRate.toFixed(1)}%,${new Date(endpoint.lastCheck).toLocaleString('zh-CN')}`
          )
        })
      }

      // 创建并下载文件
      const blob = new Blob([csvContent.join('\n')], { type: 'text/csv;charset=utf-8;' })
      const link = document.createElement('a')
      const url = URL.createObjectURL(blob)
      link.setAttribute('href', url)
      link.setAttribute('download', `dashboard_${new Date().toISOString().split('T')[0]}.csv`)
      link.style.visibility = 'hidden'
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)

      toast.success('数据导出成功')
    } catch (error) {
      console.error('Export error:', error)
      toast.error('数据导出失败')
    }
  }

  // 如果正在加载且没有任何数据，显示加载状态
  if (loading && !stats && endpoints.length === 0) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  // 只有在确定初始加载失败的情况下才显示错误页面
  if (initialLoadFailed && !loading && !stats && endpoints.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        <AlertTriangle className="h-8 w-8 mx-auto mb-2" />
        <p>数据加载中或连接失败</p>
        <Button variant="outline" size="sm" onClick={() => loadDashboardData(true)} className="mt-2">
          重新连接
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* 页面标题和控制栏 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">实时监控仪表板</h1>
          <p className="text-muted-foreground">
            系统运行状态、性能指标和实时监控数据
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select value={refreshInterval.toString()} onValueChange={(value: string) => setRefreshInterval(Number(value))}>
            <SelectTrigger className="w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="3000">3秒</SelectItem>
              <SelectItem value="5000">5秒</SelectItem>
              <SelectItem value="10000">10秒</SelectItem>
              <SelectItem value="30000">30秒</SelectItem>
            </SelectContent>
          </Select>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAutoRefresh(!autoRefresh)}
          >
            <RefreshCw className={`h-4 w-4 mr-2 ${autoRefresh ? 'animate-spin' : ''}`} />
            {autoRefresh ? '自动刷新' : '手动刷新'}
          </Button>
          <Button variant="outline" size="sm" onClick={() => loadDashboardData(true)}>
            <RefreshCw className="h-4 w-4 mr-2" />
            立即刷新
          </Button>
          <Button variant="outline" size="sm" onClick={exportDashboardData}>
            <Download className="h-4 w-4 mr-2" />
            导出数据
          </Button>
        </div>
      </div>

      {/* 时间范围选择 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">时间范围:</span>
          <div className="flex gap-1">
            {timeRanges.map((range) => (
              <Button
                key={range.value}
                variant={selectedTimeRange === range.value ? 'default' : 'outline'}
                size="sm"
                onClick={() => setSelectedTimeRange(range.value)}
              >
                {range.label}
              </Button>
            ))}
          </div>
        </div>
        <div className="text-sm text-muted-foreground">
          最后更新: {lastUpdate.toLocaleTimeString('zh-CN')}
        </div>
      </div>

      {/* 核心指标卡片 */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">总请求数</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.totalRequests ? stats.totalRequests.toLocaleString() : '--'}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats ? '总请求数' : '暂无数据'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">成功率</CardTitle>
            <TrendingUp className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.successRate !== undefined ? `${stats.successRate.toFixed(1)}%` : '--'}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats ? '请求成功率' : '暂无数据'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">平均响应时间</CardTitle>
            <Zap className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.avgResponseTime !== undefined ? `${stats.avgResponseTime}ms` : '--'}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats ? '平均响应时间' : '暂无数据'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">每秒请求数</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.requestsPerSecond !== undefined ? stats.requestsPerSecond.toFixed(1) : '--'}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats ? '每秒请求数' : '暂无数据'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">运行时间</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-lg font-bold">
              {formatUptime(stats?.systemUptime)}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats ? '系统持续运行中' : '应用启动后运行时间'}
            </p>
          </CardContent>
        </Card>
      </div>

      
      {/* 实时图表区域 */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>请求趋势</CardTitle>
            <CardDescription>
              实时请求量和错误数量变化
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <AreaChart data={realtimeMetrics?.requestHistory || []}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Area
                  type="monotone"
                  dataKey="requests"
                  stackId="1"
                  stroke="hsl(var(--primary))"
                  fill="hsl(var(--primary))"
                  name="请求数"
                />
                <Area
                  type="monotone"
                  dataKey="errors"
                  stackId="2"
                  stroke="hsl(var(--destructive))"
                  fill="hsl(var(--destructive))"
                  name="错误数"
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      {/* 端点状态监控 */}
      <Card>
        <CardHeader>
          <CardTitle>端点状态监控</CardTitle>
          <CardDescription>
            各端点的健康状态和性能指标
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {endpoints.length > 0 ? (
              endpoints.map((endpoint, index) => (
                <div key={index} className="border rounded-lg p-4 hover:shadow-md transition-shadow">
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2">
                      {getStatusIcon(endpoint.status)}
                      <h3 className="font-medium">{endpoint.name}</h3>
                    </div>
                    <Badge className={getStatusColor(endpoint.status)}>
                      {endpoint.status === 'healthy' ? '健康' :
                       endpoint.status === 'unhealthy' ? '异常' : '降级'}
                    </Badge>
                  </div>
                  <div className="space-y-2 text-sm mb-4">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">请求数:</span>
                      <span className="font-medium">{endpoint.requests.toLocaleString()}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">响应时间:</span>
                      <span className="font-medium">{endpoint.avgResponseTime}ms</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">成功率:</span>
                      <span className="font-medium">{endpoint.successRate.toFixed(1)}%</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">最后检查:</span>
                      <span className="font-medium">
                        {new Date(endpoint.lastCheck).toLocaleTimeString('zh-CN')}
                      </span>
                    </div>
                  </div>
                  <div className="flex gap-2 pt-3 border-t">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => testEndpoint(endpoint.name)}
                      disabled={testingEndpoints.has(endpoint.name)}
                      className="flex-1"
                    >
                      {testingEndpoints.has(endpoint.name) ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <TestTube className="h-3 w-3" />
                      )}
                      <span className="ml-1">测试</span>
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => toggleEndpoint(endpoint.name, endpoint.status === 'healthy')}
                      className="flex-1"
                    >
                      {endpoint.status === 'healthy' ? (
                        <Pause className="h-3 w-3" />
                      ) : (
                        <Play className="h-3 w-3" />
                      )}
                      <span className="ml-1">{endpoint.status === 'healthy' ? '禁用' : '启用'}</span>
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => window.open('/endpoints', '_blank')}
                    >
                      <ExternalLink className="h-3 w-3" />
                    </Button>
                  </div>
                </div>
              ))
            ) : (
              <div className="col-span-full text-center text-muted-foreground py-8">
                暂无端点数据
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}