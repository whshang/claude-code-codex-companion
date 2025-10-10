package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"claude-code-codex-companion/internal/config"
)

type iterationStats struct {
	firstByte time.Duration
	total     time.Duration
	status    int
}

type iterationMeasurement struct {
	Index        int           `json:"index"`
	Status       int           `json:"status"`
	FirstByte    time.Duration `json:"first_byte"`
	Total        time.Duration `json:"total"`
	ErrorMessage string        `json:"error,omitempty"`
}

type benchmarkRecord struct {
	Endpoint       string                 `json:"endpoint"`
	URL            string                 `json:"url"`
	Model          string                 `json:"model"`
	Prompt         string                 `json:"prompt"`
	Agent          string                 `json:"agent"`
	Time           time.Time              `json:"time"`
	Iterations     []iterationMeasurement `json:"iterations"`
	MemAlloc       uint64                 `json:"mem_alloc"`
	MemTotalAlloc  uint64                 `json:"mem_total_alloc"`
	MemNumGC       uint32                 `json:"mem_num_gc"`
	OutputFilePath string                 `json:"output_file"`
}

func main() {
	var (
		configPath   = flag.String("config", "config.yaml", "配置文件路径")
		endpointName = flag.String("endpoint", "", "目标端点名称（必填）")
		baseURL      = flag.String("base-url", "", "覆盖端点基础URL，可选")
		model        = flag.String("model", "", "请求使用的模型名称（默认读取配置或手动指定）")
		prompt       = flag.String("prompt", "请简要介绍一下你自己，控制在三句话以内。", "用户提示词内容")
		iterations   = flag.Int("iterations", 5, "请求迭代次数")
		timeout      = flag.Duration("timeout", 90*time.Second, "单次请求超时时间")
		agent        = flag.String("agent", "CCCC Stream Benchmark", "User-Agent 标识")
		outputPath   = flag.String("output", "", "结果输出文件（默认写入 logs/benchmarks/stream-YYYYmmdd-HHMMSS.txt）")
	)

	flag.Parse()

	if *endpointName == "" {
		fmt.Fprintln(os.Stderr, "必须指定 --endpoint")
		os.Exit(2)
	}
	if *iterations <= 0 {
		fmt.Fprintln(os.Stderr, "--iterations 必须为正整数")
		os.Exit(2)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	epConfig, err := findEndpoint(cfg, *endpointName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	finalURL := resolveTargetURL(epConfig, *baseURL)
	if finalURL == "" {
		fmt.Fprintf(os.Stderr, "端点 %s 缺少 URLOpenAI，无法基准测试\n", epConfig.Name)
		os.Exit(1)
	}

	resolvedModel := *model
	if resolvedModel == "" {
		resolvedModel = deriveDefaultModel(epConfig)
	}
	if resolvedModel == "" {
		fmt.Fprintln(os.Stderr, "无法推断模型名称，请使用 --model 指定")
		os.Exit(2)
	}

	client := &http.Client{Timeout: *timeout}
	stats := make([]iterationStats, 0, *iterations)
	record := benchmarkRecord{
		Endpoint:   epConfig.Name,
		URL:        finalURL,
		Model:      resolvedModel,
		Prompt:     *prompt,
		Agent:      *agent,
		Time:       time.Now(),
		Iterations: make([]iterationMeasurement, 0, *iterations),
	}

	for i := 0; i < *iterations; i++ {
		payload, err := buildRequestBody(resolvedModel, *prompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "构造请求体失败: %v\n", err)
			os.Exit(1)
		}

		req, err := http.NewRequest(http.MethodPost, finalURL, bytes.NewReader(payload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "构造请求失败: %v\n", err)
			os.Exit(1)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("User-Agent", *agent)

		applyAuthHeaders(req, epConfig)

		iterationStart := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "请求失败 (迭代 %d): %v\n", i+1, err)
			stats = append(stats, iterationStats{status: 0})
			record.Iterations = append(record.Iterations, iterationMeasurement{
				Index:        i + 1,
				Status:       0,
				FirstByte:    0,
				Total:        0,
				ErrorMessage: err.Error(),
			})
			continue
		}

		firstByteDur, totalDur := consumeStream(resp.Body)
		resp.Body.Close()

		stats = append(stats, iterationStats{firstByte: firstByteDur, total: totalDur, status: resp.StatusCode})
		record.Iterations = append(record.Iterations, iterationMeasurement{
			Index:     i + 1,
			Status:    resp.StatusCode,
			FirstByte: firstByteDur,
			Total:     totalDur,
		})

		fmt.Printf("迭代 %d/%d | 状态 %d | 首字节 %s | 总耗时 %s\n", i+1, *iterations, resp.StatusCode, firstByteDur, totalDur)

		remaining := iterationStart.Add(500 * time.Millisecond).Sub(time.Now())
		if remaining > 0 {
			time.Sleep(remaining)
		}
	}

	memStats := printSummary(stats)
	if memStats != nil {
		record.MemAlloc = memStats.Alloc
		record.MemTotalAlloc = memStats.TotalAlloc
		record.MemNumGC = memStats.NumGC
	}

	if err := persistRecord(&record, *outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "写入基准结果失败: %v\n", err)
	}
}

func findEndpoint(cfg *config.Config, name string) (*config.EndpointConfig, error) {
	for _, ep := range cfg.Endpoints {
		if ep.Name == name {
			return &ep, nil
		}
	}
	return nil, fmt.Errorf("未找到端点: %s", name)
}

