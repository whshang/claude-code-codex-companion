package proxy

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// cache.go: 缓存管理模块
// 负责处理请求处理过程中的所有缓存逻辑，包括：
// - 请求体转换结果的缓存
// - 模型重写结果的缓存
//
// 目标：
// - 将所有与 requestProcessingCache 相关的结构体和函数集中到此文件。
// - 提供清晰的缓存读写接口。
// - 减少核心逻辑对缓存实现的直接依赖。

const requestProcessingCacheKey = "request_processing_cache"

type cachedConversion struct {
	body []byte
	err  error
}

type modelRewriteCache struct {
	originalModel  string
	rewrittenModel string
	body           []byte
}

type requestProcessingCache struct {
	originalBody  []byte
	conversions   map[string]cachedConversion
	modelRewrites map[string]*modelRewriteCache
}

// getRequestProcessingCache retrieves or creates a request-scoped cache from the gin.Context.
func getRequestProcessingCache(c *gin.Context, originalBody []byte) *requestProcessingCache {
	if val, exists := c.Get(requestProcessingCacheKey); exists {
		if cache, ok := val.(*requestProcessingCache); ok && cache != nil {
			return cache
		}
	}
	cache := &requestProcessingCache{
		originalBody:  originalBody,
		conversions:   make(map[string]cachedConversion),
		modelRewrites: make(map[string]*modelRewriteCache),
	}
	c.Set(requestProcessingCacheKey, cache)
	return cache
}

func (rc *requestProcessingCache) conversionMapKey(key string, body []byte) string {
	sum := md5.Sum(body)
	return key + ":" + hex.EncodeToString(sum[:])
}

func (rc *requestProcessingCache) GetConvertedBody(key string, body []byte, converter func([]byte) ([]byte, error)) ([]byte, error) {
	if rc.conversions == nil {
		rc.conversions = make(map[string]cachedConversion)
	}
	mapKey := rc.conversionMapKey(key, body)
	if cached, ok := rc.conversions[mapKey]; ok {
		return cached.body, cached.err
	}
	converted, err := converter(body)
	rc.conversions[mapKey] = cachedConversion{body: converted, err: err}
	return converted, err
}

func (rc *requestProcessingCache) StoreModelRewrite(endpointName string, body []byte, originalModel, rewrittenModel string) {
	if rc.modelRewrites == nil {
		rc.modelRewrites = make(map[string]*modelRewriteCache)
	}
	var bodyCopy []byte
	if len(body) > 0 {
		bodyCopy = make([]byte, len(body))
		copy(bodyCopy, body)
	}
	rc.modelRewrites[endpointName] = &modelRewriteCache{
		originalModel:  originalModel,
		rewrittenModel: rewrittenModel,
		body:           bodyCopy,
	}
}

func (rc *requestProcessingCache) GetModelRewrite(endpointName string) (*modelRewriteCache, bool) {
	if rc.modelRewrites == nil {
		return nil, false
	}
	entry, ok := rc.modelRewrites[endpointName]
	return entry, ok
}