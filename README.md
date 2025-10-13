# Claude Code and Codex Companion (CCCC)

[English Version / README_en.md](README_en.md)

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **统一的 AI 编程助手 API 转发代理**  
> 服务 Claude Code、Codex 以及其他 OpenAI/Anthropic 兼容客户端，提供透明代理、格式互转、运行时学习和可视化运维能力。

---

## 📖 项目简介

CCCC（Claude Code and Codex Companion）是一个面向 AI 编程助手的透明代理层，能够：

- 🔄 **自动格式转换**：在 `/v1/messages`、`/v1/chat/completions`、`/v1/responses` 之间双向转换，保障旧/新 API 兼容。
- 🎯 **智能路由与自适应**：依照客户端类型优选端点，学习并记忆 `/responses` 与 `/chat/completions` 的兼容策略。
- 🛡️ **高可用保障**：支持多端点、健康检查、标签路由、自动降级与黑名单策略。
- 🧠 **运行时学习**：记录参数/认证/格式偏好与 `supports_responses` 状态，可选择写回 `config.yaml`。
- ⚙️ **工具调用增强**：零配置开启 Tool Calling，自动注入提示并解析响应。
- 📊 **完整可观测性**：提供 Web 控制台、请求日志、统计面板与调试包导出。

| 能力 | Claude Code 原生 | Codex 原生 | CCCC |
| --- | --- | --- | --- |
| 多端点负载均衡 | ❌ | ❌ | ✅ |
| 格式互转 | ❌ | ❌ | ✅ |
| 模型重写 | ❌ | ❌ | ✅ |
| Tool Calling 增强 | ❌ | ❌ | ✅ |
| 运行时学习与持久化 | ❌ | ❌ | ✅ |
| Web 管理/日志/统计 | ❌ | ❌ | ✅ |

更多技术细节可参考 `docs/` 目录，如 [《透明代理优化计划》](docs/透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md)、[《Tool Calling 指南》](docs/工具调用增强_Tool_Calling_Enhancement.md) 等。

---

## ✨ 核心特性

### 双客户端兼容与格式转换

#### 端点智能路由
- **双 URL 配置**：单端点可同时配置 `url_anthropic` 与 `url_openai`，根据客户端类型自动路由
  - Claude Code 请求 → `url_anthropic`（Anthropic Messages API）
  - Codex 请求 → `url_openai`（OpenAI Responses/Chat Completions API）
- **单 URL 降级**：仅配置一个 URL 时，自动在代理侧进行格式转换
  - 仅 `url_anthropic`：Codex 请求转换为 Anthropic 格式后转发
  - 仅 `url_openai`：Claude Code 请求转换为 OpenAI 格式后转发

#### Responses API 自适应学习
- **首次请求**：优先尝试 `/v1/responses` 格式（OpenAI 新版 API）
- **智能降级**：遇到 404/405 或特定 400 错误时自动切换 `/v1/chat/completions`
- **学习记录**：成功格式持久化到 `openai_preference` 字段，下次直接使用
- **手动控制**：可显式设置 `supports_responses: true/false` 或 `openai_preference: responses/chat_completions/auto`

#### 流式响应转换（Codex 专用增强）
- **真实流式 SSE**：上游返回 `text/event-stream` 时
  - 自动转换 Chat Completions SSE → Responses API SSE
  - 保持流式数据完整性，无需缓存
- **模拟流式 SSE**：上游返回 JSON 但客户端期望流式时（`stream: true`）
  1. **格式探测**：识别 Chat Completions JSON（`choices`）或 Responses JSON（`output`）
  2. **格式转换**：Chat Completions JSON → Responses JSON
  3. **SSE 生成**：Responses JSON → Responses API SSE 流
  4. **立即返回**：写入并刷新 SSE 数据后立即返回，避免被后续逻辑覆盖
- **事件完整性**：确保发送 `response.created`、`response.output_text.delta`、`response.completed` 三个核心事件

详见 [OpenAI 集成设计](docs/OpenAI集成设计_OpenAI_Integration_Design.md)、[SSE 重构设计](docs/SSE重构设计_SSE_Refactor_Design.md)。

### 智能路由与自适应系统
- 标签路由、优先级控制、健康检查与自动黑名单。
- `auth_type: auto` 动态尝试 `x-api-key` / `Authorization` 并记录成功结果。
- 404/405 或特定 400 错误会自动降级 `/responses`，其他错误不会误伤。
- 详见 [标签路由设计](docs/标签路由设计_Tag_based_Routing_Design.md)、[认证方式自动学习](docs/认证方式自动学习_Auth_Method_Auto_Learning.md)。

### 运行时学习与配置持久化
- 识别 400 错误中的不兼容参数（如 `tools`、`tool_choice`），立即移除并重试。
- 记录 `supports_responses`、认证方式、`openai_preference`、`count_tokens` 能力并可回写配置。
- `/admin` 中提供“保存配置”按钮同步学习结果。
- 详见 [学习持久化实现](docs/学习持久化实现_Learning_Persistence_Implementation.md)。

### Tool Calling 增强
- 请求包含 `tools` 时自动注入提示，解析返回结果，并在 OpenAI/Anthropic 格式内回写。
- `native_tool_support` + `tool_enhancement_mode` 控制针对每个端点的策略，支持自动学习。
- 日志记录 `tool_enhancement_applied`、`tool_call_count` 等指标。
- 详见 [Tool Calling 指南](docs/工具调用增强_Tool_Calling_Enhancement.md)。

