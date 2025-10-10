# 认证方式自动学习 | Auth Method Auto-Learning

## 中文

当端点设置 `auth_type: auto` 时，代理会在探针测试与运行时尝试 `x-api-key` 与 `Authorization` 头部，记录成功方式并持久化到 `config.yaml`，后续请求直接复用。

### 关键逻辑

1. **探针测试**（`internal/web/endpoint_testing.go`）：根据请求格式决定首选认证方式，失败则切换并记录 `DetectedAuthHeader`。  
2. **运行时代理**（`internal/proxy/proxy_logic.go`）：收到 401/403 时切换头部重试，并同步更新端点状态。  
3. **配置持久化**（`internal/proxy/server.go::PersistEndpointLearning`）：将学习结果写回配置文件。  

### 配置建议

- 当认证方式不确定时使用 `auto`；若供应商只接受单一头部，可显式配置 `auth_type`。  
- 建议在 `/admin` → “测试端点” 中确认探测结果，再保存配置以减少重试。  

### 常见问题

若探测失败，请检查 `auth_value` 是否正确，或上游是否需要额外的 Header（例如组织 ID）。

## English

With `auth_type: auto`, CCCC probes both `x-api-key` and `Authorization` headers during tests and live traffic, records the successful method, and persists it to `config.yaml` for future calls.

### Key Flow

1. **Probe testing** (`internal/web/endpoint_testing.go`): Picks a preferred header by format, switching on failure and storing `DetectedAuthHeader`.  
2. **Runtime proxy** (`internal/proxy/proxy_logic.go`): Switches headers on 401/403 and updates endpoint state.  
3. **Persistence** (`internal/proxy/server.go::PersistEndpointLearning`): Writes learned auth method back to the configuration file.  

### Configuration Tips

- Use `auto` when the expected header is unknown; specify `auth_type` if the provider affords only one option.  
- Confirm probe results in `/admin` → endpoint testing, then save the configuration to avoid repeated retries.  

### FAQ

If detection keeps failing, verify the API key and check whether the upstream requires extra headers such as organization IDs.  
# 认证方式自动学习

## 概要

当端点使用 `auth_type: auto` 时，CCCC 会在探针测试与运行时尝试 `x-api-key` 与 `Authorization` 头部，记录成功的方式并持久化到 `config.yaml`，后续请求直接复用。

## 关键逻辑

1. **探针测试** `internal/web/endpoint_testing.go`：按请求格式决定首选认证，失败后切换并记录 `DetectedAuthHeader`。
2. **运行时代理** `internal/proxy/proxy_logic.go`：收到 401/403 时切换头部重试，同步更新端点状态。
3. **持久化** `internal/proxy/server.go::PersistEndpointLearning`：将学习结果写回配置。

## 配置建议

- 默认值为空或 `auto` 时启用该流程；若供应商只接受单一头部，可显式设置 `auth_type`。
- 建议在 `/admin`→端点测试中确认探测结果，再保存配置以避免频繁重试。

## 常见问题

若探测失败，请确认 `auth_value` 是否正确，或上游是否需要额外头部。

---

# Auth Method Auto-Learning

## Summary

With `auth_type: auto`, CCCC probes both `x-api-key` and `Authorization` headers during tests and live traffic, stores the successful method, and persists it to `config.yaml` for future calls.

## Key Flow

1. **Probe testing** `internal/web/endpoint_testing.go`: Determines preferred authentication based on request format, switches and records `DetectedAuthHeader` on failure.
2. **Runtime proxy** `internal/proxy/proxy_logic.go`: Switches headers on 401/403 errors, synchronously updates endpoint status.
3. **Persistence** `internal/proxy/server.go::PersistEndpointLearning`: Writes learning results back to configuration.

## Configuration Tips

- Enables this flow when value is empty or `auto`; if provider only accepts single header, explicitly set `auth_type`.
- Suggest confirming probe results in `/admin`→endpoint testing, then save config to avoid frequent retries.

## FAQ

If detection keeps failing, verify the API key or check whether the upstream requires additional headers.
