//go:build !wasm

package serial

import (
	"fmt"
	"time"

	"github.com/tarm/serial"
)

// NativePort wraps the tarm/serial implementation
type NativePort struct {
	port *serial.Port
	cfg  *Config
}

// Open opens a native serial port
func Open(cfg *Config) (Port, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	serialConfig := &serial.Config{
		Name:        cfg.Device,
		Baud:        cfg.Baud,
		ReadTimeout: time.Duration(cfg.ReadTimeout) * time.Millisecond,
	}

	port, err := serial.OpenPort(serialConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", cfg.Device, err)
	}

	return &NativePort{
		port: port,
		cfg:  cfg,
	}, nil
}

// Read reads data from the serial port
func (p *NativePort) Read(b []byte) (int, error) {
	return p.port.Read(b)
}

// Write writes data to the serial port
func (p *NativePort) Write(b []byte) (int, error) {
	return p.port.Write(b)
}

// Close closes the serial port
func (p *NativePort) Close() error {
	if p.port != nil {
		return p.port.Close()
	}
	return nil
}

// Flush flushes the serial port buffers
func (p *NativePort) Flush() error {
	// tarm/serial doesn't expose flush, but we can simulate it
	// by just ensuring all data is written (which Write() does)
	return nil
}
