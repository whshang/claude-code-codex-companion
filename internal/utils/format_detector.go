package utils

import (
	"encoding/json"
	"strings"
	"sync"
)

// RequestFormat represents the detected API format
type RequestFormat string

const (
	FormatAnthropic RequestFormat = "anthropic"
	FormatOpenAI    RequestFormat = "openai"
	FormatUnknown   RequestFormat = "unknown"
)

// ClientType represents the detected client type
type ClientType string

const (
ClientClaudeCode ClientType = "claude-code"
ClientCodex      ClientType = "codex"
ClientGemini     ClientType = "gemini"
	ClientUnknown    ClientType = "unknown"
)

// FormatDetectionResult contains the result of format detection
type FormatDetectionResult struct {
	Format      RequestFormat
	ClientType  ClientType
	Confidence  float64 // 0.0 - 1.0
	DetectedBy  string  // detection method used
}

// 简单的路径检测缓存，避免重复计算
var (
	pathDetectionCache = make(map[string]*FormatDetectionResult)
	cacheMutex         sync.RWMutex
	cacheMaxSize       = 1000 // 限制缓存大小，避免内存泄漏
	// LRU缓存用于更高效的缓存管理
	lruCache *LRUCache
)

// LRUCache LRU缓存实现
type LRUCache struct {
	items map[string]*cacheItem
	head  *cacheItem
	tail  *cacheItem
	max   int
}

type cacheItem struct {
	key   string
	value *FormatDetectionResult
	prev  *cacheItem
	next  *cacheItem
}

func NewLRUCache(max int) *LRUCache {
	return &LRUCache{
		items: make(map[string]*cacheItem),
		head:  nil,
		tail:  nil,
		max:   max,
	}
}

func (l *LRUCache) Get(key string) (*FormatDetectionResult, bool) {
	if item, exists := l.items[key]; exists {
		// 移动到头部
		l.moveToHead(item)
		return item.value, true
	}
	return nil, false
}

func (l *LRUCache) Put(key string, value *FormatDetectionResult) {
	if item, exists := l.items[key]; exists {
		item.value = value
		l.moveToHead(item)
		return
	}

	if len(l.items) >= l.max {
		// 移除尾部元素
		l.removeTail()
	}

	// 添加新元素到头部
	newItem := &cacheItem{
		key:   key,
		value: value,
	}
	l.addToHead(newItem)
	l.items[key] = newItem
}

// moveToHead 将节点移动到头部
func (l *LRUCache) moveToHead(item *cacheItem) {
	if item == l.head {
		return
	}

	// 从当前位置移除
	if item.prev != nil {
		item.prev.next = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	}

	// 如果是尾部节点
	if item == l.tail {
		l.tail = item.prev
	}

	// 移动到头部
	item.prev = nil
	item.next = l.head
	if l.head != nil {
		l.head.prev = item
	}
	l.head = item

	// 如果这是第一个元素
	if l.tail == nil {
		l.tail = item
	}
}

// addToHead 添加节点到头部
func (l *LRUCache) addToHead(item *cacheItem) {
	item.prev = nil
	item.next = l.head
	if l.head != nil {
		l.head.prev = item
	}
	l.head = item

	if l.tail == nil {
		l.tail = item
	}
}

// removeTail 移除尾部节点
func (l *LRUCache) removeTail() {
	if l.tail == nil {
		return
	}

	delete(l.items, l.tail.key)

	if l.tail.prev != nil {
		l.tail.prev.next = nil
	} else {
		// 只有一个元素
		l.head = nil
	}

	l.tail = l.tail.prev
}

// getCachedPathDetection 从缓存获取路径检测结果
func getCachedPathDetection(path string) (*FormatDetectionResult, bool) {
	// 首先尝试LRU缓存
	if lruCache != nil {
		if result, exists := lruCache.Get(path); exists {
			return result, true
		}
	}

	// 回退到传统缓存
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	result, exists := pathDetectionCache[path]
	return result, exists
}

