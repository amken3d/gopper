//go:build tinygo

package core

import (
	"errors"
	"gopper/protocol"
	"machine"
)

// DriverType identifies the bus type for a driver
type DriverType uint8

const (
	DriverTypeI2C DriverType = iota
	DriverTypeSPI
	DriverTypeGPIO
	DriverTypeCustom
)

// DriverInstance represents a registered driver instance
type DriverInstance struct {
	OID      uint8       // Object ID for Klipper
	Type     DriverType  // Bus type
	Name     string      // Driver name for identification
	Device   interface{} // The actual TinyGo driver instance
	Config   *DriverConfig
	State    DriverState
	Timer    Timer // For polling-based drivers
	pollFunc DriverPollFunc
}

// DriverState tracks the runtime state of a driver
type DriverState struct {
	Configured bool
	Active     bool
	LastError  error
	PollRate   uint32 // Polling interval in timer ticks (0 = no polling)
}

// DriverConfig holds configuration for registering a driver
type DriverConfig struct {
	Name string // Human-readable driver name
	Type DriverType

	// I2C configuration
	I2CBus  I2CBusID
	I2CAddr I2CAddress

	// SPI configuration
	SPIBus   SPIBusID
	SPIMode  SPIMode
	SPIRate  uint32
	SPICSPin GPIOPin // Chip select pin

	// GPIO configuration
	GPIOPins []GPIOPin

	// Custom attributes for driver-specific configuration
	Attributes map[string]interface{}

	// Lifecycle callbacks
	InitFunc      DriverInitFunc      // Called during driver registration
	ConfigureFunc DriverConfigureFunc // Called when driver is configured
	ReadFunc      DriverReadFunc      // Called to read data from driver
	WriteFunc     DriverWriteFunc     // Called to write data to driver
	CloseFunc     DriverCloseFunc     // Called when driver is unregistered

	// Optional polling support for sensors
	PollFunc DriverPollFunc // Called periodically if PollRate > 0
	PollRate uint32         // Default polling interval in milliseconds (0 = disabled)
}

// Driver lifecycle function types
type DriverInitFunc func(cfg *DriverConfig) (interface{}, error)
type DriverConfigureFunc func(device interface{}, cfg *DriverConfig) error
type DriverReadFunc func(device interface{}, params []byte) ([]byte, error)
type DriverWriteFunc func(device interface{}, data []byte) error
type DriverCloseFunc func(device interface{}) error
type DriverPollFunc func(device interface{}) ([]byte, error)

// Global driver registry
var (
	registeredDrivers = make(map[uint8]*DriverInstance)
	driversByName     = make(map[string]*DriverInstance)
)

// RegisterDriver registers a new driver instance with the system.
// This automatically generates and registers Klipper commands for the driver.
func RegisterDriver(oid uint8, config *DriverConfig) error {
	if config == nil {
		return errors.New("driver config is nil")
	}

	if config.Name == "" {
		return errors.New("driver name is required")
	}

	// Check if OID already exists
	if _, exists := registeredDrivers[oid]; exists {
		return errors.New("driver OID already registered")
	}

	// Check if name already exists
	if _, exists := driversByName[config.Name]; exists {
		return errors.New("driver name already registered")
	}

	// Create driver instance
	instance := &DriverInstance{
		OID:    oid,
		Type:   config.Type,
		Name:   config.Name,
		Config: config,
		State: DriverState{
			Configured: false,
			Active:     false,
			PollRate:   config.PollRate,
		},
	}

	// Initialize driver if InitFunc is provided
	if config.InitFunc != nil {
		device, err := config.InitFunc(config)
		if err != nil {
			return err
		}
		instance.Device = device
	}

	// Register in maps
	registeredDrivers[oid] = instance
	driversByName[config.Name] = instance

	return nil
}

// GetDriver retrieves a registered driver by OID
func GetDriver(oid uint8) (*DriverInstance, bool) {
	instance, exists := registeredDrivers[oid]
	return instance, exists
}

// GetDriverByName retrieves a registered driver by name
func GetDriverByName(name string) (*DriverInstance, bool) {
	instance, exists := driversByName[name]
	return instance, exists
}

