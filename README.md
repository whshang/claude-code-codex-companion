# Claude Code and Codex Companion (CCCC)

[English Version / README_en.md](README_en.md)

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **统一的 AI 编程助手 API 转发代理**  
> 服务 Claude Code、Codex 以及其他 OpenAI/Anthropic 兼容客户端，提供透明代理、格式互转、运行时学习和可视化运维能力。

---

## 📖 项目简介

CCCC（Claude Code and Codex Companion）是一个面向 AI 编程助手的**高性能、配置简单的 API 代理**。它为 Claude Code、Codex 以及其他 OpenAI/Anthropic 兼容的客户端提供统一的接入点。

新架构的核心是**简单、明确、稳定**。我们摒弃了复杂的动态格式转换，采用固定的单向代理模式，确保每一个请求的行为都清晰可预测。

- 🎯 **静态端点代理**：提供 `/claude`, `/openai_chat`, `/openai_responses` 三个独立的、功能固定的代理端点。
- ⚙️ **简化配置**：采用清晰的 YAML 结构，为每种上游服务类型配置独立的 URL 和认证信息。
- 🛡️ **高可用保障**：(保留) 支持多端点、健康检查、标签路由、自动降级与黑名单策略。
- 📊 **完整可观测性**：(保留) 提供 Web 控制台、请求日志、统计面板与调试包导出。

| 能力 | Claude Code 原生 | Codex 原生 | CCCC (新版) |
| --- | --- | --- | --- |
| 多端点负载均衡 | ❌ | ❌ | ✅ |
| 格式互转 | ❌ | ❌ | ❌ (设计上移除) |
| 静态单向代理 | ❌ | ❌ | ✅ |
| Web 管理/日志/统计 | ❌ | ❌ | ✅ |

更多技术细节可参考 `docs/` 目录。

---

## ✨ API 端点

本项目提供以下代理端点，将传入的请求转发到您配置的上游服务。您需要根据客户端或目标模型的 API 格式，选择正确的端点进行调用。

| 本地端点 | 上游格式 | 主要用途 |
| :--- | :--- | :--- |
| `POST /claude` | Anthropic Messages | 代理到 Claude 系列模型 |
| `POST /openai_chat` | OpenAI Chat Completions | 代理到 GPT-3.5/4 等现代模型 |
| `POST /openai_responses` | OpenAI Responses (Legacy) | 兼容旧版或特定的模型端点 |

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
`config.yaml` 的配置已大幅简化。您只需为需要使用的端点类型提供 URL 和 API 密钥。

```yaml
# 服务器监听地址
server:
  host: 127.0.0.1
  port: 8080

# --- 端点配置 ---
# 为您需要使用的端点类型提供上游 URL 和 API 密钥。
# URL 推荐使用完整路径，如果缺失，系统会尝试自动补全。

anthropic_endpoint:
  url: "https://api.anthropic.com"
  api_key: "YOUR_ANTHROPIC_API_KEY_HERE"

openai_chat_endpoint:
  url: "https://api.openai.com"
  api_key: "YOUR_OPENAI_API_KEY_HERE"

# 为旧版 /responses 格式准备的端点，默认不启用
# openai_responses_endpoint:
#   url: "https://api.example.com/v1/responses"
#   api_key: "YOUR_API_KEY_FOR_RESPONSES_ENDPOINT"

# --- 其他配置 ---
logging:
  level: info
  log_directory: ./logs
```

更多字段与示例（如 `streaming`、`tools`、`http_client`、`monitoring` 等）见 [《配置指南》](docs/配置指南_Configuration_Guide.md)。

---

## 🔌 客户端与生态
- **Claude Code**：脚本生成后立即可用；若需手动设置或企业代理，参考 [《Codex 配置指南》](docs/Codex配置指南_Codex_Configuration_Guide.md) 中的说明。
- **Codex CLI**：脚本写入 `~/.codex/config.toml`，`wire_api` 默认为 `responses`，可按项目设置 `trust_level`。
- **其他 IDE/CLI**：Cursor、Continue、Aider 等接入 OpenAI 兼容接口即可，可参考 [FoxCode 端点说明](docs/FoxCode端点说明_FoxCode_Endpoint_Notes.md) 与 [88code 端点示例](docs/88code端点示例_88code_Endpoint_Example.md)。
- **探针工具**：`go run ./cmd/test_endpoints -config config.yaml -json` 评估连通性、认证、工具调用支持。

---

## 🧭 高级主题索引
- 透明代理、降级状态机：[透明代理优化计划](docs/透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md)
- 动态端点排序：[动态端点排序](docs/动态端点排序_Dynamic_Endpoint_Sorting.md)
- 认证与参数学习：[认证方式自动学习](docs/认证方式自动学习_Auth_Method_Auto_Learning.md)
- SSE 流式转换：[SSE 重构设计](docs/SSE重构设计_SSE_Refactor_Design.md)
- GORM 与统计存储：[GORM 重构规划](docs/GORM重构规划_GORM_Refactor_Plan.md)、[统计持久化设计](docs/统计持久化设计_Statistics_Persistence_Design.md)
- 验证步骤与脚本：[功能验证步骤](docs/功能验证步骤_Verification_Steps.md)、[端点向导](docs/端点向导_Endpoint_Wizard.md)
- 格式转换与端点兼容性：[格式转换与端点兼容性](docs/格式转换与端点兼容性_Format_Conversion_and_Endpoint_Compatibility.md)
- 端点测试与优化：[端点测试与优化指南](docs/端点测试与优化指南_Endpoint_Testing_and_Optimization_Guide.md)

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
- **相关项目**：CCCC fork 自 [kxn/claude-code-companion](https://github.com/kxn/claude-code-companion)，并兼容 Claude Code 与 OpenAI Codex CLI 的生态。

如果这个项目对你有帮助，请考虑点个 ⭐ 支持。Made with ❤️ for Claude Code & Codex users.
