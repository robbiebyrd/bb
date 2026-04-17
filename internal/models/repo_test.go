package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/robbiebyrd/bb/internal/models"
)

// staticSignalOutputClient is a compile-time assertion that SignalOutputClient
// embeds OutputClient and declares all expected signal methods.
type staticSignalOutputClient struct{}

func (s *staticSignalOutputClient) Run() error                                     { return nil }
func (s *staticSignalOutputClient) HandleCanMessage(_ models.CanMessageTimestamped) {}
func (s *staticSignalOutputClient) HandleCanMessageChannel() error                 { return nil }
func (s *staticSignalOutputClient) GetChannel() chan models.CanMessageTimestamped  { return nil }
func (s *staticSignalOutputClient) GetName() string                                { return "" }
func (s *staticSignalOutputClient) AddFilter(_ string, _ models.FilterInterface) error {
	return nil
}
func (s *staticSignalOutputClient) HandleSignal(_ models.CanSignalTimestamped)        {}
func (s *staticSignalOutputClient) HandleSignalChannel() error                        { return nil }
func (s *staticSignalOutputClient) GetSignalChannel() chan models.CanSignalTimestamped { return nil }

var _ models.SignalOutputClient = (*staticSignalOutputClient)(nil)

func TestSignalOutputClient_EmbeddedInOutputClient(t *testing.T) {
	var soc models.SignalOutputClient = &staticSignalOutputClient{}
	var oc models.OutputClient = soc
	assert.NotNil(t, oc)
}
