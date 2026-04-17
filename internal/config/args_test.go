package config_test

import (
	"os"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/bb/internal/config"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

// runFlags creates a minimal cobra command, binds flags, parses args, and calls apply.
func runFlags(t *testing.T, args []string, cfg *canModels.Config) {
	t.Helper()
	cmd := &cobra.Command{
		Use:  "test",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	apply := config.BindFlags(cmd, cfg)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	require.NoError(t, apply())
}

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

func TestInfluxDBConfig_TLS_DefaultFalse(t *testing.T) {
	os.Unsetenv("INFLUX_TLS")
	os.Unsetenv("INFLUX_TLS_CA_FILE")

	cfg, err := env.ParseAs[canModels.InfluxDBConfig]()
	require.NoError(t, err)
	assert.False(t, cfg.TLS)
	assert.Equal(t, "", cfg.TLSCACertFile)
}
