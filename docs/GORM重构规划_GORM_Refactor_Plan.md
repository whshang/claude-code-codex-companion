# GORM 重构规划 | GORM Refactor Plan

## 中文

日志与统计存储从手写 SQL 迁移至 GORM，同时保持 `modernc.org/sqlite` 驱动，统一管理模型、迁移与配置。

### 当前状态

- 模型定义：`internal/logger/gorm_models.go`。  
- 迁移与索引：`internal/logger/gorm_migration.go`。  
- GORM 配置（忙碌超时、WAL 模式）：`internal/logger/gorm_config.go`。  

### 后续提升

- 补充基准测试评估写入性能。  
- 对只读查询场景考虑使用原生 SQL，降低 ORM 开销。  

### 注意事项

- 升级 GORM 时同步验证 `sqlite` 驱动版本。  
- 迁移失败的回退策略：保留数据库备份并加载旧版二进制。  

## English

Log and statistics storage moved from raw SQL to GORM while keeping the `modernc.org/sqlite` driver, centralising models, migrations, and configuration.

### Current Status

- Models: `internal/logger/gorm_models.go`.  
- Migrations & indexes: `internal/logger/gorm_migration.go`.  
- GORM configuration (busy timeout, WAL): `internal/logger/gorm_config.go`.  

### Next Steps

- Add benchmarks to measure insert throughput.  
- Consider raw SQL for read-heavy paths to reduce ORM overhead.  

### Notes

- Validate the SQLite driver version when upgrading GORM.  
- Keep database backups to roll back if migrations fail.  
