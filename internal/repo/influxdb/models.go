package influxdb

import "time"

type InfluxDBCanMessage struct {
	Timestamp   time.Time `lp:"timestamp"`
	Interface   string    `lp:"tag,interface"`
	Transmit    uint8     `lp:"field,transmit"`
	ID          string    `lp:"tag,can_id"`
	Length      uint8     `lp:"field,length"`
	Remote      uint8     `lp:"field,remote"`
	Data        string    `lp:"tag,data"`
	Measurement string    `lp:"measurement"`
}
