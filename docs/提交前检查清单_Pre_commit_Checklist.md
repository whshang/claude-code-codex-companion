# 提交前检查清单 | Pre-commit Checklist

## 中文

在提交代码前请依次完成以下检查：

### 快速检查

- [ ] 执行 `go test ./...` 并确保全部通过。  
- [ ] 使用 `git status` 确认没有将 `config.yaml`、`logs/`、`*.db` 等敏感文件纳入提交。  
- [ ] 检查 `CHANGELOG.md` 是否更新到对应日期。  
- [ ] 若修改脚本或配置，更新 `/help` 页面与相关文档。  

### 敏感信息

确保 diff 中没有真实 API Key、Bearer Token、数据库快照或用户数据；如发现，请立即移除或脱敏。

### 提交信息

- 提交消息需简明描述改动，例如 `feat: refine streaming proxy state machine`。  
- 若变更影响用户或部署方式，请同步更新 README 及相关指南。  

## English

Complete the following checklist before committing:

### Quick Checks

- [ ] Run `go test ./...` and confirm all tests pass.  
- [ ] Ensure `git status` shows no sensitive files (`config.yaml`, `logs/`, `*.db`, etc.).  
- [ ] Update `CHANGELOG.md` with the appropriate date entry.  
- [ ] Refresh `/help` and documentation if scripts or configuration changed.  

### Sensitive Data

Verify that the diff contains no real API keys, bearer tokens, database dumps, or user data; remove or mask them immediately if found.

### Commit Message

- Provide a concise description such as `feat: refine streaming proxy state machine`.  
- Update README and related docs when user-facing behaviour changes.  
