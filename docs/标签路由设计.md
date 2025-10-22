# 标签路由设计 | Tag-based Routing Design

## 中文

标签路由通过 Tagger 为请求打标签，再根据端点配置筛选可用上游，实现对多客户端、多模型的精细调度。

### 工作流程

1. Tagger（内置或 Starlark）并发执行，为请求附加零个或多个标签。  
2. 如果请求带标签，仅匹配同时包含所有标签的端点；否则落到无标签端点。  
3. 在候选端点中按照优先级、健康状态进行选择与重试。  

### 配置要点

- 端点标签：`endpoints[].tags`，可在 `/admin/endpoints` 热更新。  
- Tagger 配置位于 `internal/tagging`，支持在 Web UI 中启用/禁用。  
- 无标签端点视为通用兜底，可通过优先级控制调度顺序。  

### 实践建议

为 Claude Code、Codex 分别设置 `claude`、`codex` 标签，同时保留通用端点，例如 `fallback`，以便在特殊场景启用。

## English

Tag-based routing attaches tags to incoming requests and selects endpoints whose capabilities match, enabling fine-grained control across clients and models.

### Flow

1. Taggers (built-in or Starlark) run concurrently and add zero or more tags to each request.  
2. Requests with tags match only endpoints containing all those tags; otherwise they fall back to tagless endpoints.  
3. Candidate endpoints are sorted by priority and health for selection and retry.  

### Configuration Notes

- Endpoint tags: `endpoints[].tags`, hot-editable via `/admin/endpoints`.  
- Tagger definitions reside in `internal/tagging` and can be toggled in the Web UI.  
- Tagless endpoints act as generic fallbacks whose order is controlled by priority.  

### Tips

Use tags such as `claude`, `codex`, `internal`, or `external` to separate traffic while keeping at least one generic fallback endpoint.  
