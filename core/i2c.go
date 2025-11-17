//go:build tinygo

// I2C (Inter-Integrated Circuit) support
// Implements Klipper's I2C protocol for communicating with I2C devices
package core

import (
	"gopper/protocol"
)

// I2CDevice represents a configured I2C device
type I2CDevice struct {
	OID     uint8      // Object ID
	Bus     I2CBusID   // I2C bus number
	Address I2CAddress // 7-bit I2C address
	Ready   bool       // Whether the bus has been configured
}

// Global registry of I2C devices
var i2cDevices = make(map[uint8]*I2CDevice)

// InitI2CCommands registers I2C-related commands with the command registry
func InitI2CCommands() {
	// Command to allocate an I2C device object
	RegisterCommand("config_i2c", "oid=%c", handleConfigI2C)

	// Command to configure the I2C bus and device address
	RegisterCommand("i2c_set_bus", "oid=%c i2c_bus=%u rate=%u address=%u", handleI2CSetBus)

	// Command to write data to the I2C device
	RegisterCommand("i2c_write", "oid=%c data=%*s", handleI2CWrite)

	// Command to read data from the I2C device (with optional register address)
	RegisterCommand("i2c_read", "oid=%c reg=%*s read_len=%u", handleI2CRead)

	// Response message: I2C read result (MCU → Host)
	RegisterCommand("i2c_read_response", "oid=%c response=%*s", nil)
}

// handleConfigI2C allocates an I2C device object
// Format: config_i2c oid=%c
func handleConfigI2C(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Create new I2C device instance
	device := &I2CDevice{
		OID:   uint8(oid),
		Ready: false,
	}

	// Register in global map
	i2cDevices[uint8(oid)] = device

	return nil
}

// handleI2CSetBus configures the I2C bus, rate, and device address
// Format: i2c_set_bus oid=%c i2c_bus=%u rate=%u address=%u
func handleI2CSetBus(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	bus, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	rate, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	address, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the I2C device object
	device, exists := i2cDevices[uint8(oid)]
	if !exists {
		return nil // Invalid OID
	}

	// Mask address to 7 bits (Klipper behavior)
	device.Address = I2CAddress(address & 0x7F)
	device.Bus = I2CBusID(bus)

	// Configure the I2C bus via HAL
	if err := MustI2C().ConfigureBus(device.Bus, rate); err != nil {
		return err
	}

	device.Ready = true

	return nil
}

// handleI2CWrite writes data to an I2C device
// Format: i2c_write oid=%c data=%*s
func handleI2CWrite(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Decode the data buffer
	writeData, err := protocol.DecodeVLQBytes(data)
	if err != nil {
		return err
	}

	// Get the I2C device object
	device, exists := i2cDevices[uint8(oid)]
	if !exists || !device.Ready {
		return nil // Invalid OID or not configured
	}

	// Write data via HAL
	if err := MustI2C().Write(device.Bus, device.Address, writeData); err != nil {
		// I2C error - trigger shutdown like Klipper does
		TryShutdown("I2C write error")
		return err
	}

	return nil
}

// handleI2CRead reads data from an I2C device (with optional register addressing)
// Format: i2c_read oid=%c reg=%*s read_len=%u
func handleI2CRead(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Decode the register data (may be empty)
	regData, err := protocol.DecodeVLQBytes(data)
	if err != nil {
		return err
	}

	readLen, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the I2C device object
	device, exists := i2cDevices[uint8(oid)]
	if !exists || !device.Ready {
		return nil // Invalid OID or not configured
	}

	// Read data via HAL
	readData, err := MustI2C().Read(device.Bus, device.Address, regData, uint8(readLen))
	if err != nil {
		// I2C error - trigger shutdown like Klipper does
		TryShutdown("I2C read error")
		return err
	}

	// Send i2c_read_response message
	SendResponse("i2c_read_response", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(oid))
		protocol.EncodeVLQBytes(output, readData)
	})

	return nil
}

// ShutdownAllI2C stops all I2C operations (called during shutdown)
func ShutdownAllI2C() {
	// Mark all devices as not ready
	for _, device := range i2cDevices {
		if device != nil {
			device.Ready = false
		}
	}
}
