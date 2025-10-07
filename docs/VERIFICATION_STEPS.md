# CCCC 功能验证步骤

本文档提供手动验证所有修改功能的详细步骤。

## 🚀 准备工作

### 1. 启动服务
```bash
# 使用测试配置启动服务
./cccc -config test_verification.yaml -port 8080

# 或使用现有配置
./cccc -config config.yaml -port 8080
```

### 2. 验证服务运行
```bash
curl http://localhost:8080/health
```

## 🔍 功能验证

### 1. 双URL配置路由验证

#### 1.1 验证Claude Code请求路由到Anthropic URL
```bash
# 发送Claude Code格式的请求
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: hello" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 100,
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'
```

**验证点:**
- 查看日志 `logs/proxy.log`，确认请求路由到配置了 `url_anthropic` 的端点
- 检查日志中的 `GetFullURLWithFormat` 调用，使用了anthropic格式

#### 1.2 验证Codex请求路由到OpenAI URL
```bash
# 发送Codex格式的请求
curl -X POST http://localhost:8080/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer hello" \
  -d '{
    "model": "gpt-5",
    "instructions": "You are helpful",
    "input": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "Hello"
          }
        ]
      }
    ]
  }'
```

**验证点:**
- 查看日志，确认请求路由到配置了 `url_openai` 的端点
- 检查URL选择逻辑正确

### 2. OpenAI格式自适应验证

#### 2.1 验证/responses格式自适应
```bash
# 向只配置OpenAI URL的端点发送/responses请求
curl -X POST http://localhost:8080/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer hello" \
  -d '{
    "model": "gpt-5",
    "instructions": "Test",
    "input": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
  }'
```

**验证点:**
- 查看日志中的 `openai_preference` 和 `NativeCodexFormat` 相关记录
- 如果端点不支持/responses格式，应该自动转换为/chat/completions重试
- 检查日志中的 `auto-converted to OpenAI format` 消息

#### 2.2 验证不同偏好设置
编辑配置文件，测试不同的 `openai_preference` 值：

```yaml
openai_preference: auto          # 自动探测
openai_preference: responses     # 优先使用responses
openai_preference: chat_completions  # 直接使用chat_completions
```

**验证点:**
- 每种设置都应该有不同的行为，体现在日志中

### 3. 模型重写验证

#### 3.1 验证显式模型重写
```bash
# 请求一个需要重写的模型
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: hello" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**验证点:**
- 查看日志中的 `Model rewritten in request` 消息
- 确认原始模型被正确重写为目标模型

#### 3.2 验证隐式模型重写（通用端点）
向没有配置显式重写规则的通用端点发送请求：

```bash
# 使用非标准模型名
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: hello" \
  -d '{
    "model": "unknown-model",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**验证点:**
- Claude Code客户端: 非Claude模型应重写为 `claude-sonnet-4-20250514`
- Codex客户端: 非GPT模型应重写为 `gpt-5`
- 检查日志中的隐式重写逻辑

### 4. 端点拉黑逻辑验证

#### 4.1 验证错误分类
查看配置中的黑名单设置：
```yaml
blacklist:
    enabled: true
    business_error_safe: true   # 业务错误不拉黑
    config_error_safe: false    # 配置错误拉黑
    server_error_safe: false    # 服务器错误拉黑
```

#### 4.2 测试服务器错误拉黑
向一个会返回5xx错误的端点发送请求，观察：
- 端点是否被正确拉黑
- 日志中的错误分类是否正确
- 后续请求是否跳过被拉黑的端点

#### 4.3 测试业务错误不拉黑
向一个返回业务错误（如模型不支持）但端点正常的端点发送请求，观察：
- 端点是否不被拉黑
- 日志中是否记录为 `business error`

#### 4.4 验证全局拉黑开关
1. 访问 Web 界面: http://localhost:8080
2. 进入"系统设置"
3. 找到"黑名单配置"
4. 关闭"启用端点拉黑功能"
5. 再次发送错误请求，确认端点不被拉黑

## 📊 日志分析

### 关键日志关键词

在 `logs/proxy.log` 中搜索以下关键词：

#### 双URL路由
- `GetFullURLWithFormat`
- `requestFormat`
- `actual_endpoint_format`

#### 格式自适应
- `openai_preference`
- `NativeCodexFormat`
- `auto-converted to OpenAI format`
- `Trying native /responses format`

#### 模型重写
- `Model rewritten in request`
- `Applying implicit model rewrite rule`
- `original_model` / `new_model`

#### 端点拉黑
- `shouldSkipHealthRecord`
- `business error` / `config error` / `server error`
- `Endpoint blacklisted`
- `blacklist_safe`

### 示例日志分析

```bash
# 查看双URL路由
grep "GetFullURLWithFormat" logs/proxy.log

# 查看格式转换
grep "auto-converted" logs/proxy.log

# 查看模型重写
grep "Model rewritten" logs/proxy.log

# 查看拉黑逻辑
grep "business error\|server error\|config error" logs/proxy.log
```

## 🌐 Web界面验证

访问 http://localhost:8080 进行可视化验证：

### 1. 端点管理
- 检查端点配置是否正确加载
- 查看双URL配置显示
- 观察端点状态变化

### 2. 请求日志
- 查看实时请求日志
- 过滤特定类型的请求
- 检查模型重写和格式转换记录

### 3. 系统设置
- 验证黑名单配置
- 测试全局开关功能
- 查看配置修改效果

### 4. 仪表板
- 查看请求统计
- 监控端点健康状态
- 分析错误分布

## ✅ 验证清单

### 双URL配置
- [ ] Claude Code请求路由到Anthropic URL
- [ ] Codex请求路由到OpenAI URL
- [ ] 单URL配置时正确回退
- [ ] 日志记录正确

### OpenAI格式自适应
- [ ] 自动探测/responses支持
- [ ] 失败后自动转换为chat_completions
- [ ] 学习结果记录到端点配置
- [ ] 不同偏好设置工作正常

### 模型重写
- [ ] 显式重写规则正确应用
- [ ] 隐式重写逻辑工作
- [ ] 通用端点自动重写
- [ ] 响应中模型名正确恢复

### 端点拉黑
- [ ] 业务错误不触发拉黑
- [ ] 配置错误触发拉黑
- [ ] 服务器错误触发拉黑
- [ ] 全局开关控制拉黑功能
- [ ] 错误分类正确

## 🐛 常见问题

### Q: 请求失败但看不到预期的行为
**A:** 检查以下几点：
1. 确认端点配置正确，特别是URL和认证信息
2. 查看日志级别是否设置为debug
3. 确认端点优先级设置正确

### Q: 格式自适应不工作
**A:** 检查：
1. `openai_preference` 配置是否正确
2. 端点是否为OpenAI类型
3. 请求路径是否为/responses

### Q: 模型重写不生效
**A:** 确认：
1. `model_rewrite.enabled` 为true
2. 重写规则匹配请求的模型
3. 检查通配符语法正确

### Q: 端点拉黑逻辑异常
**A:** 验证：
1. `blacklist.enabled` 为true
2. `auto_blacklist` 为true
3. 错误分类配置符合预期