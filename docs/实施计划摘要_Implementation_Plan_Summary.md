# 实施计划摘要 | Implementation Plan Summary

## 中文

项目按照迭代节奏交付，当前聚焦透明代理、工具调用、数据持久化三大主题。下表列出主要阶段与对应文档：

| 阶段 | 内容 | 参考 |
| --- | --- | --- |
| Proxy 优化 | 流式转换、降级状态机、日志截断 | [透明代理优化计划](透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md) |
| Tool Calling | Prompt 注入、结果解析、指标 | [工具调用增强](工具调用增强_Tool_Calling_Enhancement.md) |
| Data Persistence | 学习结果回写、日志/统计存储 | [学习持久化实现](学习持久化实现_Learning_Persistence_Implementation.md) |

### 待办事项

- 扩展自动化测试覆盖流式降级与工具调用场景。  
- 补充性能基准测试并在 `/admin` 展示趋势。  
- 持续完善双语文档与 `/help` 页面脚本说明。  

### 提交前检查

1. 运行 `go test ./...`。  
2. 更新 `CHANGELOG.md`。  
3. 若涉及脚本或配置，刷新 `/help` 页面示例。  

## English

The project ships iteratively with focus on the transparent proxy, tool calling, and persistence. Current milestones and references:

| Stage | Focus | Reference |
| --- | --- | --- |
| Proxy Optimisation | Streaming conversion, downgrade state machine, log truncation | [Transparent Proxy Optimisation Plan](透明代理优化计划_Transparent_Proxy_Optimisation_Plan.md) |
| Tool Calling | Prompt injection, result parsing, metrics | [Tool Calling Enhancement](工具调用增强_Tool_Calling_Enhancement.md) |
| Data Persistence | Persisting learned results, log/stat storage | [Learning Persistence Implementation](学习持久化实现_Learning_Persistence_Implementation.md) |

### TODO

- Extend automation to cover streaming downgrades and tool-calling flows.  
- Add performance benchmarks and surface them in `/admin`.  
- Keep bilingual documentation and `/help` scripts up to date.  

### Pre-commit Checklist

1. Run `go test ./...`.  
2. Update `CHANGELOG.md`.  
3. Refresh `/help` instructions when scripts or configuration change.  
