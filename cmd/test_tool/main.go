package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	reqBody := map[string]interface{}{
		"model":        "gpt-5",
		"instructions": "You are a helpful assistant. When user asks to search files, use glob_search function.",
		"input": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Use glob_search to find config.yaml",
					},
				},
			},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "glob_search",
					"description": "Search for files matching a glob pattern",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"pattern": map[string]interface{}{
								"type":        "string",
								"description": "Glob pattern",
							},
						},
						"required": []string{"pattern"},
					},
				},
			},
		},
		"stream": false,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8081/responses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	prettyJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(prettyJSON))

	// 检查是否有 function_call
	if output, ok := result["output"].([]interface{}); ok && len(output) > 0 {
		lastItem := output[len(output)-1].(map[string]interface{})
		fmt.Printf("\nLast output item type: %v\n", lastItem["type"])
		if lastItem["type"] == "function_call" {
			fmt.Println("✅ Function call detected!")
		} else {
			fmt.Println("❌ No function call, only text response")
		}
	}
}
