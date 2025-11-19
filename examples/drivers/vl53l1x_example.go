//go:build rp2040 || rp2350

// Example: VL53L1X Time-of-Flight Distance Sensor Integration
//
// This example demonstrates how to integrate the VL53L1X sensor using
// the TinyGo driver with Gopper's driver registry system.
//
// Note: TinyGo drivers repository provides VL53L1X driver. If you need
// VL53L0X support, you can adapt this pattern or use a third-party driver.
//
// Hardware Setup:
//   - VL53L1X sensor connected to I2C0
//   - SDA: GPIO4
//   - SCL: GPIO5
//   - VCC: 3.3V
//   - GND: GND
//
// Usage:
//   1. Copy this file to targets/rp2040/
//   2. Add to go.mod: tinygo.org/x/drivers v0.27.0
//   3. Call RegisterVL53L1XDriver() in main()
//   4. Use from Klipper with driver commands (OID 20)

package main

import (
	"gopper/core"

	"tinygo.org/x/drivers/vl53l1x"
)

const (
	VL53L1X_OID     = 20   // Klipper object ID for this driver
	VL53L1X_I2C_BUS = 0    // I2C bus 0 (I2C0)
	VL53L1X_ADDR    = 0x29 // Default I2C address
)

// RegisterVL53L1XDriver registers a VL53L1X distance sensor driver
// This can be used as a Z-probe for 3D printing, bed leveling, or obstacle detection
func RegisterVL53L1XDriver() error {
	config := core.NewI2CDriverConfig("vl53l1x_probe", VL53L1X_I2C_BUS, VL53L1X_ADDR)

	// Sensor configuration parameters
	config.Attributes["use_2v8_mode"] = true   // Use 2.8V I/O mode
	config.Attributes["timing_budget"] = 50000 // 50ms timing budget (microseconds)

	// Initialize: Create and configure the sensor
	config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
		// Get the machine.I2C bus
		i2c, err := core.GetMachineI2C(cfg.I2CBus)
		if err != nil {
			return nil, err
		}

		// Configure I2C bus at 400kHz (VL53L1X supports up to 400kHz)
		err = core.MustI2C().ConfigureBus(cfg.I2CBus, 400000)
		if err != nil {
			return nil, err
		}

		// Create VL53L1X device
		sensor := vl53l1x.New(i2c)

		// Configure sensor with 2.8V mode
		use2v8Mode := cfg.Attributes["use_2v8_mode"].(bool)
		sensor.Configure(use2v8Mode)

		// Set measurement timing budget
		timingBudget := cfg.Attributes["timing_budget"].(int)
		sensor.SetMeasurementTimingBudget(uint32(timingBudget))

		return sensor, nil
	}

	// Read: Perform a single distance measurement
	// Returns 2 bytes: distance in millimeters (big-endian uint16)
	// Range: 40mm to 4000mm (depending on target and lighting)
	config.ReadFunc = func(device interface{}, params []byte) ([]byte, error) {
		sensor := device.(*vl53l1x.Device)

		// Read distance (blocking read)
		distance := sensor.Read(true)

		// Check if measurement is valid
		// VL53L1X returns large values for out-of-range measurements
		if distance >= 8190 {
			distance = 8190 // Cap at max value
		}

		// Pack as 2 bytes (big-endian)
		result := []byte{
			byte(distance >> 8),
			byte(distance & 0xFF),
		}

		return result, nil
	}

	// Optional: Continuous polling mode
	// This is useful for real-time distance tracking or as a Z-probe
	config.PollFunc = func(device interface{}) ([]byte, error) {
		sensor := device.(*vl53l1x.Device)

		// Non-blocking read - returns 0 if no new data
		distance := sensor.Read(false)

		// If no new data available, return nil
		if distance == 0 {
			return nil, nil
		}

		if distance >= 8190 {
			distance = 8190
		}

		return []byte{
			byte(distance >> 8),
			byte(distance & 0xFF),
		}, nil
	}

	// Default polling rate: 50ms (20 Hz)
	// This can be overridden from Klipper with driver_start_poll command
	config.PollRate = 50 // milliseconds

	// Close: Cleanup when driver is unregistered
	config.CloseFunc = func(device interface{}) error {
		sensor := device.(*vl53l1x.Device)
		// Stop continuous mode if running
		sensor.StopContinuous()
		return nil
	}

	// Register with the driver registry
	return core.RegisterDriver(VL53L1X_OID, config)
}

// Example Klipper integration:
//
// In your Python code (e.g., klippy/extras/probe_vl53l0x.py):
//
// class ProbeVL53L0X:
//     def __init__(self, config):
//         self.printer = config.get_printer()
//         self.mcu = mcu.get_printer_mcu(self.printer, config.get('mcu', 'mcu'))
//
//         # Register commands
//         self.oid = self.mcu.create_oid()
//         self.mcu.add_config_cmd("config_driver oid=%d" % (self.oid,))
//
//         # Register response handler
//         self.mcu.register_response(self._handle_distance, "driver_poll_data", self.oid)
//
//         # Start polling at 50ms intervals (600000 ticks @ 12MHz)
//         self.mcu.add_config_cmd("driver_start_poll oid=%d poll_ticks=%d" % (self.oid, 600000))
//
//     def _handle_distance(self, params):
//         # Parse distance data (2 bytes, big-endian)
//         data = params['data']
//         distance_mm = (data[0] << 8) | data[1]
//         self.last_distance = distance_mm / 1000.0  # Convert to meters
//
//     def get_distance(self):
//         return self.last_distance
//
// In printer.cfg:
//
// [probe_vl53l0x]
// # VL53L0X probe configuration
// z_offset: 2.5  # Distance from nozzle to sensor trigger point
