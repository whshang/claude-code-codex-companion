# 端点测试与优化指南 | Endpoint Testing and Optimization Guide

## 中文

### 测试概览

本文档记录了端点测试方法、结果分析和配置优化建议，帮助用户选择最佳端点配置。

### 测试方法

#### 批量测试工具
```bash
# 基本测试
go run ./cmd/test_endpoints -config config.yaml

# JSON格式输出
go run ./cmd/test_endpoints -config config.yaml -json

# 测试特定端点
./test_single_endpoint.sh "端点名称"
```

#### 测试内容
1. **连通性**：HTTP状态码、响应时间
2. **认证验证**：API密钥有效性
3. **工具调用**：端点对工具调用的支持能力
4. **格式兼容**：Responses API vs Chat Completions

#### 日志分析
```bash
# 查看测试日志
tail -f /tmp/cccc_端点名称.log

# 数据库记录查询
sqlite3 logs/logs.db "SELECT * FROM request_logs ORDER BY timestamp DESC LIMIT 10"

# 格式转换日志
grep "format conversion" logs/proxy.log
```

### 测试结果分析

#### 成功案例（2025-10-14测试）

| 端点 | URL类型 | 成功率 | 响应时间 | 状态 |
|------|--------|--------|----------|------|
| **ThatAPI** | 双URL | **3/3** | 2-5秒 | ✅ 完美 |
| **kkyyxx.xyz** | 双URL | **3/3** | 快 | ✅ 完美 |
| **KAT-Coder-whshang.me** | 仅OpenAI | **2/3** | 30秒超时 | ⚠️ 可用 |

#### 失败案例

**认证失败**：
- `zenscaleai`: HTTP 403 - API密钥无效
- `88code-test1`: HTTP 401 - API密钥无效

**格式不兼容**：
- `KAT-Coder-3306`: Anthropic-only，Codex无法路由
- `KAT-Coder-3211`: Anthropic-only，Codex无法路由

### 配置优化建议

#### 优先级策略

**Tier 1: 主力端点**
```yaml
- name: ThatAPI  # 最佳性能
  priority: 1
  url_anthropic: https://api.aioec.tech
  url_openai: https://api.aioec.tech/v1
  openai_preference: chat_completions

- name: kkyyxx.xyz  # 稳定可靠
  priority: 2
  url_anthropic: https://api.kkyyxx.xyz
  url_openai: https://api.kkyyxx.xyz/v1
  openai_preference: chat_completions
```

**Tier 2: 备用端点**
```yaml
- name: KAT-Coder-whshang.me  # 备用（稍慢）
  priority: 3
  url_openai: https://whshang.me/v1
  openai_preference: chat_completions
```

#### 模型重写优化

**Anthropic-only端点通用规则**：
```yaml
model_rewrite:
  enabled: true
  rules:
  - source_pattern: gpt-*          # 支持Codex gpt-5模型
    target_model: <对应目标模型>
  - source_pattern: claude-*sonnet*
    target_model: <对应目标模型>
  - source_pattern: claude-*haiku*
    target_model: <对应目标模型>
```

### 关键发现

#### 核心问题：单URL端点不兼容Codex

**问题描述**：
- 仅配置 `url_anthropic` 的端点无法处理Codex客户端请求
- 错误信息：`no available endpoints compatible with format: openai and client: codex`

**根本原因**：
- 代码未实现自动格式转换功能
- CLAUDE.md文档声称支持但实际未实现

**影响端点**：
- KAT-Coder-3306
- KAT-Coder-3211
- anyrouter
- agentrouter（未测试）

#### 成功要素分析

**ThatAPI成功关键**：
```yaml
url_anthropic: https://api.aioec.tech
url_openai: https://api.aioec.tech/v1
openai_preference: chat_completions  # 大部分第三方不支持responses
```

**表现**：
- 平均响应时间：2-5秒
- 状态码：200 (所有请求)

### 优化实施

#### 配置更新步骤

1. **备份当前配置**：
   ```bash
   cp config.yaml config.yaml.backup.$(date +%Y%m%d_%H%M%S)
   ```

2. **应用优化配置**：
   - 更新成功端点的优先级
   - 为Anthropic-only端点添加gpt-*模型支持

3. **重启服务**：
   ```bash
   pkill -f "cccc"
   ./cccc
   ```

4. **验证效果**：
   ```bash
   # Codex测试
   codex exec "1+1等于几？"

   # 查看日志
   tail -f logs/proxy.log
   ```

#### 调试模式配置

测试阶段建议启用详细日志：
```yaml
logging:
  level: debug
  log_request_types: all
  log_request_body: full
  log_response_body: full  # 查看上游错误详情
  log_directory: ./logs

blacklist:
  enabled: true
  auto_blacklist: false  # 测试期间禁用自动拉黑
  business_error_safe: false
  config_error_safe: false
  server_error_safe: false
```

