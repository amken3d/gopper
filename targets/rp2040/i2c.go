//go:build rp2040 || rp2350

package main

import (
	"errors"
	"gopper/core"
	"machine"
	"sync"
)

// RPI2CDriver implements core.I2CDriver using TinyGo's machine.I2C for RP2040/RP2350.
type RPI2CDriver struct {
	mu sync.Mutex

	// Configured I2C buses
	// RP2040/RP2350 have I2C0 and I2C1
	buses map[core.I2CBusID]*machine.I2C

	// Track configuration state per bus
	configured map[core.I2CBusID]bool
}

// NewRPI2CDriver constructs the driver
func NewRPI2CDriver() *RPI2CDriver {
	return &RPI2CDriver{
		buses:      make(map[core.I2CBusID]*machine.I2C),
		configured: make(map[core.I2CBusID]bool),
	}
}

// ConfigureBus initializes a specific I2C bus with the given frequency.
func (d *RPI2CDriver) ConfigureBus(bus core.I2CBusID, frequencyHz uint32) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if already configured with same settings
	if d.configured[bus] {
		// Already configured - just update baud rate if needed
		i2c, exists := d.buses[bus]
		if !exists {
			return errors.New("I2C bus internal state error")
		}
		return i2c.SetBaudRate(frequencyHz)
	}

	// Get the machine.I2C instance for this bus
	var i2c *machine.I2C

	switch bus {
	case 0:
		// I2C0 - Default pins: SDA=GP4, SCL=GP5
		// Users can configure different pins via machine.I2C0.Configure()
		i2c = machine.I2C0
	case 1:
		// I2C1 - Default pins: SDA=GP6, SCL=GP7
		// Users can configure different pins via machine.I2C1.Configure()
		i2c = machine.I2C1
	default:
		return errors.New("unsupported I2C bus ID")
	}

	// Configure the I2C bus
	// Use default pins (can be overridden by board-specific code)
	err := i2c.Configure(machine.I2CConfig{
		Frequency: frequencyHz,
		// SDA and SCL pins are set to defaults by TinyGo
		// For I2C0: SDA=GP4, SCL=GP5
		// For I2C1: SDA=GP6, SCL=GP7
	})

	if err != nil {
		return err
	}

	// Store the configured bus
	d.buses[bus] = i2c
	d.configured[bus] = true

	return nil
}

// Write transmits data to a device at the given address on the specified bus.
func (d *RPI2CDriver) Write(bus core.I2CBusID, addr core.I2CAddress, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	i2c, exists := d.buses[bus]
	if !exists {
		return errors.New("I2C bus not configured")
	}

	// TinyGo's Tx method handles the complete I2C transaction
	// Tx(addr uint16, w, r []byte) error
	// For write-only, we pass nil for the read buffer
	err := i2c.Tx(uint16(addr), data, nil)
	if err != nil {
		return err
	}

	return nil
}

// Read reads data from a device, optionally writing a register address first.
// If regData is non-empty, it's transmitted before the read (restart in between).
func (d *RPI2CDriver) Read(bus core.I2CBusID, addr core.I2CAddress, regData []byte, readLen uint8) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	i2c, exists := d.buses[bus]
	if !exists {
		return nil, errors.New("I2C bus not configured")
	}

	// Allocate buffer for read data
	readBuf := make([]byte, readLen)

	if len(regData) > 0 {
		// Write register address, then read
		// TinyGo's Tx handles this with a restart condition
		err := i2c.Tx(uint16(addr), regData, readBuf)
		if err != nil {
			return nil, err
		}
	} else {
		// Read-only transaction
		err := i2c.Tx(uint16(addr), nil, readBuf)
		if err != nil {
			return nil, err
		}
	}

	return readBuf, nil
}

// GetMachineBus returns the underlying machine.I2C instance for a bus.
// This allows direct use of TinyGo drivers that expect machine.I2C.
func (d *RPI2CDriver) GetMachineBus(bus core.I2CBusID) (interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	i2c, exists := d.buses[bus]
	if !exists {
		return nil, errors.New("I2C bus not configured")
	}

	return i2c, nil
}
