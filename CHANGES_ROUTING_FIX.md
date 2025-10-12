# 路由修复变更总结 (Commit: 9666892)

## 🎯 核心问题

**问题**: Codex 请求（`/responses`, OpenAI格式）被错误路由到仅配置了 `url_anthropic` 的端点，导致404或格式错误。

**根本原因**: 
- `GetFullURLWithFormat` 在无对应格式URL时会跨家族回退（OpenAI请求回退到Anthropic URL）
- `actualEndpointFormat` 判断逻辑在缺少 `url_openai` 时会误设为 `anthropic`
- 缺少早期端点过滤，所有端点都会尝试处理请求

## ✅ 已实施的修复

### 1. 端点层面 (internal/endpoint/endpoint.go)

**A. GetFullURLWithFormat 严格匹配**
```go
// 之前: 跨家族回退
if requestFormat == "openai" && ep.URLOpenAI != "" {
    return ep.URLOpenAI
} else if ep.URLAnthropic != "" {  // ❌ 回退到 Anthropic
    return ep.URLAnthropic
}

// 现在: 严格匹配
if requestFormat == "openai" {
    if ep.URLOpenAI != "" {
        return ep.URLOpenAI
    }
    return ""  // ✅ 返回空字符串
}
```

**B. 新增 HasURLForFormat 辅助方法**
```go
func (e *Endpoint) HasURLForFormat(requestFormat string) bool {
    switch requestFormat {
    case "anthropic":
        return e.URLAnthropic != ""
    case "openai":
        return e.URLOpenAI != ""
    default:
        return e.URLAnthropic != "" || e.URLOpenAI != ""
    }
}
```

### 2. 代理层面 (internal/proxy/proxy_logic.go)

**A. 早期端点过滤 (第179-191行)**
```go
// 在处理开始前立即检查
if clientRequestFormat != "" && !ep.HasURLForFormat(clientRequestFormat) {
    s.logger.Debug("Skipping endpoint: no URL for request format", ...)
    return false, true  // 跳过该端点
}
```

**B. targetURL 空值检查 (第198-210行)**
```go
targetURL := ep.GetFullURLWithFormat(effectivePath, endpointRequestFormat)
if targetURL == "" {
    s.logger.Debug("Skipping endpoint: GetFullURLWithFormat returned empty URL", ...)
    return false, true  // 跳过该端点
}
```

**C. actualEndpointFormat 严格判定 (第364-401行)**
```go
// 之前: 宽松回退
if clientRequestFormat == "openai" && ep.URLOpenAI != "" {
    actualEndpointFormat = "openai"
} else if ep.URLAnthropic != "" {  // ❌ 回退逻辑
    actualEndpointFormat = "anthropic"
}

// 现在: 严格匹配
if requestIsOpenAI {
    if ep.URLOpenAI != "" {
        actualEndpointFormat = "openai"
    } else {
        // 端点不能服务 OpenAI 请求
        // 已在早期检查中被跳过
    }
}
```

## 📊 影响范围

### 路由行为变化对照表

| 场景 | 端点配置 | 之前行为 | 现在行为 | 状态 |
|------|---------|---------|---------|------|
| Codex `/responses` | 仅 `url_anthropic` | ❌ 发往 Anthropic (404) | ✅ 跳过端点 | **已修复** |
| Codex `/responses` | 仅 `url_openai` | ✅ 正常 | ✅ 正常 | 保持 |
| Codex `/responses` | 双 URL | ✅ 使用 `url_openai` | ✅ 使用 `url_openai` | 保持 |
| Claude `/v1/messages` | 仅 `url_openai` | ⚠️ 可能误判 | ✅ 跳过端点 | **已改进** |
| Claude `/v1/messages` | 仅 `url_anthropic` | ✅ 正常 | ✅ 正常 | 保持 |
| Claude `/v1/messages` | 双 URL | ✅ 使用 `url_anthropic` | ✅ 使用 `url_anthropic` | 保持 |
| 未知格式 | 任意配置 | ✅ 优先 Anthropic | ✅ 优先 Anthropic | 保持 |

### 受益场景

1. **混合端点环境**: 部分端点仅支持 OpenAI，部分仅支持 Anthropic
2. **Codex 集成**: Codex 请求不再误发到 Anthropic 端点
3. **端点隔离**: 不同客户端类型自动分流到对应端点
4. **调试优化**: 日志明确显示端点跳过原因

## 🔄 向后兼容性

### ✅ 保持兼容的场景

1. **双 URL 端点**: 行为完全不变
2. **单一客户端类型**: 现有配置正常工作
3. **未指定格式**: 保持传统优先级策略（Anthropic 优先）
4. **跨家族转换**: 仍然支持（当需要时）

### ⚠️ 可能需要调整的场景

