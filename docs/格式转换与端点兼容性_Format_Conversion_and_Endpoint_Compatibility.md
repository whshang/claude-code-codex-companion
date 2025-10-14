# 格式转换与端点兼容性 | Format Conversion and Endpoint Compatibility

## 中文

### 概述

CCCC 支持 Claude Code 和 Codex 客户端的双向格式转换，实现"单URL多客户端"的灵活配置。

### 核心能力

#### 双URL智能路由
- **Claude Code 请求** → `url_anthropic`（Anthropic Messages API）
- **Codex 请求** → `url_openai`（OpenAI Responses/Chat Completions API）
- **单URL降级**：自动在代理侧进行格式转换

#### 完整格式转换
支持 OpenAI Chat Completions ↔ Responses API ↔ Anthropic Messages 之间的完整转换：

**新增字段支持**：
- 采样控制：`presence_penalty`, `frequency_penalty`, `logit_bias`, `n`
- 输出格式：`response_format`（json_object, json_schema）
- 双路径回退：`input` → `messages` 兼容性

**转换流程**：
1. **请求转换**：客户端格式 → 统一中间格式 → 上游格式
2. **响应转换**：上游格式 → 统一中间格式 → 客户端格式
3. **流式支持**：SSE 实时转换，保持事件完整性

#### 技术实现

**Adapter 模式**：
- `OpenAIChatAdapter`：处理 Chat Completions 格式
- `OpenAIResponsesAdapter`：处理 Responses API 格式
- `InternalRequest`：统一中间层，解耦格式差异

**深拷贝安全**：
- 防止 `LogitBias`、`Stop` 等集合字段的引用共享
- 线程安全的并发处理

### 端点配置策略

#### 推荐配置模式

**模式1：双URL（最佳）**
```yaml
- name: example-dual-url
  url_anthropic: https://api.provider.com
  url_openai: https://api.provider.com/v1
  openai_preference: chat_completions  # 大部分第三方支持 chat_completions
  model_rewrite:
    enabled: true
    rules:
      - source_pattern: gpt-5*
        target_model: provider-actual-model
```

**模式2：仅OpenAI URL（可用）**
```yaml
- name: example-openai-only
  url_openai: https://api.provider.com/v1
  openai_preference: chat_completions
  model_rewrite:
    enabled: true
    rules:
      - source_pattern: gpt-5*
        target_model: provider-actual-model
```

**模式3：仅Anthropic URL（需转换）**
```yaml
- name: example-anthropic-only
  url_anthropic: https://api.provider.com
  # Codex请求自动转换为Anthropic格式
  model_rewrite:
    enabled: true
    rules:
      - source_pattern: claude-*sonnet*
        target_model: provider-actual-model
      - source_pattern: gpt-5*  # 支持Codex
        target_model: provider-actual-model
```

### 兼容性状态

| 端点类型 | Claude Code | Codex | 说明 |
|---------|-----------|-------|------|
| 双URL | ✅ 直接路由 | ✅ 直接路由 | 最佳性能 |
| 仅OpenAI | ✅ 转换 | ✅ 直接 | 需要格式转换 |
| 仅Anthropic | ✅ 直接 | ✅ 转换 | 需要格式转换 |

### 故障排除

#### 常见问题
1. **"no available endpoints"错误**：检查端点是否配置了对应URL或格式转换支持
2. **400 Bad Request**：上游可能不兼容转换后的格式，尝试其他端点
3. **工具调用失败**：确认上游端点是否支持工具调用

#### 调试步骤
1. 查看日志：`grep "format conversion" logs/proxy.log`
2. 测试端点：`go run ./cmd/test_endpoints -config config.yaml`
3. 启用详细日志：
   ```yaml
   logging:
     level: debug
     log_request_body: full
     log_response_body: full
   ```

### 测试验证

**成功案例**：
- ThatAPI：3/3 测试通过，工具增强正常
- kkyyxx.xyz：3/3 测试通过，响应速度快

**已修复问题**：
- 移除硬编码的端点选择限制
- 实现智能URL回退机制
- 完整的字段映射支持

