package conversion_test

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	conversion "claude-code-codex-companion/internal/conversion"
)

type recordingWriter struct {
	mu         sync.Mutex
	firstWrite time.Time
	totalBytes int
	writeCount int
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	now := time.Now()
	w.mu.Lock()
	if w.firstWrite.IsZero() {
		w.firstWrite = now
	}
	w.totalBytes += len(p)
	w.writeCount++
	w.mu.Unlock()
	return len(p), nil
}

func (w *recordingWriter) FirstLatency(start time.Time) time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.firstWrite.IsZero() {
		return 0
	}
	return w.firstWrite.Sub(start)
}

func (w *recordingWriter) Stats() (bytes int, writes int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.totalBytes, w.writeCount
}

type streamChunk struct {
	delay  time.Duration
	format string
}

func runChatCompletionScenario(chunks []streamChunk) (firstLatency time.Duration, totalDuration time.Duration, bytes int, writes int, err error) {
	reader, writer := io.Pipe()

	go func() {
		start := time.Now()
		for idx, chunk := range chunks {
			if chunk.delay > 0 {
				target := start.Add(chunk.delay)
				time.Sleep(time.Until(target))
			}
			fmt.Fprintf(writer, "data: %s\n\n", chunk.format)
			if idx == len(chunks)-1 {
				// leave space for [DONE] emission by caller if present
			}
		}
		// emulate upstream completion signal
		fmt.Fprint(writer, "data: [DONE]\n\n")
		writer.Close()
	}()

	rec := &recordingWriter{}
	begin := time.Now()
	err = conversion.StreamChatCompletionsToResponses(reader, rec)
	totalDuration = time.Since(begin)
	firstLatency = rec.FirstLatency(begin)
	bytes, writes = rec.Stats()
	return
}

func averageDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	var total time.Duration
	for _, v := range values {
		total += v
	}
	return total / time.Duration(len(values))
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if percentile <= 0 {
		return sorted[0]
	}
	if percentile >= 100 {
		return sorted[len(sorted)-1]
	}
	rank := percentile / 100 * float64(len(sorted)-1)
	lower := sorted[int(rank)]
	upper := sorted[int(rank)+1]
	fraction := rank - float64(int(rank))
	return lower + time.Duration(float64(upper-lower)*fraction)
}

func TestStreamChatCompletionsBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("streaming benchmark skipped in short mode")
	}

	chunks := []streamChunk{
		{
			delay:  40 * time.Millisecond,
			format: `{"id":"chatcmpl-benchmark","created":1710000000,"model":"gpt-5","choices":[{"delta":{"content":"package main"}}]}`,
		},
		{
			delay:  70 * time.Millisecond,
			format: `{"id":"chatcmpl-benchmark","created":1710000000,"model":"gpt-5","choices":[{"delta":{"content":"\n\nfunc main() {"}}]}`,
		},
		{
			delay:  85 * time.Millisecond,
			format: `{"id":"chatcmpl-benchmark","created":1710000000,"model":"gpt-5","choices":[{"delta":{"content":"\n    println(\"hello\")"}}]}`,
		},
		{
			delay:  95 * time.Millisecond,
			format: `{"id":"chatcmpl-benchmark","created":1710000000,"model":"gpt-5","choices":[{"delta":{"content":"\n}"}}]}`,
		},
		{
			delay:  110 * time.Millisecond,
			format: `{"id":"chatcmpl-benchmark","created":1710000000,"model":"gpt-5","choices":[{"delta":{},"finish_reason":"stop"}]}`,
		},
	}

	iterations := 6
	latencies := make([]time.Duration, 0, iterations)
	totals := make([]time.Duration, 0, iterations)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < iterations; i++ {
		first, total, bytes, writes, err := runChatCompletionScenario(chunks)
		if err != nil {
			t.Fatalf("iteration %d failed: %v", i+1, err)
		}
		latencies = append(latencies, first)
		totals = append(totals, total)
		t.Logf("iteration %d: first_byte=%s total=%s bytes=%d writes=%d", i+1, first, total, bytes, writes)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	t.Logf("first_byte_avg=%s first_byte_p95=%s total_avg=%s", averageDuration(latencies), percentileDuration(latencies, 95), averageDuration(totals))
	t.Logf("memory alloc_delta=%dKB total_alloc_delta=%dKB num_gc=%d", int64(after.Alloc-before.Alloc)/1024, int64(after.TotalAlloc-before.TotalAlloc)/1024, int64(after.NumGC-before.NumGC))
}
