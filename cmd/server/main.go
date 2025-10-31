package main

import (
	"github.com/robbiebyrd/bb/internal/app"
	canModels "github.com/robbiebyrd/bb/internal/models"
	"github.com/robbiebyrd/bb/internal/output/csv"
	"github.com/robbiebyrd/bb/internal/output/influxdb"
)

func main() {
	b := app.NewBerthaApp()

	b.AddOutputs([]canModels.OutputClient{
		csv.NewClient(&b.Ctx, b.Cfg, b.Log),
		influxdb.NewClient(&b.Ctx, b.Cfg, b.Log),
	})

	b.Run()
}
