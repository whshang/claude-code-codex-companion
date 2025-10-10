# SSE 重构设计 | SSE Refactor Design

## 中文

SSE 重构目标是统一 OpenAI ↔ Anthropic 流式转换逻辑，降低跨碎片状态管理复杂度，并实现真正的边读边写。

### 核心改动

- 聚合头部信息后再写出，避免重复解析。  
- `internal/conversion/streaming.go` 提供 `StreamChatCompletionsToResponses` 与 `StreamAnthropicSSEToOpenAI` 两条管线。  
- 删除冗余状态结构，复用统一事件生成器，同时记录 `conversion_path`。  

### 后续计划

- 补充工具调用、thinking、异常 chunk 等边界测试用例。  
- 观察性能指标，必要时在 `/admin` 展示流式延迟。  

## English

The SSE refactor unifies streaming conversion between OpenAI and Anthropic, trimming cross-chunk state management and enabling true streaming.

### Key Changes

- Aggregate headers before streaming to avoid re-parsing.  
- `internal/conversion/streaming.go` exposes `StreamChatCompletionsToResponses` and `StreamAnthropicSSEToOpenAI`.  
- Simplified state by reusing a single event generator and logging `conversion_path`.  

### Next Steps

- Add unit tests covering tool calls, thinking segments, and malformed chunks.  
- Monitor streaming latency and surface the results in `/admin` when needed.  
