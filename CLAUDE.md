# Claude Code 集成指南

## 📋 概述

CCCC (Claude Code and Codex Companion) 为 Claude Code 提供了完整的 API 代理支持，实现多端点负载均衡、故障转移和模型重写功能。零配置工具调用能力由 Toolify 社区鼎力协助打造，文档中也包含相关指引。

### Codex 兼容性增强

CCCC 完整支持 Codex 客户端，自动处理各种格式转换场景：

**流式响应转换**：
- ✅ **真实流式 SSE**：上游返回 `text/event-stream` 时，自动转换 Chat Completions SSE → Responses API SSE
- ✅ **模拟流式 SSE**：上游返回 JSON 但客户端期望流式时（`stream: true`），自动执行三阶段转换：
  1. Chat Completions JSON → Responses JSON
  2. Responses JSON → Responses SSE 流
  3. 立即发送并返回，避免被后续逻辑覆盖
- ✅ **格式探测**：根据 `response_keys` 智能判断响应类型并提取内容
- ✅ **事件完整性**：确保发送 `response.created`、`response.output_text.delta`、`response.completed` 三个核心事件

**路径自适应**：
- `/responses` 请求根据 `openai_preference` 配置智能路由
- 自动学习端点是否原生支持 Responses API
- 失败时自动降级到 `/chat/completions` 并转换格式

详见 [AGENTS.md](AGENTS.md) 获取完整的 Codex 集成说明。

## 🚀 快速配置

### 端点URL智能路由

CCCC支持端点双URL配置，实现智能客户端路由：

```yaml
endpoints:
  # 双URL配置 - 智能路由
  - name: smart-endpoint
    url_anthropic: https://api.provider.com/anthropic  # Claude Code请求 → 此URL
    url_openai: https://api.provider.com/openai        # Codex请求 → 此URL
    auth_type: auth_token
    auth_value: your-token
    
  # 单URL配置 - 自动格式转换
  - name: anthropic-only
    url_anthropic: https://api.provider.com/anthropic  # 仅配置Anthropic URL
    # Claude Code直接使用，Codex请求会自动转换为Anthropic格式
    
  - name: openai-only
    url_openai: https://api.provider.com/openai        # 仅配置OpenAI URL
    # Codex直接使用，Claude Code请求会自动转换为OpenAI格式
```

### OpenAI格式自适应机制

对于OpenAI端点，系统支持新旧两种API格式的自适应：

```yaml
endpoints:
  - name: adaptive-openai
    url_openai: https://api.openai.com
    openai_preference: auto  # 可选值：auto | responses | chat_completions
```

**自适应流程**：
1. **优先尝试**：`/responses` 格式（Codex新版API，https://platform.openai.com/docs/guides/responses）
2. **失败降级**：自动切换 `/chat/completions` 格式（传统API，https://platform.openai.com/docs/guides/chat）
3. **学习记录**：成功格式保存到端点配置的 `openai_preference` 字段
4. **加速后续**：下次请求直接使用学习到的格式

### 方式一：自动配置脚本（推荐）

1. 启动 CCCC 服务：`./cccc -config config.yaml -port 8080`
2. 访问 http://localhost:8080/help
3. 下载对应系统的配置脚本：
   - **macOS**: `ccc.command`
   - **Linux**: `ccc.sh`
   - **Windows**: `ccc.bat`
4. 执行脚本，自动配置环境变量

### 方式二：手动配置

#### 环境变量配置

**Linux/macOS:**
```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_AUTH_TOKEN="hello"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"
export API_TIMEOUT_MS="600000"
```

**Windows (PowerShell):**
```powershell
$env:ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
$env:ANTHROPIC_AUTH_TOKEN="hello"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"
$env:API_TIMEOUT_MS="600000"
```

#### settings.json 配置

编辑 `~/.claude/settings.json`：
```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8080",
    "ANTHROPIC_AUTH_TOKEN": "hello",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "API_TIMEOUT_MS": "600000"
  }
}
```

## 🔧 端点配置

### Claude Code 专用端点

```yaml
endpoints:
  - name: anthropic-official
    url_anthropic: https://api.anthropic.com
    auth_type: api_key
    auth_value: sk-ant-xxxxx
    enabled: true
    priority: 1
```

### 通用端点（支持 Claude Code 和 Codex）

