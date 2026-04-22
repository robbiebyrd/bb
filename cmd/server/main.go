package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	dbcassets "github.com/robbiebyrd/cantou/dbcs"
	"github.com/robbiebyrd/cantou/internal/app"
	"github.com/robbiebyrd/cantou/internal/client/dedupe"
	"github.com/robbiebyrd/cantou/internal/client/logging"
	"github.com/robbiebyrd/cantou/internal/client/signal-dispatch"
	"github.com/robbiebyrd/cantou/internal/config"
	canModels "github.com/robbiebyrd/cantou/internal/models"
	"github.com/robbiebyrd/cantou/internal/output/crtd"
	"github.com/robbiebyrd/cantou/internal/output/csv"
	"github.com/robbiebyrd/cantou/internal/output/influxdb"
	mf4out "github.com/robbiebyrd/cantou/internal/output/mf4"
	"github.com/robbiebyrd/cantou/internal/output/mqtt"
	"github.com/robbiebyrd/cantou/internal/output/prometheus"
	"github.com/robbiebyrd/cantou/internal/parser/dbc"
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
		Use:           "cantou",
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
			var parsers []canModels.ParserInterface
			for _, dbcPath := range iface.DBCFiles {
				p, err := dbc.NewDBCParserClient(logger, dbcPath)
				if err != nil {
					logger.Error("failed to load DBC file", "interface", iface.Name, "path", dbcPath, "error", err)
					os.Exit(1)
				}
				parsers = append(parsers, p)
			}
			if !cfg.DisableOBD2 {
				obd2Parser, err := dbc.NewDBCParserClientFromBytes(logger, "obd2-builtin", dbcassets.OBD2DBC)
				if err != nil {
					logger.Error("failed to load built-in OBD-II DBC", "error", err)
					os.Exit(1)
				}
				parsers = append(parsers, obd2Parser)
			}
			if len(parsers) == 0 {
				continue
			}
			dispatcher := signaldispatch.New(dbc.NewMultiParser(parsers...), cfg.MessageBufferSize, logger)
			b.AddSignalDispatcher(dispatcher, i)
		}

		var outputs []canModels.OutputClient

		if cfg.CRTDLogger.CanOutputFile != "" || cfg.CRTDLogger.SignalOutputFile != "" {
			crtdClient, err := crtd.NewClient(ctx, &cfg, logger)
			if err != nil {
				logger.Error("failed to create CRTD client", "error", err)
				os.Exit(1)
			}
			outputs = append(outputs, crtdClient)
		}

		if cfg.CSVLog.CanOutputFile != "" || cfg.CSVLog.SignalOutputFile != "" {
			csvClient, err := csv.NewClient(ctx, &cfg, logger, connections)
			if err != nil {
				logger.Error("failed to create CSV client", "error", err)
				os.Exit(1)
			}
			outputs = append(outputs, csvClient)
		}

		if cfg.MF4Logger.CanOutputFile != "" || cfg.MF4Logger.SignalOutputFile != "" {
			mf4Client, err := mf4out.NewClient(ctx, &cfg, logger)
			if err != nil {
				logger.Error("failed to create MF4 client", "error", err)
				os.Exit(1)
			}
			outputs = append(outputs, mf4Client)
		}

		if cfg.InfluxDB.Host != "" {
			var influxFilters []canModels.FilterInput
			if cfg.InfluxDB.Dedupe {
				influxFilters = append(influxFilters, canModels.FilterInput{
					Name:   "deduper",
					Filter: dedupe.NewDedupeFilterClient(logger, cfg.InfluxDB.DedupeTimeout, cfg.InfluxDB.DedupeIDs),
				})
			}
			influxClient, err := influxdb.NewClient(ctx, &cfg, logger, connections, influxFilters...)
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

		if cfg.Prometheus.ListenAddr != "" {
			prometheusClient, err := prometheus.NewClient(ctx, &cfg, logger, connections)
			if err != nil {
				logger.Error("failed to create Prometheus client", "error", err)
				os.Exit(1)
			}
			outputs = append(outputs, prometheusClient)
		}

		b.AddOutputs(outputs)
		return b.Run()
	}

	if err := rootCmd.Execute(); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}
