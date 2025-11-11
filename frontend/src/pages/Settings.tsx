import { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import { Badge } from '@/components/ui/badge'
import {
  RefreshCw,
  Server,
  Folder,
  ExternalLink,
  Monitor,
  Power,
  Info,
  Shield
} from 'lucide-react'
import { wailsAPI, isWailsEnvironment } from '@/lib/wails-api'
import { toast } from 'sonner'
import { getGlobalDebugConsole } from '@/components/DebugConsole'

// Settings form schema
const settingsSchema = z.object({
  // Server settings
  serverHost: z.string().min(1, '主机地址不能为空'),
  serverPort: z.number().min(1).max(65535),

  // Logging settings
  logLevel: z.enum(['debug', 'info', 'warn', 'error']),
  logRequestTypes: z.enum(['all', 'failed', 'success']),
  logRequestBody: z.enum(['none', 'truncated', 'full']),
  logResponseBody: z.enum(['none', 'truncated', 'full']),
  logDirectory: z.string().min(1, '日志目录不能为空'),

  // Blacklist settings
  blacklistEnabled: z.boolean(),
  autoBlacklist: z.boolean(),
  businessErrorSafe: z.boolean(),
  configErrorSafe: z.boolean(),
  serverErrorSafe: z.boolean(),

  // Debug settings
  debugConsoleEnabled: z.boolean(),
})

type SettingsFormValues = z.infer<typeof settingsSchema>

const defaultValues: SettingsFormValues = {
  // Server settings - Wails桌面应用内置代理服务器端口
  serverHost: '127.0.0.1',
  serverPort: 8080, // 内置HTTP代理服务器端口，用于处理API请求

  // Logging settings
  logLevel: 'info',
  logRequestTypes: 'all',
  logRequestBody: 'truncated',
  logResponseBody: 'truncated',
  logDirectory: './logs',

  // Blacklist settings
  blacklistEnabled: true,
  autoBlacklist: true,
  businessErrorSafe: true,
  configErrorSafe: false,
  serverErrorSafe: false,

  // Debug settings
  debugConsoleEnabled: false,
}

export function Settings() {
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [serverStatus, setServerStatus] = useState<any>(null)
  const [restarting, setRestarting] = useState(false)
  const [configPath, setConfigPath] = useState<string>('')
  const [settingsLoaded, setSettingsLoaded] = useState(false)

  const form = useForm<SettingsFormValues>({
    resolver: zodResolver(settingsSchema),
    defaultValues,
  })

  // Watch blacklistEnabled to control other switches
  const blacklistEnabled = form.watch('blacklistEnabled')
  // Use a fallback to default value to avoid delay
  const shouldShowBlacklistOptions = blacklistEnabled ?? defaultValues.blacklistEnabled

  useEffect(() => {
    const loadSettings = async () => {
      setLoading(true)
      try {
        // 暂时绕过有问题的 LoadConfig 调用，直接使用默认配置
        // TODO: 修复 LoadConfig 方法的卡死问题
        console.log('Using default settings due to LoadConfig timeout issue')

        const settings = {
          server: {
            host: "127.0.0.1",
            port: 8080, // 内置代理服务器端口，与Wails架构分离
            auto_sort_endpoints: false,
            default_model: "claude-sonnet-4-20250514"
          },
          logging: {
            level: "info",
            log_request_types: "all", // 请求日志类型：记录所有请求
            log_request_body: "truncated", // 请求体日志：截断显示
            log_response_body: "truncated" // 响应体日志：截断显示
          },
          blacklist: {
            enabled: true,
            auto_blacklist: true,
            business_error_safe: true,
            config_error_safe: false,
            server_error_safe: false
          },
          proxy: {
            timeout: 30000,
            max_retries: 3,
            retry_delay: 1000
          },
          debug: {
            console_enabled: false
          },
          database: {
            backup_enabled: true,
            cleanup_days: 30
          }
        }

        // 将后端的嵌套配置映射到前端的平级表单字段
        const mappedSettings = {
          // Server settings
          serverHost: settings.server?.host || defaultValues.serverHost,
          serverPort: typeof settings.server?.port === 'number' ? settings.server.port : (typeof settings.server?.port === 'string' ? parseInt(settings.server.port) : defaultValues.serverPort),

          // Logging settings
          logLevel: (settings.logging?.level as "error" | "info" | "warn" | "debug") || defaultValues.logLevel,
          logRequestTypes: (settings.logging?.log_request_types as "all" | "success" | "failed") || defaultValues.logRequestTypes,
          logRequestBody: (settings.logging?.log_request_body as "none" | "truncated" | "full") || defaultValues.logRequestBody,
          logResponseBody: (settings.logging?.log_response_body as "none" | "truncated" | "full") || defaultValues.logResponseBody,
          logDirectory: defaultValues.logDirectory, // 使用默认值，因为配置中没有这个字段

          // Blacklist settings
          blacklistEnabled: settings.blacklist?.enabled || defaultValues.blacklistEnabled,
          autoBlacklist: settings.blacklist?.auto_blacklist || defaultValues.autoBlacklist,
          businessErrorSafe: settings.blacklist?.business_error_safe || defaultValues.businessErrorSafe,
          configErrorSafe: settings.blacklist?.config_error_safe || defaultValues.configErrorSafe,
          serverErrorSafe: settings.blacklist?.server_error_safe || defaultValues.serverErrorSafe,

          // Debug settings
          debugConsoleEnabled: settings.debug?.console_enabled || defaultValues.debugConsoleEnabled,
        }

        console.log('Mapped settings for form:', mappedSettings)
        form.reset(mappedSettings)
      } catch (error) {
        console.error('Failed to load settings:', error)
        // Keep default values on error
        form.reset(defaultValues)
      } finally {
        setLoading(false)
        setSettingsLoaded(true)
      }
    }

    // 加载桌面特性数据
    const loadDesktopData = async () => {
      if (isWailsEnvironment()) {
        try {
          const [status, path] = await Promise.all([
            wailsAPI.GetServerStatus(),
            wailsAPI.GetConfigPath()
          ])
          setServerStatus(status)
          setConfigPath(path)
        } catch (error) {
          console.error('Failed to load desktop data:', error)
          // 设置默认状态以避免显示问题
          setServerStatus({ running: false, host: '127.0.0.1', port: 8080 })
        }
      }
    }

    loadSettings()
    loadDesktopData()

    // 定期刷新服务器状态 - 与Layout.tsx保持一致的3秒间隔
    if (isWailsEnvironment()) {
      const interval = setInterval(async () => {
        try {
          const status = await wailsAPI.GetServerStatus()
          setServerStatus(status)
        } catch (error) {
          console.error('Failed to refresh server status:', error)
          // 设置错误状态，避免UI显示异常
          setServerStatus({ running: false, host: '127.0.0.1', port: 8080 })
        }
      }, 3000) // 改为3秒，与Layout.tsx保持一致

      return () => clearInterval(interval)
    }
  }, [form])

  const onSubmit = async (data: SettingsFormValues) => {
    setSaving(true)
    try {
      // 将前端的平级字段映射为后端的嵌套结构
      const backendConfig = {
        server: {
          host: data.serverHost,
          port: data.serverPort,
          auto_sort_endpoints: false,
        },
        logging: {
          level: data.logLevel,
          request_types: data.logRequestTypes,
          request_body: data.logRequestBody,
          response_body: data.logResponseBody,
          directory: data.logDirectory,
        },
        blacklist: {
          enabled: data.blacklistEnabled,
          auto_blacklist: data.autoBlacklist,
          business_error_safe: data.businessErrorSafe,
          config_error_safe: data.configErrorSafe,
          server_error_safe: data.serverErrorSafe,
        },
        debug: {
          console_enabled: data.debugConsoleEnabled,
        },
      }

      await wailsAPI.SaveConfig(backendConfig)
      toast.success('设置保存成功')

      // 如果调试控制台设置发生变化，更新全局状态
      const debugConsole = getGlobalDebugConsole()
      debugConsole.setDebugEnabled(data.debugConsoleEnabled)
    } catch (error) {
      console.error('Failed to save settings:', error)
      toast.error('设置保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleReset = () => {
    form.reset(defaultValues)
    toast.info('设置已重置为默认值')
  }

  // 桌面特性处理函数
  const handleRestartServer = async () => {
    setRestarting(true)
    try {
      const result = await wailsAPI.RestartServer()
      toast.success(result)

      // 刷新服务器状态
      const status = await wailsAPI.GetServerStatus()
      setServerStatus(status)
    } catch (error) {
      console.error('Failed to restart server:', error)
      toast.error('服务器重启失败')
    } finally {
      setRestarting(false)
    }
  }

  const handleOpenConfigDirectory = async () => {
    try {
      await wailsAPI.OpenConfigDirectory()
      toast.success('配置目录已打开')
    } catch (error) {
      console.error('Failed to open config directory:', error)
      toast.error('打开配置目录失败')
    }
  }

  if (loading) {
    return <div className="flex items-center justify-center h-64">加载中...</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">应用设置</h1>
          <p className="text-muted-foreground">
            配置代理服务参数和拉黑策略
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleReset}>
            重置
          </Button>
          <Button onClick={form.handleSubmit(onSubmit)} disabled={saving}>
            {saving ? '保存中...' : '保存配置'}
          </Button>
        </div>
      </div>

  
      {/* 桌面特性监控卡片 - 仅在Wails环境中显示 */}
      {isWailsEnvironment() && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {/* 服务器状态 */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">服务器状态</CardTitle>
              <Server className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <Badge
                    variant={serverStatus?.running ? "default" : "destructive"}
                    className="text-xs"
                  >
                    {serverStatus?.running ? "运行中" : "已停止"}
                  </Badge>
                </div>
                {serverStatus && (
                  <div className="text-sm text-muted-foreground">
                    {serverStatus.host}:{serverStatus.port}
                  </div>
                )}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleRestartServer}
                  disabled={restarting}
                  className="w-full mt-2"
                >
                  {restarting ? (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                      重启中...
                    </>
                  ) : (
                    <>
                      <Power className="mr-2 h-4 w-4" />
                      重启服务器
                    </>
                  )}
                </Button>
              </div>
            </CardContent>
          </Card>

          {/* 配置管理 */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">配置管理</CardTitle>
              <Folder className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                <div className="text-sm font-medium truncate" title={configPath}>
                  {configPath ? configPath.split('/').pop() : '配置文件'}
                </div>
                {configPath && (
                  <div className="text-xs text-muted-foreground truncate" title={configPath}>
                    {configPath}
                  </div>
                )}
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleOpenConfigDirectory}
                    className="flex-1"
                  >
                    <ExternalLink className="mr-2 h-4 w-4" />
                    打开目录
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* 应用信息 */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">应用信息</CardTitle>
              <Info className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">模式:</span>
                  <Badge variant="secondary">桌面版</Badge>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">版本:</span>
                  <span>v1.0.0</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">运行时:</span>
                  <span>Wails v2</span>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          {/* Server Settings */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle>基础配置</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <FormField
                  control={form.control}
                  name="serverHost"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>主机地址</FormLabel>
                      <FormControl>
                        <Input placeholder="0.0.0.0" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="serverPort"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>端口</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          min={1}
                          max={65535}
                          {...field}
                          onChange={(e) => field.onChange(parseInt(e.target.value))}
                        />
                      </FormControl>
                      <FormDescription>
                        Wails桌面应用内置HTTP代理服务器端口，用于处理API请求
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </CardContent>
          </Card>

          {/* Logging Settings */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle>日志配置</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <FormField
                  control={form.control}
                  name="logLevel"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>日志级别</FormLabel>
                      <Select onValueChange={field.onChange} defaultValue={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="选择日志级别" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="debug">Debug</SelectItem>
                          <SelectItem value="info">Info</SelectItem>
                          <SelectItem value="warn">Warn</SelectItem>
                          <SelectItem value="error">Error</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="logRequestTypes"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>记录请求类型</FormLabel>
                      <Select onValueChange={field.onChange} defaultValue={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="选择记录类型" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="all">全部</SelectItem>
                          <SelectItem value="failed">仅失败</SelectItem>
                          <SelectItem value="success">仅成功</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="logRequestBody"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>记录请求体</FormLabel>
                      <Select onValueChange={field.onChange} defaultValue={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="选择记录方式" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="none">不记录</SelectItem>
                          <SelectItem value="truncated">截断</SelectItem>
                          <SelectItem value="full">完整</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="logResponseBody"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>记录响应体</FormLabel>
                      <Select onValueChange={field.onChange} defaultValue={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="选择记录方式" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="none">不记录</SelectItem>
                          <SelectItem value="truncated">截断</SelectItem>
                          <SelectItem value="full">完整</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={form.control}
                name="logDirectory"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>日志目录</FormLabel>
                    <FormControl>
                      <Input placeholder="./logs" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </CardContent>
          </Card>

          {/* Blacklist Settings */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Shield className="w-5 h-5" />
                端点拉黑
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* 主开关 - 启用端点拉黑功能 */}
              <FormField
                control={form.control}
                name="blacklistEnabled"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4 bg-muted/30">
                    <div className="space-y-0.5">
                      <FormLabel className="text-base font-medium">启用端点拉黑功能</FormLabel>
                      <FormDescription>
                        开启后系统将根据错误类型自动拉黑问题端点
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              {shouldShowBlacklistOptions && (
                <div className="space-y-4">
                  
                  {/* 拉黑策略配置 */}
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="autoBlacklist"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                          <div className="space-y-0.5 flex-1">
                            <FormLabel className="text-base font-medium flex items-center gap-2">
                              自动拉黑失败端点
                              <Badge variant="secondary" className="text-xs">
                                主要策略
                              </Badge>
                            </FormLabel>
                            <FormDescription>
                              检测到错误时自动执行拉黑操作
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch
                              checked={field.value}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="businessErrorSafe"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                          <div className="space-y-0.5 flex-1">
                            <FormLabel className="text-base font-medium flex items-center gap-2">
                              业务错误不触发拉黑
                              <Badge variant="outline" className="text-xs text-orange-600">
                                推荐
                              </Badge>
                            </FormLabel>
                            <FormDescription>
                              业务逻辑错误不应触发拉黑
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch
                              checked={field.value}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="configErrorSafe"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                          <div className="space-y-0.5 flex-1">
                            <FormLabel className="text-base font-medium flex items-center gap-2">
                              配置错误不触发拉黑
                              <Badge variant="outline" className="text-xs">
                                调试模式
                              </Badge>
                            </FormLabel>
                            <FormDescription>
                              配置问题不应触发拉黑
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch
                              checked={field.value}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="serverErrorSafe"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                          <div className="space-y-0.5 flex-1">
                            <FormLabel className="text-base font-medium flex items-center gap-2">
                              服务器错误不触发拉黑
                              <Badge variant="outline" className="text-xs">
                                开发环境
                              </Badge>
                            </FormLabel>
                            <FormDescription>
                              服务端问题不应触发拉黑
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch
                              checked={field.value}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                  </div>
                </div>
              )}

              {settingsLoaded && !blacklistEnabled && (
                <div className="text-center py-8 text-muted-foreground">
                  <Shield className="w-12 h-12 mx-auto mb-4 opacity-50" />
                  <p className="text-sm">
                    启用端点拉黑功能后，下方配置选项将可用
                  </p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Debug Settings */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Monitor className="w-5 h-5" />
                调试功能
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <FormField
                control={form.control}
                name="debugConsoleEnabled"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4 bg-muted/30">
                    <div className="space-y-0.5">
                      <FormLabel className="text-base font-medium">启用调试控制台</FormLabel>
                      <FormDescription>
                        开启后将在右下角显示调试控制台，用于监控端点测试和API调用的详细信息
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </CardContent>
          </Card>
        </form>
      </Form>
    </div>
  )
}