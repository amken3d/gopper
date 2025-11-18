package kinematics

import "gopper/standalone"

// Kinematics defines the interface for coordinate transformations
type Kinematics interface {
	// CalcPosition converts XYZ coordinates to stepper positions
	CalcPosition(pos standalone.Position) ([]float64, error)

	// GetAxisNames returns the names of axes controlled by this kinematics
	GetAxisNames() []string

	// CheckLimits validates that a position is within configured limits
	CheckLimits(pos standalone.Position) error
}

// AxisLimits represents position limits for an axis
type AxisLimits struct {
	Min float64
	Max float64
}
