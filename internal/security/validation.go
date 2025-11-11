package security

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// SanitizeInput 净化用户输入，检查长度、恶意脚本等
func SanitizeInput(input string, maxLength int) (string, error) {
	// 长度检查
	if utf8.RuneCountInString(input) > maxLength {
		return "", fmt.Errorf("输入长度超过限制：%d", maxLength)
	}

	// 检查恶意脚本标签
	scriptPattern := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	if scriptPattern.MatchString(input) {
		return "", fmt.Errorf("输入包含不允许的脚本标签")
	}

	// 检查javascript:协议
	if strings.Contains(strings.ToLower(input), "javascript:") {
		return "", fmt.Errorf("输入包含不允许的javascript协议")
	}

	// 检查其他危险的HTML标签
	dangerousTags := []string{"<iframe", "<object", "<embed", "<form", "<input", "<meta"}
	lowerInput := strings.ToLower(input)
	for _, tag := range dangerousTags {
		if strings.Contains(lowerInput, tag) {
			return "", fmt.Errorf("输入包含不允许的HTML标签")
		}
	}

	return input, nil
}

// ValidateEndpointName 验证端点名称
func ValidateEndpointName(name string) error {
	if name == "" {
		return fmt.Errorf("端点名称不能为空")
	}

	if _, err := SanitizeInput(name, 100); err != nil {
		return fmt.Errorf("端点名称验证失败: %v", err)
	}

	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("端点名称不能包含 '/' 或 '\\'")
	}

	if strings.ContainsAny(name, "<>&\"'") {
		return fmt.Errorf("端点名称包含不允许的特殊字符")
	}

	return nil
}

// ValidateTags 验证标签列表
func ValidateTags(tags []string) error {
	if len(tags) > 20 {
		return fmt.Errorf("标签数量超过限制（最多20个）")
	}

	for _, tag := range tags {
		if tag == "" {
			continue // 跳过空标签
		}

		if _, err := SanitizeInput(tag, 50); err != nil {
			return fmt.Errorf("标签 '%s' 验证失败: %v", tag, err)
		}

		// 检查标签中的特殊字符
		if strings.ContainsAny(tag, "<>&\"'") {
			return fmt.Errorf("标签 '%s' 包含不允许的特殊字符", tag)
		}
	}

	return nil
}

// ValidateURL 验证URL格式和安全性
func ValidateURL(url string) error {
	if url == "" {
		return fmt.Errorf("URL不能为空")
	}

	if _, err := SanitizeInput(url, 500); err != nil {
		return fmt.Errorf("URL验证失败: %v", err)
	}

	// 检查协议
	lowerURL := strings.ToLower(url)
	if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
		return fmt.Errorf("URL必须使用http或https协议")
	}

	// 检查恶意协议
	if strings.Contains(lowerURL, "javascript:") || strings.Contains(lowerURL, "data:") {
		return fmt.Errorf("URL包含不允许的协议")
	}

	return nil
}

// ValidateModelName 验证模型名称
func ValidateModelName(name string) error {
	if name == "" {
		return nil // 允许为空
	}

	if _, err := SanitizeInput(name, 200); err != nil {
		return fmt.Errorf("模型名称验证失败: %v", err)
	}

	// 模型名称应该只包含字母、数字、连字符、下划线、点和冒号
	validModelName := regexp.MustCompile(`^[a-zA-Z0-9\-_.:]+$`)
	if !validModelName.MatchString(name) {
		return fmt.Errorf("模型名称包含不允许的字符")
	}

	return nil
}

// ValidateAuthToken 验证认证令牌
func ValidateAuthToken(token string) error {
	if token == "" {
		return fmt.Errorf("认证令牌不能为空")
	}

	if _, err := SanitizeInput(token, 1000); err != nil {
		return fmt.Errorf("认证令牌验证失败: %v", err)
	}

	// 检查令牌中不应该包含的字符
	if strings.ContainsAny(token, "<>&\"'") {
		return fmt.Errorf("认证令牌包含不允许的字符")
	}

	return nil
}

// ValidatePatternString 验证通配符模式字符串
func ValidatePatternString(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("模式不能为空")
	}

	if _, err := SanitizeInput(pattern, 200); err != nil {
		return fmt.Errorf("模式验证失败: %v", err)
	}

	// 通配符模式应该只包含字母、数字、连字符、下划线、点、星号和冒号
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9\-_.:*]+$`)
	if !validPattern.MatchString(pattern) {
		return fmt.Errorf("模式包含不允许的字符")
	}

	return nil
}

// ValidateLogDays 验证日志保留天数
func ValidateLogDays(days int) error {
	if days < 0 {
		return fmt.Errorf("日志保留天数不能为负数")
	}

	if days > 365 {
		return fmt.Errorf("日志保留天数不能超过365天")
	}

	return nil
}

// ValidateGenericText 验证通用文本输入
func ValidateGenericText(text string, maxLength int, fieldName string) error {
	if text == "" {
		return nil // 允许为空
	}

	if _, err := SanitizeInput(text, maxLength); err != nil {
		return fmt.Errorf("%s验证失败: %v", fieldName, err)
	}

	return nil
}
