# SSE 重构设计 | SSE Refactor Design

## 中文

SSE 重构目标是统一 OpenAI ↔ Anthropic 流式转换逻辑，降低跨碎片状态管理复杂度，并实现真正的边读边写。

### 核心改动

- 聚合头部信息后再写出，避免重复解析。  
- `internal/conversion/streaming.go` 提供 `StreamChatCompletionsToResponses` 与 `StreamAnthropicSSEToOpenAI` 两条管线。  
- 删除冗余状态结构，复用统一事件生成器，同时记录 `conversion_path`。  

### 流式响应转换详解（Codex 专用增强）

#### 1. 真实流式 SSE 转换

当上游端点返回 `text/event-stream` 格式时：

**Chat Completions SSE → Responses API SSE**
- **输入格式**：`data: {"choices":[{"delta":{"content":"text"}}]}`
- **转换管线**：`StreamChatCompletionsToResponses` 函数
- **输出格式**：
  ```
  event: response.created
  data: {"type":"response.created","response":{...}}

  event: response.output_text.delta
  data: {"type":"response.output_text.delta","delta":{"text":"text"}}

  event: response.completed
  data: {"type":"response.completed","response":{...}}
  ```
- **特点**：边读边写，无需缓存，保持低延迟

#### 2. 模拟流式 SSE 转换

当上游端点返回 JSON 但客户端期望流式（`stream: true`）时：

**三阶段转换流程**（`internal/proxy/proxy_logic.go`）：

**阶段 1：格式探测**
```go
// 智能识别响应格式
if _, hasChoices := respData["choices"]; hasChoices {
    format = "chat_completions"
} else if _, hasOutput := respData["output"]; hasOutput {
    format = "responses"
}
```

**阶段 2：格式转换**（如需要）
```go
// Chat Completions JSON → Responses JSON
if format == "chat_completions" {
    convertedData, err := conversion.AnthropicToOpenAIResponse(
        respData, originalRequestBody, endpoint, logger,
    )
    if err == nil {
        respData = convertedData
        responseKeys = []string{"output"}
    }
}
```

**阶段 3：SSE 生成与立即返回**
```go
// 生成 SSE 事件流
ssePayload, err := convertResponseJSONToSSE(respData, responseKeys, logger)
if err == nil {
    // 设置 SSE 响应头
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.WriteHeader(http.StatusOK)
    
    // 立即写入并刷新
    w.Write([]byte(ssePayload))
    if flusher, ok := w.(http.Flusher); ok {
        flusher.Flush()
    }
    
    // 立即返回，避免被后续逻辑覆盖
    return
}
```

#### 3. 事件完整性保证

`convertResponseJSONToSSE` 函数确保发送三个核心事件：

```go
func convertResponseJSONToSSE(respData map[string]interface{}, responseKeys []string, logger *zap.Logger) (string, error) {
    // 1. 提取响应内容（优先 Responses 格式）
    var content string
    if output, ok := respData["output"].([]interface{}); ok && len(output) > 0 {
        // Responses 格式：output[0].content[0].text
        if contentArray, ok := output[0].(map[string]interface{})["content"].([]interface{}); ok {
            if textObj, ok := contentArray[0].(map[string]interface{}); ok {
                content = textObj["text"].(string)
            }
        }
    } else if choices, ok := respData["choices"].([]interface{}); ok {
        // Chat Completions 格式：choices[0].message.content
        content = choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
    }
    
    // 2. 生成三个核心事件
    var sseEvents strings.Builder
    
    // Event 1: response.created
    sseEvents.WriteString("event: response.created\n")
    sseEvents.WriteString("data: {\"type\":\"response.created\"}\n\n")
    
    // Event 2: response.output_text.delta
    sseEvents.WriteString("event: response.output_text.delta\n")
    sseEvents.WriteString(fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":{\"text\":\"%s\"}}\n\n", content))
    
    // Event 3: response.completed
    sseEvents.WriteString("event: response.completed\n")
    sseEvents.WriteString("data: {\"type\":\"response.completed\"}\n\n")
    
    return sseEvents.String(), nil
}
```

#### 4. 关键技术细节

**避免响应覆盖**：
- 写入 SSE 数据后立即 `return`，防止后续代码覆盖响应
- 不再执行常规 JSON 写入路径

**内容提取优先级**：
1. Responses 格式：`output[0].content[0].text`
2. Chat Completions 格式：`choices[0].message.content`
3. 简化格式：直接 `text` 或 `content` 字段

**错误处理**：
- 无法提取内容时记录 "No content found" 并返回错误
- 转换失败时降级到常规 JSON 响应

### 后续计划

