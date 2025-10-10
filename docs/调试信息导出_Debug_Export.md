# 调试信息导出 | Debug Export

## 中文

日志详情页提供“一键导出调试信息”按钮，可生成包含请求尝试、端点配置与 Tagger 定义的 ZIP，便于问题上报与复现。

### ZIP 结构

```
debug_[REQUEST_ID]_[TIMESTAMP].zip
├── README.txt              # ASCII 说明文件
├── meta.json               # 请求 ID、客户端类型等全局信息
├── attempts/               # 每次重试一个目录
│   └── attempt_1/
│       ├── meta.json
│       ├── original_request_body.txt
│       ├── final_request_body.txt
│       ├── original_response_body.txt
│       └── final_response_body.txt
├── endpoints/              # 相关端点配置 JSON
└── taggers/                # 当前启用的 Tagger 配置
```

### 使用场景

- 分析复杂的降级流程或参数学习结果。  
- 将完整上下文提供给供应商或社区，快速定位问题。  
- 离线回放运行时状态（模型重写、工具增强等）。  

### 实现提示

- 入口：`/admin/logs` → 日志详情 → “导出调试信息”。  
- 后端逻辑：`internal/web/debug_export.go` 负责汇总日志、配置和 Tagger 数据。  
- 如需扩展其他资料，可在 ZIP 中新增目录，并在 README.txt 中说明。  

## English

The log detail view offers an “Export Debug Info” button that bundles request attempts, endpoint configs, and tagger definitions into a ZIP for issue reports and reproduction.

### ZIP Layout

```
debug_[REQUEST_ID]_[TIMESTAMP].zip
├── README.txt
├── meta.json
├── attempts/
│   └── attempt_1/
│       ├── meta.json
│       ├── original_request_body.txt
│       ├── final_request_body.txt
│       ├── original_response_body.txt
│       └── final_response_body.txt
├── endpoints/
└── taggers/
```

### Use Cases

- Analyse downgrade workflows and parameter-learning outcomes.  
- Provide a complete snapshot to vendors or the community for troubleshooting.  
- Replay runtime state (model rewrite, tool enhancement) offline.  

### Implementation Notes

- Entry point: `/admin/logs` → log detail → “Export Debug Info”.  
- Backend: `internal/web/debug_export.go` collects logs, configs, and taggers.  
- Extend the bundle by adding subdirectories and documenting them in README.txt.  
