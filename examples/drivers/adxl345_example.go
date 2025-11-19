//go:build rp2040 || rp2350

// Example: ADXL345 Accelerometer for Input Shaping
//
// This example demonstrates how to integrate the ADXL345 accelerometer
// for measuring printer resonances and implementing input shaping.
//
// Note: The TinyGo ADXL345 driver uses I2C. For SPI-based accelerometers,
// consider using LIS3DH or implementing a custom SPI driver.
//
// Hardware Setup:
//   - ADXL345 connected via I2C
//   - I2C0: SDA=GPIO4, SCL=GPIO5
//   - Address: 0x53 (SDO/ALT ADDRESS pin low) or 0x1D (pin high)
//   - VCC: 3.3V
//   - GND: GND
//
// Usage:
//   1. Copy this file to targets/rp2040/
//   2. Add to go.mod: tinygo.org/x/drivers v0.27.0
//   3. Call RegisterADXL345Driver() in main()
//   4. Use from Klipper with driver commands (OID 21)

package main

import (
	"gopper/core"

	"tinygo.org/x/drivers/adxl345"
)

const (
	ADXL345_OID     = 21   // Klipper object ID
	ADXL345_I2C_BUS = 0    // I2C bus 0 (SDA=GP4, SCL=GP5)
	ADXL345_ADDR    = 0x53 // Default I2C address (SDO pin low)
)

// RegisterADXL345Driver registers an ADXL345 accelerometer driver
// This is used for input shaping to reduce printer resonances and ringing
func RegisterADXL345Driver() error {
	config := core.NewI2CDriverConfig(
		"adxl345_accel",
		ADXL345_I2C_BUS,
		ADXL345_ADDR,
	)

	// Configuration attributes
	config.Attributes["data_rate"] = adxl345.RATE_0_78HZ // 100Hz for testing, use Rate3200Hz for input shaping
	config.Attributes["range"] = adxl345.RANGE_16G       // ±16g range

	// Initialize: Configure I2C and create ADXL345 device
	config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
		// Get machine.I2C instance for TinyGo driver
		i2c, err := core.GetMachineI2C(cfg.I2CBus)
		if err != nil {
			return nil, err
		}

		// Configure I2C bus at 400kHz
		err = core.MustI2C().ConfigureBus(cfg.I2CBus, 400000)
		if err != nil {
			return nil, err
		}

		// Create ADXL345 device
		sensor := adxl345.New(i2c)

		// Configure sensor
		sensor.Configure()

		// Set data rate from attributes
		dataRate := cfg.Attributes["data_rate"].(adxl345.Rate)
		sensor.SetRate(dataRate)

		// Set measurement range
		measureRange := cfg.Attributes["range"].(adxl345.Range)
		sensor.SetRange(measureRange)

		// Return pointer to device
		return &sensor, nil
	}

	// Read: Single acceleration measurement
	// Returns 6 bytes: X, Y, Z as int16 (big-endian)
	// Units: raw ADC values (±16g range = ±32768 counts = ~256 counts/g)
	config.ReadFunc = func(device interface{}, params []byte) ([]byte, error) {
		sensor := device.(*adxl345.Device)

		// Read raw acceleration on all three axes
		x, y, z := sensor.ReadRawAcceleration()

		// Pack as 6 bytes: X (2), Y (2), Z (2) - big-endian int16
		result := []byte{
			byte(x >> 8), byte(x),
			byte(y >> 8), byte(y),
			byte(z >> 8), byte(z),
		}

		return result, nil
	}

	// Poll: Continuous high-speed sampling for resonance measurement
	config.PollFunc = func(device interface{}) ([]byte, error) {
		sensor := device.(*adxl345.Device)

		// Read raw acceleration
		x, y, z := sensor.ReadRawAcceleration()

		return []byte{
			byte(x >> 8), byte(x),
			byte(y >> 8), byte(y),
			byte(z >> 8), byte(z),
		}, nil
	}

	// Polling rate: 10ms default (100Hz)
	// For input shaping at 3.2kHz, set poll_ticks to ~3750 @ 12MHz timer
	config.PollRate = 10 // milliseconds

	// Close: Put sensor in standby mode
	config.CloseFunc = func(device interface{}) error {
		sensor := device.(*adxl345.Device)
		sensor.Halt()
		return nil
	}

	return core.RegisterDriver(ADXL345_OID, config)
}

// Example Klipper integration for input shaping:
//
// In klippy/extras/adxl345_gopper.py:
//
// class ADXL345Gopper:
//     def __init__(self, config):
//         self.printer = config.get_printer()
//         self.mcu = mcu.get_printer_mcu(self.printer, config.get('mcu'))
//
//         # Configuration
//         self.oid = self.mcu.create_oid()
//         self.query_rate = config.getint('rate', 3200)  # 3.2kHz default
//
//         # Commands
//         self.mcu.add_config_cmd("config_driver oid=%d" % (self.oid,))
//
//         # Calculate poll ticks for desired sample rate
//         mcu_freq = self.mcu.get_constants().get('CLOCK_FREQ', 12000000)
//         poll_ticks = int(mcu_freq / self.query_rate)
//
//         # Response handler for acceleration data
//         self.mcu.register_response(self._handle_accel_data, "driver_poll_data", self.oid)
//
//         # Buffer for storing samples
//         self.samples = []
//         self.is_measuring = False
//
//     def start_measurements(self):
//         # Start polling at desired rate
//         self.is_measuring = True
//         self.samples = []
//         mcu_freq = self.mcu.get_constants().get('CLOCK_FREQ', 12000000)
//         poll_ticks = int(mcu_freq / self.query_rate)
//         self.mcu.send("driver_start_poll oid=%d poll_ticks=%d" % (self.oid, poll_ticks))
//
//     def stop_measurements(self):
//         self.mcu.send("driver_stop_poll oid=%d" % (self.oid,))
//         self.is_measuring = False
//         return self.samples
//
//     def _handle_accel_data(self, params):
//         # Parse acceleration data (6 bytes: X, Y, Z as int16 big-endian)
//         data = params['data']
//         x = struct.unpack('>h', bytes([data[0], data[1]]))[0]
//         y = struct.unpack('>h', bytes([data[2], data[3]]))[0]
//         z = struct.unpack('>h', bytes([data[4], data[5]]))[0]
//
//         # Convert to g-forces (±16g range = ±32768 counts)
//         scale = 16.0 / 32768.0
//         accel = {
//             'x': x * scale,
//             'y': y * scale,
//             'z': z * scale,
//             'time': self.mcu.estimated_print_time(params['#sent_time'])
//         }
//
//         if self.is_measuring:
//             self.samples.append(accel)
//
// In printer.cfg:
//
// [adxl345]
// # ADXL345 connected via I2C
// i2c_bus: i2c0
// rate: 3200  # 3.2kHz sample rate
//
// [resonance_tester]
// accel_chip: adxl345
// probe_points: 100, 100, 20  # Center of bed
//
// Usage:
//   ACCELEROMETER_QUERY  # Test connection
//   TEST_RESONANCES AXIS=X  # Measure X-axis resonances
//   TEST_RESONANCES AXIS=Y  # Measure Y-axis resonances
//   SHAPER_CALIBRATE  # Auto-calibrate input shaper