**场景**: 端点仅配置 `url_anthropic`，但期望服务 OpenAI/Codex 请求
- **之前**: 会尝试使用 Anthropic URL（通常失败）
- **现在**: 立即跳过该端点
- **建议**: 为端点添加 `url_openai` 配置

**检测方法**:
```bash
# 查看日志中是否有跳过端点的记录
grep "Skipping endpoint: no URL for request format" logs/*.log
```

## 🛠️ 回滚指南

### 快速回滚

```bash
# 回滚到修复前的版本
git revert 9666892

# 重新构建
go build -o cccc .

# 重启服务
./cccc -config config.yaml
```

### 部分回滚 (仅保留转换框架，回退路由逻辑)

如果新路由逻辑有问题但转换框架正常，可以仅回退 endpoint.go 和 proxy_logic.go:

```bash
# 回退特定文件到上一版本
git checkout HEAD~1 -- internal/endpoint/endpoint.go
git checkout HEAD~1 -- internal/proxy/proxy_logic.go

# 提交部分回退
git commit -m "partial revert: restore previous routing logic"

# 重新构建和重启
go build -o cccc . && ./cccc -config config.yaml
```

## 🧪 验证步骤

### 1. 配置验证

```bash
# 检查端点配置是否正确
cat config.yaml | grep -A 5 "url_openai\|url_anthropic"
```

### 2. 运行时验证

```bash
# 启动服务（调试模式）
./cccc -config config.yaml -port 8080

# 测试 Codex 请求
curl -X POST http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}'

# 测试 Claude 请求  
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}]}'
```

### 3. 日志验证

```bash
# 检查是否正确跳过端点
grep "Skipping endpoint" logs/*.log

# 检查实际路由目标
grep "actual_endpoint_format" logs/*.log

# 检查是否有误路由
grep "OpenAI request but no OpenAI URL" logs/*.log
```

## 📈 后续优化建议

### 短期 (1-2周)

1. **配置校验警告**: 启动时检测并警告不匹配的端点配置
2. **监控指标**: 添加端点跳过次数的 metrics
3. **文档更新**: 更新 README 和 AGENTS.md 说明新路由逻辑

### 中期 (1个月)

1. **单元测试**: 补充 endpoint 和 proxy 层的测试用例
2. **集成测试**: 端到端验证不同配置场景
3. **性能优化**: 缓存 HasURLForFormat 结果

### 长期 (考虑中)

1. **配置迁移工具**: 自动检测并建议端点配置优化
2. **动态路由**: 支持运行时修改端点URL
3. **健康检查增强**: 检测端点格式支持情况

## 📞 问题排查

### 问题 1: Codex 请求返回 502/504

**可能原因**: 所有端点都被跳过，没有可用端点
**排查方法**:
```bash
grep "Skipping endpoint" logs/*.log | wc -l  # 统计跳过次数
grep "no endpoints available" logs/*.log     # 查找无端点错误
```

**解决方案**: 为至少一个端点添加 `url_openai` 配置

### 问题 2: Claude 请求失败

**可能原因**: 仅配置了 `url_openai` 的端点被跳过
**排查方法**:
```bash
curl -v http://localhost:8080/v1/messages ... 2>&1 | grep -i "skip\|error"
```

**解决方案**: 为端点添加 `url_anthropic` 配置或使用支持 Anthropic 的端点

### 问题 3: 日志中出现 "returned empty URL"

**原因**: `GetFullURLWithFormat` 因格式不匹配返回空字符串（正常行为）
**处理**: 这是预期行为，端点会被正确跳过，尝试下一个端点

## 🔗 相关文档

- [AGENTS.md](./AGENTS.md) - Agent 集成指南与端点配置说明
- [README.md](./README.md) - 项目总体配置说明
- [TODO_1_统一转换与自动学习实现方案.md](./todo/TODO_1_统一转换与自动学习实现方案.md) - 转换框架设计文档

## 📝 变更清单

### 新增文件
- `internal/conversion/adapter_factory.go` - 适配器工厂
- `internal/conversion/openai_chat_format_adapter.go` - Chat 适配器
- `internal/conversion/openai_responses_format_adapter.go` - Responses 适配器
- `internal/conversion/conversion_manager.go` - 转换管理器
- `internal/conversion/responses_conversion.go` - 转换函数
- `internal/conversion/openai_responses_types.go` - Responses 类型定义
- `CHANGES_ROUTING_FIX.md` - 本文档

### 修改文件
- `internal/endpoint/endpoint.go` - 严格URL匹配逻辑
- `internal/proxy/proxy_logic.go` - 早期过滤与格式判定
- `internal/config/types.go` - 转换配置类型
- `internal/config/validation.go` - 转换配置验证

### 删除文件
- `internal/conversion/message_aggregator.go` - 已迁移到新框架

---

**变更日期**: 2025-01-12  
**提交哈希**: 9666892  
**维护人**: Claude Code  
**审查状态**: ✅ 已通过编译和基础验证
