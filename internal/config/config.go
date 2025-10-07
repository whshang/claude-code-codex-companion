package config

// 实现 EndpointConfig 接口，用于统一验证
func (e EndpointConfig) GetName() string     { return e.Name }
func (e EndpointConfig) GetURL() string {
	// 优先返回Anthropic URL，如果为空则返回OpenAI URL
	if e.URLAnthropic != "" {
		return e.URLAnthropic
	}
	return e.URLOpenAI
}
func (e EndpointConfig) GetAuthType() string { return e.AuthType }
func (e EndpointConfig) GetAuthValue() string { return e.AuthValue }