```yaml
endpoints:
  - name: universal-provider
    url_anthropic: https://api.provider.com/anthropic
    url_openai: https://api.provider.com/openai
    auth_type: auth_token
    auth_value: your-token
    enabled: true
    priority: 2
    count_tokens_enabled: false   # 可选：如上游不支持 /messages/count_tokens，可关闭以避免探测
```

## 🧠 Tool Calling 零配置增强（感谢 Toolify）

- 无需自定义系统提示，CCCC 会在检测到 `tools` 字段时自动注入规范化提示，并解析模型返回的 XML 函数调用。
- 支持端点级控制：`native_tool_support` 与 `tool_enhancement_mode`（auto/force/disable），留空时会根据测试结果自动学习。
- 请求日志会记录 `tool_enhancement_applied`、`tool_call_count` 等字段，在 `/admin/logs` 页面即可验证函数调用是否触发。
- 若上游返回工具调用相关的业务错误，代理会自动将该端点切换到增强模式并持久化，避免持续失败。

## 🎯 模型重写机制

### 显式重写规则

为Claude Code的模型配置映射规则：

```yaml
endpoints:
  - name: claude-rewriter
    url_anthropic: https://api.provider.com
    model_rewrite:
      enabled: true
      rules:
        # Claude Code最新模型映射
        - source_pattern: claude-sonnet-4-20250514
          target_model: qwen-plus
        - source_pattern: claude-3-5-sonnet-20241022
          target_model: qwen-turbo
        # 通用Claude模型映射
        - source_pattern: claude-*
          target_model: qwen-max
```

### 隐式重写（零配置）

对于未配置重写规则的通用端点：

```yaml
endpoints:
  - name: universal-endpoint
    url_anthropic: https://api.provider.com
    # 没有配置 model_rewrite，系统自动应用隐式重写：
    # - Claude Code 请求：非 Claude 模型 → claude-sonnet-4-20250514
    # - 目标端点：优先使用声明的 target_model 确保命中供应商模型
```
endpoints:
  - name: universal-provider
    url_anthropic: https://api.provider.com/anthropic
    url_openai: https://api.provider.com/openai
    endpoint_type: openai
    auth_type: auth_token
    auth_value: your-token
    enabled: true
    priority: 2
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: claude-*
          target_model: provider-claude-equivalent

# 当只配置其中一个 URL 时，CCCC 仍会自动转换请求格式以兼容另一端：
# - 仅有 url_anthropic：OpenAI/Codex 请求会转换为 Anthropic 格式再转发
# - 仅有 url_openai：Claude Code 请求会转换为 OpenAI 格式再转发
# 因此最简配置只需保留一个 URL 即可完成跨客户端复用。
```

## 🎯 模型重写

### 常见模型映射

```yaml
model_rewrite:
  enabled: true
  rules:
    # Claude Code 模型映射到国产模型
    - source_pattern: claude-sonnet-4-20250514
      target_model: qwen-plus
    - source_pattern: claude-3-5-sonnet-20241022
      target_model: qwen-turbo
    - source_pattern: claude-*
      target_model: qwen-max
```

### 隐式重写（通用端点）

对于未配置显式重写规则的通用端点，系统会自动应用隐式重写：
- **Claude Code 客户端**：非 Claude 模型自动重写为 `claude-sonnet-4-20250514`
- **Codex 客户端**：非 GPT 模型自动重写为 `gpt-5`
- **第三方端点**：若启用了模型重写规则，CCCC 会优先映射到端点声明的 `target_model`，确保命中供应商自定义模型名

## 🔄 格式转换

CCCC 自动处理 Claude Code 与不同端点类型之间的格式转换：

### Anthropic 端点
- 直接透传，无需转换
- 保持原生 API 格式和功能
- 如果 Codex/OpenAI 端点请求被路由到仅有 `url_anthropic` 的端点，系统会自动将请求转换为 Anthropic `messages` 格式

### OpenAI 端点  
- 自动将 Anthropic 格式转换为 OpenAI 格式
- 支持工具调用、流式响应等高级功能
- 自动转换消息格式、参数名称等
- 默认优先尝试 Codex `/responses` 接口；若遇到 4xx/网络错误会自动切换 `/chat/completions`
- 学习到的最佳格式会写入端点的 `openai_preference` 字段，以便下次直接使用

#### 批量连通性测试

```bash
# 使用当前配置批量验证所有端点
go run ./cmd/test_endpoints -config config.yaml
```

- 工具会分别向每个端点发送 Anthropic 与 OpenAI 的测试请求，自动补齐所需 Header；
- 测试过程中会记录 400/401/429 等错误类型，并更新运行时的 `openai_preference` 与认证检测结果；
- 建议在停用自动拉黑的情况下使用，以免测试期造成端点拉黑。

## ⚙️ OpenAI 格式自适应

- 第一次请求：尝试 `/responses` 以兼容 Codex 新版 API
- 请求失败：自动转换为 `/chat/completions` 并立即重试
- 学习记录：成功/失败策略会保存到端点配置（非必填项自动补齐）
- 手动覆盖：可在端点上显式设置 `openai_preference: responses|chat_completions|auto`

## 🛡️ 高级功能

### 双URL配置

一个端点同时配置两个URL，根据客户端类型智能路由：

```yaml
endpoints:
  - name: smart-provider
    url_anthropic: https://api.provider.com/anthropic  # Claude Code 路由
    url_openai: https://api.provider.com/openai        # Codex 路由
    endpoint_type: openai
    auth_type: auth_token
    auth_value: token
    enabled: true
    priority: 1
