package planner

import (
	"errors"
	"gopper/core"
	"gopper/standalone"
	"gopper/standalone/kinematics"
	"gopper/standalone/stepgen"
)

// Planner handles motion planning and execution
type Planner struct {
	config     *standalone.MachineConfig
	kinematics kinematics.Kinematics
	steppers   map[string]*stepgen.Stepper

	// Current state
	currentPos standalone.Position
	moveQueue  []*standalone.Move
	queueSize  int
	executing  bool
}

// NewPlanner creates a new motion planner
func NewPlanner(config *standalone.MachineConfig, kin kinematics.Kinematics) *Planner {
	return &Planner{
		config:     config,
		kinematics: kin,
		steppers:   make(map[string]*stepgen.Stepper),
		currentPos: standalone.Position{},
		moveQueue:  make([]*standalone.Move, 0, 32),
		queueSize:  0,
		executing:  false,
	}
}

// InitSteppers initializes stepper motors for all configured axes
func (p *Planner) InitSteppers(gpioDriver core.GPIODriver) error {
	axisNames := p.kinematics.GetAxisNames()

	for _, name := range axisNames {
		axisConfig, ok := p.config.Axes[name]
		if !ok {
			continue // Skip if axis not configured
		}

		stepper, err := stepgen.NewStepper(name, axisConfig)
		if err != nil {
			return err
		}

		err = stepper.InitPins(gpioDriver)
		if err != nil {
			return err
		}

		p.steppers[name] = stepper
	}

	return nil
}

// QueueMove adds a move to the queue
func (p *Planner) QueueMove(move *standalone.Move) error {
	// Check limits
	err := p.kinematics.CheckLimits(move.End)
	if err != nil {
		return err
	}

	// Calculate trapezoidal profile
	p.calculateTrapezoid(move)

	// Add to queue
	p.moveQueue = append(p.moveQueue, move)
	p.queueSize++

	// Start execution if not already running
	if !p.executing {
		p.executeNextMove()
	}

	return nil
}

// calculateTrapezoid calculates the trapezoidal velocity profile for a move
func (p *Planner) calculateTrapezoid(move *standalone.Move) {
	// Limit velocity to axis maximums
	maxVel := move.Velocity
	dx := abs(move.End.X - move.Start.X)
	dy := abs(move.End.Y - move.Start.Y)
	dz := abs(move.End.Z - move.Start.Z)

	if dx > 0 {
		axisVel := maxVel * dx / move.Distance
		if axisConfig, ok := p.config.Axes["x"]; ok {
			if axisVel > axisConfig.MaxVelocity {
				maxVel = axisConfig.MaxVelocity * move.Distance / dx
			}
		}
	}
	if dy > 0 {
		axisVel := maxVel * dy / move.Distance
		if axisConfig, ok := p.config.Axes["y"]; ok {
			if axisVel > axisConfig.MaxVelocity {
				maxVel = axisConfig.MaxVelocity * move.Distance / dy
			}
		}
	}
	if dz > 0 {
		axisVel := maxVel * dz / move.Distance
		if axisConfig, ok := p.config.Axes["z"]; ok {
			if axisVel > axisConfig.MaxVelocity {
				maxVel = axisConfig.MaxVelocity * move.Distance / dz
			}
		}
	}

	move.Velocity = maxVel

	// Calculate acceleration/deceleration times
	// Using simplified trapezoidal profile (no lookahead for now)
	accelDist := (maxVel * maxVel) / (2.0 * move.Accel)

	if accelDist * 2.0 >= move.Distance {
		// Triangle profile (can't reach full speed)
		accelDist = move.Distance / 2.0
		move.CruiseVel = sqrt(move.Accel * accelDist)
		move.StartVel = 0
		move.EndVel = 0

		accelTime := move.CruiseVel / move.Accel
		move.AccelTicks = secondsToTicks(accelTime)
		move.CruiseTicks = 0
		move.DecelTicks = move.AccelTicks
		move.Duration = move.AccelTicks + move.DecelTicks
	} else {
		// Trapezoidal profile
		cruiseDist := move.Distance - 2.0*accelDist
		move.CruiseVel = maxVel
		move.StartVel = 0
		move.EndVel = 0

		accelTime := maxVel / move.Accel
		cruiseTime := cruiseDist / maxVel
		decelTime := accelTime

		move.AccelTicks = secondsToTicks(accelTime)
		move.CruiseTicks = secondsToTicks(cruiseTime)
		move.DecelTicks = secondsToTicks(decelTime)
		move.Duration = move.AccelTicks + move.CruiseTicks + move.DecelTicks
	}
}

// executeNextMove starts executing the next move in the queue
func (p *Planner) executeNextMove() {
	if p.queueSize == 0 {
		p.executing = false
		return
	}

	// Get next move
	move := p.moveQueue[0]
	p.moveQueue = p.moveQueue[1:]
	p.queueSize--

	p.executing = true

	// Calculate axis positions
	endPositions, err := p.kinematics.CalcPosition(move.End)
	if err != nil {
		// Error - skip this move
		p.executeNextMove()
		return
	}

	// Command each stepper
	axisNames := p.kinematics.GetAxisNames()
	for i, name := range axisNames {
		if i >= len(endPositions) {
			break
		}

		stepper, ok := p.steppers[name]
		if !ok {
			continue
		}

		// For now, use constant velocity (simplified)
		// TODO: Implement proper acceleration profiles
		stepper.MoveTo(endPositions[i], move.CruiseVel, move.Accel)
	}

	// Update current position
	p.currentPos = move.End

	// Schedule completion check
	completionTimer := &core.Timer{
		WakeTime: core.GetSystemTime() + move.Duration,
		Handler: func(t *core.Timer) uint8 {
			p.executeNextMove()
			return core.SF_DONE
		},
	}
	core.ScheduleTimer(completionTimer)
}

// GetCurrentPosition returns the current position
func (p *Planner) GetCurrentPosition() standalone.Position {
	// In real implementation, read from steppers
	return p.currentPos
}

// SetPosition sets the current position
func (p *Planner) SetPosition(pos standalone.Position) {
	p.currentPos = pos

	// Update stepper positions
	positions, err := p.kinematics.CalcPosition(pos)
	if err != nil {
		return
	}

	axisNames := p.kinematics.GetAxisNames()
	for i, name := range axisNames {
		if i >= len(positions) {
			break
		}

		stepper, ok := p.steppers[name]
		if !ok {
			continue
		}

		stepper.SetPosition(positions[i])
	}
}

// ClearQueue clears the move queue and stops all motion
func (p *Planner) ClearQueue() {
	p.moveQueue = make([]*standalone.Move, 0, 32)
	p.queueSize = 0
	p.executing = false

	// Stop all steppers
	for _, stepper := range p.steppers {
		stepper.Stop()
	}
}

// IsIdle returns true if no moves are queued or executing
func (p *Planner) IsIdle() bool {
	return !p.executing && p.queueSize == 0
}

// WaitIdle blocks until all moves are complete
func (p *Planner) WaitIdle() error {
	// In embedded context, we can't block
	// Caller should check IsIdle() periodically
	return errors.New("WaitIdle not supported in embedded mode")
}

// Helper functions

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

func secondsToTicks(seconds float64) uint32 {
	return uint32(seconds * float64(core.GetTimerFrequency()))
}
