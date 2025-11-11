package starlark

import (
	"fmt"
	"net/http"
	"time"
)

// Tagger 实现基于Starlark脚本的tagger
type Tagger struct {
	name     string
	tag      string
	executor *Executor
	enabled  bool
}

// NewTagger 创建新的Starlark tagger
func NewTagger(name, tag, script string, timeout time.Duration) *Tagger {
	return &Tagger{
		name:     name,
		tag:      tag,
		executor: NewExecutor(name, script, timeout),
		enabled:  true,
	}
}

// Name 返回tagger名称
func (t *Tagger) Name() string {
	return t.name
}

// Tag 返回tagger产生的tag
func (t *Tagger) Tag() string {
	return t.tag
}

// ShouldTag 执行Starlark脚本判断是否应该添加tag
func (t *Tagger) ShouldTag(request *http.Request) (bool, error) {
	if !t.enabled {
		return false, nil
	}
	
	shouldTag, err := t.executor.ExecuteScript(request)
	if err != nil {
		return false, fmt.Errorf("starlark tagger %s failed: %w", t.name, err)
	}
	
	return shouldTag, nil
}

// SetEnabled 设置tagger启用状态
func (t *Tagger) SetEnabled(enabled bool) {
	t.enabled = enabled
}

// IsEnabled 返回tagger启用状态
func (t *Tagger) IsEnabled() bool {
	return t.enabled
}