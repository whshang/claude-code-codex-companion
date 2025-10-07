package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"claude-code-codex-companion/internal/i18n"
)

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，生成默认配置文件
			if err := generateDefaultConfig(filename); err != nil {
				return nil, fmt.Errorf("failed to generate default config file: %v", err)
			}
			// 重新读取生成的配置文件
			data, err = os.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("failed to read generated config file: %v", err)
			}
		} else {
			return nil, fmt.Errorf("failed to read config file: %v", err)
		}
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	return &config, nil
}

// generateDefaultConfig 生成默认配置文件
func generateDefaultConfig(filename string) error {
	defaultConfig := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Endpoints: []EndpointConfig{
			{
				Name:         "example-anthropic",
				URLAnthropic: "https://api.anthropic.com",
				AuthType:     "api_key",
				AuthValue:    "YOUR_ANTHROPIC_API_KEY_HERE",
				Enabled:      false, // 默认禁用，需要用户配置
				Priority:     1,
				Tags:         []string{},
			},
			{
				Name:         "example-openai",
				URLOpenAI:    "https://api.openai.com",
				AuthType:     "auth_token",
				AuthValue:    "YOUR_OPENAI_API_KEY_HERE",
				Enabled:      false, // 默认禁用，需要用户配置
				Priority:     2,
				Tags:         []string{},
			},
			{
				Name:         "example-anthropic-oauth",
				URLAnthropic: "https://api.anthropic.com",
				AuthType:     "oauth",
				Enabled:      false, // 默认禁用，需要用户配置
				Priority:     3,
				Tags:         []string{},
				OAuthConfig: &OAuthConfig{
					AccessToken:  "sk-ant-oat01-YOUR_ACCESS_TOKEN_HERE",
					RefreshToken: "sk-ant-ort01-YOUR_REFRESH_TOKEN_HERE",
					ExpiresAt:    1724924000000, // 示例时间戳，请设置为实际过期时间戳（毫秒）
					TokenURL:     "https://console.anthropic.com/v1/oauth/token",
					ClientID:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
					Scopes:       []string{"user:inference", "user:profile"},
					AutoRefresh:  true,
				},
			},
		},
		Logging: LoggingConfig{
			Level:           "info",
			LogRequestTypes: "failed",
			LogRequestBody:  "truncated",
			LogResponseBody: "truncated",
			LogDirectory:    "./logs",
		},
		Validation: ValidationConfig{},
		Tagging: TaggingConfig{
			PipelineTimeout: "5s",
			Taggers:         []TaggerConfig{},
		},
		Timeouts: TimeoutConfig{
			TLSHandshake:       "10s",
			ResponseHeader:     "60s", 
			IdleConnection:     "90s",
			HealthCheckTimeout: "30s",
			CheckInterval:      "30s",
		},
		Blacklist: BlacklistConfig{
			Enabled:            true,
			AutoBlacklist:      true,
			BusinessErrorSafe:  true,
			ConfigErrorSafe:    false,
			ServerErrorSafe:    false,
			SSEValidationSafe:  false,
		},
	}

	// 序列化为YAML
	data, err := yaml.Marshal(defaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %v", err)
	}

	// 添加注释说明
	header := i18n.T("default_config_header", `# Claude Code Codex Companion 默认配置文件
# 这是自动生成的默认配置文件，请根据需要修改各项配置
# 注意：endpoints 中的示例端点默认为禁用状态，需要配置正确的 API 密钥并启用

`)

	finalData := header + string(data)

	// 写入配置文件
	if err := os.WriteFile(filename, []byte(finalData), 0644); err != nil {
		return fmt.Errorf("failed to write default config file: %v", err)
	}

	fmt.Printf(i18n.T("default_config_generated", "默认配置文件已生成: %s\n"), filename)
	fmt.Println(i18n.T("config_edit_instruction", "请编辑配置文件，设置正确的端点信息和 API 密钥后重新启动服务"))

	return nil
}

func SaveConfig(config *Config, filename string) error {
	// 首先验证配置
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	// 序列化为YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// 创建备份文件
	if _, err := os.Stat(filename); err == nil {
		backupFilename := filename + ".backup"
		if err := os.Rename(filename, backupFilename); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
	}

	// 写入新配置
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}