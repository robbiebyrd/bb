package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	canModels "github.com/robbiebyrd/cantou/internal/models"
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
	f.StringVar(&cfg.InfluxDB.MessageDatabase, "influx-message-database", cfg.InfluxDB.MessageDatabase, "InfluxDB database for CAN messages")
	f.StringVar(&cfg.InfluxDB.SignalDatabase, "influx-signal-database", cfg.InfluxDB.SignalDatabase, "InfluxDB database for decoded signals (empty = disabled)")
	f.StringVar(&cfg.InfluxDB.SignalTableName, "influx-signal-table", cfg.InfluxDB.SignalTableName, "InfluxDB table name for decoded signals")
	f.StringVar(&cfg.InfluxDB.TableName, "influx-table", cfg.InfluxDB.TableName, "InfluxDB table name")
	f.IntVar(&cfg.InfluxDB.FlushTime, "influx-flush-time", cfg.InfluxDB.FlushTime, "InfluxDB flush interval in milliseconds")
	f.IntVar(&cfg.InfluxDB.MaxWriteLines, "influx-max-write-lines", cfg.InfluxDB.MaxWriteLines, "InfluxDB max lines per write batch")
	f.IntVar(&cfg.InfluxDB.MaxConnections, "influx-max-connections", cfg.InfluxDB.MaxConnections, "InfluxDB max concurrent connections")
	f.BoolVar(&cfg.InfluxDB.Dedupe, "influx-dedupe", cfg.InfluxDB.Dedupe, "Enable InfluxDB message deduplication")
	f.IntVar(&cfg.InfluxDB.DedupeTimeout, "influx-dedupe-timeout", cfg.InfluxDB.DedupeTimeout, "InfluxDB dedupe window in milliseconds")

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
	f.StringVar(&cfg.CSVLog.CanOutputFile, "csv-can-output-file", cfg.CSVLog.CanOutputFile, "CSV output file path for CAN messages")
	f.StringVar(&cfg.CSVLog.SignalOutputFile, "csv-signal-output-file", cfg.CSVLog.SignalOutputFile, "CSV output file path for decoded signals (empty = disabled)")
	f.BoolVar(&cfg.CSVLog.IncludeHeaders, "csv-headers", cfg.CSVLog.IncludeHeaders, "Include headers in CSV output")

	// CRTD
	f.StringVar(&cfg.CRTDLogger.CanOutputFile, "crtd-can-output-file", cfg.CRTDLogger.CanOutputFile, "CRTD log output file path for CAN messages")
	f.StringVar(&cfg.CRTDLogger.SignalOutputFile, "crtd-signal-output-file", cfg.CRTDLogger.SignalOutputFile, "CRTD log output file path for decoded signals (empty = disabled)")

	// MF4 (ASAM MDF 4.x)
	f.StringVar(&cfg.MF4Logger.CanOutputFile, "mf4-can-output-file", cfg.MF4Logger.CanOutputFile, "MF4 output file path for CAN messages (empty = disabled)")
	f.StringVar(&cfg.MF4Logger.SignalOutputFile, "mf4-signal-output-file", cfg.MF4Logger.SignalOutputFile, "MF4 output file path for decoded signals (empty = disabled)")
	f.BoolVar(&cfg.MF4Logger.Finalize, "mf4-finalize", cfg.MF4Logger.Finalize, "Rewrite the MF4 file's DT length and ID magic on graceful shutdown so it is recognised as finalized (false = always leave unfinalized)")

	// Prometheus
	f.StringVar(&cfg.Prometheus.ListenAddr, "prometheus-listen-addr", cfg.Prometheus.ListenAddr, "Prometheus metrics listener address (empty = disabled, e.g. 127.0.0.1:9091)")
	f.StringVar(&cfg.Prometheus.Path, "prometheus-path", cfg.Prometheus.Path, "Prometheus metrics HTTP path")

	// Global
	f.IntVar(&cfg.MessageBufferSize, "msg-buffer-size", cfg.MessageBufferSize, "Incoming message channel buffer size")
	f.IntVar(&cfg.SimEmitRate, "sim-rate", cfg.SimEmitRate, "Sim interface emit rate in nanoseconds between messages")
	f.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	f.StringVar(&cfg.CanInterfaceSeparator, "interface-separator", cfg.CanInterfaceSeparator, "Separator for interface name components in env vars")
	f.BoolVar(&cfg.LogCanMessages, "log-can-messages", cfg.LogCanMessages, "Enable logging of CAN messages to output clients")
	f.BoolVar(&cfg.LogSignals, "log-signals", cfg.LogSignals, "Enable logging of decoded signals to output clients")
	f.BoolVar(&cfg.DisableOBD2, "disable-obd2", cfg.DisableOBD2, "Disable auto-injection of the built-in OBD-II DBC for all interfaces")

	// Complex types: stored in local vars, applied by the returned function.
	dedupeIDsStr := formatUint32Slice(cfg.MQTTConfig.DedupeIDs)
	f.StringVar(&dedupeIDsStr, "mqtt-dedupe-ids", dedupeIDsStr, "Comma-separated list of CAN IDs to deduplicate (empty = all)")

	influxDedupeIDsStr := formatUint32Slice(cfg.InfluxDB.DedupeIDs)
	f.StringVar(&influxDedupeIDsStr, "influx-dedupe-ids", influxDedupeIDsStr, "Comma-separated list of CAN IDs to deduplicate for InfluxDB (empty = all)")

	var ifaceStrs []string
	f.StringArrayVar(&ifaceStrs, "interface", formatInterfaces(cfg.CanInterfaces), "CAN interface in name:net:uri[:dbcfiles[:loop]] format; dbcfiles is comma-separated (repeatable)")

	var signalFilterStrs []string
	f.StringArrayVar(&signalFilterStrs, "signal-filter", nil, "Signal filter rule in name:field:op:value format (repeatable); field: signal|message, op: eq|neq|contains|notcontains|startswith|notstartswith|endswith|notendswith")

	var signalFilterOpStrs []string
	f.StringArrayVar(&signalFilterOpStrs, "signal-filter-op", nil, "Signal filter operator in name:and|or format (default and)")

	var signalFilterModeStrs []string
	f.StringArrayVar(&signalFilterModeStrs, "signal-filter-mode", nil, "Signal filter mode in name:exclude|include format (default exclude)")

	return func() error {
		if cmd.Flags().Changed("mqtt-dedupe-ids") {
			ids, err := parseUint32Slice(dedupeIDsStr)
			if err != nil {
				return fmt.Errorf("parsing --mqtt-dedupe-ids: %w", err)
			}
			cfg.MQTTConfig.DedupeIDs = ids
		}
		if cmd.Flags().Changed("influx-dedupe-ids") {
			ids, err := parseUint32Slice(influxDedupeIDsStr)
			if err != nil {
				return fmt.Errorf("parsing --influx-dedupe-ids: %w", err)
			}
			cfg.InfluxDB.DedupeIDs = ids
		}
		if cmd.Flags().Changed("interface") {
			ifaces, err := parseInterfaces(ifaceStrs)
			if err != nil {
				return fmt.Errorf("parsing --interface: %w", err)
			}
			cfg.CanInterfaces = ifaces
		}
		if cmd.Flags().Changed("signal-filter") {
			if err := applySignalFilters(cfg, signalFilterStrs); err != nil {
				return fmt.Errorf("parsing --signal-filter: %w", err)
			}
		}
		if cmd.Flags().Changed("signal-filter-op") {
			if err := applySignalFilterOps(cfg, signalFilterOpStrs); err != nil {
				return fmt.Errorf("parsing --signal-filter-op: %w", err)
			}
		}
		if cmd.Flags().Changed("signal-filter-mode") {
			if err := applySignalFilterModes(cfg, signalFilterModeStrs); err != nil {
				return fmt.Errorf("parsing --signal-filter-mode: %w", err)
			}
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
		if len(iface.DBCFiles) > 0 || iface.Loop {
			result[i] += ":" + strings.Join(iface.DBCFiles, ",")
		}
		if iface.Loop {
			result[i] += ":true"
		}
	}
	return result
}