```

### 标签路由

为特定类型的 Claude Code 请求配置专用端点：

```yaml
endpoints:
  - name: claude-code-premium
    url_anthropic: https://premium-api.com
    tags: ["claude-code", "premium"]
    # 只处理带有 claude-code 和 premium 标签的请求
```

### 参数覆盖

为 Claude Code 请求设置特定参数：

```yaml
endpoints:
  - name: claude-optimized
    parameter_overrides:
      - key: temperature
        value: 0.3
      - key: max_tokens
        value: 8192
```

## 📊 监控与调试

### Web 管理界面

访问 http://localhost:8080 查看：
- **仪表板**：Claude Code 请求统计和成功率
- **请求日志**：过滤查看 Claude Code 相关请求
- **端点状态**：监控各端点的 Claude Code 支持情况

### 日志查看

```bash
# 查看 Claude Code 请求
grep "claude-code" logs/proxy.log

# 查看模型重写日志
grep "Model rewritten" logs/proxy.log

# 查看格式转换日志
grep "format conversion" logs/proxy.log
```

## 🔧 故障排除

### 常见问题

#### Q: Claude Code 连接失败
**A:** 检查以下几点：
1. CCCC 服务是否正常运行
2. 环境变量 `ANTHROPIC_BASE_URL` 是否正确设置
3. 端点是否配置了 `url_anthropic` 或通用 URL
4. 查看日志：`grep "claude-code" logs/proxy.log`

#### Q: 模型不支持错误
**A:** 配置模型重写规则：
```yaml
model_rewrite:
  enabled: true
  rules:
    - source_pattern: claude-sonnet-4-20250514
      target_model: 实际支持的模型名
```

#### Q: 功能缺失（如工具调用不工作）
**A:** 确认端点类型：
- 使用 `endpoint_type: anthropic` 获得完整功能支持
- OpenAI 端点会自动转换，但某些高级功能可能受限

### 调试模式

启用详细日志进行调试：

```yaml
logging:
    level: debug
    log_request_body: "full"
    log_response_body: "truncated"
