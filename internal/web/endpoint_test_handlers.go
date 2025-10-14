package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"claude-code-codex-companion/internal/endpoint"

	"github.com/gin-gonic/gin"
)

// handleTestEndpoint 测试单个端点
func (s *AdminServer) handleTestEndpoint(c *gin.Context) {
	encodedEndpointName := c.Param("id")
	endpointName, err := url.PathUnescape(encodedEndpointName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endpoint name encoding"})
		return
	}

	// 从所有端点中查找
	allEndpoints := s.endpointManager.GetAllEndpoints()
	var targetEndpoint *endpoint.Endpoint
	for _, ep := range allEndpoints {
		if ep.Name == endpointName {
			targetEndpoint = ep
			break
		}
	}

	if targetEndpoint == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 执行测试
	result := s.testSingleEndpoint(targetEndpoint)

	c.JSON(http.StatusOK, result)
}

// handleTestAllEndpoints 批量测试所有端点
func (s *AdminServer) handleTestAllEndpoints(c *gin.Context) {
	results := s.testAllEndpoints()

	// 触发动态排序更新
	if s.dynamicSorter != nil {
		s.dynamicSorter.ForceUpdate()
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

// handleTestAllEndpointsStream 批量测试所有端点（流式返回）
func (s *AdminServer) handleTestAllEndpointsStream(c *gin.Context) {
	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	allEndpoints := s.endpointManager.GetAllEndpoints()
	resultsChan := make(chan *BatchTestResult, len(allEndpoints))

	// 启动并发测试
	var wg sync.WaitGroup
	for _, ep := range allEndpoints {
		wg.Add(1)
		go func(endpoint *endpoint.Endpoint) {
			defer wg.Done()

			// 测试单个端点
			result := s.testSingleEndpoint(endpoint)
			resultsChan <- result

			// 记录日志
			s.logger.Info(fmt.Sprintf("📊 Endpoint test completed: %s", endpoint.Name), nil)
		}(ep)
	}

	// 关闭channel的goroutine
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 实时发送测试结果
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming unsupported"})
		return
	}

	for result := range resultsChan {
		// 将结果编码为JSON
		data, err := json.Marshal(result)
		if err != nil {
			continue
		}

		// 发送SSE事件
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	// 发送完成事件
	fmt.Fprintf(c.Writer, "event: done\ndata: {\"message\": \"All tests completed\"}\n\n")
	flusher.Flush()
}
