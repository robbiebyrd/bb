package models

type InfluxDBConfig struct {
	Host           string `env:"HOST,required"`
	Token          string `env:"TOKEN,required"`
	Database       string `env:"DATABASE" envDefault:"can_data"`
	TableName      string `env:"TABLE" envDefault:"can_message"`
	FlushTime      int    `env:"FLUSH_TIME" envDefault:"100"`
	MaxWriteLines  int    `env:"MAX_WRITE_LINES" envDefault:"1000"`
	MaxConnections int    `env:"MAX_CONNECTIONS" envDefault:"5"`
}

type Config struct {
	CanInterfaces     []CanInterfaceOption `envPrefix:"INTERFACE"`
	MessageBufferSize int                  `env:"MSG_BUFFER_SIZE" envDefault:"81920"`
	InfluxDB          InfluxDBConfig       `envPrefix:"INFLUX_"`
	SimEmitRate       int                  `env:"SIM_RATE" envDefault:"10"`
}
