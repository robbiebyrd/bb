package dedupe

import (
	"log/slog"
	"time"

	hc "github.com/mitchellh/hashstructure/v2"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type DedupeFilterClient struct {
	storage map[uint64]time.Time
	l       *slog.Logger
	timeout int
	ids     map[uint32]struct{}
}

func NewDedupeFilterClient(l *slog.Logger, timeout int, ids []uint32) canModels.FilterInterface {
	idMap := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		idMap[id] = struct{}{}
	}
	return &DedupeFilterClient{
		storage: make(map[uint64]time.Time),
		l:       l,
		timeout: timeout,
		ids:     idMap,
	}
}

func (dc *DedupeFilterClient) Add(_ canModels.CanMessageFilter) error {
	return nil
}

func (dc *DedupeFilterClient) Mode(_ canModels.CanFilterGroupOperator) {}

func (dc *DedupeFilterClient) Filter(canMsg canModels.CanMessageTimestamped) bool {
	if len(dc.ids) > 0 {
		if _, ok := dc.ids[canMsg.ID]; !ok {
			return false
		}
	}

	msgHash, err := hashCanMessageData(canMsg)
	if err != nil {
		dc.l.Error("error hashing message for shadow copy", "error", err)
		return false
	}

	value, ok := dc.storage[msgHash]

	if !ok {
		dc.l.Debug("no previous message with hash found", "msgHash", msgHash)
		dc.storage[msgHash] = time.Now()
		// Sweep expired entries on new additions
		for hash, t := range dc.storage {
			if time.Since(t) >= time.Duration(dc.timeout)*time.Millisecond {
				delete(dc.storage, hash)
			}
		}
		return false
	}

	if time.Since(value) >= time.Duration(dc.timeout)*time.Millisecond {
		dc.l.Debug("updating message hash", "msgHash", msgHash)
		dc.storage[msgHash] = time.Now()
		return false
	}

	dc.l.Debug("skipping duplicate message with hash", "msgHash", msgHash)
	return true
}

func stripTimestampFromMessage(canMsg canModels.CanMessageTimestamped) *canModels.CanMessageData {
	return &canModels.CanMessageData{
		Interface: canMsg.Interface,
		ID:        canMsg.ID,
		Transmit:  canMsg.Transmit,
		Remote:    canMsg.Remote,
		Length:    canMsg.Length,
		Data:      canMsg.Data,
	}
}

func hashCanMessageData(canMsg canModels.CanMessageTimestamped) (uint64, error) {
	updatedMsg := stripTimestampFromMessage(canMsg)

	hashed, err := hc.Hash(updatedMsg, hc.FormatV2, nil)
	if err != nil {
		return 0, err
	}

	return hashed, nil
}
