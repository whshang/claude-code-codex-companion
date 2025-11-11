package builtin

import (
	"fmt"

	"claude-code-codex-companion/internal/interfaces"
)

// TaggerFactory 内置tagger工厂函数类型
type TaggerFactory func(name, tag string, config map[string]interface{}) (interfaces.Tagger, error)

// BuiltinTaggerFactory 内置tagger工厂
type BuiltinTaggerFactory struct {
	creators map[string]TaggerFactory
}

// NewBuiltinTaggerFactory 创建内置tagger工厂
func NewBuiltinTaggerFactory() *BuiltinTaggerFactory {
	factory := &BuiltinTaggerFactory{
		creators: make(map[string]TaggerFactory),
	}

	// 注册所有内置tagger类型
	factory.Register("path", NewPathTagger)
	factory.Register("header", NewHeaderTagger)
	factory.Register("query", NewQueryTagger)
	factory.Register("body-json", NewBodyJSONTagger)
	factory.Register("user-message", NewUserMessageTagger)
	factory.Register("model", NewModelTagger)
	factory.Register("thinking", NewThinkingTagger)

	return factory
}

// Register 注册一个新的内置tagger类型
func (f *BuiltinTaggerFactory) Register(taggerType string, factory TaggerFactory) {
	f.creators[taggerType] = factory
}

// CreateTagger 创建指定类型的内置tagger
func (f *BuiltinTaggerFactory) CreateTagger(taggerType, name, tag string, config map[string]interface{}) (interfaces.Tagger, error) {
	creator, exists := f.creators[taggerType]
	if !exists {
		return nil, fmt.Errorf("unknown builtin tagger type: %s", taggerType)
	}

	return creator(name, tag, config)
}

// ListSupportedTypes 列出所有支持的内置tagger类型
func (f *BuiltinTaggerFactory) ListSupportedTypes() []string {
	types := make([]string, 0, len(f.creators))
	for taggerType := range f.creators {
		types = append(types, taggerType)
	}
	return types
}

// IsSupported 检查指定类型是否被支持
func (f *BuiltinTaggerFactory) IsSupported(taggerType string) bool {
	_, exists := f.creators[taggerType]
	return exists
}