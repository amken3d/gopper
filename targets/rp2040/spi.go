//go:build rp2040 || rp2350

package main

import (
	"errors"
	"gopper/core"
	"machine"
	"sync"
)

// RP2040/RP2350 SPI Bus Configurations
// Matches Klipper's RP2040 SPI bus definitions
// Each bus specifies which SPI controller and GPIO pins to use

type spiBusConfig struct {
	spi  *machine.SPI // SPI controller (SPI0 or SPI1)
	sck  machine.Pin  // Clock pin
	mosi machine.Pin  // Master Out Slave In
	miso machine.Pin  // Master In Slave Out
	name string       // Human-readable name
}

var rp2040SPIBuses = map[core.SPIBusID]spiBusConfig{
	// SPI0 configurations
	0: {spi: machine.SPI0, sck: machine.GPIO2, mosi: machine.GPIO3, miso: machine.GPIO0, name: "spi0a"},
	1: {spi: machine.SPI0, sck: machine.GPIO6, mosi: machine.GPIO7, miso: machine.GPIO4, name: "spi0b"},
	2: {spi: machine.SPI0, sck: machine.GPIO18, mosi: machine.GPIO19, miso: machine.GPIO16, name: "spi0c"},
	3: {spi: machine.SPI0, sck: machine.GPIO22, mosi: machine.GPIO23, miso: machine.GPIO20, name: "spi0d"},
	4: {spi: machine.SPI0, sck: machine.GPIO2, mosi: machine.GPIO3, miso: machine.GPIO4, name: "spi0e"},

	// SPI1 configurations
	5: {spi: machine.SPI1, sck: machine.GPIO10, mosi: machine.GPIO11, miso: machine.GPIO8, name: "spi1a"},
	6: {spi: machine.SPI1, sck: machine.GPIO14, mosi: machine.GPIO15, miso: machine.GPIO12, name: "spi1b"},
	7: {spi: machine.SPI1, sck: machine.GPIO26, mosi: machine.GPIO27, miso: machine.GPIO24, name: "spi1c"},
	8: {spi: machine.SPI1, sck: machine.GPIO10, mosi: machine.GPIO11, miso: machine.GPIO12, name: "spi1d"},
}

// RP2040SPIDriver implements core.SPIDriver using TinyGo's machine.SPI
type RP2040SPIDriver struct {
	mu sync.Mutex

	// Track configured buses to avoid reconfiguration
	// Map of bus ID to configured SPI instance
	configuredBuses map[core.SPIBusID]*spiInstance
}

// spiInstance holds configuration for a specific SPI bus
type spiInstance struct {
	spi    *machine.SPI
	busID  core.SPIBusID
	mode   core.SPIMode
	rate   uint32
	config spiBusConfig
}

// NewRP2040SPIDriver creates a new RP2040 SPI driver
func NewRP2040SPIDriver() *RP2040SPIDriver {
	return &RP2040SPIDriver{
		configuredBuses: make(map[core.SPIBusID]*spiInstance),
	}
}

// ConfigureBus sets up a hardware SPI bus with specified parameters
func (d *RP2040SPIDriver) ConfigureBus(config core.SPIConfig) (interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if bus is already configured with same settings
	if inst, exists := d.configuredBuses[config.BusID]; exists {
		// If settings match, return existing instance
		if inst.mode == config.Mode && inst.rate == config.Rate {
			return inst, nil
		}
		// Otherwise, reconfigure
	}

	// Validate bus ID
	busConfig, exists := rp2040SPIBuses[config.BusID]
	if !exists {
		return nil, errors.New("invalid SPI bus ID")
	}

	// Configure SPI pins and controller
	spi := busConfig.spi

	// TinyGo's SPI mode constants match standard SPI modes
	var mode uint8
	switch config.Mode {
	case 0:
		mode = 0 // Mode 0: CPOL=0, CPHA=0
	case 1:
		mode = 1 // Mode 1: CPOL=0, CPHA=1
	case 2:
		mode = 2 // Mode 2: CPOL=1, CPHA=0
	case 3:
		mode = 3 // Mode 3: CPOL=1, CPHA=1
	default:
		return nil, errors.New("invalid SPI mode")
	}

	// Configure SPI with TinyGo's machine.SPIConfig
	err := spi.Configure(machine.SPIConfig{
		Frequency: config.Rate,
		SCK:       busConfig.sck,
		SDO:       busConfig.mosi, // SDO = Serial Data Out (MOSI)
		SDI:       busConfig.miso, // SDI = Serial Data In (MISO)
		Mode:      mode,
	})
	if err != nil {
		return nil, err
	}

	// Create and store instance
	inst := &spiInstance{
		spi:    spi,
		busID:  config.BusID,
		mode:   config.Mode,
		rate:   config.Rate,
		config: busConfig,
	}

	d.configuredBuses[config.BusID] = inst

	return inst, nil
}

// Transfer performs a bidirectional SPI transfer
func (d *RP2040SPIDriver) Transfer(busHandle interface{}, txData []byte, rxData []byte) error {
	inst, ok := busHandle.(*spiInstance)
	if !ok {
		return errors.New("invalid SPI bus handle")
	}

	// Validate buffer lengths match
	if len(txData) != len(rxData) {
		return errors.New("tx and rx buffer lengths must match")
	}

	// TinyGo's SPI.Tx performs a full-duplex transfer
	// It sends txData and receives into rxData simultaneously
	err := inst.spi.Tx(txData, rxData)
	if err != nil {
		return err
	}

	return nil
}

// GetBusInfo returns information about available SPI buses
func (d *RP2040SPIDriver) GetBusInfo() map[core.SPIBusID]string {
	info := make(map[core.SPIBusID]string)
	for id, config := range rp2040SPIBuses {
		info[id] = config.name
	}
	return info
}
