# AI Agent 集成指南

## 📋 概述

CCCC (Claude Code and Codex Companion) 不仅支持 Claude Code 和 Codex，还可以为各种 AI Agent 和自定义应用提供统一的 API 代理服务。通过智能路由、格式转换和负载均衡，为 AI Agent 提供稳定可靠的 LLM 接入层。

## 🎯 支持的 Agent 类型

### 1. 编程助手 Agent
- **Claude Code**：Anthropic 官方编程助手
- **Codex**：OpenAI 编程助手
- **Cursor**：基于 VS Code 的 AI 编程工具
- **Continue**：VS Code AI 扩展
- **Aider**：命令行 AI 编程助手

### 2. 通用对话 Agent
- **ChatGPT 客户端**：各种第三方 ChatGPT 应用
- **自定义 Web Agent**：基于 Web 的 AI 助手
- **企业内部 Agent**：定制化 AI 解决方案

### 3. 专业领域 Agent
- **数据分析 Agent**：处理数据查询和分析
- **文档处理 Agent**：文档理解和生成
- **多模态 Agent**：图像、文本、音频处理

## 🚀 快速集成

### 基础配置

1. **启动 CCCC 服务**
```bash
./cccc -config config.yaml -port 8080
```

2. **配置通用端点**
```yaml
endpoints:
  - name: universal-llm
    url_anthropic: https://api.provider.com/anthropic
    url_openai: https://api.provider.com/openai
    endpoint_type: openai
    auth_type: auth_token
    auth_value: your-token
    enabled: true
    priority: 1
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: gpt-*
          target_model: provider-gpt-equivalent
        - source_pattern: claude-*
          target_model: provider-claude-equivalent
# 只配置其中一个 URL 时，CCCC 会自动转换请求格式以兼容所有客户端。
# 例如：缺少 url_openai 也能服务 Codex，因为请求会自动转换为 Anthropic 格式。
```

### Agent 连接配置

#### OpenAI 兼容 Agent
```bash
# 设置环境变量
export OPENAI_API_BASE="http://127.0.0.1:8080/v1"
export OPENAI_API_KEY="hello"
```

#### Anthropic 兼容 Agent
```bash
# 设置环境变量
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_API_KEY="hello"
```

## 🔧 高级配置

### 1. 标签路由

为不同类型的 Agent 配置专用端点：

```yaml
# 配置标签路由规则
tagging:
    enabled: true
    taggers:
        - name: agent-router
          type: builtin
          config:
              rules:
                  - pattern: "^/v1/chat/completions"
                    tag: "chat-agent"
                  - pattern: "^/v1/completions"
                    tag: "completion-agent"
                  - pattern: "^/responses"
                    tag: "codex-agent"

# 端点配置
endpoints:
  # 专门处理对话类 Agent
  - name: chat-agent-endpoint
    url_openai: https://chat-provider.com
    tags: ["chat-agent"]
    priority: 1
    
  # 专门处理编程类 Agent
  - name: code-agent-endpoint
    url_openai: https://code-provider.com
    tags: ["codex-agent", "completion-agent"]
    priority: 2
    
  # 通用端点作为兜底
  - name: universal-endpoint
    url_openai: https://universal-provider.com
    priority: 3
```

### 2. 参数优化

为不同类型的 Agent 优化参数：

```yaml
endpoints:
  # 对话 Agent - 注重创造性
  - name: chat-optimized
    parameter_overrides:
      - key: temperature
        value: 0.8
      - key: top_p
        value: 0.9
      - key: presence_penalty
        value: 0.1
    
  # 编程 Agent - 注重准确性
  - name: code-optimized
    parameter_overrides:
      - key: temperature
        value: 0.2
      - key: top_p
        value: 0.1
      - key: frequency_penalty
        value: 0.0
    
  # 数据分析 Agent - 注重稳定性
  - name: data-optimized
    parameter_overrides:
      - key: temperature
        value: 0.0
      - key: max_tokens
        value: 4096
```

### 3. 多模型支持

为 Agent 配置多个模型选择：

```yaml
endpoints:
  - name: multi-model-endpoint
    url_openai: https://api.provider.com
    model_rewrite:
      enabled: true
      rules:
        # 编程任务
        - source_pattern: "*-code*"
          target_model: code-specialist-model
        # 对话任务  
        - source_pattern: "*-chat*"
          target_model: chat-specialist-model
        # 分析任务
        - source_pattern: "*-analysis*"
          target_model: analysis-specialist-model
        # 默认模型
        - source_pattern: "*"
          target_model: general-purpose-model
```

