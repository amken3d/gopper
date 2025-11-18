package core

// Stepper motor control implementation
// Inspired by Klipper's stepper.c with PIO optimization for RP2040/RP2350

import (
	"errors"
)

const (
	// Queue size for pending moves
	StepperQueueSize = 16

	// Step generation modes
	StepModeNormal = 0 // Normal stepping
	StepModeEdge   = 1 // Step on both edges (STEPPER_BOTH_EDGE)
)

// StepperMove represents a single queued move segment
type StepperMove struct {
	Interval  uint32 // Base step interval in timer ticks (12MHz)
	Count     uint16 // Number of steps in this move
	Add       int16  // Acceleration: added to interval each step
	Direction uint8  // Direction: 0=forward, 1=reverse
}

// Stepper represents a single stepper motor axis
type Stepper struct {
	// Configuration (from config_stepper command)
	OID             uint8  // Object ID
	StepPin         uint8  // Step pulse output pin
	DirPin          uint8  // Direction output pin
	InvertStep      bool   // Invert step signal polarity
	InvertDir       bool   // Invert direction signal polarity
	MinStopInterval uint32 // Minimum interval between steps (safety limit)

	// State
	Position int64 // Current position in steps (signed)
	NextDir  uint8 // Direction for next move

	// Move queue
	Queue     [StepperQueueSize]StepperMove
	QueueHead uint8 // Next move to execute
	QueueTail uint8 // Next slot to fill

	// Timer for next step event
	StepTimer Timer

	// Current move state
	CurrentInterval uint32 // Current interval (changes with acceleration)
	CurrentCount    uint16 // Steps remaining in current move
	CurrentAdd      int16  // Current acceleration value

	// Hardware backend
	Backend StepperBackend
}

// Global stepper registry
var (
	steppers     [16]*Stepper // Max 16 steppers
	stepperCount uint8

	// Backend factory function (set by platform-specific code)
	stepperBackendFactory func() StepperBackend
)

// GetStepper returns a stepper by OID
func GetStepper(oid uint8) *Stepper {
	if oid >= stepperCount {
		return nil
	}
	return steppers[oid]
}

// NewStepper creates a new stepper instance
func NewStepper(oid uint8, stepPin, dirPin uint8, invertStep bool, minStopInterval uint32) (*Stepper, error) {
	if oid >= 16 {
		return nil, errors.New("stepper OID exceeds maximum")
	}

	s := &Stepper{
		OID:             oid,
		StepPin:         stepPin,
		DirPin:          dirPin,
		InvertStep:      invertStep,
		MinStopInterval: minStopInterval,
		Position:        0,
		NextDir:         0,
		QueueHead:       0,
		QueueTail:       0,
	}

	// Initialize step timer
	s.StepTimer.Handler = s.stepperEventHandler

	// Create backend if factory is available
	if stepperBackendFactory != nil {
		backend := stepperBackendFactory()
		if backend != nil {
			err := s.InitBackend(backend)
			if err != nil {
				return nil, err
			}
		}
	}

	// Store in registry
	steppers[oid] = s
	if oid >= stepperCount {
		stepperCount = oid + 1
	}

	return s, nil
}

// SetStepperBackendFactory sets the factory function for creating stepper backends
// This should be called by platform-specific initialization code
func SetStepperBackendFactory(factory func() StepperBackend) {
	stepperBackendFactory = factory
}

// InitBackend initializes the hardware backend
func (s *Stepper) InitBackend(backend StepperBackend) error {
	s.Backend = backend
	return backend.Init(s.StepPin, s.DirPin, s.InvertStep, s.InvertDir)
}

