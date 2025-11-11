package validator

import "strings"

// ClientValidationKeywords 客户端验证失败的关键字组合
// 采用关键字提取而非严格匹配，提高识别灵活性
var ClientValidationKeywords = [][]string{
	// 中文关键字组合（含义：禁止在客户端之外调用）
	{"请勿", "之外"},              // "请勿在XX之外使用"
	{"暂不支持", "非"},             // "暂不支持非XX请求"
	{"不支持", "非"},              // "不支持非XX"
	{"仅支持", "客户端"},           // "仅支持XX客户端"
	{"仅", "官方"},                // "仅官方客户端"
	{"禁止", "之外"},              // "禁止在XX之外"
	{"限制", "客户端"},            // "限制客户端访问"
	{"拒绝", "客户端"},            // "拒绝客户端"

	// 英文关键字组合
	{"only", "cli"},              // "only claude code cli"
	{"only", "client"},           // "only official client"
	{"unauthorized", "client"},   // "unauthorized client"
	{"invalid", "client"},        // "invalid client"
	{"client", "not allowed"},    // "client not allowed"
	{"client", "verification"},   // "client verification failed"
	{"client", "required"},       // "client authentication required"
	{"forbidden", "client"},      // "forbidden for this client"

	// HTML错误页关键字
	{"403", "forbidden"},
	{"404", "not found"},
	{"access", "denied"},
}

// HTMLResponsePatterns HTML响应的特征（通常是403/404错误页）
var HTMLResponsePatterns = []string{
	"<!DOCTYPE",
	"<!doctype",
	"<html",
	"<HTML",
	"<head>",
	"<HEAD>",
	"<body>",
	"<BODY>",
}

// containsAllKeywords 检查字符串是否包含所有关键字（不区分大小写）
func containsAllKeywords(text string, keywords []string) bool {
	lowerText := strings.ToLower(text)
	for _, keyword := range keywords {
		if !strings.Contains(lowerText, strings.ToLower(keyword)) {
			return false
		}
	}
	return true
}

// IsClientValidationFailure 检查错误消息是否为客户端验证失败
// 采用灵活的关键字匹配，而非严格的字符串匹配
func IsClientValidationFailure(errorMsg string, statusCode int) bool {
	if errorMsg == "" {
		return false
	}

	lowerMsg := strings.ToLower(errorMsg)

	// 1. 检查客户端限制关键字组合
	for _, keywords := range ClientValidationKeywords {
		if containsAllKeywords(errorMsg, keywords) {
			return true
		}
	}

	// 2. 检查 JSON 解析错误 + HTML 响应特征
	// 通常是返回了 HTML 403/404 页面而非 JSON
	if strings.Contains(lowerMsg, "invalid json") ||
	   strings.Contains(lowerMsg, "json parse") ||
	   strings.Contains(lowerMsg, "invalid character") {
		for _, htmlPattern := range HTMLResponsePatterns {
			if strings.Contains(errorMsg, htmlPattern) ||
			   strings.Contains(lowerMsg, strings.ToLower(htmlPattern)) {
				// JSON解析错误 + HTML内容 = 可能是403/404页面
				return true
			}
		}

		// 特殊情况：invalid character '<' 或 'd' 在开头，通常是HTML
		if strings.Contains(lowerMsg, "invalid character '<'") ||
		   strings.Contains(lowerMsg, "invalid character 'd'") {
			return true
		}
	}

	// 3. 特定HTTP状态码 + 空响应或HTML
	if statusCode == 403 {
		// 403 + 包含HTML标签 = 客户端验证失败
		for _, htmlPattern := range HTMLResponsePatterns {
			if strings.Contains(errorMsg, htmlPattern) {
				return true
			}
		}
	}
	// 🔧 404错误表示路径不存在，不是客户端验证失败
	// if statusCode == 404 {
	//     // 404 + 包含HTML标签 = 客户端验证失败
	//     for _, htmlPattern := range HTMLResponsePatterns {
	//         if strings.Contains(errorMsg, htmlPattern) {
	//             return true
	//         }
	//     }
	// }

	return false
}

// CheckClientValidation 检查响应是否为客户端验证失败
// 返回 true 表示这是客户端验证错误（端点正常，但拒绝测试）
func CheckClientValidation(statusCode int, responseBody string, errorMsg string) bool {
	// 检查错误消息
	if IsClientValidationFailure(errorMsg, statusCode) {
		return true
	}

	// 检查响应体
	if IsClientValidationFailure(responseBody, statusCode) {
		return true
	}

	return false
}
