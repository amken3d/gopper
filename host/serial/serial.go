package serial

import (
	"io"
)

// Port represents a serial port interface
// This abstraction allows for different implementations:
// - Native serial (using github.com/tarm/serial)
// - WebSerial (for TinyGo WASM builds)
// - Mock serial (for testing)
type Port interface {
	io.ReadWriteCloser

	// Flush flushes any buffered data
	Flush() error
}

// Config holds serial port configuration
type Config struct {
	// Device path (e.g., "/dev/ttyACM0", "COM3")
	Device string

	// Baud rate (typically 250000 for Klipper, but USB CDC ignores this)
	Baud int

	// Read timeout in milliseconds (0 = blocking)
	ReadTimeout int
}

// DefaultConfig returns a default configuration for Klipper
func DefaultConfig(device string) *Config {
	return &Config{
		Device:      device,
		Baud:        250000,      // Standard Klipper baud rate
		ReadTimeout: 100,         // 100ms read timeout
	}
}
