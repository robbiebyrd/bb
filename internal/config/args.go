package config

import (
	"encoding/json"
	"fmt"

	"github.com/caarlos0/env/v11"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func Load() (canModels.Config, string) {
	cfg, err := env.ParseAs[canModels.Config]()
	if err != nil {
		panic(err)
	} else if len(cfg.CanInterfaces) == 0 {
		panic("no CAN interfaces configured")
	}
	cfgJson, err := ToJSON(cfg)
	if err != nil || cfgJson == nil {
		panic(err)
	}

	return cfg, *cfgJson
}

func ToJSON(config canModels.Config) (*string, error) {
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}

	jsonString := string(jsonBytes)
	return &jsonString, nil
}
