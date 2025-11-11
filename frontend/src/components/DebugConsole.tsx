import { useState, useEffect } from 'react'
import { toast } from 'sonner'

interface DebugMessage {
  id: string
  timestamp: string
  message: string
}

export function useDebugConsole() {
  const [messages, setMessages] = useState<DebugMessage[]>([])
  const [enabled, setEnabled] = useState(false)

  // 从设置中加载调试控制台开关状态
  useEffect(() => {
    const loadDebugSetting = async () => {
      try {
        // 优先从后端配置读取，如果失败则使用localStorage作为fallback
        try {
          const { wailsAPI } = await import('@/lib/wails-api')
          const config = await wailsAPI.LoadConfig()
          const debugEnabled = config?.debug?.console_enabled === true
          setEnabled(debugEnabled)
          localStorage.setItem('debugConsoleEnabled', debugEnabled.toString())
        } catch (configError) {
          console.warn('Failed to load debug setting from backend, using localStorage:', configError)
          const saved = localStorage.getItem('debugConsoleEnabled')
          setEnabled(saved === 'true')
        }
      } catch (error) {
        console.error('Failed to load debug setting:', error)
        // 最终fallback到localStorage
        const saved = localStorage.getItem('debugConsoleEnabled')
        setEnabled(saved === 'true')
      }
    }
    loadDebugSetting()
  }, [])

  // 监听调试事件
  useEffect(() => {
    const handleDebugMessage = (event: CustomEvent) => {
      addMessage(event.detail)
    }

    const handleDebugClear = () => {
      clearMessages()
    }

    const handleDebugEnabled = (event: CustomEvent) => {
      setEnabled(event.detail)
    }

    // 添加事件监听器
    if (typeof window !== 'undefined') {
      window.addEventListener('debug-message', handleDebugMessage as EventListener)
      window.addEventListener('debug-clear', handleDebugClear as EventListener)
      window.addEventListener('debug-enabled', handleDebugEnabled as EventListener)

      // 清理函数
      return () => {
        window.removeEventListener('debug-message', handleDebugMessage as EventListener)
        window.removeEventListener('debug-clear', handleDebugClear as EventListener)
        window.removeEventListener('debug-enabled', handleDebugEnabled as EventListener)
      }
    }
  }, [])

  const addMessage = (message: string) => {
    if (!enabled) return

    const newMessage: DebugMessage = {
      id: Date.now().toString(),
      timestamp: new Date().toLocaleTimeString('zh-CN', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      }),
      message
    }

    setMessages(prev => {
      const updated = [...prev, newMessage]
      // 保持最多100条消息
      if (updated.length > 100) {
        return updated.slice(-100)
      }
      return updated
    })
  }

  const clearMessages = () => {
    setMessages([])
  }

  const setDebugEnabled = async (enabled: boolean) => {
    setEnabled(enabled)
    localStorage.setItem('debugConsoleEnabled', enabled.toString())

    // 尝试更新后端配置
    try {
      const { wailsAPI } = await import('@/lib/wails-api')
      const config = await wailsAPI.LoadConfig()
      config.debug = config.debug || {}
      config.debug.console_enabled = enabled
      await wailsAPI.SaveConfig(config)
    } catch (error) {
      console.warn('Failed to update debug setting in backend config:', error)
      // 即使后端更新失败，也继续使用localStorage
    }

    if (!enabled) {
      clearMessages()
    }
  }

  return {
    messages,
    enabled,
    addMessage,
    clearMessages,
    setDebugEnabled
  }
}

export function DebugConsole() {
  const { messages, enabled, clearMessages } = useDebugConsole()

  if (!enabled || messages.length === 0) {
    return null
  }

  return (
    <div className="fixed bottom-4 right-4 w-96 bg-black text-green-400 font-mono text-xs p-3 rounded-lg shadow-lg max-h-64 overflow-auto border border-green-600 z-50">
      <div className="flex justify-between items-center mb-2">
        <span className="text-yellow-400">调试控制台</span>
        <div className="flex gap-2">
          <button
            onClick={() => {
              const text = messages.map(msg => `[${msg.timestamp}] ${msg.message}`).join('\n')
              navigator.clipboard.writeText(text)
              toast.success('调试日志已复制到剪贴板')
            }}
            className="text-blue-400 hover:text-blue-300 text-xs"
            title="复制所有日志"
          >
            复制
          </button>
          <button
            onClick={clearMessages}
            className="text-red-400 hover:text-red-300 text-xs"
            title="清除所有日志"
          >
            清除
          </button>
        </div>
      </div>
      {messages.map((msg) => (
        <div key={msg.id} className="whitespace-pre-wrap break-all">
          <span className="text-gray-400">[{msg.timestamp}]</span> {msg.message}
        </div>
      ))}
    </div>
  )
}

// 全局调试控制台实例
let globalDebugConsole: ReturnType<typeof useDebugConsole> | null = null

export function getGlobalDebugConsole() {
  if (!globalDebugConsole) {
    // 创建一个模拟的hook实例用于全局调用
    const messages: DebugMessage[] = []

    return {
      messages,
      enabled: false,
      addMessage: (message: string) => {
        if (typeof window !== 'undefined') {
          const event = new CustomEvent('debug-message', { detail: message })
          window.dispatchEvent(event)
        }
      },
      clearMessages: () => {
        if (typeof window !== 'undefined') {
          const event = new CustomEvent('debug-clear')
          window.dispatchEvent(event)
        }
      },
      setDebugEnabled: async (enabled: boolean) => {
        if (typeof window !== 'undefined') {
          localStorage.setItem('debugConsoleEnabled', enabled.toString())

          // 尝试更新后端配置
          try {
            const { wailsAPI } = await import('@/lib/wails-api')
            const config = await wailsAPI.LoadConfig()
            config.debug = config.debug || {}
            config.debug.console_enabled = enabled
            await wailsAPI.SaveConfig(config)
          } catch (error) {
            console.warn('Failed to update debug setting in backend config:', error)
          }

          const event = new CustomEvent('debug-enabled', { detail: enabled })
          window.dispatchEvent(event)
        }
      }
    }
  }
  return globalDebugConsole
}