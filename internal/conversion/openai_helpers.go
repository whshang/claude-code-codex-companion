package conversion

// normalizeOpenAIFinishReason maps OpenAI finish reasons to internal reasons
func normalizeOpenAIFinishReason(reason string) string {
    switch reason {
    case "tool_calls":
        return "tool_use"
    case "length":
        return "max_tokens"
    case "stop_sequence":
        return "stop_sequence"
    case "stop", "":
        return "end_turn"
    default:
        return "end_turn"
    }
}

// detectMediaType extracts media type when URL is data URI; otherwise empty
func detectMediaType(url string) string {
    if url == "" {
        return ""
    }
    if len(url) > 5 && url[:5] == "data:" {
        // data:<media>;base64,...
        for i := 5; i < len(url); i++ {
            if url[i] == ';' || url[i] == ',' {
                return url[5:i]
            }
        }
        return ""
    }
    return ""
}
