# Changelog

## 2025-10-16
### Added
- **配置系统全面增强**：新增五大配置模块提升系统可配置性和性能
  - `streaming` 配置：流式响应超时、重试次数、最小数据包大小、SSE 验证和缓存
  - `tools` 配置：工具调用超时、最大并行数、验证和缓存选项
  - `http_client` 配置：连接池大小、缓冲区配置、HTTP/2 支持、压缩和长连接
  - `monitoring` 配置：指标收集间隔、慢请求阈值、详细指标和请求追踪
  - `format_detection` 配置：缓存优化、LRU 支持、路径缓存和请求体结构检测
- **动态端点排序**：`server.auto_sort_endpoints` 选项，根据可用性和响应速度动态调整端点优先级
- **Web 界面优化**：设置页面从两栏布局优化为四栏布局，显著提高空间利用率
- **完整国际化支持**：所有新增配置项完整支持中英文切换，包括：
  - 端口、自动排序端点、流式超时、流式缓存等关键配置项
  - 所有帮助文本和占位符的英文翻译
  - 确保英文界面无中文遗漏

### Changed
- **配置文件结构**：在 `internal/config/types.go` 中新增配置结构体
  - `StreamingConfig`：流式转换配置
  - `ToolsConfig`：工具调用配置
  - `HTTPClientConfig`：HTTP 客户端配置
  - `MonitoringConfig`：性能监控配置
  - `FormatDetectionConfig`：格式检测配置
- **默认配置优化**：所有新配置模块设置合理的生产环境默认值
- **HTTP 客户端工厂**：在 `internal/common/httpclient/factory.go` 中集成新配置系统
- **格式检测增强**：在 `internal/utils/format_detector.go` 中实现缓存优化和结构检测
- **日志模型优化**：在 `internal/logger/gorm_models.go` 中增强请求记录字段

### Improved
- **设置页面 UI**：采用响应式四栏布局（col-md-3），更好地利用屏幕空间
- **配置项分组**：按功能模块分组显示，提高配置可读性和管理效率
- **表单体验**：优化输入框、开关和下拉菜单的布局和对齐方式

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
