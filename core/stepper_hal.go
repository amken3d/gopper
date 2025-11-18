package core

// StepperBackend defines the hardware abstraction for stepper control
// Implementations can use GPIO, PIO, or other methods
type StepperBackend interface {
	// Init initializes the stepper hardware
	// stepPin: GPIO pin for step pulses
	// dirPin: GPIO pin for direction signal
	// invertStep: invert step pin polarity
	// invertDir: invert direction pin polarity
	Init(stepPin, dirPin uint8, invertStep, invertDir bool) error

	// Step generates a single step pulse
	// Must handle pulse width timing internally
	// Should be fast (called from timer interrupt)
	Step()

	// SetDirection sets the direction output
	// dir: true = reverse, false = forward
	// Must ensure proper dir-to-step setup time
	SetDirection(dir bool)

	// Stop immediately halts stepping
	Stop()

	// GetName returns backend implementation name
	GetName() string
}

// StepperBackendInfo provides information about available backends
type StepperBackendInfo struct {
	Name          string
	MaxStepRate   uint32 // Maximum steps/second per axis
	MinPulseNs    uint32 // Minimum step pulse width (ns)
	TypicalJitter uint32 // Typical timing jitter (ns)
	CPUOverhead   uint8  // CPU overhead percentage (0-100)
}
