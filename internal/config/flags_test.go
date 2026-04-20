package config_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/bb/internal/config"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestBindFlags_InfluxHostOverridesEnv(t *testing.T) {
	cfg := canModels.Config{
		InfluxDB: canModels.InfluxDBConfig{Host: "http://env-host:8086"},
	}
	runFlags(t, []string{"--influx-host", "http://cli-host:8086"}, &cfg)
	assert.Equal(t, "http://cli-host:8086", cfg.InfluxDB.Host)
}

func TestBindFlags_EnvPreservedWhenFlagAbsent(t *testing.T) {
	cfg := canModels.Config{
		InfluxDB: canModels.InfluxDBConfig{Host: "http://env-host:8086"},
	}
	runFlags(t, nil, &cfg)
	assert.Equal(t, "http://env-host:8086", cfg.InfluxDB.Host)
}

func TestBindFlags_MQTTHost(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--mqtt-host", "tcp://localhost:1883", "--mqtt-client-id", "test-client"}, &cfg)
	assert.Equal(t, "tcp://localhost:1883", cfg.MQTTConfig.Host)
	assert.Equal(t, "test-client", cfg.MQTTConfig.ClientId)
}

func TestBindFlags_Interface(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--interface", "sim0:sim:can0"}, &cfg)
	require.Len(t, cfg.CanInterfaces, 1)
	assert.Equal(t, "sim0", cfg.CanInterfaces[0].Name)
	assert.Equal(t, "sim", cfg.CanInterfaces[0].Network)
	assert.Equal(t, "can0", cfg.CanInterfaces[0].URI)
}

func TestBindFlags_MultipleInterfaces(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--interface", "sim0:sim:can0", "--interface", "real0:can:vcan0"}, &cfg)
	require.Len(t, cfg.CanInterfaces, 2)
	assert.Equal(t, "sim0", cfg.CanInterfaces[0].Name)
	assert.Equal(t, "real0", cfg.CanInterfaces[1].Name)
}

func TestBindFlags_InterfaceWithDBCAndLoop(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--interface", "pb0:playback:/tmp/log.crtd:/path/to.dbc:true"}, &cfg)
	require.Len(t, cfg.CanInterfaces, 1)
	iface := cfg.CanInterfaces[0]
	assert.Equal(t, "pb0", iface.Name)
	assert.Equal(t, "playback", iface.Network)
	assert.Equal(t, "/tmp/log.crtd", iface.URI)
	assert.Equal(t, []string{"/path/to.dbc"}, iface.DBCFiles)
	assert.True(t, iface.Loop)
}

func TestBindFlags_InterfaceWithMultipleDBCFiles(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--interface", "sim0:sim:can0:/a.dbc,/b.dbc"}, &cfg)
	require.Len(t, cfg.CanInterfaces, 1)
	assert.Equal(t, []string{"/a.dbc", "/b.dbc"}, cfg.CanInterfaces[0].DBCFiles)
}

func TestBindFlags_DisableOBD2(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--disable-obd2"}, &cfg)
	assert.True(t, cfg.DisableOBD2)
}

func TestBindFlags_DedupeIDs(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--mqtt-dedupe-ids", "256,512"}, &cfg)
	assert.Equal(t, []uint32{256, 512}, cfg.MQTTConfig.DedupeIDs)
}

func TestBindFlags_DedupeIDsPreservedWhenAbsent(t *testing.T) {
	cfg := canModels.Config{
		MQTTConfig: canModels.MQTTConfig{DedupeIDs: []uint32{100, 200}},
	}
	runFlags(t, nil, &cfg)
	assert.Equal(t, []uint32{100, 200}, cfg.MQTTConfig.DedupeIDs)
}

func TestBindFlags_InterfaceInvalidFormat(t *testing.T) {
	cfg := canModels.Config{}
	cmd, apply := newTestCmd(&cfg)
	cmd.SetArgs([]string{"--interface", "onlyname"})
	require.NoError(t, cmd.Execute())
	err := apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name:net:uri")
}

