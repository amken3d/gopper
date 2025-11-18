package gcode

import (
	"gopper/standalone"
)

// Interpreter executes G-code commands
type Interpreter struct {
	state    *standalone.MachineState
	config   *standalone.MachineConfig
	planner  Planner // Interface to motion planner
}

// Planner interface for motion planning
type Planner interface {
	QueueMove(move *standalone.Move) error
	GetCurrentPosition() standalone.Position
	SetPosition(pos standalone.Position)
	ClearQueue()
}

// NewInterpreter creates a new G-code interpreter
func NewInterpreter(config *standalone.MachineConfig, planner Planner) *Interpreter {
	return &Interpreter{
		state: &standalone.MachineState{
			Position:     standalone.Position{},
			Homed:        [4]bool{false, false, false, false},
			AbsoluteMode: true,
			FeedRate:     config.DefaultVelocity,
			ExtrudeMode:  false, // Relative extrusion by default
			Temperature:  make(map[string]float64),
			TargetTemp:   make(map[string]float64),
		},
		config:  config,
		planner: planner,
	}
}

// Execute executes a parsed G-code command
func (interp *Interpreter) Execute(cmd *standalone.GCodeCommand) error {
	if cmd == nil {
		return nil
	}

	switch cmd.Type {
	case 'G':
		return interp.executeG(cmd)
	case 'M':
		return interp.executeM(cmd)
	case 'T':
		return interp.executeT(cmd)
	}

	return nil
}

// executeG handles G-codes
func (interp *Interpreter) executeG(cmd *standalone.GCodeCommand) error {
	switch cmd.Number {
	case 0, 1: // G0/G1 - Linear move
		return interp.doMove(cmd)
	case 28: // G28 - Home
		return interp.doHome(cmd)
	case 90: // G90 - Absolute positioning
		interp.state.AbsoluteMode = true
	case 91: // G91 - Relative positioning
		interp.state.AbsoluteMode = false
	case 92: // G92 - Set position
		return interp.doSetPosition(cmd)
	}

	return nil
}

// executeM handles M-codes
func (interp *Interpreter) executeM(cmd *standalone.GCodeCommand) error {
	switch cmd.Number {
	case 82: // M82 - Absolute extrusion
		interp.state.ExtrudeMode = false
	case 83: // M83 - Relative extrusion
		interp.state.ExtrudeMode = true
	case 104: // M104 - Set extruder temperature
		if cmd.HasParameter('S') {
			temp := cmd.GetParameter('S', 0)
			interp.state.TargetTemp["extruder"] = temp
		}
	case 109: // M109 - Set extruder temperature and wait
		if cmd.HasParameter('S') {
			temp := cmd.GetParameter('S', 0)
			interp.state.TargetTemp["extruder"] = temp
			// TODO: Wait for temperature
		}
	case 140: // M140 - Set bed temperature
		if cmd.HasParameter('S') {
			temp := cmd.GetParameter('S', 0)
			interp.state.TargetTemp["bed"] = temp
		}
	case 190: // M190 - Set bed temperature and wait
		if cmd.HasParameter('S') {
			temp := cmd.GetParameter('S', 0)
			interp.state.TargetTemp["bed"] = temp
			// TODO: Wait for temperature
		}
	case 114: // M114 - Get current position
		// TODO: Report position
	case 105: // M105 - Get temperature
		// TODO: Report temperature
	}

	return nil
}

// executeT handles tool changes
func (interp *Interpreter) executeT(cmd *standalone.GCodeCommand) error {
	// TODO: Implement tool change
	return nil
}

// doMove executes a linear move (G0/G1)
func (interp *Interpreter) doMove(cmd *standalone.GCodeCommand) error {
	// Get current position
	current := interp.planner.GetCurrentPosition()
	target := current

	// Update feedrate if specified
	if cmd.HasParameter('F') {
		interp.state.FeedRate = cmd.GetParameter('F', 0) / 60.0 // Convert mm/min to mm/s
	}

	// Calculate target position
	if interp.state.AbsoluteMode {
		// Absolute positioning
		if cmd.HasParameter('X') {
			target.X = cmd.GetParameter('X', current.X)
		}
		if cmd.HasParameter('Y') {
			target.Y = cmd.GetParameter('Y', current.Y)
		}
		if cmd.HasParameter('Z') {
			target.Z = cmd.GetParameter('Z', current.Z)
		}
	} else {
		// Relative positioning
		if cmd.HasParameter('X') {
			target.X = current.X + cmd.GetParameter('X', 0)
		}
		if cmd.HasParameter('Y') {
			target.Y = current.Y + cmd.GetParameter('Y', 0)
		}
		if cmd.HasParameter('Z') {
			target.Z = current.Z + cmd.GetParameter('Z', 0)
		}
	}

	// Handle extruder
	if cmd.HasParameter('E') {
		if interp.state.ExtrudeMode {
			// Relative extrusion
			target.E = current.E + cmd.GetParameter('E', 0)
		} else {
			// Absolute extrusion
			target.E = cmd.GetParameter('E', current.E)
		}
	}

	// Calculate distance
	dx := target.X - current.X
	dy := target.Y - current.Y
	dz := target.Z - current.Z
	de := target.E - current.E
	distance := sqrt(dx*dx + dy*dy + dz*dz)

	// Skip if no movement
	if distance < 0.001 && abs(de) < 0.001 {
		return nil
	}

	// Create move
	move := &standalone.Move{
		Start:    current,
		End:      target,
		Velocity: interp.state.FeedRate,
		Accel:    interp.config.DefaultAccel,
		Distance: distance,
	}

	// Queue move
	return interp.planner.QueueMove(move)
}

// doHome executes homing (G28)
func (interp *Interpreter) doHome(cmd *standalone.GCodeCommand) error {
	// TODO: Implement homing
	// For now, just mark axes as homed and set position to 0
	if !cmd.HasParameter('X') && !cmd.HasParameter('Y') && !cmd.HasParameter('Z') {
		// Home all axes
		interp.state.Homed = [4]bool{true, true, true, false}
		interp.planner.SetPosition(standalone.Position{X: 0, Y: 0, Z: 0, E: 0})
	} else {
		if cmd.HasParameter('X') {
			interp.state.Homed[0] = true
		}
		if cmd.HasParameter('Y') {
			interp.state.Homed[1] = true
		}
		if cmd.HasParameter('Z') {
			interp.state.Homed[2] = true
		}
	}

	return nil
}

// doSetPosition sets the current position (G92)
func (interp *Interpreter) doSetPosition(cmd *standalone.GCodeCommand) error {
	current := interp.planner.GetCurrentPosition()

	if cmd.HasParameter('X') {
		current.X = cmd.GetParameter('X', 0)
	}
	if cmd.HasParameter('Y') {
		current.Y = cmd.GetParameter('Y', 0)
	}
	if cmd.HasParameter('Z') {
		current.Z = cmd.GetParameter('Z', 0)
	}
	if cmd.HasParameter('E') {
		current.E = cmd.GetParameter('E', 0)
	}

	interp.planner.SetPosition(current)
	return nil
}

// GetState returns the current machine state
func (interp *Interpreter) GetState() *standalone.MachineState {
	return interp.state
}

// Simple math functions (to avoid importing math for embedded)
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method for square root
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