// ifaceByName returns a pointer to the CanInterfaceOption with the given name, or nil.
func ifaceByName(cfg *canModels.Config, name string) *canModels.CanInterfaceOption {
	for i := range cfg.CanInterfaces {
		if cfg.CanInterfaces[i].Name == name {
			return &cfg.CanInterfaces[i]
		}
	}
	return nil
}

// applySignalFilters parses "name:field:op:value" strings and appends rules to the named interface.
func applySignalFilters(cfg *canModels.Config, strs []string) error {
	for _, s := range strs {
		idx := strings.Index(s, ":")
		if idx < 0 {
			return fmt.Errorf("signal filter %q must be in name:field:op:value format", s)
		}
		name, rule := s[:idx], s[idx+1:]
		iface := ifaceByName(cfg, name)
		if iface == nil {
			return fmt.Errorf("signal filter references unknown interface %q", name)
		}
		iface.SignalFilters = append(iface.SignalFilters, rule)
	}
	return nil
}

// applySignalFilterOps parses "name:and|or" strings and sets the operator on the named interface.
func applySignalFilterOps(cfg *canModels.Config, strs []string) error {
	for _, s := range strs {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("signal filter op %q must be in name:and|or format", s)
		}
		iface := ifaceByName(cfg, parts[0])
		if iface == nil {
			return fmt.Errorf("--signal-filter-op references unknown interface %q", parts[0])
		}
		iface.SignalFilterOp = parts[1]
	}
	return nil
}

// applySignalFilterModes parses "name:exclude|include" strings and sets the mode on the named interface.
func applySignalFilterModes(cfg *canModels.Config, strs []string) error {
	for _, s := range strs {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("signal filter mode %q must be in name:exclude|include format", s)
		}
		iface := ifaceByName(cfg, parts[0])
		if iface == nil {
			return fmt.Errorf("--signal-filter-mode references unknown interface %q", parts[0])
		}
		iface.SignalFilterMode = parts[1]
	}
	return nil
}

// parseInterfaces parses "name:net:uri[:dbcfiles[:loop]]" strings into CanInterfaceOptions.
// dbcfiles is a comma-separated list of DBC file paths.
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
		if len(parts) >= 4 && parts[3] != "" {
			opt.DBCFiles = strings.Split(parts[3], ",")
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
