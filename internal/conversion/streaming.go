package conversion

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "strings"
    "time"
)

const (
	defaultScannerBuffer      = 64 * 1024
	defaultScannerMaxCapacity = 2 * 1024 * 1024
)

// StreamChatCompletionsToResponses 将 OpenAI Chat Completions SSE 转为 Codex Responses SSE
// 策略：
// - 纯文本增量：转换为 response.output_text.delta，并在开始发 response.created，结束发 response.completed
// - 含 tool_calls 的增量：直接透传原始 Chat Completions chunk，避免把函数参数当成文本注入
func StreamChatCompletionsToResponses(r io.Reader, w io.Writer) error {
    scanner := bufio.NewScanner(r)
    scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

    var responseID string
    var model string
    var createdEmitted bool
    var completedEmitted bool

    // 发出 Responses 事件
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
            "type":   "response.created",
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
        if err := emitCreatedIfNeeded(); err != nil { return err }
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
            if err := emitCompleted(); err != nil { return err }
            if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil { return err }
            continue
        }

        // 解析 Chat Completions chunk
        var chunk OpenAIStreamChunk
        if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
            // 无法解析则直接透传（容错）
            if _, err2 := io.WriteString(w, "data: "+dataContent+"\n\n"); err2 != nil { return err2 }
            continue
        }
        if chunk.Model != "" { model = chunk.Model }
        if chunk.ID != "" && responseID == "" { responseID = chunk.ID }

        // 如果包含 tool_calls，直接透传整个chunk，避免把函数参数当文本
        hasToolCalls := false
        for _, ch := range chunk.Choices {
            if len(ch.Delta.ToolCalls) > 0 { hasToolCalls = true; break }
        }
        if hasToolCalls {
            // 先确保 created 发出（避免客户端等待 response.created）
            if err := emitCreatedIfNeeded(); err != nil { return err }
            if _, err := io.WriteString(w, "data: "+dataContent+"\n\n"); err != nil { return err }
            continue
        }

        // 文本增量 → response.output_text.delta
        if err := emitCreatedIfNeeded(); err != nil { return err }
        for _, ch := range chunk.Choices {
            if text, ok := ch.Delta.Content.(string); ok && text != "" {
                if err := writeEvent("response.output_text.delta", map[string]interface{}{
                    "type":        "response.output_text.delta",
                    "delta":       text,
                    "response_id": responseID,
                }); err != nil { return err }
            }
        }
    }

    if err := scanner.Err(); err != nil {
        return err
    }

    if !completedEmitted {
        if err := emitCompleted(); err != nil { return err }
    }
    return nil
}

type anthropicStreamState struct {
	writer        io.Writer
	messageID     string
	model         string
	contentBlocks map[int]*contentBlockState
	currentIndex  int
	usage         *usageInfo
}

type contentBlockState struct {
	blockType string
	text      strings.Builder
	toolUseID string
	toolName  string
	toolInput strings.Builder
}

type usageInfo struct {
	inputTokens  int
	outputTokens int
}

func newAnthropicStreamState(w io.Writer) *anthropicStreamState {
	return &anthropicStreamState{
		writer:        w,
		contentBlocks: make(map[int]*contentBlockState),
		usage:         &usageInfo{},
	}
}

func (s *anthropicStreamState) handleEvent(eventType, data string) error {
	switch eventType {
	case "message_start":
		return s.handleMessageStart(data)
	case "content_block_start":
		return s.handleContentBlockStart(data)
	case "content_block_delta":
		return s.handleContentBlockDelta(data)
	case "content_block_stop":
		return s.handleContentBlockStop(data)
	case "message_delta":
		return s.handleMessageDelta(data)
	case "message_stop":
		return s.handleMessageStop()
	}
	return nil
}

func (s *anthropicStreamState) handleMessageStart(data string) error {
	var event struct {
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return err
	}

	s.messageID = event.Message.ID
	s.model = event.Message.Model
	s.usage.inputTokens = event.Message.Usage.InputTokens

	chunk := map[string]interface{}{
		"id":      s.messageID,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   s.model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"role": "assistant",
				},
				"finish_reason": nil,
			},
		},
	}

	return s.writeChunk(chunk)
}

