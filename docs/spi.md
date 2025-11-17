# SPI (Serial Peripheral Interface) Implementation

This document describes the SPI implementation in Gopper for RP2040/RP2350 platforms.

## Overview

Gopper implements both hardware and software SPI support, matching Klipper's SPI protocol. This allows communication with SPI devices such as:
- TMC stepper driver chips (TMC2130, TMC5160, etc.)
- Accelerometers (ADXL345, LIS2DW, etc.)
- SD cards
- Display controllers
- Other SPI peripherals

## Architecture

The SPI implementation follows Gopper's standard three-layer architecture:

1. **HAL Interface** (`core/spi_hal.go`): Abstract interface for SPI operations
2. **Command Layer** (`core/spi.go`): Klipper protocol command handlers
3. **Platform Layer** (`targets/rp2040/spi.go`, `targets/rp2040/spi_software.go`): Hardware-specific implementation

## Hardware SPI Support

### RP2040/RP2350 SPI Bus Configurations

The RP2040 and RP2350 have two hardware SPI controllers (SPI0 and SPI1), each supporting multiple GPIO pin configurations. Gopper supports all 9 standard bus configurations from Klipper:

| Bus ID | Controller | MISO Pin | MOSI Pin | SCK Pin | Name   |
|--------|------------|----------|----------|---------|--------|
| 0      | SPI0       | GPIO0    | GPIO3    | GPIO2   | spi0a  |
| 1      | SPI0       | GPIO4    | GPIO7    | GPIO6   | spi0b  |
| 2      | SPI0       | GPIO16   | GPIO19   | GPIO18  | spi0c  |
| 3      | SPI0       | GPIO20   | GPIO23   | GPIO22  | spi0d  |
| 4      | SPI0       | GPIO4    | GPIO3    | GPIO2   | spi0e  |
| 5      | SPI1       | GPIO8    | GPIO11   | GPIO10  | spi1a  |
| 6      | SPI1       | GPIO12   | GPIO15   | GPIO14  | spi1b  |
| 7      | SPI1       | GPIO24   | GPIO27   | GPIO26  | spi1c  |
| 8      | SPI1       | GPIO12   | GPIO11   | GPIO10  | spi1d  |

**Note:** The chip select (CS) pin is specified separately in the `config_spi` command and can be any available GPIO pin.

### Common Use Cases

- **Bus 2 (spi0c)**: Commonly used for ADXL345 accelerometer on Raspberry Pi Pico boards
- **Bus 5 (spi1a)**: Alternative for SPI devices when SPI0 is in use
- **Bus 0 (spi0a)**: Default SPI bus on many RP2040 boards

## Software SPI Support

Software SPI (bit-banging) is available as a fallback when:
- Hardware SPI pins are already in use
- Non-standard pin combinations are needed
- Lower speeds are acceptable

Software SPI can use any GPIO pins but is slower than hardware SPI and consumes more CPU time.

## Klipper Commands

### Device Configuration

#### `config_spi`
Configure an SPI device with a chip select pin.

**Format:** `config_spi oid=%c pin=%u cs_active_high=%c`

**Parameters:**
- `oid`: Object identifier (unique per SPI device)
- `pin`: GPIO pin number for chip select
- `cs_active_high`: CS polarity (0=active low, 1=active high)

**Example:**
```
config_spi oid=5 pin=17 cs_active_high=0
```

#### `config_spi_without_cs`
Configure an SPI device without automatic chip select management.

**Format:** `config_spi_without_cs oid=%c`

**Parameters:**
- `oid`: Object identifier

**Example:**
```
config_spi_without_cs oid=6
```

### Bus Configuration

#### `spi_set_bus`
Set the SPI bus parameters for a configured device.

**Format:** `spi_set_bus oid=%c spi_bus=%u mode=%u rate=%u`

**Parameters:**
- `oid`: Object identifier
- `spi_bus`: Hardware bus ID (0-8 for hardware, ≥128 for software SPI)
- `mode`: SPI mode (0-3)
- `rate`: Clock rate in Hz

**SPI Modes:**
- **Mode 0**: CPOL=0, CPHA=0 (clock idle low, sample on rising edge)
- **Mode 1**: CPOL=0, CPHA=1 (clock idle low, sample on falling edge)
- **Mode 2**: CPOL=1, CPHA=0 (clock idle high, sample on falling edge)
- **Mode 3**: CPOL=1, CPHA=1 (clock idle high, sample on rising edge)

**Example:**
```
spi_set_bus oid=5 spi_bus=2 mode=3 rate=4000000
```

### Data Transfer

#### `spi_transfer`
Send data to SPI device and receive response.

**Format:** `spi_transfer oid=%c data=%*s`

**Parameters:**
- `oid`: Object identifier
- `data`: Byte array to transmit

**Response:** `spi_transfer_response oid=%c response=%*s`

**Example:**
```
spi_transfer oid=5 data=\x12\x00
→ Response: spi_transfer_response oid=5 response=\x00\xFF
```