// setCachedPathDetection 设置路径检测结果到缓存
func setCachedPathDetection(path string, result *FormatDetectionResult) {
	// 同时设置到两种缓存
	if lruCache != nil {
		lruCache.Put(path, result)
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// 简单的缓存淘汰策略：超过最大值时清空缓存
	if len(pathDetectionCache) >= cacheMaxSize {
		pathDetectionCache = make(map[string]*FormatDetectionResult)
	}

	pathDetectionCache[path] = result
}

// InitializeLRUCache 初始化LRU缓存
func InitializeLRUCache(maxSize int) {
	lruCache = NewLRUCache(maxSize)
}

// DetectRequestFormat automatically detects the API format from request path and body
func DetectRequestFormat(path string, requestBody []byte) *FormatDetectionResult {
	// 1. Body-based detection first, as it's more reliable than path.
	if len(requestBody) > 0 {
		var reqData map[string]interface{}
		if err := json.Unmarshal(requestBody, &reqData); err == nil {
			bodyResult := detectFromBody(reqData)
			// If body detection is highly confident, trust it regardless of path.
			if bodyResult.Confidence > 0.7 {
				return bodyResult
			}
		}
	}

	// 2. Path-based detection as a strong secondary signal if body is not conclusive.
	if cached, exists := getCachedPathDetection(path); exists {
		return cached
	}

	// Anthropic API paths
	if strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/v1/messages") ||
		strings.HasSuffix(path, "/count_tokens") || strings.HasSuffix(path, "/v1/count_tokens") {
		result := &FormatDetectionResult{
			Format:     FormatAnthropic,
			ClientType: ClientClaudeCode,
			Confidence: 0.95, // Path is a strong indicator, but body can override.
			DetectedBy: "path",
		}
		setCachedPathDetection(path, result)
		return result
	}

	// OpenAI API paths
	openaiPaths := []string{
		"/chat/completions", "/v1/chat/completions", "/completions", "/v1/completions",
		"/embeddings", "/v1/embeddings", "/models", "/v1/models", "/images/generations",
		"/v1/images/generations", "/audio/transcriptions", "/v1/audio/transcriptions",
		"/audio/translations", "/v1/audio/translations", "/audio/speech", "/v1/audio/speech",
		"/files", "/v1/files", "/fine_tuning", "/v1/fine_tuning", "/batches", "/v1/batches",
		"/responses", "/v1/responses", "/realtime", "/v1/realtime",
	}

	for _, openaiPath := range openaiPaths {
		if strings.HasSuffix(path, openaiPath) || strings.Contains(path, openaiPath+"/") {
			result := &FormatDetectionResult{
				Format:     FormatOpenAI,
				ClientType: ClientCodex,
				Confidence: 0.95,
				DetectedBy: "path",
			}
			setCachedPathDetection(path, result)
			return result
		}
	}

	// 3. If path detection also fails, fall back to any body detection result.
	if len(requestBody) > 0 {
		var reqData map[string]interface{}
		if err := json.Unmarshal(requestBody, &reqData); err == nil {
			bodyResult := detectFromBody(reqData)
			if bodyResult.Confidence > 0.3 { // Use a lower threshold here.
				return bodyResult
			}
		}
	}

	// 4. Default to unknown
	return &FormatDetectionResult{
		Format:     FormatUnknown,
		ClientType: ClientUnknown,
		Confidence: 0.0,
		DetectedBy: "unknown",
	}
}

// detectFromBody detects format from request body structure
func detectFromBody(reqData map[string]interface{}) *FormatDetectionResult {
	result := &FormatDetectionResult{
		Format:     FormatUnknown,
		ClientType: ClientUnknown,
		Confidence: 0.0,
	}

	anthropicScore := 0.0
	openAIScore := 0.0

	// Anthropic format characteristics
	if _, hasSystem := reqData["system"]; hasSystem {
		anthropicScore += 0.3
	}

	if _, hasMaxTokens := reqData["max_tokens"]; hasMaxTokens {
		anthropicScore += 0.1
	}

	// Check for Anthropic-specific fields
	if _, hasThinking := reqData["thinking"]; hasThinking {
		anthropicScore += 0.2
	}

	// OpenAI format characteristics
	if messages, ok := reqData["messages"].([]interface{}); ok && len(messages) > 0 {
		if msg, ok := messages[0].(map[string]interface{}); ok {
			if role, ok := msg["role"].(string); ok {
				if role == "system" || role == "developer" {
					// OpenAI 格式的 system 消息在 messages 数组内
					openAIScore += 0.3
				} else if role == "user" || role == "assistant" {
					// Both formats can have user/assistant messages
					// Check for OpenAI-specific message structure
					if _, hasContent := msg["content"]; hasContent {
						openAIScore += 0.1
						anthropicScore += 0.1
					}
				}
			}
		}
	}

	// Enhanced tool detection for better accuracy
	// Anthropic tools are in a separate "tools" array
	if _, hasAnthropicTools := reqData["tools"]; hasAnthropicTools {
		anthropicScore += 0.25
	}

	// OpenAI tools are in "tools" array within messages or top-level
	if tools, ok := reqData["tools"].([]interface{}); ok && len(tools) > 0 {
		// Check if tools have OpenAI format (function.type)
		if len(tools) > 0 {
			if tool, ok := tools[0].(map[string]interface{}); ok {
				if toolType, hasType := tool["type"].(string); hasType && toolType == "function" {
					openAIScore += 0.25
				}
			}
		}
	}

	// Enhanced model name detection
	if modelName, ok := reqData["model"].(string); ok {
		// Claude 模型特征
		if strings.Contains(modelName, "claude") || strings.Contains(modelName, "sonnet") ||
		   strings.Contains(modelName, "opus") || strings.Contains(modelName, "haiku") {
			anthropicScore += 0.3
		}
		// GPT 模型特征
		if strings.Contains(modelName, "gpt") || strings.Contains(modelName, "chatgpt") ||
		   strings.Contains(modelName, "davinci") || strings.Contains(modelName, "curie") {
			openAIScore += 0.3
		}
	}

	// Codex-specific format detection (instructions field)
	// Codex 使用 instructions 字段代替 messages 数组
	if instructions, hasInstructions := reqData["instructions"]; hasInstructions {
		if _, ok := instructions.(string); ok {
			// 这是 Codex 特有的格式，需要转换为标准 OpenAI 格式
			// 注意：虽然是 OpenAI 兼容格式，但需要格式转换
			openAIScore += 0.5 // 高分表示是 OpenAI 格式家族
			result.Format = FormatOpenAI // Codex 是 OpenAI 的变体
			result.ClientType = ClientCodex
			result.Confidence = 0.95
			result.DetectedBy = "codex-instructions"
			return result // 立即返回，确保优先识别 Codex 格式
		}
	}

	// OpenAI-specific fields
	if _, hasMaxCompletionTokens := reqData["max_completion_tokens"]; hasMaxCompletionTokens {
		openAIScore += 0.2
	}

	if _, hasTopP := reqData["top_p"]; hasTopP {
		openAIScore += 0.1
		anthropicScore += 0.1 // Both support this
	}

	if _, hasFrequencyPenalty := reqData["frequency_penalty"]; hasFrequencyPenalty {
		openAIScore += 0.2 // OpenAI-specific
	}

	if _, hasPresencePenalty := reqData["presence_penalty"]; hasPresencePenalty {
		openAIScore += 0.2 // OpenAI-specific
	}

	// Determine format based on scores
	if anthropicScore > openAIScore && anthropicScore > 0.3 {
		result.Format = FormatAnthropic
		result.ClientType = ClientClaudeCode
		result.Confidence = anthropicScore
		result.DetectedBy = "body-structure"
	} else if openAIScore > anthropicScore && openAIScore > 0.3 {
		result.Format = FormatOpenAI
		result.ClientType = ClientCodex
		result.Confidence = openAIScore
		result.DetectedBy = "body-structure"
	}

	return result
}

// GetClientTypeName returns a human-readable client type name
func (c ClientType) String() string {
	switch c {
	case ClientClaudeCode:
		return "Claude Code"
	case ClientCodex:
		return "Codex"
	default:
		return "Unknown"
	}
}

// GetFormatName returns a human-readable format name
func (f RequestFormat) String() string {
	switch f {
	case FormatAnthropic:
		return "Anthropic"
	case FormatOpenAI:
		return "OpenAI"
	default:
		return "Unknown"
	}
}