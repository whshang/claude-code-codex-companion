# 配置指南 | Configuration Guide

## 核心理念：简单与明确

新版配置的核心理念是**简单与明确**。我们不再使用包含复杂规则的 `endpoints` 列表，而是为每种主要的上游 API 格式提供独立的、顶层的配置块。这使得配置意图一目了然，并从根本上消除了格式兼容性问题。

---

## 基础结构

`config.yaml` 的基础结构现在由 `server` 配置和几个独立的端点配置块组成。

```yaml
# 服务器监听地址
server:
  host: "127.0.0.1"
  port: 8080

# --- 端点配置 ---
# 为您需要使用的端点类型提供上游 URL 和 API 密钥。
# 如果一个配置块被注释掉或未提供，则对应的代理端点将不会被激活。

# 用于 /claude 代理端点
anthropic_endpoint:
  url: "https://api.anthropic.com"
  api_key: "YOUR_ANTHROPIC_API_KEY_HERE"

# 用于 /openai_chat 代理端点 (兼容 GPT-3.5/4, 88code 等)
openai_chat_endpoint:
  url: "https://api.openai.com"
  api_key: "YOUR_OPENAI_API_KEY_HERE"

# 用于 /openai_responses 代理端点 (兼容旧版格式)
# 默认不启用。如果需要，请取消注释并填入信息。
# openai_responses_endpoint:
#   url: "https://api.example.com/v1/responses"
#   api_key: "YOUR_API_KEY_FOR_RESPONSES_ENDPOINT"

# --- 其他配置 ---
logging:
  level: info
  log_directory: ./logs

# (其他如 timeouts, blacklist 等配置保持不变)
```

---

## 端点配置详解

每个端点配置块（如 `anthropic_endpoint`）都包含以下字段：

| 字段 | 类型 | 说明 | 是否必须 |
| :--- | :--- | :--- | :--- |
| `url` | `string` | 上游服务的 URL。推荐填写完整路径（如 `https://api.openai.com/v1/chat/completions`）。如果只填写域名，系统会尝试自动补全标准路径。 | 是 |
| `api_key` | `string` | 用于访问上游服务的 API 密钥。系统会自动将其作为 `Bearer` Token 添加到 `Authorization` 请求头中。 | 否 |

### URL 自动补全

为了简化配置，系统提供了 URL 自动补全功能。

- **如果您为 `openai_chat_endpoint` 的 `url` 字段提供了 `https://api.openai.com`**，系统在转发请求时会自动将其补全为 `https://api.openai.com/v1/chat/completions`。
- **如果您为 `anthropic_endpoint` 的 `url` 字段提供了 `https://api.anthropic.com`**，系统会自动补全为 `https://api.anthropic.com/v1/messages`。

**最佳实践**：始终在配置文件中提供完整的 URL，以保证行为的明确性。

---

## 废弃的配置

以下旧版配置字段已被**完全废弃**，在配置文件中存在也不会生效：

- `endpoints` 列表
- `supports_responses`
- `openai_preference`
- `url_anthropic` (在 `endpoints` 列表内部)
- `url_openai` (在 `endpoints` 列表内部)

所有与动态格式转换、学习和路由相关的功能都已被移除，以支持更简单、更稳定的静态代理模型。

---
## English Guide

## Core Philosophy: Simplicity and Clarity

The core philosophy of the new configuration is **simplicity and clarity**. We have moved away from a complex `endpoints` list with multiple rules. Instead, we now use separate, top-level configuration blocks for each major upstream API format. This makes the configuration's intent obvious and fundamentally eliminates format compatibility issues.

---

## Basic Structure

The basic structure of `config.yaml` now consists of the `server` configuration and several independent endpoint configuration blocks.

```yaml
# Server listen address
server:
  host: "127.0.0.1"
  port: 8080

# --- Endpoint Configurations ---
# Provide the upstream URL and API key for each endpoint type you intend to use.
# If a block is commented out or not provided, the corresponding proxy endpoint will not be activated.

# For the /claude proxy endpoint
anthropic_endpoint:
  url: "https://api.anthropic.com"
  api_key: "YOUR_ANTHROPIC_API_KEY_HERE"

# For the /openai_chat proxy endpoint (compatible with GPT-3.5/4, 88code, etc.)
openai_chat_endpoint:
  url: "https://api.openai.com"
  api_key: "YOUR_OPENAI_API_KEY_HERE"

# For the /openai_responses proxy endpoint (for legacy formats)
# Disabled by default. Uncomment and fill in if needed.
# openai_responses_endpoint:
#   url: "https://api.example.com/v1/responses"
#   api_key: "YOUR_API_KEY_FOR_RESPONSES_ENDPOINT"

# --- Other Configurations ---
logging:
  level: info
  log_directory: ./logs

# (Other configurations like timeouts, blacklist, etc., remain the same)
```

---

## Endpoint Configuration Details

Each endpoint configuration block (e.g., `anthropic_endpoint`) contains the following fields:

| Field | Type | Description | Required |
| :--- | :--- | :--- | :--- |
| `url` | `string` | The URL of the upstream service. Providing the full path is recommended (e.g., `https://api.openai.com/v1/chat/completions`). If you only provide the domain, the system will attempt to auto-complete the standard path. | Yes |
| `api_key` | `string` | The API key for accessing the upstream service. The system will automatically add this as a `Bearer` Token in the `Authorization` header. | No |

### URL Auto-Completion

To simplify configuration, the system provides a URL auto-completion feature.

- **If you provide `https://api.openai.com` for the `url` field of `openai_chat_endpoint`**, the system will auto-complete it to `https://api.openai.com/v1/chat/completions` when forwarding the request.
- **If you provide `https://api.anthropic.com` for the `url` field of `anthropic_endpoint`**, the system will auto-complete it to `https://api.anthropic.com/v1/messages`.

**Best Practice**: Always provide the full URL in the configuration file to ensure explicit and clear behavior.

---

## Deprecated Configurations

The following old configuration fields are now **completely deprecated** and will have no effect if they exist in the configuration file:

- The `endpoints` list
- `supports_responses`
- `openai_preference`
- `url_anthropic` (inside the `endpoints` list)
- `url_openai` (inside the `endpoints` list)

All features related to dynamic format conversion, learning, and routing have been removed in favor of a simpler, more stable static proxy model.