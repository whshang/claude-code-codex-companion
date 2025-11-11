import React, { useState, useEffect } from "react"
import { useForm } from "react-hook-form"
import * as z from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { Plus, Edit, Trash2, Activity, CheckCircle, XCircle, AlertCircle, AlertTriangle, ArrowUpDown, RefreshCw, Save, X, Globe, Clock, Target, Shield, Settings } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { useToast } from "@/hooks/use-toast"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { getGlobalDebugConsole } from "@/components/DebugConsole"
import { wailsAPI } from "@/lib/wails-api"
import type {
  Endpoint as EndpointDTO,
  CreateEndpointParams,
  UpdateEndpointParams,
  OperationResult,
  EndpointTestResult,
  APIResult,
} from "@/types/api"

// 类型定义
type EndpointType = "anthropic" | "openai"
type AuthType = "api_key" | "bearer_token" | "basic_auth" | "none"
type EndpointStatus = "healthy" | "degraded" | "unhealthy"

type UIEndpoint = EndpointDTO & {
  endpoint_type: EndpointType
  type: EndpointType
  auth_type: AuthType
  status: EndpointStatus
  blacklisted: boolean
  blacklist_reason?: string
  success_rate?: number
  last_test_time?: string
  openai_preference?: "auto" | "responses" | "chat_completions"
  error_code?: string
  error_message?: string
}

const normalizeEndpoint = (endpoint: any): UIEndpoint => {
  const raw = endpoint ?? {}
  const endpointType = (raw.endpoint_type ?? raw.type ?? "anthropic") as EndpointType

  return {
    ...(raw as EndpointDTO),
    endpoint_type: endpointType,
    type: endpointType,
    auth_type: (raw.auth_type ?? "api_key") as AuthType,
    status: (raw.status ?? "healthy") as EndpointStatus,
    blacklisted: Boolean(raw.blacklisted),
    blacklist_reason: raw.blacklist_reason,
    success_rate: raw.success_rate,
    last_test_time: raw.last_test_time,
    openai_preference: raw.openai_preference ?? "auto",
    tags: Array.isArray(raw.tags) ? raw.tags : [],
  } as UIEndpoint
}

// Form schema
const endpointFormSchema = z.object({
  name: z.string().min(1, "端点名称不能为空"),
  url_anthropic: z.string().url("请输入有效的URL").optional().or(z.literal("")),
  url_openai: z.string().url("请输入有效的URL").optional().or(z.literal("")),
  auth_type: z.enum(["api_key", "bearer_token", "basic_auth", "none"]),
  auth_value: z.string().optional(),
  priority: z.number().min(1, "优先级必须大于0").max(100, "优先级不能超过100"),
  enabled: z.boolean(),
  tags: z.string().optional(),
  model_rewrite: z.object({
    enabled: z.boolean(),
    target_model: z.string().optional(),
    rules: z.array(z.object({
      source_pattern: z.string(),
      target_model: z.string()
    })).optional()
  }).optional(),
  target_model: z.string().optional(),
  openai_preference: z.enum(["auto", "responses", "chat_completions"]),
}).refine((data) => {
  // 至少需要填写一个URL
  return (data.url_anthropic && data.url_anthropic.trim() !== "") ||
         (data.url_openai && data.url_openai.trim() !== "")
}, {
  message: "至少需要填写一个URL（Anthropic URL 或 OpenAI URL）",
  path: ["url_anthropic"]
}).refine((data) => {
  // 当认证类型不是"none"时，认证值不能为空
  if (data.auth_type !== "none") {
    return data.auth_value && data.auth_value.trim() !== ""
  }
  return true
}, {
  message: "认证值不能为空",
  path: ["auth_value"]
})

type EndpointFormData = z.infer<typeof endpointFormSchema>