### 4. 工具调用与 count_tokens 配置

- **的标准工具调用支持**：当 Agent 请求包含 `tools` 字段时，CCCC 会自动处理 OpenAI 标准的工具调用格式，确保在不同 API 间正确转换。
- **端点级 count_tokens**：对于仅供补全的供应商，可在端点配置中（或管理界面“高级配置”面板）关闭 `count_tokens_enabled`，避免触发不兼容的 `/messages/count_tokens` 调用。
- **本地估算兜底**：当所有端点都不支持 `count_tokens` 时，CCCC 会返回轻量估算并标记 `proxy_estimated=true`，日志中不会再重复记录无效的 404/Invalid URL。

## 🎯 具体 Agent 集成示例

### Cursor 集成

```yaml
# Cursor 专用端点配置
endpoints:
  - name: cursor-endpoint
    url_openai: https://api.provider.com
    endpoint_type: openai
    auth_type: auth_token
    auth_value: cursor-token
    enabled: true
    priority: 1
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: gpt-4
          target_model: provider-gpt4-equivalent
        - source_pattern: gpt-3.5-turbo
          target_model: provider-gpt35-equivalent
    parameter_overrides:
      - key: temperature
        value: 0.3
      - key: max_tokens
        value: 2048
```

**Cursor 配置文件** (`~/.cursor/settings.json`):
```json
{
  "openai.baseURL": "http://127.0.0.1:8080/v1",
  "openai.apiKey": "hello",
  "models.completion": "gpt-4",
  "models.chat": "gpt-4"
}
```

### Continue 集成

```yaml
# Continue 专用端点
endpoints:
  - name: continue-endpoint
    url_openai: https://api.provider.com
    tags: ["continue"]
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: gpt-4
          target_model: provider-premium-model
        - source_pattern: claude-*
          target_model: provider-claude-model
```

**Continue 配置文件** (`~/.continue/config.json`):
```json
{
  "models": [
    {
      "title": "CCCC Provider",
      "provider": "openai",
      "model": "gpt-4",
      "apiBase": "http://127.0.0.1:8080/v1",
      "apiKey": "hello"
    }
  ]
}
```

### Aider 集成

```yaml
# Aider 专用端点
endpoints:
  - name: aider-endpoint
    url_openai: https://api.provider.com
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: gpt-4
          target_model: provider-coding-model
        - source_pattern: gpt-3.5-turbo
          target_model: provider-fast-model
    parameter_overrides:
      - key: temperature
        value: 0.1
      - key: max_tokens
        value: 8192
```

**Aider 配置**:
```bash
export OPENAI_API_BASE="http://127.0.0.1:8080/v1"
export OPENAI_API_KEY="hello"
export AIDER_MODEL="gpt-4"
```

## 🛡️ 安全和认证

### 1. API Key 管理

```yaml
# 为不同 Agent 使用不同的认证
endpoints:
  - name: production-agent
    auth_type: auth_token
    auth_value: prod-api-key
    
  - name: development-agent  
    auth_type: auth_token
    auth_value: dev-api-key
    
  - name: testing-agent
    auth_type: auth_token
    auth_value: test-api-key
```

### 2. 访问控制

使用标签路由实现访问控制：

```yaml
endpoints:
  # 内网 Agent 专用
  - name: internal-agent
    url_openai: https://internal-api.com
    tags: ["internal", "trusted"]
    priority: 1
    
  # 外网 Agent 专用
  - name: external-agent
    url_openai: https://external-api.com
    tags: ["external"]
    priority: 2
```

### 3. 请求限制

```yaml
# 在系统级别配置限制
server:
    rate_limit:
        enabled: true
        requests_per_minute: 1000
        burst: 100
```

## 📊 监控和调试

### Agent 请求追踪

```yaml
logging:
    level: info
    log_request_body: "truncated"
    log_response_body: "none"
    # 为不同 Agent 添加标识
    include_agent_info: true
```

### Web 界面监控

访问 http://localhost:8080 查看：
- **Agent 类型分布**：不同 Agent 的请求比例
- **性能指标**：各 Agent 的响应时间和成功率
- **错误分析**：按 Agent 类型分类的错误统计

### 日志分析

