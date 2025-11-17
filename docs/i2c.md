# I2C Implementation in Gopper

This document describes the I2C (Inter-Integrated Circuit) implementation in Gopper, which provides compatibility with Klipper's I2C protocol.

## Overview

The I2C implementation allows Gopper to communicate with I2C peripheral devices such as:
- Temperature sensors (e.g., BME280, SHT3x)
- Accelerometers (e.g., ADXL345 for input shaping)
- Display controllers (e.g., SSD1306 OLED)
- I/O expanders (e.g., MCP23017)
- Other I2C-compatible sensors and actuators

## Architecture

The I2C implementation follows Gopper's standard three-layer architecture:

### 1. Core Layer (`core/i2c.go`)
Implements Klipper protocol command handlers:
- **config_i2c**: Allocates an I2C device object
- **i2c_set_bus**: Configures the I2C bus, frequency, and device address
- **i2c_write**: Writes data to an I2C device
- **i2c_read**: Reads data from an I2C device (with optional register addressing)

### 2. HAL Layer (`core/i2c_hal.go`)
Defines the platform-independent I2C interface:
```go
type I2CDriver interface {
    ConfigureBus(bus I2CBusID, frequencyHz uint32) error
    Write(bus I2CBusID, addr I2CAddress, data []byte) error
    Read(bus I2CBusID, addr I2CAddress, regData []byte, readLen uint8) ([]byte, error)
}
```

### 3. Platform Layer (`targets/rp2040/i2c.go`)
Implements the HAL interface using TinyGo's `machine.I2C` API for RP2040/RP2350.

## Supported Hardware

### RP2040/RP2350

The RP2040 and RP2350 microcontrollers have two I2C controllers:

- **I2C0** (Bus ID 0)
    - Default SDA: GPIO4
    - Default SCL: GPIO5

- **I2C1** (Bus ID 1)
    - Default SDA: GPIO6
    - Default SCL: GPIO7

Both controllers support standard I2C frequencies:
- 100 kHz (standard mode)
- 400 kHz (fast mode)
- Up to 1 MHz (fast mode plus)

## Klipper Protocol

### Command: config_i2c
Allocates an I2C device object.

**Format**: `config_i2c oid=%c`

**Parameters**:
- `oid`: Object ID for this I2C device

### Command: i2c_set_bus
Configures the I2C bus, frequency, and device address.

**Format**: `i2c_set_bus oid=%c i2c_bus=%u rate=%u address=%u`

**Parameters**:
- `oid`: Object ID of the I2C device
- `i2c_bus`: Bus number (0 or 1 for RP2040)
- `rate`: I2C frequency in Hz (e.g., 100000 for 100 kHz, 400000 for 400 kHz)
- `address`: 7-bit I2C device address (automatically masked to 7 bits)

### Command: i2c_write
Writes data to an I2C device.

**Format**: `i2c_write oid=%c data=%*s`

**Parameters**:
- `oid`: Object ID of the I2C device
- `data`: Buffer of bytes to write

**Error Handling**: Triggers firmware shutdown on I2C NACK or timeout errors.

### Command: i2c_read
Reads data from an I2C device, optionally writing a register address first.

**Format**: `i2c_read oid=%c reg=%*s read_len=%u`

**Parameters**:
- `oid`: Object ID of the I2C device
- `reg`: Optional register address to write before reading (can be empty for simple reads)
- `read_len`: Number of bytes to read

**Response**: `i2c_read_response oid=%c response=%*s`

**Error Handling**: Triggers firmware shutdown on I2C NACK or timeout errors.

## Usage Example (Python/Klipper)

Here's how Klipper's Python code typically uses I2C:

```python
# Configure I2C device
oid = self.mcu.create_oid()
self.mcu.add_config_cmd("config_i2c oid=%d" % oid)
self.mcu.add_config_cmd("i2c_set_bus oid=%d i2c_bus=%d rate=%d address=%d"
                        % (oid, 0, 400000, 0x76))

# Write data
data = [0x01, 0x02, 0x03]
self.mcu.send_cmd("i2c_write oid=%d data=%s" % (oid, data))

# Read from register
reg = [0xF7]  # Register address
read_len = 3
response = self.mcu.send_with_response("i2c_read oid=%d reg=%s read_len=%d"
                                       % (oid, reg, read_len),
                                       "i2c_read_response oid=%c response=%*s")
```

## Implementation Details

### Thread Safety
The RP2040 I2C driver uses a mutex (`sync.Mutex`) to serialize I2C operations, preventing concurrent access to the same bus.

### Error Handling
I2C errors (NACK, timeout, bus errors) trigger a firmware shutdown via `TryShutdown()`, matching Klipper's behavior. This ensures that communication failures are detected and handled safely.

### Register Addressing
The `i2c_read` command supports writing a register address before reading:
- If `reg` is non-empty, it's transmitted first, followed by a restart condition and the read operation
- If `reg` is empty, a simple read transaction is performed
- This matches the behavior of TinyGo's `Tx()` method

### Bus Configuration
I2C buses are configured on-demand when `i2c_set_bus` is called:
- The first call to `i2c_set_bus` for a given bus initializes it with the specified frequency
- Subsequent calls can update the baud rate via `SetBaudRate()`
- Each I2C device object maintains its own address and bus assignment

## Common I2C Devices

### ADXL345 Accelerometer (Input Shaping)
- **Address**: 0x53 (default) or 0x1D (alternate)
- **Frequency**: 400 kHz
- **Usage**: Measures printer vibrations for input shaping calibration

### BME280 Environmental Sensor
- **Address**: 0x76 (default) or 0x77 (alternate)
- **Frequency**: 100-400 kHz
- **Usage**: Measures temperature, humidity, and barometric pressure

### SSD1306 OLED Display
- **Address**: 0x3C or 0x3D
- **Frequency**: 400 kHz
- **Usage**: Status display for printer information

## Debugging

### I2C Communication Issues

If you encounter I2C communication problems:

1. **Check wiring**:
    - Verify SDA and SCL connections
    - Ensure pull-up resistors are present (typically 4.7kΩ for 100kHz, 2.2kΩ for 400kHz)
    - Check power supply to the I2C device

2. **Verify address**:
    - Use an I2C scanner to detect devices
    - Some devices have configurable addresses (check device datasheet)
    - Addresses are 7-bit (0x00 to 0x7F)

3. **Check bus speed**:
    - Start with 100 kHz for testing
    - Some devices may not support 400 kHz or faster speeds
    - Long wires may require lower speeds

4. **Monitor firmware logs**:
    - I2C errors trigger firmware shutdown
    - Check Klipper logs for "I2C write error" or "I2C read error" messages

## Future Enhancements

Potential improvements for the I2C implementation:

- [ ] Support for 10-bit I2C addresses
- [ ] Configurable pin assignments (currently uses default pins)
- [ ] I2C bus scanning utility
- [ ] Support for clock stretching and other advanced features
- [ ] Multi-master I2C support
- [ ] Configurable timeout values

## References

- [Klipper I2C implementation (RP2040)](https://github.com/Klipper3d/klipper/blob/master/src/rp2040/i2c.c)
- [Klipper I2C commands](https://github.com/Klipper3d/klipper/blob/master/src/i2ccmds.c)
- [TinyGo machine.I2C documentation](https://tinygo.org/docs/reference/microcontrollers/machine/)
- [RP2040 Datasheet - I2C](https://datasheets.raspberrypi.com/rp2040/rp2040-datasheet.pdf)

