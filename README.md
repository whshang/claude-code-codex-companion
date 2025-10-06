# Claude Code and Codex Companion (CCCC)

**统一的 AI 编程助手 API 转发代理**

[![GitHub Stars](https://img.shields.io/github/stars/whshang/claude-code-codex-companion?style=social)](https://github.com/whshang/claude-code-codex-companion)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> 🎯 为 **Claude Code** 和 **Codex** 两大顶级 AI 编程 CLI 工具提供统一的 API 转发、负载均衡和故障转移解决方案。

---

## 📖 项目简介

CCCC (Claude Code and Codex Companion) 是一个智能 AI API 代理工具，专为 [Claude Code](https://claude.ai/code) 和 [Codex](https://github.com/openai/codex-cli) 设计。通过统一的接口管理多个上游 API 端点，实现：

- 🔄 **自动格式转换**：Anthropic ↔ OpenAI 格式无缝切换
- 🎯 **智能路由**：根据客户端类型自动选择最佳端点
- 🛡️ **高可用保障**：多端点故障转移，健康检查，自动重试
- 🔧 **灵活配置**：模型重写、参数覆盖、标签路由
- 📊 **完整可观测**：Web 管理界面，详细日志，性能统计

### 为什么选择 CCCC？

| 特性 | Claude Code 原生 | Codex 原生 | CCCC |
|------|----------------|-----------|------|
| 多端点负载均衡 | ❌ | ❌ | ✅ |
| 故障自动切换 | ❌ | ❌ | ✅ |
| Anthropic/OpenAI 互转 | ❌ | ❌ | ✅ |
| 模型名称重写 | ❌ | ❌ | ✅ |
| Web 管理界面 | ❌ | ❌ | ✅ |
| 统一接入国产大模型 | ❌ | ❌ | ✅ |

---

## ✨ 核心特性

### 🔄 双客户端支持
- **Claude Code**：完整支持 Anthropic API 格式
- **Codex**：原生支持 Codex `/responses` API，自动转换为 OpenAI 格式
- **同端口服务**：一个代理同时为两个客户端服务
- **智能识别**：自动检测客户端类型和请求格式

### 🎯 智能路由系统
- **双URL智能路由**：一个端点同时配置 `url_anthropic` 和 `url_openai`，Claude Code请求自动路由到Anthropic URL，Codex请求自动路由到OpenAI URL
- **单URL容错转换**：仅配置一个URL时，自动在代理侧完成格式转换（缺少`url_openai`时将Codex请求转为Anthropic格式；缺少`url_anthropic`时将Claude Code请求转为OpenAI格式）
- **OpenAI格式自适应**：优先尝试 `/responses` 格式（Codex新API），失败后自动切换 `/chat/completions` 格式，学习结果记录到端点配置的 `openai_preference` 字段
- **优先级选择**：按配置优先级自动选择端点
- **客户端过滤**：端点级别的客户端类型白名单
- **标签路由**：基于请求特征的动态路由
- **健康检查**：实时监控端点状态，自动隔离故障节点
- **智能拉黑**：区分业务错误、配置错误、服务器错误，可配置拉黑策略
- **全局拉黑控制**：系统设置中可全局启用/禁用拉黑功能
- **错误类型识别**：自动识别业务错误（API返回错误）、配置错误（客户端问题）、服务器错误（基础设施问题）

### 🧠 智能参数学习系统（NEW!）
- **自动学习不支持的参数**：从 400 错误自动识别端点不支持的参数（如 `tools`、`tool_choice`）
- **实时自动重试**：学习后立即移除不支持参数并重试，避免端点被黑名单
- **零配置运行**：无需手动配置参数白名单，系统自动适配各端点差异
- **持久化学习**：学习结果在端点生命周期内保持，避免重复试错

### 🔧 高级配置能力
- **显式模型重写**：`gpt-5` → `qwen3-coder`，`claude-sonnet-4-20250514` → `kimi-k2`，支持正则表达式匹配
- **隐式模型重写**：通用端点根据客户端类型自动应用默认模型（Claude Code→`claude-sonnet-4-20250514`，Codex→`gpt-5`）
- **参数覆盖**：动态修改 temperature、max_tokens 等
- **格式转换**：Anthropic ↔ OpenAI 自动转换，支持单URL配置时的跨格式兼容
- **工具调用**：完整支持 function calling 和 tools
- **OpenAI格式自适应**：优先尝试 `/responses` 格式（https://platform.openai.com/docs/guides/responses），失败后自动降级到 `/chat/completions`（https://platform.openai.com/docs/guides/chat）并记录偏好
- **格式偏好配置**：可设置端点优先使用 responses、chat_completions 或自动检测，学习结果可回写端点配置

### 🧪 端点自适应能力
- **自动接口类型检测**：使用重写后的模型对端点进行自检，确认 Anthropic/OpenAI API 兼容性
- **认证方式探测**：在 `auth_type` 留空或使用 `auto` 时自动尝试 `Authorization` 与 `x-api-key`，并记忆成功方案
- **首轮测试向导**：一次性触发所有端点的连通性测试，生成响应时间、HTTP 状态和格式验证报告
- **配置回写机制**：测试学习到的 `openai_preference`、认证方式等信息可持久化回端点配置（非必填项自动补齐）

#### 批量测试 CLI

```bash
# 读取指定配置文件批量验证所有端点（默认 ./config.yaml）
go run ./cmd/test_endpoints -config config.yaml -json
```

- 同时模拟 **Claude Code (Anthropic 格式)** 与 **Codex (OpenAI Responses/Chat 格式)**，自动补齐认证头与格式；
- 失败结果会区分账号/额度（429）、认证（401）、格式（400）等类型，并记录到运行时配置；
- 可配合全局拉黑开关关闭自动拉黑，以安全执行保守测试。

### 📊 企业级可观测性
- **Web 管理界面**：实时查看端点状态、请求日志
- **详细日志**：请求/响应完整追踪，包含参数学习过程
- **性能统计**：成功率、响应时间、流量分析
- **调试导出**：一键导出请求详情

---

## 🚀 快速开始

### 安装方式

#### 方式一：下载预编译版本（推荐新手）

从 [Releases](https://github.com/whshang/claude-code-codex-companion/releases) 下载对应系统的版本：

```bash
# macOS (Apple Silicon)
wget https://github.com/whshang/claude-code-codex-companion/releases/latest/download/cccc-darwin-arm64.tar.gz
tar -xzf cccc-darwin-arm64.tar.gz

# macOS (Intel)
wget https://github.com/whshang/claude-code-codex-companion/releases/latest/download/cccc-darwin-amd64.tar.gz

# Linux (x64)
wget https://github.com/whshang/claude-code-codex-companion/releases/latest/download/cccc-linux-amd64.tar.gz

# Windows (x64)
# 下载 cccc-windows-amd64.zip 并解压
```

#### 方式二：从源码编译

```bash
# 克隆仓库
git clone https://github.com/whshang/claude-code-codex-companion.git
cd claude-code-codex-companion

# 安装依赖
go mod download

# 编译
go build -o cccc

# 或使用 Makefile
make build
```

### 初次运行

```bash
# 1. 启动服务（首次运行会生成配置文件）
./cccc -config config.yaml -port 8080

# 2. 打开 Web 管理界面
# 浏览器访问: http://localhost:8080
```

### 配置端点

#### 通过 Web 界面（推荐）

1. 访问 http://localhost:8080
2. 进入"端点管理"
3. 点击"新增端点"，填写：
   - **名称**：端点标识（如 `openai-primary`）
   - **Anthropic URL**：用于 Claude Code 的 API 地址（可选）
   - **OpenAI URL**：用于 Codex 的 API 地址（可选）
   - **认证**：API Key、Bearer Token 或选择自动探测
   - **支持的客户端**：`claude-code`、`codex` 或留空（支持所有）

#### 通过配置文件

编辑 `config.yaml`：

```yaml
server:
    host: 127.0.0.1
    port: 8080

endpoints:
    # Claude Code 端点
    - name: anthropic-official
      url_anthropic: https://api.anthropic.com
      auth_type: api_key
      auth_value: sk-ant-xxxxx
      enabled: true
      priority: 1

    # Codex 端点（OpenAI）
    - name: openai-official
      url_openai: https://api.openai.com
      auth_type: auth_token
      auth_value: sk-openai-xxxxx
      enabled: true
      priority: 1
      model_rewrite:
        enabled: true
        rules:
            - source_pattern: gpt-5*
              target_model: gpt-4-turbo

    # 通用端点（同时支持两者）- 双URL配置示例
    - name: universal-api
      url_anthropic: https://api.your-provider.com/anthropic  # Claude Code 请求路由到此
      url_openai: https://api.your-provider.com/openai        # Codex 请求路由到此
      endpoint_type: openai
      auth_type: auth_token
      auth_value: your-token
      enabled: true
      priority: 2
      openai_preference: auto  # OpenAI格式偏好：auto/responses/chat_completions
      # 系统自动检测客户端，无需配置 supported_clients
      model_rewrite:
        enabled: true
        rules:
            - source_pattern: claude-*
              target_model: qwen3-coder
            - source_pattern: gpt-*
              target_model: qwen3-coder

logging:
    level: info
    log_directory: ./logs

# 黑名单配置（新增功能）
blacklist:
    enabled: true              # 全局启用黑名单功能
    auto_blacklist: true        # 自动拉黑失败的端点
    business_error_safe: true   # 业务错误（API返回错误）不触发拉黑
    config_error_safe: false    # 配置错误（客户端问题）触发拉黑
    server_error_safe: false    # 服务器错误（基础设施问题）触发拉黑
```

---

## 🔌 客户端配置

### Claude Code 配置

#### 方式一：使用自动脚本（推荐）

访问 http://localhost:8080/help ，下载对应系统的脚本：

- **Windows**: `ccc.bat`
- **macOS**: `ccc.command`
- **Linux**: `ccc.sh`

脚本会自动配置所有必需的环境变量和设置文件。

#### 方式二：手动配置

**Linux/macOS:**
```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_AUTH_TOKEN="hello"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"
export API_TIMEOUT_MS="600000"

claude interactive
```

**Windows (PowerShell):**
```powershell
$env:ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
$env:ANTHROPIC_AUTH_TOKEN="hello"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"
$env:API_TIMEOUT_MS="600000"

claude interactive
```

#### 方式三：修改 settings.json

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

### Codex 配置

#### 方式一：使用自动脚本（推荐）

访问 http://localhost:8080/help?client=codex ，下载对应系统的脚本。

#### 方式二：手动配置文件

编辑 `~/.codex/config.toml`：

```toml
model_provider = "cccc"
model = "gpt-5"

[model_providers.cccc]
name = "cccc"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
requires_openai_auth = true

[projects."/path/to/your/project"]
trust_level = "trusted"
```

**说明**：
- `model_provider`: 使用自定义提供商名称（如 cccc）
- `base_url`: 代理服务器地址
- `wire_api`: 使用 responses API（Codex 原生格式）
- `requires_openai_auth`: 启用认证（API Key 可以是任意值如 "hello"）
- `projects`: 配置项目信任级别

---

## 📚 高级配置

### 模型重写规则

将不支持的模型自动映射到实际可用的模型：

```yaml
endpoints:
  - name: qwen-api
    url_openai: https://api.qwen.com
    model_rewrite:
      enabled: true
      rules:
          # Codex 的 gpt-5 映射到通义千问
          - source_pattern: gpt-5*
            target_model: qwen-turbo
          # Claude Code 的 claude-sonnet 映射到通义千问
          - source_pattern: claude-*sonnet*
            target_model: qwen-plus
          # 通配符支持
          - source_pattern: gpt-4*
            target_model: qwen-max
```

### 标签路由

根据请求特征路由到不同端点：

```yaml
tagging:
    enabled: true
    taggers:
        - name: path-router
          type: builtin
          config:
              rules:
                  - pattern: "^/v1/chat/completions"
                    tag: "openai-compatible"
                  - pattern: "^/responses"
                    tag: "codex-api"

endpoints:
    - name: openai-endpoint
      tags: ["openai-compatible"]
      # 只处理 OpenAI 格式请求
    
    - name: codex-endpoint
      tags: ["codex-api"]
      # 只处理 Codex 请求
```

### 参数覆盖

动态修改请求参数：

```yaml
endpoints:
    - name: custom-endpoint
      parameter_overrides:
          - key: temperature
            value: 0.7
          - key: max_tokens
            value: 4096
          - key: top_p
            value: 0.9
```

### 双URL配置（新增功能）

一个端点同时支持两种API格式，根据请求类型智能路由：

```yaml
endpoints:
    - name: dual-endpoint
      # Claude Code 请求自动路由到 Anthropic URL
      url_anthropic: https://api.provider.com/v1/anthropic
      # Codex 请求自动路由到 OpenAI URL  
      url_openai: https://api.provider.com/v1/openai
      endpoint_type: openai
      auth_type: auth_token
      auth_value: your-token
      enabled: true
      priority: 1
      # OpenAI格式偏好设置
      openai_preference: auto  # 可选值：auto/responses/chat_completions
```

### 黑名单配置（新增功能）

精细化控制端点拉黑策略：

```yaml
# 全局黑名单配置
blacklist:
    enabled: true              # 是否启用黑名单功能
    auto_blacklist: true        # 是否自动拉黑失败端点
    business_error_safe: true   # 业务错误（如API返回错误信息）是否安全（不触发拉黑）
    config_error_safe: false    # 配置错误（如认证失败、格式错误）是否安全
    server_error_safe: false    # 服务器错误（如5xx、网络问题）是否安全
```

**错误类型说明**：
- **业务错误**：API正常返回错误信息（如模型不支持、参数错误），通常不应拉黑端点
- **配置错误**：客户端配置问题（如认证失败、请求格式错误），应该拉黑端点
- **服务器错误**：基础设施问题（如5xx错误、网络超时），应该拉黑端点

---

## 📊 监控与调试

### Web 管理界面

访问 http://localhost:8080 查看：

- **仪表板**：端点状态、请求统计、性能指标
- **端点管理**：实时配置端点，拖拽调整优先级
- **请求日志**：查看所有请求详情，支持过滤和搜索
- **系统设置**：日志级别、超时配置、验证规则

### 日志查看

```bash
# 实时日志
tail -f logs/proxy.log

# 查看错误
grep -i error logs/proxy.log

# 查看特定客户端
grep "codex" logs/proxy.log
grep "claude-code" logs/proxy.log
```

### 调试导出

在 Web 界面的"请求日志"中，点击任何请求的"导出"按钮，会生成包含完整请求/响应详情的调试包到 `debug/` 目录。

---

## 🔍 常见问题

<details>
<summary><strong>Q: 为什么 Codex 调用一直失败？</strong></summary>

**A:** 检查以下几点：
1. 端点配置了 `supported_clients: [codex]`
2. 端点类型为 `endpoint_type: openai`
3. 模型重写规则正确（如 `gpt-5*` → 实际支持的模型）
4. 查看日志：`grep "codex" logs/proxy.log`

详见 [CHANGELOG.md](./CHANGELOG.md) 的 "Known Issues" 部分。
</details>

<details>
<summary><strong>Q: 如何同时使用多个号池？</strong></summary>

**A:** 
1. 在"端点管理"中添加所有号池端点
2. 设置不同的优先级（数字越小优先级越高）
3. 启用健康检查，代理会自动切换到可用的端点
</details>

<details>
<summary><strong>Q: 支持哪些国产大模型？</strong></summary>

**A:** 只要提供 OpenAI 兼容接口的都支持：
- 通义千问 (Qwen)
- 智谱 GLM
- 月之暗面 Kimi
- 百川 Baichuan
- 豆包 (Doubao)
- 以及任何 OpenRouter 支持的模型

配置时选择 `endpoint_type: openai` 并设置好模型重写规则即可。
</details>

<details>
<summary><strong>Q: 端点被黑名单了怎么办？</strong></summary>

**A:**
1. 查看日志找出失败原因
2. 在 Web 界面"端点管理"中点击"重置"按钮
3. 或重启代理服务自动清除黑名单
4. 调整 `recovery_threshold` 参数控制恢复策略
</details>

---

## 🤝 致谢与贡献

### 致敬原项目

CCCC 是从 [@kxn](https://github.com/kxn) 的 [claude-code-companion](https://github.com/kxn/claude-code-companion) 项目 fork 而来。感谢原作者创建了这个优秀的 Claude Code 代理工具！

**相比原项目的主要改进**：
- ✅ 新增完整的 Codex 客户端支持
- ✅ 实现 Codex `/responses` 格式自动转换
- ✅ 客户端类型自动检测和智能路由
- ✅ 智能参数学习系统（自动适配端点差异）
- ✅ 自动重试机制（避免端点误判）
- ✅ 增强的模型重写功能（支持隐式重写）
- ✅ 工具调用完整支持（tools 字段保留）
- ✅ 改进的响应验证和 SSE 处理
- ✅ 国际化支持（9种语言）
- ✅ 更详细的文档和配置示例

### 如何贡献

欢迎贡献代码、报告问题或提出建议！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 贡献指南

- 遵循 Go 官方代码风格
- 添加必要的注释和文档
- 编写单元测试
- 确保 `go test ./...` 通过
- 更新 CHANGELOG.md

---

## 📝 更新日志

详细的版本历史和变更记录请查看 [CHANGELOG.md](./CHANGELOG.md)。

**最新版本亮点**：
- 🧠 智能参数学习系统（自动识别并移除不支持参数）
- 🔄 自动重试机制（学习后立即重试，避免端点被黑名单）
- 🎉 完整的 Codex 客户端支持
- 🌍 国际化支持（9种语言：中文、英文、日语等）
- 🔄 Anthropic ↔ OpenAI 格式自动转换
- 🎯 客户端特定端点路由
- 🛠️ 增强的模型重写和工具调用支持
- 🔗 **双URL配置**：一个端点同时配置Anthropic和OpenAI URL，智能路由
- 🎛️ **OpenAI格式自适应**：智能尝试/responses格式，失败后自动切换/chat/completions
- 🚫 **智能黑名单策略**：区分业务/配置/服务器错误，可配置拉黑规则
- ⚙️ **全局黑名单控制**：系统设置中可全局启用/禁用拉黑功能
- 🤖 **隐式模型重写**：通用端点根据客户端类型自动应用默认模型映射

---

## 📄 许可证

本项目基于 MIT License 开源 - 详见 [LICENSE](./LICENSE) 文件。

---

## 📮 联系方式

- **问题反馈**：[GitHub Issues](https://github.com/whshang/claude-code-codex-companion/issues)
- **功能建议**：[GitHub Discussions](https://github.com/whshang/claude-code-codex-companion/discussions)
- **原项目**：[kxn/claude-code-companion](https://github.com/kxn/claude-code-companion)

---

## ⭐ 项目状态

![GitHub last commit](https://img.shields.io/github/last-commit/whshang/claude-code-codex-companion)
![GitHub issues](https://img.shields.io/github/issues/whshang/claude-code-codex-companion)
![GitHub pull requests](https://img.shields.io/github/issues-pr/whshang/claude-code-codex-companion)

---

<div align="center">

**如果这个项目对你有帮助，请给个 ⭐️ Star 支持一下！**

Made with ❤️ for Claude Code and Codex users

</div>