```bash
# 查看特定 Agent 的请求
grep "chat-agent" logs/proxy.log
grep "codex-agent" logs/proxy.log

# 查看模型重写情况
grep "Model rewritten" logs/proxy.log | grep "gpt-4"

# 查看格式转换
grep "format conversion" logs/proxy.log
```

## 🔧 故障排除

### 常见问题

#### Q: Agent 连接被拒绝
**A:** 检查配置：
1. 端点是否启用且优先级正确
2. 认证信息是否正确
3. 模型重写规则是否匹配

#### Q: 响应格式不兼容
**A:** 确认端点类型：
- OpenAI 格式 Agent 使用 `endpoint_type: openai`
- Anthropic 格式 Agent 使用 `endpoint_type: anthropic`
- 通用 Agent 配置双 URL 支持

#### Q: 性能问题
**A:** 优化配置：
1. 启用健康检查自动隔离故障端点
2. 调整超时时间设置
3. 使用参数覆盖优化请求参数

### 调试模式

```yaml
logging:
    level: debug
    log_request_body: "full"
    log_response_body: "truncated"
    
# 启用详细的端点日志
endpoints:
  - name: debug-endpoint
    debug: true
    log_all_requests: true
```

## 🔧 端点自适应能力详解

### 1. 双URL智能路由（核心特性）
同一端点可同时配置 `url_anthropic` 与 `url_openai`，实现跨客户端复用：

```yaml
# 完整双URL配置
endpoints:
  - name: universal-endpoint
    url_anthropic: https://api.provider.com/v1/anthropic  # Claude Code专用
    url_openai: https://api.provider.com/v1/openai        # Codex专用
    auth_type: auth_token
    auth_value: shared-token
```

**智能路由逻辑**：
- **Claude Code 请求** → 自动路由到 `url_anthropic`
- **Codex 请求** → 自动路由到 `url_openai`
- **缺少对应URL时** → 自动在代理侧进行格式转换：
  - 缺少 `url_openai`：Codex请求转为Anthropic格式 → 发送到 `url_anthropic`
  - 缺少 `url_anthropic`：Claude Code请求转为OpenAI格式 → 发送到 `url_openai`

### 2. OpenAI格式自适应学习
系统智能处理 OpenAI 的新旧两种API格式，并持久化学习结果：

**自适应流程**：
1. **首次请求**：优先尝试 `/responses`（Codex新格式）
2. **失败检测**：遇到 4xx 或格式错误自动切换 `/chat/completions`
3. **学习记录**：成功格式写入端点的 `openai_preference` 字段
4. **加速后续**：下次直接使用学习到的格式，避免试错

**配置示例**：
```yaml
endpoints:
  - name: adaptive-endpoint
    url_openai: https://api.openai.com
    openai_preference: auto  # auto（自动）| responses（新格式）| chat_completions（旧格式）
```

**参考文档**：
- OpenAI Responses API: https://platform.openai.com/docs/guides/responses
- OpenAI Chat API: https://platform.openai.com/docs/guides/chat

### 3. 模型映射协同机制
结合客户端类型和端点 `model_rewrite` 规则选择最佳供应商模型：

**显式重写**（推荐）：
```yaml
endpoints:
  - name: multi-client-endpoint
    url_anthropic: https://api.provider.com/anthropic
    url_openai: https://api.provider.com/openai
    model_rewrite:
      enabled: true
      rules:
        # Claude Code模型映射
        - source_pattern: claude-sonnet-4-20250514
          target_model: provider-claude-model
        - source_pattern: claude-*
          target_model: provider-default-claude
        # Codex模型映射
        - source_pattern: gpt-5*
          target_model: provider-gpt-model
        - source_pattern: gpt-*
          target_model: provider-default-gpt
```

**隐式重写**（零配置）：
- **Claude Code** → 未配置重写规则时默认使用 `claude-sonnet-4-20250514`
- **Codex** → 未配置重写规则时默认使用 `gpt-5`
- **通用端点** → 优先使用 `target_model`，确保命中供应商自定义模型

### 4. 智能拉黑策略分级
通过 `blacklist` 配置区分三种错误类型，避免误拉黑：

**错误类型分类**（`internal/validator/error_types.go`）：
```go
// 1. 业务错误（BusinessError）
// - HTTP 400-499 且有错误信息
// - 例：模型不支持、参数错误、内容违规
// - 不应触发拉黑（可能是用户输入问题）

// 2. 配置错误（ConfigError）
// - HTTP 401/403 认证失败
// - HTTP 422 格式验证失败
// - 应触发拉黑（端点配置有问题）

// 3. 服务器错误（ServerError）
// - HTTP 500-599
// - 网络连接失败、超时
// - 应触发拉黑（端点不可用）
```

