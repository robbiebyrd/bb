package config_test

import (
	"os"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestMQTTConfig_OptionalWhenHostAbsent(t *testing.T) {
	os.Unsetenv("MQTT_HOST")
	os.Unsetenv("MQTT_CLIENT_ID")

	cfg, err := env.ParseAs[canModels.MQTTConfig]()
	require.NoError(t, err, "config parsing must not error when MQTT vars are absent")
	assert.Equal(t, "", cfg.Host)
	assert.Equal(t, "", cfg.ClientId)
}

func TestInfluxDBConfig_OptionalWhenHostAbsent(t *testing.T) {
	os.Unsetenv("INFLUX_HOST")

	cfg, err := env.ParseAs[canModels.InfluxDBConfig]()
	require.NoError(t, err, "config parsing must not error when INFLUX_HOST is absent")
	assert.Equal(t, "", cfg.Host)
}