## English

### Overview

CCCC supports bidirectional format conversion between Claude Code and Codex clients, enabling "single URL, multiple clients" flexible configuration.

### Core Capabilities

#### Dual-URL Smart Routing
- **Claude Code requests** → `url_anthropic` (Anthropic Messages API)
- **Codex requests** → `url_openai` (OpenAI Responses/Chat Completions API)
- **Single URL fallback** → automatic format conversion on proxy side

#### Complete Format Conversion
Supports full conversion between OpenAI Chat Completions ↔ Responses API ↔ Anthropic Messages:

**New field support**:
- Sampling control: `presence_penalty`, `frequency_penalty`, `logit_bias`, `n`
- Output format: `response_format` (json_object, json_schema)
- Dual-path fallback: `input` → `messages` compatibility

**Conversion flow**:
1. **Request conversion**: Client format → unified intermediate → upstream format
2. **Response conversion**: Upstream format → unified intermediate → client format
3. **Streaming support**: Real-time SSE conversion with event integrity

### Technical Implementation

**Adapter Pattern**:
- `OpenAIChatAdapter`: Handles Chat Completions format
- `OpenAIResponsesAdapter`: Handles Responses API format
- `InternalRequest`: Unified intermediate layer, decouples format differences

**Deep copy safety**:
- Prevents reference sharing in collection fields like `LogitBias`, `Stop`
- Thread-safe concurrent processing

### Endpoint Configuration Strategies

#### Recommended Patterns

**Pattern 1: Dual URL (Best)**
```yaml
- name: example-dual-url
  url_anthropic: https://api.provider.com
  url_openai: https://api.provider.com/v1
  openai_preference: chat_completions  # Most third-party support chat_completions
  model_rewrite:
    enabled: true
    rules:
      - source_pattern: gpt-5*
        target_model: provider-actual-model
```

**Pattern 2: OpenAI Only (Workable)**
```yaml
- name: example-openai-only
  url_openai: https://api.provider.com/v1
  openai_preference: chat_completions
  model_rewrite:
    enabled: true
    rules:
      - source_pattern: gpt-5*
        target_model: provider-actual-model
```

**Pattern 3: Anthropic Only (Requires conversion)**
```yaml
- name: example-anthropic-only
  url_anthropic: https://api.provider.com
  # Codex requests automatically converted to Anthropic format
  model_rewrite:
    enabled: true
    rules:
      - source_pattern: claude-*sonnet*
        target_model: provider-actual-model
      - source_pattern: gpt-5*  # Support for Codex
        target_model: provider-actual-model
```

### Compatibility Status

| Endpoint Type | Claude Code | Codex | Notes |
|-------------|-----------|-------|-------|
| Dual URL | ✅ Direct routing | ✅ Direct routing | Best performance |
| OpenAI Only | ✅ Conversion | ✅ Direct | Requires format conversion |
| Anthropic Only | ✅ Direct | ✅ Conversion | Requires format conversion |

### Troubleshooting

#### Common Issues
1. **"no available endpoints" error**: Check endpoint URL configuration or format conversion support
2. **400 Bad Request**: Upstream may not support converted format, try other endpoints
3. **Tool call failures**: Verify upstream tool-call support

#### Debug Steps
1. Check logs: `grep "format conversion" logs/proxy.log`
2. Test endpoints: `go run ./cmd/test_endpoints -config config.yaml`
3. Enable detailed logging:
   ```yaml
   logging:
     level: debug
     log_request_body: full
     log_response_body: full
   ```

### Test Results

**Successful cases**:
- ThatAPI: 3/3 tests passed
- kkyyxx.xyz: 3/3 tests passed, fast response

**Fixed issues**:
- Removed hardcoded endpoint selection restrictions
- Implemented smart URL fallback mechanism
- Complete field mapping support

### Related Files
- Core conversion: `internal/conversion/` directory
- Endpoint selection: `internal/endpoint/selector.go`
- Proxy logic: `internal/proxy/proxy_logic.go`
- Configuration: `docs/配置指南_Configuration_Guide.md`