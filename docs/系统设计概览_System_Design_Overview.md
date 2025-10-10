# 系统设计概览 | System Design Overview

## 中文

CCCC 是多协议 AI 代理，核心能力包括端点管理、格式转换、模型重写、健康检查与日志分析。本篇作为导航，便于快速定位实现位置。

### 核心组件

- **Endpoint Manager** (`internal/endpoint`)：维护优先级、标签、健康状态，并记录 `supports_responses`、`openai_preference` 等学习结果。  
- **Format Conversion** (`internal/conversion`)：实现 Anthropic ↔ OpenAI 请求/响应转换及 SSE 流式转换。  
- **Proxy Logic** (`internal/proxy`)：负责请求入口、降级状态机、工具增强、日志写入与配置持久化。  
- **Model Rewrite** (`internal/modelrewrite`)：通配符映射与响应回写。  
- **Tagging & Routing** (`internal/tagging`)：结合内置与 Starlark Tagger 进行路由。  
- **Health & Statistics** (`internal/health`, `internal/statistics`)：执行周期性探测与指标聚合。  

### 配置与客户端

- [《配置指南》](配置指南_Configuration_Guide.md)：`config.yaml` 字段说明。  
- [《Codex 配置指南》](Codex配置指南_Codex_Configuration_Guide.md)：客户端脚本示例。  
- [《模型重写设计》](模型重写设计_Model_Rewrite_Design.md)。  

### 可观测性

- [《调试信息导出》](调试信息导出_Debug_Export.md)：导出调试包。  
- [《功能验证步骤》](功能验证步骤_Verification_Steps.md)：人工验证清单。  

### 开发参考

- [《实施计划摘要》](实施计划摘要_Implementation_Plan_Summary.md)、[《透明代理优化计划》](透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md)。  
- 提交前请查阅 [《提交前检查清单》](提交前检查清单_Pre_commit_Checklist.md)。  

## English

CCCC is a multi-protocol AI proxy offering endpoint management, format conversion, model rewriting, health checks, and logging. This page guides you to the relevant modules.

### Core Components

- **Endpoint Manager** (`internal/endpoint`): manages priorities, tags, health, and learned values (`supports_responses`, `openai_preference`).  
- **Format Conversion** (`internal/conversion`): handles Anthropic ↔ OpenAI conversions and SSE streaming.  
- **Proxy Logic** (`internal/proxy`): entry point, downgrade state machine, tool enhancement, logging, and persistence.  
- **Model Rewrite** (`internal/modelrewrite`): wildcard mapping and response rewriting.  
- **Tagging & Routing** (`internal/tagging`): built-in and Starlark taggers for routing decisions.  
- **Health & Statistics** (`internal/health`, `internal/statistics`): periodic probes and metric aggregation.  

### Configuration & Clients

- [Configuration Guide](配置指南_Configuration_Guide.md).  
- [Codex Configuration Guide](Codex配置指南_Codex_Configuration_Guide.md).  
- [Model Rewrite Design](模型重写设计_Model_Rewrite_Design.md).  

### Observability

- [Debug Export](调试信息导出_Debug_Export.md).  
- [Verification Steps](功能验证步骤_Verification_Steps.md).  

### Development Notes

- [Implementation Plan Summary](实施计划摘要_Implementation_Plan_Summary.md), [Transparent Proxy Optimisation Plan](透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md).  
- Review the [Pre-commit Checklist](提交前检查清单_Pre_commit_Checklist.md) before committing.  
