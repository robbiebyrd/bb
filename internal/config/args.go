package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/caarlos0/env/v11"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func Load(logger *slog.Logger) (canModels.Config, string) {
	cfg, err := env.ParseAs[canModels.Config]()
	if err != nil {
		panic(err)
	}

	if len(cfg.CanInterfaces) == 0 {
		logger.Warn("no can interfaces defined in env, defaulting to single simulation interface")
		cfg.CanInterfaces = []canModels.CanInterfaceOption{
			{
				Name:    "simulation",
				URI:     "can0",
				Network: "sim",
			},
		}
	}
	cfgJson, err := ToJSON(cfg)
	if err != nil {
		panic(err)
	}
	if cfgJson == nil {
		panic("config.ToJSON returned nil without error")
	}

	return cfg, *cfgJson
}

// SetLogLevel updates lvl based on the string log level from config.
func SetLogLevel(lvl *slog.LevelVar, level string) {
	switch level {
	case "debug", "DEBUG":
		lvl.Set(slog.LevelDebug)
	case "error", "ERROR":
		lvl.Set(slog.LevelError)
	case "warn", "WARN":
		lvl.Set(slog.LevelWarn)
	}
}

// LoadInfluxToken reads the InfluxDB token from TokenFile when Token is empty.
func LoadInfluxToken(cfg *canModels.Config, logger *slog.Logger) error {
	if cfg.InfluxDB.Token != "" || cfg.InfluxDB.TokenFile == "" {
		return nil
	}
	jsonFile, err := os.Open(cfg.InfluxDB.TokenFile)
	if err != nil {
		return fmt.Errorf("opening influxdb token file %s: %w", cfg.InfluxDB.TokenFile, err)
	}
	defer jsonFile.Close()

	var creds struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(jsonFile).Decode(&creds); err != nil {
		return fmt.Errorf("decoding influxdb token file: %w", err)
	}
	cfg.InfluxDB.Token = creds.Token
	return nil
}

func ToJSON(config canModels.Config) (*string, error) {
	// Zero secrets before marshalling to prevent them appearing in logs.
	config.InfluxDB.Token = ""
	config.MQTTConfig.Password = ""
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	jsonString := string(jsonBytes)
	return &jsonString, nil
}
