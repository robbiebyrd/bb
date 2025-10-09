package csv

type CSVCanMessage struct {
	Timestamp string `csv:"Timestamp"`
	Interface string `csv:"Interface"`
	Transmit  uint8  `lp:"Transmit"`
	ID        string `lp:"CAN ID"`
	Length    uint8  `lp:"Data Length"`
	Remote    uint8  `lp:"Remote"`
	Data      string `lp:"Data"`
}
