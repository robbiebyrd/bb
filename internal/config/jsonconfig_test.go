package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func writeTempJSON(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestOverlayConfigFile_NestedOBD(t *testing.T) {
	cfg, err := env.ParseAs[canModels.Config]()
	require.NoError(t, err)

	path := writeTempJSON(t, `{
		"interfaces": [
			{"name":"mx","net":"stn","uri":"rfcomm://AA:BB:CC:DD:EE:FF",
			 "obd":{"mode":"hybrid","pids":["010C","010D"],"hwFilter":["7E8","244"],"pollMs":500}}
		]
	}`)
	require.NoError(t, OverlayConfigFile(&cfg, path))

	require.Len(t, cfg.CanInterfaces, 1)
	i := cfg.CanInterfaces[0]
	assert.Equal(t, "mx", i.Name)
	assert.Equal(t, "stn", i.Network)
	assert.Equal(t, "hybrid", i.OBD.Mode)
	assert.Equal(t, []string{"010C", "010D"}, i.OBD.PIDs)
	assert.Equal(t, []string{"7E8", "244"}, i.OBD.HWFilters)
	assert.Equal(t, 500, i.OBD.PollMS)
}

func TestOverlayConfigFile_PreservesEnvForOmittedKeys(t *testing.T) {
	t.Setenv("MSG_BUFFER_SIZE", "4242")
	cfg, err := env.ParseAs[canModels.Config]()
	require.NoError(t, err)

	// File omits messageBufferSize, so the env value must survive the overlay.
	path := writeTempJSON(t, `{"logLevel":"debug"}`)
	require.NoError(t, OverlayConfigFile(&cfg, path))

	assert.Equal(t, "debug", cfg.LogLevel)       // from file
	assert.Equal(t, 4242, cfg.MessageBufferSize) // from env, untouched
}

func TestValidateInterfaces_RejectsMissingName(t *testing.T) {
	err := ValidateInterfaces([]canModels.CanInterfaceOption{{Network: "stn"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestConfigExampleJSON_Parses(t *testing.T) {
	cfg, err := env.ParseAs[canModels.Config]()
	require.NoError(t, err)
	require.NoError(t, OverlayConfigFile(&cfg, "../../config.example.json"))
	require.NoError(t, ValidateInterfaces(cfg.CanInterfaces))

	require.GreaterOrEqual(t, len(cfg.CanInterfaces), 1)
	assert.Equal(t, "bertha", cfg.CanInterfaces[0].Name)
	assert.Equal(t, "hybrid", cfg.CanInterfaces[0].OBD.Mode)
	assert.Equal(t, "127.0.0.1:9091", cfg.Prometheus.ListenAddr)
}
