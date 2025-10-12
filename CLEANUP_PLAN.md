# 跨家族转换代码清理计划

## 目标
完全移除所有 Anthropic ↔ OpenAI 跨家族转换代码，仅保留 OpenAI 内部的 Chat ↔ Responses 转换。

## 清理范围

### 1. 需要删除的文件
- `internal/conversion/openai_to_anthropic_request.go` - OpenAI请求到Anthropic格式转换
- `internal/conversion/anthropic_to_openai_response.go` - Anthropic响应到OpenAI格式转换
- `internal/conversion/anthropic_format_adapter.go` - Anthropic格式适配器

### 2. 需要修改的文件

#### `internal/proxy/proxy_logic.go`
需要移除的逻辑：
- Line 362: `shouldConvertAnthropicResponseToOpenAI` 变量及其所有使用
- Line 408-418: 跨家族转换判断逻辑（Anthropic→OpenAI, OpenAI→Anthropic）
- Line 441-483: 跨家族转换执行代码块
  - OpenAI → Anthropic 请求转换分支
  - 相关路径转换逻辑
- Line 1527-1562: Anthropic响应到OpenAI响应转换代码
- Line 2114: `shouldConvertAnthropicResponseToOpenAI` 函数参数
- 所有调用 `conversion.ConvertOpenAIRequestJSONToAnthropic` 的地方
- 所有调用 `conversion.ConvertAnthropicResponseJSONToOpenAI` 的地方
- 所有调用 `conversion.ConvertAnthropicSSEToOpenAI` 的地方

保留的逻辑：
- Codex /responses ↔ OpenAI /chat/completions 转换
- 基于 `openai_preference` 的路径选择
- OpenAI 格式内部的参数映射

#### `internal/conversion/request_converter.go`
- 检查是否有 Anthropic ↔ OpenAI 转换逻辑，如有则移除

#### `internal/conversion/streaming.go` 和 `stream_helpers.go`
- 移除 `StreamAnthropicSSEToOpenAI` 相关函数

### 3. 需要清理的引用
检查并移除以下位置的跨家族转换引用：
- `internal/endpoint/selector.go`
- `internal/config/validation.go`
- `internal/web/admin.go`
- `internal/web/endpoint_testing.go`
- `internal/web/endpoint_crud.go`
- 测试文件：`internal/conversion/*_test.go`

## 执行步骤

### 阶段1: 移除proxy_logic.go中的跨家族转换调用
1. 移除 `shouldConvertAnthropicResponseToOpenAI` 变量
2. 移除跨家族转换判断逻辑
3. 移除跨家族转换执行代码
4. 移除响应转换代码
5. 更新函数签名，删除相关参数

### 阶段2: 删除跨家族转换实现文件
1. 删除 `openai_to_anthropic_request.go`
2. 删除 `anthropic_to_openai_response.go`
3. 删除 `anthropic_format_adapter.go`
4. 从 `streaming.go` 中移除相关函数

### 阶段3: 清理其他引用
1. 搜索并移除所有对已删除函数的引用
2. 清理测试文件
3. 更新相关注释和文档

### 阶段4: 编译和测试
1. 运行 `go build` 确保编译通过
2. 运行 `go test ./...` 确保测试通过
3. 运行 `golangci-lint run` 进行静态检查

## 验证清单
- [ ] 所有 Anthropic ↔ OpenAI 转换代码已删除
- [ ] 仅保留 OpenAI Chat ↔ Responses 转换
- [ ] 代码能够成功编译
- [ ] 所有测试通过
- [ ] 没有遗留的死代码或未使用的导入
- [ ] 日志中没有跨家族转换相关记录
