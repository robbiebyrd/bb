package influxdb

import "time"

type InfluxDBCanMessage struct {
	Timestamp   time.Time `lp:"timestamp"`
	Interface   string    `lp:"tag,interface"`
	Transmit    uint8     `lp:"field,transmit"`
	ID          string    `lp:"tag,can_id"`
	Length      uint8     `lp:"field,length"`
	Remote      uint8     `lp:"field,remote"`
	Data        string    `lp:"field,data"`
	Measurement string    `lp:"measurement"`
}

type InfluxDBSignalMessage struct {
	Timestamp   time.Time `lp:"timestamp"`
	Interface   string    `lp:"tag,interface"`
	Message     string    `lp:"tag,message"`
	Signal      string    `lp:"tag,signal"`
	Unit        string    `lp:"tag,unit"`
	Value       float64   `lp:"field,value"`
	Measurement string    `lp:"measurement"`
}
