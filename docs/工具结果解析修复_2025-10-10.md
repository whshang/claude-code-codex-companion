# Tool Result 解析修复说明

## 问题描述

用户在使用 Claude Code 发送包含 tool_result 的消息时遇到错误：

```
API Error: 400 {"details":"user.tool_result is missing tool_use_id","error":"Request format conversion failed"}
```

## 根本原因

在 `internal/conversion/anthropic_types.go` 的 `GetContentBlocks()` 方法中，当解析 `tool_result` 类型的 content block 时，如果该 block 的 `content` 字段是数组格式（嵌套的 `[]AnthropicContentBlock`），代码只提取了嵌套内容的 `type` 和 `text` 字段，**但没有提取 `tool_use_id` 和 `is_error` 字段**。

### 问题代码位置

```234:241:internal/conversion/anthropic_types.go
for _, contentItem := range contentArray {
    if contentMap, ok := contentItem.(map[string]interface{}); ok {
        contentBlock := AnthropicContentBlock{}
        if typ, exists := contentMap["type"].(string); exists {
            contentBlock.Type = typ
        }
        if text, exists := contentMap["text"].(string); exists {
            contentBlock.Text = text
        }
        // ❌ 缺少 tool_use_id 和 is_error 的提取
        contentBlocks = append(contentBlocks, contentBlock)
    }
}
```

### Claude Code 发送的实际数据结构

```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_123456",
      "content": [
        {
          "type": "text",
          "text": "Tool execution result"
        }
      ]
    }
  ]
}
```

当 JSON 反序列化时：
1. `AnthropicMessage.Content` 是 `interface{}` 类型
2. JSON 解码器将其解析为 `[]interface{}`（不是 `[]AnthropicContentBlock`）
3. 调用 `GetContentBlocks()` 时走 `case []interface{}` 分支
4. 外层正确提取了 `tool_use_id: "toolu_123456"`（第 221-223 行）
5. 但嵌套的 `content` 数组再次解析时（第 229-252 行），**遗漏了 `tool_use_id` 提取**
6. 导致后续转换时找不到 `tool_use_id`

## 修复方案

在递归处理嵌套 content 数组时，添加对 `tool_use_id` 和 `is_error` 字段的提取：

```go
for _, contentItem := range contentArray {
    if contentMap, ok := contentItem.(map[string]interface{}); ok {
        contentBlock := AnthropicContentBlock{}
        if typ, exists := contentMap["type"].(string); exists {
            contentBlock.Type = typ
        }
        if text, exists := contentMap["text"].(string); exists {
            contentBlock.Text = text
        }
        // ✅ 修复：递归处理嵌套 content 中的 tool_use_id 和 is_error
        if toolUseID, exists := contentMap["tool_use_id"].(string); exists {
            contentBlock.ToolUseID = toolUseID
        }
        if isError, exists := contentMap["is_error"].(bool); exists {
            contentBlock.IsError = &isError
        }
        contentBlocks = append(contentBlocks, contentBlock)
    }
}
```

## 验证测试

新增 4 个测试用例验证修复：

1. **TestToolResultWithNestedContent** - 嵌套数组格式的 tool_result
2. **TestToolResultWithStringContent** - 字符串格式的 tool_result
3. **TestToolResultWithError** - 带 is_error 标志的 tool_result
4. **TestMixedUserContent** - 混合 tool_result 和普通文本的用户消息

所有测试均通过：

```bash
$ go test -v ./internal/conversion -run TestToolResult
=== RUN   TestToolResultWithNestedContent
    tool_result_test.go:77: ✅ Tool result with nested content parsed successfully
    tool_result_test.go:78:    ToolCallID: toolu_123456
    tool_result_test.go:79:    Content: Tool execution result
--- PASS: TestToolResultWithNestedContent (0.00s)
=== RUN   TestToolResultWithStringContent
    tool_result_test.go:125: ✅ Tool result with string content parsed successfully
--- PASS: TestToolResultWithStringContent (0.00s)
=== RUN   TestToolResultWithError
    tool_result_test.go:178: ✅ Tool result with error flag parsed successfully
--- PASS: TestToolResultWithError (0.00s)
=== RUN   TestMixedUserContent
    tool_result_test.go:265: ✅ Mixed user content (tool_result + text) parsed successfully
--- PASS: TestMixedUserContent (0.00s)
PASS
```

## 相关文件修改

1. **internal/conversion/anthropic_types.go** - 核心修复（第 241-247 行）
2. **internal/conversion/tool_result_test.go** - 新增测试文件
3. **internal/i18n/translator.go** - 修复缺少 `fmt` 导入
4. **CHANGELOG.md** - 记录此次修复

## 技术洞察

### 为什么会发生这个问题？

Go 的 `encoding/json` 包在反序列化 `interface{}` 类型时：
- JSON 对象 → `map[string]interface{}`
- JSON 数组 → `[]interface{}`
- JSON 字符串 → `string`
- JSON 数字 → `float64`
- JSON 布尔 → `bool`

因此 `AnthropicMessage.Content interface{}` 永远不会被解析为 `[]AnthropicContentBlock` 类型，而总是 `[]interface{}`。这意味着 `GetContentBlocks()` 的第 266 行（`case []AnthropicContentBlock`）实际上是**死代码**，永远不会被执行。

### Fallback 机制

代码中已经有一个 fallback 机制（request_converter.go 第 158-167 行）：

```go
if tr.ToolUseID == "" {
    // 尝试通过工具名匹配最近一次生成的 call id
    if tr.Name != "" {
        if fallbackID, exists := latestCallIDByName[tr.Name]; exists && fallbackID != "" {
            tr.ToolUseID = fallbackID
        }
    }
    if tr.ToolUseID == "" {
        return nil, nil, errors.New("user.tool_result is missing tool_use_id")
    }
}
```

但这个机制无法在当前场景下工作，因为：
1. `tool_result` block 通常没有 `name` 字段
2. 即使有 `name`，也可能因为多个工具调用而匹配错误

现在修复后，这个 fallback 机制作为最后一道保险，但应该很少被触发。

## 影响范围

这个修复影响所有使用 Claude Code 进行 tool calling 的场景，特别是：
- 当 tool_result 的 content 是数组格式时
- 当需要代理转换 Anthropic 格式到 OpenAI 格式时

修复后，这些场景将正常工作，不再出现 "missing tool_use_id" 错误。

