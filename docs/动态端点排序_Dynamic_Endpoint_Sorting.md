# 动态端点排序 (Dynamic Endpoint Sorting)

## 📊 概述 (Overview)

动态端点排序是 CCCC 的智能负载均衡功能，根据端点的实时性能指标自动调整请求优先级，确保始终优先使用最快、最稳定的端点。

Dynamic Endpoint Sorting is CCCC's intelligent load balancing feature that automatically adjusts request priorities based on real-time performance metrics, ensuring that the fastest and most stable endpoints are always prioritized.

## 🎯 核心原理 (Core Principles)

### 排序策略 (Sorting Strategy)

系统根据以下指标对端点进行排序（优先级从高到低）：

The system sorts endpoints based on the following metrics (in descending priority):

1. **可用性 (Availability)** - 端点当前是否可用
   - Available endpoints are always prioritized over unavailable ones

2. **成功率 (Success Rate)** - 最近请求的成功比率
   - Higher success rate = higher priority

3. **响应速度 (Response Time)** - 平均响应时间
   - Faster response = higher priority

4. **原始优先级 (Original Priority)** - 配置文件中的初始优先级
   - Used as tiebreaker when other metrics are equal

### 重要特性 (Key Features)

- ✅ **只看当下性能** - 不考虑历史请求总量，只关注当前速度和错误率
  - Focus on current performance only, not historical request volume

- ✅ **客户端验证安全** - "只允许 Claude Code CLI" 等业务错误不视为端点故障
  - Client validation failures (like "Claude Code CLI only") are not treated as endpoint failures

- ✅ **自动持久化** - 优先级变更自动保存到配置文件
  - Priority changes are automatically persisted to config.yaml

- ✅ **实时更新** - 无需重启服务器，优先级变更立即生效
  - Changes take effect immediately without server restart

## 🚀 使用方法 (Usage)

### 1. 启用动态排序 (Enable Dynamic Sorting)

在 `config.yaml` 中启用：

```yaml
server:
  host: 0.0.0.0
  port: 8080
  auto_sort_endpoints: true  # 启用动态排序
```

### 2. 测试端点性能 (Test Endpoint Performance)

通过管理后台触发批量测试：

1. 访问 `http://localhost:8080`
2. 进入 **端点配置** 页面
3. 点击 **批量测试** 按钮

或通过 API：

```bash
# 测试所有端点
curl -X POST http://localhost:8080/admin/api/endpoints/test-all \
  -H "Authorization: Bearer your-token"

# 测试单个端点
curl -X POST http://localhost:8080/admin/api/endpoints/test \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d '{"endpoint_name": "your-endpoint"}'
```

### 3. 手动触发排序 (Manual Sort Trigger)

点击 **端点排序** 按钮，或通过 API：

```bash
curl -X POST http://localhost:8080/admin/api/endpoints/sort \
  -H "Authorization: Bearer your-token"
```

## 📈 性能指标详解 (Performance Metrics)

### 可用性状态 (Availability Status)

- **Active (活跃)** - 端点正常工作，最近请求成功
- **Inactive (不可用)** - 端点连续失败，被暂时禁用
- **Checking (检测中)** - 正在进行健康检查

### 成功率计算 (Success Rate Calculation)

```
成功率 = 成功请求数 / 总请求数
Success Rate = Successful Requests / Total Requests
```

### 响应时间 (Response Time)

系统记录每个端点的平均响应时间，包括：
- DNS 查找时间 (DNS Lookup Time)
- TCP 连接时间 (TCP Connect Time)
- TLS 握手时间 (TLS Handshake Time)
- 首字节时间 (Time to First Byte)
- 内容下载时间 (Content Download Time)

## 🔧 高级配置 (Advanced Configuration)

### 端点拉黑策略 (Endpoint Blacklist Strategy)

配合端点拉黑功能，可以自动隔离失败的端点：

```yaml
blacklist:
  enabled: true              # 启用拉黑功能
  auto_blacklist: true        # 自动拉黑失败端点
  business_error_safe: true   # 业务错误不触发拉黑
  config_error_safe: false    # 配置错误触发拉黑
  server_error_safe: false    # 服务器错误触发拉黑
```

**错误类型说明 (Error Types)**：

1. **业务错误 (Business Error)** - API 正常返回错误响应
   - 示例：模型不支持、参数错误、客户端验证失败
   - 推荐：`business_error_safe: true` (不拉黑)

2. **配置错误 (Config Error)** - 客户端配置问题
   - 示例：认证失败 (401)、权限不足 (403)
   - 推荐：`config_error_safe: false` (拉黑)

3. **服务器错误 (Server Error)** - 基础设施问题
   - 示例：5xx 错误、网络超时、连接失败
   - 推荐：`server_error_safe: false` (拉黑)

### 健康检查配置 (Health Check Configuration)

```yaml
timeouts:
  health_check_timeout: 30s   # 健康检查超时时间
  check_interval: 30s          # 健康检查间隔
  recovery_threshold: 1        # 恢复阈值（连续成功次数）
```

## 📊 监控与调试 (Monitoring & Debugging)

### 查看端点状态 (View Endpoint Status)

