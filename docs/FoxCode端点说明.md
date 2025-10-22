# FoxCode 端点说明 | FoxCode Endpoint Notes

## 中文

FoxCode 提供国内可用的 Claude Code 镜像，可通过脚本快速生成配置文件或手动设置 `ANTHROPIC_BASE_URL`。

### 推荐做法

1. 使用 `/help` 页面下载 `cccc-setup-claude-code.sh`，指定 `https://code.newcli.com/claude`。  
2. 生成或更新 `~/.claude/settings.json`：  
   ```json
   {
     "env": {
       "ANTHROPIC_BASE_URL": "https://code.newcli.com/claude",
       "ANTHROPIC_AUTH_TOKEN": "YOUR_API_KEY",
       "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": 1
     }
   }
   ```
3. 在 CCCC 中添加端点，并设置 `supports_responses: true` 以直接透传 `/v1/messages`。  

### 备注

- 支持 VS Code、Cursor、JetBrains 系列等 IDE。  
- 建议在标签路由中为 FoxCode 单独设定标签，以便与其他号池区分。  

## English

FoxCode provides a mainland-accessible mirror for Claude Code. Use the helper script or edit `settings.json` to point traffic through CCCC.

### Recommended Steps

1. Download `cccc-setup-claude-code.sh` from `/help` and set `https://code.newcli.com/claude`.  
2. Update `~/.claude/settings.json`:  
   ```json
   {
     "env": {
       "ANTHROPIC_BASE_URL": "https://code.newcli.com/claude",
       "ANTHROPIC_AUTH_TOKEN": "YOUR_API_KEY",
       "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": 1
     }
   }
   ```
3. Add the endpoint to CCCC and set `supports_responses: true` so `/v1/messages` can be passed through.  

### Notes

- Compatible with VS Code, Cursor, and JetBrains IDEs.  
- Consider assigning a dedicated tag for FoxCode in the routing configuration to separate it from other pools.  