export default function Endpoints() {
  const [endpoints, setEndpoints] = useState<UIEndpoint[]>([])
  const [loading, setLoading] = useState(true)
  const [editingEndpoint, setEditingEndpoint] = useState<UIEndpoint | null>(null)
  const [showAddEndpointDialog, setShowAddEndpointDialog] = useState(false)
  const [testingEndpoints, setTestingEndpoints] = useState<Set<string>>(new Set())
  const [testingAll, setTestingAll] = useState(false)
  const [currentSortMode, setCurrentSortMode] = useState<string>("default")
  const [autoSortEndpoints, setAutoSortEndpoints] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [endpointToDelete, setEndpointToDelete] = useState<UIEndpoint | null>(null)
  const { toast } = useToast()

  const form = useForm<EndpointFormData>({
    resolver: zodResolver(endpointFormSchema) as any,
    defaultValues: {
      name: "",
      url_anthropic: "",
      url_openai: "",
      auth_type: "api_key",
      auth_value: "",
      priority: 1,
      enabled: true,
      tags: "",
      model_rewrite: {
        enabled: false,
        target_model: "",
        rules: []
      },
      target_model: "",
      openai_preference: "auto",
    },
  })

  // 加载端点数据
  useEffect(() => {
    loadEndpoints()
  }, [])

  const loadEndpoints = async () => {
    try {
      setLoading(true)
      const data = await wailsAPI.GetEndpoints()
      console.log("Raw data from GetEndpoints:", data)

      // 更灵活的数组检查
      if (data && Array.isArray(data)) {
        // 直接是数组
        const mappedEndpoints = data.map(normalizeEndpoint)
        setEndpoints(mappedEndpoints)
        console.log("Mapped endpoints:", mappedEndpoints)
      } else if (data && typeof data === 'object' && data !== null) {
        // 如果data是对象，尝试获取数组属性
        const dataObj = data as { endpoints?: unknown[], data?: unknown[], success?: boolean }
        const endpointArray = (dataObj.endpoints || dataObj.data || []) as unknown[]

        if (Array.isArray(endpointArray)) {
          const mappedEndpoints = endpointArray.map(normalizeEndpoint)
          setEndpoints(mappedEndpoints)
          console.log("Mapped endpoints:", mappedEndpoints)
        } else {
          console.error("Endpoints data is not an array:", endpointArray)
          toast({
            title: "加载失败",
            description: "返回的数据格式不正确：期望数组格式",
            variant: "destructive",
          })
        }
      } else {
        console.error("Invalid endpoints data:", data)
        toast({
          title: "加载失败",
          description: "返回的数据格式不正确：无效数据",
          variant: "destructive",
        })
      }
    } catch (error) {
      console.error("Failed to load endpoints:", error)
      toast({
        title: "加载失败",
        description: "无法连接到后端服务",
        variant: "destructive",
      })
    } finally {
      setLoading(false)
    }
  }

  
  // 根据填写的URL自动判断端点类型
  const determineEndpointType = (url_anthropic: string, url_openai: string): EndpointType => {
    const hasAnthropic = url_anthropic && url_anthropic.trim() !== ""
    const hasOpenAI = url_openai && url_openai.trim() !== ""

    if (hasAnthropic && hasOpenAI) {
      return "openai" // 如果两个URL都填写，默认为OpenAI类型（支持双URL）
    } else if (hasAnthropic) {
      return "anthropic"
    } else if (hasOpenAI) {
      return "openai"
    } else {
      return "anthropic" // 默认值
    }
  }

  const handleSubmit = async (data: EndpointFormData) => {
    try {
      const endpointType = determineEndpointType(data.url_anthropic || "", data.url_openai || "")

      // 处理模型重写数据：将target_model转换为通配符规则
      let rules = data.model_rewrite?.rules || []

      // 严格使用表单中的target_model，确保用户清空输入框时空值被正确处理
      const targetModel = data.target_model || ""

      // 如果有target_model但没有通配符规则，则添加通配符规则
      if (targetModel && !rules.some(rule => rule.source_pattern === "*")) {
        rules = [{ source_pattern: "*", target_model: targetModel }, ...rules]
      }

      const modelRewriteData = {
        enabled: data.model_rewrite?.enabled || targetModel || rules.length > 0,
        target_model: targetModel, // 严格使用表单中的target_model
        rules: rules
      }

      const endpointData = {
        ...data,
        type: endpointType,
        endpoint_type: endpointType,
        tags: data.tags ? data.tags.split(",").map(tag => tag.trim()).filter(tag => tag) : [],
        target_model: targetModel, // 使用处理后的target_model，保持一致性
        model_rewrite: modelRewriteData,
      }

      let result: OperationResult
      if (editingEndpoint) {
        result = await (wailsAPI.UpdateEndpoint(editingEndpoint.id, endpointData as UpdateEndpointParams) as Promise<OperationResult>)
      } else {
        result = await (wailsAPI.CreateEndpoint(endpointData as CreateEndpointParams) as Promise<OperationResult>)
      }

      if (result.success) {
        toast({
          title: editingEndpoint ? "更新成功" : "创建成功",
          description: `端点"${data.name}"已${editingEndpoint ? "更新" : "创建"}`,
        })
        setShowAddEndpointDialog(false)
        setEditingEndpoint(null)
        form.reset()
        loadEndpoints()
      } else {
        toast({
          title: editingEndpoint ? "更新失败" : "创建失败",
          description: result.message || "操作失败",
          variant: "destructive",
        })
      }
    } catch (error) {
      console.error("Failed to save endpoint:", error)
      toast({
        title: "操作失败",
        description: "无法连接到后端服务",
        variant: "destructive",
      })
    }
  }

  const handleEditEndpoint = (endpoint: UIEndpoint) => {
    setEditingEndpoint(endpoint)

    // 处理模型重写数据：将通配符规则从rules中提取到target_model字段用于显示
    let modelRewriteData = endpoint.model_rewrite || {
      enabled: false,
      target_model: "",
      rules: []
    }

    // 查找通配符规则 (*)，优先使用用户保存的target_model，而不是通配符规则
    let targetModelForDisplay = endpoint.target_model || ""

    // 从rules中移除通配符规则，因为要作为target_model显示
    let filteredRules = modelRewriteData.rules?.filter(rule => rule.source_pattern !== "*") || []

    form.reset({
      name: endpoint.name,
      url_anthropic: endpoint.url_anthropic || "",
      url_openai: endpoint.url_openai || "",
      auth_type: endpoint.auth_type,
      auth_value: endpoint.auth_value || "",
      priority: endpoint.priority,
      enabled: endpoint.enabled,
      tags: endpoint.tags.join(", "),
      target_model: targetModelForDisplay,
      openai_preference: endpoint.openai_preference || "auto",
      model_rewrite: {
        ...modelRewriteData,
        target_model: targetModelForDisplay,
        rules: filteredRules
      },
    })
    setShowAddEndpointDialog(true)
  }

  const handleDeleteEndpoint = (endpoint: UIEndpoint) => {
    setEndpointToDelete(endpoint)
    setShowDeleteDialog(true)
  }

  const confirmDeleteEndpoint = async () => {
    if (!endpointToDelete) return

    try {
      const result = await (wailsAPI.DeleteEndpoint(endpointToDelete.id) as Promise<OperationResult>)

      if (result.success) {
        toast({
          title: "删除成功",
          description: "端点已删除",
        })
        loadEndpoints()
      } else {
        toast({
          title: "删除失败",
          description: result.message || "删除失败",
          variant: "destructive",
        })
      }
    } catch (error) {
      console.error("Failed to delete endpoint:", error)
      toast({
        title: "删除失败",
        description: "无法连接到后端服务",
        variant: "destructive",
      })
    } finally {
      setShowDeleteDialog(false)
      setEndpointToDelete(null)
    }
  }

  const handleTestEndpoint = async (endpoint: UIEndpoint) => {
    try {
      setTestingEndpoints(prev => new Set(prev).add(endpoint.name))

      // 发送调试信息到调试控制台
      const debugConsole = getGlobalDebugConsole()
      debugConsole.addMessage(`开始测试端点: ${endpoint.name} (ID: ${endpoint.id})`)
      debugConsole.addMessage(`端点URL: ${endpoint.url_anthropic || endpoint.url_openai}`)
      debugConsole.addMessage(`端点类型: ${endpoint.type}`)
      debugConsole.addMessage(`认证类型: ${endpoint.auth_type}`)

      debugConsole.addMessage(`正在调用后端TestEndpoint API...`)
      const result = await (wailsAPI.TestEndpoint(endpoint.id) as Promise<EndpointTestResult>)
      debugConsole.addMessage(`API调用完成，收到响应`)
      debugConsole.addMessage(`测试结果: ${JSON.stringify(result, null, 2)}`)

      if (result.success) {
        toast({
          title: "测试完成",
          description: `端点"${endpoint.name}"测试成功`,
        })
        debugConsole.addMessage(`✅ 端点"${endpoint.name}"测试成功`)
        loadEndpoints()
      } else {
        toast({
          title: "测试失败",
          description: result.error || result.message || "测试失败",
          variant: "destructive",
        })
        debugConsole.addMessage(`❌ 端点"${endpoint.name}"测试失败: ${result.error || result.message}`)
      }
    } catch (error) {
      console.error("Failed to test endpoint:", error)
      const debugConsole = getGlobalDebugConsole()
      debugConsole.addMessage(`💥 测试异常: ${error instanceof Error ? error.message : '未知错误'}`)
      toast({
        title: "测试失败",
        description: "无法连接到后端服务",
        variant: "destructive",
      })
    } finally {
      setTestingEndpoints(prev => {
        const newSet = new Set(prev)
        newSet.delete(endpoint.name)
        return newSet
      })
    }
  }

  // 专门用于批量测试的函数，不包含UI状态更新和toast通知
  const handleTestEndpointForBatch = async (endpoint: UIEndpoint): Promise<void> => {
    try {
      const result = await (wailsAPI.TestEndpoint(endpoint.id) as Promise<EndpointTestResult>)

      if (!result.success) {
        throw new Error(result.error || result.message || "测试失败")
      }
    } catch (error) {
      throw error instanceof Error ? error : new Error("测试异常")
    }
  }

  const testAllEndpoints = async () => {
    try {
      setTestingAll(true)
      const enabledEndpoints = endpoints.filter(ep => ep.enabled && !ep.blacklisted)

      // 创建测试超时函数
      const testWithTimeout = async (endpoint: UIEndpoint, timeoutMs: number = 30000): Promise<{endpoint: UIEndpoint, success: boolean, error?: string}> => {
        try {
          const timeoutPromise = new Promise<{endpoint: UIEndpoint, success: boolean, error: string}>((_, reject) => {
            setTimeout(() => reject(new Error(`测试超时 (${timeoutMs}ms)`)), timeoutMs)
          })

          const testPromise = handleTestEndpointForBatch(endpoint)

          await Promise.race([testPromise, timeoutPromise])
          return {endpoint, success: true}
        } catch (error) {
          return {endpoint, success: false, error: error instanceof Error ? error.message : '未知错误'}
        }
      }

      // 并发测试所有端点
      const testPromises = enabledEndpoints.map(endpoint => testWithTimeout(endpoint))
      const results = await Promise.allSettled(testPromises)

      // 统计结果
      let successCount = 0
      let failureCount = 0
      const failedEndpoints: string[] = []

      results.forEach((result) => {
        if (result.status === 'fulfilled') {
          if (result.value.success) {
            successCount++
          } else {
            failureCount++
            failedEndpoints.push(`${result.value.endpoint.name}: ${result.value.error}`)
          }
        } else {
          failureCount++
          failedEndpoints.push(`测试异常: ${result.reason}`)
        }
      })

      // 刷新端点列表
      await loadEndpoints()

      // 显示结果
      if (failureCount === 0) {
        toast({
          title: "批量测试完成",
          description: `所有 ${successCount} 个端点测试成功`,
        })
      } else {
        toast({
          title: "批量测试完成",
          description: `成功: ${successCount}，失败: ${failureCount}`,
          variant: failureCount > 0 ? "destructive" : "default",
        })
        console.error("Failed endpoints:", failedEndpoints)
      }
    } catch (error) {
      console.error("Failed to test all endpoints:", error)
      toast({
        title: "批量测试失败",
        description: "批量测试过程中出现错误",
        variant: "destructive",
      })
    } finally {
      setTestingAll(false)
    }
  }

  const getEndpointUrl = (endpoint: UIEndpoint) => {
    return endpoint.url_anthropic || endpoint.url_openai || ""
  }

  const formatResponseTime = (endpoint: UIEndpoint) => {
    // 如果端点被拉黑，显示拉黑状态
    if (endpoint.blacklisted) {
      return "已拉黑"
    }

    // 如果端点禁用，显示禁用状态
    if (!endpoint.enabled) {
      return "已禁用"
    }

    // 根据端点状态显示不同内容
    switch (endpoint.status) {
      case "healthy":
        return endpoint.response_time ? `${endpoint.response_time}ms` : "-"
      case "degraded":
        return endpoint.response_time ? `${endpoint.response_time}ms` : "降级"
      case "unhealthy":
      default:
        // 显示错误码，如果没有具体错误信息则显示"不可用"
        return endpoint.error_code || "不可用"
    }
  }

  const getResponseTimeTooltip = (endpoint: UIEndpoint) => {
    // 如果端点被拉黑，显示拉黑原因
    if (endpoint.blacklisted && endpoint.blacklist_reason) {
      return `拉黑原因: ${endpoint.blacklist_reason}`
    }

    // 如果端点禁用，显示禁用提示
    if (!endpoint.enabled) {
      return "端点已被手动禁用"
    }

    // 根据端点状态显示不同tooltip
    switch (endpoint.status) {
      case "healthy":
        return endpoint.response_time ? `响应时间: ${endpoint.response_time}ms` : "端点状态良好"
      case "degraded":
        return endpoint.response_time
          ? `性能降级: ${endpoint.response_time}ms`
          : "端点性能降级，响应较慢"
      case "unhealthy":
      default:
        return endpoint.error_message || "端点不可用，请检查配置或网络连接"
    }
  }

  const formatSuccessRate = (endpoint: UIEndpoint) => {
    if (!endpoint.success_rate) return "-"
    return `${(endpoint.success_rate * 100).toFixed(1)}%`
  }

  const getSortModeText = () => {
    switch (currentSortMode) {
      case "name":
        return "按名称排序"
      case "status":
        return "按状态排序"
      case "responseTime":
        return "按响应时间排序"
      default:
        return "默认排序"
    }
  }

  const handleSortChange = (mode: string) => {
    setCurrentSortMode(mode)
  }

  const handleAutoSortToggle = (checked: boolean) => {
    setAutoSortEndpoints(checked)
    toast({
      title: checked ? "自动调整已启用" : "自动调整已禁用",
      description: checked
        ? "端点将根据响应时间自动调整优先级"
        : "端点优先级将保持固定",
    })
  }

  const filteredAndSortedEndpoints = () => {
    let filtered = [...endpoints]

    // 排序逻辑
    filtered.sort((a, b) => {
      switch (currentSortMode) {
        case "name":
          return a.name.localeCompare(b.name)
        case "status":
          const statusOrder = { healthy: 0, degraded: 1, unhealthy: 2 }
          return statusOrder[a.status] - statusOrder[b.status]
        case "responseTime":
          const aTime = a.response_time || Infinity
          const bTime = b.response_time || Infinity
          return aTime - bTime
        default:
          return a.priority - b.priority
      }
    })

    return filtered
  }

  return (
    <div className="space-y-6">
    <div className="flex items-center justify-between">
    <div>
    <h1 className="text-3xl font-bold tracking-tight">端点管理</h1>
    <p className="text-muted-foreground">
    管理API端点，配置认证信息和优先级
    </p>
    </div>
    <div className="flex gap-2">
      <Button
        variant="outline"
          onClick={testAllEndpoints}
            disabled={testingAll || endpoints.length === 0}
        >
          <Activity className="w-4 h-4 mr-2" />
          {testingAll ? "测试中..." : "批量测试"}
        </Button>
      <Button
      variant="outline"
      onClick={() => setShowAddEndpointDialog(true)}
    >
        <Plus className="w-4 h-4 mr-2" />
      添加端点
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
        <Button variant="outline">
          <ArrowUpDown className="w-4 h-4 mr-2" />
            {getSortModeText()}
        </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => handleSortChange("default")}>
          默认排序
        </DropdownMenuItem>
    <DropdownMenuItem onClick={() => handleSortChange("name")}>
      按名称排序
    </DropdownMenuItem>
    <DropdownMenuItem onClick={() => handleSortChange("status")}>
          按状态排序
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => handleSortChange("responseTime")}>
          <div className="flex items-center justify-between w-full">
            <span>按响应时间排序</span>
            {currentSortMode === "responseTime" && (
              <div className="flex items-center ml-2">
                <Switch
                  checked={autoSortEndpoints}
                  onCheckedChange={handleAutoSortToggle}
                />
                <span className="ml-2 text-xs text-muted-foreground">自动调整</span>
              </div>
            )}
          </div>
        </DropdownMenuItem>
    </DropdownMenuContent>
    </DropdownMenu>
    </div>
    </div>

      {/* 端点卡片列表 - 移除外围包装 */}
      <div className="space-y-4">
      {filteredAndSortedEndpoints().length === 0 ? (
          <div className="text-center py-12">
            <div className="flex flex-col items-center space-y-3">
              <h3 className="text-lg font-medium">暂无端点数据</h3>
              <p className="text-muted-foreground">点击"添加端点"按钮创建第一个API端点</p>
            </div>
          </div>
        ) : (
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2">
        {filteredAndSortedEndpoints().map((endpoint) => (
                <Card key={endpoint.id} className="relative group hover:shadow-md transition-shadow">
                  {/* 状态指示条 */}
                  <div className={`absolute top-0 left-0 right-0 h-1 rounded-t-lg ${
                    endpoint.status === "healthy" ? "bg-green-500" :
                    endpoint.status === "degraded" ? "bg-yellow-500" : "bg-red-500"
                  }`} />

                  <CardHeader className="pb-2 pr-16">
                    <div className="flex flex-col">
                      <div className="flex-1">
                        <CardTitle className="text-base font-semibold">
                          <span className="truncate">{endpoint.name}</span>
                        </CardTitle>
                        </div>
                    </div>

                    {/* 操作按钮组 - 右上角固定定位，一排显示，大小一致 */}
                    <div className="absolute top-2 right-2 flex space-x-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              className="text-xs px-3 h-7 w-16"
                              onClick={() => handleEditEndpoint(endpoint)}
                            >
                              编辑
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>编辑端点</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>

                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              className="text-xs px-3 h-7 w-16"
                              onClick={() => handleTestEndpoint(endpoint)}
                              disabled={testingEndpoints.has(endpoint.name)}
                            >
                              {testingEndpoints.has(endpoint.name) ? "测试中" : "测试"}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>测试端点</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>

                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              className="text-xs px-3 h-7 w-16 text-red-600 hover:text-red-700"
                              onClick={() => handleDeleteEndpoint(endpoint)}
                            >
                              删除
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>删除端点</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </div>
                  </CardHeader>

                  <CardContent className="space-y-3">
                    {/* URL信息 - 在地址中体现端点类型 */}
                    <div className="space-y-1">
                      <div className="text-sm font-medium">
                        端点地址
                      </div>
                      <div className="text-xs text-muted-foreground space-y-1">
                        {endpoint.url_anthropic && (
                          <div className="p-1.5 bg-muted rounded break-all">
                            <div className="font-medium mb-0.5 flex items-center gap-1">
                              <span className="w-1.5 h-1.5 bg-blue-500 rounded-full"></span>
                              Anthropic API
                            </div>
                            <div className="font-mono text-[9px] leading-tight">{endpoint.url_anthropic}</div>
                          </div>
                        )}
                        {endpoint.url_openai && (
                          <div className="p-1.5 bg-muted rounded break-all">
                            <div className="font-medium mb-0.5 flex items-center gap-1">
                              <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
                              OpenAI API
                            </div>
                            <div className="font-mono text-[9px] leading-tight">{endpoint.url_openai}</div>
                          </div>
                        )}
                      </div>
                    </div>

                    {/* 性能指标 */}
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-0.5">
                        <div className="text-xs font-medium text-muted-foreground">
                          响应时间
                        </div>
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <div
                                className={`
                                  text-base font-semibold cursor-help
                                  ${endpoint.blacklisted || !endpoint.enabled || endpoint.status === "unhealthy"
                                    ? "text-red-600"
                                    : endpoint.status === "degraded"
                                    ? "text-yellow-600"
                                    : "text-green-600"
                                  }
                                `}
                              >
                                {formatResponseTime(endpoint)}
                              </div>
                            </TooltipTrigger>
                            <TooltipContent>
                              <p className="text-sm">{getResponseTimeTooltip(endpoint)}</p>
                            </TooltipContent>
                          </Tooltip>
                        </TooltipProvider>
                      </div>
                      <div className="space-y-0.5">
                        <div className="text-xs font-medium text-muted-foreground">
                          成功率
                        </div>
                        <div className="text-base font-semibold">
                          {formatSuccessRate(endpoint)}
                        </div>
                      </div>
                    </div>

                    {/* 配置信息 - 优化避免横向滚动 */}
                    <div className="space-y-1">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-1.5 text-xs">
                          <span className="w-1.5 h-1.5 bg-blue-500 rounded-full" title={`优先级: ${endpoint.priority}`}></span>
                          <span className="text-gray-500">P{endpoint.priority}</span>

                          <span className="text-gray-400">|</span>

                          <span className={`w-1.5 h-1.5 rounded-full ${
                            endpoint.auth_type === 'api_key' ? 'bg-green-500' :
                            endpoint.auth_type === 'bearer_token' ? 'bg-blue-500' : 'bg-gray-500'
                          }`} title={`认证: ${endpoint.auth_type}`}></span>
                          <span className="text-gray-500 capitalize">
                            {endpoint.auth_type === "api_key" ? "API" :
                             endpoint.auth_type === "bearer_token" ? "Bearer" :
                             endpoint.auth_type === "basic_auth" ? "Basic" : "无"}
                          </span>

                          {endpoint.target_model && (
                            <>
                              <span className="text-gray-400">|</span>
                              <span className="w-1.5 h-1.5 bg-purple-500 rounded-full" title="目标模型"></span>
                              <span className="text-gray-500 truncate max-w-16">{endpoint.target_model}</span>
                            </>
                          )}
                        </div>
                      </div>
                      {endpoint.tags.length > 0 && (
                        <div className="flex flex-wrap gap-1">
                          {endpoint.tags.slice(0, 3).map((tag, index) => (
                            <Badge key={index} variant="secondary" className="text-[10px] px-1">
                              {tag.length > 8 ? `${tag.slice(0, 8)}...` : tag}
                            </Badge>
                          ))}
                          {endpoint.tags.length > 3 && (
                            <Badge variant="secondary" className="text-[10px] px-1">
                              +{endpoint.tags.length - 3}
                            </Badge>
                          )}
                        </div>
                      )}
                      {/* 学习信息显示 */}
                      {endpoint.openai_preference && endpoint.openai_preference !== "auto" && (
                        <Badge variant="outline" className="text-[10px] px-1 bg-blue-50 text-blue-700 border-blue-200" title="学习到的OpenAI格式偏好">
                          📚 {endpoint.openai_preference === "responses" ? "Responses" : "Chat"}
                        </Badge>
                      )}
                      {endpoint.supports_responses !== undefined && (
                        <Badge variant="outline" className="text-[10px] px-1 bg-green-50 text-green-700 border-green-200" title="是否支持 Responses API">
                          ✓ {endpoint.supports_responses ? "Responses支持" : "仅Chat"}
                        </Badge>
                      )}
                    </div>

                    {/* 黑名单原因 */}
                    {endpoint.blacklist_reason && (
                      <div className="p-2 bg-red-50 border border-red-200 rounded text-xs text-red-700">
                        拉黑原因: {endpoint.blacklist_reason}
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>

        {/* 添加/编辑端点对话框 */}
      <Dialog open={showAddEndpointDialog} onOpenChange={setShowAddEndpointDialog}>
        <DialogContent className="sm:max-w-[900px] max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingEndpoint ? "编辑端点" : "添加端点"}
            </DialogTitle>
            <DialogDescription>
            配置API端点的连接信息和认证方式
            <br />
            <span className="text-xs text-muted-foreground">
              端点类型将根据填写的URL自动判断：仅填写Anthropic URL则为Anthropic类型，仅填写OpenAI URL或同时填写两个URL则为OpenAI类型
              </span>
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-6">
              {/* 基本信息 */}
              <div className="space-y-4">
                <h3 className="text-lg font-medium text-gray-900">基本信息</h3>
                <FormField
                  control={form.control}
                  name="name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>端点名称</FormLabel>
                      <FormControl>
                        <Input placeholder="输入端点名称" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="tags"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>标签</FormLabel>
                      <FormControl>
                        <Input
                          placeholder="输入标签，用逗号分隔"
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        例如：claude-code, production, backup
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              {/* 连接配置 */}
              <div className="space-y-4">
                <h3 className="text-lg font-medium text-gray-900">连接配置</h3>
                <div className="grid grid-cols-2 gap-4">
                  <FormField
                    control={form.control}
                    name="url_anthropic"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Anthropic URL</FormLabel>
                        <FormControl>
                          <Input placeholder="https://api.anthropic.com" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="url_openai"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>OpenAI URL</FormLabel>
                        <FormControl>
                          <Input placeholder="https://api.openai.com" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </div>

              {/* 认证配置 */}
              <div className="space-y-4">
                <h3 className="text-lg font-medium text-gray-900">认证配置</h3>
                <div className="grid grid-cols-2 gap-4">
                  <FormField
                    control={form.control}
                    name="auth_type"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>认证类型</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="选择认证类型" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="api_key">API Key</SelectItem>
                            <SelectItem value="bearer_token">Bearer Token</SelectItem>
                            <SelectItem value="basic_auth">Basic Auth</SelectItem>
                            <SelectItem value="none">无认证</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="auth_value"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>认证值</FormLabel>
                        <FormControl>
                          <Input
                            type="password"
                            placeholder="输入认证信息"
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </div>

              {/* 基础设置 */}
              <div className="space-y-4">
                <h3 className="text-lg font-medium text-gray-900">基础设置</h3>
                <div className="grid grid-cols-2 gap-4">
                  <FormField
                    control={form.control}
                    name="priority"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>优先级</FormLabel>
                        <FormControl>
                          <Input
                            type="number"
                            min="1"
                            max="100"
                            {...field}
                            onChange={(e) => field.onChange(parseInt(e.target.value))}
                          />
                        </FormControl>
                        <FormDescription>
                          数值越大优先级越高 (1-100)
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="enabled"
                    render={({ field }) => (
                      <FormItem className="flex flex-row items-center h-10">
                        <FormLabel className="flex-1">启用端点</FormLabel>
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

              {/* 兼容性配置 */}
              <div className="space-y-4">
                <h3 className="text-lg font-medium text-gray-900">兼容性配置</h3>
                {((form.watch("url_openai") || "") && (form.watch("url_openai") || "").trim() !== "") && (
                  <FormField
                    control={form.control}
                    name="openai_preference"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>OpenAI 格式偏好</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="选择格式偏好" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="auto">自动检测</SelectItem>
                            <SelectItem value="responses">Responses API</SelectItem>
                            <SelectItem value="chat_completions">Chat Completions</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          对于 OpenAI 端点的 API 格式偏好
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <FormField
                  control={form.control}
                  name="target_model"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>测试模型</FormLabel>
                      <FormControl>
                        <Input
                          placeholder="例如：claude-3-sonnet-20240229"
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        可选，指定此端点支持的目标模型（向后兼容）
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              {/* 模型重写配置 */}
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="text-lg font-medium text-gray-900">模型重写配置</h3>
                  <FormField
                    control={form.control}
                    name="model_rewrite.enabled"
                    render={({ field }) => (
                      <FormItem className="flex flex-row items-center space-x-2 space-y-0">
                        <FormControl>
                          <Checkbox
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                        <FormLabel className="text-base font-medium">启用模型重写</FormLabel>
                      </FormItem>
                    )}
                  />
                </div>

                {/* 当启用模型重写时才显示详细配置 */}
                {form.watch("model_rewrite.enabled") && (
                  <div className="space-y-4 pl-6 border-l-2 border-gray-200">
                    <FormDescription>
                      启用后，可以将请求的模型名称重写为端点支持的模型
                    </FormDescription>

                    {/* 模型重写规则配置 */}
                    <div className="space-y-4">
                      <div className="flex items-center justify-between">
                        <FormLabel className="text-base font-medium">模型重写规则</FormLabel>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            const currentRules = form.getValues("model_rewrite.rules") || []
                            form.setValue("model_rewrite.rules", [
                              ...currentRules,
                              { source_pattern: "", target_model: "" }
                            ])
                          }}
                        >
                          添加规则
                        </Button>
                      </div>

                      <FormDescription>
                        配置模型名称重写规则，支持通配符匹配。使用 "*" 作为默认规则匹配所有模型。例如：* → glm-4.5，claude-haiku-* → glm-4.5，claude-sonnet-* → glm-4.6
                      </FormDescription>

                      <FormField
                        control={form.control}
                        name="model_rewrite.rules"
                        render={({ field }) => (
                          <FormItem>
                            <FormControl>
                              <div className="space-y-1">
                                {(field.value || []).map((rule, index) => (
                                  <div key={index} className="flex items-center space-x-2">
                                    <Input
                                      placeholder="源模型模式 (如: * 或 claude-haiku-*)"
                                      value={rule.source_pattern}
                                      onChange={(e) => {
                                        const newRules = [...(field.value || [])]
                                        newRules[index] = { ...rule, source_pattern: e.target.value }
                                        field.onChange(newRules)
                                      }}
                                      className="flex-1"
                                    />
                                    <Input
                                      placeholder="目标模型 (如: glm-4.5)"
                                      value={rule.target_model}
                                      onChange={(e) => {
                                        const newRules = [...(field.value || [])]
                                        newRules[index] = { ...rule, target_model: e.target.value }
                                        field.onChange(newRules)
                                      }}
                                      className="flex-1"
                                    />
                                    <Button
                                      type="button"
                                      variant="ghost"
                                      size="icon"
                                      onClick={() => {
                                        const newRules = [...(field.value || [])]
                                        newRules.splice(index, 1)
                                        field.onChange(newRules)
                                      }}
                                      className="h-9 w-9 text-red-500 hover:text-red-700 hover:bg-red-50"
                                    >
                                      <Trash2 className="h-4 w-4" />
                                    </Button>
                                  </div>
                                ))}

                                {(field.value || []).length === 0 && (
                                  <div className="text-center py-4 text-muted-foreground border-2 border-dashed rounded-md">
                                    暂无重写规则，点击"添加规则"按钮开始配置
                                  </div>
                                )}
                              </div>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                  </div>
                )}
              </div>

              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setShowAddEndpointDialog(false)
                    setEditingEndpoint(null)
                    form.reset()
                  }}
                >
                  取消
                </Button>
                <Button type="submit">
                  {editingEndpoint ? "更新" : "创建"}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      {/* 删除确认对话框 */}
      <Dialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>确认删除端点</DialogTitle>
            <DialogDescription>
              确定要删除端点 "<span className="font-semibold">{endpointToDelete?.name}</span>" 吗？
              <br />
              此操作不可撤销，端点的所有配置和数据将被永久删除。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setShowDeleteDialog(false)
                setEndpointToDelete(null)
              }}
            >
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDeleteEndpoint}
            >
              确认删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
