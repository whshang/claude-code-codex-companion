# OpenAI 集成设计 | OpenAI Integration Design

## 中文

为了让 Claude Code、Codex 等客户端无缝使用 OpenAI 兼容端点，CCCC 在请求与响应两个方向上执行格式转换，并结合降级状态机与学习机制保持兼容性。

### 核心流程

1. **请求识别**：通过 `utils.DetectRequestFormat` 判断客户端格式，如 OpenAI Responses、Chat Completions 或 Anthropic Messages。
2. **端点智能路由**：
   - **双 URL 配置**：单端点可同时配置 `url_anthropic` 与 `url_openai`
     - Claude Code 请求 → `url_anthropic`（Anthropic Messages API）
     - Codex 请求 → `url_openai`（OpenAI Responses/Chat Completions API）
   - **单 URL 降级**：仅配置一个 URL 时，自动在代理侧进行格式转换
     - 仅 `url_anthropic`：Codex 请求转换为 Anthropic 格式后转发
     - 仅 `url_openai`：Claude Code 请求转换为 OpenAI 格式后转发
3. **请求转换**：在 `internal/conversion/request_converter.go` 中调用 `RequestConverter` 将消息体转换为目标格式，并在必要时重写模型参数。
4. **流式处理**：
   - **真实流式**：上游返回 `text/event-stream` 时，使用 `StreamChatCompletionsToResponses` 或 `StreamAnthropicSSEToOpenAI` 将事件逐片转换并直接写回客户端
   - **模拟流式**：上游返回 JSON 但客户端期望流式（`stream: true`）时，执行三阶段转换：
     1. 格式探测：识别 Chat Completions JSON（`choices`）或 Responses JSON（`output`）
     2. 格式转换：Chat Completions JSON → Responses JSON
     3. SSE 生成：Responses JSON → SSE 流并立即返回
5. **响应回写**：非流式响应由 `response_converter.go` 负责转换，并在日志中记录 `conversion_path`、`supports_responses_flag`。
6. **降级学习**：`proxy_logic` 中的状态机会在遇到 404/405 或包含 "unsupported" 的 400 时降级到 `/chat/completions`，同时更新 `supports_responses` 与 `openai_preference`。

### Responses API 自适应学习机制

#### 自动格式探测流程

1. **首次请求**：优先尝试 `/v1/responses` 格式（OpenAI 新版 API）
2. **智能降级**：遇到以下情况自动切换 `/v1/chat/completions`：
   - HTTP 404/405（端点不存在或方法不允许）
   - HTTP 400 且错误信息包含 "unsupported" 或 "not supported"
3. **学习记录**：成功格式持久化到端点配置的 `openai_preference` 字段
4. **后续优化**：下次请求直接使用学习到的格式，避免重复试错

#### 手动控制选项

```yaml
endpoints:
  - name: custom-endpoint
    url_openai: https://api.provider.com
    # 显式控制 Responses API 支持
    supports_responses: true/false  # true=支持, false=不支持
    
    # 或使用 openai_preference 精细控制
    openai_preference: auto          # auto=自动探测
                                     # responses=强制使用 /responses
                                     # chat_completions=强制使用 /chat/completions
```

### 流式响应转换详解

#### 场景 1：真实流式 SSE 转换

当上游端点返回 `text/event-stream` 格式时：

```go
// 检测到流式响应
if strings.Contains(contentType, "text/event-stream") {
    if isCodexRequest {
        // Chat Completions SSE → Responses API SSE
        conversion.StreamChatCompletionsToResponses(upstreamReader, w, logger)
    } else {
        // Responses SSE → Chat Completions SSE  
        conversion.StreamAnthropicSSEToOpenAI(upstreamReader, w, logger)
    }
    return
}
```

**特点**：
- 边读边写，无需缓存整个响应
- 保持低延迟，适合长文本生成
- 自动转换事件格式以匹配客户端期望

#### 场景 2：模拟流式 SSE 转换

