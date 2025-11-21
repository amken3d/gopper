//go:build rp2040

package pio

import (
	"gopper/core"
)

// StepperBackendMode selects which backend to use for steppers
type StepperBackendMode int

const (
	// StepperBackendAuto automatically selects best available backend
	StepperBackendAuto StepperBackendMode = iota
	// StepperBackendPIO uses PIO-based step generation (RP2040/RP2350 only)
	StepperBackendPIO
	// StepperBackendGPIO uses GPIO-based step generation (universal fallback)
	StepperBackendGPIO
)

var (
	// Current backend mode
	stepperBackendMode = StepperBackendPIO // Default to PIO for best performance

	// PIO allocation tracking
	// RP2040 has 2 PIO blocks (PIO0, PIO1) with 4 state machines each
	pioAllocations = [2][4]bool{} // [pioNum][smNum]
	nextPIONum     = uint8(0)
	nextSMNum      = uint8(0)
)

// InitSteppers initializes the stepper subsystem
func InitSteppers() {
	// Register stepper commands
	core.RegisterStepperCommands()

	// Set backend factory function
	// This is called by config_stepper command when a stepper is created
	core.SetStepperBackendFactory(createStepperBackend)
}

// createStepperBackend creates a stepper backend based on current mode
func createStepperBackend() core.StepperBackend {
	switch stepperBackendMode {
	case StepperBackendPIO:
		return createPIOBackend()
	case StepperBackendGPIO:
		return NewGPIOStepperBackend()
	case StepperBackendAuto:
		// Try PIO first, fall back to GPIO if PIO is exhausted
		backend := createPIOBackend()
		if backend != nil {
			return backend
		}
		return NewGPIOStepperBackend()
	default:
		return NewGPIOStepperBackend()
	}
}

// createPIOBackend creates a PIO-based stepper backend
// Returns nil if no PIO resources available
func createPIOBackend() core.StepperBackend {
	// Find available PIO state machine
	pioNum, smNum, ok := allocatePIO()
	if !ok {
		// No PIO available, return nil to fall back to GPIO
		return nil
	}

	return NewPIOStepperBackend(pioNum, smNum)
}

// allocatePIO allocates a PIO state machine
// Returns (pioNum, smNum, ok)
func allocatePIO() (uint8, uint8, bool) {
	// Round-robin allocation across PIO blocks and state machines
	for i := 0; i < 8; i++ { // 2 PIO × 4 SM = 8 total
		pioNum := nextPIONum
		smNum := nextSMNum

		// Advance to next slot
		nextSMNum++
		if nextSMNum >= 4 {
			nextSMNum = 0
			nextPIONum = (nextPIONum + 1) % 2
		}

		// Check if this slot is free
		if !pioAllocations[pioNum][smNum] {
			pioAllocations[pioNum][smNum] = true
			return pioNum, smNum, true
		}
	}

	// All PIO resources exhausted
	return 0, 0, false
}

// SetStepperBackendMode sets the backend mode for future steppers
// Must be called before creating steppers
func SetStepperBackendMode(mode StepperBackendMode) {
	stepperBackendMode = mode
}

// GetPIOAllocationStatus returns PIO allocation status for debugging
func GetPIOAllocationStatus() [2][4]bool {
	return pioAllocations
}
