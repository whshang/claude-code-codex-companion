# Codex 配置指南 | Codex Configuration Guide

## 中文

Codex CLI 走的是 Anthropic 协议，因此可以直接复用 CCCC 为 Claude Code 提供的代理层。推荐从 `/help` 页面下载 `cccc-setup-codex.sh` 脚本自动生成配置，该脚本会：

1. 创建或更新 `~/.codex/config.toml`（内部为 JSON 结构，写入 `env.ANTHROPIC_BASE_URL` 等变量）。
2. 备份旧配置，例如 `config.toml.backup-<timestamp>`。
3. 视情况生成 `~/.codex/auth.json` 用于存放 API Key。

若需手动配置：

- 设置环境变量：
  ```bash
  export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
  ```
- 编辑 `~/.codex/config.toml`：
  ```json
  {
    "env": {
      "ANTHROPIC_BASE_URL": "http://127.0.0.1:8080"
    },
    "hooks": {},
    "permissions": {
      "allow": [],
      "deny": []
    }
  }
  ```
- 如需按项目定制权限或标签，可在 CCCC 的 `config.yaml` 中结合标签路由与 `supports_responses`。

验证方式：

1. 执行 `codex --version` 确认 CLI 正常运行。
2. 在 `/admin/endpoints` 中查看请求记录，确认 `client_type=codex`，且必要时路径已自动从 `/responses` 转换为 `/chat/completions`。

## English

Codex CLI speaks the Anthropic protocol and can share the same CCCC proxy used for Claude Code. Download `cccc-setup-codex.sh` from the `/help` page to bootstrap configuration; the script will:

1. Create or update `~/.codex/config.toml` (JSON structure containing `env.ANTHROPIC_BASE_URL`, etc.).
2. Back up the previous configuration as `config.toml.backup-<timestamp>`.
3. Optionally generate `~/.codex/auth.json` to store the API key.

Manual setup steps:

- Export the environment variable:
  ```bash
  export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
  ```
- Edit `~/.codex/config.toml`:
  ```json
  {
    "env": {
      "ANTHROPIC_BASE_URL": "http://127.0.0.1:8080"
    },
    "hooks": {},
    "permissions": {
      "allow": [],
      "deny": []
    }
  }
  ```
- For advanced routing, adjust `config.yaml` in CCCC with tags, priorities, or `supports_responses`.

Validation checklist:

1. Run `codex --version` to ensure the CLI works with the proxy.
2. Inspect `/admin/endpoints` to confirm `client_type=codex` and that requests downgrade from `/responses` to `/chat/completions` when necessary.
