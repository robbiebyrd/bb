// Package stn provides a CanConnection for STN11xx-class OBD/CAN adapters such as
// the OBDLink MX+ (STN2120). It is a thin Driver over internal/obd's ST command
// set wired into the shared obdconn machinery. In addition to passive monitor and
// active PID poll modes, STN supports hybrid mode — interleaving monitoring with
// periodic polling — which the AT-only ELM327 cannot.
package stn

import (
	"context"
	"log/slog"

	"github.com/robbiebyrd/cantou/internal/connection/obdconn"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	"github.com/robbiebyrd/cantou/internal/obd"
)

// driver adapts *obd.STN to obdconn.Driver, mapping the generic filter and
// monitor calls onto the ST command set (hardware pass filters + STM/STMA).
type driver struct {
	*obd.STN
}

func (d driver) SetFilters(ids []uint32) error { return d.SetPassFilters(ids) }

func (d driver) Monitor(onFrame func(obd.Frame), onError func(error), stop <-chan struct{}) error {
	return d.STN.Monitor(onFrame, onError, stop)
}

// Factory builds an STN driver over a transport.
func Factory(t obd.Transport, proto obd.Protocol, debug bool, logf obd.Logf) obdconn.Driver {
	return driver{obd.NewSTN(t, proto, debug, logf)}
}

// NewConnection builds an STN CanConnection from an interface option.
func NewConnection(
	ctx context.Context,
	cfg *canModels.Config,
	channel chan canModels.CanMessageTimestamped,
	logger *slog.Logger,
	opt canModels.CanInterfaceOption,
) *obdconn.Base {
	return obdconn.New(ctx, cfg, channel, logger, opt, canModels.NetworkSTN, Factory)
}
