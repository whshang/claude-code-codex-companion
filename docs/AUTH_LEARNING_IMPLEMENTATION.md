# 认证方式自动学习功能实现方案

## 功能概述

当端点配置 `auth_type: auto` 时，系统在测试或运行时自动探测有效的认证方式（`api_key` 或 `auth_token`），并将学习结果持久化到配置文件。

## 实现方案

### 1. 核心文件

- ✅ `internal/web/auth_learning.go` - 已创建，包含配置持久化逻辑
- ⚠️  `internal/web/endpoint_testing.go` - 需要修改，集成认证探测逻辑
- ⚠️  `internal/proxy/proxy_logic.go` - 需要修改，运行时认证探测

### 2. 认证探测策略

#### 测试场景（endpoint_testing.go）

```go
func (s *AdminServer) testEndpointFormatWithStream(ep *endpoint.Endpoint, format string, ...) *EndpointTestResult {
    // ...现有代码...
    
    // 智能认证探测
    authType := ep.AuthType
    detectedAuthMethod := ""
    
    if authType == "auto" || authType == "" {
        // 方案1：同时尝试两种认证头
        if format == "anthropic" {
            req.Header.Set("x-api-key", ep.AuthValue)
            req.Header.Set("Authorization", "Bearer " + ep.AuthValue)
            detectedAuthMethod = "api_key" // Anthropic格式默认api_key
        } else {
            // OpenAI格式优先auth_token
            req.Header.Set("Authorization", "Bearer " + ep.AuthValue)
            detectedAuthMethod = "auth_token"
        }
    } else {
        // 使用配置的认证方式（现有逻辑）
        authHeader, _ := ep.GetAuthHeader()
        if format == "anthropic" {
            req.Header.Set("x-api-key", authHeader)
        } else {
            req.Header.Set("Authorization", authHeader)
        }
    }
    
    // ...发送请求...
    resp, err := client.Do(req)
    
    // 成功后保存学习结果
    if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
        if authType == "auto" || authType == "" {
            s.updateAuthType(ep, detectedAuthMethod)
        }
    }
    
    // ...处理响应...
}
```

#### 运行时场景（proxy_logic.go）

```go
// 在 proxyToEndpoint 函数中
if ep.AuthType == "auto" || ep.AuthType == "" {
    // 首次尝试默认方式
    // 如果401/403，则尝试另一种方式并记录
    // 详见 proxy_logic.go 第478-544行的现有认证逻辑
    
    // 当前已有的认证切换逻辑（第497-530行）可以扩展
    // 在认证成功后调用 updateAuthType
}
```

### 3. 配置持久化

#### 更新端点配置并保存（auth_learning.go）

```go
func (s *AdminServer) updateAuthType(ep *endpoint.Endpoint, authType string) {
    // 1. 更新内存中的端点
    ep.AuthType = authType
    
    // 2. 更新配置结构
    for i := range s.config.Endpoints {
        if s.config.Endpoints[i].Name == ep.Name {
            s.config.Endpoints[i].AuthType = authType
            break
        }
    }
    
    // 3. 保存到文件
    s.saveConfigToFile()
    
    // 4. 记录日志
    s.logger.Info(fmt.Sprintf("✓ Auto-learned auth type '%s' for endpoint '%s'", authType, ep.Name))
}
```

### 4. 测试结果展示

测试结果JSON中包含认证学习信息：

```json
{
  "endpoint_name": "test-endpoint",
  "results": [{
    "format": "anthropic",
    "success": true,
    "auth_method_learned": "api_key",
    "config_updated": true
  }]
}
```

## 实现步骤

### 第一阶段：测试场景（优先级高）✅

1. ✅ 创建 `auth_learning.go` 文件
2. ⚠️  修改 `endpoint_testing.go` 中的认证逻辑
   - 在 `testEndpointFormatWithStream` 函数中添加auto探测
   - 成功后调用 `updateAuthType`
3. ⚠️  更新测试结果结构，添加学习信息字段

### 第二阶段：运行时场景（优先级中）

1. ⚠️  修改 `proxy_logic.go` 中的认证逻辑
   - 扩展现有的认证切换机制（第497-530行）
   - 在成功后调用配置更新

### 第三阶段：UI展示（优先级低）

1. ⚠️  更新前端测试界面，显示认证学习状态
2. ⚠️  添加手动触发认证学习的按钮

## 配置示例

### 使用auto模式

```yaml
endpoints:
  - name: test-endpoint
    url_anthropic: https://api.provider.com
    auth_type: auto  # 或留空
    auth_value: sk-xxxxx
    enabled: true
```

### 学习后自动更新为

```yaml
endpoints:
  - name: test-endpoint
    url_anthropic: https://api.provider.com
    auth_type: api_key  # 自动学习并更新
    auth_value: sk-xxxxx
    enabled: true
```

## 日志输出示例

```
[INFO] Testing endpoint 'test-endpoint' with auth_type='auto'
[INFO] Trying authentication with x-api-key header
[INFO] ✓ Authentication successful with api_key method
[INFO] ✓ Auto-learned and saved auth type 'api_key' for endpoint 'test-endpoint'
[INFO] Config file updated: /path/to/config.yaml
```

## 安全考虑

1. **配置备份**：更新配置前自动备份
2. **回滚机制**：保存失败时回滚内存中的更改
3. **并发安全**：使用锁保护配置文件写入
4. **验证机制**：保存后重新验证配置的有效性

## 测试用例

### 单元测试

```go
func TestUpdateAuthType(t *testing.T) {
    // 测试认证类型更新
    // 测试配置文件保存
    // 测试回滚机制
}

func TestAuthTypeDetection(t *testing.T) {
    // 测试auto模式下的认证探测
    // 测试不同格式的认证方式选择
}
```

### 集成测试

```bash
# 1. 配置一个auto模式端点
# 2. 运行测试
# 3. 验证配置文件已更新
# 4. 验证下次请求使用学习到的认证方式
```

## 下一步行动

1. ⚠️  **立即执行**：修改 `endpoint_testing.go`，集成认证探测
2. ⚠️  **本周完成**：修改 `proxy_logic.go`，运行时学习
3. ⚠️  **下周完成**：添加UI展示和手动触发功能
4. ⚠️  **测试验证**：编写完整的测试用例

## 状态追踪

- [x] 需求分析和方案设计
- [x] 创建配置持久化模块
- [ ] 修改测试场景代码
- [ ] 修改运行时场景代码
- [ ] 添加UI展示
- [ ] 编写测试用例
- [ ] 文档更新

---

**创建时间**: 2025-10-06
**最后更新**: 2025-10-06
**负责人**: CCCC开发团队
