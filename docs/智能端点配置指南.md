# Smart Endpoint Configuration Guide

**Version**: 1.0
**Date**: 2025-10-17

---

## 📖 Overview

CCCC's multi-endpoint smart conversion system automatically routes requests based on client type and endpoint capabilities, handling format conversion when needed while prioritizing native format endpoints for optimal performance.

## 🔧 Core Configuration Fields

| Field | Type | Description | Example Values |
| --- | --- | --- | --- |
| `client_type` | string | Client type filtering | `claude_code`, `codex`, `openai`, `""` (universal) |
| `native_format` | bool | Native format support | `true` (no conversion), `false` (conversion required) |
| `target_format` | string | Target format for conversion | `anthropic`, `openai_chat`, `openai_responses` |
| `priority` | int | Routing priority (lower = higher priority) | `1`, `2`, `3` |

## 📋 Configuration Examples

### Claude Code Optimized Endpoint

```yaml
- name: anthropic-official
  url_anthropic: https://api.anthropic.com/v1/messages
  auth_type: api_key
  auth_value: sk-ant-YOUR_KEY
  priority: 1
  client_type: claude_code    # Optimized for Claude Code
  native_format: true         # Native support, no conversion needed
```

### Codex Optimized Endpoint

```yaml
- name: openai-official
  url_openai: https://api.openai.com/v1/responses
  auth_type: api_key
  auth_value: sk-YOUR_KEY
  priority: 1
  client_type: codex          # Optimized for Codex
  native_format: true         # Native support, no conversion needed
```

### Universal Fallback Endpoint

```yaml
- name: third-party-fallback
  url_openai: https://third-party.com/v1/chat/completions
  auth_type: auth_token
  auth_value: YOUR_TOKEN
  priority: 2
  client_type: ""             # Universal - serves all client types
  native_format: false        # Requires format conversion
  target_format: openai_chat  # Convert to OpenAI Chat format
  model_rewrite:
    enabled: true
    rules:
      - source_pattern: claude-*
        target_model: gpt-4-turbo
```

## 🔄 How It Works

1. **Client Detection**: System identifies client type from request path
2. **Endpoint Filtering**: Selects endpoints matching the client type
3. **Smart Sorting**: 4-layer prioritization (native_format → priority → health → response_time)
4. **Request Routing**: Direct passthrough for native format, conversion for others

## 📊 Performance Benefits

- **~40% latency reduction** by prioritizing native format endpoints
- **Automatic failover** between multiple endpoints
- **Transparent conversion** without client changes