```

## 🎯 最佳实践

### 1. 端点配置策略
- **主要端点**：使用官方 Anthropic API，配置最高优先级
- **备用端点**：配置国产大模型作为降级选项
- **通用端点**：设置较低优先级，作为最后的兜底选择

### 2. 模型重写策略
- 为每个备用端点配置完整的模型映射规则
- 使用通配符规则处理未明确列出的模型
- 定期更新模型映射以跟上 Claude Code 的模型更新

### 3. 性能优化
- 启用健康检查，自动隔离故障端点
- 配置合适的超时时间，避免长时间等待
- 使用参数覆盖优化请求参数

### 4. 安全考虑
- 在生产环境中使用真实的 API Key
- 定期轮换认证凭据
- 监控异常请求模式

### 5. count_tokens 建议
- 对不支持 `/messages/count_tokens` 的第三方端点，推荐将 `count_tokens_enabled` 设置为 `false`，避免重复探测。
- 如需保留估算能力，可依赖 CCCC 的本地 `proxy_estimated` 结果；实际补全仍然会照常路由到可用端点。

## 📚 相关文档

- [README.md](./README.md) - 总体介绍和配置
- [AGENTS.md](./AGENTS.md) - AI Agent 集成指南
- [CHANGELOG.md](./CHANGELOG.md) - 版本更新记录
- `blacklist.enabled`: 是否启用端点拉黑（关闭后所有失败仅作记录）
- `blacklist.auto_blacklist`: 失败是否触发自动拉黑
- `blacklist.business_error_safe`: 业务错误（API 正常返回错误）是否视为安全
- `blacklist.config_error_safe`: 配置错误是否视为安全
- `blacklist.server_error_safe`: 服务端/网络错误是否视为安全

## 🧪 端点测试助手

### 自动化测试功能
- **一键自检**：在管理后台触发"测试端点"即可对当前端点执行 Anthropic/OpenAI 双格式测试
- **重写验证**：自检会使用模型重写后的模型进行调用，确保实际请求与端点兼容
- **认证探测**：当 `auth_type` 为空或为 `auto` 时，自检会依次尝试 `Authorization` 与 `x-api-key` 头并记录成功选项
- **配置回写**：测试结果会同步更新端点内存配置（如 `openai_preference`、检测到的认证方式），减轻人工维护成本

### 端点URL智能路由（需求1）
当端点同时配置两个URL时：
- **`url_anthropic`**：专门服务 Claude Code 客户端请求
- **`url_openai`**：专门服务 Codex 客户端请求
- **智能路由**：系统自动根据客户端类型选择对应URL

当端点只配置一个URL时：
- **仅 `url_anthropic`**：OpenAI/Codex 请求会在代理侧自动转换为 Anthropic 格式再转发
- **仅 `url_openai`**：Claude Code 请求会在代理侧自动转换为 OpenAI 格式再转发
- **格式转换引擎**：内置 `anthropic_to_openai_response.go` 和 `openai_to_anthropic_request.go` 处理双向转换

```yaml
# 双URL配置示例
endpoints:
  - name: dual-endpoint
    url_anthropic: https://api.provider.com/v1/anthropic  # Claude Code → 此URL
    url_openai: https://api.provider.com/v1/openai        # Codex → 此URL
    auth_type: auth_token
    auth_value: your-token

# 单URL配置示例（自动格式转换）
endpoints:
  - name: single-anthropic
    url_anthropic: https://api.provider.com/v1/messages
    # 没有url_openai，但Codex请求会自动转换为Anthropic格式并发送

  - name: single-openai
    url_openai: https://api.provider.com/v1/chat/completions
    # 没有url_anthropic，但Claude Code请求会自动转换为OpenAI格式并发送
```

### OpenAI格式自适应（需求2）
系统智能处理 OpenAI 的新旧两种API格式：

**首次请求流程**：
1. **优先尝试 `/responses` 格式**（OpenAI新格式）：适用于 Codex 最新版本
2. **失败自动降级**：遇到 4xx 错误或格式不兼容时，立即转换为 `/chat/completions` 格式重试
3. **学习记录**：成功的格式偏好会自动保存到端点配置的 `openai_preference` 字段
4. **加速后续请求**：下次请求直接使用学习到的格式，避免重复试错

**配置选项**：
```yaml
endpoints:
  - name: openai-endpoint
    url_openai: https://api.openai.com
    openai_preference: auto  # 可选值：auto | responses | chat_completions
    # auto: 自动检测（默认）
    # responses: 强制使用 /responses 格式
    # chat_completions: 强制使用 /chat/completions 格式
```

**参考文档**：
- OpenAI Responses API: https://platform.openai.com/docs/guides/responses
- OpenAI Chat API: https://platform.openai.com/docs/guides/chat

### 模型重写机制（需求3）

**显式重写**：通过 `model_rewrite` 配置明确的模型映射规则
```yaml
endpoints:
  - name: custom-provider
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: claude-sonnet-4-20250514  # Claude Code请求的模型
          target_model: qwen-plus                    # 映射到供应商的模型
        - source_pattern: gpt-5*                     # Codex请求的模型
          target_model: qwen-max                     # 映射到供应商的模型
