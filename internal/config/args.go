package config

import (
	"github.com/caarlos0/env/v11"

	canModels "github.com/robbiebyrd/bb/internal/models/can"
)

func Load() canModels.Config {
	cfg, err := env.ParseAs[canModels.Config]()
	if err != nil {
		panic(err)
	}
	return cfg
}
