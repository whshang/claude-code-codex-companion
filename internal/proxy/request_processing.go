package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/tagging"
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
func (s *Server) processRequestTags(req *http.Request) *tagging.TaggedRequest {
	taggedRequest, err := s.taggingManager.ProcessRequest(req)
	if err != nil {
		s.logger.Error("Failed to process request tags", err)
		return nil
	}
	
	if taggedRequest != nil {
		// 记录详细的tagging结果
		s.logger.Debug(fmt.Sprintf("Tagging completed: found %d tags: %v", len(taggedRequest.Tags), taggedRequest.Tags))
		for _, result := range taggedRequest.TaggerResults {
			if result.Error != nil {
				s.logger.Debug(fmt.Sprintf("Tagger %s failed: %v", result.TaggerName, result.Error))
			} else {
				s.logger.Debug(fmt.Sprintf("Tagger %s: matched=%t, tag=%s, duration=%v", 
					result.TaggerName, result.Matched, result.Tag, result.Duration))
			}
		}
	}
	
	return taggedRequest
}

// selectEndpointForRequest selects the appropriate endpoint based on tags and request format
func (s *Server) selectEndpointForRequest(taggedRequest *tagging.TaggedRequest, requestFormat string, clientType string) (*endpoint.Endpoint, error) {
	if taggedRequest != nil && len(taggedRequest.Tags) > 0 {
		// 使用tag和格式匹配选择endpoint
		selectedEndpoint, err := s.endpointManager.GetEndpointWithTagsFormatAndClient(taggedRequest.Tags, requestFormat, clientType)
		s.logger.Debug(fmt.Sprintf("Request tagged with: %v, format: %s, client: %s, selected endpoint: %s",
			taggedRequest.Tags,
			requestFormat,
			clientType,
			func() string { if selectedEndpoint != nil { return selectedEndpoint.Name } else { return "none" } }()))
		return selectedEndpoint, err
	} else {
		// 使用格式和客户端匹配选择endpoint
		selectedEndpoint, err := s.endpointManager.GetEndpointWithFormatAndClient(requestFormat, clientType)
		s.logger.Debug(fmt.Sprintf("Request has no tags, format: %s, client: %s, using format-based endpoint selection", requestFormat, clientType))
		return selectedEndpoint, err
	}
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