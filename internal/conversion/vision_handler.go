package conversion

import (
	"encoding/base64"
	"fmt"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// VisionHandler 视觉内容处理器
type VisionHandler struct {
	logger *logger.Logger
}

// NewVisionHandler 创建视觉处理器
func NewVisionHandler(logger *logger.Logger) *VisionHandler {
	return &VisionHandler{
		logger: logger,
	}
}

// ProcessMultipartContent 处理多模态内容转换
func (h *VisionHandler) ProcessMultipartContent(content interface{}, sourceFormat, targetFormat string) (interface{}, error) {
	switch sourceFormat {
	case "openai":
		return h.processOpenAIMultipartContent(content, targetFormat)
	case "anthropic":
		return h.processAnthropicMultipartContent(content, targetFormat)
	case "gemini":
		return h.processGeminiMultipartContent(content, targetFormat)
	default:
		return content, fmt.Errorf("unsupported source format: %s", sourceFormat)
	}
}

// processOpenAIMultipartContent 处理OpenAI多模态内容
func (h *VisionHandler) processOpenAIMultipartContent(content interface{}, targetFormat string) (interface{}, error) {
	switch targetFormat {
	case "gemini":
		if contentStr, ok := content.(string); ok {
			return []interface{}{
				map[string]interface{}{"text": contentStr},
			}, nil
		} else if contentParts, ok := content.([]interface{}); ok {
			var geminiParts []interface{}
			for _, part := range contentParts {
				if partMap, ok := part.(map[string]interface{}); ok {
					partType := h.getStringValue(partMap, "type", "")
					switch partType {
					case "text":
						if text, ok := partMap["text"].(string); ok {
							geminiParts = append(geminiParts, map[string]interface{}{"text": text})
						}
					case "image_url":
						if _, ok := partMap["image_url"].(map[string]interface{}); ok {
							if geminiPart, err := h.convertOpenAIImageToGemini(part); err == nil {
								geminiParts = append(geminiParts, geminiPart)
							}
						}
					}
				}
			}
			return geminiParts, nil
		}
	case "anthropic":
		if contentStr, ok := content.(string); ok {
			return contentStr, nil
		} else if contentParts, ok := content.([]interface{}); ok {
			var anthropicContent []interface{}
			for _, part := range contentParts {
				if partMap, ok := part.(map[string]interface{}); ok {
					partType := h.getStringValue(partMap, "type", "")
					switch partType {
					case "text":
						if text, ok := partMap["text"].(string); ok {
							anthropicContent = append(anthropicContent, map[string]interface{}{
								"type": "text",
								"text": text,
							})
						}
					case "image_url":
						if anthropicImage, err := h.convertOpenAIImageToAnthropic(part); err == nil {
							anthropicContent = append(anthropicContent, anthropicImage)
						}
					}
				}
			}
			return anthropicContent, nil
		}
	}
	return content, nil
}

// processAnthropicMultipartContent 处理Anthropic多模态内容
func (h *VisionHandler) processAnthropicMultipartContent(content interface{}, targetFormat string) (interface{}, error) {
	switch targetFormat {
	case "gemini":
		if contentStr, ok := content.(string); ok {
			return []interface{}{
				map[string]interface{}{"text": contentStr},
			}, nil
		} else if contentParts, ok := content.([]interface{}); ok {
			var geminiParts []interface{}
			for _, part := range contentParts {
				if partMap, ok := part.(map[string]interface{}); ok {
					partType := h.getStringValue(partMap, "type", "")
					switch partType {
					case "text":
						if text, ok := partMap["text"].(string); ok {
							geminiParts = append(geminiParts, map[string]interface{}{"text": text})
						}
					case "image":
						if geminiPart, err := h.convertAnthropicImageToGemini(part); err == nil {
							geminiParts = append(geminiParts, geminiPart)
						}
					}
				}
			}
			return geminiParts, nil
		}
	case "openai":
		if contentStr, ok := content.(string); ok {
			return contentStr, nil
		} else if contentParts, ok := content.([]interface{}); ok {
			var openAIContent []interface{}
			for _, part := range contentParts {
				if partMap, ok := part.(map[string]interface{}); ok {
					partType := h.getStringValue(partMap, "type", "")
					switch partType {
					case "text":
						if text, ok := partMap["text"].(string); ok {
							openAIContent = append(openAIContent, map[string]interface{}{
								"type": "text",
								"text": text,
							})
						}
					case "image":
						if openAIImage, err := h.convertAnthropicImageToOpenAI(part); err == nil {
							openAIContent = append(openAIContent, openAIImage)
						}
					}
				}
			}
			return openAIContent, nil
		}
	}
	return content, nil
}

// processGeminiMultipartContent 处理Gemini多模态内容
func (h *VisionHandler) processGeminiMultipartContent(content interface{}, targetFormat string) (interface{}, error) {
	if parts, ok := content.([]interface{}); ok {
		switch targetFormat {
		case "openai":
			var openAIContent []interface{}
			var textParts []string

			for _, part := range parts {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						textParts = append(textParts, text)
						openAIContent = append(openAIContent, map[string]interface{}{
							"type": "text",
							"text": text,
						})
					} else if _, ok := partMap["inlineData"].(map[string]interface{}); ok {
						if openAIImage, err := h.convertGeminiImageToOpenAI(part); err == nil {
							openAIContent = append(openAIContent, openAIImage)
						}
					}
				}
			}

			if len(openAIContent) == 1 && len(textParts) == 1 {
				return textParts[0], nil
			}
			return openAIContent, nil

		case "anthropic":
			var anthropicContent []interface{}

			for _, part := range parts {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						anthropicContent = append(anthropicContent, map[string]interface{}{
							"type": "text",
							"text": text,
						})
					} else if _, ok := partMap["inlineData"].(map[string]interface{}); ok {
						if anthropicImage, err := h.convertGeminiImageToAnthropic(part); err == nil {
							anthropicContent = append(anthropicContent, anthropicImage)
						}
					}
				}
			}

			if len(anthropicContent) == 1 {
				if textContent, ok := anthropicContent[0].(map[string]interface{}); ok && textContent["type"] == "text" {
					return textContent["text"], nil
				}
			}
			return anthropicContent, nil
		}
	}
	return content, nil
}

