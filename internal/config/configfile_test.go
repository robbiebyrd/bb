package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envValue string
		want     string
	}{
		{"flag space form", []string{"--config", "a.json"}, "", "a.json"},
		{"flag equals form", []string{"--config=b.json"}, "", "b.json"},
		{"short space form", []string{"-c", "c.json"}, "", "c.json"},
		{"short equals form", []string{"-c=d.json"}, "", "d.json"},
		{"flag overrides env", []string{"--config", "flag.json"}, "env.json", "flag.json"},
		{"env fallback", []string{"--log-level", "debug"}, "env.json", "env.json"},
		{"none", []string{}, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveConfigPath(tc.args, tc.envValue))
		})
	}
}

func TestConfigSchema_IsValidJSON(t *testing.T) {
	data, err := os.ReadFile("../../config.schema.json")
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	assert.Equal(t, "object", schema["type"])
	_, hasDefs := schema["$defs"]
	assert.True(t, hasDefs, "schema should define $defs")
}
