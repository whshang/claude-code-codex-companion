import { BrowserRouter as Router, Routes, Route, useLocation } from 'react-router-dom'
import { useEffect } from 'react'
import { Toaster } from 'sonner'
import { ThemeProvider } from '@/components/theme-provider'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Layout } from '@/components/layout/Layout'
import Dashboard from '@/pages/Dashboard'
import { Settings } from '@/pages/Settings'
import Endpoints from '@/pages/Endpoints'
import { Logs } from '@/pages/Logs'
import Config from '@/pages/Config'
import { DebugConsole, useDebugConsole } from '@/components/DebugConsole'
import './style.css'

// API请求拦截组件 - 处理API路径请求
function ApiRequestHandler() {
  const location = useLocation()

  // 如果是API请求，返回null让浏览器处理
  if (location.pathname.startsWith('/admin/api/') ||
      location.pathname.startsWith('/v1/') ||
      location.pathname.startsWith('/claude/') ||
      location.pathname.startsWith('/openai')) {
    return null
  }

  // 对于其他未知路径，重定向到首页
  return <Dashboard />
}

function App() {
  const debugConsole = useDebugConsole()

  // 在开发环境下自动启用调试控制台
  useEffect(() => {
    if (process.env.NODE_ENV === 'development' || window.location.hostname === 'localhost') {
      debugConsole.setDebugEnabled(true)
    }
  }, [debugConsole])

  return (
    <ThemeProvider defaultTheme="system" storageKey="cccc-ui-theme">
      <TooltipProvider>
        <Router>
          <Layout>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="/endpoints" element={<Endpoints />} />
              <Route path="/logs" element={<Logs />} />
              <Route path="/config" element={<Config />} />
              {/* Catch-all route for API requests and unknown paths */}
              <Route path="*" element={<ApiRequestHandler />} />
            </Routes>
          </Layout>
          <Toaster />
          <DebugConsole />
        </Router>
      </TooltipProvider>
    </ThemeProvider>
  )
}

export default App
