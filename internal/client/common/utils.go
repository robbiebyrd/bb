package common

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// FilterSet is a concurrency-safe named collection of FilterInterface values.
// Output clients embed one and guard AddFilter / ShouldFilter with its mutex.
type FilterSet struct {
	mu      sync.RWMutex
	filters map[string]canModels.FilterInterface
}

func NewFilterSet() *FilterSet {
	return &FilterSet{filters: make(map[string]canModels.FilterInterface)}
}

func NewFilterSetFromInputs(filters []canModels.FilterInput) *FilterSet {
	fs := NewFilterSet()
	for _, f := range filters {
		fs.filters[f.Name] = f.Filter
	}
	return fs
}

// Add registers a named filter. Returns an error if the name is already in use.
func (fs *FilterSet) Add(name string, filter canModels.FilterInterface) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if _, ok := fs.filters[name]; ok {
		return fmt.Errorf("filter group already exists: %v", name)
	}
	fs.filters[name] = filter
	return nil
}

// Has reports whether a filter with the given name is registered.
func (fs *FilterSet) Has(name string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	_, ok := fs.filters[name]
	return ok
}

// Len returns the number of registered filters.
func (fs *FilterSet) Len() int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return len(fs.filters)
}

// ShouldFilter returns true (and the matching filter name) if any filter in the
// set rejects msg. The empty string is returned for name when no filter matches.
func (fs *FilterSet) ShouldFilter(msg canModels.CanMessageTimestamped) (bool, string) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	for name, filter := range fs.filters {
		if filter.Filter(msg) {
			return true, name
		}
	}
	return false, ""
}

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

// ShouldFilter returns true and the matching filter name if any filter rejects
// the message. The empty string is returned for name when no filter matches.
//
// Deprecated: prefer FilterSet.ShouldFilter, which is concurrency-safe.
func ShouldFilter(filters map[string]canModels.FilterInterface, canMsg canModels.CanMessageTimestamped) (bool, string) {
	for name, filter := range filters {
		if filter.Filter(canMsg) {
			return true, name
		}
	}
	return false, ""
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
