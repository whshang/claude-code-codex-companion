# 88code 端点示例 | 88code Endpoint Example

## 中文

88code 提供同时兼容 Claude Code 与 Codex 的国内镜像。推荐通过脚本生成配置并在 CCCC 中添加备用端点。

### 配置步骤

1. 获取 88code API Key。  
2. 运行 `./cccc-setup-claude-code.sh --url https://www.88code.org/api --key <API_KEY>` 和 `./cccc-setup-codex.sh --url https://www.88code.org/openai/v1 --key <API_KEY>`。  
3. 在 `config.yaml` 中添加端点：  
   ```yaml
   - name: 88code
     url_anthropic: https://www.88code.org/api
     url_openai: https://www.88code.org/openai/v1
     auth_type: auth_token
     auth_value: YOUR_API_KEY
     supports_responses: false
     tags: ["community", "backup"]
   ```

### 注意事项

- 建议设置较低优先级，并关注返回的速率限制头。  
- 若启用 `count_tokens`，请先在探针工具中确认兼容性。  

## English

88code offers a mainland-accessible mirror compatible with both Claude Code and Codex. Configure it as a fallback endpoint in CCCC.

### Setup Steps

1. Obtain the 88code API key.  
2. Run `./cccc-setup-claude-code.sh --url https://www.88code.org/api --key <API_KEY>` and `./cccc-setup-codex.sh --url https://www.88code.org/openai/v1 --key <API_KEY>`.  
3. Add the endpoint to `config.yaml`:  
   ```yaml
   - name: 88code
     url_anthropic: https://www.88code.org/api
     url_openai: https://www.88code.org/openai/v1
     auth_type: auth_token
     auth_value: YOUR_API_KEY
     supports_responses: false
     tags: ["community", "backup"]
   ```

### Notes

- Keep the priority lower and monitor rate-limit headers.  
- Validate `count_tokens` behaviour with the probe before enabling globally.  
