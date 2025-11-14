package models

type InfluxDBConfig struct {
	Host           string `env:"HOST,required"`
	Token          string `env:"TOKEN" envDefault:""`
	TokenFile      string `env:"TOKEN_FILE" envDefault:"./config/influxdb/token.json`
	Database       string `env:"DATABASE" envDefault:"can_data"`
	TableName      string `env:"TABLE" envDefault:"can_message"`
	FlushTime      int    `env:"FLUSH_TIME" envDefault:"100"`
	MaxWriteLines  int    `env:"MAX_WRITE_LINES" envDefault:"1000"`
	MaxConnections int    `env:"MAX_CONNECTIONS" envDefault:"5"`
}

type MQTTConfig struct {
	Host          string   `env:"HOST,required"`
	ClientId      string   `env:"CLIENTID,required"`
	Topic         string   `env:"TOPIC" envDefault:"can_data"`
	Qos           uint8    `env:"QOS" envDefault:"0"`
	ShadowCopy    bool     `env:"SHADOW_COPY" envDefault:"false"`
	Dedupe        bool     `env:"DEDUPE" envDefault:"true"`
	DedupeTimeout int      `env:"DEDUPE_TIMEOUT_MS" envDefault:"1000"`
	DedupeIDs     []uint32 `env:"DEDUPE_IDS" envDefault:""` // Comma-separated list of IDs to dedupe
}

type CSVLogConfig struct {
	OutputFile     string `env:"OUTPUT_FILE,required"`
	IncludeHeaders bool   `env:"OUTPUT_HEADERS" envDefault:"true"`
}

type Config struct {
	CanInterfaces         []CanInterfaceOption `envPrefix:"INTERFACE"`
	MessageBufferSize     int                  `env:"MSG_BUFFER_SIZE" envDefault:"81920"`
	InfluxDB              InfluxDBConfig       `envPrefix:"INFLUX_"`
	CSVLog                CSVLogConfig         `envPrefix:"CSV_"`
	MQTTConfig            MQTTConfig           `envPrefix:"MQTT_"`
	SimEmitRate           int                  `env:"SIM_RATE" envDefault:"10"`
	LogLevel              string               `env:"LOG_LEVEL" envDefault:"info"`
	CanInterfaceSeparator string               `env:"CAN_INTERFACE_SEPARATOR" envDefault:"-"`
}