// UnregisterDriver removes a driver from the registry
func UnregisterDriver(oid uint8) error {
	instance, exists := registeredDrivers[oid]
	if !exists {
		return errors.New("driver not found")
	}

	// Stop polling if active
	if instance.State.Active {
		instance.Timer.Next = nil // Cancel timer
	}

	// Call close function if provided
	if instance.Config.CloseFunc != nil {
		if err := instance.Config.CloseFunc(instance.Device); err != nil {
			return err
		}
	}

	// Remove from maps
	delete(registeredDrivers, oid)
	delete(driversByName, instance.Name)

	return nil
}

// Helper functions for creating common driver configurations

// NewI2CDriverConfig creates a driver config for an I2C device
func NewI2CDriverConfig(name string, bus I2CBusID, addr I2CAddress) *DriverConfig {
	return &DriverConfig{
		Name:       name,
		Type:       DriverTypeI2C,
		I2CBus:     bus,
		I2CAddr:    addr,
		Attributes: make(map[string]interface{}),
	}
}

// NewSPIDriverConfig creates a driver config for an SPI device
func NewSPIDriverConfig(name string, bus SPIBusID, mode SPIMode, rate uint32, csPin GPIOPin) *DriverConfig {
	return &DriverConfig{
		Name:       name,
		Type:       DriverTypeSPI,
		SPIBus:     bus,
		SPIMode:    mode,
		SPIRate:    rate,
		SPICSPin:   csPin,
		Attributes: make(map[string]interface{}),
	}
}

// Helper functions to get machine.* interfaces for driver initialization

// GetMachineI2C returns a configured machine.I2C instance for a bus
func GetMachineI2C(bus I2CBusID) (*machine.I2C, error) {
	busInterface, err := MustI2C().GetMachineBus(bus)
	if err != nil {
		return nil, err
	}

	i2c, ok := busInterface.(*machine.I2C)
	if !ok {
		return nil, errors.New("bus is not a machine.I2C instance")
	}

	return i2c, nil
}

// GetMachineSPI returns a configured machine.SPI instance for a bus handle
func GetMachineSPI(busHandle interface{}) (*machine.SPI, error) {
	if busHandle == nil {
		return nil, errors.New("invalid bus handle")
	}

	spiInterface, err := MustSPI().GetMachineBus(busHandle)
	if err != nil {
		return nil, err
	}

	spi, ok := spiInterface.(*machine.SPI)
	if !ok {
		return nil, errors.New("bus is not a machine.SPI instance")
	}

	return spi, nil
}

// StartPolling starts periodic polling for a driver
func StartPolling(instance *DriverInstance, pollRateTicks uint32) error {
	if instance.Config.PollFunc == nil {
		return errors.New("driver does not support polling")
	}

	if pollRateTicks == 0 {
		return errors.New("poll rate must be greater than 0")
	}

	instance.State.PollRate = pollRateTicks
	instance.State.Active = true
	instance.pollFunc = instance.Config.PollFunc

	// Schedule first poll
	instance.Timer.WakeTime = GetTime() + pollRateTicks
	instance.Timer.Handler = driverPollHandler
	ScheduleTimer(&instance.Timer)

	return nil
}

// StopPolling stops periodic polling for a driver
func StopPolling(instance *DriverInstance) {
	instance.State.Active = false
	instance.Timer.Next = nil // Cancel timer
}

// driverPollHandler is the timer callback for driver polling
func driverPollHandler(t *Timer) uint8 {
	// Find the driver instance that owns this timer
	var instance *DriverInstance
	for _, inst := range registeredDrivers {
		if inst != nil && &inst.Timer == t {
			instance = inst
			break
		}
	}

	if instance == nil || !instance.State.Active {
		return SF_DONE
	}

	// Call the poll function
	if instance.pollFunc != nil {
		data, err := instance.pollFunc(instance.Device)
		if err != nil {
			instance.State.LastError = err
		} else {
			instance.State.LastError = nil

			// Send data as a response if available
			if len(data) > 0 {
				SendResponse("driver_poll_data", func(output protocol.OutputBuffer) {
					protocol.EncodeVLQUint(output, uint32(instance.OID))
					for _, b := range data {
						protocol.EncodeVLQUint(output, uint32(b))
					}
				})
			}
		}
	}

	// Reschedule
	t.WakeTime = t.WakeTime + instance.State.PollRate
	return SF_RESCHEDULE
}
