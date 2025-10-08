package toolcall

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptGenerator generates system prompts for tool calling
type PromptGenerator struct {
	triggerGen *TriggerSignalGenerator
}

// NewPromptGenerator creates a new prompt generator
func NewPromptGenerator() *PromptGenerator {
	return &PromptGenerator{
		triggerGen: NewTriggerSignalGenerator(),
	}
}

// GeneratePrompt creates a system prompt with tool descriptions
func (p *PromptGenerator) GeneratePrompt(tools []Tool) (string, string) {
	triggerSignal := p.triggerGen.Generate()
	toolsList := p.buildToolsList(tools)

	template := p.getPromptTemplate(triggerSignal)
	prompt := strings.Replace(template, "{tools_list}", toolsList, 1)

	return prompt, triggerSignal
}

// buildToolsList creates formatted tool descriptions
func (p *PromptGenerator) buildToolsList(tools []Tool) string {
	var builder strings.Builder

	for i, tool := range tools {
		name := tool.Function.Name
		description := tool.Function.Description
		params := tool.Function.Parameters

		// Extract schema details
		props, _ := params["properties"].(map[string]interface{})
		required, _ := params["required"].([]interface{})
		requiredList := make([]string, 0, len(required))
		for _, r := range required {
			if str, ok := r.(string); ok {
				requiredList = append(requiredList, str)
			}
		}

		// Build parameter summary
		paramsSummary := p.buildParamsSummary(props)

		// Build detailed parameter specs
		detailBlock := p.buildParamDetails(props, requiredList)

		// Format description block
		descBlock := "None"
		if description != "" {
			descBlock = fmt.Sprintf("```\n%s\n```", description)
		}

		// Format required parameters
		requiredStr := "None"
		if len(requiredList) > 0 {
			requiredStr = strings.Join(requiredList, ", ")
		}

		builder.WriteString(fmt.Sprintf("%d. <tool name=\"%s\">\n", i+1, name))
		builder.WriteString(fmt.Sprintf("   Description:\n%s\n", descBlock))
		builder.WriteString(fmt.Sprintf("   Parameters summary: %s\n", paramsSummary))
		builder.WriteString(fmt.Sprintf("   Required parameters: %s\n", requiredStr))
		builder.WriteString(fmt.Sprintf("   Parameter details:\n%s\n", detailBlock))

		if i < len(tools)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// buildParamsSummary creates a brief summary of parameters
func (p *PromptGenerator) buildParamsSummary(props map[string]interface{}) string {
	if len(props) == 0 {
		return "None"
	}

	var params []string
	for name, propData := range props {
		if propMap, ok := propData.(map[string]interface{}); ok {
			paramType, _ := propMap["type"].(string)
			if paramType == "" {
				paramType = "any"
			}
			params = append(params, fmt.Sprintf("%s (%s)", name, paramType))
		}
	}

	return strings.Join(params, ", ")
}

// buildParamDetails creates detailed parameter specifications
func (p *PromptGenerator) buildParamDetails(props map[string]interface{}, requiredList []string) string {
	if len(props) == 0 {
		return "(no parameter details)"
	}

	requiredMap := make(map[string]bool)
	for _, r := range requiredList {
		requiredMap[r] = true
	}

	var builder strings.Builder
	for name, propData := range props {
		propMap, ok := propData.(map[string]interface{})
		if !ok {
			continue
		}

		paramType, _ := propMap["type"].(string)
		if paramType == "" {
			paramType = "any"
		}

		isRequired := "No"
		if requiredMap[name] {
			isRequired = "Yes"
		}

		builder.WriteString(fmt.Sprintf("- %s:\n", name))
		builder.WriteString(fmt.Sprintf("  - type: %s\n", paramType))
		builder.WriteString(fmt.Sprintf("  - required: %s\n", isRequired))

		if desc, ok := propMap["description"].(string); ok && desc != "" {
			builder.WriteString(fmt.Sprintf("  - description: %s\n", desc))
		}

		if enum, ok := propMap["enum"]; ok {
			if enumJSON, err := json.Marshal(enum); err == nil {
				builder.WriteString(fmt.Sprintf("  - enum: %s\n", enumJSON))
			}
		}

		if defVal, ok := propMap["default"]; ok {
			if defJSON, err := json.Marshal(defVal); err == nil {
				builder.WriteString(fmt.Sprintf("  - default: %s\n", defJSON))
			}
		}

		// Add constraints if present
		constraints := p.extractConstraints(propMap)
		if constraints != "" {
			builder.WriteString(fmt.Sprintf("  - constraints: %s\n", constraints))
		}
	}

	return builder.String()
}

// extractConstraints extracts JSON Schema constraints
func (p *PromptGenerator) extractConstraints(propMap map[string]interface{}) string {
	constraints := make(map[string]interface{})
	constraintKeys := []string{
		"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
		"minLength", "maxLength", "pattern", "format",
		"minItems", "maxItems", "uniqueItems",
	}

	for _, key := range constraintKeys {
		if val, ok := propMap[key]; ok {
			constraints[key] = val
		}
	}

	// Handle array items
	if propMap["type"] == "array" {
		if items, ok := propMap["items"].(map[string]interface{}); ok {
			if itemType, ok := items["type"].(string); ok {
				constraints["items.type"] = itemType
			}
		}
	}

	if len(constraints) == 0 {
		return ""
	}

	if jsonData, err := json.Marshal(constraints); err == nil {
		return string(jsonData)
	}

	return ""
}

// getPromptTemplate returns the system prompt template
func (p *PromptGenerator) getPromptTemplate(triggerSignal string) string {
	return fmt.Sprintf(`You have access to the following available tools to help solve problems:

{tools_list}

**IMPORTANT CONTEXT NOTES:**
1. You can call MULTIPLE tools in a single response if needed.
2. The conversation context may already contain tool execution results from previous function calls. Review the conversation history carefully to avoid unnecessary duplicate tool calls.
3. When tool execution results are present in the context, they will be formatted with XML tags like <tool_result>...</tool_result> for easy identification.
4. This is the ONLY format you can use for tool calls, and any deviation will result in failure.

When you need to use tools, you **MUST** strictly follow this format. Do NOT include any extra text, explanations, or dialogue on the first and second lines of the tool call syntax:

1. When starting tool calls, begin on a new line with exactly:
%s
No leading or trailing spaces, output exactly as shown above. The trigger signal MUST be on its own line and appear only once.

2. Starting from the second line, **immediately** follow with the complete <function_calls> XML block.

3. For multiple tool calls, include multiple <function_call> blocks within the same <function_calls> wrapper.

4. Do not add any text or explanation after the closing </function_calls> tag.

STRICT ARGUMENT KEY RULES:
- You MUST use parameter keys EXACTLY as defined (case- and punctuation-sensitive). Do NOT rename, add, or remove characters.
- If a key starts with a hyphen (e.g., -i, -C), you MUST keep the hyphen in the tag name. Example: <-i>true</-i>, <-C>2</-C>.
- Never convert "-i" to "i" or "-C" to "C". Do not pluralize, translate, or alias parameter keys.
- The <tool> tag must contain the exact name of a tool from the list. Any other tool name is invalid.
- The <args> must contain all required arguments for that tool.

CORRECT Example (multiple tool calls, including hyphenated keys):
...response content (optional)...
%s
<function_calls>
    <function_call>
        <tool>Grep</tool>
        <args>
            <-i>true</-i>
            <-C>2</-C>
            <path>.</path>
        </args>
    </function_call>
    <function_call>
        <tool>search</tool>
        <args>
            <keywords>["Python Document", "how to use python"]</keywords>
        </args>
    </function_call>
</function_calls>

INCORRECT Example (extra text + wrong key names — DO NOT DO THIS):
...response content (optional)...
%s
I will call the tools for you.
<function_calls>
    <function_call>
        <tool>Grep</tool>
        <args>
            <i>true</i>
            <C>2</C>
            <path>.</path>
        </args>
    </function_call>
</function_calls>

Now please be ready to strictly follow the above specifications.`, triggerSignal, triggerSignal, triggerSignal)
}
