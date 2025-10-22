# Claude Code and Codex Companion (CCCC)

[中文版 / README.md](README.md)

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **Unified API proxy for AI coding assistants**
> Serves Claude Code, Codex, and other OpenAI/Anthropic compatible clients with intelligent endpoint selection, conditional format conversion, and full observability.

---

## 📖 Overview

CCCC (Claude Code and Codex Companion) is a **high-performance, intelligent API proxy** for AI coding assistants. It provides a unified access point for Claude Code, Codex, and other OpenAI/Anthropic compatible clients.

The core architecture is a **multi-endpoint smart conversion system**. The system maintains an endpoint pool, marks each endpoint with its native supported format, and intelligently selects the optimal endpoint based on client type:

- 🎯 **Smart Endpoint Selection**: 4-layer sorting algorithm (native_format → priority → health → response_time), prioritizes endpoints that don't require conversion, ~40% performance improvement.
- 🔄 **Conditional Format Conversion**: Only converts formats when necessary (native_format=false), avoiding unnecessary performance overhead.
- 🏷️ **Client Filtering**: Supports configuring client_type for endpoints (claude_code/codex/openai/universal) for precise routing.
- 🛡️ **High Availability**: Supports multi-endpoint load balancing, health checks, tag-based routing, automatic failover, and blacklisting.
- 📊 **Full Observability**: Provides web console, request logs, statistics dashboard, and debug bundle exports with endpoint visualization.

| Capability | Claude Code Native | Codex Native | CCCC |
| --- | --- | --- | --- |
| Multi-endpoint load balancing | ❌ | ❌ | ✅ |
| Smart format conversion | ❌ | ❌ | ✅ (conditional) |
| Performance-optimized routing | ❌ | ❌ | ✅ (native priority) |
| Web console / logs / stats | ❌ | ❌ | ✅ |

Technical details are available in the `docs/` directory.

---

## ✨ Core Features: Smart Endpoint Selection

CCCC provides three core fields for each endpoint to enable intelligent routing:

| Field | Type | Description | Example Values |
| --- | --- | --- | --- |
| `client_type` | string | Client type filtering | `claude_code`, `codex`, `openai`, `""` (universal) |
| `native_format` | bool | Whether endpoint natively supports client format | `true` (no conversion needed), `false` (conversion required) |
| `target_format` | string | Target format for conversion | `anthropic`, `openai_chat`, `openai_responses` |

### Smart Selection Process

```
1. Client request → Identify client type (claude_code/codex/openai)
2. Filter endpoint pool → Select candidates by client_type
3. 4-layer sorting:
   ├─ native_format=true priority (optimal performance, ~40% improvement)
   ├─ priority ascending (custom user priority)
   ├─ health status priority (exclude faulty endpoints)
   └─ response time ascending (select fastest endpoint)
4. Select endpoint → Execute request
   ├─ native_format=true → Direct passthrough (fast path)
   └─ native_format=false → Format conversion then forwarding
```

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

### Configuration Example

The `config.yaml` uses a multi-endpoint smart conversion architecture, supporting intelligent routing field configuration for each endpoint:

```yaml
server:
  host: 127.0.0.1
  port: 8081
  auto_sort_endpoints: true  # Enable dynamic endpoint sorting

endpoints:
  # Claude Code optimized endpoint - native support
  - name: anthropic-official
    url_anthropic: https://api.anthropic.com/v1/messages
    auth_type: api_key
    auth_value: YOUR_ANTHROPIC_API_KEY
    enabled: true
    priority: 1
    client_type: claude_code    # Optimized for Claude Code
    native_format: true          # Native support, no conversion

  # Codex optimized endpoint - native support
  - name: openai-official
    url_openai: https://api.openai.com/v1/responses
    auth_type: api_key
    auth_value: YOUR_OPENAI_API_KEY
    enabled: true
    priority: 1
    client_type: codex          # Optimized for Codex
    native_format: true          # Native support, no conversion

  # Universal endpoint - requires conversion
  - name: universal-provider
    url_openai: https://api.provider.com/v1/chat/completions
    auth_type: auth_token
    auth_value: YOUR_TOKEN
    enabled: true
    priority: 2
    client_type: ""              # Empty = universal, supports all clients
    native_format: false         # Requires format conversion
    target_format: openai_chat   # Convert to OpenAI Chat format
    model_rewrite:               # Optional: model name rewriting
      enabled: true
      rules:
        - source_pattern: claude-*
          target_model: provider-claude-model

logging:
  level: info
  log_directory: ./logs
```

