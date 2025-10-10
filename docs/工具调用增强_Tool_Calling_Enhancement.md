# 工具调用增强 | Tool Calling Enhancement

## 中文

当客户端请求包含 `tools` 时，CCCC 可以零配置自动注入系统提示、解析工具调用结果，并按照目标客户端格式回写响应。

### 主要能力

- 自动注入提示，引导模型使用触发信号与 XML 包裹函数调用。
- 仅对需要增强的端点执行，`tool_enhancement_mode` 支持 `auto` / `force` / `disable`。
- 解析结果并填充：
  - OpenAI：`choices[].message.tool_calls` + `finish_reason="tool_calls"`。
  - Anthropic：`content` 中的 `type="tool_use"` 块。
- 非流式路径已集成；流式增强在规划中。

### 学习机制

- `native_tool_support` 可显式配置或在 `/admin` 端点测试中学习。
- 如果运行时出现业务错误，会自动将端点标记为 `native_tool_support=false` 并强制启用增强。
- 日志增加 `tool_enhancement_applied`、`tool_calls_detected`、`tool_call_count` 等字段，便于排查。

### 代码与配置

- 实现入口：`internal/proxy/toolify_integration.go`、`internal/toolcall`。
- 相关配置字段详见《[配置指南](配置指南_Configuration_Guide.md)》。
- 建议通过 `go run ./cmd/probe-endpoints -config config.yaml` 检查端点的工具调用支持。

## English

When a client request includes `tools`, CCCC can inject system prompts, parse tool-call responses, and emit native-format results without global configuration.

### Capabilities

- Injects guidance so models emit trigger signals with XML-wrapped function calls.
- Enhancement runs only where needed; `tool_enhancement_mode` accepts `auto` / `force` / `disable`.
- Populates results for:
  - OpenAI: `choices[].message.tool_calls` with `finish_reason="tool_calls"`.
  - Anthropic: `content` blocks containing `type="tool_use"`.
- Non-streaming support is live; streaming support is on the roadmap.

### Learning

- `native_tool_support` can be set manually or discovered via `/admin` endpoint tests.
- Runtime business errors automatically flip the endpoint to `native_tool_support=false` and `tool_enhancement_mode=force`.
- Logs track `tool_enhancement_applied`, `tool_calls_detected`, and `tool_call_count`.

### Code & Configuration

- Core logic resides in `internal/proxy/toolify_integration.go` and `internal/toolcall`.
- Configuration fields are documented in the [Configuration Guide](配置指南_Configuration_Guide.md).
- Run `go run ./cmd/probe-endpoints -config config.yaml` to verify tool support per endpoint.
