# Claude Code and Codex Companion (CCCC)

[English Version / README_en.md](README_en.md)

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **统一的 AI 编程助手 API 转发代理**
> 服务 Claude Code、Codex 以及其他 OpenAI/Anthropic 兼容客户端，提供智能端点选择、透明代理、条件格式转换和可视化运维能力。

---

## 📖 项目简介

CCCC（Claude Code and Codex Companion）是一个面向 AI 编程助手的**高性能、智能化的 API 代理**。它为 Claude Code、Codex 以及其他 OpenAI/Anthropic 兼容的客户端提供统一的接入点。

CCCC 的核心架构是**多端点智能转换系统**。系统维护端点池,为每个端点标记其原生支持的格式,并根据客户端类型智能选择最优端点:

- 🎯 **智能端点选择**：4层排序算法(native_format → priority → health → response_time),优先选择无需转换的端点,性能提升~40%。
- 🔄 **条件格式转换**：只在必要时进行格式转换(native_format=false),避免不必要的性能损耗。
- 🏷️ **客户端过滤**：支持为端点配置client_type(claude_code/codex/openai/universal),实现精准路由。
- 🛡️ **高可用保障**：支持多端点负载均衡、健康检查、标签路由、自动降级与黑名单策略。
- 📊 **完整可观测性**：提供 Web 控制台、请求日志、统计面板与调试包导出,可视化端点配置。

| 能力 | Claude Code 原生 | Codex 原生 | CCCC |
| --- | --- | --- | --- |
| 多端点负载均衡 | ❌ | ❌ | ✅ |
| 智能格式转换 | ❌ | ❌ | ✅ (条件转换) |
| 性能优化路由 | ❌ | ❌ | ✅ (原生优先) |
| Web 管理/日志/统计 | ❌ | ❌ | ✅ |

更多技术细节可参考 `docs/` 目录。

---

## ✨ 核心特性：智能端点选择

### 端点配置字段

CCCC为每个端点提供三个核心字段来实现智能路由:

| 字段 | 类型 | 说明 | 示例值 |
| --- | --- | --- | --- |
| `client_type` | string | 客户端类型过滤 | `claude_code`, `codex`, `openai`, `""` (通用) |
| `native_format` | bool | 是否原生支持客户端格式 | `true` (无需转换), `false` (需要转换) |
| `target_format` | string | 转换目标格式 | `anthropic`, `openai_chat`, `openai_responses` |

### 智能选择流程

```
1. 客户端请求 → 识别客户端类型(claude_code/codex/openai)
2. 过滤端点池 → 根据client_type筛选候选端点
3. 4层排序:
   ├─ native_format=true优先 (性能最优,~40%提升)
   ├─ priority升序 (用户自定义优先级)
   ├─ 健康状态优先 (排除故障端点)
   └─ 响应时间升序 (选择最快端点)
4. 选中端点 → 执行请求
   ├─ native_format=true → 直接透传(fast path)
   └─ native_format=false → 格式转换后转发
```

---

## 🚀 快速开始

### 安装与启动
```bash
git clone https://github.com/whshang/claude-code-codex-companion.git
cd claude-code-codex-companion
go build -o claude-code-codex-companion .
# 或 make build

./claude-code-codex-companion -config config.yaml
```

默认监听 `127.0.0.1:8080`，控制台地址 `http://127.0.0.1:8080/admin/`。

### 一键配置脚本
- 访问 `http://127.0.0.1:8080/help` 下载跨平台脚本。
- 示例：
  ```bash
  ./cccc-setup-claude-code.sh --url http://127.0.0.1:8080 --key hello
  ./cccc-setup-codex.sh --url http://127.0.0.1:8080 --key hello
  ./cccc-setup.sh --url http://127.0.0.1:8080 --key hello
  ```
- 脚本会备份并更新 `~/.claude/settings.json`、`~/.codex/config.toml`、`auth.json`。

### 配置示例

`config.yaml` 采用多端点智能转换架构,支持为每个端点配置智能路由字段:

```yaml
server:
  host: 127.0.0.1
  port: 8081
  auto_sort_endpoints: true  # 启用动态端点排序

endpoints:
  # Claude Code专用端点 - 原生支持
  - name: anthropic-official
    url_anthropic: https://api.anthropic.com/v1/messages
    auth_type: api_key
    auth_value: YOUR_ANTHROPIC_API_KEY
    enabled: true
    priority: 1
    client_type: claude_code    # 专为Claude Code优化
    native_format: true          # 原生支持,无需转换

  # Codex专用端点 - 原生支持
  - name: openai-official
    url_openai: https://api.openai.com/v1/responses
    auth_type: api_key
    auth_value: YOUR_OPENAI_API_KEY
    enabled: true
    priority: 1
    client_type: codex          # 专为Codex优化
    native_format: true          # 原生支持,无需转换

  # 通用端点 - 需要转换
  - name: universal-provider
    url_openai: https://api.provider.com/v1/chat/completions
    auth_type: auth_token
    auth_value: YOUR_TOKEN
    enabled: true
    priority: 2
    client_type: ""              # 空字符串=通用,支持所有客户端
    native_format: false         # 需要格式转换
    target_format: openai_chat   # 转换为OpenAI Chat格式
    model_rewrite:               # 可选:模型名称重写
      enabled: true
      rules:
        - source_pattern: claude-*
          target_model: provider-claude-model

logging:
  level: info
  log_directory: ./logs
```

