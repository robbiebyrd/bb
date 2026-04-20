package common

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logCapture is a minimal slog.Handler that records every log entry.
type logCapture struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *logCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *logCapture) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}
func (h *logCapture) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *logCapture) WithGroup(_ string) slog.Handler      { return h }

func (h *logCapture) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.recs)
}

func TestStartThroughputReporter_DoesNotBlock(t *testing.T) {
	var counter atomic.Uint64
	done := make(chan struct{})
	defer close(done)
	l := slog.New(&logCapture{})

	start := time.Now()
	StartThroughputReporter(done, l, "test", "can", &counter, func() int { return 0 }, time.Second)
	assert.Less(t, time.Since(start), 50*time.Millisecond, "StartThroughputReporter must return immediately")
}

func TestStartThroughputReporter_LogsOnTick(t *testing.T) {
	var counter atomic.Uint64
	counter.Add(100)
	capture := &logCapture{}
	done := make(chan struct{})
	defer close(done)

	StartThroughputReporter(done, slog.New(capture), "output-test", "can", &counter, func() int { return 7 }, 20*time.Millisecond)

	require.Eventually(t, func() bool { return capture.count() > 0 }, 200*time.Millisecond, 5*time.Millisecond)

	capture.mu.Lock()
	rec := capture.recs[0]
	capture.mu.Unlock()

	assert.Equal(t, "client throughput", rec.Message)
	attrs := map[string]any{}
	rec.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	assert.Equal(t, "output-test", attrs["client"])
	assert.Equal(t, "can", attrs["channel"])
	assert.Contains(t, attrs, "msgs_per_sec")
	assert.Contains(t, attrs, "buffer_queue")
}

func TestStartThroughputReporter_ExitsWhenDoneClosed(t *testing.T) {
	capture := &logCapture{}
	done := make(chan struct{})

	var counter atomic.Uint64
	StartThroughputReporter(done, slog.New(capture), "output-test", "signal", &counter, func() int { return 0 }, 20*time.Millisecond)

	require.Eventually(t, func() bool { return capture.count() > 0 }, 200*time.Millisecond, 5*time.Millisecond)

	close(done)
	time.Sleep(10 * time.Millisecond) // let goroutine exit

	countAtClose := capture.count()
	time.Sleep(60 * time.Millisecond) // 3 more potential ticks — none should fire
	assert.Equal(t, countAtClose, capture.count(), "no more logs should appear after done is closed")
}

func TestPadOrTrim(t *testing.T) {
	// pad: input shorter than half of size
	assert.Equal(t, []byte{1, 2, 0, 0}, PadOrTrim([]byte{1, 2}, 4), "Should pad with zeros on the right.")

	// pad: input longer than half of size (bug case)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 0, 0, 0}, PadOrTrim([]byte{1, 2, 3, 4, 5}, 8), "Should pad with zeros on the right.")

	// trim: input longer than size
	assert.Equal(t, []byte{1, 2, 3}, PadOrTrim([]byte{1, 2, 3, 4, 5}, 3), "Should trim to size.")

	// exact: input equals size
	assert.Equal(t, []byte{1, 2, 3}, PadOrTrim([]byte{1, 2, 3}, 3), "Should return unchanged when exact fit.")
}

func TestArrayContainsTrue(t *testing.T) {
	allTrue := ArrayContainsTrue([]bool{true, true, true})
	assert.Equal(t, true, allTrue, "Should be true.")

	allFalse := ArrayContainsTrue([]bool{false, false, false})
	assert.Equal(t, false, allFalse, "Should be false.")

	oneTrue := ArrayContainsTrue([]bool{true, false, false})
	assert.Equal(t, true, oneTrue, "Should be true.")

	empty := ArrayContainsTrue([]bool{})
	assert.Equal(t, false, empty, "OR with no filters should be false.")
}

func TestArrayContainsFalse(t *testing.T) {
	oneFalse := ArrayContainsFalse([]bool{true, true, false})
	assert.Equal(t, true, oneFalse, "Should be true.")

	allFalseAndTrue := ArrayContainsFalse([]bool{false, false, false})
	assert.Equal(t, true, allFalseAndTrue, "Should be true.")

	allTrueAndFalse := ArrayContainsFalse([]bool{true, true, true})
	assert.Equal(t, false, allTrueAndFalse, "Should be true.")
}

func TestArrayAllTrue(t *testing.T) {
	allTrue := ArrayAllTrue([]bool{true, true, true})
	assert.Equal(t, true, allTrue, "Should be true.")

	oneFalse := ArrayAllTrue([]bool{true, true, false})
	assert.Equal(t, false, oneFalse, "Should be true.")

	empty := ArrayAllTrue([]bool{})
	assert.Equal(t, true, empty, "AND with no filters should be true (vacuous truth).")
}

