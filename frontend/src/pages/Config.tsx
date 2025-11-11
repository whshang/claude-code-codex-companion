import { useState, useEffect } from 'react'
import { wailsAPI } from '@/lib/wails-api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import {
  RefreshCw,
  Settings,
  Server,
  FileText,
  Shield,
  Download,
  Upload,
  Edit,
  Save,
  X,
  Activity,
  Plus,
  Trash2,
  Key,
} from 'lucide-react'
import { toast } from 'sonner'

// 简化的配置类型定义
interface SimpleConfig {
  server: {
    host: string
    port: number
    auto_sort_endpoints: boolean
    config_flush_interval?: string
    default_model?: string
    claude_code_auth_token?: string
  }
  logging: {
    level: string
    log_request_body?: string
    log_response_body?: string
  }
  health_check: {
    model: string
    max_tokens: number
    temperature: number
    timeout: string
  }
  blacklist: {
    enabled: boolean
    auto_blacklist: boolean
    business_error_safe: boolean
    config_error_safe: boolean
    server_error_safe: boolean
  }
  security: {
    cors: {
      enabled: boolean
      allowed_origins: string[]
    }
    rate_limit: {
      enabled: boolean
      requests_per_minute: number
    }
  }
}

const emptyConfig: SimpleConfig = {
  server: {
    host: '-',
    port: 0,
    auto_sort_endpoints: false,
    config_flush_interval: '-',
    claude_code_auth_token: ''
  },
  logging: {
    level: '-',
    log_request_body: '-',
    log_response_body: '-'
  },
  health_check: {
    model: 'claude-sonnet-4-20250929',
    max_tokens: 512,
    temperature: 0,
    timeout: '30s'
  },
  blacklist: {
    enabled: false,
    auto_blacklist: false,
    business_error_safe: false,
    config_error_safe: false,
    server_error_safe: false
  },
  security: {
    cors: {
      enabled: false,
      allowed_origins: []
    },
    rate_limit: {
      enabled: false,
      requests_per_minute: 0
    }
  }
}