**配置说明**:
- `client_type`: 留空表示通用端点,支持所有客户端;指定值则只服务该类型客户端
- `native_format`: `true`表示端点原生支持客户端格式,系统优先选择这些端点以获得最佳性能
- `target_format`: 当`native_format=false`时,指定转换目标格式

完整配置示例见 `config.yaml.example`。

---

## 🌐 API 端点路径

CCCC采用**透明代理**模式，客户端使用标准API路径即可：

### Claude Code 客户端

```bash
# Claude Code自动使用 /v1/messages 路径
base_url: http://127.0.0.1:8081
```

实际请求路径：`http://127.0.0.1:8081/v1/messages`

### Codex 客户端

```bash
# Codex自动使用 /responses 路径
base_url: http://127.0.0.1:8081
```

实际请求路径：`http://127.0.0.1:8081/responses` 或 `http://127.0.0.1:8081/v1/responses`

### OpenAI 兼容客户端

```bash
# 使用标准OpenAI Chat Completions路径
base_url: http://127.0.0.1:8081
```

实际请求路径：`http://127.0.0.1:8081/chat/completions` 或 `http://127.0.0.1:8081/v1/chat/completions`

**工作原理**：
1. 系统根据请求路径自动识别客户端类型
2. SmartSelector选择最优端点（优先native_format=true）
3. 如需格式转换，自动执行并透明转发
4. 客户端无感知，开箱即用

---

## 🔌 客户端与生态
- **Claude Code**：脚本生成后立即可用；若需手动设置或企业代理，参考 [《Codex 配置指南》](docs/Codex配置指南_Codex_Configuration_Guide.md) 中的说明。
- **Codex CLI**：脚本写入 `~/.codex/config.toml`，`wire_api` 默认为 `responses`，可按项目设置 `trust_level`。
- **其他 IDE/CLI**：Cursor、Continue、Aider 等接入 OpenAI 兼容接口即可，可参考 [FoxCode 端点说明](docs/FoxCode端点说明_FoxCode_Endpoint_Notes.md) 与 [88code 端点示例](docs/88code端点示例_88code_Endpoint_Example.md)。
- **探针工具**：`go run ./cmd/test_endpoints -config config.yaml -json` 评估连通性、认证、工具调用支持。

---

## 🧭 高级主题索引
- **核心架构**: [智能端点选择](docs/动态端点排序_Dynamic_Endpoint_Sorting.md)、[端点管理](docs/端点测试与优化指南_Endpoint_Testing_and_Optimization_Guide.md)
- **代理机制**: [透明代理优化计划](docs/透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md)、[格式转换指南](docs/格式转换与端点兼容性_Format_Conversion_and_Endpoint_Compatibility.md)
- 动态端点排序：[动态端点排序](docs/动态端点排序_Dynamic_Endpoint_Sorting.md)
- 认证与参数学习：[认证方式自动学习](docs/认证方式自动学习_Auth_Method_Auto_Learning.md)
- SSE 流式转换：[SSE 重构设计](docs/SSE重构设计_SSE_Refactor_Design.md)
- GORM 与统计存储：[GORM 重构规划](docs/GORM重构规划_GORM_Refactor_Plan.md)、[统计持久化设计](docs/统计持久化设计_Statistics_Persistence_Design.md)
- 验证步骤与脚本：[功能验证步骤](docs/功能验证步骤_Verification_Steps.md)、[端点向导](docs/端点向导_Endpoint_Wizard.md)
- 格式转换与端点兼容性：[格式转换与端点兼容性](docs/格式转换与端点兼容性_Format_Conversion_and_Endpoint_Compatibility.md)
- 端点测试与优化：[端点测试与优化指南](docs/端点测试与优化指南_Endpoint_Testing_and_Optimization_Guide.md)

---

## 🤝 贡献与开发
- 设计文档集中在 `docs/`，可从 [《系统设计概览》](docs/系统设计概览_System_Design_Overview.md) 与 [《实施计划摘要》](docs/实施计划摘要_Implementation_Plan_Summary.md) 开始。
- 提交前请执行 `go test ./...`，并遵循 [《提交前检查清单》](docs/提交前检查清单_Pre_commit_Checklist.md)。
- 欢迎通过 Issue / PR 分享端点案例、脚本或翻译，提升生态。

---

## 📝 更新日志

项目采用日期分组记录，详见 [CHANGELOG.md](CHANGELOG.md)。

---

## 🙏 致谢

- **基础组件**：基于 [Gin](https://github.com/gin-gonic/gin) 构建 HTTP 服务，使用 SQLite 记录日志与统计。
- **相关项目**：CCCC fork 自 [kxn/claude-code-companion](https://github.com/kxn/claude-code-companion)，并兼容 Claude Code 与 OpenAI Codex CLI 的生态。

如果这个项目对你有帮助，请考虑点个 ⭐ 支持。Made with ❤️ for Claude Code & Codex users.
