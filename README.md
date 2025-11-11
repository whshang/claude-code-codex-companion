# Claude Code and Codex Companion (CCCC)

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://golang.org/)
[![React Version](https://img.shields.io/badge/React-18+-61DAFB?logo=react)](https://reactjs.org/)
[![Wails Version](https://img.shields.io/badge/Wails-2.10+-blue?logo=wails)](https://wails.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **桌面客户端 + 代理服务器 一体化 AI API 管理系统**
> 提供现代化桌面界面管理多端点 AI API 代理，支持 Claude Code、Codex 等客户端的统一接入。

> **🆕 最新更新**: 统一代理管线现已支持 OpenAI Chat ↔ OpenAI Responses ↔ Anthropic 的请求/响应互转、自动失败回退与 SSE 流式处理。

---

## 📖 项目简介

CCCC（Claude Code and Codex Companion）是一个**桌面客户端 + 代理服务器一体化的 AI API 管理系统**。项目采用现代化架构，提供直观的桌面应用界面来管理强大的后端代理服务。

### 🎯 核心设计理念

- **🖥️ 桌面优先**：提供原生桌面应用体验，告别 Web 界面的复杂性
- **🚀 零配置启动**：开箱即用的桌面应用，内置智能代理服务器
- **📱 现代化界面**：基于 React + TypeScript + shadcn/ui 的现代化用户界面
- **⚡ 高性能代理**：智能端点选择、格式转换、负载均衡
- **🛡️ 企业级可靠性**：健康检查、故障转移、日志监控

### 🏗️ 系统架构

```
┌─────────────────┐    ┌─────────────────┐
│   Wails Desktop │────│   HTTP Server   │
│     Frontend    │    │   (Port 8080)   │
│   React + TS    │    │                 │
└─────────────────┘    └─────────────────┘
         │                        │
         └─────────── OnDomReady ─┘
```

**架构说明**：
- **桌面应用**：基于 Wails 框架的现代化桌面应用，提供直观的图形界面
- **同步启动**：应用启动时自动同步启动内置 HTTP 服务器
- **轻量化设计**：采用简洁架构，避免复杂的依赖关系，确保高可靠性

## ✨ 功能概览

- **桌面体验**：React + TypeScript + shadcn/ui 打造的原生桌面 UI，响应式布局、快捷键、暗色主题开箱即用。
- **智能代理**：图形化端点 CRUD、优先级/负载均衡、健康检查、失败回退，以及 OpenAI Chat ↔ Responses ↔ Anthropic 的双向格式转换。
- **可观测性**：实时仪表板、结构化日志（搜索/过滤/导出）、请求追踪与错误诊断，帮助快速排查问题。
- **配置与部署**：内置在线配置编辑器，单二进制跨平台分发（macOS/Windows/Linux），无需额外 Web 服务器。

### 🧭 能力进展

| 能力项 | 状态 | 说明 |
| --- | --- | --- |
| 多格式转换（OpenAI Chat ↔ OpenAI Responses ↔ Anthropic） | ✅ 已完成 | 请求/响应及 SSE 均已在统一管线内自动转换 |
| 失败回退与动态排序 | ✅ 已完成 | 自动重试、端点学习、`count_tokens` 估算回退已启用 |
| 模型重写闭环 | ⚠️ 部分完成 | 请求侧生效，响应侧回写待接入 |
| 日志与统计存储 | ✅ 已完成 | 桌面端使用统一的 GORM 日志与统计数据库 |

### ⚠️ 已知限制
- 响应体尚未恢复模型重写前的名称，客户端会看到供应商别名。

---

## 🚀 快速开始

### 系统要求

- **操作系统**：macOS 10.15+ / Windows 10+ / Linux (Ubuntu 18.04+)
- **内存**：最低 4GB RAM，推荐 8GB+
- **磁盘空间**：100MB 可用空间
- **网络**：互联网连接（用于访问 AI 服务）

### 安装方式

#### 方式一：直接下载（推荐）

1. **下载应用**
   ```bash
   # macOS
   curl -L https://github.com/whshang/claude-code-codex-companion/releases/latest/download/cccc-proxy-macos.zip -o cccc-proxy.zip
   unzip cccc-proxy.zip

   # Windows
   # 从 Releases 页面下载 cccc-proxy-windows.exe

   # Linux
   curl -L https://github.com/whshang/claude-code-codex-companion/releases/latest/download/cccc-proxy-linux.tar.gz -o cccc-proxy.tar.gz
   tar -xzf cccc-proxy.tar.gz
   ```

2. **启动应用**
   ```bash
   # macOS
   open cccc-proxy.app

   # Windows
   ./cccc-proxy.exe

   # Linux
   ./cccc-proxy
   ```

#### 方式二：从源码构建

1. **克隆仓库**
   ```bash
   git clone https://github.com/whshang/claude-code-codex-companion.git
   cd claude-code-codex-companion
   ```

2. **构建桌面应用**
```bash
wails build -clean  # 推荐使用此命令
# 或者使用统一脚本
./start.sh build
```

3. **启动应用**
   ```bash
   open build/bin/cccc-proxy.app  # macOS
   ./build/bin/cccc-proxy        # Linux
   # Windows: build/bin/cccc-proxy.exe
   ```

### 首次使用

1. **启动应用**：双击启动 CCCC 桌面应用，应用会自动启动内置服务器
2. **验证服务**：应用启动后，服务器会自动在 `http://localhost:8080` 运行
3. **检查状态**：在应用界面查看服务器状态和运行信息
4. **开始使用**：配置 Claude Code 或 Codex 连接到 `http://localhost:8080`

---

## 📖 使用指南

### 端点配置

#### Anthropic Claude 端点

```yaml
name: "Anthropic Official"
url_anthropic: "https://api.anthropic.com"
auth_type: "api_key"
auth_value: "sk-ant-xxxxx"
enabled: true
priority: 1
```

#### OpenAI 兼容端点

```yaml
name: "OpenAI Compatible"
url_openai: "https://api.openai.com/v1/chat/completions"
auth_type: "api_key"
auth_value: "sk-xxxxx"
enabled: true
priority: 2
```

#### 通用端点（支持多种格式）

```yaml
name: "Universal Provider"
url_anthropic: "https://api.provider.com/anthropic"
url_openai: "https://api.provider.com/openai"
auth_type: "api_key"
auth_value: "your-api-key"
enabled: true
priority: 3
```

### 客户端配置

#### Claude Code 配置

**方式一：环境变量**
```bash
export ANTHROPIC_BASE_URL="http://localhost:8080"
export ANTHROPIC_AUTH_TOKEN="hello"
```

**方式二：settings.json**
```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:8080",
    "ANTHROPIC_AUTH_TOKEN": "hello"
  }
}
```

### 项目结构

```
claude-code-codex-companion/
├── README.md / AGENTS.md        # 文档
├── app.go / main.go             # Wails 桌面入口
├── frontend/                    # React + Tailwind 前端
│   ├── src/                     # 组件、页面
│   └── package.json             # 前端依赖
├── internal/                    # 共享后端核心（代理、端点、配置、日志…）
├── start.sh                     # 统一开发/构建脚本
├── wails.json                   # Wails 配置
└── .cccc-data/                  # 运行期数据库（生成）
```

**架构说明**：
- **双模式支持**：提供独立的代理服务器和桌面应用两种运行模式
- **轻量化设计**：桌面应用采用简洁架构，避免复杂依赖
- **同步启动**：桌面应用启动时自动启动内置HTTP服务器

### 开发环境搭建

1. **安装依赖**
   ```bash
   # Go 1.23+
   go version

   # Node.js 18+
   node --version
   npm --version
   ```

2. **开发代理服务器（可选调试共享核心）**
```bash
go run main.go
```

3. **开发桌面应用**
```bash
# 安装前端依赖
cd frontend
npm install

# 返回项目根目录启动 Wails Dev Server
cd ..
wails dev

# 如需分别调试
# 终端1
cd frontend && npm run dev
# 终端2
wails dev --no-frontend
```

### 构建发布

```bash
# 构建桌面应用（当前平台）
wails build -clean

# 使用脚本构建/打包
./start.sh build --open

# 验证输出
ls -la build/bin/
```

---

## 🤝 贡献指南

我们欢迎所有形式的贡献！

### 贡献方式

- 🐛 **报告 Bug**：在 Issues 中报告问题
- 💡 **功能建议**：提出新功能想法
- 📝 **文档改进**：完善文档和示例
- 🔧 **代码贡献**：提交 Pull Request

### 开发流程

1. **Fork** 项目到你的 GitHub
2. **创建** 功能分支 (`git checkout -b feature/amazing-feature`)
3. **提交** 你的更改 (`git commit -m 'Add amazing feature'`)
4. **推送** 到分支 (`git push origin feature/amazing-feature`)
5. **创建** Pull Request

### 代码规范

- Go 代码遵循 [Effective Go](https://golang.org/doc/effective_go.html)
- React/TypeScript 代码使用 [ESLint](https://eslint.org/) 和 [Prettier](https://prettier.io/)
- 提交信息遵循 [Conventional Commits](https://conventionalcommits.org/)

---

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)。

---

## 🙏 致谢

- [Wails](https://wails.io/) - 跨平台桌面应用框架
- [React](https://reactjs.org/) - 用户界面库
- [shadcn/ui](https://ui.shadcn.com/) - UI 组件库
- [kxn/claude-code-companion](https://github.com/kxn/claude-code-companion) - 本项目的早期后端服务来源
- [daodao97/code-switch](https://github.com/daodao97/code-switch) - 路由与应用设计的宝贵参考

---

## 📞 支持

- 📧 **邮箱**：support@cccc-proxy.dev
- 💬 **讨论**：[GitHub Discussions](https://github.com/whshang/claude-code-codex-companion/discussions)
- 🐛 **问题**：[GitHub Issues](https://github.com/whshang/claude-code-codex-companion/issues)

---

<div align="center">

**⭐ 如果这个项目对你有帮助，请给我们一个 Star！**

Made with ❤️ by the CCCC Team

</div>
