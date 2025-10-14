# Claude Code and Codex Companion (CCCC)

[中文版 / README.md](README.md)

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **A unified proxy layer for AI coding assistants**  
> Bridges Claude Code, Codex, and other Anthropic/OpenAI compatible clients with format adaptation, intelligent routing, runtime learning, and rich observability.

---

## 📖 Overview

CCCC (Claude Code and Codex Companion) delivers:

- 🔄 **Format conversion** between `/v1/messages`, `/v1/chat/completions`, and `/v1/responses`, keeping old and new APIs transparent to clients.
- 🎯 **Adaptive routing** that selects endpoints per client, learns `/responses` vs `/chat/completions` preferences, and persists them when desired.
- 🛡️ **High availability** via multi-endpoint load balancing, health checks, tag-based routing, and granular blacklist strategies.
- 🧠 **Runtime learning** of parameters, authentication headers, `supports_responses`, and `count_tokens` capabilities, with optional `config.yaml` persistence.
- 📊 **Operations toolkit** including a web console, request logs, statistics dashboards, and debug bundle exports.

| Capability | Claude Code native | Codex native | CCCC |
| --- | --- | --- | --- |
| Multi-endpoint load balancing | ❌ | ❌ | ✅ |
| Format auto-conversion | ❌ | ❌ | ✅ |
| Model rewriting | ❌ | ❌ | ✅ |
| Runtime learning & persistence | ❌ | ❌ | ✅ |
| Web console / logs / stats | ❌ | ❌ | ✅ |

Deeper details live in the `docs/` folder, e.g. [Transparent Proxy Optimisation Plan](docs/透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md) and more.

---

## ✨ Core Features

### Dual-client compatibility
- Detects Claude Code / Codex / OpenAI payloads and converts between `/responses`, `/chat/completions`, and `/messages` when needed.
- Supports dual URLs (`url_anthropic` + `url_openai`) or a single URL with transparent conversion.
- Learns `supports_responses` and `openai_preference` at runtime and persists them if desired.
- See [OpenAI Integration Design](docs/OpenAI集成设计_OpenAI_Integration_Design.md) for the conversion flow.

### Intelligent routing & self-adaptation
- Tag-based routing, priority control, health checks, and fine-grained blacklist configuration.
- `auth_type: auto` tries `x-api-key` and `Authorization`, remembering the winning strategy.
- 404/405 or keyword-filtered 400 errors trigger `/responses` downgrades; other errors avoid false positives.
- See [Tag-based Routing Design](docs/标签路由设计_Tag_based_Routing_Design.md) and [Auth Method Auto-Learning](docs/认证方式自动学习_Auth_Method_Auto_Learning.md).

### Runtime learning & persistence
- Learns unsupported parameters from 400 errors (e.g. `tools`, `tool_choice`), strips them, and retries instantly.
- Records `supports_responses`, auth headers, `openai_preference`, and `count_tokens` support; `/admin` can save them back to `config.yaml`.
- See [Learning Persistence Implementation](docs/学习持久化实现_Learning_Persistence_Implementation.md).

### Advanced configuration
- Model rewriting (wildcards, implicit defaults, response rewrites).
- Parameter overrides per endpoint (`temperature`, `max_tokens`, etc.).
- Dual URL support: configure both Anthropic and OpenAI endpoints in a single entry.
- See [Configuration Guide](docs/配置指南_Configuration_Guide.md) and [Model Rewrite Design](docs/模型重写设计_Model_Rewrite_Design.md).

### Observability & diagnostics
- `/admin` offers dashboards, endpoint management, request logs, and testing wizards.
- Batch probe via `go run ./cmd/test_endpoints -config config.yaml -json` to validate connectivity and auth.
- “Export Debug Info” packages raw and converted payloads, endpoint configs, and taggers into a ZIP.
- Logs and statistics persist in SQLite (`logs.db`, `statistics.db`).
- See [Logging Enhancements](docs/被拉黑端点日志增强_Blacklisted_Endpoint_Logging_Enhancements.md) and [Debug Export](docs/调试信息导出_Debug_Export.md).

