# Claude Code and Codex Companion (CCCC)

[中文版 / README.md](README.md)

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **A unified proxy layer for AI coding assistants**  
> Bridges Claude Code, Codex, and other Anthropic/OpenAI compatible clients with format adaptation, intelligent routing, runtime learning, and rich observability.

---

## 📖 Overview

CCCC (Claude Code and Codex Companion) is a **high-performance, easy-to-configure API proxy** for AI coding assistants. It provides a unified access point for Claude Code, Codex, and other clients compatible with OpenAI/Anthropic.

The core of the new architecture is **simplicity, clarity, and stability**. We have abandoned complex dynamic format conversion in favor of a fixed, one-way proxy model, ensuring that the behavior of every request is clear and predictable.

- 🎯 **Static Endpoint Proxy**: Provides three independent, fixed-function proxy endpoints: `/claude`, `/openai_chat`, and `/openai_responses`.
- ⚙️ **Simplified Configuration**: Uses a clear YAML structure to configure separate URLs and authentication details for each type of upstream service.
- 🛡️ **High Availability**: (Retained) Supports health checks, tag-based routing, automatic failover, and blacklisting.
- 📊 **Full Observability**: (Retained) Provides a web console, request logs, statistics dashboard, and debug bundle exports.

| Capability | Claude Code native | Codex native | CCCC (New) |
| --- | --- | --- | --- |
| Multi-endpoint load balancing | ❌ | ❌ | ✅ |
| Format auto-conversion | ❌ | ❌ | ❌ (Removed by design) |
| Static one-way proxy | ❌ | ❌ | ✅ |
| Web console / logs / stats | ❌ | ❌ | ✅ |

Deeper details live in the `docs/` folder.

---

## ✨ API Endpoints

This project provides the following proxy endpoints, which forward incoming requests to your configured upstream services. You need to select the correct endpoint that matches the API format of your client or target model.

| Local Endpoint | Upstream Format | Primary Use Case |
| :--- | :--- | :--- |
| `POST /claude` | Anthropic Messages | Proxy to Claude series models |
| `POST /openai_chat` | OpenAI Chat Completions | Proxy to modern models like GPT-3.5/4, etc. |
| `POST /openai_responses` | OpenAI Responses (Legacy) | Compatibility with older or specific model endpoints |

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

The `config.yaml` has been greatly simplified. You only need to provide the URL and API key for the endpoint types you intend to use.

```yaml
# Server listen address
server:
  host: "127.0.0.1"
  port: 8080

# --- Endpoint Configurations ---
# Provide the upstream URL and API key for each endpoint type you need.
# Providing the full URL is recommended, but the system will attempt to auto-complete it if missing.

anthropic_endpoint:
  url: "https://api.anthropic.com"
  api_key: "YOUR_ANTHROPIC_API_KEY_HERE"

openai_chat_endpoint:
  url: "https://api.openai.com"
  api_key: "YOUR_OPENAI_API_KEY_HERE"

# Endpoint for the legacy /responses format, disabled by default
# openai_responses_endpoint:
#   url: "https://api.example.com/v1/responses"
#   api_key: "YOUR_API_KEY_FOR_RESPONSES_ENDPOINT"

# --- Other Configurations ---
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