通过 Web 管理界面：
- 访问 `http://localhost:8080`
- 查看 **端点配置** 页面
- 查看每个端点的状态、成功率、响应时间

### 日志查看 (Log Viewing)

```bash
# 查看排序日志
grep "端点排序" logs/proxy.log

# 查看优先级变更
grep "更新端点.*的优先级" logs/proxy.log

# 查看测试结果
grep "Anthropic format test" logs/proxy.log
```

### API 查询端点信息 (Query Endpoint Info)

```bash
# 获取所有端点状态
curl http://localhost:8080/admin/api/endpoints \
  -H "Authorization: Bearer your-token"

# 获取端点详情
curl http://localhost:8080/admin/api/endpoints/endpoint-name \
  -H "Authorization: Bearer your-token"
```

## 🎓 最佳实践 (Best Practices)

### 1. 定期测试端点 (Regular Testing)

建议每天或每周执行一次批量测试，确保端点性能数据是最新的：

```bash
# 设置定时任务 (Cron Job)
0 2 * * * curl -X POST http://localhost:8080/admin/api/endpoints/test-all \
  -H "Authorization: Bearer your-token"
```

### 2. 监控成功率 (Monitor Success Rate)

关注成功率低于 95% 的端点，及时排查问题：

```bash
# 查看失败请求
grep "❌.*test failed" logs/proxy.log
```

### 3. 优化响应时间 (Optimize Response Time)

- 使用地理位置接近的端点
- 启用 HTTP/2 和压缩
- 配置合适的超时时间

### 4. 合理配置拉黑策略 (Configure Blacklist Wisely)

**开发/测试环境** (Development/Testing):
```yaml
blacklist:
  enabled: true
  auto_blacklist: true
  business_error_safe: true   # 宽松策略
  config_error_safe: true
  server_error_safe: true
```

**生产环境** (Production):
```yaml
blacklist:
  enabled: true
  auto_blacklist: true
  business_error_safe: true   # 避免误判
  config_error_safe: false    # 快速隔离配置问题
  server_error_safe: false    # 保护整体可用性
```

## 🔍 故障排除 (Troubleshooting)

### Q: 点击"端点排序"没有变化

**可能原因**：
1. 端点没有性能数据（未执行过测试）
2. 所有端点性能相同
3. 动态排序功能未启用

**解决方案**：
1. 先执行"批量测试"
2. 检查 `config.yaml` 中 `auto_sort_endpoints: true`
3. 查看日志：`grep "端点排序" logs/proxy.log`

### Q: 测试后端点仍然排在后面

**可能原因**：
1. 端点状态被标记为"不可用"
2. 成功率低于其他端点
3. 响应时间慢于其他端点

**解决方案**：
1. 检查端点详情页的测试结果
2. 查看具体错误信息
3. 使用"重置状态"清除失败记录

### Q: 优先级没有保存到配置文件

**可能原因**：
1. 配置文件权限问题
2. 配置文件路径错误

**解决方案**：
```bash
# 检查文件权限
ls -la config.yaml

# 查看保存日志
grep "端点优先级已持久化" logs/proxy.log
```

## 📚 相关文档 (Related Documentation)

- [README.md](../README.md) - 项目总览
- [CLAUDE_CODE.md](../CLAUDE_CODE.md) - Claude Code 集成指南
- [AGENTS.md](../AGENTS.md) - AI Agent 集成
- [CHANGELOG.md](../CHANGELOG.md) - 版本更新记录

## 🔗 技术实现细节 (Technical Implementation)

### 核心组件 (Core Components)

- **DynamicEndpointSorter** (`internal/utils/dynamic_sorter.go`)
  - 排序引擎和性能指标计算

- **EndpointManager** (`internal/endpoint/endpoint_manager.go`)
  - 端点生命周期管理

- **EndpointTesting** (`internal/web/endpoint_testing.go`)
  - 端点测试和性能收集

- **ClientValidation** (`internal/validator/client_validation.go`)
  - 业务错误 vs 端点故障的智能区分

### 排序算法 (Sorting Algorithm)

```go
// 伪代码 (Pseudo Code)
sort(endpoints) {
    // 1. 按可用性分组
    available = filter(endpoints, ep => ep.IsAvailable())
    unavailable = filter(endpoints, ep => !ep.IsAvailable())

    // 2. 对可用端点排序
    sort(available, (a, b) => {
        if (a.successRate != b.successRate)
            return b.successRate - a.successRate
        if (a.responseTime != b.responseTime)
            return a.responseTime - b.responseTime
        return a.priority - b.priority
    })

    // 3. 重新分配优先级
    priority = 1
    for (ep in available) {
        ep.priority = priority++
    }
    for (ep in unavailable) {
        ep.priority = priority++
    }

    // 4. 持久化到配置文件
    persist(endpoints)
}
```

## 🎯 性能优化建议 (Performance Tips)

1. **使用 HTTP/2** - 提升连接复用效率
2. **启用压缩** - 减少带宽占用
3. **配置合理超时** - 避免慢端点拖累整体性能
4. **定期清理日志** - 保持系统轻量运行
5. **监控端点健康** - 及时发现和隔离问题端点

---

**最后更新 (Last Updated)**: 2025-01-22
**版本 (Version)**: 1.0.0
