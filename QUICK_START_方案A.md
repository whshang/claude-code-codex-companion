# 🚀 方案A快速开始指南

## 5分钟快速上手

### 步骤1：准备配置文件

```bash
# 复制示例配置
cp config.yaml.solutionA config.yaml
```

### 步骤2：配置你的端点

编辑 `config.yaml`，最少配置一个端点：

```yaml
server:
  host: 127.0.0.1
  port: 8080

endpoints:
  # Claude Code端点（必须配置）
  - name: my-anthropic
    url_anthropic: https://api.anthropic.com/v1/messages
    auth_type: api_key
    auth_value: sk-ant-YOUR_API_KEY_HERE  # ⬅️ 替换为你的Key
    enabled: true
    priority: 1
    client_type: claude_code
    native_format: true
    tags: ["official"]
```

### 步骤3：启动服务

```bash
# 编译（如果还没编译）
go build -o cccc .

# 启动
./cccc -config config.yaml
```

### 步骤4：验证运行

```bash
# 访问管理界面
open http://localhost:8080

# 或使用curl测试
curl http://localhost:8080/api/endpoints
```

## 高级配置：多端点+故障转移

```yaml
endpoints:
  # 主端点：Anthropic官方
  - name: anthropic-primary
    url_anthropic: https://api.anthropic.com/v1/messages
    auth_type: api_key
    auth_value: sk-ant-xxx
    priority: 1
    client_type: claude_code
    native_format: true

  # 备用端点：第三方（需要转换）
  - name: openai-fallback
    url_openai: https://third-party.com/v1/chat/completions
    auth_type: auth_token
    auth_value: xxx
    priority: 2
    client_type: claude_code
    native_format: false      # 需要格式转换
    target_format: openai_chat
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: claude-*
          target_model: gpt-4-turbo
```

## 故障排除

### 问题1：启动失败

```bash
# 检查配置文件语法
go run . -config config.yaml -validate

# 查看详细日志
./cccc -config config.yaml -log-level debug
```

### 问题2：端点不可用

1. 检查API Key是否正确
2. 检查URL是否可访问
3. 查看 `logs/proxy.log` 详细错误

### 问题3：Claude Code连接失败

确保环境变量已设置：

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_AUTH_TOKEN="hello"
```

## 更多帮助

- 完整配置示例：`config.yaml.solutionA`
- 详细指南：`docs/方案A实施指南_SolutionA_Implementation_Guide.md`
- 实施总结：`方案A实施总结_Implementation_Summary.md`
