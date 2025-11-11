package builtin

import (
	"fmt"
	"net/http"
	"strings"
)

// RequestAnalyzer 请求分析器接口，定义公共的请求处理逻辑
type RequestAnalyzer interface {
	// IsJSONContent 检查是否为JSON内容类型
	IsJSONContent(request *http.Request) bool
	
	// GetCachedBody 从请求上下文获取缓存的请求体
	GetCachedBody(request *http.Request) ([]byte, error)
	
	// ExtractJSONValue 从JSON数据中提取指定路径的值
	ExtractJSONValue(data map[string]interface{}, path string) (interface{}, error)
}

// BaseRequestAnalyzer 基础请求分析器，提供公共实现
type BaseRequestAnalyzer struct{}

// NewBaseRequestAnalyzer 创建基础请求分析器
func NewBaseRequestAnalyzer() *BaseRequestAnalyzer {
	return &BaseRequestAnalyzer{}
}

// IsJSONContent 检查是否为JSON内容类型
func (ba *BaseRequestAnalyzer) IsJSONContent(request *http.Request) bool {
	contentType := request.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// GetCachedBody 从请求上下文获取缓存的请求体
func (ba *BaseRequestAnalyzer) GetCachedBody(request *http.Request) ([]byte, error) {
	bodyContent, ok := request.Context().Value("cached_body").([]byte)
	if !ok || len(bodyContent) == 0 {
		return nil, fmt.Errorf("no cached body found in request context")
	}
	return bodyContent, nil
}

// ExtractJSONValue 从JSON数据中提取指定路径的值
func (ba *BaseRequestAnalyzer) ExtractJSONValue(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分，返回值
			return current[part], nil
		}

		// 中间部分，继续深入
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, fmt.Errorf("path '%s' not found in JSON data", strings.Join(parts[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("invalid path: %s", path)
}