**分级策略配置**：
```yaml
# 生产环境推荐配置
blacklist:
  enabled: true              # 启用拉黑功能
  auto_blacklist: true        # 自动拉黑
  business_error_safe: true   # ✓ 业务错误安全（不拉黑）
  config_error_safe: false    # ✗ 配置错误拉黑
  server_error_safe: false    # ✗ 服务器错误拉黑

# 开发环境宽松配置
blacklist:
  enabled: true
  auto_blacklist: true
  business_error_safe: true   # ✓ 业务错误安全
  config_error_safe: true     # ✓ 配置错误也安全（方便调试）
  server_error_safe: true     # ✓ 服务器错误也安全（方便调试）

# 调试模式（完全禁用拉黑）
blacklist:
  enabled: false              # 所有失败仅记录日志，不拉黑任何端点
```

### 5. 端点测试向导与配置回写
管理界面支持全自动端点测试，学习结果同步到端点配置：

**一键测试功能**：
1. **单端点测试**：
   - 自动测试 Anthropic 和 OpenAI 两种格式（如果都配置了URL）
   - 使用重写后的模型进行真实调用
   - 验证认证方式（`x-api-key` vs `Authorization`）
   - 测量响应时间和性能指标

2. **批量测试**：
   - 一次性测试所有端点
   - 并发执行，快速获得完整报告
   - 显示HTTP状态、响应时间、格式验证结果

**配置自动回写**：
测试学习到的以下信息会自动更新到端点配置：
- `openai_preference`：成功的OpenAI格式（responses/chat_completions）
- `DetectedAuthHeader`：有效的认证头类型（x-api-key/Authorization）
- `LearnedUnsupportedParams`：端点不支持的参数列表（运行时学习）

**测试结果示例**：
```json
{
  "endpoint_name": "test-endpoint",
  "results": [
    {
      "format": "anthropic",
      "success": true,
      "response_time": 1234,
      "status_code": 200,
      "url": "https://api.provider.com/v1/messages",
      "model_rewrite": {
        "original_model": "claude-sonnet-4-20250514",
        "rewritten_model": "qwen-plus",
        "rule_applied": true
      }
    },
    {
      "format": "openai",
      "success": true,
      "response_time": 987,
      "status_code": 200,
      "url": "https://api.provider.com/v1/chat/completions"
    }
  ]
}
```

### 6. 认证方式自动探测
当 `auth_type` 设为 `auto` 或留空时，系统自动探测有效认证方式：

```yaml
endpoints:
  - name: auto-detect-auth
    url_anthropic: https://api.provider.com
    auth_type: auto            # 或不填此字段
    auth_value: your-api-key
```

**探测流程**：
1. **首次请求/测试**：依次尝试 `x-api-key` 和 `Authorization` 头
2. **记录成功方案**：保存到端点的 `DetectedAuthHeader` 字段（内存）
3. **后续请求优化**：直接使用检测到的认证方式
4. **持久化选项**：可选择将学习结果写入配置文件

## 🎯 最佳实践

### 1. 端点规划
- **专用端点**：为高流量 Agent 配置专用端点
- **通用端点**：为低流量或测试 Agent 配置通用端点
- **故障转移**：配置多层级端点作为备份

### 2. 模型管理
- **统一命名**：使用一致的模型命名规范
- **版本控制**：为不同版本配置不同的重写规则
- **性能优化**：根据任务类型选择合适的模型

### 3. 监控告警
- **成功率监控**：设置成功率告警阈值
- **响应时间监控**：监控异常延迟
- **错误率监控**：及时发现端点问题

### 4. 扩展性考虑
- **水平扩展**：支持添加新的端点和提供商
- **垂直扩展**：支持单个端点的容量扩展
- **配置热更新**：支持不重启服务的配置更新

## 📚 相关文档

- [README.md](./README.md) - 总体介绍和配置
- [CLAUDE.md](./CLAUDE.md) - Claude Code 专用指南
- [CHANGELOG.md](./CHANGELOG.md) - 版本更新记录

## 🤝 贡献

欢迎提交更多 Agent 的集成示例和配置最佳实践！请通过 GitHub Issues 或 Pull Requests 贡献您的经验。