```

**隐式重写**：对于未配置显式规则的通用端点，系统自动应用默认映射
- **Claude Code 客户端** → 默认使用 `claude-sonnet-4-20250514`
- **Codex 客户端** → 默认使用 `gpt-5`
- **自适应供应商模型**：优先使用端点配置的 `target_model`，确保命中供应商自定义模型名

### 智能拉黑策略（需求4）

系统区分三种错误类型，并提供精细化的拉黑控制：

**错误类型定义**（`internal/validator/error_types.go`）：
1. **业务错误（BusinessError）**：API正常返回错误响应（如模型不支持、参数错误）
   - HTTP 400-499 且有结构化错误信息
   - 通常不应触发端点拉黑（可能是用户输入问题）

2. **配置错误（ConfigError）**：客户端配置问题（如认证失败、请求格式错误）
   - HTTP 401, 403 认证失败
   - HTTP 422 格式验证失败
   - 应该触发端点拉黑（配置有问题）

3. **服务器错误（ServerError）**：基础设施问题（如5xx错误、网络超时）
   - HTTP 500-599
   - 网络连接失败、超时
   - 应该触发端点拉黑（端点不可用）

**全局拉黑配置**（`config.yaml`）：
```yaml
blacklist:
  enabled: true              # 全局拉黑功能开关（关闭后所有失败仅记录，不拉黑）
  auto_blacklist: true        # 是否自动拉黑失败端点
  business_error_safe: true   # 业务错误不触发拉黑（推荐：true）
  config_error_safe: false    # 配置错误触发拉黑（推荐：false）
  server_error_safe: false    # 服务器错误触发拉黑（推荐：false）
```

**拉黑策略示例**：
```yaml
# 宽松策略（开发/测试环境）
blacklist:
  enabled: true
  auto_blacklist: true
  business_error_safe: true   # 业务错误安全
  config_error_safe: true     # 配置错误也安全
  server_error_safe: true     # 服务器错误也安全

# 严格策略（生产环境）
blacklist:
  enabled: true
  auto_blacklist: true
  business_error_safe: true   # 业务错误安全（避免误判）
  config_error_safe: false    # 配置错误拉黑（快速隔离问题端点）
  server_error_safe: false    # 服务器错误拉黑（保护整体可用性）

# 完全禁用拉黑（调试模式）
blacklist:
  enabled: false              # 关闭拉黑，所有失败仅记录日志
```

### 测试验证机制（需求5 & 6）

**使用重写后模型验证**：
- 测试时使用 `model_rewrite` 规则后的实际模型名
- Claude Code 默认测试模型：`claude-sonnet-4-20250514`（如有重写则使用目标模型）
- Codex 默认测试模型：`gpt-5`（如有重写则使用目标模型）
- 确保测试环境与实际请求完全一致

**认证方式自动探测**：
```yaml
endpoints:
  - name: auto-auth-endpoint
    url_anthropic: https://api.provider.com
    auth_type: auto            # 或留空，触发自动探测
    auth_value: your-api-key
```

探测流程：
1. **首次测试**：依次尝试 `x-api-key` 和 `Authorization` 两种认证头
2. **记录成功方案**：将成功的认证方式记录到端点的 `DetectedAuthHeader` 字段
3. **后续请求**：直接使用学习到的认证方式，提高效率

**测试方法**：

1. **单端点测试**（管理界面）：
   - 点击端点操作菜单中的"测试端点"
   - 自动测试 Anthropic 和 OpenAI 两种格式（如果都配置了URL）
   - 显示响应时间、HTTP状态码、格式验证结果
   - 实时更新 `openai_preference` 等学习字段

2. **批量测试**（管理界面）：
   - 点击"批量测试所有端点"
   - 并发测试所有启用的端点
   - 生成完整的连通性测试报告
   - 包含响应时间、模型重写验证、认证方式探测结果

3. **程序化测试**（API调用）：
```bash
# 测试单个端点
curl -X POST http://localhost:8080/api/endpoints/test \
  -H "Content-Type: application/json" \
  -d '{"endpoint_name": "your-endpoint", "format": "anthropic"}'

# 批量测试
curl -X POST http://localhost:8080/api/endpoints/test-all
```

**测试返回信息**：
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
      },
      "performance_metrics": {
        "first_byte_time": 500,
        "content_download_time": 734,
        "total_size": 2048
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

**配置自动回写**：
- `openai_preference`：学习到的OpenAI格式偏好
- `DetectedAuthHeader`：成功的认证头类型
- 运行时学习的参数限制（`LearnedUnsupportedParams`）
- 这些信息保存在内存中，可选择性持久化到配置文件
