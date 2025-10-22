# Proxy Logic 重构任务清单

## 📋 项目概述

### 背景
`internal/proxy/proxy_logic.go` 文件已达到 3494 行，包含 47 个函数，严重违反了单一职责原则和代码可维护性要求。该文件涵盖了缓存管理、核心代理逻辑、流式处理、参数处理、工具调用、模型列表、错误处理等多个功能模块。

### 目标
将 3494 行的巨型文件重构为 8 个专门模块，每个模块控制在 300-800 行以内，提升代码的可维护性、可读性和可测试性。

### 预期收益
- **可维护性**: 每个文件职责单一，易于理解和修改
- **可测试性**: 小模块易于编写单元测试
- **开发效率**: 多个开发者可并行工作于不同模块
- **代码质量**: 降低复杂度，减少bug引入概率

## 🎯 重构范围

### 当前文件状态
- **文件**: `internal/proxy/proxy_logic.go`
- **行数**: 3494 行
- **函数数量**: 47 个
- **主要功能**:
  - 请求处理缓存管理
  - 核心代理转发逻辑
  - 流式响应处理
  - 参数覆盖和hack处理
  - 工具调用检测转换
  - 模型列表API处理
  - 错误学习和自适应

### 依赖分析
- **调用者**:
  - `handler.go`: 调用 `handleModelsList`
  - `endpoint_management.go`: 调用 `proxyToEndpoint`
- **依赖的包**:
  - `internal/conversion`: 格式转换
  - `internal/endpoint`: 端点管理
  - `internal/tagging`: 标签系统
  - `internal/utils`: 工具函数
  - `internal/validator`: 响应验证
  - `github.com/gin-gonic/gin`: Web框架

## 📁 目标文件结构

```
internal/proxy/
├── proxy_logic.go         # 原文件（重构完成后删除）
├── cache.go              # (~300行) 缓存管理模块
├── core.go               # (~800行) 核心代理逻辑
├── streaming.go          # (~600行) 流式处理模块
├── parameters.go         # (~400行) 参数处理模块
├── tools.go              # (~300行) 工具调用模块
├── models.go             # (~500行) 模型列表模块
├── errors.go             # (~400行) 错误处理模块
└── utils.go              # (~300行) 工具类模块
```

## 🔧 详细重构计划

### 阶段1: 准备工作 (1-2天)

#### [ ] 1.1 创建新文件结构
- [ ] 创建上述8个新文件的基本结构
- [ ] 添加适当的包声明和导入
- [ ] 编写文件头注释说明模块职责

#### [ ] 1.2 备份和分支管理
- [ ] 创建重构专用分支 `refactor/proxy-logic`
- [ ] 备份当前 `proxy_logic.go` 文件
- [ ] 设置CI/CD以监控重构过程

#### [ ] 1.3 分析和文档化
- [ ] 分析所有函数间的依赖关系
- [ ] 识别共享数据结构和常量
- [ ] 编写迁移映射表（哪个函数迁移到哪个文件）

### 阶段2: 安全迁移 (3-5天)

#### [ ] 2.1 迁移 utils.go (工具类模块)
**目标**: 无依赖的工具函数，最安全迁移
**内容**:
- [ ] `teeCaptureWriter` 结构体及方法
- [ ] `limitedBuffer` 结构体及方法
- [ ] `addConversionStage`, `setConversionContext`
- [ ] `getSupportsResponsesFlag`, `updateSupportsResponsesContext`
- [ ] `min`, `ensureOpenAIStreamTrue` 等工具函数
**验证**: 编译通过，无功能变更

#### [ ] 2.2 迁移 cache.go (缓存管理模块)
**目标**: 请求处理缓存逻辑
**内容**:
- [ ] `cachedConversion`, `modelRewriteCache`, `requestProcessingCache` 结构体
- [ ] `getRequestProcessingCache` 函数
- [ ] 所有缓存相关的方法
**验证**: 缓存功能正常工作

#### [ ] 2.3 迁移 errors.go (错误处理模块)
**目标**: 错误学习和自适应逻辑
**内容**:
- [ ] `autoRemoveUnsupportedParams` 函数
- [ ] `learnUnsupportedParamsFromError` 函数
- [ ] `shouldMarkResponsesUnsupported`, `containsResponsesUnsupportedHint`
**验证**: 错误处理逻辑正确

#### [ ] 2.4 迁移 parameters.go (参数处理模块)
**目标**: 参数覆盖和hack处理
**内容**:
- [ ] `applyParameterOverrides` 函数
- [ ] `applyOpenAIUserLengthHack`, `applyGPT5ModelHack`
- [ ] `processRateLimitHeaders` 函数
**验证**: 参数覆盖功能正常

