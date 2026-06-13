package models

type InfluxDBConfig struct {
	Host            string   `env:"HOST"             envDefault:""                              json:"host"`
	Token           string   `env:"TOKEN"            envDefault:""                              json:"token"`
	TokenFile       string   `env:"TOKEN_FILE"       envDefault:"./config/influxdb/token.json" json:"tokenFile"`
	MessageDatabase string   `env:"MESSAGE_DATABASE" envDefault:"can_data"                     json:"messageDatabase"`
	SignalDatabase  string   `env:"SIGNAL_DATABASE"  envDefault:""                              json:"signalDatabase"`
	TableName       string   `env:"TABLE"            envDefault:"can_message"                  json:"table"`
	SignalTableName string   `env:"SIGNAL_TABLE"     envDefault:"can_signal"                   json:"signalTable"`
	FlushTime       int      `env:"FLUSH_TIME"       envDefault:"100"                          json:"flushTime"`
	MaxWriteLines   int      `env:"MAX_WRITE_LINES"  envDefault:"1000"                         json:"maxWriteLines"`
	MaxConnections  int      `env:"MAX_CONNECTIONS"  envDefault:"5"                            json:"maxConnections"`
	TLS             bool     `env:"TLS"              envDefault:"false"                        json:"tls"`
	TLSCACertFile   string   `env:"TLS_CA_FILE"      envDefault:""                              json:"tlsCaFile"`
	Dedupe          bool     `env:"DEDUPE"           envDefault:"false"                        json:"dedupe"`
	DedupeTimeout   int      `env:"DEDUPE_TIMEOUT_MS" envDefault:"1000"                        json:"dedupeTimeoutMs"`
	DedupeIDs       []uint32 `env:"DEDUPE_IDS"       envDefault:""                              json:"dedupeIds"`
}

type MQTTConfig struct {
	Host          string   `env:"HOST"             envDefault:""         json:"host"`
	ClientId      string   `env:"CLIENT_ID"        envDefault:""         json:"clientId"`
	Topic         string   `env:"TOPIC"            envDefault:"can_data" json:"topic"`
	Qos           uint8    `env:"QOS"              envDefault:"0"        json:"qos"`
	ShadowCopy    bool     `env:"SHADOW_COPY"      envDefault:"false"    json:"shadowCopy"`
	Dedupe        bool     `env:"DEDUPE"           envDefault:"true"     json:"dedupe"`
	DedupeTimeout int      `env:"DEDUPE_TIMEOUT_MS" envDefault:"1000"    json:"dedupeTimeoutMs"`
	DedupeIDs     []uint32 `env:"DEDUPE_IDS"       envDefault:""         json:"dedupeIds"`
	Username      string   `env:"USERNAME"         envDefault:""         json:"username"`
	Password      string   `env:"PASSWORD"         envDefault:""         json:"password"`
	TLS           bool     `env:"TLS"              envDefault:"false"    json:"tls"`
	TLSCACertFile string   `env:"TLS_CA_FILE"      envDefault:""         json:"tlsCaFile"`
}

type CSVLogConfig struct {
	CanOutputFile    string `env:"CAN_OUTPUT_FILE"    envDefault:"" json:"canOutputFile"`
	SignalOutputFile string `env:"SIGNAL_OUTPUT_FILE" envDefault:"" json:"signalOutputFile"`
	IncludeHeaders   bool   `env:"OUTPUT_HEADERS"     envDefault:"true" json:"includeHeaders"`
}

type CRTDLogConfig struct {
	CanOutputFile    string `env:"CAN_OUTPUT_FILE"    envDefault:"" json:"canOutputFile"`
	SignalOutputFile string `env:"SIGNAL_OUTPUT_FILE" envDefault:"" json:"signalOutputFile"`
}

type MF4LogConfig struct {
	CanOutputFile    string `env:"CAN_OUTPUT_FILE"    envDefault:"" json:"canOutputFile"`
	SignalOutputFile string `env:"SIGNAL_OUTPUT_FILE" envDefault:"" json:"signalOutputFile"`
	// Finalize rewrites the DT block length and flips the ID block magic
	// to "MDF     " on graceful shutdown so external MDF4 tools accept the
	// file as finalized. Disable to always leave the file in streaming
	// ("UnFinMF ") form — safer if cantou may be killed abruptly.
	Finalize bool `env:"FINALIZE" envDefault:"true" json:"finalize"`
}

type PrometheusConfig struct {
	ListenAddr string `env:"LISTEN_ADDR" envDefault:""         json:"listenAddr"`
	Path       string `env:"PATH"        envDefault:"/metrics" json:"path"`
}

type Config struct {
	CanInterfaces []CanInterfaceOption `envPrefix:"INTERFACE"   json:"interfaces"`
	InfluxDB      InfluxDBConfig       `envPrefix:"INFLUX_"     json:"influx"`
	CSVLog        CSVLogConfig         `envPrefix:"CSV_"        json:"csv"`
	CRTDLogger    CRTDLogConfig        `envPrefix:"CRTD_"       json:"crtd"`
	MF4Logger     MF4LogConfig         `envPrefix:"MF4_"        json:"mf4"`
	MQTTConfig    MQTTConfig           `envPrefix:"MQTT_"       json:"mqtt"`
	Prometheus    PrometheusConfig     `envPrefix:"PROMETHEUS_" json:"prometheus"`

	DisableOBD2       bool `env:"DISABLE_OBD2"            envDefault:"false" json:"disableObd2"`
	MessageBufferSize int  `env:"MSG_BUFFER_SIZE"         envDefault:"81920" json:"messageBufferSize"`
	// SimEmitRate is the fixed sleep interval between simulated CAN frames in
	// milliseconds. 0 means unset. When set, it takes priority over SimEmitRateMin/Max.
	// A value of 10 (10ms) yields ~100 msg/s — a reasonable rate for local development.
	SimEmitRate int `env:"SIM_RATE"     envDefault:"0" json:"simRate"`
	// SimEmitRateMin / SimEmitRateMax define an inclusive millisecond range for a
	// random per-frame sleep interval. Both must be non-zero; only active when
	// SimEmitRate is 0. Returns an error at startup if only one is set.
	SimEmitRateMin        int    `env:"SIM_RATE_MIN" envDefault:"0" json:"simRateMin"`
	SimEmitRateMax        int    `env:"SIM_RATE_MAX" envDefault:"0" json:"simRateMax"`
	LogLevel              string `env:"LOG_LEVEL"               envDefault:"info" json:"logLevel"`
	CanInterfaceSeparator string `env:"CAN_INTERFACE_SEPARATOR" envDefault:"-"    json:"canInterfaceSeparator"`
	LogCanMessages        bool   `env:"LOG_CAN_MESSAGES"        envDefault:"true" json:"logCanMessages"`
	LogSignals            bool   `env:"LOG_SIGNALS"             envDefault:"true" json:"logSignals"`
}
