package common

import (
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// StartThroughputReporter starts a background goroutine that logs channel
// throughput at the given interval until done is closed. counter is
// incremented by the caller on each message; bufferLen returns the current
// depth of the input channel.
func StartThroughputReporter(
	done <-chan struct{},
	l *slog.Logger,
	name, channel string,
	counter *atomic.Uint64,
	bufferLen func() int,
	interval time.Duration,
) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var lastCount uint64
		for {
			select {
			case <-ticker.C:
				current := counter.Load()
				secs := uint64(interval.Seconds())
				if secs == 0 {
					secs = 1
				}
				rate := (current - lastCount) / secs
				lastCount = current
				l.Info("client throughput",
					"client", name,
					"channel", channel,
					"msgs_per_sec", rate,
					"buffer_queue", bufferLen(),
				)
			case <-done:
				return
			}
		}
	}()
}

// ShouldFilter returns true and the matching filter name if any filter rejects the message.
func ShouldFilter(filters map[string]canModels.FilterInterface, canMsg canModels.CanMessageTimestamped) (bool, *string) {
	for name, filter := range filters {
		if filter.Filter(canMsg) {
			return true, &name
		}
	}
	return false, nil
}

func PadOrTrim(bb []byte, size int) []byte {
	l := len(bb)
	if l >= size {
		return bb[:size]
	}
	tmp := make([]byte, size)
	copy(tmp, bb)
	return tmp
}

func ArrayAllTrue(arr []bool) bool {
	return !ArrayContainsBool(arr, false)
}

func ArrayContainsFalse(arr []bool) bool {
	return ArrayContainsBool(arr, false)
}

func ArrayContainsTrue(arr []bool) bool {
	return ArrayContainsBool(arr, true)
}

func ArrayContainsBool(arr []bool, value bool) bool {
	return slices.Contains(arr, value)
}