**Configuration Notes**:
- `client_type`: Leave empty for universal endpoints that support all clients; specify values to serve only that client type
- `native_format`: `true` means the endpoint natively supports the client format, system prioritizes these for best performance
- `target_format`: Specifies conversion target format when `native_format=false`

Full configuration examples available in `config.yaml.example`.

---

## 🌐 API Endpoint Paths

CCCC uses **transparent proxy** mode - clients use standard API paths:

### Claude Code Client

```bash
# Claude Code automatically uses /v1/messages path
base_url: http://127.0.0.1:8081
```

Actual request path: `http://127.0.0.1:8081/v1/messages`

### Codex Client

```bash
# Codex automatically uses /responses path
base_url: http://127.0.0.1:8081
```

Actual request path: `http://127.0.0.1:8081/responses` or `http://127.0.0.1:8081/v1/responses`

### OpenAI Compatible Clients

```bash
# Use standard OpenAI Chat Completions path
base_url: http://127.0.0.1:8081
```

Actual request path: `http://127.0.0.1:8081/chat/completions` or `http://127.0.0.1:8081/v1/chat/completions`

**How it works**:
1. System automatically identifies client type based on request path
2. SmartSelector chooses optimal endpoint (prioritizing native_format=true)
3. Format conversion happens automatically if needed, transparent to client
4. Clients work out-of-the-box without any changes

---

## 🔌 Clients & Ecosystem
- **Claude Code**: Use the setup script or configure manually (see [Codex Configuration Guide](docs/Codex配置指南.md) for environment variables).
- **Codex CLI**: The script writes `~/.codex/config.toml`; the default `wire_api` is `responses` and per-project `trust_level` can be customised.
- **Other IDE/CLI tools** (Cursor, Continue, Aider, etc.): point them to the OpenAI-compatible CCCC endpoint; see [FoxCode Endpoint Notes](docs/FoxCode端点说明.md) and [88code Endpoint Example](docs/88code端点示例.md).
- **Probes**: `go run ./cmd/test_endpoints -config config.yaml -json` validates availability, auth, and tool calling support.

---

## 🧭 Advanced Topics Index
- **Core Architecture**: [Smart Endpoint Selection](docs/动态端点排序.md), [Endpoint Management](docs/端点测试与优化指南.md)
- **Proxy Mechanisms**: [SSE Refactor Design](docs/SSE重构设计.md), [Codex Tool Call Streaming Fix](docs/Codex流式工具调用修复_2025-10-10.md)
- Dynamic endpoint sorting: [Dynamic Endpoint Sorting](docs/动态端点排序.md)
- Authentication & parameter learning: [Auto Learning](docs/认证方式自动学习.md)
- SSE streaming conversion: [SSE Refactor Design](docs/SSE重构设计.md)
- Persistence & statistics: [Learning Persistence Implementation](docs/学习持久化实现.md), [Statistics Persistence Design](docs/统计持久化设计.md)
- Smart endpoint configuration: [Smart Endpoint Configuration Guide](docs/智能端点配置指南.md)
- Validation scripts & endpoint wizard: [Verification Steps](docs/功能验证步骤.md), [Endpoint Wizard](docs/端点向导.md)

---

## 🤝 Contributing
- Architectural notes live in `docs/`. Start with [System Design Overview](docs/系统设计概览.md) and [Smart Endpoint Configuration Guide](docs/智能端点配置指南.md).
- Run `go test ./...` before opening a PR and make sure scripts and examples stay in sync with your changes.
- PRs, issues, and discussion posts tracking new endpoint examples, scripts, or translations are warmly welcomed.

---

## 📝 Changelog

Changes are grouped by date in [CHANGELOG.md](CHANGELOG.md).

---

## 🙏 Acknowledgements

- **Foundation**: Built with [Gin](https://github.com/gin-gonic/gin) and SQLite for persistence.
- **Upstream inspiration**: CCCC forked from [kxn/claude-code-companion](https://github.com/kxn/claude-code-companion) and aims to stay compatible with Claude Code and OpenAI Codex CLI ecosystems.

If this project helps you, please consider starring ⭐ the repository. Made with ❤️ for Claude Code & Codex users.
