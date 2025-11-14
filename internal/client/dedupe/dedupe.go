package dedupe

import (
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/mitchellh/hashstructure/v2"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type DedupeFilterClient struct {
	storage map[uint64]time.Time
	l       *slog.Logger
	timeout int
	ids     []uint32
}

func NewDedupeFilterClient(l *slog.Logger, timeout int, ids []uint32) canModels.FilterInterface {
	return &DedupeFilterClient{make(map[uint64]time.Time), l, timeout, ids}
}

func (dc *DedupeFilterClient) Add(_ canModels.CanMessageFilter) error {
	return nil
}

func (dc *DedupeFilterClient) Mode(_ canModels.CanFilterGroupOperator) {}

func (dc *DedupeFilterClient) Filter(canMsg canModels.CanMessageTimestamped) bool {
	if !slices.Contains(dc.ids, canMsg.ID) {
		return false
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

func marshalFrom(source *canModels.CanMessageTimestamped, destination *canModels.CanMessageData) error {
	bytes, err := json.Marshal(source)
	json.Unmarshal(bytes, destination)
	return err
}

func stripTimestampFromMessage(canMsg canModels.CanMessageTimestamped) (*canModels.CanMessageData, error) {
	var small = &canModels.CanMessageData{}
	err := marshalFrom(&canMsg, small)
	if err != nil {
		return nil, err
	}

	return small, nil
}

func hashCanMessageData(canMsg canModels.CanMessageTimestamped) (uint64, error) {
	updatedMsg, err := stripTimestampFromMessage(canMsg)
	if err != nil {
		return 0, err
	}

	hashed, err := hashstructure.Hash(updatedMsg, hashstructure.FormatV2, nil)
	if err != nil {
		return 0, err
	}

	return hashed, nil
}
