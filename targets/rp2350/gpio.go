//go:build rp2350

package main

import (
	"gopper/core"
	"machine"
)

// RPGPIODriver implements the GPIODriver interface for RP2040
type RPGPIODriver struct {
	// Track configured pins to prevent conflicts
	configuredPins map[core.GPIOPin]machine.Pin
}

func (d *RPGPIODriver) ConfigureInputPullUp(pin core.GPIOPin) error {
	// Check if already configured
	if _, exists := d.configuredPins[pin]; exists {
		// Already configured, this is OK
		return nil
	}

	// Map pin to machine.Pin
	machinePin := d.pinNumberToMachinePin(pin)

	// Configure as input with pull-up resistor
	machinePin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	// Track configured pin
	d.configuredPins[pin] = machinePin

	return nil
}

func (d *RPGPIODriver) ConfigureInputPullDown(pin core.GPIOPin) error {
	// Check if already configured
	if _, exists := d.configuredPins[pin]; exists {
		// Already configured, this is OK
		return nil
	}

	// Map pin to machine.Pin
	machinePin := d.pinNumberToMachinePin(pin)

	// Configure as input with pull-down resistor
	machinePin.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})

	// Track configured pin
	d.configuredPins[pin] = machinePin

	return nil
}

func (d *RPGPIODriver) ReadPin(pin core.GPIOPin) bool {
	// ReadPin is a convenience wrapper around GetPin that returns just the bool value
	value, _ := d.GetPin(pin)
	return value
}

// NewRPGPIODriver creates a new RP2040 GPIO driver
func NewRPGPIODriver() *RPGPIODriver {
	return &RPGPIODriver{
		configuredPins: make(map[core.GPIOPin]machine.Pin),
	}
}

// ConfigureOutput configures a pin as a digital output
func (d *RPGPIODriver) ConfigureOutput(pin core.GPIOPin) error {
	// Check if already configured
	if _, exists := d.configuredPins[pin]; exists {
		// Already configured, this is OK
		return nil
	}

	// Map pin to machine.Pin
	// RP2040 has GPIO0-GPIO29
	machinePin := d.pinNumberToMachinePin(pin)

	// Configure as output
	machinePin.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Track configured pin
	d.configuredPins[pin] = machinePin

	return nil
}

// SetPin sets the pin to high (true) or low (false)
func (d *RPGPIODriver) SetPin(pin core.GPIOPin, value bool) error {
	machinePin, exists := d.configuredPins[pin]
	if !exists {
		// Pin isn't configured - configure it first
		if err := d.ConfigureOutput(pin); err != nil {
			return err
		}
		machinePin = d.configuredPins[pin]
	}

	machinePin.Set(value)
	return nil
}

// GetPin reads the current pin state
func (d *RPGPIODriver) GetPin(pin core.GPIOPin) (bool, error) {
	machinePin, exists := d.configuredPins[pin]
	if !exists {
		// Pin not configured
		return false, nil
	}

	return machinePin.Get(), nil
}

// pinNumberToMachinePin converts a pin to a machine.Pin
// This mapping is RP2040-specific
func (d *RPGPIODriver) pinNumberToMachinePin(pin core.GPIOPin) machine.Pin {
	// For RP2040, pins map directly to GPIO numbers
	// GPIO0 = 0, GPIO1 = 1, etc.
	// TinyGo's machine package defines pins as constants
	// We use a simple offset calculation
	return machine.Pin(pin)
}
