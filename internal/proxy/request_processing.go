package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/utils"

	"github.com/gin-gonic/gin"
)

// readRequestBody reads and buffers the request body
func (s *Server) readRequestBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", err)
		return nil, err
	}
	
	// 重新设置请求体供后续使用
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// processRequestTags handles request tagging with error handling
func (s *Server) processRequestTags(req *http.Request) {
	// Tagging system has been removed - this function is a placeholder
	// Requests are now processed without tagging
	s.logger.Debug("Request processing without tagging system")
}

// selectEndpointForRequest selects the appropriate endpoint based on request format and client type
func (s *Server) selectEndpointForRequest(requestFormat string, clientType string) (*endpoint.Endpoint, error) {
	// 使用格式和客户端类型匹配选择endpoint
	selectedEndpoint, err := s.endpointManager.GetEndpointWithFormatAndClient(requestFormat, clientType)
	s.logger.Debug(fmt.Sprintf("Request format: %s, client: %s, selected endpoint: %s",
		requestFormat, clientType,
		func() string { if selectedEndpoint != nil { return selectedEndpoint.Name } else { return "none" } }()))
	return selectedEndpoint, err
}

// extractModelFromRequest extracts the model name from the request body
func (s *Server) extractModelFromRequest(requestBody []byte) string {
	if len(requestBody) == 0 {
		return ""
	}
	return utils.ExtractModelFromRequestBody(string(requestBody))
}

// rebuildRequestBody rebuilds the request body from the cached bytes
func (s *Server) rebuildRequestBody(c *gin.Context, requestBody []byte) {
	if c.Request.Body != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
}

// isRequestExpectingStream 检查请求是否期望流式响应
func (s *Server) isRequestExpectingStream(req *http.Request) bool {
	if req == nil {
		return false
	}
	accept := req.Header.Get("Accept")
	return accept == "text/event-stream" || strings.Contains(accept, "text/event-stream")
}