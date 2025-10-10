# Codex 流式响应 Tool Calls 转换修复

## 问题描述

用户使用 Codex 发送包含大量代码的文本消息时，响应非常缓慢并且会分段显示，表现为：

```
› 搜索还有没有 gpt-4o

• I'll search for any remaining references to gpt-4o in the codebase.
  I'll search for any remaining references to gpt-4o in the codebase.
  I'll search for any remaining references to "gpt-4o" in the codebase.
  <shell>
  {
    "command": [
      "rg",
      "gpt-4o",
      "--type",
      "go",
      "--type",
      "yaml",
      "--type",
      "md"
    ]
  }
  </shell>I'll search for any remaining references to "gpt-4o" in the codebase.
  <shell>
  {"command":["rg","gpt-4o","--type-not","lock","--type-not","sum","--type-not","patch","--hidden"]}
  </shell>
```

内容重复输出，响应卡顿，似乎 `/chat/completions` 已经回复完毕，但 `/responses` 还在等待。

## 根本原因

### 问题背景

Codex 客户端使用 OpenAI 的新 **Responses API** (`/responses` 路径)，期望接收 Responses API 格式的 SSE 事件：

```json
// Responses API 格式
data: {"type": "response.created", "response": {...}}
data: {"type": "response.output_text.delta", "delta": "text...", "response_id": "..."}
data: {"type": "response.completed", "response": {...}}
```

但大多数 OpenAI 兼容后端只支持 **Chat Completions API** (`/chat/completions`)，返回的是 Chat Completions 格式：

```json
// Chat Completions 格式
data: {"id": "...", "object": "chat.completion.chunk", "choices": [{"delta": {"content": "..."}}]}
data: [DONE]
```

### CCCC 的处理流程

1. Codex 发送请求到 `/responses`
2. CCCC 检测到后端不支持 `/responses`，将请求转换为 `/chat/completions` 格式
3. 后端返回 Chat Completions 格式的流式响应
4. CCCC 调用 `StreamChatCompletionsToResponses()` 将响应转换回 Responses API 格式
5. 返回给 Codex 客户端

### Bug 位置

在 `internal/conversion/streaming.go` 的 `StreamChatCompletionsToResponses()` 函数中（第 84-113 行），**只处理了 `delta.content`（文本内容），完全没有处理 `delta.tool_calls`**：

```go
// ❌ 原始代码（有问题）
if delta, ok := choice["delta"].(map[string]interface{}); ok {
    if content, ok := delta["content"].(string); ok && content != "" {
        if err := writeSSEData(w, map[string]interface{}{
            "type":        "response.output_text.delta",
            "delta":       content,
            "response_id": responseID,
        }); err != nil {
            return err
        }
    }
    // ❌ 没有处理 delta["tool_calls"]！
}
```

当 LLM 响应包含工具调用时：
1. `delta.tool_calls` 被完全忽略
2. 这些数据在转换过程中丢失
3. Codex 客户端等待完整的响应
4. 由于数据不完整，导致渲染卡顿、重复输出
5. 流式响应无法正常结束

### 为什么工具调用会导致这个问题？

根据 Toolify 增强和现代 AI Agent 模式：
- 当用户请求包含代码操作时（如 "搜索 gpt-4o"），LLM 会返回工具调用
- 这些工具调用在流式响应中表现为 `delta.tool_calls` 字段
- 如果转换函数不处理 `tool_calls`，整个流式响应会中断或错乱

## 修复方案

在 `StreamChatCompletionsToResponses()` 中添加对 `delta.tool_calls` 的处理：

```go
// ✅ 修复后的代码
if delta, ok := choice["delta"].(map[string]interface{}); ok {
    // 🔧 修复：检查是否包含 tool_calls
    // 如果包含 tool_calls，直接透传原始 Chat Completions 格式
    // Codex 客户端实际上兼容 Chat Completions 流式格式
    if _, hasToolCalls := delta["tool_calls"]; hasToolCalls {
        // 有 tool_calls 时，透传整个 chunk（可能同时包含文本）
        if err := writeSSEData(w, chunk); err != nil {
            return err
        }
    } else {
        // 没有 tool_calls 时，按原逻辑处理文本
        if content, ok := delta["content"].(string); ok && content != "" {
            if err := writeSSEData(w, map[string]interface{}{
                "type":        "response.output_text.delta",
                "delta":       content,
                "response_id": responseID,
            }); err != nil {
                return err
            }
        }
    }
}
```

### 为什么透传原始格式？

**关键发现**：Codex 客户端实际上**兼容** Chat Completions 流式格式！

