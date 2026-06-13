package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// ValidateSimRate returns an error when exactly one of SimEmitRateMin / SimEmitRateMax
// is set. Both must be provided together; setting only one is a configuration mistake.
func ValidateSimRate(cfg *canModels.Config) error {
	minSet := cfg.SimEmitRateMin != 0
	maxSet := cfg.SimEmitRateMax != 0
	if minSet != maxSet {
		return fmt.Errorf("SIM_RATE_MIN and SIM_RATE_MAX must both be set or both be unset")
	}
	return nil
}

// ValidateInterfaces ensures every configured interface has a name. The env
// loader enforces this via the `required` tag, but a JSON overlay bypasses that,
// so re-check after overlaying.
func ValidateInterfaces(ifaces []canModels.CanInterfaceOption) error {
	for i, iface := range ifaces {
		if iface.Name == "" {
			return fmt.Errorf("interface %d has no name (every interface needs a name)", i)
		}
	}
	return nil
}

// OverlayConfigFile overlays a JSON config file onto cfg. json.Unmarshal only
// sets keys present in the file, leaving env/default values for the rest. Note
// that a JSON "interfaces" array replaces the env-derived interfaces wholesale.
func OverlayConfigFile(cfg *canModels.Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return nil
}

// ConfigFileEnv names the env var that points at an optional JSON config file.
const ConfigFileEnv = "CONFIG_FILE"

// resolveConfigPath returns the JSON config file path from the --config/-c flag
// (parsed manually because Load runs before cobra) or, failing that, the
// CONFIG_FILE env value.
func resolveConfigPath(args []string, envValue string) string {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--config" || a == "-c":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "--config="):
			return strings.TrimPrefix(a, "--config=")
		case strings.HasPrefix(a, "-c="):
			return strings.TrimPrefix(a, "-c=")
		}
	}
	return envValue
}

// ConfigFilePath resolves the JSON config file path from the --config/-c CLI flag
// or the CONFIG_FILE env var.
func ConfigFilePath() string {
	return resolveConfigPath(os.Args[1:], os.Getenv(ConfigFileEnv))
}

func Load(logger *slog.Logger) (canModels.Config, string) {
	cfg, err := env.ParseAs[canModels.Config]()
	if err != nil {
		panic(err)
	}

	// A JSON config file (--config / CONFIG_FILE) is overlaid on top of the
	// env-derived config: only keys present in the file are applied, so env vars
	// and defaults remain in effect for everything the file omits.
	if path := ConfigFilePath(); path != "" {
		if err := OverlayConfigFile(&cfg, path); err != nil {
			panic(err)
		}
		logger.Info("applied JSON config overlay", "path", path)
	}

	if err := ValidateSimRate(&cfg); err != nil {
		panic(err)
	}
	if err := ValidateInterfaces(cfg.CanInterfaces); err != nil {
		panic(err)
	}

	if len(cfg.CanInterfaces) == 0 {
		logger.Warn("no can interfaces defined in env, defaulting to single simulation interface")
		cfg.CanInterfaces = []canModels.CanInterfaceOption{
			{
				Name:    "simulation",
				URI:     "can0",
				Network: "sim",
			},
		}
	}
	cfgJson, err := ToJSON(cfg)
	if err != nil {
		panic(err)
	}
	if cfgJson == nil {
		panic("config.ToJSON returned nil without error")
	}

	return cfg, *cfgJson
}

// SetLogLevel updates lvl based on the string log level from config.
func SetLogLevel(lvl *slog.LevelVar, level string) {
	switch level {
	case "debug", "DEBUG":
		lvl.Set(slog.LevelDebug)
	case "error", "ERROR":
		lvl.Set(slog.LevelError)
	case "warn", "WARN":
		lvl.Set(slog.LevelWarn)
	}
}

// LoadInfluxToken reads the InfluxDB token from TokenFile when Token is empty.
func LoadInfluxToken(cfg *canModels.Config, logger *slog.Logger) error {
	if cfg.InfluxDB.Token != "" || cfg.InfluxDB.TokenFile == "" {
		return nil
	}
	jsonFile, err := os.Open(cfg.InfluxDB.TokenFile)
	if err != nil {
		return fmt.Errorf("opening influxdb token file %s: %w", cfg.InfluxDB.TokenFile, err)
	}
	defer jsonFile.Close()

	var creds struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(jsonFile).Decode(&creds); err != nil {
		return fmt.Errorf("decoding influxdb token file: %w", err)
	}
	cfg.InfluxDB.Token = creds.Token
	return nil
}

func ToJSON(config canModels.Config) (*string, error) {
	// Zero secrets before marshalling to prevent them appearing in logs.
	config.InfluxDB.Token = ""
	config.MQTTConfig.Password = ""
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	jsonString := string(jsonBytes)
	return &jsonString, nil
}