当上游端点返回 JSON 但客户端期望流式（`stream: true`）时：

```go
// 三阶段转换流程
if isCodexRequest && clientExpectsStreaming {
    // 阶段 1：格式探测
    var format string
    if _, hasChoices := respData["choices"]; hasChoices {
        format = "chat_completions"
    } else if _, hasOutput := respData["output"]; hasOutput {
        format = "responses"
    }
    
    // 阶段 2：格式转换（如需要）
    if format == "chat_completions" {
        convertedData, err := conversion.AnthropicToOpenAIResponse(
            respData, originalRequestBody, endpoint, logger,
        )
        if err == nil {
            respData = convertedData
        }
    }
    
    // 阶段 3：SSE 生成与立即返回
    ssePayload, err := convertResponseJSONToSSE(respData, responseKeys, logger)
    if err == nil {
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(ssePayload))
        if flusher, ok := w.(http.Flusher); ok {
            flusher.Flush()
        }
        return  // 立即返回，避免被后续逻辑覆盖
    }
}
```

**关键点**：
- 写入 SSE 数据后立即 `return`，防止响应被覆盖
- 确保发送 `response.created`、`response.output_text.delta`、`response.completed` 三个核心事件
- 优先提取 Responses 格式内容（`output[0].content[0].text`），兼容 Chat Completions 格式（`choices[0].message.content`）

### 关键配置

- `supports_responses` 与 `openai_preference` 决定是否优先尝试 `/responses`。
- `model_rewrite` 在转换前（请求）与回写后（响应）统一模型名称。
- `native_tool_support` 与工具增强逻辑协同，确保转换后依旧具备 Tool Calling 能力。
- `url_anthropic` 与 `url_openai` 双 URL 配置实现智能路由与格式转换。

### 相关代码

- `internal/conversion/*`：请求/响应转换、SSE 管道。
- `internal/proxy/proxy_logic.go`：降级状态机、学习与日志策略、流式转换三阶段逻辑。
- `internal/config/types.go`、`internal/proxy/server.go`：配置字段定义与持久化。
- `convertResponseJSONToSSE`：模拟流式 SSE 生成函数（`proxy_logic.go`）。

## English

To make Claude Code, Codex, and other clients interoperate with OpenAI-style endpoints, CCCC converts requests/responses in both directions while coordinating downgrade and learning logic.

### Core Flow

1. **Format detection**: `utils.DetectRequestFormat` determines whether the payload is OpenAI Responses, Chat Completions, or Anthropic Messages.
2. **Intelligent endpoint routing**:
   - **Dual URL configuration**: Single endpoint can configure both `url_anthropic` and `url_openai`
     - Claude Code requests → `url_anthropic` (Anthropic Messages API)
     - Codex requests → `url_openai` (OpenAI Responses/Chat Completions API)
   - **Single URL fallback**: When only one URL is configured, format conversion happens at proxy side
     - Only `url_anthropic`: Codex requests converted to Anthropic format before forwarding
     - Only `url_openai`: Claude Code requests converted to OpenAI format before forwarding
3. **Request conversion**: `internal/conversion/request_converter.go` uses `RequestConverter` to translate payloads and optionally rewrite model parameters.
4. **Streaming processing**:
   - **Real streaming**: When upstream returns `text/event-stream`, use `StreamChatCompletionsToResponses` or `StreamAnthropicSSEToOpenAI` to convert chunks on the fly
   - **Simulated streaming**: When upstream returns JSON but client expects streaming (`stream: true`), execute three-phase conversion:
     1. Format detection: Identify Chat Completions JSON (`choices`) or Responses JSON (`output`)
     2. Format conversion: Chat Completions JSON → Responses JSON
     3. SSE generation: Responses JSON → SSE stream and return immediately
