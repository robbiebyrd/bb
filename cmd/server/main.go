package main

import (
	"github.com/robbiebyrd/bb/internal/app"
	"github.com/robbiebyrd/bb/internal/client/dedupe"
	canModels "github.com/robbiebyrd/bb/internal/models"
	"github.com/robbiebyrd/bb/internal/output/csv"
	"github.com/robbiebyrd/bb/internal/output/influxdb"
	"github.com/robbiebyrd/bb/internal/output/mqtt"
)

func main() {
	b := app.NewApp()
	ctx, cfg, log := b.GetContext(), b.GetConfig(), b.GetLogger()

	b.AddOutputs([]canModels.OutputClient{
		csv.NewClient(ctx, cfg, log),
		influxdb.NewClient(ctx, cfg, log),
		mqtt.NewClient(ctx, cfg, log, canModels.FilterInput{
			Name:   "deduper",
			Filter: dedupe.NewDedupeFilterClient(log, cfg.MQTTConfig.DedupeTimeout, cfg.MQTTConfig.DedupeIDs),
		}),
	})

	b.Run()
}
