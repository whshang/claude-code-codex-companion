# Token Usage 统计程序设计方案

## 需求理解

根据用户需求，需要创建一个独立的统计程序，分析特定时间段内的 API 请求数据：

### 数据筛选条件
1. **时间范围**：2025年8月26日 14:00 - 18:00 (GMT+8 时区)
2. **端点过滤**：只统计 `endpoint` 字段包含 `api.anthropic.com` 的请求
3. **状态码过滤**：只统计 `status_code = 200` 的成功请求
4. **模型过滤**：排除所有包含 `haiku` 的模型请求

### 数据提取要求
1. **Token Usage 提取**：从 `original_response_body` 字段解析 JSON，提取：
   - `input_tokens`
   - `cache_creation_input_tokens` 
   - `cache_read_input_tokens`
   - `output_tokens`

2. **Rate Limit 状态提取**：从 `original_response_headers` 字段解析 JSON，提取：
   - `Anthropic-Ratelimit-Unified-5h-Status` 字段值（预期值：`allowed`、`allowed_warning`、`rejected`）

### 统计聚合要求
按照 Rate Limit 状态分组统计：
- 每个状态的请求数量
- 每个状态的 token usage 总和（各类 token 分别统计）
- 重点关注 `allowed` 和 `allowed_warning` 状态

## 数据库结构分析

基于代码分析，确认数据存储结构：

### 数据库信息
- **数据库类型**：SQLite
- **默认路径**：`./logs/logs.db`
- **表名**：`request_logs`

### 关键字段映射
```sql
CREATE TABLE request_logs (
    -- 时间和基础信息
    timestamp DATETIME NOT NULL,           -- 请求时间
    endpoint TEXT NOT NULL,                -- 端点URL
    status_code INTEGER DEFAULT 0,         -- HTTP状态码
    model TEXT DEFAULT '',                 -- 模型名称
    
    -- 原始响应数据（关键字段）
    original_response_headers TEXT DEFAULT '{}',  -- 原始响应头（JSON字符串）
    original_response_body TEXT DEFAULT '',       -- 原始响应体（JSON字符串）
    
    -- 其他字段...
);
```

### 索引情况
根据代码，存在以下索引：
- `idx_timestamp` on `timestamp`
- `idx_endpoint` on `endpoint` 
- `idx_status_code` on `status_code`

## 技术实现方案

### 实现路径规划

可以按以下步骤落地统计逻辑（不限定具体编程语言）：

1. 连接 SQLite 数据库，确认时间范围与索引使用情况。
2. 构建 SQL 查询语句，应用所有筛选条件：
   - timestamp BETWEEN '2025-08-26 14:00:00' AND '2025-08-26 18:00:00'（考虑 GMT+8 与数据库时区的换算）。
   - endpoint LIKE '%api.anthropic.com%'
   - status_code = 200
   - model NOT LIKE '%haiku%'（处理大小写与空值情况）。
3. 遍历查询结果：
   - 解析 original_response_body（JSON）提取 usage 信息。
   - 解析 original_response_headers（JSON）提取 rate limit 状态。
   - 按状态分组累计统计并输出统一格式的结果。

上述步骤既可通过 Go/SQL 执行，也可在 Web 界面中调用 API 来实现。

### SQL 查询语句示例

```sql
-- 注意：需要根据数据库中时间戳的存储格式调整时间范围
-- 如果数据库存储UTC时间，需要转换GMT+8到UTC (减去8小时)
SELECT 
    original_response_body,
    original_response_headers,
    model,
    timestamp,
    request_id
FROM request_logs 
WHERE timestamp >= '2025-08-26 06:00:00'  -- GMT+8 14:00 转换为UTC 06:00
  AND timestamp <= '2025-08-26 10:00:00'  -- GMT+8 18:00 转换为UTC 10:00
  AND endpoint LIKE '%api.anthropic.com%'
  AND status_code = 200
  AND (model IS NULL OR model NOT LIKE '%haiku%')
ORDER BY timestamp;
```

### 数据解析策略

#### Response Body 解析
预期 `original_response_body` 包含如下 JSON 结构：
```json
{
    "usage": {
        "input_tokens": 123,
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": 45,
        "output_tokens": 67
    }
}
```

#### Response Headers 解析
预期 `original_response_headers` 包含如下 JSON 结构：
```json
{
    "Anthropic-Ratelimit-Unified-5h-Status": "allowed"
}
```

### 错误处理策略
1. **数据库连接失败**：输出错误信息并退出
2. **JSON 解析失败**：记录警告，跳过该条记录
3. **字段缺失**：使用默认值（token 为 0，状态为 "unknown"）
4. **空结果集**：输出提示信息

### 输出格式设计

```
Token Usage Statistics Report
============================
Time Range: 2025-08-26 14:00:00 - 18:00:00 (GMT+8)
Filter: api.anthropic.com, Status=200, Non-Haiku models

Summary by Rate Limit Status:
-----------------------------

ALLOWED:
  Request Count: 150
  Total Input Tokens: 45,230
  Total Cache Creation Tokens: 1,200
  Total Cache Read Tokens: 8,450
  Total Output Tokens: 12,340

ALLOWED_WARNING:
  Request Count: 25
  Total Input Tokens: 7,850
  Total Cache Creation Tokens: 200
  Total Cache Read Tokens: 1,100
  Total Output Tokens: 2,100

REJECTED: (if any)
  Request Count: 0

UNKNOWN/ERROR: (parsing failures)
  Request Count: 2

Total Processed Records: 177
```

## 实现注意事项

1. **时区处理**：
   - 确认数据库中时间戳的存储格式（UTC vs 本地时间）
   - GMT+8 14:00-18:00 需要转换为对应的UTC时间进行查询
   - 如果数据库存储UTC时间：GMT+8 14:00 = UTC 06:00，GMT+8 18:00 = UTC 10:00
2. **JSON 字段处理**：原始字段可能为空字符串或 "null"，需要安全解析
3. **模型名称过滤**：考虑大小写不敏感匹配
4. **内存使用**：如果数据量大，考虑分批处理
5. **脚本独立性**：不依赖主项目的任何模块，可独立运行

## 实现建议

可以通过以下方式实现Token使用统计分析：

1. **Go实现**：在项目中添加独立的统计分析命令
2. **SQL查询**：直接通过SQL查询生成统计报告
3. **Web界面**：在管理后台添加Token统计分析页面

该设计确保了程序的独立性和可维护性，同时充分利用了现有数据库结构中的所有相关信息。
