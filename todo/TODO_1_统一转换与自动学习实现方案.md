# OpenAI 内部格式自适应与分流（简化方案，TODO）

背景与决策：由于 Codex 相关问题长期难以稳定，现阶段放弃 Anthropic ↔ OpenAI 之间的跨家族转换，仅支持 OpenAI 家族内部的 Chat Completions（/v1/chat/completions）与 Responses（/v1/responses）双向适配与自动学习；根据配置中 url_openai 进行快速分流与模型重写。

参考规范：
- docs/OpenAI 格式（Chat Completions） - New API.md
- docs/OpenAI 格式（Responses） - New API.md

---

## 范围与原则（OpenAI-only）

- 仅支持 OpenAI 家族：Chat Completions 与 Responses 的请求/响应（JSON 与 SSE）。
- 不再进行 Anthropic ↔ OpenAI 的转换；若端点仅有 url_anthropic，则视为不支持，快速失败并提示配置 url_openai。
- 分流优先级：按客户端请求格式与端点 openai_preference 选择 Responses 优先、失败降级 Chat。
- 工具调用安全：严格区分文本与工具事件，arguments 不作为文本输出。
- 自动学习：openai_preference / supports_responses / DetectedAuthHeader / NativeToolSupport / CountTokensEnabled 覆盖“端点测试 + 真实请求”，并持久化到 config.yaml。

---

## 核心改造项

### 1) 端点分流与 OpenAI 格式自适应

- 端点选择：
  - 仅考虑配置了 url_openai 的端点；过滤掉仅有 url_anthropic 的端点。
  - 未找到可用 url_openai 时快速失败（400/422），并在错误中提示“请配置 url_openai 或启用 OpenAI 端点”。
- 自适应策略（proxy/proxy_logic.go）：
  - 首次尝试 /responses；4xx（排除 401/403）或网络异常时自动降级到 /chat/completions 并立即重试。
  - 成功后写回 ep.openai_preference 与 supports_responses（responses 成功即 true），通过 Server.PersistEndpointLearning 持久化。
- 管理批量测试仅覆盖 OpenAI 两种格式；移除 Anthropic 测试项。

### 2) 非流式（JSON）转换（仅 OpenAI）

- Chat ↔ Responses 请求/响应双向转换：
  - usage 对齐：prompt/completion ↔ input/output。
  - finish_reason 映射：stop/length/tool_calls。
  - tools/tool_choice 映射：auto/none/required ↔ auto/any/tool。
- 文件建议：统一收敛到现有 openai_* 与 responses_* 文件（保留/合并）：
  - internal/conversion/openai_responses_format_adapter.go
  - internal/conversion/responses_conversion.go
  - internal/conversion/openai_chunk_aggregator.go

### 3) 流式（SSE）转换（仅 OpenAI）

- Chat → Responses：
  - delta.content → response.output_text.delta
  - delta.tool_calls（首块含 id/name）→ response.function_call.started；后续 → response.function_call_arguments.delta
  - 结束补发 response.completed（若上游缺失），或映射 finish_reason/[DONE]
- Responses → Chat：
  - response.output_text.delta → delta.content
  - response.function_call.* → delta.tool_calls[…]
  - 结束发空增量 + finish_reason 或 [DONE]
- 完整性校验仅针对 OpenAI：满足 finish_reason | response.completed | [DONE] 任一即可。

### 4) 工具调用安全与防注入（保留）

- 聚合阶段区分 text 与 tool_call，arguments 增量按 index/ID 累积。
- 输出阶段：
  - Responses：function_call.started / arguments.delta / completed
  - Chat：delta.tool_calls（arguments 多块增量）

### 5) 自动学习与回写（仅 OpenAI）

- /responses 成功：supports_responses=true；失败且 chat 成功：openai_preference=chat_completions。
- 认证 auto 探测：记录 ep.DetectedAuthHeader（x-api-key / Authorization），必要时回填 ep.AuthType。
- 学习到的 NativeToolSupport/CountTokensEnabled 同步持久化。

### 6) 验证器与容错

- SmartDetectContentType：自动设置 JSON/SSE。
- conversion_adapter_mode 新增 openai_only：
  - openai_only：仅启用 Chat↔Responses 转换与路由，禁用所有 Anthropic 相关路径。
  - auto（默认）：若启用 openai_only，则优先生效；否则沿用现有策略。
  - legacy：保留旧实现回退。

### 7) 观测与调试

- 日志字段：
  - conversionStages：request:responses->chat_completions / request:chat_completions->responses / streaming:... 等
  - supports_responses_flag、openai_preference 变化、学习触发原因（first_4xx / network_error / admin_test）
- 管理后台：端点测试与日志均移除/隐藏 Anthropic 相关入口与统计。

---

## 测试计划（OpenAI-only）

- 单测（internal/conversion）：
  - Chat↔Responses（SSE/JSON）：文本与 tool_calls 双覆盖；必须满足终止完整性规则。
  - 非流式互转：usage/finish_reason/模型名对齐。
- 端到端（cmd/test_endpoints）：
  - 验证 Responses 优先与失败降级策略；学习回写（openai_preference/supports_responses/DetectedAuthHeader）。

---

## 验收标准

- 仅使用 OpenAI 端点即可完成所有 Codex 请求；对仅有 url_anthropic 的端点快速失败并给出明确提示。
- /responses 优先、失败自动降级 /chat/completions，学习结果在真实请求与管理测试中均可持久化并生效。
- 工具调用在两种格式下均不发生“参数作为文本”注入问题；流式终止信号完整（finish_reason | response.completed | [DONE]）。

---

## 里程碑与落地步骤

- D1：引入 conversion_adapter_mode=openai_only 开关；端点选择仅使用 url_openai；无 url_openai 快速失败。
- D2：打通 Responses 优先与 Chat 降级路径；落地学习与持久化；更新管理端“测试端点”为 OpenAI-only。
- D3：完善 SSE 映射与完整性校验；补齐单测与端到端测试；灰度启用 openai_only，观测后全量。

---

## 与当前未提交改动的对齐建议

- 保留/复用（OpenAI-only 直接受益）：
  - internal/conversion/openai_responses_format_adapter.go
  - internal/conversion/openai_responses_types.go
  - internal/conversion/responses_conversion.go（及相关测试 responses_conversion_test.go）
  - internal/conversion/openai_chunk_aggregator.go / stream_helpers.go
  - internal/conversion/conversion_manager.go / adapter_factory.go
- 标记弃用/禁用（在 openai_only 模式下不走）：
  - internal/conversion/anthropic_to_openai_response.go
  - internal/conversion/openai_to_anthropic_request.go
  - 相关 Anthropic 适配与测试路径
- 端点/验证：
  - internal/endpoint/selector.go：仅选择含 url_openai 的端点；缺失则报错。
  - internal/config/validation.go：在 openai_only 下，url_openai 缺失时给出明确校验提示。

---

## 风险与回退

- 风险：部分供应商对 Responses/Chat 的实现差异导致解析容错需求；通过宽松解析与补发终止事件化解。
- 回退：切换 conversion_adapter_mode=legacy 或关闭 openai_only；必要时强制 openai_preference=chat_completions。
