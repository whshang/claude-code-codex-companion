package conversion

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type anthropicToolCallState struct {
	ID        string
	Name      string
	Index     int
	Arguments strings.Builder
}

// StreamChatCompletionsToResponses 将 OpenAI /chat/completions SSE 转换为 Responses SSE。
// 解析失败时回退为原始透传，确保不会中断流式输出。
func StreamChatCompletionsToResponses(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	var (
		rawLines   []string
		chunks     []OpenAIStreamChunk
		responseID string
		model      string
		usage      *OpenAIUsage
	)

	pythonFixer := NewPythonJSONFixer(nil)

	for scanner.Scan() {
		line := scanner.Text()
		rawLines = append(rawLines, line)

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			continue
		}

		var chunk OpenAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return writeFallbackSSE(w, rawLines)
		}

		// 规范化工具调用参数中的 Python 风格 JSON
		if pythonFixer != nil {
			for ci := range chunk.Choices {
				for ti := range chunk.Choices[ci].Delta.ToolCalls {
					args := chunk.Choices[ci].Delta.ToolCalls[ti].Function.Arguments
					if fixed, changed := pythonFixer.FixPythonStyleJSON(args); changed {
						chunk.Choices[ci].Delta.ToolCalls[ti].Function.Arguments = fixed
					}
				}
			}
		}

		if responseID == "" && chunk.ID != "" {
			responseID = chunk.ID
		}
		if model == "" && chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		chunks = append(chunks, chunk)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if len(chunks) == 0 {
		return writeFallbackSSE(w, rawLines)
	}

	payload, err := buildResponsesSSEFromChunks(chunks, responseID, model, usage)
	if err != nil {
		return writeFallbackSSE(w, rawLines)
	}

	if _, err := w.Write(payload); err != nil {
		return err
	}
	_, err = io.WriteString(w, "data: [DONE]\n\n")
	return err
}

// StreamChatCompletionsToResponsesUnified 当前直接复用统一实现。
func StreamChatCompletionsToResponsesUnified(r io.Reader, w io.Writer) error {
	return StreamChatCompletionsToResponses(r, w)
}

// StreamAnthropicSSEToOpenAI 将 Anthropic SSE 转换为 OpenAI SSE。
func StreamAnthropicSSEToOpenAI(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	streamID := ""
	model := ""
	var usage *OpenAIUsage
	finishReason := ""
	toolStates := make(map[int]*anthropicToolCallState)
	jsonFixer := NewPythonJSONFixer(nil)

	var currentEvent string
	var dataBuilder strings.Builder
	flushEvent := func() error {
		if currentEvent == "" {
			return nil
		}
		payload := strings.TrimSpace(dataBuilder.String())
		dataBuilder.Reset()
		if payload == "" {
			currentEvent = ""
			return nil
		}
		switch currentEvent {
		case "message_start":
			var evt struct {
				Message struct {
					ID    string `json:"id"`
					Model string `json:"model"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(payload), &evt); err != nil {
				return err
			}
			if evt.Message.ID != "" {
				streamID = evt.Message.ID
			}
			if evt.Message.Model != "" {
				model = evt.Message.Model
			}
			if streamID == "" {
				streamID = generateStreamID()
			}
			chunk := map[string]interface{}{
				"id":      streamID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   model,
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"role": "assistant",
						},
					},
				},
			}
			return writeOpenAISSEChunk(w, chunk)

		case "content_block_start":
			var evt struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type  string          `json:"type"`
					ID    string          `json:"id"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(payload), &evt); err != nil {
				return err
			}
			if evt.ContentBlock.Type == "tool_use" {
				state := &anthropicToolCallState{
					ID:    evt.ContentBlock.ID,
					Name:  evt.ContentBlock.Name,
					Index: evt.Index,
				}
				if len(evt.ContentBlock.Input) > 0 {
					state.Arguments.WriteString(strings.TrimSpace(string(evt.ContentBlock.Input)))
				}
				toolStates[evt.Index] = state
				args := formatToolArguments(state.Arguments.String(), jsonFixer)
				chunk := map[string]interface{}{
					"id":      streamID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   model,
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]interface{}{
								"tool_calls": []map[string]interface{}{
									{
										"index": state.Index,
										"id":    state.ID,
										"type":  "function",
										"function": map[string]interface{}{
											"name":      state.Name,
											"arguments": args,
										},
									},
								},
							},
						},
					},
				}
				return writeOpenAISSEChunk(w, chunk)
			}
		case "content_block_delta":
			var base map[string]interface{}
			if err := json.Unmarshal([]byte(payload), &base); err != nil {
				return err
			}
			idxVal, _ := base["index"].(float64)
			index := int(idxVal)
			deltaMap, _ := base["delta"].(map[string]interface{})
			if deltaMap == nil {
				currentEvent = ""
				return nil
			}
			deltaType, _ := deltaMap["type"].(string)
			switch deltaType {
			case "text_delta":
				text, _ := deltaMap["text"].(string)
				if text == "" {
					currentEvent = ""
					return nil
				}
				chunk := map[string]interface{}{
					"id":      streamID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   model,
					"choices": []map[string]interface{}{
						{
							"index": index,
							"delta": map[string]interface{}{
								"content": text,
							},
						},
					},
				}
				return writeOpenAISSEChunk(w, chunk)
			case "input_json_delta":
				partial, _ := deltaMap["partial_json"].(string)
				state := toolStates[index]
				if state == nil {
					return nil
				}
				state.Arguments.WriteString(partial)
				args := formatToolArguments(state.Arguments.String(), jsonFixer)
				chunk := map[string]interface{}{
					"id":      streamID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   model,
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]interface{}{
								"tool_calls": []map[string]interface{}{
									{
										"index": index,
										"id":    state.ID,
										"type":  "function",
										"function": map[string]interface{}{
											"name":      state.Name,
											"arguments": args,
										},
									},
								},
							},
						},
					},
				}
				return writeOpenAISSEChunk(w, chunk)
			default:
				// ignore other deltas for now
			}

		case "message_delta":
			var evt struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage *AnthropicUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(payload), &evt); err != nil {
				return err
			}
			if evt.Usage != nil {
				usage = &OpenAIUsage{
					PromptTokens:     evt.Usage.InputTokens,
					CompletionTokens: evt.Usage.OutputTokens,
					TotalTokens:      evt.Usage.InputTokens + evt.Usage.OutputTokens,
				}
			}
			finishReason = mapAnthropicFinishReason(evt.Delta.StopReason)

		case "message_stop":
			chunk := map[string]interface{}{
				"id":      streamID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   model,
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": finishReason,
					},
				},
			}
			if usage != nil {
				chunk["usage"] = usage
			}
			if err := writeOpenAISSEChunk(w, chunk); err != nil {
				return err
			}
			_, err := io.WriteString(w, "data: [DONE]\n\n")
			return err
		}

		currentEvent = ""
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			if err := flushEvent(); err != nil {
				return err
			}
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			dataBuilder.Reset()
		case strings.HasPrefix(line, "data:"):
			if dataBuilder.Len() > 0 {
				dataBuilder.WriteByte('\n')
			}
			dataBuilder.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case strings.TrimSpace(line) == "":
			if err := flushEvent(); err != nil {
				return err
			}
			currentEvent = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return flushEvent()
}

