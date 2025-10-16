# 格式转换与端点兼容性 | Format Conversion and Endpoint Compatibility

## 核心架构变更：从动态转换到静态代理

**重要提示**：本项目的核心架构已于 2025 年 10 月进行了重大重构。旧的动态、双向格式转换功能已被**完全移除**，取而代之的是一个更简单、更稳定、更明确的**静态单向代理模型**。

本文档描述的是当前（新版）架构。

---

### 架构概述 (New Architecture)

新系统的核心思想是“一个端点，一种格式”。每个暴露给客户端的 API 端点都硬性绑定一个特定的上游 API 格式。不再有运行时的格式猜测或自动转换。

#### 数据流路径

系统的行为现在是完全可预测的：

*   **Claude 路径**:
    `客户端 -> POST /claude -> [Anthropic 适配器] -> 上游 (anthropic_endpoint.url)`

*   **OpenAI Chat 路径**:
    `客户端 -> POST /openai_chat -> [OpenAI Chat 适配器] -> 上游 (openai_chat_endpoint.url)`

*   **OpenAI Responses (旧版) 路径**:
    `客户端 -> POST /openai_responses -> [OpenAI Responses 适配器] -> 上游 (openai_responses_endpoint.url)`

这种设计消除了所有模糊性。如果你向 `/claude` 端点发送请求，它将永远被当作 Anthropic Messages 格式处理并转发。

---

### 为何做出此项改变？ (Motivation)

旧的动态转换架构虽然功能强大，但也带来了几个难以解决的问题：

1.  **兼容性地狱**：不同的上游服务对 API 格式有严格但细微的差别，动态转换很难完美适配所有情况。
2.  **配置困惑**：用户难以理解 `url_openai` 和 `url_anthropic` 的组合会产生何种行为。
3.  **不稳定的行为**：运行时的“学习”和“降级”机制可能导致代理的行为在两次请求之间发生变化，使调试变得异常困难。

通过切换到静态代理模型，我们用**明确性**换取了**灵活性**，从而获得了更高的**稳定性和可靠性**。

---

### 这对用户意味着什么？

1.  **您必须选择正确的端点**：作为用户，您现在需要知道您的客户端应用期望何种 API 格式，并将其指向正确的代理端点。
    *   如果您的客户端（如最新版的 VSCode Claude 插件）使用 Anthropic Messages API，请将其指向 `http://localhost:8080/claude`。
    *   如果您的客户端（如大多数现代 AI 工具）使用 OpenAI Chat Completions API，请将其指向 `http://localhost:8080/openai_chat`。

2.  **配置文件极大简化**：您不再需要在 `config.yaml` 中配置如 `supports_responses` 或 `openai_preference` 等字段。只需为 `anthropic_endpoint` 或 `openai_chat_endpoint` 提供 URL 和 API 密钥即可。

---

### 已废弃的概念

以下概念和配置项在当前版本中已**不再适用**：

*   双向格式转换 (Bidirectional format conversion)
*   运行时端点能力学习 (Runtime learning of endpoint capabilities)
*   `endpoints` 列表中的 `url_anthropic` 和 `url_openai` 双 URL 配置
*   `supports_responses` 和 `openai_preference` 字段

我们相信，新的架构虽然在功能上有所简化，但其实用性和稳定性远超从前。

---
## English Version

## Core Architectural Change: From Dynamic Conversion to Static Proxy

**IMPORTANT**: The core architecture of this project was significantly refactored in October 2025. The old dynamic, bidirectional format conversion feature has been **completely removed** in favor of a simpler, more stable, and more explicit **static one-way proxy model**.

This document describes the current (new) architecture.

---

### Architecture Overview (New Architecture)

The core idea of the new system is "one endpoint, one format." Each API endpoint exposed to the client is hard-wired to a specific upstream API format. There is no more runtime format guessing or automatic conversion.

#### Data Flow Paths

The system's behavior is now entirely predictable:

*   **Claude Path**:
    `Client -> POST /claude -> [Anthropic Adapter] -> Upstream (anthropic_endpoint.url)`

*   **OpenAI Chat Path**:
    `Client -> POST /openai_chat -> [OpenAI Chat Adapter] -> Upstream (openai_chat_endpoint.url)`

*   **OpenAI Responses (Legacy) Path**:
    `Client -> POST /openai_responses -> [OpenAI Responses Adapter] -> Upstream (openai_responses_endpoint.url)`

This design eliminates all ambiguity. If you send a request to the `/claude` endpoint, it will always be processed as the Anthropic Messages format and forwarded accordingly.

---

### Why This Change? (Motivation)

The old dynamic conversion architecture, while powerful, introduced several difficult problems:

1.  **Compatibility Hell**: Different upstream services have strict but subtle differences in their API formats, which dynamic conversion struggled to adapt to perfectly.
2.  **Configuration Confusion**: It was difficult for users to understand what behavior the combination of `url_openai` and `url_anthropic` would produce.
3.  **Unstable Behavior**: Runtime "learning" and "fallback" mechanisms could cause the proxy's behavior to change between requests, making debugging extremely difficult.

By switching to a static proxy model, we traded **flexibility** for **explicitness**, gaining much higher **stability and reliability**.

---

### What This Means for Users

1.  **You Must Choose the Right Endpoint**: As a user, you are now responsible for knowing what API format your client application expects and pointing it to the correct proxy endpoint.
    *   If your client (like recent versions of the VSCode Claude plugin) uses the Anthropic Messages API, point it to `http://localhost:8080/claude`.
    *   If your client (like most modern AI tools) uses the OpenAI Chat Completions API, point it to `http://localhost:8080/openai_chat`.

2.  **Configuration is Greatly Simplified**: You no longer need to configure fields like `supports_responses` or `openai_preference` in `config.yaml`. Simply provide a URL and API key for `anthropic_endpoint` or `openai_chat_endpoint`.

---

### Deprecated Concepts

The following concepts and configuration items are **no longer applicable** in the current version:

*   Bidirectional format conversion
*   Runtime learning of endpoint capabilities
*   Dual URL configuration (`url_anthropic` and `url_openai`) within the `endpoints` list
*   The `supports_responses` and `openai_preference` fields

We believe that while the new architecture is simpler in function, its practicality and stability far exceed what came before.