func resolveTargetURL(ep *config.EndpointConfig, override string) string {
	base := strings.TrimSpace(override)
	if base == "" {
		base = strings.TrimSpace(ep.URLOpenAI)
	}
	if base == "" {
		return ""
	}
	base = strings.TrimRight(base, "/")
	return base + path.Join("", "/chat/completions")
}

func deriveDefaultModel(ep *config.EndpointConfig) string {
	if ep.ModelRewrite != nil && ep.ModelRewrite.Enabled {
		for _, rule := range ep.ModelRewrite.Rules {
			if strings.Contains(strings.ToLower(rule.TargetModel), "gpt") {
				return rule.TargetModel
			}
		}
	}
	return "gpt-5"
}

func buildRequestBody(model, prompt string) ([]byte, error) {
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{{
			"role":    "user",
			"content": prompt,
		}},
		"stream":      true,
		"temperature": 0,
	}
	return json.Marshal(body)
}

func applyAuthHeaders(req *http.Request, ep *config.EndpointConfig) {
	value := strings.TrimSpace(ep.AuthValue)
	if value == "" {
		return
	}
	switch strings.ToLower(ep.AuthType) {
	case "api_key", "x-api-key":
		req.Header.Set("x-api-key", value)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", value))
	case "auth_token", "bearer", "":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", value))
	case "auto":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", value))
		req.Header.Set("x-api-key", value)
	case "oauth":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", value))
	default:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", value))
	}
}

func consumeStream(body io.ReadCloser) (time.Duration, time.Duration) {
	start := time.Now()
	scanner := bufio.NewScanner(body)
	firstByte := time.Duration(0)

	for scanner.Scan() {
		line := scanner.Text()
		if firstByte == 0 && (strings.HasPrefix(line, "data: ") || strings.HasPrefix(line, "event:")) {
			firstByte = time.Since(start)
		}
		if strings.TrimSpace(line) == "data: [DONE]" {
			break
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		firstByte = 0
	}

	total := time.Since(start)
	return firstByte, total
}

func printSummary(stats []iterationStats) *runtime.MemStats {
	valid := make([]iterationStats, 0, len(stats))
	for _, s := range stats {
		if s.firstByte > 0 && s.total > 0 && s.status >= 200 && s.status < 300 {
			valid = append(valid, s)
		}
	}

	if len(valid) == 0 {
		fmt.Println("无有效样本，检查端点可用性或认证信息。")
		return nil
	}

	firstBytes := make([]time.Duration, len(valid))
	totals := make([]time.Duration, len(valid))
	for i, s := range valid {
		firstBytes[i] = s.firstByte
		totals[i] = s.total
	}

	fmt.Println("--- 基准统计 ---")
	printDurationStats("首字节延迟", firstBytes)
	printDurationStats("总耗时", totals)

	runtime.GC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Printf("当前进程内存：Alloc=%0.2fMB TotalAlloc=%0.2fMB NumGC=%d\n",
		float64(mem.Alloc)/1024.0/1024.0,
		float64(mem.TotalAlloc)/1024.0/1024.0,
		mem.NumGC)
	return &mem
}

func printDurationStats(label string, values []time.Duration) {
	if len(values) == 0 {
		return
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	avg := time.Duration(0)
	for _, v := range sorted {
		avg += v
	}
	avg /= time.Duration(len(sorted))

	p95 := percentile(sorted, 0.95)
	p99 := percentile(sorted, 0.99)

	fmt.Printf("%s | 样本 %d | 平均 %s | P95 %s | 最小 %s | 最大 %s | P99 %s\n",
		label,
		len(sorted),
		avg,
		p95,
		sorted[0],
		sorted[len(sorted)-1],
		p99,
	)
}

func percentile(sorted []time.Duration, perc float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if perc <= 0 {
		return sorted[0]
	}
	if perc >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := perc * float64(len(sorted)-1)
	idx := int(pos)
	fraction := pos - float64(idx)
	if idx+1 < len(sorted) {
		return sorted[idx] + time.Duration(float64(sorted[idx+1]-sorted[idx])*fraction)
	}
	return sorted[idx]
}

func persistRecord(record *benchmarkRecord, override string) error {
	if record == nil {
		return nil
	}

	var output string
	if strings.TrimSpace(override) != "" {
		output = override
	} else {
		baseDir := filepath.Join("logs", "benchmarks")
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			return err
		}
		filename := fmt.Sprintf("stream-%s.txt", record.Time.Format("20060102-150405"))
		output = filepath.Join(baseDir, filename)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# Stream Benchmark\nTime: %s\nEndpoint: %s\nURL: %s\nModel: %s\nAgent: %s\n\n", record.Time.Format(time.RFC3339), record.Endpoint, record.URL, record.Model, record.Agent)
	for _, it := range record.Iterations {
		errText := it.ErrorMessage
		if errText == "" {
			errText = "-"
		}
		fmt.Fprintf(&buf, "Iteration %d | Status %d | FirstByte %s | Total %s | Error %s\n", it.Index, it.Status, it.FirstByte, it.Total, errText)
	}
	fmt.Fprintf(&buf, "\nMem Alloc: %d bytes\nMem TotalAlloc: %d bytes\nMem NumGC: %d\n", record.MemAlloc, record.MemTotalAlloc, record.MemNumGC)

	if err := os.WriteFile(output, buf.Bytes(), 0o644); err != nil {
		return err
	}
	record.OutputFilePath = output
	fmt.Printf("结果已写入 %s\n", output)
	return nil
}
