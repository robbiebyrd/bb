package obdconn

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/cantou/internal/models"
	"github.com/robbiebyrd/cantou/internal/obd"
)

func testConfig() *canModels.Config {
	return &canModels.Config{CanInterfaceSeparator: "-"}
}

func testChannel() chan canModels.CanMessageTimestamped {
	return make(chan canModels.CanMessageTimestamped, 100)
}

// nopDriver is a stand-in Driver used to exercise option handling without
// touching hardware.
type nopDriver struct{ extended bool }

func (n nopDriver) Init() error                         { return nil }
func (n nopDriver) SetFilters([]uint32) error           { return nil }
func (n nopDriver) Request(string) ([]obd.Frame, error) { return nil, nil }
func (n nopDriver) Extended() bool                      { return n.extended }
func (n nopDriver) Close() error                        { return nil }
func (n nopDriver) Monitor(func(obd.Frame), func(error), <-chan struct{}) error {
	return nil
}

func nopFactory(obd.Transport, obd.Protocol, bool, obd.Logf) Driver { return nopDriver{} }

func newBase(opt canModels.CanInterfaceOption, kind string) *Base {
	logger := slog.New(slog.DiscardHandler)
	return New(context.Background(), testConfig(), testChannel(), logger, opt, kind, nopFactory)
}

func TestNew_DefaultsMonitorMode(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{Name: "mx", Network: canModels.NetworkSTN}, canModels.NetworkSTN)
	assert.Equal(t, ModeMonitor, b.Mode())
}

func TestNew_EmptyNamePanics(t *testing.T) {
	assert.Panics(t, func() {
		newBase(canModels.CanInterfaceOption{Network: canModels.NetworkSTN}, canModels.NetworkSTN)
	})
}

func TestNew_DefaultsNetworkToKind(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{Name: "dev"}, canModels.NetworkELM327)
	assert.Equal(t, canModels.NetworkELM327, b.GetNetwork())
}

func TestNew_DefaultsURIToName(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{Name: "dev"}, canModels.NetworkELM327)
	assert.Equal(t, "dev", b.GetURI())
}

func TestNew_ELM327HybridFallsBackToPollWithPIDs(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name: "elm",
		OBD:  canModels.OBDOptions{Mode: ModeHybrid, PIDs: []string{"010C"}},
	}, canModels.NetworkELM327)
	assert.Equal(t, ModePoll, b.Mode())
}

func TestNew_ELM327HybridFallsBackToMonitorWithoutPIDs(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name: "elm",
		OBD:  canModels.OBDOptions{Mode: ModeHybrid},
	}, canModels.NetworkELM327)
	assert.Equal(t, ModeMonitor, b.Mode())
}

func TestNew_STNKeepsHybrid(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name: "mx",
		OBD:  canModels.OBDOptions{Mode: ModeHybrid, PIDs: []string{"010C"}},
	}, canModels.NetworkSTN)
	assert.Equal(t, ModeHybrid, b.Mode())
}

func TestOpen_PollWithoutPIDsErrors(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name:    "mx",
		Network: canModels.NetworkSTN,
		URI:     "/dev/does-not-exist-cantou-test",
		OBD:     canModels.OBDOptions{Mode: ModePoll},
	}, canModels.NetworkSTN)
	err := b.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PID")
}

func TestOpen_InvalidPIDErrors(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name:    "mx",
		Network: canModels.NetworkSTN,
		OBD:     canModels.OBDOptions{Mode: ModePoll, PIDs: []string{"01ZZ"}},
	}, canModels.NetworkSTN)
	err := b.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PID")
}

func TestBase_GettersAndSetters(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{Name: "mx", Network: canModels.NetworkSTN}, canModels.NetworkSTN)

	b.SetID(7)
	assert.Equal(t, 7, b.GetID())

	b.SetName("renamed")
	assert.Equal(t, "renamed", b.GetName())

	b.SetURI("/dev/rfcomm0")
	assert.Equal(t, "/dev/rfcomm0", b.GetURI())

	b.SetNetwork("stn")
	assert.Equal(t, "stn", b.GetNetwork())

	p := "/x.dbc"
	b.SetDBCFilePath(&p)
	assert.Equal(t, &p, b.GetDBCFilePath())

	assert.Nil(t, b.GetConnection())
	b.SetConnection(nil)
	assert.False(t, b.IsOpen())
}

func TestBase_GetInterfaceName(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name:    "mx",
		Network: canModels.NetworkSTN,
		URI:     "/dev/rfcomm0",
	}, canModels.NetworkSTN)
	assert.Equal(t, "mx-stn-/dev/rfcomm0", b.GetInterfaceName())
}

func TestBase_ImplementsCanConnection(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{Name: "mx", Network: canModels.NetworkSTN}, canModels.NetworkSTN)
	var _ canModels.CanConnection = b
}

func TestNew_AutoAddsResponseIDsWhenPollingFiltered(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name:    "mx",
		Network: canModels.NetworkSTN,
		OBD: canModels.OBDOptions{
			Mode:      ModePoll,
			PIDs:      []string{"010C"},
			HWFilters: []string{"1C4"},
		},
	}, canModels.NetworkSTN)
	assert.Contains(t, b.filters, uint32(0x1C4)) // user filter preserved
	assert.Contains(t, b.filters, uint32(0x7E8)) // response IDs auto-added
	assert.Contains(t, b.filters, uint32(0x7EF))
}

func TestNew_NoResponseIDsWithoutHardwareFilter(t *testing.T) {
	// An empty filter already passes everything, so nothing is added.
	b := newBase(canModels.CanInterfaceOption{
		Name:    "mx",
		Network: canModels.NetworkSTN,
		OBD:     canModels.OBDOptions{Mode: ModePoll, PIDs: []string{"010C"}},
	}, canModels.NetworkSTN)
	assert.Empty(t, b.filters)
}

func TestNew_NoResponseIDsInMonitorMode(t *testing.T) {
	b := newBase(canModels.CanInterfaceOption{
		Name:    "mx",
		Network: canModels.NetworkSTN,
		OBD:     canModels.OBDOptions{Mode: ModeMonitor, HWFilters: []string{"1C4"}},
	}, canModels.NetworkSTN)
	assert.Equal(t, []uint32{0x1C4}, b.filters) // untouched
}

func TestNew_ResponseFilterOptOut(t *testing.T) {
	off := false
	b := newBase(canModels.CanInterfaceOption{
		Name:    "mx",
		Network: canModels.NetworkSTN,
		OBD: canModels.OBDOptions{
			Mode:           ModePoll,
			PIDs:           []string{"010C"},
			HWFilters:      []string{"1C4"},
			ResponseFilter: &off,
		},
	}, canModels.NetworkSTN)
	assert.Equal(t, []uint32{0x1C4}, b.filters) // opt-out: not added
}
