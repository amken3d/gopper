package config

import (
	"encoding/json"
	"gopper/standalone"
)

// LoadConfig parses a JSON configuration string and returns a MachineConfig
func LoadConfig(jsonData []byte) (*standalone.MachineConfig, error) {
	var config standalone.MachineConfig

	err := json.Unmarshal(jsonData, &config)
	if err != nil {
		return nil, err
	}

	// Apply defaults
	applyDefaults(&config)

	return &config, nil
}

// applyDefaults fills in missing configuration values with sensible defaults
func applyDefaults(config *standalone.MachineConfig) {
	// Default mode
	if config.Mode == "" {
		config.Mode = "standalone"
	}

	// Default kinematics
	if config.Kinematics == "" {
		config.Kinematics = "cartesian"
	}

	// Default motion parameters
	if config.DefaultVelocity == 0 {
		config.DefaultVelocity = 50.0 // 50 mm/s
	}
	if config.DefaultAccel == 0 {
		config.DefaultAccel = 500.0 // 500 mm/s^2
	}
	if config.JunctionDeviation == 0 {
		config.JunctionDeviation = 0.05 // 0.05mm
	}

	// Apply defaults to each axis
	for name, axis := range config.Axes {
		if axis.MaxVelocity == 0 {
			axis.MaxVelocity = 300.0
		}
		if axis.MaxAccel == 0 {
			axis.MaxAccel = 1000.0
		}
		if axis.HomingVel == 0 {
			axis.HomingVel = 5.0
		}
		if axis.StepsPerMM == 0 {
			axis.StepsPerMM = 80.0 // Common value
		}
		config.Axes[name] = axis
	}

	// Apply defaults to heaters
	for name, heater := range config.Heaters {
		if heater.MinTemp == 0 {
			heater.MinTemp = 0.0
		}
		if heater.MaxTemp == 0 {
			heater.MaxTemp = 300.0
		}
		if heater.MaxPower == 0 {
			heater.MaxPower = 1.0
		}
		config.Heaters[name] = heater
	}
}

// DefaultCartesianConfig returns a default configuration for a Cartesian printer
func DefaultCartesianConfig() *standalone.MachineConfig {
	return &standalone.MachineConfig{
		Mode:       "standalone",
		Kinematics: "cartesian",
		Axes: map[string]standalone.AxisConfig{
			"x": {
				StepPin:     "gpio0",
				DirPin:      "gpio1",
				EnablePin:   "gpio8",
				StepsPerMM:  80.0,
				MaxVelocity: 300.0,
				MaxAccel:    3000.0,
				HomingVel:   50.0,
				MinPosition: 0.0,
				MaxPosition: 220.0,
			},
			"y": {
				StepPin:     "gpio2",
				DirPin:      "gpio3",
				EnablePin:   "gpio8",
				StepsPerMM:  80.0,
				MaxVelocity: 300.0,
				MaxAccel:    3000.0,
				HomingVel:   50.0,
				MinPosition: 0.0,
				MaxPosition: 220.0,
			},
			"z": {
				StepPin:     "gpio4",
				DirPin:      "gpio5",
				EnablePin:   "gpio8",
				StepsPerMM:  400.0,
				MaxVelocity: 10.0,
				MaxAccel:    100.0,
				HomingVel:   5.0,
				MinPosition: 0.0,
				MaxPosition: 250.0,
			},
			"e": {
				StepPin:     "gpio6",
				DirPin:      "gpio7",
				EnablePin:   "gpio8",
				StepsPerMM:  96.0,
				MaxVelocity: 50.0,
				MaxAccel:    5000.0,
				HomingVel:   0.0,
				MinPosition: -10000.0,
				MaxPosition: 10000.0,
			},
		},
		Endstops: map[string]standalone.EndstopConfig{
			"x": {Pin: "gpio20", Invert: false},
			"y": {Pin: "gpio21", Invert: false},
			"z": {Pin: "gpio22", Invert: false},
		},
		Heaters: map[string]standalone.HeaterConfig{
			"extruder": {
				SensorPin: "ADC0",
				HeaterPin: "gpio10",
				PID:       [3]float64{0.1, 0.5, 0.05},
				MinTemp:   0.0,
				MaxTemp:   300.0,
				MaxPower:  1.0,
			},
			"bed": {
				SensorPin: "ADC1",
				HeaterPin: "gpio11",
				PID:       [3]float64{0.2, 1.0, 0.1},
				MinTemp:   0.0,
				MaxTemp:   150.0,
				MaxPower:  1.0,
			},
		},
		DefaultVelocity:   50.0,
		DefaultAccel:      500.0,
		JunctionDeviation: 0.05,
	}
}
