package config_test

import (
	"encoding/json"
	"log/slog"
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

func TestToJSON_StripsMQTTPassword(t *testing.T) {
	cfg := canModels.Config{
		MQTTConfig: canModels.MQTTConfig{Password: "super-secret"},
	}
	jsonStr, err := config.ToJSON(cfg)
	require.NoError(t, err)
	assert.NotContains(t, *jsonStr, "super-secret", "ToJSON must not expose MQTT password in output")
}

func TestToJSON_StripsInfluxToken(t *testing.T) {
	cfg := canModels.Config{
		InfluxDB: canModels.InfluxDBConfig{Token: "influx-token"},
	}
	jsonStr, err := config.ToJSON(cfg)
	require.NoError(t, err)
	assert.NotContains(t, *jsonStr, "influx-token", "ToJSON must not expose InfluxDB token in output")
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

func TestSetLogLevel(t *testing.T) {
	cases := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			lvl := &slog.LevelVar{}
			lvl.Set(slog.LevelInfo)
			config.SetLogLevel(lvl, tc.input)
			assert.Equal(t, tc.expected, lvl.Level())
		})
	}
}

func TestSetLogLevel_UnknownLeavesLevelUnchanged(t *testing.T) {
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	config.SetLogLevel(lvl, "unknown")
	assert.Equal(t, slog.LevelInfo, lvl.Level())
}

func TestLoadInfluxToken_AlreadySet(t *testing.T) {
	cfg := &canModels.Config{
		InfluxDB: canModels.InfluxDBConfig{Token: "existing-token"},
	}
	logger := slog.Default()
	err := config.LoadInfluxToken(cfg, logger)
	require.NoError(t, err)
	assert.Equal(t, "existing-token", cfg.InfluxDB.Token)
}

func TestLoadInfluxToken_FromFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "influx-token-*.json")
	require.NoError(t, err)
	require.NoError(t, json.NewEncoder(f).Encode(map[string]string{"token": "file-token"}))
	f.Close()

	cfg := &canModels.Config{
		InfluxDB: canModels.InfluxDBConfig{TokenFile: f.Name()},
	}
	logger := slog.Default()
	err = config.LoadInfluxToken(cfg, logger)
	require.NoError(t, err)
	assert.Equal(t, "file-token", cfg.InfluxDB.Token)
}

func TestLoadInfluxToken_FileMissing(t *testing.T) {
	cfg := &canModels.Config{
		InfluxDB: canModels.InfluxDBConfig{TokenFile: "/nonexistent/path/token.json"},
	}
	logger := slog.Default()
	err := config.LoadInfluxToken(cfg, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opening influxdb token file")
}