### 后续行动

1. **联系端点提供商**：
   - KAT-Coder-3306: 确认OpenAI格式兼容性
   - zenscaleai: 更新API密钥
   - 88code-test1: 更新API密钥

2. **逐一测试**：按优先级测试所有优化后的端点

3. **持续监控**：根据实际使用情况优化配置

### 文档更新

- 更新 `CLAUDE.md` 和 `AGENTS.md` 中的格式转换说明
- 同步 `CHANGELOG.md` 记录配置变更
- 维护 `docs/格式转换与端点兼容性_Format_Conversion_and_Endpoint_Compatibility.md`

## English

### Testing Overview

This document records endpoint testing methods, result analysis, and configuration optimization recommendations to help users select the best endpoint configuration.

### Testing Methods

#### Batch Testing Tool
```bash
# Basic test
go run ./cmd/test_endpoints -config config.yaml

# JSON output
go run ./cmd/test_endpoints -config config.yaml -json

# Test specific endpoint
./test_single_endpoint.sh "endpoint name"
```

#### Test Contents
1. **Connectivity**: HTTP status codes, response time
2. **Authentication**: API key validity
3. **Tool calls**: Endpoint support for tool calling
4. **Format compatibility**: Responses API vs Chat Completions

### Test Results Analysis

#### Successful Cases (2025-10-14)

| Endpoint | URL Type | Success Rate | Response Time | Status |
|----------|----------|-------------|---------------|--------|
| **ThatAPI** | Dual URL | **3/3** | 2-5 seconds | ✅ Perfect |
| **kkyyxx.xyz** | Dual URL | **3/3** | Fast | ✅ Perfect |
| **KAT-Coder-whshang.me** | OpenAI Only | **2/3** | 30s timeout | ⚠️ Workable |

#### Failed Cases

**Authentication failures**:
- `zenscaleai`: HTTP 403 - Invalid API key
- `88code-test1`: HTTP 401 - Invalid API key

**Format incompatibility**:
- `KAT-Coder-3306`: Anthropic-only, Codex cannot route
- `KAT-Coder-3211`: Anthropic-only, Codex cannot route

### Configuration Optimization

#### Priority Strategy

**Tier 1: Primary endpoints**
```yaml
- name: ThatAPI  # Best performance
  priority: 1
  url_anthropic: https://api.aioec.tech
  url_openai: https://api.aioec.tech/v1
  openai_preference: chat_completions

- name: kkyyxx.xyz  # Stable and reliable
  priority: 2
  url_anthropic: https://api.kkyyxx.xyz
  url_openai: https://api.kkyyxx.xyz/v1
  openai_preference: chat_completions
```

#### Model Rewrite Optimization

**Universal rules for Anthropic-only endpoints**:
```yaml
model_rewrite:
  enabled: true
  rules:
  - source_pattern: gpt-*          # Support for Codex gpt-5 models
    target_model: <target model>
  - source_pattern: claude-*sonnet*
    target_model: <target model>
  - source_pattern: claude-*haiku*
    target_model: <target model>
```

### Key Findings

#### Core Issue: Single-URL endpoints incompatible with Codex

**Problem description**:
- Endpoints with only `url_anthropic` cannot handle Codex client requests
- Error: `no available endpoints compatible with format: openai and client: codex`

**Root cause**:
- Code doesn't implement automatic format conversion
- CLAUDE.md claims support but not implemented

#### Success Factors Analysis

**ThatAPI success factors**:
```yaml
url_anthropic: https://api.aioec.tech
url_openai: https://api.aioec.tech/v1
openai_preference: chat_completions  # Most third-party doesn't support responses
```

**Performance**:
- Average response time: 2-5 seconds
- Status codes: 200 (all requests)

### Implementation Steps

1. **Backup current config**:
   ```bash
   cp config.yaml config.yaml.backup.$(date +%Y%m%d_%H%M%S)
   ```

2. **Apply optimized config**:
   - Update successful endpoint priorities
   - Add gpt-* model support for Anthropic-only endpoints

3. **Restart service**:
   ```bash
   pkill -f "cccc"
   ./cccc
   ```

4. **Verify results**:
   ```bash
   # Codex test
   codex exec "What is 1+1?"

   # Check logs
   tail -f logs/proxy.log
   ```

### Related Files
- Testing: `internal/web/endpoint_testing.go`
- Configuration: `docs/配置指南_Configuration_Guide.md`
- Format conversion: `docs/格式转换与端点兼容性_Format_Conversion_and_Endpoint_Compatibility.md`
- Logs: `logs/proxy.log`, `logs/logs.db`