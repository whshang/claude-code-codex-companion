package utils

import (
	"sync"
	"time"
)

// CircularBuffer represents a circular buffer for request records
type CircularBuffer struct {
	records   []RequestRecord
	size      int
	head      int
	count     int
	mutex     sync.RWMutex
	windowDur time.Duration
}

// RequestRecord represents a single request record
type RequestRecord struct {
	Timestamp       time.Time
	Success         bool
	FirstByteTime   time.Duration // 首字节返回时间（TTFB - Time To First Byte）
	ResponseTime    time.Duration // 完整响应时间（包含下载时间）

	// 请求ID（用于追踪失败原因）
	RequestID string
}

// NewCircularBuffer creates a new circular buffer with the specified size and time window
func NewCircularBuffer(size int, windowDuration time.Duration) *CircularBuffer {
	return &CircularBuffer{
		records:   make([]RequestRecord, size),
		size:      size,
		head:      0,
		count:     0,
		windowDur: windowDuration,
	}
}

// Add adds a new record to the circular buffer
func (cb *CircularBuffer) Add(record RequestRecord) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.records[cb.head] = record
	cb.head = (cb.head + 1) % cb.size

	if cb.count < cb.size {
		cb.count++
	}
}

// GetWindowStats returns statistics for records within the time window
func (cb *CircularBuffer) GetWindowStats(now time.Time) (total, failed int) {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	cutoff := now.Add(-cb.windowDur)

	for i := 0; i < cb.count; i++ {
		idx := (cb.head - 1 - i + cb.size) % cb.size
		record := cb.records[idx]

		if record.Timestamp.Before(cutoff) {
			break // Records are sorted by time, so we can break early
		}

		total++
		if !record.Success {
			failed++
		}
	}

	return total, failed
}

// ShouldMarkInactive determines if the endpoint should be marked as inactive
// based on the failure pattern in the time window
func (cb *CircularBuffer) ShouldMarkInactive(now time.Time) bool {
	total, failed := cb.GetWindowStats(now)
	
	// Mark inactive if: more than 1 request in window AND all requests failed
	return total > 1 && failed == total
}

// Clear clears all records from the buffer
func (cb *CircularBuffer) Clear() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.count = 0
	cb.head = 0
}

// GetRecentFailureRequestIDs 获取时间窗口内的所有失败请求ID
func (cb *CircularBuffer) GetRecentFailureRequestIDs(now time.Time) []string {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	cutoff := now.Add(-cb.windowDur)
	var failureRequestIDs []string

	for i := 0; i < cb.count; i++ {
		idx := (cb.head - 1 - i + cb.size) % cb.size
		record := cb.records[idx]

		if record.Timestamp.Before(cutoff) {
			break
		}

		if !record.Success && record.RequestID != "" {
			failureRequestIDs = append(failureRequestIDs, record.RequestID)
		}
	}

	return failureRequestIDs
}

// GetLastResponseTime 获取最近一次成功请求的首字节时间（TTFB）
// 用于动态排序，避免因消息体大小影响性能判断
// 返回最近成功请求的首字节时间，如果没有成功请求则返回0
func (cb *CircularBuffer) GetLastResponseTime() time.Duration {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	// 从最新的记录开始查找
	for i := 0; i < cb.count; i++ {
		idx := (cb.head - 1 - i + cb.size) % cb.size
		record := cb.records[idx]

		// 优先使用首字节时间（更准确的性能指标）
		if record.Success && record.FirstByteTime > 0 {
			return record.FirstByteTime
		}
		// 向后兼容：如果没有首字节时间，使用完整响应时间
		if record.Success && record.ResponseTime > 0 {
			return record.ResponseTime
		}
	}

	return 0 // 没有找到成功请求
}