### 高级配置能力
- 模型重写：通配符映射、隐式默认映射、响应模型回写。
- 参数覆盖：按端点覆盖 `temperature`、`max_tokens` 等。
- 双 URL 单端点：同时配置 `url_anthropic` 与 `url_openai`，缺失时自动格式转换。
- 详见 [配置指南](docs/配置指南_Configuration_Guide.md)、[模型重写设计](docs/模型重写设计_Model_Rewrite_Design.md)。

### 可观测性与调试
- `/admin` 提供仪表盘、端点管理、请求日志、测试向导。
- `go run ./cmd/test_endpoints -config config.yaml -json` 一键批量探针、验证认证。
- “导出调试信息” 生成 ZIP 包含请求/响应、端点配置与 Tagger 信息。
- 日志与统计持久化使用 SQLite（`logs.db`、`statistics.db`）。
- 详见 [日志增强设计](docs/被拉黑端点日志增强_Blacklisted_Endpoint_Logging_Enhancements.md)、[调试信息导出](docs/调试信息导出_Debug_Export.md)。

---

## 🚀 快速开始

### 安装与启动
```bash
git clone https://github.com/whshang/claude-code-codex-companion.git
cd claude-code-codex-companion
go build -o claude-code-codex-companion .
# 或 make build

./claude-code-codex-companion -config config.yaml
```

默认监听 `127.0.0.1:8080`，控制台地址 `http://127.0.0.1:8080/admin/`。

### 一键配置脚本
- 访问 `http://127.0.0.1:8080/help` 下载跨平台脚本。
- 示例：
  ```bash
  ./cccc-setup-claude-code.sh --url http://127.0.0.1:8080 --key hello
  ./cccc-setup-codex.sh --url http://127.0.0.1:8080 --key hello
  ./cccc-setup.sh --url http://127.0.0.1:8080 --key hello
  ```
- 脚本会备份并更新 `~/.claude/settings.json`、`~/.codex/config.toml`、`auth.json`。

### 配置示例
```yaml
server:
  host: 127.0.0.1
  port: 8080

endpoints:
  - name: universal-provider
    url_anthropic: https://api.provider.com/anthropic
    url_openai: https://api.provider.com/openai
    auth_type: auto
    auth_value: your-token
    supports_responses: false
    openai_preference: auto
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: claude-*sonnet*
          target_model: qwen-plus
        - source_pattern: gpt-5*
          target_model: qwen-max

logging:
  level: info
  log_directory: ./logs
```

更多字段与示例见 [《配置指南》](docs/配置指南_Configuration_Guide.md)。

---

## 🔌 客户端与生态
- **Claude Code**：脚本生成后立即可用；若需手动设置或企业代理，参考 [《Codex 配置指南》](docs/Codex配置指南_Codex_Configuration_Guide.md) 中的说明。
- **Codex CLI**：脚本写入 `~/.codex/config.toml`，`wire_api` 默认为 `responses`，可按项目设置 `trust_level`。
- **其他 IDE/CLI**：Cursor、Continue、Aider 等接入 OpenAI 兼容接口即可，可参考 [FoxCode 端点说明](docs/FoxCode端点说明_FoxCode_Endpoint_Notes.md) 与 [88code 端点示例](docs/88code端点示例_88code_Endpoint_Example.md)。
- **探针工具**：`go run ./cmd/test_endpoints -config config.yaml -json` 评估连通性、认证、工具调用支持。

---

## 🧭 高级主题索引
- 透明代理、降级状态机：[透明代理优化计划](docs/透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md)
- Tool Calling 增强与指标：[工具调用增强](docs/工具调用增强_Tool_Calling_Enhancement.md)
- 认证与参数学习：[认证方式自动学习](docs/认证方式自动学习_Auth_Method_Auto_Learning.md)
- SSE 流式转换：[SSE 重构设计](docs/SSE重构设计_SSE_Refactor_Design.md)
- GORM 与统计存储：[GORM 重构规划](docs/GORM重构规划_GORM_Refactor_Plan.md)、[统计持久化设计](docs/统计持久化设计_Statistics_Persistence_Design.md)
- 验证步骤与脚本：[功能验证步骤](docs/功能验证步骤_Verification_Steps.md)、[端点向导](docs/端点向导_Endpoint_Wizard.md)

---

## 🤝 贡献与开发
- 设计文档集中在 `docs/`，可从 [《系统设计概览》](docs/系统设计概览_System_Design_Overview.md) 与 [《实施计划摘要》](docs/实施计划摘要_Implementation_Plan_Summary.md) 开始。
- 提交前请执行 `go test ./...`，并遵循 [《提交前检查清单》](docs/提交前检查清单_Pre_commit_Checklist.md)。
- 欢迎通过 Issue / PR 分享端点案例、脚本或翻译，提升生态。

---

## 📝 更新日志

项目采用日期分组记录，详见 [CHANGELOG.md](CHANGELOG.md)。

---

## 🙏 致谢

- **基础组件**：基于 [Gin](https://github.com/gin-gonic/gin) 构建 HTTP 服务，使用 SQLite 记录日志与统计。
- **社区协作**：感谢 Toolify 社区在工具调用增强、日志设计方面的灵感与协作。
- **相关项目**：CCCC fork 自 [kxn/claude-code-companion](https://github.com/kxn/claude-code-companion)，并兼容 Claude Code 与 OpenAI Codex CLI 的生态。

如果这个项目对你有帮助，请考虑点个 ⭐ 支持。Made with ❤️ for Claude Code & Codex users.
