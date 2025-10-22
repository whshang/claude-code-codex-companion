# 模型重写设计 | Model Rewrite Design

## 中文

模型重写允许针对每个端点配置通配符，将客户端请求的模型名映射到供应商实际模型，并支持在响应阶段回写统一名称。

### 配置规则

```yaml
model_rewrite:
  enabled: true
  rules:
    - source_pattern: "gpt-5*"
      target_model: "qwen-max"
    - source_pattern: "claude-*sonnet*"
      target_model: "glm-4.6"
```

- `source_pattern` 支持 `*` 与 `?` 通配符；规则按顺序匹配，命中后立即停止。  
- 若启用隐式映射，未匹配的模型会回落到默认映射策略。  

### 实现

- `internal/modelrewrite/rewriter.go`：处理请求与响应中的模型字段。  
- `internal/proxy/proxy_logic.go`：在请求预处理阶段应用规则，并在需要时回写响应模型。  
- 日志记录 `original_model` 与 `rewritten_model`，方便排查。  

### 建议

在 `/admin` → “测试端点” 中验证目标模型是否被上游接受，避免因名称不匹配而失败。

## English

Model rewriting lets each endpoint map client model names to provider models via wildcards, and optionally rewrite model names in responses for consistency.

### Configuration

```yaml
model_rewrite:
  enabled: true
  rules:
    - source_pattern: "gpt-5*"
      target_model: "qwen-max"
    - source_pattern: "claude-*sonnet*"
      target_model: "glm-4.6"
```

- `source_pattern` accepts `*` and `?` wildcards; rules are matched in order and stop at the first match.  
- When implicit mapping is enabled, unmatched models fall back to default rules.  

### Implementation

- `internal/modelrewrite/rewriter.go`: rewrites model fields in both requests and responses.  
- `internal/proxy/proxy_logic.go`: applies rules during preprocessing and rewrites response models when necessary.  
- Logs track `original_model` and `rewritten_model` for debugging.  

### Tips

Use the `/admin` endpoint tester to confirm the upstream accepts the target model names before deploying the mapping.  
