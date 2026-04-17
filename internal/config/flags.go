package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// BindFlags registers all config fields as cobra flags on cmd, using the current
// cfg values (e.g. loaded from env) as defaults. CLI flags override env vars.
//
// The returned apply function must be called at the start of RunE to parse
// complex flag types (interfaces, dedupe IDs) into cfg.
func BindFlags(cmd *cobra.Command, cfg *canModels.Config) func() error {
	f := cmd.Flags()

	// InfluxDB
	f.StringVar(&cfg.InfluxDB.Host, "influx-host", cfg.InfluxDB.Host, "InfluxDB host URL")
	f.StringVar(&cfg.InfluxDB.Token, "influx-token", cfg.InfluxDB.Token, "InfluxDB auth token")
	f.StringVar(&cfg.InfluxDB.TokenFile, "influx-token-file", cfg.InfluxDB.TokenFile, "Path to InfluxDB token JSON file")
	f.StringVar(&cfg.InfluxDB.Database, "influx-database", cfg.InfluxDB.Database, "InfluxDB database name")
	f.StringVar(&cfg.InfluxDB.TableName, "influx-table", cfg.InfluxDB.TableName, "InfluxDB table name")
	f.IntVar(&cfg.InfluxDB.FlushTime, "influx-flush-time", cfg.InfluxDB.FlushTime, "InfluxDB flush interval in milliseconds")
	f.IntVar(&cfg.InfluxDB.MaxWriteLines, "influx-max-write-lines", cfg.InfluxDB.MaxWriteLines, "InfluxDB max lines per write batch")
	f.IntVar(&cfg.InfluxDB.MaxConnections, "influx-max-connections", cfg.InfluxDB.MaxConnections, "InfluxDB max concurrent connections")

	// MQTT
	f.StringVar(&cfg.MQTTConfig.Host, "mqtt-host", cfg.MQTTConfig.Host, "MQTT broker address (e.g. tcp://localhost:1883)")
	f.StringVar(&cfg.MQTTConfig.ClientId, "mqtt-client-id", cfg.MQTTConfig.ClientId, "MQTT client ID")
	f.StringVar(&cfg.MQTTConfig.Topic, "mqtt-topic", cfg.MQTTConfig.Topic, "MQTT base topic")
	f.Uint8Var(&cfg.MQTTConfig.Qos, "mqtt-qos", cfg.MQTTConfig.Qos, "MQTT QoS level (0, 1, or 2)")
	f.BoolVar(&cfg.MQTTConfig.ShadowCopy, "mqtt-shadow-copy", cfg.MQTTConfig.ShadowCopy, "Retain messages on MQTT broker")
	f.BoolVar(&cfg.MQTTConfig.Dedupe, "mqtt-dedupe", cfg.MQTTConfig.Dedupe, "Enable MQTT message deduplication")
	f.IntVar(&cfg.MQTTConfig.DedupeTimeout, "mqtt-dedupe-timeout", cfg.MQTTConfig.DedupeTimeout, "MQTT dedupe window in milliseconds")
	f.StringVar(&cfg.MQTTConfig.Username, "mqtt-username", cfg.MQTTConfig.Username, "MQTT username")
	f.StringVar(&cfg.MQTTConfig.Password, "mqtt-password", cfg.MQTTConfig.Password, "MQTT password")
	f.BoolVar(&cfg.MQTTConfig.TLS, "mqtt-tls", cfg.MQTTConfig.TLS, "Enable TLS for MQTT connection")

	// CSV
	f.StringVar(&cfg.CSVLog.OutputFile, "csv-output-file", cfg.CSVLog.OutputFile, "CSV output file path")
	f.BoolVar(&cfg.CSVLog.IncludeHeaders, "csv-headers", cfg.CSVLog.IncludeHeaders, "Include headers in CSV output")

	// CRTD
	f.StringVar(&cfg.CRTDLogger.OutputFile, "crtd-output-file", cfg.CRTDLogger.OutputFile, "CRTD log output file path")

	// Global
	f.IntVar(&cfg.MessageBufferSize, "msg-buffer-size", cfg.MessageBufferSize, "Incoming message channel buffer size")
	f.IntVar(&cfg.SimEmitRate, "sim-rate", cfg.SimEmitRate, "Sim interface emit rate in nanoseconds between messages")
	f.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	f.StringVar(&cfg.CanInterfaceSeparator, "interface-separator", cfg.CanInterfaceSeparator, "Separator for interface name components in env vars")

	// Complex types: stored in local vars, applied by the returned function.
	dedupeIDsStr := formatUint32Slice(cfg.MQTTConfig.DedupeIDs)
	f.StringVar(&dedupeIDsStr, "mqtt-dedupe-ids", dedupeIDsStr, "Comma-separated list of CAN IDs to deduplicate (empty = all)")

	var ifaceStrs []string
	f.StringArrayVar(&ifaceStrs, "interface", formatInterfaces(cfg.CanInterfaces), "CAN interface in name:net:uri[:dbcfile[:loop]] format (repeatable)")

	return func() error {
		if cmd.Flags().Changed("mqtt-dedupe-ids") {
			ids, err := parseUint32Slice(dedupeIDsStr)
			if err != nil {
				return fmt.Errorf("parsing --mqtt-dedupe-ids: %w", err)
			}
			cfg.MQTTConfig.DedupeIDs = ids
		}
		if cmd.Flags().Changed("interface") {
			ifaces, err := parseInterfaces(ifaceStrs)
			if err != nil {
				return fmt.Errorf("parsing --interface: %w", err)
			}
			cfg.CanInterfaces = ifaces
		}
		return nil
	}
}

func formatUint32Slice(ids []uint32) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ",")
}

func parseUint32Slice(s string) ([]uint32, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	ids := make([]uint32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid ID %q: %w", p, err)
		}
		ids = append(ids, uint32(v))
	}
	return ids, nil
}

func formatInterfaces(ifaces []canModels.CanInterfaceOption) []string {
	result := make([]string, len(ifaces))
	for i, iface := range ifaces {
		result[i] = iface.Name + ":" + iface.Network + ":" + iface.URI
		if iface.DBCFile != "" || iface.Loop {
			result[i] += ":" + iface.DBCFile
		}
		if iface.Loop {
			result[i] += ":true"
		}
	}
	return result
}

// parseInterfaces parses "name:net:uri[:dbcfile[:loop]]" strings into CanInterfaceOptions.
func parseInterfaces(strs []string) ([]canModels.CanInterfaceOption, error) {
	ifaces := make([]canModels.CanInterfaceOption, 0, len(strs))
	for _, s := range strs {
		parts := strings.SplitN(s, ":", 5)
		if len(parts) < 3 {
			return nil, fmt.Errorf("interface %q must be in name:net:uri format", s)
		}
		opt := canModels.CanInterfaceOption{
			Name:    parts[0],
			Network: parts[1],
			URI:     parts[2],
		}
		if len(parts) >= 4 {
			opt.DBCFile = parts[3]
		}
		if len(parts) >= 5 {
			loop, err := strconv.ParseBool(parts[4])
			if err != nil {
				return nil, fmt.Errorf("interface %q loop value %q: %w", s, parts[4], err)
			}
			opt.Loop = loop
		}
		ifaces = append(ifaces, opt)
	}
	return ifaces, nil
}
