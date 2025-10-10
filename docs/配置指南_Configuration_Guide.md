# 配置指南 | Configuration Guide

## 中文

`config.yaml` 用于定义代理监听地址与上游端点。关键字段包括 `url_anthropic`、`url_openai`、`supports_responses`、`openai_preference`、`model_rewrite` 与 `parameter_overrides`。

### 基础结构

```yaml
server:
  host: 127.0.0.1
  port: 8080

endpoints:
  - name: primary
    url_anthropic: https://api.example.com/v1/messages
    url_openai: https://api.example.com/v1
    auth_type: auth_token
    auth_value: YOUR_API_KEY
    supports_responses: false
    openai_preference: auto
    tags: ["default"]
```

### 常用字段

| 字段 | 说明 |
| --- | --- |
| `supports_responses` | 指定端点是否原生支持 `/responses`，`true` 表示直接透传，`false` 表示自动降级到 `/chat/completions`。 |
| `openai_preference` | `auto` / `responses` / `chat_completions`，控制首选请求格式。 |
| `model_rewrite` | 使用通配符将客户端模型映射为供应商模型，详见《[模型重写设计](模型重写设计_Model_Rewrite_Design.md)》。 |
| `parameter_overrides` | 为特定端点覆盖 `temperature`、`max_tokens` 等参数。 |
| `native_tool_support` / `tool_enhancement_mode` | 控制工具调用增强策略，详见《[工具调用增强](工具调用增强_Tool_Calling_Enhancement.md)》。 |

### 配置流程

1. 在 `/admin/endpoints` 添加端点或导入 YAML。
2. 使用批量测试工具验证连通性与认证方式。
3. 当运行时学习到新偏好时，可通过“保存配置”将 `supports_responses`、`openai_preference` 等字段写回配置文件。

### 故障排查

如果请求总是降级，请确认上游是否真正支持 `/responses`，或手动将 `supports_responses` 设置为 `false`。

## English

`config.yaml` defines the proxy host and upstream endpoints. Key fields include `url_anthropic`, `url_openai`, `supports_responses`, `openai_preference`, `model_rewrite`, and `parameter_overrides`.

### Basic Structure

```yaml
server:
  host: 127.0.0.1
  port: 8080

endpoints:
  - name: primary
    url_anthropic: https://api.example.com/v1/messages
    url_openai: https://api.example.com/v1
    auth_type: auth_token
    auth_value: YOUR_API_KEY
    supports_responses: false
    openai_preference: auto
    tags: ["default"]
```

### Key Fields

| Field | Description |
| --- | --- |
| `supports_responses` | Whether the upstream natively serves `/responses`; `true` passes through, `false` downgrades to `/chat/completions`. |
| `openai_preference` | `auto` / `responses` / `chat_completions`, defines the preferred format. |
| `model_rewrite` | Maps client models to provider models via wildcards; see [Model Rewrite Design](模型重写设计_Model_Rewrite_Design.md). |
| `parameter_overrides` | Endpoint-level overrides for `temperature`, `max_tokens`, etc. |
| `native_tool_support` / `tool_enhancement_mode` | Governs tool-calling enhancement; see [Tool Calling Enhancement](工具调用增强_Tool_Calling_Enhancement.md). |

### Workflow

1. Add endpoints through `/admin/endpoints` or import YAML.
2. Run the batch probe to verify connectivity and authentication.
3. When runtime learning discovers preferences, use “Save Config” to persist values like `supports_responses` and `openai_preference`.

### Troubleshooting

If requests keep downgrading, verify that the upstream truly supports `/responses`, or explicitly set `supports_responses` to `false`.
