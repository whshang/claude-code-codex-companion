# 功能验证步骤 | Verification Steps

## 中文

以下清单用于人工验证关键能力，确保代理、工具调用、降级学习与日志持久化正常运行。

### 核心用例

1. **透明代理**  
   - 执行 `curl -X POST http://127.0.0.1:8080/v1/responses ...`。  
   - 在 `/admin/logs` 查看该请求，确认 `conversion_path` 包含 `responses->chat_completions`。  
2. **Claude Code / Codex 客户端**  
   - 运行 `./cccc-setup-claude-code.sh` 与 `./cccc-setup-codex.sh` 生成配置。  
   - 分别执行 `claude`、`codex`，确认命令成功且路由到预期端点。  
3. **工具调用**  
   - 构造携带 `tools` 的请求，确认日志中 `tool_enhancement_applied=true` 且响应包含工具调用结果。  
4. **降级学习**  
   - 对仅支持 `/chat/completions` 的端点发送 `/responses`，观察第二次请求成功且配置自动更新 `openai_preference=chat_completions`。  
5. **日志导出**  
   - 在 `/admin/logs` 选中请求，点击“导出调试信息”，检查 ZIP 内包含请求/响应、端点配置等文件。  

### 自动化建议

- 使用 `go run ./cmd/test_endpoints -config config.yaml -json` 进行批量探针。  
- 为工具调用与流式场景编写回归脚本，比较首字节延迟与内存占用。  

## English

Use the following checklist to confirm that proxy routing, tool calling, downgrade learning, and log persistence behave as expected.

### Core Scenarios

1. **Transparent proxy**  
   - Run `curl -X POST http://127.0.0.1:8080/v1/responses ...`.  
   - Inspect `/admin/logs` and ensure the entry shows `conversion_path` containing `responses->chat_completions`.  
2. **Claude Code / Codex clients**  
   - Execute `./cccc-setup-claude-code.sh` and `./cccc-setup-codex.sh`.  
   - Run `claude` and `codex`, verifying that requests hit the intended endpoints.  
3. **Tool calling**  
   - Submit a request with `tools`; confirm `tool_enhancement_applied=true` and that the response embeds tool results.  
4. **Downgrade learning**  
   - Send `/responses` to an endpoint that only supports `/chat/completions`; the second attempt should succeed and set `openai_preference=chat_completions`.  
5. **Debug export**  
   - From `/admin/logs`, choose “Export Debug Info” and verify the ZIP contains the request/response pair, endpoint configs, and tagger definitions.  

### Automation Tips

- Run `go run ./cmd/test_endpoints -config config.yaml -json` for bulk probes.  
- Create regression scripts for tool calling and streaming cases to compare first-byte latency and memory usage.  
