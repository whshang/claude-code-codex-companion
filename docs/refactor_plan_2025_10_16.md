# 改造计划：API 端点统一与配置简化

**日期**: 2025-10-16
**作者**: Gemini

## 1. 摘要 (TL;DR)

本次改造的核心目标是简化并明确化项目 API 的代理行为。我们将废弃当前动态、模糊的 API 端点（如 `/openai`），并引入三个功能单一、静态的代理端点：`/claude`、`/openai_chat` 和 `/openai_responses`。与此对应，`config.yaml` 的配置方式也将进行简化，要求为每种上游格式提供独立的 URL。此举将彻底解决端点格式兼容性问题，使系统行为更加可预测和稳定。

## 2. 动机 (Motivation)

经过多次测试发现，当前系统试图在多种 OpenAI/Anthropic API 格式（`/messages`, `/responses`, `/chat/completions`）之间进行动态双向转换的架构，引发了以下问题：

*   **兼容性问题**: 部分上游端点（特别是仅支持 Codex 的服务）严格要求 `/chat/completions` 格式，而我们动态转换的请求可能无法通过其验证。
*   **配置模糊**: 用户在配置 `url_openai` 时，难以确定应提供哪种格式的 URL，导致代理行为不符合预期。
*   **逻辑复杂**: 双向转换和动态判断的逻辑使得代码库难以维护，排查问题（Debug）的成本很高。

为了让代理服务更健壮、更易于使用，我们决定放弃复杂的动态转换，回归到更简单、更明确的单向代理模型。

## 3. 拟定变更 (Proposed Changes)

### 3.1. API 端点重构

我们将对外暴露以下三个固定的 API 端点，并废弃所有旧的、动态的端点。

| 新的本地端点 | 绑定的上游格式 | 主要用途和目标服务 |
| :--- | :--- | :--- |
| `POST /claude` | Anthropic Messages | 代理到 Claude 系列模型 |
| `POST /openai_chat` | OpenAI Chat Completions | 代理到 GPT-3.5/4, 88code 等现代模型 |
| `POST /openai_responses` | OpenAI Responses (Legacy) | 兼容需要旧格式的特定或自建模型 |

客户端应用需要根据其目标上游服务的格式，选择调用对应的本地端点。

### 3.2. 配置文件 `config.yaml` 结构变更

`config.yaml` 将从一个通用的 `endpoints` 列表，转变为使用功能明确的独立 URL 字段。

**变更前 (Before):**

```yaml
# 旧的、模糊的配置方式
endpoints:
  - name: "my-openai-service"
    type: openai
    url: https://api.example.com/v1
    # 系统需要猜测或拼接 /chat/completions 或 /responses
```

**变更后 (After):**

```yaml
# 新的、清晰的配置方式
# 直接为每种目标格式提供 URL

# 用于 /claude 端点
anthropic_url: "https://api.anthropic.com/v1/messages"

# 用于 /openai_chat 端点
openai_chat_url: "https://www.88code.org/v1/chat/completions"

# 用于 /openai_responses 端点
openai_responses_url: "https://api.example.com/v1/responses"
```

#### URL 自动补全机制

为提升用户体验，系统将实现智能的 URL 后缀补全。
*   **推荐**: 用户在配置中填写完整的 URL。
*   **兼容**: 如果用户只填写了基础 URL (e.g., `https://api.example.com`)，系统会根据字段类型自动补全标准路径 (e.g., 为 `openai_chat_url` 补全 `/v1/chat/completions`)。

### 3.3. 内部架构调整

*   **代理层 (`internal/proxy`)**:
    *   `server.go` 将注册上述三个新的静态路由。
    *   每个路由将绑定一个独立的处理器函数 (e.g., `handleOpenAIChatProxy`)。
    *   处理器函数内部将直接实例化其所需的格式适配器 (e.g., `NewOpenAIChatAdapter`)，不再通过工厂动态获取。

*   **转换层 (`internal/conversion`)**:
    *   `adapter_factory.go` 和 `conversion_manager.go` 中的动态适配器选择逻辑将被废弃。
    *   代码结构将从“请求 -> 动态工厂 -> 适配器”转变为“请求 -> 静态处理器 -> 特定适配器”。

#### 简化的数据流:

```
1. Client -> POST /openai_chat -> handleOpenAIChatProxy()
                                     |
                                     +--> adapter := NewOpenAIChatAdapter()
                                     |
                                     +--> adapter.ConvertRequest(...)
                                     |
                                     +--> Forward to `openai_chat_url`

2. Client -> POST /claude      -> handleClaudeProxy()
                                     |
                                     +--> adapter := NewAnthropicAdapter()
                                     ...
```

## 4. 对用户和开发者的影响

*   **配置文件必须更新**: 所有用户都需要将其 `config.yaml` 文件迁移到新的格式。
*   **客户端调用必须修改**: 所有调用本代理的应用，都需要将其请求的 URL 从旧端点更新为新的、功能匹配的端点（例如，从 `/openai` 改为 `/openai_chat`）。

## 5. 迁移步骤

1.  **更新配置**: 根据本文档 3.2 节的指导，修改 `config.yaml` 文件。
2.  **更新客户端**: 检查所有调用本服务的客户端程序，将其 API 请求地址更新为新的端点。
3.  **重启服务**: 使用新的配置重新启动代理服务。
4.  **测试**: 验证每个新端点是否都能成功代理到其对应的上游服务。

## 6. 预期收益

*   **清晰性 (Clarity)**: API 和配置的意图一目了然，杜绝了模糊性。
*   **稳定性 (Stability)**: 移除了复杂的动态逻辑，减少了潜在的 Bug，使系统行为 100% 可预测。
*   **易用性 (Ease of Use)**: 虽然要求配置更具体，但从根本上解决了用户的兼容性困惑。
*   **可维护性 (Maintainability)**: 代码逻辑更简单、直接，便于未来维护和新功能开发。
