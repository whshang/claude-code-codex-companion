package builtin

import (
	"net/http"
	
	jsonutils "claude-code-codex-companion/internal/common/json"
)

// EnhancedBaseTagger 增强的基础tagger，提供公共的请求处理功能
type EnhancedBaseTagger struct {
	BaseTagger
	analyzer RequestAnalyzer
}

// NewEnhancedBaseTagger 创建增强的基础tagger
func NewEnhancedBaseTagger(name, tag string) *EnhancedBaseTagger {
	return &EnhancedBaseTagger{
		BaseTagger: BaseTagger{name: name, tag: tag},
		analyzer:   NewBaseRequestAnalyzer(),
	}
}

// IsJSONContent 检查是否为JSON内容类型
func (et *EnhancedBaseTagger) IsJSONContent(request *http.Request) bool {
	return et.analyzer.IsJSONContent(request)
}

// GetCachedBody 从请求上下文获取缓存的请求体
func (et *EnhancedBaseTagger) GetCachedBody(request *http.Request) ([]byte, error) {
	return et.analyzer.GetCachedBody(request)
}

// ParseJSONBody 解析JSON请求体
func (et *EnhancedBaseTagger) ParseJSONBody(request *http.Request) (map[string]interface{}, error) {
	bodyContent, err := et.GetCachedBody(request)
	if err != nil {
		return nil, err
	}
	
	var jsonData map[string]interface{}
	if err := jsonutils.SafeUnmarshal(bodyContent, &jsonData); err != nil {
		return nil, err
	}
	
	return jsonData, nil
}

// ExtractJSONValue 从JSON数据中提取指定路径的值
func (et *EnhancedBaseTagger) ExtractJSONValue(data map[string]interface{}, path string) (interface{}, error) {
	return et.analyzer.ExtractJSONValue(data, path)
}

// ExtractStringValue 从JSON数据中提取字符串值
func (et *EnhancedBaseTagger) ExtractStringValue(data map[string]interface{}, path string) (string, error) {
	value, err := et.ExtractJSONValue(data, path)
	if err != nil {
		return "", err
	}
	
	if strValue, ok := value.(string); ok {
		return strValue, nil
	}
	
	return "", nil
}