# 学习持久化实现 | Learning Persistence Implementation

## 中文

代理会在运行时或端点测试时学习 `auth_type`、`openai_preference`、`supports_responses`、`native_tool_support` 等信息，并通过 `PersistEndpointLearning` 写回 `config.yaml`，确保重启后仍然生效。

### 流程

1. **学习**：`proxy_logic` 与 `endpoint_testing` 更新端点对象上的字段。  
2. **持久化**：调用 `server.PersistEndpointLearning(ep)`，触发统一的 `updateEndpointConfig`。  
3. **写盘**：配置文件加锁写回，只在字段变化时真正写入，避免并发冲突。  

### 代码位置

- `internal/proxy/server.go`：`PersistEndpointLearning`、`updateEndpointConfig`。  
- `internal/proxy/proxy_logic.go`：学习完成后触发持久化。  
- `internal/web/endpoint_testing.go`：探针测试结束后更新学习结果。  

### 建议

批量测试完成后，建议在 `/admin` 中点击“保存配置”，将最新学习结果写入 `config.yaml`。

## English

During runtime or endpoint tests, CCCC learns fields such as `auth_type`, `openai_preference`, `supports_responses`, and `native_tool_support`, then persists them via `PersistEndpointLearning` so the settings survive restarts.

### Flow

1. **Learning**: `proxy_logic` and `endpoint_testing` update properties on the endpoint object.  
2. **Persist**: `server.PersistEndpointLearning(ep)` triggers the unified `updateEndpointConfig`.  
3. **Flush**: The configuration file is written with locking and only when values change.  

### Key Files

- `internal/proxy/server.go`: `PersistEndpointLearning`, `updateEndpointConfig`.  
- `internal/proxy/proxy_logic.go`: Invokes persistence after learning completes.  
- `internal/web/endpoint_testing.go`: Applies test results to runtime learning.  

### Tips

After running probes, use “Save Config” in `/admin` to confirm the learned values are written back to `config.yaml`.
