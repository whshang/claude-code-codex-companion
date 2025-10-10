# 端点向导 | Endpoint Wizard

## 中文

端点向导通过预设模板简化新端点的添加流程。用户选择模板、填写密钥与标签后，即可生成并保存配置，支持自定义 `endpoint_profiles.yaml`。

### 工作流程

1. 选择预设 Profile（如 Anthropic、OpenAI、社区供应商）。  
2. 填写认证信息、可选标签与优先级。  
3. 预览生成的 YAML，确认后写入 `config.yaml` 并可立即执行连通性测试。  

### 预设结构

```yaml
profiles:
  - profile_id: anthropic
    display_name: "Anthropic (Claude)"
    endpoint_type: anthropic
    url_anthropic: https://api.anthropic.com
    auth_type: api_key
    path_prefix: /v1/messages
```

- 支持从电子表格导出后转换为 YAML。  
- 可附加 `default_model_options`、`supports_responses` 等字段，给使用者提供提示。  

### 集成说明

- 前端文件：`web/static/endpoint-wizard.js`、`web/templates/endpoints-modal.html`。  
- 提交后调用 `/admin/api/endpoints` 保存，并可自动触发端点连通性测试。  
- 建议结合标签路由与优先级，避免新增端点覆盖现有生产流量。  

## English

The endpoint wizard streamlines creating new endpoints with predefined templates. Pick a profile, supply secrets/tags, and generate configuration stored in `endpoint_profiles.yaml`.

### Workflow

1. Choose a profile (Anthropic, OpenAI, community providers, etc.).  
2. Enter authentication details, optional tags, and priority.  
3. Preview the generated YAML, save it into `config.yaml`, and run a connectivity test immediately.  

### Profile Structure

```yaml
profiles:
  - profile_id: anthropic
    display_name: "Anthropic (Claude)"
    endpoint_type: anthropic
    url_anthropic: https://api.anthropic.com
    auth_type: api_key
    path_prefix: /v1/messages
```

- Profiles can be exported from spreadsheets and converted to YAML.  
- Additional hints such as `default_model_options` or `supports_responses` can assist operators.  

### Integration

- Frontend modules: `web/static/endpoint-wizard.js`, `web/templates/endpoints-modal.html`.  
- Submissions call `/admin/api/endpoints` and may trigger a connectivity test.  
- Combine with tag routing and priorities to prevent new endpoints from hijacking production traffic.  
