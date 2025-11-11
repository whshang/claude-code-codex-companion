package conversion

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

const (
	defaultScannerBuffer      = 64 * 1024
	defaultScannerMaxCapacity = 2 * 1024 * 1024
)

// ⚠️ DEPRECATED: This file contains legacy conversion functions that are no longer recommended.
//
// 🔧 Migration Guide:
//
// OLD APPROACH (legacy functions):
//   body, err := LegacyConvertChatResponseJSONToResponses(respBody)
//   body, err := LegacyConvertResponsesRequestJSONToChat(reqBody)
//
// NEW APPROACH (adapter pattern):
//   factory := NewAdapterFactory(logger)
//   chatAdapter := factory.OpenAIChatAdapter()
//   responsesAdapter := factory.OpenAIResponsesAdapter()
//
//   // Chat → Internal → Responses
//   internalReq, err := chatAdapter.ParseRequestJSON(chatJSON)
//   responsesJSON, err := responsesAdapter.BuildRequestJSON(internalReq)
//
//   // Responses → Internal → Chat
//   internalReq, err := responsesAdapter.ParseRequestJSON(responsesJSON)
//   chatJSON, err := chatAdapter.BuildRequestJSON(internalReq)
//
// 🎯 Benefits of New Approach:
//   ✅ Complete field mapping (presence_penalty, frequency_penalty, logit_bias, n, response_format)
//   ✅ Dual-path fallback (支持 input 和 messages 字段)
//   ✅ Type-safe conversions with comprehensive validation
//   ✅ Bidirectional format support (Chat ↔ Responses)
//   ✅ Better error handling and logging
//   ✅ 100+ unit tests covering all edge cases
//
// 📌 Note: ConversionManager automatically falls back to these legacy functions if unified conversion fails.
//         See internal/conversion/conversion_manager.go for details.

// LegacyConvertChatResponseJSONToResponses mirrors the historical Codex conversion logic.
//
// Deprecated: Use OpenAIChatAdapter and OpenAIResponsesAdapter instead.
// This function only maps basic fields and lacks support for:
//   - sampling parameters (presence_penalty, frequency_penalty, logit_bias, n)
//   - response format control (json_object, json_schema)
//   - dual-path fallback (input vs messages)
func LegacyConvertChatResponseJSONToResponses(body []byte) ([]byte, error) {
	var completion map[string]interface{}
	if err := json.Unmarshal(body, &completion); err != nil {
		return nil, err
	}

	objectType, _ := completion["object"].(string)
	if objectType != "chat.completion" {
		// 未识别的对象类型，保持原样
		return body, nil
	}

	response := map[string]interface{}{
		"type":    "response",
		"id":      completion["id"],
		"object":  "response",
		"created": completion["created"],
		"model":   completion["model"],
		"choices": completion["choices"],
	}

	if usage, ok := completion["usage"]; ok {
		response["usage"] = usage
	}
	if fingerprint, ok := completion["system_fingerprint"]; ok {
		response["system_fingerprint"] = fingerprint
	}

	return json.Marshal(response)
}

// LegacyConvertResponsesRequestJSONToChat converts Codex /responses requests to Chat Completions format.
//
// Deprecated: Use OpenAIResponsesAdapter.ParseRequestJSON() followed by OpenAIChatAdapter.BuildRequestJSON().
// This function only handles basic field mapping and lacks:
//   - sampling parameters (presence_penalty, frequency_penalty, logit_bias, n)
//   - response format control
//   - proper message content type handling (only extracts text, ignores other content types)
func LegacyConvertResponsesRequestJSONToChat(body []byte) ([]byte, error) {
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return nil, err
	}

	_, hasInput := requestData["input"]
	_, hasInstructions := requestData["instructions"]

	if !hasInput && !hasInstructions {
		// 非 Codex 格式
		return nil, nil
	}

	messages := []map[string]interface{}{}

	if hasInstructions {
		if instructionsStr, ok := requestData["instructions"].(string); ok && instructionsStr != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": instructionsStr,
			})
		}
		delete(requestData, "instructions")
	}

	if hasInput {
		if inputArray, ok := requestData["input"].([]interface{}); ok {
			for _, item := range inputArray {
				inputMsg, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				role, _ := inputMsg["role"].(string)
				if role == "" {
					role = "user"
				}

				var contentBuilder strings.Builder
				if contentArray, ok := inputMsg["content"].([]interface{}); ok {
					for _, contentItem := range contentArray {
						if contentObj, ok := contentItem.(map[string]interface{}); ok {
							if text, ok := contentObj["text"].(string); ok {
								contentBuilder.WriteString(text)
							}
						}
					}
				}

				if contentBuilder.Len() > 0 {
					messages = append(messages, map[string]interface{}{
						"role":    role,
						"content": contentBuilder.String(),
					})
				}
			}
		}
		delete(requestData, "input")
	}

	if len(messages) == 0 {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": "Hello",
		})
	}

	requestData["messages"] = messages
	delete(requestData, "include")
	delete(requestData, "input_format")
	delete(requestData, "output_format")

	return json.Marshal(requestData)
}