// newTestCmd is like runFlags but returns cmd and apply separately for error testing.
func newTestCmd(cfg *canModels.Config) (*cobra.Command, func() error) {
	cmd := &cobra.Command{
		Use:  "test",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	apply := config.BindFlags(cmd, cfg)
	return cmd, apply
}

func TestParseUint32Slice_Empty(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--mqtt-dedupe-ids", ""}, &cfg)
	assert.Nil(t, cfg.MQTTConfig.DedupeIDs)
}

func TestParseUint32Slice_Valid(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--mqtt-dedupe-ids", "1,2,3"}, &cfg)
	assert.Equal(t, []uint32{1, 2, 3}, cfg.MQTTConfig.DedupeIDs)
}

func TestParseUint32Slice_Whitespace(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--mqtt-dedupe-ids", " 10 , 20 "}, &cfg)
	assert.Equal(t, []uint32{10, 20}, cfg.MQTTConfig.DedupeIDs)
}

func TestParseUint32Slice_Invalid(t *testing.T) {
	cfg := canModels.Config{}
	cmd, apply := newTestCmd(&cfg)
	cmd.SetArgs([]string{"--mqtt-dedupe-ids", "abc"})
	require.NoError(t, cmd.Execute())
	err := apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ID")
}

func TestParseUint32Slice_Overflow(t *testing.T) {
	cfg := canModels.Config{}
	cmd, apply := newTestCmd(&cfg)
	// 2^32 exceeds uint32 max
	cmd.SetArgs([]string{"--mqtt-dedupe-ids", "4294967296"})
	require.NoError(t, cmd.Execute())
	err := apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ID")
}

func TestBindFlags_LogCanMessages(t *testing.T) {
	cfg := canModels.Config{LogCanMessages: true}
	runFlags(t, []string{"--log-can-messages=false"}, &cfg)
	assert.False(t, cfg.LogCanMessages)
}

func TestBindFlags_LogSignals(t *testing.T) {
	cfg := canModels.Config{LogSignals: true}
	runFlags(t, []string{"--log-signals=false"}, &cfg)
	assert.False(t, cfg.LogSignals)
}

func TestBindFlags_PrometheusListenAddr(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--prometheus-listen-addr", ":9091"}, &cfg)
	assert.Equal(t, ":9091", cfg.Prometheus.ListenAddr)
}

func TestBindFlags_PrometheusPath(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--prometheus-path", "/custom-metrics"}, &cfg)
	assert.Equal(t, "/custom-metrics", cfg.Prometheus.Path)
}

func TestBindFlags_PrometheusDefaultsPreserved(t *testing.T) {
	cfg := canModels.Config{
		Prometheus: canModels.PrometheusConfig{ListenAddr: "", Path: "/metrics"},
	}
	runFlags(t, nil, &cfg)
	assert.Equal(t, "", cfg.Prometheus.ListenAddr)
	assert.Equal(t, "/metrics", cfg.Prometheus.Path)
}

func TestBindFlags_InfluxDedupe(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--influx-dedupe", "--influx-dedupe-timeout", "500"}, &cfg)
	assert.True(t, cfg.InfluxDB.Dedupe)
	assert.Equal(t, 500, cfg.InfluxDB.DedupeTimeout)
}

func TestBindFlags_InfluxDedupeIDs(t *testing.T) {
	cfg := canModels.Config{}
	runFlags(t, []string{"--influx-dedupe-ids", "256,512"}, &cfg)
	assert.Equal(t, []uint32{256, 512}, cfg.InfluxDB.DedupeIDs)
}

func TestBindFlags_InfluxDedupeIDsPreservedWhenAbsent(t *testing.T) {
	cfg := canModels.Config{
		InfluxDB: canModels.InfluxDBConfig{DedupeIDs: []uint32{100, 200}},
	}
	runFlags(t, nil, &cfg)
	assert.Equal(t, []uint32{100, 200}, cfg.InfluxDB.DedupeIDs)
}
