package conversion

// No imports needed for types only

// Gemini API类型定义
// 基于Google Gemini API文档和Python实现

// GeminiRequest Gemini API请求结构
type GeminiRequest struct {
	Contents         []GeminiContent         `json:"contents,omitempty"`
	SystemInstruction *GeminiContent         `json:"systemInstruction,omitempty"`
	Tools            []GeminiTool            `json:"tools,omitempty"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	Stream           bool                    `json:"stream,omitempty"`
}

// GeminiContent Gemini内容结构
type GeminiContent struct {
	Role  string           `json:"role"`
	Parts []GeminiPart     `json:"parts"`
}

// GeminiPart Gemini内容片段
type GeminiPart struct {
	Text             string                `json:"text,omitempty"`
	InlineData       *GeminiInlineData     `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
	Thought          bool                  `json:"thought,omitempty"` // 思考内容标记
}

// GeminiInlineData 内联数据（图像等）
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64编码
}

// GeminiFunctionCall 函数调用
type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// GeminiFunctionResponse 函数响应
type GeminiFunctionResponse struct {
	Name     string      `json:"name"`
	Response interface{} `json:"response"`
}

// GeminiTool 工具定义
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
	FunctionDeclarationsSnake []GeminiFunctionDeclaration `json:"function_declarations,omitempty"` // 兼容snake_case
}

// GeminiFunctionDeclaration 函数声明
type GeminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// GeminiGenerationConfig 生成配置
type GeminiGenerationConfig struct {
	Temperature      *float64             `json:"temperature,omitempty"`
	TopP             *float64             `json:"topP,omitempty"`
	TopK             *int                 `json:"topK,omitempty"`
	MaxOutputTokens  *int                 `json:"maxOutputTokens,omitempty"`
	StopSequences    []string             `json:"stopSequences,omitempty"`
	ResponseMimeType string               `json:"responseMimeType,omitempty"`
	ResponseSchema   map[string]interface{} `json:"responseSchema,omitempty"`
	ThinkingConfig   *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

// GeminiThinkingConfig 思考配置
type GeminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"` // -1表示动态思考，0表示禁用，正数表示token预算
}

// GeminiResponse Gemini API响应结构
type GeminiResponse struct {
	Candidates    []GeminiCandidate     `json:"candidates"`
	UsageMetadata *GeminiUsageMetadata  `json:"usageMetadata,omitempty"`
}

// GeminiCandidate 候选响应
type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
	Index        int           `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

// GeminiSafetyRating 安全评级
type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// GeminiUsageMetadata 使用统计
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
	ThinkingTokensCount  int `json:"thinkingTokensCount,omitempty"` // 思考token数
}

// GeminiStreamingChunk 流式响应块
type GeminiStreamingChunk struct {
	Candidates    []GeminiCandidate     `json:"candidates,omitempty"`
	UsageMetadata *GeminiUsageMetadata  `json:"usageMetadata,omitempty"`
}

// 注意：所有流式响应类型已在各自的types.go文件中定义，这里不再重复声明