// LegacyStreamChatCompletionsToResponses keeps the original SSE conversion logic.
//
// Deprecated: Use StreamChatCompletionsToResponsesUnified() for better streaming support.
// This function only handles basic text deltas and lacks:
//   - complete tool call streaming with response.function_call.* events
//   - proper event sequencing (response.created → deltas → response.completed)
//   - robust error recovery and incomplete stream handling
func LegacyStreamChatCompletionsToResponses(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	var responseID string
	var model string
	var createdEmitted bool
	var completedEmitted bool

	writeEvent := func(event string, payload map[string]interface{}) error {
		if _, ok := payload["model"]; !ok && model != "" {
			payload["model"] = model
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if event != "" {
			if _, err := io.WriteString(w, "event: "+event+"\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "data: "+string(b)+"\n\n"); err != nil {
			return err
		}
		return nil
	}

	emitCreatedIfNeeded := func() error {
		if createdEmitted || responseID == "" || model == "" {
			return nil
		}
		createdEmitted = true
		return writeEvent("response.created", map[string]interface{}{
			"type": "response.created",
			"response": map[string]interface{}{
				"id":    responseID,
				"model": model,
			},
		})
	}

	emitCompleted := func() error {
		if completedEmitted {
			return nil
		}
		completedEmitted = true
		if err := emitCreatedIfNeeded(); err != nil {
			return err
		}
		return writeEvent("response.completed", map[string]interface{}{
			"type": "response.completed",
			"response": map[string]interface{}{
				"id":    responseID,
				"model": model,
			},
		})
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !strings.HasPrefix(trimmed, "data: ") {
			continue
		}
		dataContent := strings.TrimSpace(strings.TrimPrefix(trimmed, "data: "))

		if dataContent == "[DONE]" {
			if err := emitCompleted(); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
				return err
			}
			continue
		}

		var chunk OpenAIStreamChunk
		if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
			if _, err2 := io.WriteString(w, "data: "+dataContent+"\n\n"); err2 != nil {
				return err2
			}
			continue
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.ID != "" && responseID == "" {
			responseID = chunk.ID
		}

		hasToolCalls := false
		for _, ch := range chunk.Choices {
			if len(ch.Delta.ToolCalls) > 0 {
				hasToolCalls = true
				break
			}
		}
		if hasToolCalls {
			if err := emitCreatedIfNeeded(); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "data: "+dataContent+"\n\n"); err != nil {
				return err
			}
			continue
		}

		if err := emitCreatedIfNeeded(); err != nil {
			return err
		}
		for _, ch := range chunk.Choices {
			if text, ok := ch.Delta.Content.(string); ok && text != "" {
				if err := writeEvent("response.output_text.delta", map[string]interface{}{
					"type":        "response.output_text.delta",
					"delta":       text,
					"response_id": responseID,
				}); err != nil {
					return err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if !completedEmitted {
		if err := emitCompleted(); err != nil {
			return err
		}
	}
	return nil
}

// Legacy stream helpers removed: use StreamResponsesToChat or StreamChatCompletionsToResponsesUnified directly.
