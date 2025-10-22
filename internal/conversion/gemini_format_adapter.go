package conversion

import "fmt"

// GeminiFormatAdapter is a lightweight placeholder implementation that keeps
// the adapter factory functional. The detailed Gemini conversion logic was
// removed during the duplicate refactor effort; callers that still require it
// will receive a structured error.
type GeminiFormatAdapter struct{}

func NewGeminiFormatAdapter() *GeminiFormatAdapter {
	return &GeminiFormatAdapter{}
}

func (g *GeminiFormatAdapter) Name() string {
	return "gemini"
}

func (g *GeminiFormatAdapter) ParseRequestJSON(payload []byte) (*InternalRequest, error) {
	return nil, NewConversionError("unsupported", "gemini request conversion is not implemented", nil)
}

func (g *GeminiFormatAdapter) BuildRequestJSON(req *InternalRequest) ([]byte, error) {
	return nil, fmt.Errorf("gemini request conversion is not implemented")
}

func (g *GeminiFormatAdapter) ParseResponseJSON(payload []byte) (*InternalResponse, error) {
	return nil, NewConversionError("unsupported", "gemini response conversion is not implemented", nil)
}

func (g *GeminiFormatAdapter) BuildResponseJSON(resp *InternalResponse) ([]byte, error) {
	return nil, fmt.Errorf("gemini response conversion is not implemented")
}

func (g *GeminiFormatAdapter) ParseSSE(event string, data []byte) ([]InternalEvent, error) {
	return nil, NewConversionError("unsupported", "gemini SSE parsing is not implemented", nil)
}

func (g *GeminiFormatAdapter) BuildSSE(events []InternalEvent) ([]SSEPayload, error) {
	return nil, fmt.Errorf("gemini SSE rendering is not implemented")
}

