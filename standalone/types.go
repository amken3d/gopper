package standalone

// Position represents a position in machine coordinates
type Position struct {
	X float64
	Y float64
	Z float64
	E float64 // Extruder
}

// Move represents a planned move with timing information
type Move struct {
	Start    Position
	End      Position
	Velocity float64  // Max velocity (mm/s)
	Accel    float64  // Acceleration (mm/s^2)
	Distance float64  // Total distance (mm)
	Duration uint32   // Duration in timer ticks

	// Trapezoidal profile parameters
	AccelTicks   uint32 // Time spent accelerating
	CruiseTicks  uint32 // Time spent at cruise velocity
	DecelTicks   uint32 // Time spent decelerating
	CruiseVel    float64 // Actual cruise velocity reached
	StartVel     float64 // Starting velocity
	EndVel       float64 // Ending velocity
}

// AxisConfig represents configuration for a single axis
type AxisConfig struct {
	StepPin      string  // GPIO pin for step pulses
	DirPin       string  // GPIO pin for direction
	EnablePin    string  // GPIO pin for enable (optional)
	StepsPerMM   float64 // Steps per millimeter
	MaxVelocity  float64 // Maximum velocity (mm/s)
	MaxAccel     float64 // Maximum acceleration (mm/s^2)
	HomingVel    float64 // Homing velocity (mm/s)
	MinPosition  float64 // Minimum position (mm)
	MaxPosition  float64 // Maximum position (mm)
	InvertDir    bool    // Invert direction signal
	InvertEnable bool    // Invert enable signal
}

// EndstopConfig represents configuration for an endstop
type EndstopConfig struct {
	Pin    string // GPIO pin
	Invert bool   // Invert signal
}

// HeaterConfig represents configuration for a heater
type HeaterConfig struct {
	SensorPin  string    // ADC pin for thermistor
	HeaterPin  string    // GPIO/PWM pin for heater
	PID        [3]float64 // PID gains [Kp, Ki, Kd]
	MinTemp    float64   // Minimum safe temperature
	MaxTemp    float64   // Maximum safe temperature
	MaxPower   float64   // Maximum power (0.0-1.0)
}

// MachineConfig represents the complete machine configuration
type MachineConfig struct {
	Mode       string                    // "standalone" or "klipper"
	Kinematics string                    // "cartesian", "corexy", "delta"
	Axes       map[string]AxisConfig     // "x", "y", "z", "e", etc.
	Endstops   map[string]EndstopConfig  // "x", "y", "z", etc.
	Heaters    map[string]HeaterConfig   // "extruder", "bed", etc.

	// Global motion parameters
	DefaultVelocity float64 // Default feedrate (mm/s)
	DefaultAccel    float64 // Default acceleration (mm/s^2)
	JunctionDeviation float64 // Junction deviation for cornering (mm)
}

// MachineState represents the current machine state
type MachineState struct {
	Position     Position // Current position
	Homed        [4]bool  // Homing status [X, Y, Z, E]
	AbsoluteMode bool     // Absolute (G90) vs relative (G91) positioning
	FeedRate     float64  // Current feedrate (mm/s)
	ExtrudeMode  bool     // Absolute vs relative extrusion
	Temperature  map[string]float64 // Current temperatures
	TargetTemp   map[string]float64 // Target temperatures
}

// GCodeCommand represents a parsed G-code command
type GCodeCommand struct {
	Type       byte               // 'G', 'M', 'T'
	Number     int                // Command number (e.g., 0 for G0, 28 for G28)
	Parameters map[byte]float64   // Parameters (X, Y, Z, E, F, S, etc.)
	Comment    string             // Comment text
}
