package main

import (
	"os"

	"github.com/robbiebyrd/bb/internal/app"
	"github.com/robbiebyrd/bb/internal/client/dedupe"
	canModels "github.com/robbiebyrd/bb/internal/models"
	"github.com/robbiebyrd/bb/internal/output/crtd"
	"github.com/robbiebyrd/bb/internal/output/csv"
	"github.com/robbiebyrd/bb/internal/output/influxdb"
	"github.com/robbiebyrd/bb/internal/output/mqtt"
)

func main() {
	b := app.NewApp()
	ctx, cfg, log := b.GetContext(), b.GetConfig(), b.GetLogger()
	connections := b.GetConnections()

	var outputs []canModels.OutputClient

	if cfg.CRTDLogger.OutputFile != "" {
		outputs = append(outputs, crtd.NewClient(ctx, cfg, log))
	}

	if cfg.CSVLog.OutputFile != "" {
		csvClient, err := csv.NewClient(ctx, cfg, log, connections)
		if err != nil {
			log.Error("failed to create CSV client", "error", err)
			os.Exit(1)
		}
		outputs = append(outputs, csvClient)
	}

	if cfg.InfluxDB.Host != "" {
		influxClient, err := influxdb.NewClient(ctx, cfg, log, connections)
		if err != nil {
			log.Error("failed to create InfluxDB client", "error", err)
			os.Exit(1)
		}
		outputs = append(outputs, influxClient)
	}

	if cfg.MQTTConfig.Host != "" {
		var mqttFilters []canModels.FilterInput
		if cfg.MQTTConfig.Dedupe {
			mqttFilters = append(mqttFilters, canModels.FilterInput{
				Name:   "deduper",
				Filter: dedupe.NewDedupeFilterClient(log, cfg.MQTTConfig.DedupeTimeout, cfg.MQTTConfig.DedupeIDs),
			})
		}
		mqttClient, err := mqtt.NewClient(ctx, cfg, log, connections, mqttFilters...)
		if err != nil {
			log.Error("failed to create MQTT client", "error", err)
			os.Exit(1)
		}
		outputs = append(outputs, mqttClient)
	}

	b.AddOutputs(outputs)

	if err := b.Run(); err != nil {
		log.Error("application error", "error", err)
		os.Exit(1)
	}
}
