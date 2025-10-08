# Tool Call Enhancement 验证指南

本文档介绍如何验证工具调用增强功能是否生效，特别是在 Claude Code 和 Codex 中的使用。

## 🎯 验证方法

### 方法1：使用提供的测试脚本

#### 基础工具调用测试
```bash
./test_tool_call.sh
```

#### Claude Code/Codex 专用测试
```bash
./test_tool_claude_code.sh
```

### 方法2：手动验证

#### 1. 启动代理服务
```bash
./cccc -config config.yaml
```

#### 2. 发送包含 tools 的请求

**Anthropic 格式（Claude Code）**:
```bash
curl -X POST "http://127.0.0.1:8081/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "What is the weather in Tokyo?"}
    ],
    "tools": [
      {
        "name": "get_weather",
        "description": "Get current weather for a location",
        "input_schema": {
          "type": "object",
          "properties": {
            "location": {"type": "string", "description": "City name"}
          },
          "required": ["location"]
        }
      }
    ]
  }'
```

**OpenAI 格式（Codex）**:
```bash
curl -X POST "http://127.0.0.1:8081/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-5-codex",
    "messages": [
      {"role": "user", "content": "What is the weather in Tokyo?"}
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "Get current weather for a location",
          "parameters": {
            "type": "object",
            "properties": {
              "location": {"type": "string", "description": "City name"}
            },
            "required": ["location"]
          }
        }
      }
    ]
  }'
```

**Codex /responses 格式**:
```bash
curl -X POST "http://127.0.0.1:8081/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-5",
    "input": [
      {"role": "user", "content": [{"type": "text", "text": "What is the weather in Tokyo?"}]}
    ],
    "tools": [
      {
        "name": "get_weather",
        "description": "Get current weather for a location",
        "input_schema": {
          "type": "object",
          "properties": {
            "location": {"type": "string", "description": "City name"}
          },
          "required": ["location"]
        }
      }
    ]
  }'
```

## 📊 预期结果

### ✅ 成功的响应格式

**Anthropic 格式响应**:
```json
{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "model": "claude-sonnet-4-20250514",
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_abc123",
      "name": "get_weather",
      "input": {"location": "Tokyo"}
    }
  ]
}
```

**OpenAI 格式响应**:
```json
{
  "id": "chatcmpl_abc123",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-5-codex",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"location\": \"Tokyo\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ]
}
```

### ❌ 失败的响应

如果工具调用增强没有生效，你会看到：
- 普通的文本回复（如 "I cannot help with that"）
- `content` 字段包含文字而不是工具调用
- 没有 `tool_use` 或 `tool_calls` 字段

## 🔍 日志验证

检查代理日志以确认工具调用增强是否被触发：

```bash
tail -f logs/proxy.log | grep -i "tool\|injected"
```

**预期的日志条目**：
```
INFO  Tool calling: injected system prompt for request {"endpoint": "KAT-Coder", "client_format": "anthropic"}
```

## 🚀 在 Claude Code 中使用

### 配置 Claude Code

```bash
# 设置环境变量
export ANTHROPIC_BASE_URL="http://127.0.0.1:8081"
export ANTHROPIC_AUTH_TOKEN="hello"

# 启动 Claude Code
claude code
```

### 验证方法

1. **直接使用 tools**：
```python
# 在 Claude Code 中测试
import subprocess

def get_weather(location):
    """获取指定位置的天气信息"""
    # 这里可以调用天气 API
    return f"当前{location}的天气：晴天，25°C"

# Claude 应该能调用这个函数
```

2. **检查日志**：
```bash
# 在另一个终端监控日志
tail -f logs/proxy.log | grep "Tool calling"
```

## 🚀 在 Codex 中使用

### 配置 Codex

```bash
# 设置环境变量
export OPENAI_BASE_URL="http://127.0.0.1:8081"
export OPENAI_API_KEY="hello"
```

### 验证方法

1. **直接 API 调用**：使用上面提供的 curl 命令
2. **通过 OpenAI 客户端库**：
```python
import openai

client = openai.OpenAI(
    base_url="http://127.0.0.1:8081",
    api_key="hello"
)

response = client.chat.completions.create(
    model="gpt-5-codex",
    messages=[{"role": "user", "content": "What is the weather in Tokyo?"}],
    tools=[{
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "Get current weather for a location",
            "parameters": {
                "type": "object",
                "properties": {
                    "location": {"type": "string", "description": "City name"}
                },
                "required": ["location"]
            }
        }
    }]
)

# 检查 response.choices[0].message.tool_calls
```

## 🛠️ 故障排除

### 常见问题

1. **没有工具调用响应**
   - 检查端点是否已启用
   - 确认请求包含 `tools` 字段
   - 查看日志是否有错误信息

2. **响应格式不正确**
   - 确认日志中有 "injected system prompt"
   - 检查端点模型是否支持 XML 输出
   - 验证配置的模型重写规则

3. **日志显示错误**
   - 检查认证信息是否正确
   - 确认端点 URL 可访问
   - 查看详细错误信息

### 调试技巧

1. **启用详细日志**：
```yaml
logging:
  level: debug
  log_request_body: full
  log_response_body: truncated
```

2. **检查请求/响应**：
```bash
# 查看最近的请求
tail -100 logs/proxy.log | grep -A 10 -B 10 "Tool calling"
```

3. **测试不同端点**：
   - 逐个启用端点进行测试
   - 比较不同端点的行为差异

## 📈 成功指标

工具调用增强成功的关键指标：

1. ✅ 请求包含 `tools` 字段时自动注入系统提示
2. ✅ 响应包含标准的 `tool_use` 或 `tool_calls` 格式
3. ✅ 日志显示 "injected system prompt" 消息
4. ✅ Claude Code 和 Codex 能正常接收和处理工具调用
5. ✅ 不影响不包含 tools 的普通请求

如果满足以上所有条件，说明工具调用增强功能已成功集成并可投入使用。