import { useEffect, useCallback } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'

// 检测操作系统
function getOS(): 'mac' | 'windows' | 'linux' | 'unknown' {
  if (typeof window === 'undefined') return 'unknown'

  const userAgent = window.navigator.userAgent.toLowerCase()
  const platform = window.navigator.platform.toLowerCase()

  if (platform.includes('mac') || userAgent.includes('mac')) {
    return 'mac'
  } else if (platform.includes('win') || userAgent.includes('win')) {
    return 'windows'
  } else if (platform.includes('linux') || userAgent.includes('linux')) {
    return 'linux'
  }

  return 'unknown'
}

// 获取正确的修饰键显示
function getModifierKey(): string {
  const os = getOS()
  return os === 'mac' ? 'Cmd' : 'Ctrl'
}

// 获取完整的快捷键显示
function getShortcutDisplay(baseKey: string, ctrl?: boolean, alt?: boolean, shift?: boolean): string {
  const parts: string[] = []

  if (ctrl) parts.push(getModifierKey())
  if (alt) parts.push('Alt')
  if (shift) parts.push('Shift')

  parts.push(baseKey)
  return parts.join('+')
}

interface KeyboardShortcut {
  key: string
  ctrlKey?: boolean
  altKey?: boolean
  shiftKey?: boolean
  metaKey?: boolean
  description: string
  action: () => void
  display?: string // 自动生成的显示文本
}

export function useKeyboardShortcuts(shortcuts: KeyboardShortcut[]) {
  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    // 忽略在输入框中触发的快捷键
    const target = event.target as HTMLElement
    if (
      target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.contentEditable === 'true'
    ) {
      return
    }

  
    for (const shortcut of shortcuts) {
      const keyMatch = event.key.toLowerCase() === shortcut.key.toLowerCase()

      // 处理Mac的Cmd键和Windows/Linux的Ctrl键
      let modifierMatch = true
      const os = getOS()

      if (shortcut.ctrlKey) {
        if (os === 'mac') {
          modifierMatch = modifierMatch && event.metaKey
        } else {
          modifierMatch = modifierMatch && event.ctrlKey
        }
      }

      const altMatch = !!shortcut.altKey === event.altKey
      const shiftMatch = !!shortcut.shiftKey === event.shiftKey
      const metaMatch = !!shortcut.metaKey === event.metaKey

      if (keyMatch && modifierMatch && altMatch && shiftMatch && metaMatch) {
        event.preventDefault()
        event.stopPropagation()
        shortcut.action()
        break
      }
    }
  }, [shortcuts])

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [handleKeyDown])
}

// 应用级快捷键Hook
export function useAppShortcuts() {
  const navigate = useNavigate()
  const location = useLocation()

  const shortcuts: KeyboardShortcut[] = [
    // 导航快捷键 - Mac使用metaKey（Cmd），Windows/Linux使用ctrlKey
    {
      key: '1',
      ctrlKey: true,
      metaKey: true, // 在Mac上Cmd键会触发，在Windows/Linux上Ctrl键会触发
      description: '导航到仪表板',
      action: () => navigate('/'),
      display: getShortcutDisplay('1', true)
    },
    {
      key: '2',
      ctrlKey: true,
      metaKey: true,
      description: '导航到端点管理',
      action: () => navigate('/endpoints'),
      display: getShortcutDisplay('2', true)
    },
    {
      key: '3',
      ctrlKey: true,
      metaKey: true,
      description: '导航到配置管理',
      action: () => navigate('/config'),
      display: getShortcutDisplay('3', true)
    },
    {
      key: '4',
      ctrlKey: true,
      metaKey: true,
      description: '导航到请求日志',
      action: () => navigate('/logs'),
      display: getShortcutDisplay('4', true)
    },
    {
      key: '5',
      ctrlKey: true,
      metaKey: true,
      description: '导航到应用设置',
      action: () => navigate('/settings'),
      display: getShortcutDisplay('5', true)
    },

    // 通用快捷键 - Mac使用metaKey（Cmd），Windows/Linux使用ctrlKey
    {
      key: 'r',
      ctrlKey: true,
      metaKey: true,
      description: '刷新当前页面',
      action: () => window.location.reload(),
      display: getShortcutDisplay('R', true)
    },
    {
      key: 't',
      ctrlKey: true,
      altKey: true,
      metaKey: true,
      description: '切换主题',
      action: () => {
        // 触发主题切换按钮点击
        const themeButton = document.querySelector('[data-theme-toggle]') as HTMLButtonElement
        if (themeButton) {
          themeButton.click()
        }
      },
      display: getShortcutDisplay('T', true, true)
    },
    {
      key: 'n',
      ctrlKey: true,
      metaKey: true,
      description: '新建端点',
      action: () => {
        if (location.pathname === '/endpoints') {
          // 在端点页面时，触发新建端点按钮
          const addButton = document.querySelector('[data-add-endpoint]') as HTMLButtonElement
          if (addButton) {
            addButton.click()
          }
        } else {
          // 否则导航到端点页面
          navigate('/endpoints')
        }
      },
      display: getShortcutDisplay('N', true)
    },
    {
      key: 's',
      ctrlKey: true,
      metaKey: true,
      description: '保存配置',
      action: () => {
        // 查找并点击保存按钮
        const saveButton = document.querySelector('[data-save-config]') as HTMLButtonElement
        if (saveButton) {
          saveButton.click()
        }
      },
      display: getShortcutDisplay('S', true)
    },
    {
      key: '?',
      shiftKey: true,
      description: '显示快捷键帮助',
      action: () => {
        // 显示快捷键帮助对话框
        const helpButton = document.querySelector('[data-shortcuts-help]') as HTMLButtonElement
        if (helpButton) {
          helpButton.click()
        }
      },
      display: getShortcutDisplay('?', false, false, true)
    }
  ]

  useKeyboardShortcuts(shortcuts)

  return shortcuts
}