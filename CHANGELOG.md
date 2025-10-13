# Changelog

## 2025-10-13
### Fixed
- **关键修复**：修复 Codex 非流式响应转 SSE 时无内容的问题
  - 修复 `convertResponseJSONToSSE` 函数，正确从 Responses API 格式（`output[].content[].text`）提取内容
  - 添加对 Responses JSON 结构的优先解析，同时保持对 Chat Completions 格式的向后兼容
  - 修复 SSE 流立即写入并返回的逻辑，避免后续 JSON 写入覆盖 SSE 流
  - 完整实现 Chat Completions JSON → Responses JSON → Responses SSE 的三阶段转换
  - 解决了 Codex 客户端在 `stream: true` 但上游返回非流式 JSON 时的 "无回复" 问题
  - 详见日志："Converting non-streaming JSON to SSE for Codex (client expects stream)"

## 2025-10-10
### Fixed
- **关键修复**：修复 Claude Code 发送 tool_result 时 tool_use_id 丢失的问题
  - 在 `GetContentBlocks()` 方法中递归处理嵌套 content 数组时，现在正确提取 `tool_use_id` 和 `is_error` 字段
  - 解决了 "user.tool_result is missing tool_use_id" 错误
  - 添加了完整的测试套件验证修复效果
- **关键修复**：修复 Codex 流式响应 "stream closed before response.completed" 错误
  - 完整实现 Chat Completions SSE → Responses API SSE 格式转换
  - 将 `delta.content` 转换为 `response.content.delta` 事件
  - 透传 `tool_calls` 保持 Codex 工具调用兼容性
  - 始终发送 `response.done` 事件确保流正确结束
  - 解决了 Codex 响应被截断、无法完成的问题
- 修复 `internal/i18n/translator.go` 缺少 `fmt` 导入的编译错误

## 2025-10-08
### Added
- 扩展工具调用增强：改进提示注入与结果解析，补充相关文档说明。

## 2025-10-07
### Added
- 智能端点自适应系统：自动识别 URL、格式与模型重写策略，并强化黑名单逻辑。
### Fixed
- 修复端点测试与路径转换问题，优化响应时间显示。

## 2025-10-06
### Added
- 完善端点测试工具，支持双格式自检与更多日志指标。
### Fixed
- 修正 `make dev` 开发脚本在热重载场景下的兼容性问题。

## 2025-10-03
### Added
- 智能参数学习与自动重试机制：从 400 错误学习不兼容参数并即时重试。
### Changed
- 更新 README / CHANGELOG 并整合项目说明，移除已过时的 `CLAUDE.md`。
### Fixed
- 修正 README 中 Codex 配置示例。

## 2025-09-30
### Changed
- 补充 README，新增从源码编译及生成可执行文件的说明。

## 2025-09-11
### Added
- 允许针对 iflow 域名的 SSE 流缺失 `[DONE]` 标记时继续处理。

## 2025-09-01
### Changed
- 审查代码库中重复与无用的实现，保持主干精简。

## 2025-08-31
### Added
- 多语言翻译与文档首轮补充，为后续功能扩展打下基础。
