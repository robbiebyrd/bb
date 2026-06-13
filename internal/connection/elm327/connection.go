// Package elm327 provides a CanConnection for generic ELM327 OBD/CAN adapters.
// It is a thin Driver over internal/obd's AT command set wired into the shared
// obdconn machinery. ELM327 supports passive monitor and active PID poll modes;
// hybrid mode is downgraded by obdconn since the AT command set cannot interleave
// monitoring and polling on a single adapter.
package elm327

import (
	"context"
	"log/slog"

	"github.com/robbiebyrd/cantou/internal/connection/obdconn"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	"github.com/robbiebyrd/cantou/internal/obd"
)

// driver adapts *obd.ELM327 to obdconn.Driver, mapping the generic filter and
// monitor calls onto the AT command set.
type driver struct {
	*obd.ELM327
}

func (d driver) SetFilters(ids []uint32) error { return d.SetRxFilter(ids) }

func (d driver) Monitor(onFrame func(obd.Frame), onError func(error), stop <-chan struct{}) error {
	return d.MonitorAll(onFrame, onError, stop)
}

// Factory builds an ELM327 driver over a transport.
func Factory(t obd.Transport, proto obd.Protocol, debug bool, logf obd.Logf) obdconn.Driver {
	return driver{obd.NewELM327(t, proto, debug, logf)}
}

// NewConnection builds an ELM327 CanConnection from an interface option.
func NewConnection(
	ctx context.Context,
	cfg *canModels.Config,
	channel chan canModels.CanMessageTimestamped,
	logger *slog.Logger,
	opt canModels.CanInterfaceOption,
) *obdconn.Base {
	return obdconn.New(ctx, cfg, channel, logger, opt, canModels.NetworkELM327, Factory)
}
