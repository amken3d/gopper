//go:build rp2040 || rp2350

package pio

import (
	"device/arm"
	"device/rp"
	"gopper/core"
	"machine"
)

// GPIOStepperBackend implements stepper control using direct GPIO
// This is the baseline/fallback implementation
// Performance: ~200kHz max step rate, ~200ns pulse width
type GPIOStepperBackend struct {
	stepPin    machine.Pin
	dirPin     machine.Pin
	invertStep bool
	invertDir  bool

	// Cached register values for fast access
	stepSetMask   uint32
	stepClearMask uint32
	dirSetMask    uint32
	dirClearMask  uint32
}

// NewGPIOStepperBackend creates a new GPIO-based stepper backend
func NewGPIOStepperBackend() *GPIOStepperBackend {
	return &GPIOStepperBackend{}
}

// Init initializes the GPIO stepper backend
func (b *GPIOStepperBackend) Init(stepPin, dirPin uint8, invertStep, invertDir bool) error {
	b.stepPin = machine.Pin(stepPin)
	b.dirPin = machine.Pin(dirPin)
	b.invertStep = invertStep
	b.invertDir = invertDir

	// Configure step pin as output
	b.stepPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	b.stepPin.Low()

	// Configure direction pin as output
	b.dirPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	b.dirPin.Low()

	// Pre-calculate register masks for fast GPIO access
	// Using SIO (Single-cycle I/O) for fastest possible toggling
	b.stepSetMask = 1 << stepPin
	b.stepClearMask = 1 << stepPin
	b.dirSetMask = 1 << dirPin
	b.dirClearMask = 1 << dirPin

	// Apply inversion if needed
	if invertStep {
		b.stepSetMask, b.stepClearMask = b.stepClearMask, b.stepSetMask
	}
	if invertDir {
		b.dirSetMask, b.dirClearMask = b.dirClearMask, b.dirSetMask
	}

	return nil
}

// Step generates a single step pulse
// Optimized for minimum pulse width and CPU cycles
// Pulse width: ~200ns @ 125MHz (25 cycles)
func (b *GPIOStepperBackend) Step() {
	// Step HIGH
	rp.SIO.GPIO_OUT_SET.Set(b.stepSetMask)

	// Pulse width delay
	// Each NOP is ~8ns @ 125MHz
	// Target: 100ns minimum for Trinamic drivers
	// 13 NOPs = ~104ns
	arm.Asm("nop\nnop\nnop\nnop\nnop\nnop\nnop\nnop\nnop\nnop\nnop\nnop\nnop")

	// Step LOW
	rp.SIO.GPIO_OUT_CLR.Set(b.stepClearMask)
}

// StepBothEdge generates a step pulse optimized for both-edge stepping
// Used when STEPPER_BOTH_EDGE mode is enabled
// Toggles the pin instead of explicit set/clear
func (b *GPIOStepperBackend) StepBothEdge() {
	// Toggle step pin
	rp.SIO.GPIO_OUT_XOR.Set(b.stepSetMask)
}

// SetDirection sets the direction output
// Ensures proper dir-to-step setup time (20ns minimum for TMC drivers)
func (b *GPIOStepperBackend) SetDirection(dir bool) {
	if dir {
		// Reverse direction
		rp.SIO.GPIO_OUT_SET.Set(b.dirSetMask)
	} else {
		// Forward direction
		rp.SIO.GPIO_OUT_CLR.Set(b.dirClearMask)
	}

	// Dir-to-step setup time: 20ns minimum for TMC2209
	// Add a few NOPs to ensure timing
	// 3 NOPs = ~24ns @ 125MHz
	arm.Asm("nop\nnop\nnop")
}

// Stop immediately halts stepping
func (b *GPIOStepperBackend) Stop() {
	// Ensure step pin is low
	rp.SIO.GPIO_OUT_CLR.Set(b.stepClearMask)
}

// GetName returns the backend name
func (b *GPIOStepperBackend) GetName() string {
	return "GPIO"
}

// GetInfo returns backend performance information
func (b *GPIOStepperBackend) GetInfo() core.StepperBackendInfo {
	return core.StepperBackendInfo{
		Name:          "GPIO",
		MaxStepRate:   200000, // 200 kHz
		MinPulseNs:    200,    // 200ns pulse width
		TypicalJitter: 500,    // ~500ns jitter (interrupt-based)
		CPUOverhead:   15,     // ~15% CPU at max rate (4 axes)
	}
}

// FastGPIOSet is an optimized GPIO set function using direct register access
func FastGPIOSet(pin uint8, high bool) {
	mask := uint32(1) << pin
	if high {
		rp.SIO.GPIO_OUT_SET.Set(mask)
	} else {
		rp.SIO.GPIO_OUT_CLR.Set(mask)
	}
}

// FastGPIOToggle is an optimized GPIO toggle function
func FastGPIOToggle(pin uint8) {
	mask := uint32(1) << pin
	rp.SIO.GPIO_OUT_XOR.Set(mask)
}
