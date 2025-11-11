package interfaces

import (
	"net/http"
	"time"
)

// Tagger 是所有tagger（内置和Starlark）必须实现的接口
type Tagger interface {
	Name() string
	Tag() string
	ShouldTag(request *http.Request) (bool, error)
}

// TaggerResult 记录单个tagger的执行结果
type TaggerResult struct {
	TaggerName string
	Tag        string
	Matched    bool
	Error      error
	Duration   time.Duration
}

// TaggedRequest 包含已标记tag的请求信息
type TaggedRequest struct {
	OriginalRequest *http.Request
	Tags           []string
	TaggingTime    time.Time
	TaggerResults  []TaggerResult
}

// TaggedEndpoint 包含tags信息的endpoint
type TaggedEndpoint struct {
	Name     string
	URL      string
	Tags     []string
	Priority int
	Enabled  bool
}

// Tag 表示一个标签的基本信息
type Tag struct {
	Name        string
	Description string
}