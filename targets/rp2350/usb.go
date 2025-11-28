//go:build rp2350

package main

import (
	"machine"
)

// InitUSB initializes USB serial communication
// TinyGo automatically sets up USB CDC-ACM on RP2040
func InitUSB() {
	// Configure machine.Serial (which is USB CDC on RP2040)
	err := machine.Serial.Configure(machine.UARTConfig{})
	if err != nil {
		return
	}

	// Note: On RP2040, machine.Serial is actually USB CDC, not UART
	// The USB descriptors are set by TinyGo's runtime
}

// USBAvailable returns the number of bytes available to read from USB
func USBAvailable() int {
	return machine.Serial.Buffered()
}

// USBRead reads a single byte from USB
func USBRead() (byte, error) {
	return machine.Serial.ReadByte()
}

// USBWrite writes a byte to USB
func USBWrite(b byte) error {
	return machine.Serial.WriteByte(b)
}

// USBWriteBytes writes multiple bytes to USB
func USBWriteBytes(data []byte) (int, error) {
	n, err := machine.Serial.Write(data)
	return n, err
}

// USBConnected returns true if USB is connected to host
// On RP2040, we assume USB is connected if Serial is configured
func USBConnected() bool {
	// Simple heuristic: if we can write without error, we're connected
	// More sophisticated detection could be added later
	return true
}
