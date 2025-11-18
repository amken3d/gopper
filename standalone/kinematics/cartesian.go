package kinematics

import (
	"errors"
	"gopper/standalone"
)

// Cartesian implements basic Cartesian kinematics (XYZ 1:1 mapping)
type Cartesian struct {
	config *standalone.MachineConfig
}

// NewCartesian creates a new Cartesian kinematics instance
func NewCartesian(config *standalone.MachineConfig) (*Cartesian, error) {
	// Validate required axes
	if _, ok := config.Axes["x"]; !ok {
		return nil, errors.New("X axis not configured")
	}
	if _, ok := config.Axes["y"]; !ok {
		return nil, errors.New("Y axis not configured")
	}
	if _, ok := config.Axes["z"]; !ok {
		return nil, errors.New("Z axis not configured")
	}

	return &Cartesian{
		config: config,
	}, nil
}

// CalcPosition converts XYZ coordinates to stepper positions
// For Cartesian, this is a 1:1 mapping
func (k *Cartesian) CalcPosition(pos standalone.Position) ([]float64, error) {
	// Return positions in order: X, Y, Z, E
	return []float64{pos.X, pos.Y, pos.Z, pos.E}, nil
}

// GetAxisNames returns the axis names for Cartesian kinematics
func (k *Cartesian) GetAxisNames() []string {
	return []string{"x", "y", "z", "e"}
}

// CheckLimits validates that a position is within configured limits
func (k *Cartesian) CheckLimits(pos standalone.Position) error {
	// Check X axis
	if xAxis, ok := k.config.Axes["x"]; ok {
		if pos.X < xAxis.MinPosition || pos.X > xAxis.MaxPosition {
			return errors.New("X position out of limits")
		}
	}

	// Check Y axis
	if yAxis, ok := k.config.Axes["y"]; ok {
		if pos.Y < yAxis.MinPosition || pos.Y > yAxis.MaxPosition {
			return errors.New("Y position out of limits")
		}
	}

	// Check Z axis
	if zAxis, ok := k.config.Axes["z"]; ok {
		if pos.Z < zAxis.MinPosition || pos.Z > zAxis.MaxPosition {
			return errors.New("Z position out of limits")
		}
	}

	return nil
}
