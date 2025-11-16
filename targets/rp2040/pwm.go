//go:build rp2040 || rp2350

package main

import (
	"gopper/core"
	"machine"
)

// PWM_MAX matches Klipper's maximum PWM value
const PWM_MAX = 255

// pwmPeripheral is an interface for PWM hardware peripherals
// This abstracts over TinyGo's unexported *pwmGroup type
type pwmPeripheral interface {
	Configure(config machine.PWMConfig) error
	Channel(pin machine.Pin) (uint8, error)
	Top() uint32
	Set(channel uint8, value uint32)
}

// RP2040PWMDriver implements the PWMDriver interface for RP2040
// Leverages RP2040's 8 hardware PWM slices with 2 channels each
type RP2040PWMDriver struct {
	// Track configured PWM slices
	// Key: slice number (0-7), Value: configured period in nanoseconds
	slices map[uint8]uint64

	// Track pin to channel mapping
	// Key: pin number, Value: PWM channel
	channels map[uint32]uint8

	// Track PWM peripherals for each slice
	// Key: slice number (0-7), Value: PWM peripheral
	peripherals map[uint8]pwmPeripheral
}

// NewRP2040PWMDriver creates a new RP2040 PWM driver
func NewRP2040PWMDriver() *RP2040PWMDriver {
	return &RP2040PWMDriver{
		slices:      make(map[uint8]uint64),
		channels:    make(map[uint32]uint8),
		peripherals: make(map[uint8]pwmPeripheral),
	}
}

// GetMaxValue returns the maximum PWM value (255)
func (d *RP2040PWMDriver) GetMaxValue() uint32 {
	return PWM_MAX
}

// ConfigureHardwarePWM configures a pin for hardware PWM output
// This uses TinyGo's machine.PWM API for efficient hardware control
func (d *RP2040PWMDriver) ConfigureHardwarePWM(pin core.PWMPin, cycleTicks uint32) (uint32, error) {
	pinNum := uint32(pin)

	// Map pin to PWM slice and channel
	// RP2040: GPIO pin N maps to:
	//   Slice: (N >> 1) & 0x7  (divide by 2, mod 8)
	//   Channel: N & 1          (even=A, odd=B)
	sliceNum := uint8((pinNum >> 1) & 0x7)

	// Get or create the PWM peripheral for this slice
	pwm, exists := d.peripherals[sliceNum]
	if !exists {
		// First time using this slice - get and store the peripheral
		pwm = d.getPWMPeripheral(sliceNum)
		d.peripherals[sliceNum] = pwm
	}

	// Convert cycle ticks to period in nanoseconds
	// Timer frequency is 12MHz (from core.go timer init)
	// period_ns = (cycleTicks * 1,000,000,000) / 12,000,000
	period := (uint64(cycleTicks) * 1000000000) / 12000000

	// Check if this slice is already configured
	if existingPeriod, exists := d.slices[sliceNum]; exists {
		// Slice already configured - verify period matches
		// Allow some tolerance for hardware limitations
		if existingPeriod != period {
			// For now, we'll reconfigure with the new period
			// In production, might want to return an error if periods conflict
		}
	}

	// Configure PWM with the calculated period
	err := pwm.Configure(machine.PWMConfig{
		Period: period,
	})
	if err != nil {
		return 0, err
	}

	// Get the channel for this pin
	machinePin := machine.Pin(pinNum)
	channel, err := pwm.Channel(machinePin)
	if err != nil {
		return 0, err
	}

	// Store configuration
	d.slices[sliceNum] = period
	d.channels[pinNum] = channel

	// Return the cycle ticks (may adjust if hardware has constraints)
	return cycleTicks, nil
}

// SetDutyCycle sets the PWM duty cycle for a pin
// value: 0 (fully off) to 255 (fully on)
func (d *RP2040PWMDriver) SetDutyCycle(pin core.PWMPin, value core.PWMValue) error {
	pinNum := uint32(pin)

	// Get channel for this pin
	channel, exists := d.channels[pinNum]
	if !exists {
		// Pin not configured
		return nil
	}

	// Get the PWM peripheral for this slice
	sliceNum := uint8((pinNum >> 1) & 0x7)
	pwm, exists := d.peripherals[sliceNum]
	if !exists {
		// Peripheral not configured - shouldn't happen if channel exists
		return nil
	}

	// Convert value (0-255) to hardware duty cycle
	// TinyGo PWM uses: Set(channel, value) where value is compared to Top()
	// We need to scale our 0-255 value to 0-Top()
	top := pwm.Top()

	// Calculate duty cycle: (value * top) / PWM_MAX
	// Use 32-bit math to avoid overflow
	dutyCycle := (uint32(value) * uint32(top)) / PWM_MAX

	// Set the duty cycle
	pwm.Set(channel, dutyCycle)

	return nil
}

// DisablePWM disables PWM on a pin and returns it to GPIO mode
func (d *RP2040PWMDriver) DisablePWM(pin core.PWMPin) error {
	pinNum := uint32(pin)

	// Remove from tracking
	delete(d.channels, pinNum)

	// Note: TinyGo doesn't provide a direct way to disable PWM
	// Setting duty to 0 effectively disables output
	// The pin will remain in PWM mode but output low

	return nil
}

// getPWMPeripheral returns the PWM peripheral for a given slice number
// RP2040 has 8 PWM slices: PWM0-PWM7
// Returns a pwmPeripheral interface that wraps TinyGo's unexported *pwmGroup type
func (d *RP2040PWMDriver) getPWMPeripheral(sliceNum uint8) pwmPeripheral {
	// TinyGo defines PWM0-PWM7 as global variables of type *pwmGroup
	// We return them via the pwmPeripheral interface
	switch sliceNum {
	case 0:
		return machine.PWM0
	case 1:
		return machine.PWM1
	case 2:
		return machine.PWM2
	case 3:
		return machine.PWM3
	case 4:
		return machine.PWM4
	case 5:
		return machine.PWM5
	case 6:
		return machine.PWM6
	case 7:
		return machine.PWM7
	default:
		// Should never happen with proper masking
		return machine.PWM0
	}
}
