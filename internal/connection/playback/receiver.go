package playback

import (
	"sync"
	"time"

	commonUtils "github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// Receive starts a goroutine that parses the log file and emits frames onto
// the channel with their original inter-message timing.
//
// When loop is false the goroutine exits after the last frame.
// When loop is true it replays from the start indefinitely until the context
// is cancelled or Discontinue() is called.
func (c *PlaybackCanClient) Receive(wg *sync.WaitGroup) {
	c.streaming = true

	parser, err := DetectParser(c.uri, c.l)
	if err != nil {
		c.l.Error("playback: unsupported log format",
			"path", c.uri, "error", err)
		return
	}

	wg.Go(func() {
		for c.streaming {
			entries, err := parser.Parse(c.uri)
			if err != nil {
				c.l.Error("playback: failed to parse log file",
					"path", c.uri, "error", err)
				return
			}

			if len(entries) == 0 {
				c.l.Warn("playback: no frames in log file", "path", c.uri)
				return
			}

			c.l.Info("playback: starting",
				"path", c.uri, "frames", len(entries), "loop", c.loop)

			start := time.Now()

			for _, entry := range entries {
				if !c.streaming {
					return
				}

				if delay := time.Until(start.Add(time.Duration(entry.OffsetNs))); delay > 0 {
					select {
					case <-time.After(delay):
					case <-c.ctx.Done():
						return
					}
				}

				select {
				case c.channel <- canModels.CanMessageTimestamped{
					Timestamp: time.Now().UnixNano(),
					Interface: c.id,
					ID:        entry.ID,
					Transmit:  entry.Transmit,
					Remote:    entry.Remote,
					Length:    entry.Length,
					Data:      commonUtils.PadOrTrim(entry.Data, 8),
				}:
				case <-c.ctx.Done():
					return
				}
			}

			c.l.Info("playback: finished", "path", c.uri, "loop", c.loop)

			if !c.loop {
				return
			}
		}
	})
}
