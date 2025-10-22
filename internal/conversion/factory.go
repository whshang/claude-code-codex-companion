package conversion

import (
	"fmt"
	"sync"

	"claude-code-codex-companion/internal/logger"
)

// FormatType 定义支持的API格式类型
type FormatType string

const (
	FormatOpenAI     FormatType = "openai"
	FormatAnthropic  FormatType = "anthropic"
	FormatGemini     FormatType = "gemini"
	FormatResponses  FormatType = "responses" // OpenAI Responses API
)

// ConversionResult 统一的转换结果
type ConversionResult struct {
	Success bool                   `json:"success"`
	Data    interface{}            `json:"data,omitempty"`
	Headers map[string]string      `json:"headers,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Warning string                 `json:"warning,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ConversionContext 转换上下文，包含请求信息和状态
type ConversionContext struct {
	RequestID     string                 `json:"request_id"`
	SourceFormat  FormatType             `json:"source_format"`
	TargetFormat  FormatType             `json:"target_format"`
	OriginalModel string                 `json:"original_model"`
	Headers       map[string]string      `json:"headers,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Logger        *logger.Logger         `json:"-"`
}

// ConverterInterface 统一的转换器接口
type ConverterInterface interface {
	// ConvertRequest 转换请求格式
	ConvertRequest(ctx *ConversionContext, data interface{}) *ConversionResult
	
	// ConvertResponse 转换响应格式
	ConvertResponse(ctx *ConversionContext, data interface{}) *ConversionResult
	
	// ConvertStreamingChunk 转换流式响应chunk
	ConvertStreamingChunk(ctx *ConversionContext, data interface{}) *ConversionResult
	
	// GetSupportedFormats 获取支持的格式列表
	GetSupportedFormats() []FormatType
	
	// ValidateFormat 验证格式是否支持
	ValidateFormat(format FormatType) bool
	
	// ResetStreamingState 重置流式状态（用于处理新的流）
	ResetStreamingState()
	
	// SetOriginalModel 设置原始模型名称
	SetOriginalModel(model string)
}

// ConverterFactory 转换器工厂
type ConverterFactory struct {
	converters map[FormatType]ConverterInterface
	mutex      sync.RWMutex
	logger     *logger.Logger
}

// NewConverterFactory 创建转换器工厂
func NewConverterFactory(logger *logger.Logger) *ConverterFactory {
	return &ConverterFactory{
		converters: make(map[FormatType]ConverterInterface),
		logger:     logger,
	}
}

// RegisterConverter 注册转换器
func (f *ConverterFactory) RegisterConverter(format FormatType, converter ConverterInterface) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	f.converters[format] = converter
	f.logger.Info("Converter registered", map[string]interface{}{
		"format": format,
		"type":   fmt.Sprintf("%T", converter),
	})
}

// GetConverter 获取指定格式的转换器
func (f *ConverterFactory) GetConverter(format FormatType) (ConverterInterface, error) {
	f.mutex.RLock()
	converter, exists := f.converters[format]
	f.mutex.RUnlock()
	
	if !exists {
		converter = f.createConverter(format)
		if converter != nil {
			f.mutex.Lock()
			f.converters[format] = converter
			f.mutex.Unlock()
		}
	}
	
	if converter == nil {
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	
	return converter, nil
}

// createConverter 创建转换器实例
// NewOpenAIConverter creates a new OpenAI converter (placeholder)
func NewOpenAIConverter(logger *logger.Logger) ConverterInterface {
	return nil // TODO: Implement
}

// NewAnthropicConverter creates a new Anthropic converter (placeholder)
func NewAnthropicConverter(logger *logger.Logger) ConverterInterface {
	return nil // TODO: Implement
}

// NewGeminiConverter creates a new Gemini converter (placeholder)
func NewGeminiConverter(logger *logger.Logger) ConverterInterface {
	return nil // TODO: Implement
}

// NewResponsesConverter creates a new Responses converter (placeholder)
func NewResponsesConverter(logger *logger.Logger) ConverterInterface {
	return nil // TODO: Implement
}

func (f *ConverterFactory) createConverter(format FormatType) ConverterInterface {
	switch format {
	case FormatOpenAI:
		return NewOpenAIConverter(f.logger)
	case FormatAnthropic:
		return NewAnthropicConverter(f.logger)
	case FormatGemini:
		return NewGeminiConverter(f.logger)
	case FormatResponses:
		return NewResponsesConverter(f.logger)
	default:
		f.logger.Error("Unsupported format for converter creation", fmt.Errorf("unsupported format: %s", format))
		return nil
	}
}

// GetSupportedFormats 获取所有支持的格式
func (f *ConverterFactory) GetSupportedFormats() []FormatType {
	return []FormatType{
		FormatOpenAI,
		FormatAnthropic,
		FormatGemini,
		FormatResponses,
	}
}

// ValidateFormat 验证格式是否支持
func (f *ConverterFactory) ValidateFormat(format FormatType) bool {
	supportedFormats := f.GetSupportedFormats()
	for _, supportedFormat := range supportedFormats {
		if supportedFormat == format {
			return true
		}
	}
	return false
}

// NewConversionContext 创建转换上下文
func NewConversionContext(sourceFormat, targetFormat FormatType, requestID, originalModel string, headers map[string]string) *ConversionContext {
	return &ConversionContext{
		RequestID:     requestID,
		SourceFormat:  sourceFormat,
		TargetFormat:  targetFormat,
		OriginalModel: originalModel,
		Headers:       headers,
		Metadata:      make(map[string]interface{}),
	}
}

// WithLogger 设置日志记录器
func (ctx *ConversionContext) WithLogger(logger *logger.Logger) *ConversionContext {
	ctx.Logger = logger
	return ctx
}

// WithMetadata 添加元数据
func (ctx *ConversionContext) WithMetadata(key string, value interface{}) *ConversionContext {
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]interface{})
	}
	ctx.Metadata[key] = value
	return ctx
}

// GetMetadata 获取元数据
func (ctx *ConversionContext) GetMetadata(key string) (interface{}, bool) {
	if ctx.Metadata == nil {
		return nil, false
	}
	value, exists := ctx.Metadata[key]
	return value, exists
}

// 全局工厂实例
var globalFactory *ConverterFactory
var factoryOnce sync.Once

// GetGlobalFactory 获取全局转换器工厂
func GetGlobalFactory(logger *logger.Logger) *ConverterFactory {
	factoryOnce.Do(func() {
		globalFactory = NewConverterFactory(logger)
	})
	return globalFactory
}

// 便捷函数：转换请求
func ConvertRequest(sourceFormat, targetFormat FormatType, data interface{}, originalModel string, headers map[string]string) (*ConversionResult, error) {
	factory := GetGlobalFactory(nil)
	converter, err := factory.GetConverter(sourceFormat)
	if err != nil {
		return nil, err
	}
	
	ctx := NewConversionContext(sourceFormat, targetFormat, "", originalModel, headers)
	converter.SetOriginalModel(originalModel)
	
	return converter.ConvertRequest(ctx, data), nil
}

// 便捷函数：转换响应
func ConvertResponse(sourceFormat, targetFormat FormatType, data interface{}, originalModel string) (*ConversionResult, error) {
	factory := GetGlobalFactory(nil)
	converter, err := factory.GetConverter(targetFormat)
	if err != nil {
		return nil, err
	}
	
	ctx := NewConversionContext(sourceFormat, targetFormat, "", originalModel, nil)
	converter.SetOriginalModel(originalModel)
	
	return converter.ConvertResponse(ctx, data), nil
}

// 便捷函数：转换流式chunk
func ConvertStreamingChunk(sourceFormat, targetFormat FormatType, data interface{}, originalModel string) (*ConversionResult, error) {
	factory := GetGlobalFactory(nil)
	converter, err := factory.GetConverter(targetFormat)
	if err != nil {
		return nil, err
	}
	
	ctx := NewConversionContext(sourceFormat, targetFormat, "", originalModel, nil)
	converter.SetOriginalModel(originalModel)
	
	return converter.ConvertStreamingChunk(ctx, data), nil
}