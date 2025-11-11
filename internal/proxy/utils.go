package proxy

import (
	"encoding/json"
	"io"
	"strings"

	"claude-code-codex-companion/internal/endpoint"
	"github.com/gin-gonic/gin"
)

// utils.go: 代理工具类模块
// 提供 proxy 包内部使用的、不适合放在其他模块的通用工具函数。
//
// 目标：
// - 包含如 addConversionStage, setConversionContext 等上下文管理函数。
// - 包含 min, ensureOpenAIStreamTrue 等小型辅助函数。
// - 避免成为一个“大杂烩”文件，只存放真正通用的、与代理核心流程相关的工具。

// teeCaptureWriter is a writer that captures the first N bytes written to it
// while also passing the writes to an underlying writer.
type teeCaptureWriter struct {
	underlying io.Writer
	capture    *limitedBuffer
	limit      int
}

func newTeeCaptureWriter(underlying io.Writer, limit int) *teeCaptureWriter {
	return &teeCaptureWriter{
		underlying: underlying,
		capture:    newLimitedBuffer(limit),
		limit:      limit,
	}
}

func (t *teeCaptureWriter) Write(p []byte) (n int, err error) {
	t.capture.Write(p)
	return t.underlying.Write(p)
}

func (t *teeCaptureWriter) Captured() []byte {
	return t.capture.Bytes()
}

// limitedBuffer is a buffer that only stores the first N bytes written to it.
type limitedBuffer struct {
	buf   []byte
	limit int
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{
		buf:   make([]byte, 0, limit),
		limit: limit,
	}
}

func (b *limitedBuffer) Write(p []byte) (n int, err error) {
	remaining := b.limit - len(b.buf)
	if remaining > 0 {
		toWrite := len(p)
		if toWrite > remaining {
			toWrite = remaining
		}
		b.buf = append(b.buf, p[:toWrite]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf
}

func addConversionStage(stages *[]string, stage string) {
	if stages == nil {
		return
	}
	*stages = append(*stages, stage)
}

func setConversionContext(c *gin.Context, stages []string) {
	if c == nil || len(stages) == 0 {
		return
	}
	c.Set("conversion_path", strings.Join(stages, conversionStageSeparator))
}

func getSupportsResponsesFlag(ep *endpoint.Endpoint) string {
	if ep == nil || ep.SupportsResponses == nil {
		return "U" // Unknown
	}
	if *ep.SupportsResponses {
		return "Y" // Yes
	}
	return "N" // No
}

func updateSupportsResponsesContext(c *gin.Context, ep *endpoint.Endpoint) {
	if c == nil || ep == nil {
		return
	}
	c.Set("supports_responses_flag", getSupportsResponsesFlag(ep))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ensureOpenAIStreamTrue ensures the 'stream' field is true in an OpenAI request body.
func ensureOpenAIStreamTrue(body []byte) ([]byte, bool) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body, false
	}

	stream, ok := data["stream"].(bool)
	if ok && stream {
		return body, false
	}

	data["stream"] = true
	newBody, err := json.Marshal(data)
	if err != nil {
		return body, false
	}
	return newBody, true
}