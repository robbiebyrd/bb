package main

import (
	"os"

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

	csvClient, err := csv.NewClient(ctx, cfg, log)
	if err != nil {
		log.Error("failed to create CSV client", "error", err)
		os.Exit(1)
	}

	influxClient, err := influxdb.NewClient(ctx, cfg, log)
	if err != nil {
		log.Error("failed to create InfluxDB client", "error", err)
		os.Exit(1)
	}

	var mqttFilters []canModels.FilterInput
	if cfg.MQTTConfig.Dedupe {
		mqttFilters = append(mqttFilters, canModels.FilterInput{
			Name:   "deduper",
			Filter: dedupe.NewDedupeFilterClient(log, cfg.MQTTConfig.DedupeTimeout, cfg.MQTTConfig.DedupeIDs),
		})
	}
	mqttClient, err := mqtt.NewClient(ctx, cfg, log, mqttFilters...)
	if err != nil {
		log.Error("failed to create MQTT client", "error", err)
		os.Exit(1)
	}

	b.AddOutputs([]canModels.OutputClient{csvClient, influxClient, mqttClient})

	if err := b.Run(); err != nil {
		log.Error("application error", "error", err)
		os.Exit(1)
	}
}
