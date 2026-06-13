package obd

import (
	"fmt"
	"time"

	"go.bug.st/serial"
)

// Transport is the byte stream an OBD device speaks over. It is deliberately
// just an io.ReadWriteCloser so the protocol layer can be unit-tested against an
// in-memory fake and driven over a serial port, a bound Bluetooth rfcomm device,
// or (later) a TCP socket without change.
type Transport interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// SerialConfig configures a serial/rfcomm transport.
type SerialConfig struct {
	// Port is the device path, e.g. "/dev/ttyUSB0", "/dev/rfcomm0", or a macOS
	// "/dev/cu.*" path for a bound Bluetooth adapter.
	Port string
	// Baud is the serial line speed. For Bluetooth SPP/rfcomm links the value is
	// nominal — the Bluetooth link governs throughput — but a value is still
	// required by the serial driver.
	Baud int
	// ReadTimeout bounds a single Read so the monitor loop can poll for stop
	// signals between reads. A non-blocking-ish small timeout keeps shutdown
	// responsive.
	ReadTimeout time.Duration
}

// OpenSerial opens a serial/rfcomm transport. The caller is responsible for
// having bound the Bluetooth device first (e.g. "rfcomm bind /dev/rfcomm0 <MAC> 1").
func OpenSerial(cfg SerialConfig) (Transport, error) {
	mode := &serial.Mode{
		BaudRate: cfg.Baud,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(cfg.Port, mode)
	if err != nil {
		return nil, fmt.Errorf("opening serial port %q: %w", cfg.Port, err)
	}
	readTimeout := cfg.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 50 * time.Millisecond
	}
	if err := port.SetReadTimeout(readTimeout); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("setting read timeout on %q: %w", cfg.Port, err)
	}
	return port, nil
}
