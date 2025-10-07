package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	outputJSON := flag.Bool("json", false, "以 JSON 格式输出结果")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	results, cleanup, err := web.RunEndpointBatchTest(cfg, *configPath)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "执行端点测试失败: %v\n", err)
		os.Exit(1)
	}

	if *outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			fmt.Fprintf(os.Stderr, "编码结果失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("批量端点测试结果:")
	for _, res := range results {
		fmt.Printf("\n端点: %s (耗时 %dms)\n", res.EndpointName, res.TotalTime)
		if len(res.Results) == 0 {
			fmt.Println("  无测试结果")
			continue
		}
		for _, r := range res.Results {
			status := "成功"
			if !r.Success {
				status = "失败"
			}
			fmt.Printf("  - 格式:%-9s 状态:%-2s HTTP:%3d 耗时:%4dms URL:%s\n", r.Format, status, r.StatusCode, r.ResponseTime, r.URL)
			if r.Error != "" {
				fmt.Printf("      错误: %s\n", r.Error)
			}
		}
	}
}
