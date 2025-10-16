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
	// 流式转换超时设置
	streamTimeout = 30 * time.Second
	// 最大重试次数
	maxRetries = 3
	// 最小数据包大小，避免处理过小的chunk
	minChunkSize = 10
)

// StreamChatCompletionsToResponses 将OpenAI Chat Completions SSE转换为Responses SSE (标准版本)
func StreamChatCompletionsToResponses(r io.Reader, w io.Writer) error {
	return StreamChatCompletionsToResponsesRobust(r, w)
}

// StreamChatCompletionsToResponsesRobust 增强容错性的流式转换
func StreamChatCompletionsToResponsesRobust(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	var chunks []OpenAIStreamChunk
	respID := generateResponseID()
	var model string

	// 流式读取和处理SSE数据
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        // 跳过空行和注释
        if line == "" || strings.HasPrefix(line, ":") {
            continue
        }

        // 处理data行（兼容 "data:" 与 "data: ")
        if strings.HasPrefix(line, "data:") {
            dataContent := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

            // 检查结束标记
            if dataContent == "[DONE]" {
                break
            }

			// 检查最小数据包大小
			if len(dataContent) < minChunkSize {
				continue
			}

            // 解析JSON chunk
            var chunk OpenAIStreamChunk
            if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
				// 记录错误但不中断流式处理
				continue // 跳过无效chunk
            }

			// 验证chunk的基本结构
			if chunk.ID == "" && len(chunks) == 0 {
				continue // 跳过没有ID的初始chunk
			}

            chunks = append(chunks, chunk)

            // 记录响应信息
            if chunk.ID != "" {
                respID = chunk.ID
            }
            if chunk.Model != "" {
                model = chunk.Model
            }
        }
    }

	if err := scanner.Err(); err != nil {
		return err
	}

	// 如果没有有效的chunks，生成一个最小响应避免空流
	if len(chunks) == 0 {
		chunks = append(chunks, OpenAIStreamChunk{
			ID:     respID,
			Model:  model,
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIMessage{
						Role: "assistant",
						Content: "",
					},
					FinishReason: "",
				},
			},
		})
	}

	// 构建并写入Responses格式的SSE
    sse, err := buildResponsesSSEFromChunks(chunks, respID, model, nil)
	if err != nil {
		return err
	}

	_, err = w.Write(sse)
	return err
}

func StreamChatCompletionsToResponsesUnified(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	var chunks []OpenAIStreamChunk
	respID := generateResponseID()
	var model string
	// var usage *OpenAIUsage // 暂时不使用

	// 流式读取和处理SSE数据
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        
        // 跳过空行和注释
        if line == "" || strings.HasPrefix(line, ":") {
            continue
        }
        
        // 处理data行（兼容 "data:" 与 "data: ")
        if strings.HasPrefix(line, "data:") {
            dataContent := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
            
            // 检查结束标记
            if dataContent == "[DONE]" {
                break
            }
            
            // 解析JSON chunk
            var chunk OpenAIStreamChunk
            if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
                continue // 跳过无效chunk
            }
            
            chunks = append(chunks, chunk)
            
            // 记录响应信息
            if chunk.ID != "" {
                respID = chunk.ID
            }
            if chunk.Model != "" {
                model = chunk.Model
            }
            // 暂时不处理usage，因为我们需要在流结束时收集
        }
    }

	if err := scanner.Err(); err != nil {
		return err
	}

	// 构建并写入Responses格式的SSE
    sse, err := buildResponsesSSEFromChunks(chunks, respID, model, nil)
	if err != nil {
		return err
	}
	
	_, err = w.Write(sse)
	return err
}

// StreamAnthropicSSEToResponses 已弃用，移除以简化代码路径（使用方应先统一为 OpenAI 再转换为 Responses）

// StreamResponsesToChat 将 Responses SSE 转换为 Chat SSE
func StreamResponsesToChat(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, defaultScannerBuffer), defaultScannerMaxCapacity)

	var chunks []OpenAIStreamChunk
	respID := generateResponseID()
	var model string

	// 解析Responses格式并转换为OpenAI chunks
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			// 读取下一行的data
			if scanner.Scan() {
				dataLine := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(dataLine, "data: ") {
					dataContent := strings.TrimPrefix(dataLine, "data: ")
					
					chunk, err := convertResponsesEventToChatChunk(eventType, dataContent, respID, model)
					if err != nil {
						continue
					}
					if chunk != nil {
						chunks = append(chunks, *chunk)
						if chunk.Model != "" {
							model = chunk.Model
						}
						// 暂时不处理usage
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// 构建Chat格式的SSE
	return buildChatSSEFromChunks(chunks, w)
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
			Type        string `json:"type"`
			Text        string `json:"text,omitempty"`
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
