package stepgen

import (
	"gopper/core"
	"gopper/standalone"
)

// Stepper represents a single stepper motor
type Stepper struct {
	name   string
	config standalone.AxisConfig

	// GPIO driver interface
	stepPin core.GPIOPin
	dirPin  core.GPIOPin
	enPin   core.GPIOPin

	// Current state
	position     int64   // Current position in steps
	targetPos    int64   // Target position in steps
	velocity     float64 // Current velocity (steps/s)
	acceleration float64 // Current acceleration (steps/s^2)

	// Step generation
	nextStepTime uint32      // Time for next step
	stepInterval uint32      // Interval between steps (ticks)
	stepTimer    *core.Timer // Timer for step generation
	active       bool        // Is stepper currently moving
}

// NewStepper creates a new stepper motor controller
func NewStepper(name string, config standalone.AxisConfig) (*Stepper, error) {
	stepper := &Stepper{
		name:     name,
		config:   config,
		position: 0,
		active:   false,
	}

	// Initialize timer
	stepper.stepTimer = &core.Timer{
		WakeTime: 0,
		Handler:  stepper.stepHandler,
		Next:     nil,
	}

	return stepper, nil
}

// InitPins initializes the GPIO pins for this stepper
func (s *Stepper) InitPins(gpioDriver core.GPIODriver) error {
	// Get step pin
	stepPinNum, err := core.LookupPin(s.config.StepPin)
	if err != nil {
		return err
	}
	s.stepPin = gpioDriver.GetPin(uint8(stepPinNum))
	s.stepPin.SetMode(core.PinModeOutput)

	// Get direction pin
	dirPinNum, err := core.LookupPin(s.config.DirPin)
	if err != nil {
		return err
	}
	s.dirPin = gpioDriver.GetPin(uint8(dirPinNum))
	s.dirPin.SetMode(core.PinModeOutput)

	// Get enable pin (optional)
	if s.config.EnablePin != "" {
		enPinNum, err := core.LookupPin(s.config.EnablePin)
		if err != nil {
			return err
		}
		s.enPin = gpioDriver.GetPin(uint8(enPinNum))
		s.enPin.SetMode(core.PinModeOutput)

		// Disable motor initially
		if s.config.InvertEnable {
			s.enPin.SetValue(1)
		} else {
			s.enPin.SetValue(0)
		}
	}

	return nil
}

// Enable enables the stepper motor
func (s *Stepper) Enable() {
	if s.enPin != nil {
		if s.config.InvertEnable {
			s.enPin.SetValue(0)
		} else {
			s.enPin.SetValue(1)
		}
	}
}

// Disable disables the stepper motor
func (s *Stepper) Disable() {
	if s.enPin != nil {
		if s.config.InvertEnable {
			s.enPin.SetValue(1)
		} else {
			s.enPin.SetValue(0)
		}
	}
}

// MoveTo schedules a move to the target position
func (s *Stepper) MoveTo(targetMM float64, velocity float64, accel float64) {
	// Convert mm to steps
	s.targetPos = int64(targetMM * s.config.StepsPerMM)

	// Calculate direction
	direction := int64(1)
	if s.targetPos < s.position {
		direction = -1
	}

	// Set direction pin
	dirValue := uint8(0)
	if direction > 0 {
		dirValue = 1
	}
	if s.config.InvertDir {
		dirValue = 1 - dirValue
	}
	s.dirPin.SetValue(dirValue)

	// Calculate step interval (simplified - constant velocity for now)
	stepsPerSecond := velocity * s.config.StepsPerMM
	if stepsPerSecond > 0 {
		s.stepInterval = uint32(float64(core.GetTimerFrequency()) / stepsPerSecond)
	} else {
		s.stepInterval = 1000000 // Very slow if velocity is 0
	}

	// Enable motor
	s.Enable()

	// Schedule first step
	if s.position != s.targetPos {
		s.active = true
		s.nextStepTime = core.GetSystemTime() + s.stepInterval
		s.stepTimer.WakeTime = s.nextStepTime
		core.ScheduleTimer(s.stepTimer)
	}
}

// stepHandler is called by the scheduler to generate step pulses
func (s *Stepper) stepHandler(timer *core.Timer) uint8 {
	if !s.active || s.position == s.targetPos {
		s.active = false
		return core.SF_DONE
	}

	// Generate step pulse
	s.stepPin.SetValue(1)

	// Update position
	if s.targetPos > s.position {
		s.position++
	} else {
		s.position--
	}

	// Schedule step-down (pulse width ~2us)
	timer.WakeTime = core.GetSystemTime() + core.UsToTicks(2)
	timer.Handler = s.stepDownHandler
	return core.SF_RESCHEDULE
}

// stepDownHandler turns off the step pulse
func (s *Stepper) stepDownHandler(timer *core.Timer) uint8 {
	// End step pulse
	s.stepPin.SetValue(0)

	// Check if we've reached target
	if s.position == s.targetPos {
		s.active = false
		return core.SF_DONE
	}

	// Schedule next step
	s.nextStepTime += s.stepInterval
	timer.WakeTime = s.nextStepTime
	timer.Handler = s.stepHandler
	return core.SF_RESCHEDULE
}

// GetPosition returns the current position in millimeters
func (s *Stepper) GetPosition() float64 {
	return float64(s.position) / s.config.StepsPerMM
}

// SetPosition sets the current position (for homing, etc.)
func (s *Stepper) SetPosition(posMM float64) {
	s.position = int64(posMM * s.config.StepsPerMM)
	s.targetPos = s.position
}

// IsActive returns whether the stepper is currently moving
func (s *Stepper) IsActive() bool {
	return s.active
}

// Stop immediately stops the stepper
func (s *Stepper) Stop() {
	s.active = false
	s.targetPos = s.position
}
