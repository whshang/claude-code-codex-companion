# 统计持久化设计 | Statistics Persistence Design

## 中文

端点的请求次数、成功率和失败原因存储在 `statistics.db`，重启后继续累积，避免历史数据丢失。

### 实现

- GORM 模型：`internal/statistics/models.go` 定义 `endpoint_statistics`。  
- 更新逻辑：`internal/statistics/service.go` 在请求完成后写入计数，同时保留环形缓冲用于即时判定。  
- 管理界面：`/admin/dashboard`、`/admin/endpoints` 使用持久化数据展示趋势。  

### 注意事项

- 以端点名称为键；重命名端点需手动迁移历史数据。  
- 数据库位于 `statistics.db`，建议定期备份。  

## English

Endpoint metrics (request counts, success rate, failure reasons) are stored in `statistics.db`, allowing accumulation across restarts.

### Implementation

- GORM model `internal/statistics/models.go` defines `endpoint_statistics`.  
- `internal/statistics/service.go` updates counts after each request while retaining the circular buffer for quick health checks.  
- Admin pages `/admin/dashboard` and `/admin/endpoints` render trends using this data.  

### Notes

- Metrics are keyed by endpoint name; renaming endpoints requires manual migration if history matters.  
- Keep backups of `statistics.db` to avoid data loss.  
