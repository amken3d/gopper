//go:build rp2040 || rp2350

package pio

import (
	"gopper/core"
)

var (
	// PIO allocation tracking
	// RP2040/RP2350 has 2 PIO blocks (PIO0, PIO1) with 4 state machines each
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
	core.SetStepperBackendFactory(createPIOBackend)
}

// createPIOBackend creates a PIO-based stepper backend
// Returns nil if no PIO resources available
func createPIOBackend() core.StepperBackend {
	// Find available PIO state machine
	pioNum, smNum, ok := allocatePIO()
	if !ok {
		// No PIO available (max 8 steppers supported)
		return nil
	}

	return NewStepperPIO(pioNum, smNum)
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

// GetPIOAllocationStatus returns PIO allocation status for debugging
func GetPIOAllocationStatus() [2][4]bool {
	return pioAllocations
}

// ResetPIOAllocations resets all PIO allocations (for testing)
func ResetPIOAllocations() {
	pioAllocations = [2][4]bool{}
	nextPIONum = 0
	nextSMNum = 0
}