- 补充工具调用、thinking、异常 chunk 等边界测试用例。  
- 观察性能指标，必要时在 `/admin` 展示流式延迟。  
- 优化多模态内容（图像、文档）的 SSE 转换支持。

## English

The SSE refactor unifies streaming conversion between OpenAI and Anthropic, trimming cross-chunk state management and enabling true streaming.

### Key Changes

- Aggregate headers before streaming to avoid re-parsing.  
- `internal/conversion/streaming.go` exposes `StreamChatCompletionsToResponses` and `StreamAnthropicSSEToOpenAI`.  
- Simplified state by reusing a single event generator and logging `conversion_path`.  

### Streaming Response Conversion Details (Codex Enhancement)

#### 1. Real Streaming SSE Conversion

When upstream returns `text/event-stream`:

**Chat Completions SSE → Responses API SSE**
- **Input**: `data: {"choices":[{"delta":{"content":"text"}}]}`
- **Pipeline**: `StreamChatCompletionsToResponses` function
- **Output**:
  ```
  event: response.created
  data: {"type":"response.created","response":{...}}

  event: response.output_text.delta
  data: {"type":"response.output_text.delta","delta":{"text":"text"}}

  event: response.completed
  data: {"type":"response.completed","response":{...}}
  ```
- **Feature**: Stream-as-you-go, no buffering, low latency

#### 2. Simulated Streaming SSE Conversion

When upstream returns JSON but client expects streaming (`stream: true`):

**Three-Phase Conversion** (`internal/proxy/proxy_logic.go`):

**Phase 1: Format Detection**
```go
// Smart format recognition
if _, hasChoices := respData["choices"]; hasChoices {
    format = "chat_completions"
} else if _, hasOutput := respData["output"]; hasOutput {
    format = "responses"
}
```

**Phase 2: Format Conversion** (if needed)
```go
// Chat Completions JSON → Responses JSON
if format == "chat_completions" {
    convertedData, err := conversion.AnthropicToOpenAIResponse(
        respData, originalRequestBody, endpoint, logger,
    )
    if err == nil {
        respData = convertedData
        responseKeys = []string{"output"}
    }
}
```

**Phase 3: SSE Generation & Immediate Return**
```go
// Generate SSE event stream
ssePayload, err := convertResponseJSONToSSE(respData, responseKeys, logger)
if err == nil {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.WriteHeader(http.StatusOK)
    
    // Write and flush immediately
    w.Write([]byte(ssePayload))
    if flusher, ok := w.(http.Flusher); ok {
        flusher.Flush()
    }
    
    // Return immediately to avoid override
    return
}
```

#### 3. Event Integrity Guarantee

The `convertResponseJSONToSSE` function ensures three core events:

```go
func convertResponseJSONToSSE(respData map[string]interface{}, responseKeys []string, logger *zap.Logger) (string, error) {
    // 1. Extract content (Responses format priority)
    var content string
    if output, ok := respData["output"].([]interface{}); ok && len(output) > 0 {
        // Responses format: output[0].content[0].text
        if contentArray, ok := output[0].(map[string]interface{})["content"].([]interface{}); ok {
            if textObj, ok := contentArray[0].(map[string]interface{}); ok {
                content = textObj["text"].(string)
            }
        }
    } else if choices, ok := respData["choices"].([]interface{}); ok {
        // Chat Completions format: choices[0].message.content
        content = choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
    }
    
    // 2. Generate three core events
    var sseEvents strings.Builder
    
    // Event 1: response.created
    sseEvents.WriteString("event: response.created\n")
    sseEvents.WriteString("data: {\"type\":\"response.created\"}\n\n")
    
    // Event 2: response.output_text.delta
    sseEvents.WriteString("event: response.output_text.delta\n")
    sseEvents.WriteString(fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":{\"text\":\"%s\"}}\n\n", content))
    
    // Event 3: response.completed
    sseEvents.WriteString("event: response.completed\n")
    sseEvents.WriteString("data: {\"type\":\"response.completed\"}\n\n")
    
    return sseEvents.String(), nil
}
```

#### 4. Key Technical Details

**Avoiding Response Override**:
- Immediately `return` after writing SSE data to prevent subsequent code from overwriting
- Skip regular JSON writing path

**Content Extraction Priority**:
1. Responses format: `output[0].content[0].text`
2. Chat Completions format: `choices[0].message.content`
3. Simplified format: direct `text` or `content` field

**Error Handling**:
- Log "No content found" and return error if content extraction fails
- Fallback to regular JSON response on conversion failure

### Next Steps

- Add unit tests covering tool calls, thinking segments, and malformed chunks.  
- Monitor streaming latency and surface the results in `/admin` when needed.  
- Optimize SSE conversion for multimodal content (images, documents).  