func (s *anthropicStreamState) handleContentBlockStart(data string) error {
	var event struct {
		Index        int `json:"index"`
		ContentBlock struct {
			Type  string `json:"type"`
			ID    string `json:"id,omitempty"`
			Name  string `json:"name,omitempty"`
			Input struct {
			} `json:"input,omitempty"`
		} `json:"content_block"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return err
	}

	s.currentIndex = event.Index
	block := &contentBlockState{
		blockType: event.ContentBlock.Type,
	}

	if event.ContentBlock.Type == "tool_use" {
		block.toolUseID = event.ContentBlock.ID
		block.toolName = event.ContentBlock.Name

		chunk := map[string]interface{}{
			"id":      s.messageID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   s.model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{
							{
								"index": event.Index,
								"id":    event.ContentBlock.ID,
								"type":  "function",
								"function": map[string]interface{}{
									"name":      event.ContentBlock.Name,
									"arguments": "",
								},
			},
						},
					},
					"finish_reason": nil,
				},
			},
		}
		s.contentBlocks[event.Index] = block
		return s.writeChunk(chunk)
	}

	s.contentBlocks[event.Index] = block
	return nil
}

func (s *anthropicStreamState) handleContentBlockDelta(data string) error {
	var event struct {
		Index int `json:"index"`
		Delta struct {
			Type       string `json:"type"`
			Text       string `json:"text,omitempty"`
			PartialJSON string `json:"partial_json,omitempty"`
		} `json:"delta"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return err
	}

	block, exists := s.contentBlocks[event.Index]
	if !exists {
		return nil
	}

	if event.Delta.Type == "text_delta" {
		block.text.WriteString(event.Delta.Text)

		chunk := map[string]interface{}{
			"id":      s.messageID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   s.model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": event.Delta.Text,
					},
					"finish_reason": nil,
				},
			},
		}
		return s.writeChunk(chunk)
	}

	if event.Delta.Type == "input_json_delta" {
		block.toolInput.WriteString(event.Delta.PartialJSON)

		chunk := map[string]interface{}{
			"id":      s.messageID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   s.model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{
							{
								"index": event.Index,
								"function": map[string]interface{}{
									"arguments": event.Delta.PartialJSON,
								},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		}
		return s.writeChunk(chunk)
	}

	return nil
}

func (s *anthropicStreamState) handleContentBlockStop(data string) error {
	return nil
}

func (s *anthropicStreamState) handleMessageDelta(data string) error {
	var event struct {
		Delta struct {
			StopReason   string `json:"stop_reason,omitempty"`
			StopSequence string `json:"stop_sequence,omitempty"`
		} `json:"delta"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return err
	}

	s.usage.outputTokens = event.Usage.OutputTokens

	if event.Delta.StopReason != "" {
		finishReason := convertStopReason(event.Delta.StopReason)

		chunk := map[string]interface{}{
			"id":      s.messageID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   s.model,
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         map[string]interface{}{},
					"finish_reason": finishReason,
				},
			},
		}
		return s.writeChunk(chunk)
	}

	return nil
}

func (s *anthropicStreamState) handleMessageStop() error {
	return s.writeDone()
}

func (s *anthropicStreamState) writeChunk(chunk map[string]interface{}) error {
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(s.writer, "data: %s\n\n", string(data)); err != nil {
		return err
	}

	return nil
}

func (s *anthropicStreamState) writeDone() error {
	if _, err := io.WriteString(s.writer, "data: [DONE]\n\n"); err != nil {
		return err
	}
	return nil
}

func convertStopReason(anthropicReason string) string {
	switch anthropicReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

// StreamAnthropicSSEToOpenAI 将 Anthropic Messages SSE 流转换为 OpenAI Chat Completions SSE 流
func StreamAnthropicSSEToOpenAI(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	state := newAnthropicStreamState(w)
	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if err := state.handleEvent(currentEvent, data); err != nil {
				return err
			}
			currentEvent = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