#### `spi_send`
Send data to SPI device without expecting a response (more efficient for write-only operations).

**Format:** `spi_send oid=%c data=%*s`

**Parameters:**
- `oid`: Object identifier
- `data`: Byte array to transmit

**Example:**
```
spi_send oid=5 data=\xFF\xAA\x55
```

### Safety Features

#### `config_spi_shutdown`
Configure a message to send during MCU shutdown (emergency stop).

**Format:** `config_spi_shutdown oid=%c spi_oid=%c shutdown_msg=%*s`

**Parameters:**
- `oid`: Shutdown object identifier
- `spi_oid`: SPI device object identifier
- `shutdown_msg`: Byte array to send on shutdown

**Example:**
```
config_spi_shutdown oid=10 spi_oid=5 shutdown_msg=\x00\x00\x00\x00
```

## Usage Workflow

Typical sequence for using an SPI device:

```
# 1. Configure SPI device with CS pin
config_spi oid=5 pin=17 cs_active_high=0

# 2. Set bus parameters (bus 2, mode 3, 4MHz)
spi_set_bus oid=5 spi_bus=2 mode=3 rate=4000000

# 3. Optional: Configure shutdown message for safety
config_spi_shutdown oid=10 spi_oid=5 shutdown_msg=\x00\x00

# 4. Perform transfers
spi_transfer oid=5 data=\x8B\x00  # Read WHO_AM_I register
→ Response: spi_transfer_response oid=5 response=\xE5\x00
```

## Implementation Details

### Hardware SPI (`targets/rp2040/spi.go`)

- Uses TinyGo's `machine.SPI` API
- Supports full-duplex transfers
- Configurable clock rate (up to 62.5 MHz on RP2040)
- All 4 SPI modes supported
- Automatic GPIO muxing for SPI function

### Software SPI (`targets/rp2040/spi_software.go`)

- Bit-banged implementation using GPIO
- Works on any GPIO pins
- Configurable timing based on requested rate
- Supports all 4 SPI modes
- Suitable for lower-speed devices (typically < 1 MHz)

### Chip Select Management

- Automatic CS assertion/deassertion around transfers
- Supports both active-low (default) and active-high CS
- CS pin configured as GPIO output
- For devices without CS, use `config_spi_without_cs`

### Thread Safety

- Mutex protection on bus configuration
- Safe concurrent access from multiple devices (different OIDs)
- Same bus can be shared by multiple devices with different CS pins

## Example: ADXL345 Accelerometer

Klipper configuration for ADXL345 on RP2040:

```ini
[adxl345]
cs_pin: rpi:gpio17
spi_bus: spi0c  # Bus 2: GPIO16(MISO), GPIO19(MOSI), GPIO18(SCK)
spi_speed: 4000000
```

This translates to the following commands:

```
config_spi oid=5 pin=17 cs_active_high=0
spi_set_bus oid=5 spi_bus=2 mode=3 rate=4000000
```

## Testing

### Hardware Required
- Raspberry Pi Pico or similar RP2040 board
- SPI device (e.g., ADXL345 accelerometer)
- Appropriate wiring for the selected bus

### Testing with Klipper Console

```bash
# Connect to MCU
~/klippy-env/bin/python ~/klipper/klippy/console.py -v /dev/ttyACM0

# Configure SPI device
SEND config_spi oid=5 pin=17 cs_active_high=0
SEND spi_set_bus oid=5 spi_bus=2 mode=3 rate=4000000

# Read ADXL345 WHO_AM_I register (should return 0xE5)
SEND spi_transfer oid=5 data="\x00\x00"
```

## Performance Characteristics

### Hardware SPI
- **Max Speed**: ~62.5 MHz (RP2040 peripheral limit)
- **Typical Speed**: 1-10 MHz for most devices
- **CPU Overhead**: Low (DMA capable)
- **Timing Accuracy**: Excellent

### Software SPI
- **Max Speed**: ~500 kHz (depends on CPU load)
- **Typical Speed**: 100-250 kHz
- **CPU Overhead**: High (busy-wait loops)
- **Timing Accuracy**: Good (subject to interrupts)

## Known Limitations

- Software SPI timing may be affected by interrupt latency
- Maximum SPI clock rate limited by peripheral and PCB design
- No DMA support yet (could be added for improved performance)
- CS pin must be held constant during multi-byte transfers

## Future Enhancements

- [ ] DMA support for hardware SPI
- [ ] Multi-byte transfer optimization
- [ ] SPI transaction queuing
- [ ] Enhanced error reporting
- [ ] Support for SPI slave mode

## References

- [Klipper SPI Implementation](https://github.com/Klipper3d/klipper/blob/master/src/spicmds.c)
- [RP2040 SPI Implementation](https://github.com/Klipper3d/klipper/blob/master/src/rp2040/spi.c)
- [TinyGo machine.SPI Documentation](https://tinygo.org/docs/reference/machine/spi/)
- [SPI Protocol Basics](https://en.wikipedia.org/wiki/Serial_Peripheral_Interface)
