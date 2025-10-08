package proxy

import (
    "encoding/json"
    "strings"
    "time"

    "github.com/google/uuid"

    "claude-code-codex-companion/internal/toolcall"
)

// extractToolsFromClientRequest parses incoming client request (OpenAI/Anthropic) and
// returns a normalized list of Tool definitions (OpenAI function schema compatible).
func extractToolsFromClientRequest(body []byte) []toolcall.Tool {
    var m map[string]interface{}
    if err := json.Unmarshal(body, &m); err != nil {
        return nil
    }
    rawTools, ok := m["tools"].([]interface{})
    if !ok || len(rawTools) == 0 {
        return nil
    }
    tools := make([]toolcall.Tool, 0, len(rawTools))
    for _, it := range rawTools {
        tm, ok := it.(map[string]interface{})
        if !ok {
            continue
        }

        // OpenAI shape: {type:"function", function:{name, description, parameters}}
        if funcAny, ok := tm["function"]; ok {
            if fn, ok := funcAny.(map[string]interface{}); ok {
                name, _ := fn["name"].(string)
                desc, _ := fn["description"].(string)
                params, _ := fn["parameters"].(map[string]interface{})
                if name != "" {
                    tools = append(tools, toolcall.Tool{
                        Type:     "function",
                        Function: toolcall.ToolFunction{Name: name, Description: desc, Parameters: params},
                    })
                    continue
                }
            }
        }

        // Anthropic shape: {name, description, input_schema}
        if name, ok := tm["name"].(string); ok && name != "" {
            desc, _ := tm["description"].(string)
            params, _ := tm["input_schema"].(map[string]interface{})
            tools = append(tools, toolcall.Tool{
                Type:     "function",
                Function: toolcall.ToolFunction{Name: name, Description: desc, Parameters: params},
            })
        }
    }
    if len(tools) == 0 {
        return nil
    }
    return tools
}

// injectSystemPromptToClientRequest injects or appends system prompt for OpenAI/Anthropic client formats.
// - OpenAI: prepend/append to system message (messages[].role=="system") or add one at the start.
// - Anthropic: use top-level system string if present, else set it.
func injectSystemPromptToClientRequest(body []byte, clientFormat string, systemPrompt string) ([]byte, error) {
    var m map[string]interface{}
    if err := json.Unmarshal(body, &m); err != nil {
        return body, err
    }

    format := strings.ToLower(clientFormat)
    if format == "openai" || format == "" {
        // messages: [ {role, content} ]
        if arrAny, ok := m["messages"].([]interface{}); ok {
            // find first system message
            sysIdx := -1
            for i, msgAny := range arrAny {
                if msg, ok := msgAny.(map[string]interface{}); ok {
                    if role, _ := msg["role"].(string); role == "system" {
                        sysIdx = i
                        break
                    }
                }
            }
            if sysIdx >= 0 {
                if msg, ok := arrAny[sysIdx].(map[string]interface{}); ok {
                    // content could be string or array; append as string
                    if c, ok := msg["content"].(string); ok {
                        if c == "" {
                            msg["content"] = systemPrompt
                        } else {
                            msg["content"] = c + "\n\n" + systemPrompt
                        }
                    } else {
                        // force set string
                        msg["content"] = systemPrompt
                    }
                    arrAny[sysIdx] = msg
                }
            } else {
                // prepend new system message
                newMsg := map[string]interface{}{
                    "role":    "system",
                    "content": systemPrompt,
                }
                m["messages"] = append([]interface{}{newMsg}, arrAny...)
            }
        } else {
            // if no messages array, create it
            m["messages"] = []interface{}{
                map[string]interface{}{"role": "system", "content": systemPrompt},
            }
        }
    } else if format == "anthropic" {
        if s, ok := m["system"].(string); ok {
            if s == "" {
                m["system"] = systemPrompt
            } else {
                m["system"] = s + "\n\n" + systemPrompt
            }
        } else {
            // set system field
            m["system"] = systemPrompt
        }
    }

    out, err := json.Marshal(m)
    if err != nil {
        return body, err
    }
    return out, nil
}

// extractAssistantTextForToolDetect extracts the assistant textual content from a non-streaming response
// for the given endpoint format ("openai" or "anthropic"). It returns empty string if extraction fails.
func extractAssistantTextForToolDetect(body []byte, endpointFormat string) string {
    var m map[string]interface{}
    if err := json.Unmarshal(body, &m); err != nil {
        return ""
    }
    format := strings.ToLower(endpointFormat)
    if format == "openai" || format == "" {
        // choices[0].message.content (string)
        if choices, ok := m["choices"].([]interface{}); ok && len(choices) > 0 {
            if first, ok := choices[0].(map[string]interface{}); ok {
                if msg, ok := first["message"].(map[string]interface{}); ok {
                    if c, ok := msg["content"].(string); ok {
                        return c
                    }
                }
            }
        }
    } else if format == "anthropic" {
        // content: [ {type:"text", text:"..."}, ... ]
        var b strings.Builder
        if contentArr, ok := m["content"].([]interface{}); ok {
            for _, it := range contentArr {
                if blk, ok := it.(map[string]interface{}); ok {
                    if t, _ := blk["type"].(string); t == "text" {
                        if tx, _ := blk["text"].(string); tx != "" {
                            b.WriteString(tx)
                            b.WriteString("\n")
                        }
                    }
                }
            }
        }
        return b.String()
    }
    return ""
}

// parseModelFromResponse tries to read the model string from the response body.
func parseModelFromResponse(body []byte) string {
    var m map[string]interface{}
    if err := json.Unmarshal(body, &m); err == nil {
        if model, ok := m["model"].(string); ok {
            return model
        }
    }
    return ""
}

// buildOpenAIToolCallResponse constructs a minimal valid OpenAI chat.completion tool_calls response.
func buildOpenAIToolCallResponse(model string, toolCalls []toolcall.ToolCall) []byte {
    now := time.Now().Unix()
    resp := map[string]interface{}{
        "id":      "chatcmpl-" + uuid.New().String(),
        "object":  "chat.completion",
        "created": now,
        "model":   model,
        "choices": []interface{}{
            map[string]interface{}{
                "index": 0,
                "message": map[string]interface{}{
                    "role":       "assistant",
                    "content":    nil,
                    "tool_calls": toOpenAIToolCalls(toolCalls),
                },
                "finish_reason": "tool_calls",
            },
        },
    }
    out, _ := json.Marshal(resp)
    return out
}

func toOpenAIToolCalls(toolCalls []toolcall.ToolCall) []interface{} {
    out := make([]interface{}, 0, len(toolCalls))
    for _, tc := range toolCalls {
        out = append(out, map[string]interface{}{
            "id":   tc.ID,
            "type": "function",
            "function": map[string]interface{}{
                "name":      tc.Function.Name,
                "arguments": tc.Function.ArgumentsJSON,
            },
        })
    }
    return out
}

// buildAnthropicToolCallResponse constructs a minimal valid Anthropic message with tool_use blocks.
func buildAnthropicToolCallResponse(model string, toolCalls []toolcall.ToolCall) []byte {
    content := make([]interface{}, 0, len(toolCalls))
    for _, tc := range toolCalls {
        content = append(content, map[string]interface{}{
            "type":       "tool_use",
            "id":         tc.ID,
            "name":       tc.Function.Name,
            "input":      tc.Function.Arguments,
        })
    }
    resp := map[string]interface{}{
        "id":     "msg_" + uuid.New().String(),
        "type":   "message",
        "role":   "assistant",
        "model":  model,
        "content": content,
    }
    out, _ := json.Marshal(resp)
    return out
}
