# TinyGo Driver Integration Guide

This guide explains how to integrate TinyGo drivers (from https://github.com/tinygo-org/drivers) with Gopper using the driver registry system.

## Overview

The driver registry provides a flexible, plugin-like system for integrating any TinyGo driver without modifying Gopper's core code. It offers:

- **Automatic Klipper command generation** for driver operations (read, write, configure, poll)
- **Support for all bus types**: I2C, SPI, GPIO, and custom implementations
- **Timer-based polling** for sensors that need periodic updates
- **Lifecycle management** with init, configure, read, write, and close callbacks
- **Direct machine.* access** for full TinyGo driver compatibility

## Architecture

### Components

1. **Driver Registry** (`core/driver_registry.go`): Central registry for managing driver instances
2. **Automatic Commands** (`core/driver_commands.go`): Auto-generated Klipper commands for each driver
3. **HAL Extensions**: Extended interfaces to expose `machine.I2C`, `machine.SPI`, etc.

### Klipper Commands

When you register a driver, the following commands become available:

| Command | Format | Description |
|---------|--------|-------------|
| `config_driver` | `oid=%c` | Configure a registered driver |
| `driver_read` | `oid=%c params=%*s` | Read data from driver |
| `driver_write` | `oid=%c data=%*s` | Write data to driver |
| `driver_start_poll` | `oid=%c poll_ticks=%u` | Start periodic polling |
| `driver_stop_poll` | `oid=%c` | Stop periodic polling |
| `driver_query_state` | `oid=%c` | Query driver state |
| `driver_unregister` | `oid=%c` | Unregister driver |

### Response Messages

| Response | Format | Description |
|----------|--------|-------------|
| `driver_data` | `oid=%c data=%*s` | Data from read operation |
| `driver_state` | `oid=%c configured=%c active=%c error_code=%c` | Driver state |
| `driver_poll_data` | `oid=%c data=%*s` | Data from periodic poll |

## Quick Start Examples

### Example 1: VL53L1X Time-of-Flight Sensor (I2C)

This example shows how to integrate the VL53L1X distance sensor using the TinyGo driver.

```go
package main

import (
	"gopper/core"
	"tinygo.org/x/drivers/vl53l1x"
)

func registerVL53L1XDriver() {
	// Create driver configuration
	config := core.NewI2CDriverConfig("vl53l1x", 0, 0x29) // I2C bus 0, address 0x29

	// Initialize function: create and configure the sensor
	config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
		// Get the machine.I2C bus
		i2c, err := core.GetMachineI2C(cfg.I2CBus)
		if err != nil {
			return nil, err
		}

		// Configure I2C bus if not already configured
		core.MustI2C().ConfigureBus(cfg.I2CBus, 400000) // 400 kHz

		// Create VL53L1X device
		sensor := vl53l1x.New(i2c)

		// Configure the sensor
		sensor.Configure(true) // true = use 2.8V I/O mode

		return sensor, nil
	}

	// Read function: read distance measurement
	config.ReadFunc = func(device interface{}, params []byte) ([]byte, error) {
		sensor := device.(*vl53l1x.Device)

		// Read distance in millimeters (blocking read)
		distance := sensor.Read(true)

		// Return distance as 2 bytes (big-endian)
		return []byte{byte(distance >> 8), byte(distance)}, nil
	}

	// Optional: Polling function for continuous measurements
	config.PollFunc = func(device interface{}) ([]byte, error) {
		sensor := device.(*vl53l1x.Device)

		// Non-blocking read - returns 0 if no new data
		distance := sensor.Read(false)
		if distance == 0 {
			return nil, nil // No new data yet
		}

		return []byte{byte(distance >> 8), byte(distance)}, nil
	}
	config.PollRate = 50 // Poll every 50ms (converted to timer ticks by StartPolling)

	// Register the driver with OID 10
	core.RegisterDriver(10, config)
}
```

**Usage from Klipper:**
```python
# In your printer.cfg or during runtime

# Configure the driver
DRIVER_CONFIG oid=10

# Read distance once
DRIVER_READ oid=10

# Start continuous polling at 50ms intervals (50ms * timer_freq)
DRIVER_START_POLL oid=10 poll_ticks=600000  # 50ms @ 12MHz timer

# Stop polling
DRIVER_STOP_POLL oid=10
```

### Example 2: BME280 Environmental Sensor (I2C)

Temperature, humidity, and pressure sensor.

```go
package main

import (
	"gopper/core"
	"tinygo.org/x/drivers/bme280"
)

func registerBME280Driver() {
	config := core.NewI2CDriverConfig("bme280", 0, 0x76) // I2C bus 0, address 0x76

	config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
		i2c, err := core.GetMachineI2C(cfg.I2CBus)
		if err != nil {
			return nil, err
		}

		core.MustI2C().ConfigureBus(cfg.I2CBus, 400000)

		sensor := bme280.New(i2c)
		sensor.Configure()

		return sensor, nil
	}

	// Read all measurements: temperature, pressure, humidity
	config.ReadFunc = func(device interface{}, params []byte) ([]byte, error) {
		sensor := device.(*bme280.Device)

		temp, err := sensor.ReadTemperature()
		if err != nil {
			return nil, err
		}

		pressure, err := sensor.ReadPressure()
		if err != nil {
			return nil, err
		}

		humidity, err := sensor.ReadHumidity()
		if err != nil {
			return nil, err
		}

		// Pack data: temp (2 bytes), pressure (4 bytes), humidity (2 bytes)
		result := make([]byte, 8)
		// Temperature in 0.01°C units
		tempInt := int16(temp / 10)
		result[0] = byte(tempInt >> 8)
		result[1] = byte(tempInt)
		// Pressure in Pa
		result[2] = byte(pressure >> 24)
		result[3] = byte(pressure >> 16)
		result[4] = byte(pressure >> 8)
		result[5] = byte(pressure)
		// Humidity in 0.01% units
		humInt := uint16(humidity / 10)
		result[6] = byte(humInt >> 8)
		result[7] = byte(humInt)

		return result, nil
	}

	config.PollFunc = config.ReadFunc // Reuse read function for polling
	config.PollRate = 1000 // Poll every 1 second

	core.RegisterDriver(11, config)
}
```

### Example 3: SSD1306 OLED Display (I2C)

This example shows how to use a display driver for output.

```go
package main

import (
	"gopper/core"
	"image/color"
	"tinygo.org/x/drivers/ssd1306"
)

func registerSSD1306Driver() {
	config := core.NewI2CDriverConfig("ssd1306", 0, 0x3C) // I2C bus 0, address 0x3C

	config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
		i2c, err := core.GetMachineI2C(cfg.I2CBus)
		if err != nil {
			return nil, err
		}

		core.MustI2C().ConfigureBus(cfg.I2CBus, 400000)

		// Create 128x64 OLED display
		display := ssd1306.NewI2C(i2c)
		display.Configure(ssd1306.Config{
			Address: cfg.I2CAddr,
			Width:   128,
			Height:  64,
		})

		// Clear display
		display.ClearDisplay()

		return display, nil
	}

	// Write function: update display with text or graphics
	// Format: [cmd_byte, x, y, ...data]
	// cmd_byte: 0x01 = clear, 0x02 = set pixel, 0x03 = display buffer
	config.WriteFunc = func(device interface{}, data []byte) error {
		display := device.(*ssd1306.Device)

		if len(data) < 1 {
			return nil
		}

		cmd := data[0]
		switch cmd {
		case 0x01: // Clear display
			display.ClearDisplay()

		case 0x02: // Set pixel
			if len(data) >= 3 {
				x := int16(data[1])
				y := int16(data[2])
				display.SetPixel(x, y, color.RGBA{255, 255, 255, 255})
			}

		case 0x03: // Display buffer
			display.Display()
		}

		return nil
	}

	core.RegisterDriver(12, config)
}
```

### Example 4: ADXL345 Accelerometer (SPI)

Useful for input shaping in 3D printers.

```go
package main

import (
	"gopper/core"
	"machine"
	"tinygo.org/x/drivers/adxl345"
)

func registerADXL345Driver() {
	// SPI bus 0, mode 3, 5MHz, CS on GPIO 5
	config := core.NewSPIDriverConfig("adxl345", 0, 3, 5000000, 5)

	config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
		// Configure SPI bus
		spiConfig := core.SPIConfig{
			BusID: cfg.SPIBus,
			Mode:  cfg.SPIMode,
			Rate:  cfg.SPIRate,
		}
		busHandle, err := core.MustSPI().ConfigureBus(spiConfig)
		if err != nil {
			return nil, err
		}

		// Get machine.SPI instance
		spi, err := core.GetMachineSPI(busHandle)
		if err != nil {
			return nil, err
		}

		// Configure CS pin
		csPin := machine.Pin(cfg.SPICSPin)
		csPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
		csPin.High()

		// Create ADXL345 device
		sensor := adxl345.New(spi, csPin)
		sensor.Configure()

		// Store both sensor and bus handle
		type deviceWrapper struct {
			sensor    *adxl345.Device
			busHandle interface{}
		}

		return &deviceWrapper{sensor: sensor, busHandle: busHandle}, nil
	}

	config.ReadFunc = func(device interface{}, params []byte) ([]byte, error) {
		wrapper := device.(*struct {
			sensor    *adxl345.Device
			busHandle interface{}
		})

		x, y, z, err := wrapper.sensor.ReadAcceleration()
		if err != nil {
			return nil, err
		}

		// Pack as 6 bytes (3x int16, big-endian)
		result := make([]byte, 6)
		result[0] = byte(x >> 8)
		result[1] = byte(x)
		result[2] = byte(y >> 8)
		result[3] = byte(y)
		result[4] = byte(z >> 8)
		result[5] = byte(z)

		return result, nil
	}

	// High-speed polling for resonance measurement
	config.PollFunc = config.ReadFunc
	config.PollRate = 5 // Poll every 5ms (200Hz for input shaping)

	core.RegisterDriver(13, config)
}
```

### Example 5: WS2812 RGB LED Strip (GPIO)

Control addressable LEDs.

```go
package main

import (
	"gopper/core"
	"machine"
	"tinygo.org/x/drivers/ws2812"
)

func registerWS2812Driver() {
	config := &core.DriverConfig{
		Name:     "ws2812",
		Type:     core.DriverTypeGPIO,
		GPIOPins: []core.GPIOPin{16}, // Data pin on GPIO16
		Attributes: map[string]interface{}{
			"num_leds": 30, // 30 LEDs in strip
		},
	}

	config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
		// Configure data pin
		dataPin := machine.Pin(cfg.GPIOPins[0])

		// Get number of LEDs from attributes
		numLEDs := cfg.Attributes["num_leds"].(int)

		// Create WS2812 device
		leds := ws2812.New(dataPin)
		colors := make([]color.RGBA, numLEDs)

		// Store both device and color buffer
		type deviceWrapper struct {
			leds   ws2812.Device
			colors []color.RGBA
		}

		return &deviceWrapper{leds: leds, colors: colors}, nil
	}

	// Write function: set LED colors
	// Format: [start_index, count, R, G, B, R, G, B, ...]
	config.WriteFunc = func(device interface{}, data []byte) error {
		wrapper := device.(*struct {
			leds   ws2812.Device
			colors []color.RGBA
		})

		if len(data) < 2 {
			return nil
		}

		startIdx := int(data[0])
		count := int(data[1])

		// Update color buffer
		for i := 0; i < count && i < len(wrapper.colors)-startIdx; i++ {
			if len(data) >= 3+i*3+3 {
				r := data[2+i*3]
				g := data[3+i*3]
				b := data[4+i*3]
				wrapper.colors[startIdx+i] = color.RGBA{r, g, b, 255}
			}
		}

		// Write to LEDs
		wrapper.leds.WriteColors(wrapper.colors)

		return nil
	}

	core.RegisterDriver(14, config)
}
```

## Integration Workflow

### 1. Add Driver Dependencies

First, add the TinyGo driver to your `go.mod`:

```bash
go get tinygo.org/x/drivers@latest
```

### 2. Register Drivers in Target-Specific Code

Create a `drivers.go` file in your target directory (e.g., `targets/rp2040/drivers.go`):

```go
//go:build rp2040 || rp2350

package main

import "gopper/core"

func RegisterCustomDrivers() {
	// Register all your custom drivers here
	registerVL53L0XDriver()
	registerBME280Driver()
	registerSSD1306Driver()
	// ... more drivers
}
```

### 3. Call Registration in main.go

Add the registration call in `targets/rp2040/main.go`:

```go
func main() {
	// ... existing initialization ...

	// Initialize driver registry commands
	core.InitDriverCommands()

	// Register custom drivers
	RegisterCustomDrivers()

	// ... continue with dictionary build ...
}
```

### 4. Use from Klipper

From Klipper Python code or printer.cfg:

```python
# Configure a sensor
self.driver_config = self.mcu.lookup_command("config_driver oid=%c")
self.driver_config.send([10])  # Configure driver OID 10

# Read sensor data
self.driver_read = self.mcu.lookup_command("driver_read oid=%c params=%*s")
data = self.driver_read.send_with_response([10, []], "driver_data")

# Start polling
self.driver_poll = self.mcu.lookup_command("driver_start_poll oid=%c poll_ticks=%u")
self.driver_poll.send([10, 600000])  # Poll every 50ms @ 12MHz
```

## Advanced Usage

### Custom Configuration Attributes

You can pass custom configuration through the `Attributes` map:

```go
config := core.NewI2CDriverConfig("custom_sensor", 0, 0x50)
config.Attributes["sample_rate"] = 100
config.Attributes["sensitivity"] = "high"
config.Attributes["calibration_offset"] = 42

config.InitFunc = func(cfg *core.DriverConfig) (interface{}, error) {
	sampleRate := cfg.Attributes["sample_rate"].(int)
	sensitivity := cfg.Attributes["sensitivity"].(string)
	// Use these in driver initialization
	// ...
}
```

### Lifecycle Hooks

Implement all lifecycle callbacks for complete control:

```go
config.ConfigureFunc = func(device interface{}, cfg *core.DriverConfig) error {
	// Called when config_driver command is received
	// Perform any runtime configuration here
	return nil
}

config.CloseFunc = func(device interface{}) error {
	// Called when driver is unregistered
	// Clean up resources, reset device, etc.
	return nil
}
```

### Error Handling

The driver state tracks errors automatically:

```go
config.ReadFunc = func(device interface{}, params []byte) ([]byte, error) {
	// If an error occurs, it's stored in instance.State.LastError
	data, err := performReading()
	if err != nil {
		return nil, err // Error is automatically tracked
	}
	return data, nil
}
```

Query error state from Klipper:

```python
state = self.driver_query.send_with_response([10], "driver_state")
if state['error_code'] != 0:
	# Handle error
	pass
```

## Best Practices

1. **Bus Configuration**: Always configure I2C/SPI buses before creating driver instances
2. **Error Handling**: Return meaningful errors from lifecycle functions
3. **Resource Cleanup**: Implement `CloseFunc` to properly release resources
4. **Polling Rate**: Choose appropriate poll rates based on sensor requirements
5. **Data Packing**: Use efficient binary encoding for sensor data (big-endian, fixed-width)
6. **Thread Safety**: Driver functions may be called from timer interrupts - avoid blocking operations

## Troubleshooting

### Driver Not Found
- Ensure `InitDriverCommands()` is called before dictionary build
- Check that `RegisterDriver()` was called with unique OID

### I2C/SPI Communication Errors
- Verify bus is configured with correct frequency
- Check pin assignments match your hardware
- Use oscilloscope/logic analyzer to verify signals

### Polling Not Working
- Ensure `PollFunc` is implemented
- Check that poll rate is converted to timer ticks correctly
- Verify timer system is running (check with `get_clock` command)

### Memory Issues
- TinyGo GC may collect buffers - always copy data from temporary buffers
- Avoid allocations in hot paths (polling, reads)
- Use fixed-size buffers where possible

## Contributing

When adding new driver examples:

1. Test thoroughly on hardware
2. Document pin assignments and wiring
3. Provide example Klipper Python code
4. Include error handling and edge cases
5. Follow the naming convention: `register<DriverName>Driver()`

## See Also

- [TinyGo Drivers Repository](https://github.com/tinygo-org/drivers)
- [Klipper Protocol Documentation](https://www.klipper3d.org/Protocol.html)