// 图像转换辅助方法

func (h *VisionHandler) convertOpenAIImageToGemini(openAIImage interface{}) (*GeminiPart, error) {
	imageMap, ok := openAIImage.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid OpenAI image format")
	}

	imageURLInterface, ok := imageMap["image_url"]
	if !ok {
		return nil, fmt.Errorf("missing image_url field")
	}

	imageURLMap, ok := imageURLInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid image_url format")
	}

	urlInterface, ok := imageURLMap["url"]
	if !ok {
		return nil, fmt.Errorf("missing url field")
	}

	imageURL, ok := urlInterface.(string)
	if !ok || imageURL == "" {
		return nil, fmt.Errorf("empty image URL")
	}

	return h.processImageURLToGemini(imageURL)
}

func (h *VisionHandler) convertOpenAIImageToAnthropic(openAIImage interface{}) (interface{}, error) {
	imageMap, ok := openAIImage.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid OpenAI image format")
	}

	imageURLInterface, ok := imageMap["image_url"]
	if !ok {
		return nil, fmt.Errorf("missing image_url field")
	}

	imageURLMap, ok := imageURLInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid image_url format")
	}

	urlInterface, ok := imageURLMap["url"]
	if !ok {
		return nil, fmt.Errorf("missing url field")
	}

	imageURL, ok := urlInterface.(string)
	if !ok || imageURL == "" {
		return nil, fmt.Errorf("empty image URL")
	}

	// 处理data URL格式
	var mimeType string
	var data string

	if strings.HasPrefix(imageURL, "data:") {
		parts := strings.SplitN(imageURL, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URL format")
		}

		header := parts[0]
		data = parts[1]

		if strings.HasPrefix(header, "data:image/") {
			mimeParts := strings.Split(header[5:], ";")
			if len(mimeParts) > 0 {
				mimeType = mimeParts[0]
			}
		}
	} else {
		return nil, fmt.Errorf("OpenAI image must be in data URL format for Anthropic conversion")
	}

	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %v", err)
	}

	anthropicImage := map[string]interface{}{
		"type": "image",
		"source": map[string]interface{}{
			"type":       "base64",
			"media_type": mimeType,
			"data":       data,
		},
	}

	return anthropicImage, nil
}

