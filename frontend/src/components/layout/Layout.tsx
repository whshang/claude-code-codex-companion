import { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { cn } from '@/lib/utils'
import {
  LayoutDashboard,
  Settings,
  Server,
  FileText,
  Menu,
  X,
  Cog,
  Keyboard,
  Github,
  ExternalLink,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ThemeToggle } from '@/components/ui/theme-toggle'
import { useAppShortcuts } from '@/hooks/use-keyboard-shortcuts'
import { useState, useEffect } from 'react'
import { wailsAPI } from '@/lib/wails-api'

interface LayoutProps {
  children: ReactNode
}

const navigation = [
  { name: '仪表板', href: '/', icon: LayoutDashboard },
  { name: '端点管理', href: '/endpoints', icon: Server },
  { name: '配置管理', href: '/config', icon: Cog },
  { name: '请求日志', href: '/logs', icon: FileText },
  { name: '应用设置', href: '/settings', icon: Settings },
]

export function Layout({ children }: LayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [appVersion, setAppVersion] = useState('1.0.0')
  const [serverStatus, setServerStatus] = useState<'stopped' | 'error' | 'running'>('stopped')
  const [shortcutsDialogOpen, setShortcutsDialogOpen] = useState(false)
  const location = useLocation()

  // 启用应用级快捷键
  const shortcuts = useAppShortcuts()

  // 处理外部链接点击
  const handleExternalLinkClick = async (url: string) => {
    try {
      // 优先使用Wails的OpenURL方法
      const result = await wailsAPI.OpenURL(url)
      if (!result || !result.success) {
        // 如果Wails方法失败，回退到window.open
        console.warn('Wails OpenURL failed, falling back to window.open:', result?.message)
        const opened = window.open(url, '_blank', 'noopener,noreferrer')
        if (!opened || opened.closed || typeof opened.closed === 'undefined') {
          console.warn('window.open failed, trying location.href')
          window.location.href = url
        }
      }
    } catch (error) {
      console.error('Failed to open external link:', error)
      // 最后的备选方案
      try {
        const opened = window.open(url, '_blank', 'noopener,noreferrer')
        if (!opened || opened.closed || typeof opened.closed === 'undefined') {
          window.location.href = url
        }
      } catch (fallbackError) {
        console.error('All fallback methods failed:', fallbackError)
        window.location.href = url
      }
    }
  }

  // 获取应用版本信息
  useEffect(() => {
    // 从后端获取版本信息（包含编译时间）
    const fetchVersionInfo = async () => {
      try {
        const { GetVersionInfo } = await import('../../../wailsjs/go/main/App')
        const versionInfo = await GetVersionInfo()
        setAppVersion(versionInfo)
      } catch (error) {
        console.error('获取版本信息失败:', error)
        setAppVersion('1.0.0')
      }
    }
    fetchVersionInfo()
  }, [])

  // 监控服务器状态
  useEffect(() => {
    const checkServerStatus = async () => {
      try {
        const { GetServerStatus } = await import('../../../wailsjs/go/main/App')
        const status = await GetServerStatus()
        setServerStatus(status.running ? 'running' : 'stopped')
      } catch (error) {
        console.error('获取服务器状态失败:', error)
        setServerStatus('error')
      }
    }

    // 立即检查一次
    checkServerStatus()

    // 每3秒检查一次服务器状态
    const interval = setInterval(checkServerStatus, 3000)

    return () => clearInterval(interval)
  }, [])

  return (
    <div className="min-h-screen bg-background">
      {/* Mobile sidebar */}
      <div className={cn(
        "fixed inset-0 z-50 md:hidden",
        sidebarOpen ? "block" : "hidden"
      )}>
        <div className="fixed inset-0 bg-black/50" onClick={() => setSidebarOpen(false)} />
        <div className="fixed left-0 top-0 h-full w-64 bg-card border-r flex flex-col">
          <div className="flex items-center justify-between p-6 pt-8">
            <h2 className="text-xl font-semibold leading-relaxed">CCCC</h2>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setSidebarOpen(false)}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
          <nav className="px-4 flex-1">
            {navigation.map((item) => {
              const isActive = location.pathname === item.href
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  className={cn(
                    "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors mb-1",
                    isActive
                      ? "bg-primary text-primary-foreground"
                      : "text-muted-foreground hover:text-foreground hover:bg-accent"
                  )}
                  onClick={() => setSidebarOpen(false)}
                >
                  <item.icon className="h-4 w-4" />
                  {item.name}
                </Link>
              )
            })}
          </nav>
          <div className="px-4 py-4 border-t space-y-2">
            <Button
              variant="ghost"
              className="w-full justify-start h-8 px-3"
              onClick={() => {
                handleExternalLinkClick('https://github.com/whshang/claude-code-codex-companion')
              }}
            >
              <Github className="h-4 w-4 mr-3" />
              <span className="text-sm">GitHub</span>
              <ExternalLink className="h-3 w-3 ml-auto" />
            </Button>
            <div className="flex items-center gap-2 text-xs text-muted-foreground px-3">
              <div className={`w-2 h-2 rounded-full ${serverStatus === 'running' ? 'bg-green-500' : serverStatus === 'error' ? 'bg-yellow-500' : 'bg-red-500'}`} />
              {appVersion}
            </div>
          </div>
        </div>
      </div>

      {/* Desktop sidebar */}
      <div className="hidden md:fixed md:inset-y-0 md:z-50 md:flex md:w-64 md:flex-col">
        <div className="flex grow flex-col gap-y-5 overflow-y-auto bg-card border-r px-6 pb-2">
          <div className="flex h-16 shrink-0 items-center pt-6" style={{ "--wails-draggable": "drag" } as React.CSSProperties}>
            <div className="flex items-center">
              <h2 className="text-xl font-semibold leading-relaxed">CCCC Proxy</h2>
            </div>
          </div>
          <nav className="flex flex-1 flex-col">
            <ul role="list" className="flex flex-1 flex-col gap-y-7">
              <li>
                <ul role="list" className="-mx-2 space-y-1">
                  {navigation.map((item) => {
                    const isActive = location.pathname === item.href
                    return (
                      <li key={item.name}>
                        <Link
                          to={item.href}
                          className={cn(
                            "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                            isActive
                              ? "bg-primary text-primary-foreground"
                              : "text-muted-foreground hover:text-foreground hover:bg-accent"
                          )}
                        >
                          <item.icon className="h-4 w-4" />
                          {item.name}
                        </Link>
                      </li>
                    )
                  })}
                </ul>
              </li>
            </ul>
          </nav>
          <div className="border-t pt-4 space-y-2">
            <Button
              variant="ghost"
              className="w-full justify-start h-8 px-3"
              onClick={() => {
                handleExternalLinkClick('https://github.com/whshang/claude-code-codex-companion')
              }}
            >
              <Github className="h-4 w-4 mr-3" />
              <span className="text-sm">GitHub</span>
              <ExternalLink className="h-3 w-3 ml-auto" />
            </Button>
            <div className="flex items-center justify-center gap-2 text-xs text-muted-foreground px-3">
              <div className={`w-2 h-2 rounded-full ${serverStatus === 'running' ? 'bg-green-500' : serverStatus === 'error' ? 'bg-yellow-500' : 'bg-red-500'}`} />
              {appVersion}
            </div>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="md:pl-64">
        {/* Top bar */}
        <div className="sticky top-0 z-40 flex h-20 shrink-0 items-center gap-x-4 border-b bg-card px-4 shadow-sm sm:gap-x-6 sm:px-6 lg:px-8 pt-6"
             style={{ "--wails-draggable": "drag" } as React.CSSProperties}>
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            onClick={() => setSidebarOpen(true)}
            style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
          >
            <Menu className="h-4 w-4" />
          </Button>

          <div className="flex flex-1 gap-x-4 self-stretch lg:gap-x-6">
            {/* 左侧页面标题区域 - 可拖拽，左对齐 */}
            <div className="flex flex-1 items-center" style={{ "--wails-draggable": "drag" } as React.CSSProperties}>
              <div className="flex items-center">
                <h1 className="text-lg font-semibold leading-relaxed">
                  {navigation.find(item => item.href === location.pathname)?.name || '仪表板'}
                </h1>
              </div>
            </div>
            {/* 右侧按钮区域 - 不可拖拽 */}
            <div className="flex items-center gap-x-4" style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setShortcutsDialogOpen(true)}
                title="快捷键帮助 (Shift+?)"
                className="hover:bg-accent hover:text-accent-foreground"
                data-shortcuts-help
              >
                <Keyboard className="h-4 w-4" />
              </Button>
              <ThemeToggle />
            </div>
          </div>
        </div>

        {/* Page content */}
        <main className="py-6">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            {children}
          </div>
        </main>
      </div>

      {/* 快捷键帮助对话框 */}
      <Dialog open={shortcutsDialogOpen} onOpenChange={setShortcutsDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>键盘快捷键</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <h4 className="font-medium mb-2 text-sm">导航快捷键</h4>
              <div className="space-y-1 text-sm">
                {shortcuts.slice(0, 5).map((shortcut, index) => (
                  <div key={index} className="flex justify-between">
                    <span>{shortcut.description.replace('导航到', '')}</span>
                    <kbd className="px-2 py-1 text-xs bg-muted rounded">
                      {shortcut.display}
                    </kbd>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <h4 className="font-medium mb-2 text-sm">操作快捷键</h4>
              <div className="space-y-1 text-sm">
                {shortcuts.slice(5).map((shortcut, index) => (
                  <div key={index} className="flex justify-between">
                    <span>{shortcut.description}</span>
                    <kbd className="px-2 py-1 text-xs bg-muted rounded">
                      {shortcut.display}
                    </kbd>
                  </div>
                ))}
              </div>
            </div>

            <div className="text-xs text-muted-foreground bg-muted p-3 rounded">
              💡 <strong>提示：</strong>在输入框中使用快捷键不会触发操作，避免干扰文本输入。
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}