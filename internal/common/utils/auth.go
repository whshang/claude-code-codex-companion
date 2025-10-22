package utils

import "encoding/base64"

// BasicAuth 根据 user:password 生成 Basic Auth header 的值（不包含 "Basic " 前缀）。
func BasicAuth(userInfo string) string {
	if userInfo == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(userInfo))
}

