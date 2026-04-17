package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/robbiebyrd/bb/internal/app"
	"github.com/robbiebyrd/bb/internal/client/dedupe"
	"github.com/robbiebyrd/bb/internal/client/logging"
	"github.com/robbiebyrd/bb/internal/client/signaldispatch"
	"github.com/robbiebyrd/bb/internal/config"
	canModels "github.com/robbiebyrd/bb/internal/models"
	"github.com/robbiebyrd/bb/internal/output/crtd"
	"github.com/robbiebyrd/bb/internal/output/csv"
	"github.com/robbiebyrd/bb/internal/output/influxdb"
	"github.com/robbiebyrd/bb/internal/output/mqtt"
	"github.com/robbiebyrd/bb/internal/parser/dbc"
)

func main() {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	logger := logging.NewJSONLogger(lvl)

	logger.Info("starting application")

	logger.Debug("loading config")
	cfg, cfgJSON := config.Load(logger)
	logger.Debug("loaded config", "config", cfgJSON)

	logger.Info("setting log level from config", "level", cfg.LogLevel)
	config.SetLogLevel(lvl, cfg.LogLevel)

	rootCmd := &cobra.Command{
		Use:           "bb",
		Short:         "CAN bus logger for Bertha",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	apply := config.BindFlags(rootCmd, &cfg)

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := apply(); err != nil {
			return err
		}

		// Re-apply log level in case --log-level was provided via CLI.
		config.SetLogLevel(lvl, cfg.LogLevel)

		if err := config.LoadInfluxToken(&cfg, logger); err != nil {
			logger.Error("failed to load influxdb token", "error", err)
			os.Exit(1)
		}

		b := app.NewApp(&cfg, logger, lvl)
		ctx, connections := b.GetContext(), b.GetConnections()

		for i, iface := range cfg.CanInterfaces {
			if iface.DBCFile == "" {
				continue
			}
			parser, err := dbc.NewDBCParserClient(logger, iface.DBCFile)
			if err != nil {
				logger.Error("failed to load DBC file", "interface", iface.Name, "path", iface.DBCFile, "error", err)
				os.Exit(1)
			}
			dispatcher := signaldispatch.New(parser, cfg.MessageBufferSize, logger)
			b.AddSignalDispatcher(dispatcher, i)
		}

		var outputs []canModels.OutputClient

		if cfg.CRTDLogger.OutputFile != "" {
			outputs = append(outputs, crtd.NewClient(ctx, &cfg, logger))
		}

		if cfg.CSVLog.OutputFile != "" {
			csvClient, err := csv.NewClient(ctx, &cfg, logger, connections)
			if err != nil {
				logger.Error("failed to create CSV client", "error", err)
				os.Exit(1)
			}
			outputs = append(outputs, csvClient)
		}

		if cfg.InfluxDB.Host != "" {
			influxClient, err := influxdb.NewClient(ctx, &cfg, logger, connections)
			if err != nil {
				logger.Error("failed to create InfluxDB client", "error", err)
				os.Exit(1)
			}
			outputs = append(outputs, influxClient)
		}

		if cfg.MQTTConfig.Host != "" {
			var mqttFilters []canModels.FilterInput
			if cfg.MQTTConfig.Dedupe {
				mqttFilters = append(mqttFilters, canModels.FilterInput{
					Name:   "deduper",
					Filter: dedupe.NewDedupeFilterClient(logger, cfg.MQTTConfig.DedupeTimeout, cfg.MQTTConfig.DedupeIDs),
				})
			}
			mqttClient, err := mqtt.NewClient(ctx, &cfg, logger, connections, mqttFilters...)
			if err != nil {
				logger.Error("failed to create MQTT client", "error", err)
				os.Exit(1)
			}
			outputs = append(outputs, mqttClient)
		}

		b.AddOutputs(outputs)
		return b.Run()
	}

	if err := rootCmd.Execute(); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}
