# 被拉黑端点日志增强 | Blacklisted Endpoint Logging Enhancements

## 中文

当端点被判定为失效时，系统仍会记录请求日志并附带导致失效的请求 ID，便于排查；端点恢复后清除相关上下文。

### 设计要点

- **状态追踪**：`endpoint.Endpoint` 维护 `BlacklistReason`，包含触发请求 ID、时间与摘要。  
- **日志写入**：`logBlacklistedEndpointRequest` 统一写入请求体摘要、`BlacklistCausingRequestIDs`，并标记状态码 `503`。  
- **恢复流程**：健康检查通过时清除 `BlacklistReason`，新日志不再携带历史信息。  

### 使用建议

结合 `/admin/logs` 或 `sqlite3 logs/logs.db` 查询，可快速定位导致端点被禁用的请求，辅助判定是否需要手动恢复或调整配置。

## English

Even when an endpoint is marked inactive, CCCC continues logging requests with the IDs that triggered the failure, and clears the context once the endpoint recovers.

### Key Points

- **State tracking**: `endpoint.Endpoint` stores a `BlacklistReason` with request IDs, timestamp, and summary.  
- **Logging**: `logBlacklistedEndpointRequest` records request snapshots, `BlacklistCausingRequestIDs`, and sets status code `503`.  
- **Recovery**: When health checks succeed, the stored reason is cleared so new logs have no legacy data.  

### Tips

Use `/admin/logs` or `sqlite3 logs/logs.db` to inspect `BlacklistCausingRequestIDs` and understand why an endpoint was disabled, helping decide whether to re-enable it or adjust configuration.  
