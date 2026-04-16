package config

import (
	"encoding/json"
	"log/slog"

	"github.com/caarlos0/env/v11"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func Load(logger *slog.Logger) (canModels.Config, string) {
	cfg, err := env.ParseAs[canModels.Config]()
	if err != nil {
		panic(err)
	} else if len(cfg.CanInterfaces) == 0 {
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
	if err != nil || cfgJson == nil {
		panic(err)
	}

	return cfg, *cfgJson
}

func ToJSON(config canModels.Config) (*string, error) {
	// Zero the token before marshalling to prevent it appearing in logs.
	config.InfluxDB.Token = ""
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	jsonString := string(jsonBytes)
	return &jsonString, nil
}