### 阶段3: 功能迁移 (4-6天)

#### [ ] 3.1 迁移 tools.go (工具调用模块)
**目标**: 工具调用检测和转换
**内容**:
- [ ] `requestHasTools` 函数
- [ ] `convertChatCompletionsToResponsesSSE`
- [ ] `convertChatCompletionToResponse`
- [ ] `convertCodexToOpenAI`, `updateEndpointCodexSupport`
**验证**: 工具调用转换正确

#### [ ] 3.2 迁移 models.go (模型列表模块)
**目标**: 模型列表API处理（已有调用者，最优先）
**内容**:
- [ ] `handleModelsList` 函数
- [ ] `detectModelsClientFormat`, `selectEndpointForModels`
- [ ] `convertModelsResponse` 及所有格式转换函数
**验证**: 模型列表API正常响应

#### [ ] 3.3 迁移 streaming.go (流式处理模块)
**目标**: 流式响应处理逻辑
**内容**:
- [ ] `handleStreamingResponse` 函数
- [ ] `handleStreamingConversion`, `convertStreamingResponse`
- [ ] `getExpectedFormatForClient` 函数
**验证**: 流式响应正确处理

#### [ ] 3.4 重构 core.go (核心代理逻辑)
**目标**: 拆分超大的 `proxyToEndpoint` 函数
**内容**:
- [ ] 将 `proxyToEndpoint` 拆分为多个小函数：
  - [ ] `prepareRequest` - 请求准备
  - [ ] `executeRequest` - 请求执行
  - [ ] `handleResponse` - 响应处理
  - [ ] `finalizeRequest` - 请求完成
- [ ] 重构后的 `proxyToEndpoint` 作为协调器
**验证**: 核心代理功能正常

### 阶段4: 清理和优化 (2-3天)

#### [ ] 4.1 删除原文件
- [ ] 确认所有功能已迁移
- [ ] 删除 `proxy_logic.go` 文件
- [ ] 清理不再使用的导入

#### [ ] 4.2 代码优化
- [ ] 检查并修复循环依赖
- [ ] 优化导入语句
- [ ] 添加必要的注释和文档

#### [ ] 4.3 接口抽象 (可选优化)
- [ ] 为各模块定义接口
- [ ] 实现依赖注入
- [ ] 支持插件化扩展

### 阶段5: 测试和验证 (3-5天)

#### [ ] 5.1 单元测试
- [ ] 为每个新模块编写单元测试
- [ ] 覆盖率目标: >80%
- [ ] 重点测试边界条件和错误处理

#### [ ] 5.2 集成测试
- [ ] 端到端测试代理功能
- [ ] 性能测试确保无性能下降
- [ ] 压力测试验证稳定性

#### [ ] 5.3 回归测试
- [ ] 运行现有测试套件
- [ ] 验证所有API接口正常工作
- [ ] 检查日志和监控指标

### 阶段6: 文档和培训 (1-2天)

#### [ ] 6.1 更新文档
- [ ] 更新代码注释
- [ ] 编写各模块的README
- [ ] 更新架构文档

#### [ ] 6.2 团队培训
- [ ] 组织代码审查会议
- [ ] 分享重构经验教训
- [ ] 建立维护指南

## 🛡️ 风险控制

### 技术风险
- **向后兼容性**: 保持Server接口不变，逐步迁移
- **性能影响**: 监控重构前后性能指标
- **功能正确性**: 充分的测试覆盖

### 实施风险
- **时间估算**: 预留buffer时间处理意外情况
- **团队协调**: 明确各阶段负责人
- **回滚计划**: 准备快速回滚到原始版本的方案

## 📊 进度跟踪

### 里程碑
- [ ] M1: 准备工作完成 (第2天)
- [ ] M2: 安全迁移完成 (第7天)
- [ ] M3: 功能迁移完成 (第13天)
- [ ] M4: 清理优化完成 (第16天)
- [ ] M5: 测试验证完成 (第21天)
- [ ] M6: 项目完成 (第23天)

### 关键指标
- **代码行数**: 每个文件 <1000行
- **测试覆盖率**: >80%
- **性能基准**: 不低于重构前
- **零功能回归**: 所有现有功能正常

## 📞 联系和支持

### 负责人
- **技术负责人**: [姓名]
- **测试负责人**: [姓名]
- **产品负责人**: [姓名]

### 沟通渠道
- **日报**: 每日重构进展同步
- **周会**: 每周进度review和问题解决
- **紧急联系**: 发现重大问题立即通知

---

**最后更新**: 2025年10月21日
**版本**: v1.0
**状态**: 设计阶段</content>
</xai:function_call">```
