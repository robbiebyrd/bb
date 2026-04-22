package models

type InfluxDBConfig struct {
	Host            string `env:"HOST"             envDefault:""`
	Token           string `env:"TOKEN"            envDefault:""`
	TokenFile       string `env:"TOKEN_FILE"       envDefault:"./config/influxdb/token.json"`
	MessageDatabase string `env:"MESSAGE_DATABASE" envDefault:"can_data"`
	SignalDatabase  string `env:"SIGNAL_DATABASE"  envDefault:""`
	TableName       string `env:"TABLE"            envDefault:"can_message"`
	SignalTableName string `env:"SIGNAL_TABLE"     envDefault:"can_signal"`
	FlushTime       int    `env:"FLUSH_TIME"       envDefault:"100"`
	MaxWriteLines   int    `env:"MAX_WRITE_LINES"  envDefault:"1000"`
	MaxConnections  int      `env:"MAX_CONNECTIONS"  envDefault:"5"`
	TLS             bool     `env:"TLS"              envDefault:"false"`
	TLSCACertFile   string   `env:"TLS_CA_FILE"      envDefault:""`
	Dedupe          bool     `env:"DEDUPE"           envDefault:"false"`
	DedupeTimeout   int      `env:"DEDUPE_TIMEOUT_MS" envDefault:"1000"`
	DedupeIDs       []uint32 `env:"DEDUPE_IDS"       envDefault:""` // Comma-separated list of IDs to dedupe
}

type MQTTConfig struct {
	Host          string   `env:"HOST"      envDefault:""`
	ClientId      string   `env:"CLIENT_ID" envDefault:""`
	Topic         string   `env:"TOPIC" envDefault:"can_data"`
	Qos           uint8    `env:"QOS" envDefault:"0"`
	ShadowCopy    bool     `env:"SHADOW_COPY" envDefault:"false"`
	Dedupe        bool     `env:"DEDUPE" envDefault:"true"`
	DedupeTimeout int      `env:"DEDUPE_TIMEOUT_MS" envDefault:"1000"`
	DedupeIDs     []uint32 `env:"DEDUPE_IDS" envDefault:""` // Comma-separated list of IDs to dedupe
	Username      string   `env:"USERNAME" envDefault:""`
	Password      string   `env:"PASSWORD" envDefault:""`
	TLS           bool   `env:"TLS"         envDefault:"false"`
	TLSCACertFile string `env:"TLS_CA_FILE"  envDefault:""`
}

type CSVLogConfig struct {
	CanOutputFile    string `env:"CAN_OUTPUT_FILE"    envDefault:""`
	SignalOutputFile string `env:"SIGNAL_OUTPUT_FILE" envDefault:""`
	IncludeHeaders   bool   `env:"OUTPUT_HEADERS"     envDefault:"true"`
}

type CRTDLogConfig struct {
	CanOutputFile    string `env:"CAN_OUTPUT_FILE"    envDefault:""`
	SignalOutputFile string `env:"SIGNAL_OUTPUT_FILE" envDefault:""`
}

type MF4LogConfig struct {
	CanOutputFile    string `env:"CAN_OUTPUT_FILE"    envDefault:""`
	SignalOutputFile string `env:"SIGNAL_OUTPUT_FILE" envDefault:""`
	// Finalize rewrites the DT block length and flips the ID block magic
	// to "MDF     " on graceful shutdown so external MDF4 tools accept the
	// file as finalized. Disable to always leave the file in streaming
	// ("UnFinMF ") form — safer if cantou may be killed abruptly.
	Finalize bool `env:"FINALIZE" envDefault:"true"`
}

type PrometheusConfig struct {
	ListenAddr string `env:"LISTEN_ADDR" envDefault:""`
	Path       string `env:"PATH"        envDefault:"/metrics"`
}

type Config struct {
	CanInterfaces []CanInterfaceOption `envPrefix:"INTERFACE"`
	InfluxDB      InfluxDBConfig       `envPrefix:"INFLUX_"`
	CSVLog        CSVLogConfig         `envPrefix:"CSV_"`
	CRTDLogger    CRTDLogConfig        `envPrefix:"CRTD_"`
	MF4Logger     MF4LogConfig         `envPrefix:"MF4_"`
	MQTTConfig    MQTTConfig           `envPrefix:"MQTT_"`
	Prometheus    PrometheusConfig     `envPrefix:"PROMETHEUS_"`

	DisableOBD2           bool   `env:"DISABLE_OBD2"            envDefault:"false"`
	MessageBufferSize     int    `env:"MSG_BUFFER_SIZE"         envDefault:"81920"`
	// SimEmitRate is the fixed sleep interval between simulated CAN frames, in
	// milliseconds. Ignored when SimEmitRateMin and SimEmitRateMax are both set.
	// The default (10ms) yields ~100 msg/s — a reasonable rate for local development.
	SimEmitRate    int `env:"SIM_RATE"     envDefault:"10"`
	// SimEmitRateMin / SimEmitRateMax define an inclusive millisecond range for a
	// random per-frame sleep interval. Both must be set and Min < Max for random
	// mode to activate; otherwise SimEmitRate is used.
	SimEmitRateMin int `env:"SIM_RATE_MIN" envDefault:"0"`
	SimEmitRateMax int `env:"SIM_RATE_MAX" envDefault:"0"`
	LogLevel              string `env:"LOG_LEVEL"               envDefault:"info"`
	CanInterfaceSeparator string `env:"CAN_INTERFACE_SEPARATOR" envDefault:"-"`
	LogCanMessages        bool   `env:"LOG_CAN_MESSAGES"        envDefault:"true"`
	LogSignals            bool   `env:"LOG_SIGNALS"             envDefault:"true"`
}