export default function Config() {
  const [config, setConfig] = useState<SimpleConfig>(emptyConfig)
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [lastUpdate, setLastUpdate] = useState(new Date())
  const [editingConfig, setEditingConfig] = useState<SimpleConfig>(emptyConfig)
  const [claudeCodeAuthToken, setClaudeCodeAuthToken] = useState<string>('')
  const [tokenSaving, setTokenSaving] = useState(false)

  
  // 加载配置
  const loadConfig = async (showSuccessMessage = false) => {
    setLoading(true)
    try {
      const response = await wailsAPI.LoadConfig()
      console.log('Config response:', response)

      // 处理不同的响应格式
      let configData = response
      if (response && typeof response === 'object' && 'config' in response) {
        configData = (response as any).config
      }

      // 简化配置转换，只保留我们需要的字段
      const simplifiedConfig: SimpleConfig = {
        server: {
          host: (configData as any)?.server?.host || '-',
          port: (configData as any)?.server?.port || 0,
          auto_sort_endpoints: (configData as any)?.server?.auto_sort_endpoints || false,
          config_flush_interval: (configData as any)?.server?.config_flush_interval || '-',
          default_model: (configData as any)?.server?.default_model || 'claude-sonnet-4-20250929'
        },
        logging: {
          level: (configData as any)?.logging?.level || '-',
          log_request_body: (configData as any)?.logging?.log_request_body || '-',
          log_response_body: (configData as any)?.logging?.log_response_body || '-'
        },
        health_check: {
          model: (configData as any)?.health_check?.model || 'claude-sonnet-4-20250929',
          max_tokens: (configData as any)?.health_check?.max_tokens || 512,
          temperature: (configData as any)?.health_check?.temperature || 0,
          timeout: (configData as any)?.timeouts?.health_check_timeout || '30s'
        },
        blacklist: {
          enabled: (configData as any)?.blacklist?.enabled || false,
          auto_blacklist: (configData as any)?.blacklist?.auto_blacklist || false,
          business_error_safe: (configData as any)?.blacklist?.business_error_safe || false,
          config_error_safe: (configData as any)?.blacklist?.config_error_safe || false,
          server_error_safe: (configData as any)?.blacklist?.server_error_safe || false
        },
        security: {
          cors: {
            enabled: (configData as any)?.security?.cors?.enabled || false,
            allowed_origins: (configData as any)?.security?.cors?.allowed_origins || []
          },
          rate_limit: {
            enabled: (configData as any)?.security?.rate_limit?.enabled || false,
            requests_per_minute: (configData as any)?.security?.rate_limit?.requests_per_minute || 0
          }
        }
      }

      setConfig(simplifiedConfig)
      // 只有手动刷新时才显示成功提示
      if (showSuccessMessage) {
        toast.success('配置加载成功')
      }
    } catch (error) {
      console.error('Failed to load config:', error)
      toast.error('加载配置失败: ' + (error instanceof Error ? error.message : '未知错误'))
      setConfig(emptyConfig)
    } finally {
      setLoading(false)
      setLastUpdate(new Date())
    }
  }

  // 加载Claude Code认证token
  const loadClaudeCodeAuthToken = async () => {
    try {
      const token = await wailsAPI.GetClaudeCodeAuthToken()
      setClaudeCodeAuthToken(token || 'hello') // 默认显示hello
    } catch (error) {
      console.error('Failed to load Claude Code auth token:', error)
      setClaudeCodeAuthToken('hello') // 默认值
    }
  }

  // 保存Claude Code认证token
  const saveClaudeCodeAuthToken = async (newToken: string) => {
    setTokenSaving(true)
    try {
      const result = await wailsAPI.SetClaudeCodeAuthToken(newToken)
      if (result.success) {
        setClaudeCodeAuthToken(newToken)
        toast.success('Claude Code认证token已更新')
      } else {
        toast.error('更新失败: ' + result.error)
      }
    } catch (error) {
      console.error('Failed to save Claude Code auth token:', error)
      toast.error('更新认证token失败: ' + (error instanceof Error ? error.message : '未知错误'))
    } finally {
      setTokenSaving(false)
    }
  }

  // 初始加载
  useEffect(() => {
    loadConfig()
    loadClaudeCodeAuthToken()
  }, [])

  // 导出配置
  const exportConfig = async () => {
    try {
      const dataStr = JSON.stringify(config, null, 2)
      const dataBlob = new Blob([dataStr], { type: 'application/json' })
      const url = URL.createObjectURL(dataBlob)
      const link = document.createElement('a')
      link.href = url
      link.download = 'config.json'
      link.click()
      URL.revokeObjectURL(url)
      toast.success('配置导出成功')
    } catch (error) {
      console.error('Failed to export config:', error)
      toast.error('导出配置失败')
    }
  }

  // 开始编辑配置
  const startEditing = () => {
    setEditingConfig({ ...config })
    setEditing(true)
  }

  // 取消编辑
  const cancelEditing = () => {
    setEditingConfig({ ...config })
    setEditing(false)
  }

  // 保存配置
  const saveConfig = async () => {
    setSaving(true)
    try {
      // 调用API保存配置
      await wailsAPI.SaveConfig(editingConfig)

      // 更新本地状态
      setConfig({ ...editingConfig })
      setEditing(false)
      setLastUpdate(new Date())

      toast.success('配置保存成功')
    } catch (error) {
      console.error('Failed to save config:', error)
      toast.error('保存配置失败: ' + (error instanceof Error ? error.message : '未知错误'))
    } finally {
      setSaving(false)
    }
  }

  // 更新编辑中的配置
  const updateEditingConfig = (section: keyof SimpleConfig, key: string, value: any) => {
    setEditingConfig(prev => ({
      ...prev,
      [section]: {
        ...prev[section],
        [key]: value
      }
    }))
  }

  const formatBoolean = (value: boolean) => {
    return value ? '启用' : '禁用'
  }

  return (
    <div className="space-y-6">
      {/* 页面标题和操作按钮 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">配置管理</h1>
          <p className="text-muted-foreground">
            查看和管理系统配置参数
          </p>
        </div>
        <div className="flex items-center space-x-2">
          {editing ? (
            <>
              <Button onClick={saveConfig} disabled={saving} variant="default">
                <Save className="w-4 h-4 mr-2" />
                {saving ? '保存中...' : '保存'}
              </Button>
              <Button onClick={cancelEditing} variant="outline" disabled={saving}>
                <X className="w-4 h-4 mr-2" />
                取消
              </Button>
            </>
          ) : (
            <>
              <Button onClick={startEditing} variant="default">
                <Edit className="w-4 h-4 mr-2" />
                编辑配置
              </Button>
              <Button onClick={exportConfig} variant="outline">
                <Download className="w-4 h-4 mr-2" />
                导出配置
              </Button>
              <Button onClick={() => loadConfig(true)} disabled={loading}>
                <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
                刷新
              </Button>
            </>
          )}
        </div>
      </div>

      <Tabs defaultValue="basic" className="space-y-6">
        <TabsList className="grid w-full grid-cols-3">
          <TabsTrigger value="basic">基本配置</TabsTrigger>
          <TabsTrigger value="health">健康检查</TabsTrigger>
          <TabsTrigger value="system">系统配置</TabsTrigger>
        </TabsList>

        {/* 基本配置标签页 */}
        <TabsContent value="basic" className="space-y-6">
          {/* 服务器配置 */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Server className="w-5 h-5 mr-2" />
                服务器配置
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
                <div>
                  <Label className="text-sm text-muted-foreground">监听地址</Label>
                  {editing ? (
                    <Input
                      value={editingConfig.server.host}
                      onChange={(e) => updateEditingConfig('server', 'host', e.target.value)}
                      placeholder="0.0.0.0"
                      className="mt-1"
                    />
                  ) : (
                    <p className="font-medium">{config.server.host}</p>
                  )}
                </div>
                <div>
                  <Label className="text-sm text-muted-foreground">端口</Label>
                  {editing ? (
                    <Input
                      type="number"
                      value={editingConfig.server.port}
                      onChange={(e) => updateEditingConfig('server', 'port', parseInt(e.target.value) || 0)}
                      placeholder="8080"
                      className="mt-1"
                      min="1"
                      max="65535"
                    />
                  ) : (
                    <p className="font-medium">{config.server.port}</p>
                  )}
                </div>
                <div>
                  <Label className="text-sm text-muted-foreground">默认模型</Label>
                  {editing ? (
                    <Input
                      value={editingConfig.server.default_model}
                      onChange={(e) => updateEditingConfig('server', 'default_model', e.target.value)}
                      placeholder="claude-sonnet-4-20250929"
                      className="mt-1"
                    />
                  ) : (
                    <p className="font-medium">{config.server.default_model}</p>
                  )}
                </div>
                <div>
                  <Label className="text-sm text-muted-foreground">自动排序端点</Label>
                  {editing ? (
                    <div className="flex items-center h-10 mt-1">
                      <Switch
                        checked={editingConfig.server.auto_sort_endpoints}
                        onCheckedChange={(checked: boolean) => updateEditingConfig('server', 'auto_sort_endpoints', checked)}
                      />
                    </div>
                  ) : (
                    <p className="font-medium">{formatBoolean(config.server.auto_sort_endpoints)}</p>
                  )}
                </div>
                <div>
                  <Label className="text-sm text-muted-foreground">配置刷新间隔</Label>
                  {editing ? (
                    <Input
                      value={editingConfig.server.config_flush_interval}
                      onChange={(e) => updateEditingConfig('server', 'config_flush_interval', e.target.value)}
                      placeholder="5m"
                      className="mt-1"
                    />
                  ) : (
                    <p className="font-medium">{config.server.config_flush_interval}</p>
                  )}
                </div>
              </div>

              <Separator />

              {/* Claude Code认证设置 */}
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <Label className="text-base font-medium">Claude Code 认证Token</Label>
                    <p className="text-sm text-muted-foreground mt-1">
                      用于Claude Code客户端认证的自定义token，留空使用默认值"hello"
                    </p>
                  </div>
                </div>
                <div className="flex items-center space-x-3">
                  <div className="flex-1 max-w-md">
                    <Input
                      type="password"
                      value={claudeCodeAuthToken === 'hello' ? '' : claudeCodeAuthToken}
                      onChange={(e) => {
                        const newToken = e.target.value
                        if (newToken === '') {
                          // 如果输入为空，恢复默认值
                          saveClaudeCodeAuthToken('hello')
                        } else {
                          setClaudeCodeAuthToken(newToken)
                        }
                      }}
                      placeholder={claudeCodeAuthToken === 'hello' ? '使用默认值 (hello)' : '输入自定义认证token'}
                      className="font-mono"
                      disabled={tokenSaving}
                    />
                  </div>
                  <div className="flex items-center space-x-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => saveClaudeCodeAuthToken(claudeCodeAuthToken)}
                      disabled={tokenSaving || claudeCodeAuthToken === ''}
                    >
                      {tokenSaving ? (
                        <>
                          <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                          保存中...
                        </>
                      ) : (
                        <>
                          <Save className="w-4 h-4 mr-2" />
                          应用
                        </>
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        setClaudeCodeAuthToken('hello')
                        saveClaudeCodeAuthToken('hello')
                      }}
                      disabled={tokenSaving}
                    >
                      重置为默认
                    </Button>
                  </div>
                </div>
                <div className="text-xs text-muted-foreground">
                  当前值: {claudeCodeAuthToken === 'hello' ?
                    <span className="font-mono bg-muted px-2 py-1 rounded">hello (默认)</span> :
                    <span className="font-mono bg-muted px-2 py-1 rounded">{claudeCodeAuthToken}</span>
                  }
                  {claudeCodeAuthToken !== 'hello' && (
                    <span className="ml-2 text-green-600">✓ 自定义token已启用</span>
                  )}
                </div>
              </div>
            </CardContent>
          </Card>

  
          {/* 日志配置 */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <FileText className="w-5 h-5 mr-2" />
                日志配置
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div>
                  <Label className="text-sm text-muted-foreground">日志级别</Label>
                  {editing ? (
                    <Select
                      value={editingConfig.logging.level}
                      onValueChange={(value: string) => updateEditingConfig('logging', 'level', value)}
                    >
                      <SelectTrigger className="mt-1">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="debug">Debug</SelectItem>
                        <SelectItem value="info">Info</SelectItem>
                        <SelectItem value="warn">Warn</SelectItem>
                        <SelectItem value="error">Error</SelectItem>
                      </SelectContent>
                    </Select>
                  ) : (
                    <p className="font-medium">{config.logging.level}</p>
                  )}
                </div>
                <div>
                  <Label className="text-sm text-muted-foreground">请求体日志</Label>
                  {editing ? (
                    <Select
                      value={editingConfig.logging.log_request_body}
                      onValueChange={(value: string) => updateEditingConfig('logging', 'log_request_body', value)}
                    >
                      <SelectTrigger className="mt-1">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="none">不记录</SelectItem>
                        <SelectItem value="truncated">截断</SelectItem>
                        <SelectItem value="full">完整</SelectItem>
                      </SelectContent>
                    </Select>
                  ) : (
                    <p className="font-medium">{config.logging.log_request_body}</p>
                  )}
                </div>
                <div>
                  <Label className="text-sm text-muted-foreground">响应体日志</Label>
                  {editing ? (
                    <Select
                      value={editingConfig.logging.log_response_body}
                      onValueChange={(value: string) => updateEditingConfig('logging', 'log_response_body', value)}
                    >
                      <SelectTrigger className="mt-1">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="none">不记录</SelectItem>
                        <SelectItem value="truncated">截断</SelectItem>
                        <SelectItem value="full">完整</SelectItem>
                      </SelectContent>
                    </Select>
                  ) : (
                    <p className="font-medium">{config.logging.log_response_body}</p>
                  )}
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* 健康检查配置标签页 */}
        <TabsContent value="health" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Activity className="w-5 h-5 mr-2" />
                健康检查配置
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="healthModel">默认检查模型</Label>
                  <Select
                    value={config?.health_check?.model || 'claude-sonnet-4-20250929'}
                    onValueChange={(value) => {
                      if (config) {
                        setConfig({
                          ...config,
                          health_check: {
                            ...config.health_check,
                            model: value
                          }
                        })
                      }
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择默认模型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="claude-sonnet-4-20250929">Claude Sonnet 4 (20250929)</SelectItem>
                      <SelectItem value="claude-3-5-sonnet-20241022">Claude 3.5 Sonnet</SelectItem>
                      <SelectItem value="claude-3-haiku-20240307">Claude 3 Haiku</SelectItem>
                      <SelectItem value="gpt-4">GPT-4</SelectItem>
                      <SelectItem value="gpt-4-turbo">GPT-4 Turbo</SelectItem>
                      <SelectItem value="gpt-5">GPT-5</SelectItem>
                    </SelectContent>
                  </Select>
                  <p className="text-sm text-muted-foreground">
                    选择用于端点健康检查的默认模型，建议选择广泛支持的模型
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="healthMaxTokens">最大令牌数</Label>
                  <Input
                    id="healthMaxTokens"
                    type="number"
                    value={config?.health_check?.max_tokens || 512}
                    onChange={(e) => {
                      if (config) {
                        setConfig({
                          ...config,
                          health_check: {
                            ...config.health_check,
                            max_tokens: parseInt(e.target.value) || 512
                          }
                        })
                      }
                    }}
                    min="1"
                    max="4096"
                  />
                  <p className="text-sm text-muted-foreground">
                    健康检查请求的最大令牌数，较小值可减少成本
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="healthTemperature">Temperature</Label>
                  <Input
                    id="healthTemperature"
                    type="number"
                    step="0.1"
                    value={config?.health_check?.temperature || 0}
                    onChange={(e) => {
                      if (config) {
                        setConfig({
                          ...config,
                          health_check: {
                            ...config.health_check,
                            temperature: parseFloat(e.target.value) || 0
                          }
                        })
                      }
                    }}
                    min="0"
                    max="2"
                  />
                  <p className="text-sm text-muted-foreground">
                    健康检查的随机性参数，建议使用0确保一致性
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="healthTimeout">超时时间</Label>
                  <Select
                    value={config?.health_check?.timeout || '30s'}
                    onValueChange={(value) => {
                      if (config) {
                        setConfig({
                          ...config,
                          health_check: {
                            ...config.health_check,
                            timeout: value
                          }
                        })
                      }
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择超时时间" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="10s">10秒</SelectItem>
                      <SelectItem value="30s">30秒</SelectItem>
                      <SelectItem value="60s">1分钟</SelectItem>
                      <SelectItem value="120s">2分钟</SelectItem>
                    </SelectContent>
                  </Select>
                  <p className="text-sm text-muted-foreground">
                    单次健康检查请求的超时时间
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* 系统配置标签页 */}
        <TabsContent value="system" className="space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* 黑名单配置 */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Shield className="w-5 h-5 mr-2" />
                  黑名单配置
                </CardTitle>
                <CardDescription>
                  端点拉黑和故障转移设置
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-1 gap-4">
                  <div className="flex items-center justify-between">
                    <Label>启用黑名单功能</Label>
                    {editing ? (
                      <Switch
                        checked={editingConfig.blacklist.enabled}
                        onCheckedChange={(checked: boolean) => updateEditingConfig('blacklist', 'enabled', checked)}
                      />
                    ) : (
                      <Badge variant={config.blacklist.enabled ? "default" : "secondary"}>
                        {formatBoolean(config.blacklist.enabled)}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center justify-between">
                    <Label>自动拉黑失败端点</Label>
                    {editing ? (
                      <Switch
                        checked={editingConfig.blacklist.auto_blacklist}
                        onCheckedChange={(checked: boolean) => updateEditingConfig('blacklist', 'auto_blacklist', checked)}
                      />
                    ) : (
                      <Badge variant={config.blacklist.auto_blacklist ? "default" : "secondary"}>
                        {formatBoolean(config.blacklist.auto_blacklist)}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center justify-between">
                    <Label>业务错误安全模式</Label>
                    {editing ? (
                      <Switch
                        checked={editingConfig.blacklist.business_error_safe}
                        onCheckedChange={(checked: boolean) => updateEditingConfig('blacklist', 'business_error_safe', checked)}
                      />
                    ) : (
                      <Badge variant={config.blacklist.business_error_safe ? "default" : "secondary"}>
                        {formatBoolean(config.blacklist.business_error_safe)}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center justify-between">
                    <Label>配置错误安全模式</Label>
                    {editing ? (
                      <Switch
                        checked={editingConfig.blacklist.config_error_safe}
                        onCheckedChange={(checked: boolean) => updateEditingConfig('blacklist', 'config_error_safe', checked)}
                      />
                    ) : (
                      <Badge variant={config.blacklist.config_error_safe ? "default" : "secondary"}>
                        {formatBoolean(config.blacklist.config_error_safe)}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center justify-between">
                    <Label>服务器错误安全模式</Label>
                    {editing ? (
                      <Switch
                        checked={editingConfig.blacklist.server_error_safe}
                        onCheckedChange={(checked: boolean) => updateEditingConfig('blacklist', 'server_error_safe', checked)}
                      />
                    ) : (
                      <Badge variant={config.blacklist.server_error_safe ? "default" : "secondary"}>
                        {formatBoolean(config.blacklist.server_error_safe)}
                      </Badge>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* 安全配置 */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Shield className="w-5 h-5 mr-2" />
                  安全配置
                </CardTitle>
                <CardDescription>
                  CORS 和访问控制设置
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-4">
                  <div>
                    <Label className="text-sm text-muted-foreground">CORS 跨域设置</Label>
                    <div className="mt-2">
                      <Badge variant={config.security.cors.enabled ? "default" : "secondary"}>
                        {formatBoolean(config.security.cors.enabled)}
                      </Badge>
                    </div>
                    {config.security.cors.enabled && config.security.cors.allowed_origins.length > 0 && (
                      <div className="mt-2">
                        <p className="text-sm text-muted-foreground mb-1">允许的源地址:</p>
                        <div className="flex flex-wrap gap-1">
                          {config.security.cors.allowed_origins.map((origin, index) => (
                            <Badge key={index} variant="outline" className="text-xs">
                              {origin}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>

                  <Separator />

                  <div>
                    <Label className="text-sm text-muted-foreground">速率限制</Label>
                    <div className="mt-2">
                      <Badge variant={config.security.rate_limit.enabled ? "default" : "secondary"}>
                        {formatBoolean(config.security.rate_limit.enabled)}
                      </Badge>
                      {config.security.rate_limit.enabled && (
                        <p className="text-sm mt-1">
                          每分钟 {config.security.rate_limit.requests_per_minute} 请求
                        </p>
                      )}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>

      {/* 页面底部信息 */}
      <div className="text-sm text-muted-foreground">
        最后更新: {lastUpdate.toLocaleString('zh-CN')}
      </div>
    </div>
  )
}