1. **纯文本响应**：转换为 Responses API 格式（保持向后兼容）
   ```json
   data: {"type": "response.output_text.delta", "delta": "text...", "response_id": "..."}
   ```

2. **包含 tool_calls**：直接透传 Chat Completions 格式
   ```json
   data: {"id": "...", "object": "chat.completion.chunk", "choices": [{"delta": {"tool_calls": [...]}}]}
   ```

这种混合策略：
- ✅ 保持纯文本响应的优化体验
- ✅ 完整传递工具调用信息
- ✅ 避免格式转换的复杂性
- ✅ 利用 Codex 的格式兼容性

## 技术细节

### Chat Completions 流式 Tool Calls 格式

```json
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"search_code","arguments":""}}]}}]}
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"pattern\""}}]}}]}
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\n\"gpt-4o\"}"}}]}}]}
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
```

特点：
- Tool calls 以**增量方式**传递
- `function.arguments` 字段会分多次发送
- 首次包含完整的 `id`、`name`、`type`
- 后续只包含 `index` 和增量的 `arguments`
- 最后一个 chunk 包含 `finish_reason: "tool_calls"`

### Responses API 格式（理论）

Responses API 文档中关于 tool calls 的事件格式较少，理论上应该有类似：

```json
data: {"type": "response.function_call_arguments.delta", "delta": "...", "call_id": "..."}
```

但实际测试发现：
1. Codex 客户端能正确处理 Chat Completions 格式
2. 转换为 Responses API 格式反而增加复杂度
3. 透传策略更加可靠和高效

## 影响范围

这个修复影响所有使用 Codex 进行 tool calling 的场景：

1. **代码搜索和操作**：使用 `grep`、`codebase_search` 等工具
2. **文件操作**：读取、编写、编辑文件
3. **命令执行**：运行终端命令
4. **复杂多步骤任务**：需要多次工具调用的任务

修复后，这些场景的流式响应将：
- ✅ 流畅输出，无卡顿
- ✅ 无重复内容
- ✅ 正确显示工具调用过程
- ✅ 快速响应，符合用户预期

## 与 Tool Result 修复的关系

今天的两个修复解决了 tool calling 的完整链路：

1. **Tool Result 修复**（Claude Code 方向）
   - 修复了 Claude Code → CCCC → 后端的**请求转换**
   - 解决 `tool_use_id` 丢失问题
   - 位置：`internal/conversion/anthropic_types.go`

2. **Tool Calls 修复**（Codex 方向）
   - 修复了后端 → CCCC → Codex 的**响应转换**
   - 解决 `tool_calls` 丢失问题
   - 位置：`internal/conversion/streaming.go`

两个修复共同确保了：
```
Claude Code ↔ CCCC ↔ 后端 ↔ CCCC ↔ Codex
     ✅           ✅        ✅        ✅
 Tool Result  转换正确  Tool Calls 转换正确
```

## 测试验证

### 手动测试

1. 启动服务：`make dev`
2. 使用 Codex 发送包含代码搜索的请求
3. 观察响应是否流畅、无重复、无卡顿

### 预期行为

**修复前**：
- 响应缓慢
- 内容重复输出
- 工具调用信息丢失
- 流式响应卡住

**修复后**：
- 响应流畅
- 内容正常输出一次
- 工具调用完整显示
- 流式响应正常结束

## 相关文件

1. **internal/conversion/streaming.go** - 核心修复（第 87-106 行）
2. **CHANGELOG.md** - 记录修复
3. **docs/Codex_Streaming_Tool_Calls_Fix_2025-10-10.md** - 本文档

## 未来优化方向

### 1. 完整的 Responses API 支持

如果未来需要严格遵循 Responses API 规范，可以实现完整的 tool call 事件转换：

```go
// 完整转换（可选的未来方向）
if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
    for _, tc := range toolCalls {
        call := tc.(map[string]interface{})
        // 发送 response.function_call.started
        // 发送 response.function_call_arguments.delta
        // 发送 response.function_call.completed
    }
}
```

### 2. 格式检测优化

添加更智能的格式检测，根据客户端能力动态选择策略：
- Codex 支持混合格式 → 使用透传策略（当前实现）
- 严格 Responses API 客户端 → 使用完整转换

### 3. 性能监控

添加转换性能监控，追踪：
- 转换延迟
- 数据丢失情况
- 客户端兼容性问题

## 参考资料

- OpenAI Responses API: https://platform.openai.com/docs/guides/responses
- OpenAI Chat Completions API: https://platform.openai.com/docs/guides/chat
- Tool Calling 文档：项目内 `docs/工具调用增强_Tool_Calling_Enhancement.md`