// QueueMove adds a move to the queue
func (s *Stepper) QueueMove(interval uint32, count uint16, add int16) error {
	// Check for queue overflow
	nextTail := (s.QueueTail + 1) % StepperQueueSize
	if nextTail == s.QueueHead {
		return errors.New("queue overflow")
	}

	// Validate minimum interval
	if interval < s.MinStopInterval {
		interval = s.MinStopInterval
	}

	// Add to queue
	s.Queue[s.QueueTail] = StepperMove{
		Interval:  interval,
		Count:     count,
		Add:       add,
		Direction: s.NextDir,
	}
	s.QueueTail = nextTail

	// Start stepping if not already running
	if s.CurrentCount == 0 {
		s.loadNextMove()
	}

	return nil
}

// loadNextMove loads the next move from the queue
func (s *Stepper) loadNextMove() {
	// Check if queue is empty
	if s.QueueHead == s.QueueTail {
		s.CurrentCount = 0
		return
	}

	// Load move
	move := &s.Queue[s.QueueHead]
	s.CurrentInterval = move.Interval
	s.CurrentCount = move.Count
	s.CurrentAdd = move.Add

	// Set direction
	s.Backend.SetDirection(move.Direction != 0)

	// Update position based on direction
	// Position tracking happens after each step

	// Advance queue head
	s.QueueHead = (s.QueueHead + 1) % StepperQueueSize

	// Schedule first step
	s.StepTimer.WakeTime = GetTime() + s.CurrentInterval
	ScheduleTimer(&s.StepTimer)
}

// stepperEventHandler handles timer events for step generation
// This is the main stepping loop - called for each step
func (s *Stepper) stepperEventHandler(t *Timer) uint8 {
	// Generate step pulse
	s.Backend.Step()

	// Update position
	if s.Queue[(s.QueueHead+StepperQueueSize-1)%StepperQueueSize].Direction == 0 {
		s.Position++
	} else {
		s.Position--
	}

	// Decrement step count
	s.CurrentCount--

	// Apply acceleration
	if s.CurrentAdd != 0 {
		s.CurrentInterval += uint32(s.CurrentAdd)
		// Clamp to minimum interval
		if s.CurrentInterval < s.MinStopInterval {
			s.CurrentInterval = s.MinStopInterval
		}
	}

	// Check if move is complete
	if s.CurrentCount == 0 {
		s.loadNextMove()
		if s.CurrentCount == 0 {
			// No more moves
			return SF_DONE
		}
	}

	// Schedule next step
	t.WakeTime += s.CurrentInterval
	return SF_RESCHEDULE
}

// SetNextDir sets the direction for the next queued move
func (s *Stepper) SetNextDir(dir uint8) {
	s.NextDir = dir
}

// GetPosition returns the current position
func (s *Stepper) GetPosition() int64 {
	// If currently stepping, calculate position including in-progress move
	if s.CurrentCount > 0 {
		move := &s.Queue[(s.QueueHead+StepperQueueSize-1)%StepperQueueSize]
		stepsCompleted := int64(move.Count - s.CurrentCount)

		if move.Direction == 0 {
			return s.Position + stepsCompleted
		} else {
			return s.Position - stepsCompleted
		}
	}
	return s.Position
}

// ResetClock synchronizes the step clock (for Klipper coordination)
func (s *Stepper) ResetClock(clockTime uint32) {
	// This is used by Klipper to synchronize timing across multiple steppers
	// Adjust next wake time to align with the provided clock value
	if s.CurrentCount > 0 {
		s.StepTimer.WakeTime = clockTime
	}
}

// Stop immediately stops the stepper and clears the queue
func (s *Stepper) Stop() {
	s.CurrentCount = 0
	s.QueueHead = 0
	s.QueueTail = 0
	s.Backend.Stop()
}

// IsActive returns true if the stepper has pending moves
func (s *Stepper) IsActive() bool {
	return s.CurrentCount > 0 || s.QueueHead != s.QueueTail
}

// GetQueueCount returns the number of queued moves
func (s *Stepper) GetQueueCount() uint8 {
	if s.QueueTail >= s.QueueHead {
		return s.QueueTail - s.QueueHead
	}
	return StepperQueueSize - s.QueueHead + s.QueueTail
}
