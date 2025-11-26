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

// PositionMover is an optional interface for backends that support position-based moves
// This enables hardware ramp generation (e.g., TMC5240, TMC5160)
type PositionMover interface {
	// MoveToPosition commands the backend to move to an absolute position
	// targetPos: Absolute target position in steps
	// startVel: Starting velocity (timer ticks between steps)
	// endVel: Ending velocity (timer ticks between steps)
	// accel: Acceleration value (change in interval per step)
	MoveToPosition(targetPos int64, startVel, endVel, accel uint32) error

	// GetHardwarePosition reads the current position from hardware
	// For drivers with position registers (TMC5240 XACTUAL)
	GetHardwarePosition() (int64, error)

	// IsMoving returns true if hardware is currently executing a move
	IsMoving() bool

	// GetMoveStatus returns detailed status for debugging
	// Returns: currentPos, targetPos, currentVel, statusFlags
	GetMoveStatus() (int64, int64, uint32, uint32)
}