5. **Response rewrite**: Non-streaming responses are converted in `response_converter.go`, and logs capture `conversion_path` plus `supports_responses_flag`.
6. **Downgrade learning**: The state machine in `proxy_logic` downgrades to `/chat/completions` on 404/405 or 400 with "unsupported", updating `supports_responses` and `openai_preference`.

### Responses API Adaptive Learning Mechanism

#### Automatic Format Detection Flow

1. **First request**: Try `/v1/responses` format first (OpenAI new API)
2. **Smart downgrade**: Automatically switch to `/v1/chat/completions` when:
   - HTTP 404/405 (endpoint not found or method not allowed)
   - HTTP 400 with error message containing "unsupported" or "not supported"
3. **Learning record**: Persist successful format to endpoint's `openai_preference` field
4. **Subsequent optimization**: Directly use learned format in next request to avoid retry

#### Manual Control Options

```yaml
endpoints:
  - name: custom-endpoint
    url_openai: https://api.provider.com
    # Explicitly control Responses API support
    supports_responses: true/false  # true=supported, false=not supported
    
    # Or use openai_preference for fine-grained control
    openai_preference: auto          # auto=auto-detect
                                     # responses=force /responses
                                     # chat_completions=force /chat/completions
```

### Streaming Response Conversion Details

#### Scenario 1: Real Streaming SSE Conversion

When upstream endpoint returns `text/event-stream` format:

```go
// Detect streaming response
if strings.Contains(contentType, "text/event-stream") {
    if isCodexRequest {
        // Chat Completions SSE → Responses API SSE
        conversion.StreamChatCompletionsToResponses(upstreamReader, w, logger)
    } else {
        // Responses SSE → Chat Completions SSE  
        conversion.StreamAnthropicSSEToOpenAI(upstreamReader, w, logger)
    }
    return
}
```

**Features**:
- Stream-as-you-go, no need to buffer entire response
- Maintain low latency, suitable for long text generation
- Automatically convert event format to match client expectations

#### Scenario 2: Simulated Streaming SSE Conversion

When upstream endpoint returns JSON but client expects streaming (`stream: true`):

```go
// Three-phase conversion flow
if isCodexRequest && clientExpectsStreaming {
    // Phase 1: Format detection
    var format string
    if _, hasChoices := respData["choices"]; hasChoices {
        format = "chat_completions"
    } else if _, hasOutput := respData["output"]; hasOutput {
        format = "responses"
    }
    
    // Phase 2: Format conversion (if needed)
    if format == "chat_completions" {
        convertedData, err := conversion.AnthropicToOpenAIResponse(
            respData, originalRequestBody, endpoint, logger,
        )
        if err == nil {
            respData = convertedData
        }
    }
    
    // Phase 3: SSE generation and immediate return
    ssePayload, err := convertResponseJSONToSSE(respData, responseKeys, logger)
    if err == nil {
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(ssePayload))
        if flusher, ok := w.(http.Flusher); ok {
            flusher.Flush()
        }
        return  // Return immediately to avoid override
    }
}
```

**Key Points**:
- Immediately `return` after writing SSE data to prevent response override
- Ensure sending `response.created`, `response.output_text.delta`, `response.completed` three core events
- Prioritize extracting Responses format content (`output[0].content[0].text`), compatible with Chat Completions format (`choices[0].message.content`)

### Key Configuration

- `supports_responses` and `openai_preference` control whether `/responses` is attempted first.
- `model_rewrite` normalises model names for both requests and responses.
- `native_tool_support` integrates with tool enhancement so that tool calls remain functional after conversion.
- `url_anthropic` and `url_openai` dual URL configuration enables intelligent routing and format conversion.

### Related Code

- `internal/conversion/*`: request/response converters and SSE pipelines.
- `internal/proxy/proxy_logic.go`: downgrade state machine, learning, logging, and three-phase streaming conversion logic.
- `internal/config/types.go`, `internal/proxy/server.go`: configuration fields and persistence logic.
- `convertResponseJSONToSSE`: simulated streaming SSE generation function (`proxy_logic.go`).