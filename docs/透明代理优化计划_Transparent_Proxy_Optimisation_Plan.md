# 透明代理优化计划 | Transparent Proxy Optimisation Plan

## 中文

该计划聚焦透明代理的可靠性，涵盖降级判定、流式转换、状态机复用、日志截断以及 `supports_responses` 持久化，核心工作已经落地。

### 已完成事项

| 项目 | 说明 |
| --- | --- |
| 降级判定 | 仅在 404/405 或包含 “unsupported” 的 400 时降级，避免短期故障触发永久降级。 |
| 流式转换 | 新增 `StreamChatCompletionsToResponses` 与 `StreamAnthropicSSEToOpenAI`，实现边读边写以降低延迟和内存。 |
| 状态机 | 在 `proxy_logic` 中使用循环重试与缓存，去除递归并复用格式探测、工具分析结果。 |
| 日志截断 | 统一使用 `buildBodySnapshot` 和 64KB 限制，仅持久化摘要与哈希。 |
| `supports_responses` | 支持在配置中声明并在学习后持久化，管理端测试也会更新该状态。 |

### 后续计划

补充首字节延迟与内存占用基准测试，并在 `/admin` 中展示结果，同时扩展自动化脚本覆盖流式场景。

### 参考代码

- `internal/proxy/proxy_logic.go`：状态机、降级判定、日志截断实现。
- `internal/conversion/streaming.go`：流式转换核心代码。
- `internal/config/types.go`、`internal/proxy/server.go`：`supports_responses` 字段定义与持久化逻辑。

## English

This plan focuses on strengthening the transparent proxy: relaxed downgrade rules, true streaming conversion, request-state reuse, log truncation, and persistence of `supports_responses`. All major items have shipped.

### Completed Items

| Item | Description |
| --- | --- |
| Downgrade criteria | Downgrade only on 404/405 or 400 with “unsupported”, avoiding permanent downgrades from transient failures. |
| Streaming pipeline | Added `StreamChatCompletionsToResponses` and `StreamAnthropicSSEToOpenAI`, streaming data to cut latency and memory. |
| State machine | Introduced retry loops and caching in `proxy_logic`, removing recursion and reusing detection & tool analysis results. |
| Log truncation | Standardised on `buildBodySnapshot` with a 64 KB cap, storing summaries and hashes only. |
| `supports_responses` | Allow declaration and persistence after learning; admin tests update the state as well. |

### Next Steps

Add benchmarks for first-byte latency and memory footprint, surface metrics in `/admin`, and extend automation scripts for streaming downgrade scenarios.

### Key References

- `internal/proxy/proxy_logic.go` – state machine, downgrade checks, log capture.
- `internal/conversion/streaming.go` – streaming converters.
- `internal/config/types.go`, `internal/proxy/server.go` – configuration & persistence of `supports_responses`.
