//go:build rp2350

package main

import (
	"errors"
	"gopper/core"
	"machine"
	"sync"
	"time"
)

// RP2040SoftwareSPIDriver implements core.SoftwareSPIDriver using GPIO bit-banging
type RP2040SoftwareSPIDriver struct {
	mu sync.Mutex

	// Track configured software SPI instances
	instances map[interface{}]*softwareSPIInstance
	nextID    int
}

// softwareSPIInstance holds configuration for a software SPI bus
type softwareSPIInstance struct {
	id   int
	sclk machine.Pin
	mosi machine.Pin
	miso machine.Pin
	mode core.SPIMode
	rate uint32

	// Calculated delay between clock transitions (in nanoseconds)
	halfPeriod time.Duration

	// CPOL and CPHA derived from mode
	cpol bool // Clock polarity: false = idle low, true = idle high
	cpha bool // Clock phase: false = sample on first edge, true = sample on second edge
}

// NewRP2040SoftwareSPIDriver creates a new software SPI driver
func NewRP2040SoftwareSPIDriver() *RP2040SoftwareSPIDriver {
	return &RP2040SoftwareSPIDriver{
		instances: make(map[interface{}]*softwareSPIInstance),
		nextID:    0,
	}
}

// ConfigureSoftwareSPI sets up GPIO pins for software SPI
func (d *RP2040SoftwareSPIDriver) ConfigureSoftwareSPI(sclk, mosi, miso uint32, mode core.SPIMode, rate uint32) (interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create instance
	inst := &softwareSPIInstance{
		id:   d.nextID,
		sclk: machine.Pin(sclk),
		mosi: machine.Pin(mosi),
		miso: machine.Pin(miso),
		mode: mode,
		rate: rate,
	}
	d.nextID++

	// Calculate timing
	// For software SPI, we toggle clock twice per bit (high and low)
	// So half period is 1 / (2 * rate)
	if rate > 0 {
		// Convert to nanoseconds: 1/rate seconds = 1e9/rate nanoseconds for full period
		// Half period = 1e9/(2*rate)
		inst.halfPeriod = time.Duration(500000000/rate) * time.Nanosecond
	} else {
		// Default to 100kHz if rate is 0
		inst.halfPeriod = 5 * time.Microsecond
	}

	// Decode SPI mode into CPOL and CPHA
	switch mode {
	case 0:
		inst.cpol = false // Clock idle low
		inst.cpha = false // Sample on first (rising) edge
	case 1:
		inst.cpol = false // Clock idle low
		inst.cpha = true  // Sample on second (falling) edge
	case 2:
		inst.cpol = true  // Clock idle high
		inst.cpha = false // Sample on first (falling) edge
	case 3:
		inst.cpol = true // Clock idle high
		inst.cpha = true // Sample on second (rising) edge
	default:
		return nil, errors.New("invalid SPI mode")
	}

	// Configure GPIO pins
	inst.sclk.Configure(machine.PinConfig{Mode: machine.PinOutput})
	inst.mosi.Configure(machine.PinConfig{Mode: machine.PinOutput})
	inst.miso.Configure(machine.PinConfig{Mode: machine.PinInput})

	// Set initial clock state based on CPOL
	inst.sclk.Set(inst.cpol)
	inst.mosi.Low()

	// Store instance
	d.instances[inst] = inst

	return inst, nil
}

// Transfer performs a software SPI transfer
func (d *RP2040SoftwareSPIDriver) Transfer(handle interface{}, txData []byte, rxData []byte) error {
	inst, ok := handle.(*softwareSPIInstance)
	if !ok {
		return errors.New("invalid software SPI handle")
	}

	// Validate buffer lengths match
	if len(txData) != len(rxData) {
		return errors.New("tx and rx buffer lengths must match")
	}

	// Perform bit-banged SPI transfer
	for i := 0; i < len(txData); i++ {
		rxData[i] = inst.transferByte(txData[i])
	}

	return nil
}

// transferByte transfers a single byte using bit-banging
func (inst *softwareSPIInstance) transferByte(txByte byte) byte {
	var rxByte byte = 0

	// Transfer 8 bits, MSB first
	for bit := 7; bit >= 0; bit-- {
		// Prepare output bit
		if txByte&(1<<bit) != 0 {
			inst.mosi.High()
		} else {
			inst.mosi.Low()
		}

		// Handle CPHA=0: sample before clock edge
		if !inst.cpha {
			// Small delay to let MOSI settle
			delayNs(50)

			// Read input bit before clock edge
			if inst.miso.Get() {
				rxByte |= (1 << bit)
			}
		}

		// First clock edge
		inst.toggleClock()
		time.Sleep(inst.halfPeriod)

		// Handle CPHA=1: sample after first clock edge
		if inst.cpha {
			// Read input bit after first clock edge
			if inst.miso.Get() {
				rxByte |= (1 << bit)
			}
		}

		// Second clock edge (return to idle state)
		inst.toggleClock()
		time.Sleep(inst.halfPeriod)
	}

	return rxByte
}

// toggleClock toggles the clock line
func (inst *softwareSPIInstance) toggleClock() {
	if inst.sclk.Get() {
		inst.sclk.Low()
	} else {
		inst.sclk.High()
	}
}

// delayNs provides a short delay in nanoseconds
// This is a busy-wait and should only be used for very short delays
func delayNs(ns int) {
	// On RP2040 at 125MHz, each NOP is ~8ns
	// This is a rough approximation
	loops := ns / 8
	for i := 0; i < loops; i++ {
		// Busy wait
		_ = i
	}
}