// StreamOpenAISSEToAnthropic 将 OpenAI /chat/completions SSE 转换为 Anthropic SSE。
func StreamOpenAISSEToAnthropic(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	streamID := ""
	model := ""
	startEmitted := false
	textStarted := false
	textIndex := 0
	jsonFixer := NewPythonJSONFixer(nil)
	toolStates := make(map[int]*anthropicToolCallState)
	finishReason := ""
	var usage *AnthropicUsage

	writeEvent := func(event string, payload map[string]interface{}) error {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		bytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", string(bytes)); err != nil {
			return err
		}
		return nil
	}

	emitTextDelta := func(text string) error {
		if text == "" {
			return nil
		}
		if !textStarted {
			textStarted = true
			if err := writeEvent("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": textIndex,
				"content_block": map[string]interface{}{
					"type": "text",
					"text": "",
				},
			}); err != nil {
				return err
			}
		}
		return writeEvent("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": textIndex,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": text,
			},
		})
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}

		var chunk OpenAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return err
		}

		if chunk.ID != "" {
			streamID = chunk.ID
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Usage != nil {
			usage = &AnthropicUsage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			}
		}

		if streamID == "" {
			streamID = generateStreamID()
		}

		if !startEmitted {
			startEmitted = true
			if err := writeEvent("message_start", map[string]interface{}{
				"type": "message_start",
				"message": map[string]interface{}{
					"id":    streamID,
					"model": model,
					"role":  "assistant",
				},
			}); err != nil {
				return err
			}
		}

		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finishReason = normalizeOpenAIFinishReason(choice.FinishReason)
			}
			if choice.Delta.ToolCalls != nil {
				for _, toolCall := range choice.Delta.ToolCalls {
					idx := toolCall.Index
					if idx < 0 {
						idx = 0
					}
					state, exists := toolStates[idx]
					if !exists {
						state = &anthropicToolCallState{
							ID:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Index: idx,
						}
						if state.ID == "" {
							state.ID = generateToolCallID(state.Name, idx)
						}
						if state.Name == "" {
							state.Name = toolCall.Function.Name
						}
						toolStates[idx] = state
						if err := writeEvent("content_block_start", map[string]interface{}{
							"type":  "content_block_start",
							"index": idx,
							"content_block": map[string]interface{}{
								"type":  "tool_use",
								"id":    state.ID,
								"name":  state.Name,
								"input": map[string]interface{}{},
							},
						}); err != nil {
							return err
						}
					}
					if toolCall.Function.Name != "" && state.Name == "" {
						state.Name = toolCall.Function.Name
					}
					if toolCall.Function.Arguments != "" {
						state.Arguments.WriteString(toolCall.Function.Arguments)
						args := formatToolArguments(state.Arguments.String(), jsonFixer)
						if err := writeEvent("content_block_delta", map[string]interface{}{
							"type":  "content_block_delta",
							"index": idx,
							"delta": map[string]interface{}{
								"type":         "input_json_delta",
								"partial_json": args,
							},
						}); err != nil {
							return err
						}
					}
				}
			}

			if choice.Delta.Content != nil {
				switch content := choice.Delta.Content.(type) {
				case string:
					if err := emitTextDelta(content); err != nil {
						return err
					}
				case []interface{}:
					for _, item := range content {
						switch block := item.(type) {
						case map[string]interface{}:
							if blockType, _ := block["type"].(string); blockType == "text" {
								if text, _ := block["text"].(string); text != "" {
									if err := emitTextDelta(text); err != nil {
										return err
									}
								}
							}
						}
					}
				case []map[string]interface{}:
					for _, block := range content {
						if blockType, _ := block["type"].(string); blockType == "text" {
							if text, _ := block["text"].(string); text != "" {
								if err := emitTextDelta(text); err != nil {
									return err
								}
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if startEmitted {
		if textStarted {
			if err := writeEvent("content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": textIndex,
			}); err != nil {
				return err
			}
		}

		if len(toolStates) > 0 {
			indices := make([]int, 0, len(toolStates))
			for idx := range toolStates {
				indices = append(indices, idx)
			}
			sort.Ints(indices)
			for _, idx := range indices {
				if err := writeEvent("content_block_stop", map[string]interface{}{
					"type":  "content_block_stop",
					"index": idx,
				}); err != nil {
					return err
				}
			}
		}

		if finishReason != "" || usage != nil {
			payload := map[string]interface{}{
				"type": "message_delta",
				"delta": map[string]interface{}{
					"stop_reason": finishReason,
				},
			}
			if usage != nil {
				payload["usage"] = map[string]interface{}{
					"input_tokens":  usage.InputTokens,
					"output_tokens": usage.OutputTokens,
				}
			}
			if err := writeEvent("message_delta", payload); err != nil {
				return err
			}
		}

		if err := writeEvent("message_stop", map[string]interface{}{
			"type": "message_stop",
		}); err != nil {
			return err
		}
	}

	return nil
}

// StreamGeminiSSEToOpenAI 将 Gemini SSE 转换为 OpenAI SSE。
func StreamGeminiSSEToOpenAI(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	streamID := generateStreamID()
	model := ""
	var usage *OpenAIUsage
	toolCounter := 0
	jsonFixer := NewPythonJSONFixer(nil)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk geminiStreamingChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return err
			}
			continue
		}

		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.UsageMetadata != nil {
			usage = &OpenAIUsage{
				PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
				CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      chunk.UsageMetadata.TotalTokenCount,
			}
		}

		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				switch {
				case part.Text != "":
					chunk := map[string]interface{}{
						"id":      streamID,
						"object":  "chat.completion.chunk",
						"created": time.Now().Unix(),
						"model":   model,
						"choices": []map[string]interface{}{
							{
								"index": 0,
								"delta": map[string]interface{}{
									"content": part.Text,
								},
							},
						},
					}
					if err := writeOpenAISSEChunk(w, chunk); err != nil {
						return err
					}
				case part.FunctionCall != nil:
					argsBytes, _ := json.Marshal(part.FunctionCall.Args)
					args := formatToolArguments(string(argsBytes), jsonFixer)
					toolID := generateToolCallID(part.FunctionCall.Name, toolCounter)
					toolCounter++
					toolChunk := map[string]interface{}{
						"id":      streamID,
						"object":  "chat.completion.chunk",
						"created": time.Now().Unix(),
						"model":   model,
						"choices": []map[string]interface{}{
							{
								"index": 0,
								"delta": map[string]interface{}{
									"tool_calls": []map[string]interface{}{
										{
											"index": toolCounter - 1,
											"id":    toolID,
											"type":  "function",
											"function": map[string]interface{}{
												"name":      part.FunctionCall.Name,
												"arguments": args,
											},
										},
									},
								},
							},
						},
					}
					if err := writeOpenAISSEChunk(w, toolChunk); err != nil {
						return err
					}
				}
			}

			if candidate.FinishReason != "" {
				finalChunk := map[string]interface{}{
					"id":      streamID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   model,
					"choices": []map[string]interface{}{
						{
							"index":         0,
							"delta":         map[string]interface{}{},
							"finish_reason": mapGeminiFinishReason(candidate.FinishReason),
						},
					},
				}
				if usage != nil {
					finalChunk["usage"] = usage
				}
				if err := writeOpenAISSEChunk(w, finalChunk); err != nil {
					return err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	_, err := io.WriteString(w, "data: [DONE]\n\n")
	return err
}

// StreamGeminiSSEToAnthropic 将 Gemini SSE 转换为 Anthropic SSE。
func StreamGeminiSSEToAnthropic(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	streamID := generateStreamID()
	model := ""
	startEmitted := false
	textStarted := false
	textIndex := 0
	nextIndex := 1
	jsonFixer := NewPythonJSONFixer(nil)

	writeEvent := func(event string, payload interface{}) error {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		bytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", string(bytes)); err != nil {
			return err
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk geminiStreamingChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return err
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if !startEmitted {
			startEmitted = true
			if err := writeEvent("message_start", map[string]interface{}{
				"type": "message_start",
				"message": map[string]interface{}{
					"id":    streamID,
					"model": model,
					"role":  "assistant",
				},
			}); err != nil {
				return err
			}
		}

		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					if !textStarted {
						textStarted = true
						if err := writeEvent("content_block_start", map[string]interface{}{
							"type":  "content_block_start",
							"index": textIndex,
							"content_block": map[string]interface{}{
								"type": "text",
								"text": "",
							},
						}); err != nil {
							return err
						}
					}
					if err := writeEvent("content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": textIndex,
						"delta": map[string]interface{}{
							"type": "text_delta",
							"text": part.Text,
						},
					}); err != nil {
						return err
					}
				}
				if part.FunctionCall != nil {
					toolID := generateToolCallID(part.FunctionCall.Name, nextIndex)
					idx := nextIndex
					nextIndex++

					argsBytes, _ := json.Marshal(part.FunctionCall.Args)
					args := formatToolArguments(string(argsBytes), jsonFixer)

					if err := writeEvent("content_block_start", map[string]interface{}{
						"type":  "content_block_start",
						"index": idx,
						"content_block": map[string]interface{}{
							"type":  "tool_use",
							"id":    toolID,
							"name":  part.FunctionCall.Name,
							"input": map[string]interface{}{},
						},
					}); err != nil {
						return err
					}
					if err := writeEvent("content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": idx,
						"delta": map[string]interface{}{
							"type":         "input_json_delta",
							"partial_json": args,
						},
					}); err != nil {
						return err
					}
					if err := writeEvent("content_block_stop", map[string]interface{}{
						"type":  "content_block_stop",
						"index": idx,
					}); err != nil {
						return err
					}
				}
			}
			if candidate.FinishReason != "" {
				if err := writeEvent("message_delta", map[string]interface{}{
					"type": "message_delta",
					"delta": map[string]interface{}{
						"stop_reason": mapGeminiFinishReason(candidate.FinishReason),
					},
				}); err != nil {
					return err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if startEmitted {
		if textStarted {
			if err := writeEvent("content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": textIndex,
			}); err != nil {
				return err
			}
		}
		if err := writeEvent("message_stop", map[string]interface{}{
			"type": "message_stop",
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeFallbackSSE(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeOpenAISSEChunk(w io.Writer, chunk map[string]interface{}) error {
	bytes, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(bytes))
	return err
}

func mapAnthropicFinishReason(reason string) string {
	switch strings.ToLower(reason) {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop_sequence"
	case "", "end_turn":
		return "stop"
	default:
		return "stop"
	}
}

func mapGeminiFinishReason(reason string) string {
	switch strings.ToUpper(reason) {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "TOOL_CALLS":
		return "tool_calls"
	default:
		return "stop"
	}
}

func generateStreamID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}

func generateToolCallID(name string, index int) string {
	name = strings.ReplaceAll(name, " ", "_")
	return fmt.Sprintf("tool_%s_%d_%d", name, index, time.Now().UnixNano())
}

func formatToolArguments(raw string, fixer *PythonJSONFixer) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}"
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}
	if fixer != nil {
		if fixed, changed := fixer.FixPythonStyleJSON(trimmed); changed && json.Valid([]byte(fixed)) {
			return fixed
		}
	}
	return trimmed
}

type geminiStreamingChunk struct {
	Model      string `json:"model"`
	Candidates []struct {
		FinishReason string `json:"finishReason"`
		Content      struct {
			Parts []struct {
				Text          string                 `json:"text"`
				FunctionCall  *geminiFunctionCall    `json:"functionCall"`
				FunctionResp  map[string]interface{} `json:"functionResponse"`
				UnknownFields map[string]interface{} `json:"-"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}