func (h *VisionHandler) convertAnthropicImageToGemini(anthropicImage interface{}) (*GeminiPart, error) {
	imageMap, ok := anthropicImage.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid Anthropic image format")
	}

	sourceInterface, ok := imageMap["source"]
	if !ok {
		return nil, fmt.Errorf("missing source field")
	}

	sourceMap, ok := sourceInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid source format")
	}

	mediaType := h.getStringValue(sourceMap, "media_type", "")
	data := h.getStringValue(sourceMap, "data", "")

	if data == "" {
		return nil, fmt.Errorf("empty image data")
	}

	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %v", err)
	}

	geminiPart := &GeminiPart{
		InlineData: &GeminiInlineData{
			MimeType: mediaType,
			Data:     data,
		},
	}

	return geminiPart, nil
}

func (h *VisionHandler) convertAnthropicImageToOpenAI(anthropicImage interface{}) (interface{}, error) {
	imageMap, ok := anthropicImage.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid Anthropic image format")
	}

	sourceInterface, ok := imageMap["source"]
	if !ok {
		return nil, fmt.Errorf("missing source field")
	}

	sourceMap, ok := sourceInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid source format")
	}

	mediaType := h.getStringValue(sourceMap, "media_type", "")
	data := h.getStringValue(sourceMap, "data", "")

	if data == "" {
		return nil, fmt.Errorf("empty image data")
	}

	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %v", err)
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, data)

	openAIImage := map[string]interface{}{
		"type": "image_url",
		"image_url": map[string]interface{}{
			"url":    dataURL,
			"detail": "auto",
		},
	}

	return openAIImage, nil
}

func (h *VisionHandler) convertGeminiImageToOpenAI(geminiImage interface{}) (interface{}, error) {
	partMap, ok := geminiImage.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid Gemini image part")
	}

	inlineDataInterface, ok := partMap["inlineData"]
	if !ok {
		return nil, fmt.Errorf("missing inlineData field")
	}

	inlineDataMap, ok := inlineDataInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid inlineData format")
	}

	mimeType := h.getStringValue(inlineDataMap, "mimeType", "")
	data := h.getStringValue(inlineDataMap, "data", "")

	if data == "" {
		return nil, fmt.Errorf("empty image data")
	}

	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %v", err)
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, data)

	openAIImage := map[string]interface{}{
		"type": "image_url",
		"image_url": map[string]interface{}{
			"url": dataURL,
		},
	}

	return openAIImage, nil
}

func (h *VisionHandler) convertGeminiImageToAnthropic(geminiImage interface{}) (interface{}, error) {
	partMap, ok := geminiImage.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid Gemini image part")
	}

	inlineDataInterface, ok := partMap["inlineData"]
	if !ok {
		return nil, fmt.Errorf("missing inlineData field")
	}

	inlineDataMap, ok := inlineDataInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid inlineData format")
	}

	mimeType := h.getStringValue(inlineDataMap, "mimeType", "")
	data := h.getStringValue(inlineDataMap, "data", "")

	if data == "" {
		return nil, fmt.Errorf("empty image data")
	}

	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %v", err)
	}

	anthropicImage := map[string]interface{}{
		"type": "image",
		"source": map[string]interface{}{
			"type":       "base64",
			"media_type": mimeType,
			"data":       data,
		},
	}

	return anthropicImage, nil
}

// processImageURLToGemini 处理图像URL到Gemini格式的转换
func (h *VisionHandler) processImageURLToGemini(imageURL string) (*GeminiPart, error) {
	var mimeType string
	var data string

	if strings.HasPrefix(imageURL, "data:") {
		parts := strings.SplitN(imageURL, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URL format")
		}

		header := parts[0]
		data = parts[1]

		if strings.HasPrefix(header, "data:image/") {
			mimeParts := strings.Split(header[5:], ";")
			if len(mimeParts) > 0 {
				mimeType = mimeParts[0]
			}
		}
	} else {
		data = imageURL
		mimeType = h.inferImageType(imageURL)
	}

	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %v", err)
	}

	geminiPart := &GeminiPart{
		InlineData: &GeminiInlineData{
			MimeType: mimeType,
			Data:     data,
		},
	}

	return geminiPart, nil
}

// inferImageType 从URL推断图像类型
func (h *VisionHandler) inferImageType(imageURL string) string {
	if strings.Contains(imageURL, "jpeg") || strings.Contains(imageURL, "jpg") {
		return "image/jpeg"
	} else if strings.Contains(imageURL, "png") {
		return "image/png"
	} else if strings.Contains(imageURL, "gif") {
		return "image/gif"
	} else if strings.Contains(imageURL, "webp") {
		return "image/webp"
	}
	return "image/jpeg" // 默认类型
}

// getStringValue 工具方法
func (h *VisionHandler) getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}
