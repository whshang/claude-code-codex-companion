# OpenAI 集成设计 | OpenAI Integration Design

## 中文

为了让 Claude Code、Codex 等客户端无缝使用 OpenAI 兼容端点，CCCC 在请求与响应两个方向上执行格式转换，并结合降级状态机与学习机制保持兼容性。

### 核心流程

1. **请求识别**：通过 `utils.DetectRequestFormat` 判断客户端格式，如 OpenAI Responses、Chat Completions 或 Anthropic Messages。
2. **请求转换**：在 `internal/conversion/request_converter.go` 中调用 `RequestConverter` 将消息体转换为目标格式，并在必要时重写模型参数。
3. **流式处理**：当上游要求 SSE 时，使用 `StreamChatCompletionsToResponses` 或 `StreamAnthropicSSEToOpenAI` 将事件逐片转换并直接写回客户端。
4. **响应回写**：非流式响应由 `response_converter.go` 负责转换，并在日志中记录 `conversion_path`、`supports_responses_flag`。
5. **降级学习**：`proxy_logic` 中的状态机会在遇到 404/405 或包含 “unsupported” 的 400 时降级到 `/chat/completions`，同时更新 `supports_responses` 与 `openai_preference`。

### 关键配置

- `supports_responses` 与 `openai_preference` 决定是否优先尝试 `/responses`。
- `model_rewrite` 在转换前（请求）与回写后（响应）统一模型名称。
- `native_tool_support` 与工具增强逻辑协同，确保转换后依旧具备 Tool Calling 能力。

### 相关代码

- `internal/conversion/*`：请求/响应转换、SSE 管道。
- `internal/proxy/proxy_logic.go`：降级状态机、学习与日志策略。
- `internal/config/types.go`、`internal/proxy/server.go`：配置字段定义与持久化。

## English

To make Claude Code, Codex, and other clients interoperate with OpenAI-style endpoints, CCCC converts requests/responses in both directions while coordinating downgrade and learning logic.

### Core Flow

1. **Format detection**: `utils.DetectRequestFormat` determines whether the payload is OpenAI Responses, Chat Completions, or Anthropic Messages.
2. **Request conversion**: `internal/conversion/request_converter.go` uses `RequestConverter` to translate payloads and optionally rewrite model parameters.
3. **Streaming**: When SSE is required, `StreamChatCompletionsToResponses` and `StreamAnthropicSSEToOpenAI` convert chunks on the fly and stream them back to the client.
4. **Response rewrite**: Non-streaming responses are converted in `response_converter.go`, and logs capture `conversion_path` plus `supports_responses_flag`.
5. **Downgrade learning**: The state machine in `proxy_logic` downgrades to `/chat/completions` on 404/405 or 400 with “unsupported”, updating `supports_responses` and `openai_preference`.

### Key Configuration

- `supports_responses` and `openai_preference` control whether `/responses` is attempted first.
- `model_rewrite` normalises model names for both requests and responses.
- `native_tool_support` integrates with tool enhancement so that tool calls remain functional after conversion.

### Related Code

- `internal/conversion/*`: request/response converters and SSE pipelines.
- `internal/proxy/proxy_logic.go`: downgrade state machine, learning, logging.
- `internal/config/types.go`, `internal/proxy/server.go`: configuration fields and persistence logic.
