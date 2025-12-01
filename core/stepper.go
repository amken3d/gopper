package core

// Stepper motor control implementation
// Inspired by Klipper's stepper.c with PIO optimization for RP2040/RP2350

import (
	"errors"
)

const (
	// Queue size for pending moves
	// Klipper typically sends bursts of moves, especially during direction changes
	// STEPPER_BUZZ can send 50+ moves for 10 oscillations with direction changes
	// 64 provides enough headroom for typical motion patterns including buzzing
	StepperQueueSize = 64

	// Step generation modes
	StepModeNormal = 0 // Normal stepping
	StepModeEdge   = 1 // Step on both edges (STEPPER_BOTH_EDGE)
)

// StepperMove represents a single queued move segment
type StepperMove struct {
	Interval  uint32 // Base step interval in CLOCK_FREQ ticks (1MHz for RP2350)
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

	// Clock synchronization (from reset_step_clock)
	NextStepClock uint32 // Clock time for next step (set by reset_step_clock)
	ClockSet      bool   // True if NextStepClock has been set
	LastStepTime  uint32 // Time of last step (for interval calculations)

	// Hardware backend
	Backend StepperBackend
}

// Global stepper registry
var (
	steppers     [16]*Stepper // Max 16 steppers
	stepperCount uint8

	// Backend factory function (set by platform-specific code)
	stepperBackendFactory func() StepperBackend

	// Debug: step counter for diagnostics
	totalStepCount uint32
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
	DebugPrintln("[STEPPER] NewStepper: oid=" + itoa(int(oid)) + " stepPin=" + itoa(int(stepPin)) + " dirPin=" + itoa(int(dirPin)))

	if oid >= 16 {
		DebugPrintln("[STEPPER] ERROR: OID exceeds maximum")
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
	DebugPrintln("[STEPPER] Checking for backend factory...")
	if stepperBackendFactory != nil {
		DebugPrintln("[STEPPER] Backend factory exists, creating backend...")
		backend := stepperBackendFactory()
		if backend != nil {
			DebugPrintln("[STEPPER] Backend created: " + backend.GetName())
			err := s.InitBackend(backend)
			if err != nil {
				DebugPrintln("[STEPPER] ERROR: InitBackend failed: " + err.Error())
				return nil, err
			}
			DebugPrintln("[STEPPER] Backend initialized successfully")
		} else {
			DebugPrintln("[STEPPER] WARNING: Backend factory returned nil")
		}
	} else {
		DebugPrintln("[STEPPER] WARNING: No backend factory set!")
	}

	// Store in registry
	steppers[oid] = s
	if oid >= stepperCount {
		stepperCount = oid + 1
	}

	DebugPrintln("[STEPPER] NewStepper complete")
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

	// Configure backend timing for this step rate
	s.Backend.SetStepInterval(s.CurrentInterval)

	// Advance queue head
	s.QueueHead = (s.QueueHead + 1) % StepperQueueSize

	// Get current time for timing capture
	currentTime := GetTime()

	// First update LastStepTime if reset_step_clock was called
	if s.ClockSet {
		s.LastStepTime = s.NextStepClock
		s.ClockSet = false
	}

	// Always compute WakeTime as LastStepTime + CurrentInterval
	s.StepTimer.WakeTime = s.LastStepTime + s.CurrentInterval

	// Record timing event (fast, non-blocking)
	RecordTiming(EvtLoadMove, s.OID, currentTime, s.StepTimer.WakeTime, s.CurrentInterval)

	ScheduleTimer(&s.StepTimer)
}

// stepperEventHandler handles timer events for step generation
// This is the main stepping loop - called for each step
func (s *Stepper) stepperEventHandler(t *Timer) uint8 {
	// Record timer fire event FIRST to capture timing
	startTime := GetTime()
	RecordTiming(EvtTimerFire, s.OID, startTime, t.WakeTime, uint32(s.CurrentCount))

	// Update LastStepTime FIRST (before loadNextMove might use it)
	s.LastStepTime = t.WakeTime

	// Generate step pulse
	s.Backend.Step()

	// Track total steps for diagnostics
	totalStepCount++

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

	// Check if move is complete - return directly from loadNextMove
	// (like Klipper's stepper.c does with stepper_load_next)
	if s.CurrentCount == 0 {
		return s.loadNextMoveFromHandler(t)
	}

	// Schedule next step (continuing current move)
	t.WakeTime += s.CurrentInterval
	return SF_RESCHEDULE
}

// loadNextMoveFromHandler loads the next move when called from step handler
// Returns SF_DONE if no more moves, SF_RESCHEDULE otherwise
// Unlike loadNextMove, this doesn't call ScheduleTimer - the timer system handles it
func (s *Stepper) loadNextMoveFromHandler(t *Timer) uint8 {
	// Check if queue is empty
	if s.QueueHead == s.QueueTail {
		s.CurrentCount = 0
		return SF_DONE
	}

	// Load move
	move := &s.Queue[s.QueueHead]
	s.CurrentInterval = move.Interval
	s.CurrentCount = move.Count
	s.CurrentAdd = move.Add

	// Set direction
	s.Backend.SetDirection(move.Direction != 0)

	// Configure backend timing for this step rate
	s.Backend.SetStepInterval(s.CurrentInterval)

	// Advance queue head
	s.QueueHead = (s.QueueHead + 1) % StepperQueueSize

	// Calculate next step time using LastStepTime (which was just updated)
	t.WakeTime = s.LastStepTime + s.CurrentInterval

	// Record timing event (fast, non-blocking)
	RecordTiming(EvtLoadMove, s.OID, GetTime(), t.WakeTime, s.CurrentInterval)

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
	currentTime := GetTime()

	// Record timing event (fast, non-blocking)
	RecordTiming(EvtResetClock, s.OID, currentTime, clockTime, 0)

	// Store the clock time for the next move
	// This is called BEFORE queue_step, so we save it for loadNextMove to use
	s.NextStepClock = clockTime
	s.ClockSet = true
	s.LastStepTime = clockTime // Also update LastStepTime for interval calculations

	// If already stepping, update the wake time directly
	if s.CurrentCount > 0 {
		s.StepTimer.WakeTime = clockTime
		// Re-schedule the timer in case it was removed (e.g., after shutdown)
		ScheduleTimer(&s.StepTimer)
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

// GetTotalStepCount returns the total number of steps executed (for diagnostics)
func GetTotalStepCount() uint32 {
	return totalStepCount
}

// ResetTotalStepCount resets the step counter
func ResetTotalStepCount() {
	totalStepCount = 0
}
