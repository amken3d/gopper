# RP2350 Target Status

## Working Features ✅

**Firmware size:** 47,344 bytes flash, 7,644 bytes RAM

### Fully Functional:
- **Core commands** (identify, get_uptime, get_clock, finalize_config, reset, etc.)
- **ADC support** (analog inputs, 5 channels: ADC0-ADC3, ADC_TEMPERATURE)
- **GPIO support** (digital I/O)
  - **48 GPIO pins** (gpio0-gpio47) - Full RP2350 pin count
  - Pull-up/pull-down configuration
  - Digital read/write
- **PWM support** (hardware PWM output)
- **SPI support**
  - Hardware SPI (2 controllers with multiple pin configurations)
  - Software SPI (bit-banging fallback)
- **I2C support** (hardware I2C communication)
- **Dictionary compression** (zlib compressed, works with basic command set)

## Known Limitations ⚠️

### 1. Dictionary Compression Crash
**Issue:** When endstop commands are added, the compressed dictionary crashes at offset 240 during transmission to Klipper.

**Details:**
- Successfully transmits first 240 bytes (6 chunks of 40 bytes)
- Crashes when attempting to read chunk at offset 240
- Compression works fine with Core+ADC+GPIO+PWM+SPI+I2C commands
- Adding any endstop command triggers the crash

**Suspected Cause:**
- Possible bug in `tinycompress` zlib library on RP2350 with larger input data
- Memory corruption during compression of larger dictionaries
- Buffer overflow when compressed size exceeds certain threshold

**Status:** Under investigation

### 2. Endstops Not Available
**Issue:** Endstop commands crash the firmware due to dictionary compression issue above.

**Disabled Commands:**
- `core.InitEndstopCommands()` - Digital endstops
- `core.InitAnalogEndstopCommands()` - Analog endstops
- `core.InitI2CEndstopCommands()` - I2C-based endstops
- `core.InitTriggerSyncCommands()` - Trigger synchronization for homing

**Workaround:** None currently. Endstops cannot be used on RP2350.

### 3. Steppers Not Available
**Issue:** Both PIO stepper backend and stepper command registration cause crashes.

**Details:**
- `piostepper.InitSteppers()` crashes during initialization
- `core.RegisterStepperCommands()` also causes crashes when added alone
- May be related to dictionary size/compression issue

**Disabled:**
- PIO-based stepper control
- All stepper commands (config_stepper, queue_step, etc.)

**Status:** Steppers cannot be used on RP2350 currently.

## Comparison with RP2040

| Feature | RP2040 | RP2350 |
|---------|--------|--------|
| Core Commands | ✅ | ✅ |
| ADC | ✅ (3 pins) | ✅ (4 pins) |
| GPIO Pins | ✅ (30 pins) | ✅ (48 pins) |
| PWM | ✅ | ✅ |
| SPI | ✅ | ✅ |
| I2C | ✅ | ✅ |
| Endstops | ✅ | ❌ |
| Trigger Sync | ✅ | ❌ |
| Steppers | ✅ | ❌ |
| Dictionary Compression | ✅ | ⚠️ (Limited) |

## Use Cases

### ✅ Supported:
- Basic GPIO control (LEDs, relays, etc.)
- Analog input reading (temperature sensors, voltage monitoring)
- PWM output (fans, heaters)
- SPI peripherals (displays, sensors)
- I2C peripherals (sensors, expanders)

### ❌ Not Supported:
- 3D printer motion control (no steppers)
- Homing operations (no endstops/trigger sync)
- CNC operations (no steppers)

## Build Instructions

```bash
# For Crea8 board (RP2350B)
make crea8

# For generic RP2350 boards
make rp2350

# For Pico2 board
make rp2350-pico2
```

## Flash Instructions

```bash
# Hold BOOTSEL button and plug in USB
# Copy firmware to mounted drive
cp build/gopper-crea8.uf2 /media/$USER/RPI-RP2/

# Test with Klipper console
~/klippy-env/bin/python ~/klipper/klippy/console.py /dev/ttyACM0
```

## Testing with Klipper

### Minimal printer.cfg for RP2350:

```ini
[mcu]
serial: /dev/ttyACM0

# GPIO digital output example
[output_pin led]
pin: gpio25
value: 0

# PWM output example (fan)
[fan]
pin: gpio15

# ADC input example (temperature)
[adc_temperature my_sensor]
sensor_type: Generic 3950
sensor_pin: ADC0
min_temp: 0
max_temp: 100
```

**Note:** Do NOT configure steppers or endstops - they are not supported on RP2350 currently.

## Technical Details

### Pin Enumeration
- **GPIO pins:** Indices 0-47 (gpio0-gpio47)
- **ADC channels:** Indices 48-52
  - 48: ADC0
  - 49: ADC1
  - 50: ADC2
  - 51: ADC3
  - 52: ADC_TEMPERATURE

### MCU Identification
- MCU type: "rp2350"
- Clock frequency: 1 MHz (timer)
- USB CDC serial communication at 250000 baud

## Next Steps / TODO

1. **Investigate tinycompress crash:**
   - Debug why zlib compression fails with larger dictionaries on RP2350
   - Test with different compression levels
   - Consider alternative compression library

2. **Fix PIO stepper support:**
   - Determine RP2350 PIO differences from RP2040
   - Test if PIO initialization needs different approach
   - Verify state machine allocation works on RP2350

3. **Enable endstops:**
   - Once compression issue resolved, re-enable endstop commands
   - Test homing operations

4. **Full feature parity with RP2040:**
   - Get steppers working
   - Enable all endstop types
   - Support trigger synchronization

## Development Notes

- Build tags properly isolate RP2040 and RP2350 code (`//go:build rp2040` vs `//go:build rp2350`)
- RP2350 shares most peripheral drivers with RP2040 (PWM, SPI, I2C, ADC, GPIO)
- Main differences are in pin count and currently the dictionary compression behavior
- Watchdog timer used for firmware reset (more reliable than ARM SYSRESETREQ on RP2350)

## Version History

- **2025-11-27:** Initial RP2350 target created
  - Basic peripherals working (GPIO, ADC, PWM, SPI, I2C)
  - 48-pin support implemented
  - Identified compression crash with endstops/steppers
  - Created stable release without endstops/steppers
