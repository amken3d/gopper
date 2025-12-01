//go:build rp2350

package main

import (
	"gopper/core"
	"machine"
)

// StepperGPIO implements a simple GPIO-based stepper backend
// Uses direct GPIO toggling for step pulses - simpler than PIO
type StepperGPIO struct {
	stepPin    machine.Pin
	dirPin     machine.Pin
	invertStep bool
	invertDir  bool
	direction  bool
}

// NewStepperGPIO creates a new GPIO-based stepper backend
func NewStepperGPIO() *StepperGPIO {
	return &StepperGPIO{}
}

// Init initializes the stepper GPIO pins
func (s *StepperGPIO) Init(stepPin, dirPin uint8, invertStep, invertDir bool) error {
	s.stepPin = machine.Pin(stepPin)
	s.dirPin = machine.Pin(dirPin)
	s.invertStep = invertStep
	s.invertDir = invertDir

	// Configure pins as outputs
	s.stepPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	s.dirPin.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Set initial states (step low, direction forward)
	if s.invertStep {
		s.stepPin.High()
	} else {
		s.stepPin.Low()
	}
	s.SetDirection(false)

	core.DebugPrintln("[GPIO] Stepper initialized: step=" + itoa(int(stepPin)) + " dir=" + itoa(int(dirPin)))
	return nil
}

// Step generates a single step pulse using GPIO
func (s *StepperGPIO) Step() {
	// Generate step pulse
	// Pulse HIGH
	if s.invertStep {
		s.stepPin.Low()
	} else {
		s.stepPin.High()
	}

	// Brief delay for pulse width (~2-5µs is typical for stepper drivers)
	// Using a short busy loop for ~2µs at 150MHz
	for i := 0; i < 300; i++ {
		// Empty loop for delay
	}

	// Pulse LOW
	if s.invertStep {
		s.stepPin.High()
	} else {
		s.stepPin.Low()
	}
}

// SetDirection sets the direction output
func (s *StepperGPIO) SetDirection(dir bool) {
	s.direction = dir
	actualDir := dir
	if s.invertDir {
		actualDir = !actualDir
	}

	if actualDir {
		s.dirPin.High()
	} else {
		s.dirPin.Low()
	}
}

// Stop halts stepping (nothing to do for GPIO backend)
func (s *StepperGPIO) Stop() {
	// Ensure step pin is in idle state
	if s.invertStep {
		s.stepPin.High()
	} else {
		s.stepPin.Low()
	}
}

// GetName returns the backend name
func (s *StepperGPIO) GetName() string {
	return "GPIO"
}

// SetStepInterval is a no-op for GPIO backend
// Timing is handled by the core timer system, not the backend
func (s *StepperGPIO) SetStepInterval(intervalTicks uint32) {
	// GPIO backend doesn't need to adjust anything for interval changes
	// The core timer system handles all timing
}

// createGPIOBackend creates a GPIO-based stepper backend
func createGPIOBackend() core.StepperBackend {
	return NewStepperGPIO()
}

// InitGPIOSteppers initializes the stepper subsystem with GPIO backends
func InitGPIOSteppers() {
	// Register stepper commands
	core.RegisterStepperCommands()

	// Set backend factory to use GPIO instead of PIO
	core.SetStepperBackendFactory(createGPIOBackend)

	core.DebugPrintln("[GPIO] Stepper backend factory registered")
}