---

## 🚀 Quick Start

### Install & launch
```bash
git clone https://github.com/whshang/claude-code-codex-companion.git
cd claude-code-codex-companion
go build -o claude-code-codex-companion .
# or make build

./claude-code-codex-companion -config config.yaml
```

The proxy listens on `127.0.0.1:8080` by default; the admin console lives at `http://127.0.0.1:8080/admin/`.

### One-click client scripts
- Visit `http://127.0.0.1:8080/help` to download platform-specific scripts.
- Examples:
  ```bash
  ./cccc-setup-claude-code.sh --url http://127.0.0.1:8080 --key hello
  ./cccc-setup-codex.sh --url http://127.0.0.1:8080 --key hello
  ./cccc-setup.sh --url http://127.0.0.1:8080 --key hello
  ```
- Scripts back up and regenerate `~/.claude/settings.json`, `~/.codex/config.toml`, and `auth.json`.

### Sample configuration
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

See the [Configuration Guide](docs/配置指南_Configuration_Guide.md) for field descriptions and advanced examples.

---

## 🔌 Clients & ecosystem
- **Claude Code**: Use the setup script or configure manually (see [Codex Configuration Guide](docs/Codex配置指南_Codex_Configuration_Guide.md) for environment variables).
- **Codex CLI**: The script writes `~/.codex/config.toml`; the default `wire_api` is `responses` and per-project `trust_level` can be customised.
- **Other IDE/CLI tools** (Cursor, Continue, Aider, etc.): point them to the OpenAI-compatible CCCC endpoint; see [FoxCode Endpoint Notes](docs/FoxCode端点说明_FoxCode_Endpoint_Notes.md) and [88code Endpoint Example](docs/88code端点示例_88code_Endpoint_Example.md).
- **Probes**: `go run ./cmd/test_endpoints -config config.yaml -json` validates availability, auth, and tool calling support.

---

## 🧭 Advanced topic index
- Transparent proxy & downgrade state machine: [Transparent Proxy Optimisation Plan](docs/透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md)
- Auth/parameter learning & persistence: [Learning Persistence Implementation](docs/学习持久化实现_Learning_Persistence_Implementation.md)
- SSE streaming refactor: [SSE Refactor Design](docs/SSE重构设计_SSE_Refactor_Design.md)
- GORM storage & statistics: [GORM Refactor Plan](docs/GORM重构规划_GORM_Refactor_Plan.md), [Statistics Persistence Design](docs/统计持久化设计_Statistics_Persistence_Design.md)
- Validation scripts & endpoint wizard: [Verification Steps](docs/功能验证步骤_Verification_Steps.md), [Endpoint Wizard](docs/端点向导_Endpoint_Wizard.md)

---

## 🤝 Contributing
- Architectural notes live in `docs/`. Start with [System Design Overview](docs/系统设计概览_System_Design_Overview.md) and [Implementation Plan Summary](docs/实施计划摘要_Implementation_Plan_Summary.md).
- Run `go test ./...` before opening a PR and follow the [Pre-commit Checklist](docs/提交前检查清单_Pre_commit_Checklist.md).
- PRs, issues, and discussion posts tracking new endpoint examples, scripts, or translations are warmly welcomed.

---

## 📝 Changelog

Changes are grouped by date in [CHANGELOG.md](CHANGELOG.md).

---

## 🙏 Acknowledgements

- **Foundation**: Built with [Gin](https://github.com/gin-gonic/gin) and SQLite for persistence.
- **Upstream inspiration**: CCCC forked from [kxn/claude-code-companion](https://github.com/kxn/claude-code-companion) and aims to stay compatible with Claude Code and OpenAI Codex CLI ecosystems.

If this project helps you, please consider starring ⭐ the repository. Made with ❤️ for Claude Code & Codex users.
