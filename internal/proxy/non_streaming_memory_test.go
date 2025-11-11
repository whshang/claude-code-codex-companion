package proxy

import (
	"bytes"
	"io"
	"runtime"
	"testing"
	"time"
)

type bufferingStats struct {
	duration       time.Duration
	allocDiff      int64
	totalAllocDiff int64
}

func measureReadAllScenario(size int) bufferingStats {
	payload := bytes.Repeat([]byte("A"), size)
	reader := bytes.NewReader(payload)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	var responseBodyBuffer bytes.Buffer
	decompressedCapture := newLimitedBuffer(responseCaptureLimit)
	teeReader := io.TeeReader(reader, decompressedCapture)

	start := time.Now()
	_, _ = responseBodyBuffer.ReadFrom(teeReader)
	duration := time.Since(start)

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	allocDiff := int64(after.Alloc - before.Alloc)
	totalAllocDiff := int64(after.TotalAlloc - before.TotalAlloc)
	return bufferingStats{duration: duration, allocDiff: allocDiff, totalAllocDiff: totalAllocDiff}
}

func measureChunkedScenario(size int) bufferingStats {
	payload := bytes.Repeat([]byte("A"), size)
	reader := bytes.NewReader(payload)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	decompressedCapture := newLimitedBuffer(responseCaptureLimit)
	teeReader := io.TeeReader(reader, decompressedCapture)

	start := time.Now()
	// 模拟分块写回：直接复制到 io.Discard，避免额外缓冲
	buf := make([]byte, 32*1024)
	_, _ = io.CopyBuffer(io.Discard, teeReader, buf)
	duration := time.Since(start)

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	allocDiff := int64(after.Alloc - before.Alloc)
	totalAllocDiff := int64(after.TotalAlloc - before.TotalAlloc)
	return bufferingStats{duration: duration, allocDiff: allocDiff, totalAllocDiff: totalAllocDiff}
}

func TestNonStreamingBufferingMemoryImpact(t *testing.T) {
	sizes := []int{1 << 20, 4 << 20, 8 << 20} // 1MB, 4MB, 8MB

	for _, size := range sizes {
		readAllStats := measureReadAllScenario(size)
		chunkedStats := measureChunkedScenario(size)

		t.Logf("payload=%dKB readAll alloc_delta=%.2fMB readAll_total_alloc=%.2fMB chunked alloc_delta=%.2fMB chunked_total_alloc=%.2fMB readAll_duration=%s chunked_duration=%s", size/1024,
			float64(readAllStats.allocDiff)/1024.0/1024.0,
			float64(readAllStats.totalAllocDiff)/1024.0/1024.0,
			float64(chunkedStats.allocDiff)/1024.0/1024.0,
			float64(chunkedStats.totalAllocDiff)/1024.0/1024.0,
			readAllStats.duration,
			chunkedStats.duration,
		)
	}
}
