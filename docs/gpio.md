# GPIO Implementation in Gopper

This document describes the GPIO (General Purpose Input/Output) implementation in Gopper, which provides full compatibility with Klipper's digital output protocol.

## Overview

The GPIO implementation allows Klipper to control digital output pins on the microcontroller with precise timing. This is used for various purposes including:
- Heater control (on/off or PWM)
- Fan control (on/off or PWM)
- LED indicators
- Enable/disable signals for stepper drivers
- Any other digital control signals

## Architecture

The GPIO implementation follows the same pattern as other Gopper subsystems:

```
┌─────────────────────────────────────────┐
│  Klipper Host (Python)                  │
│  - Sends GPIO commands via protocol     │
└───────────────┬─────────────────────────┘
                │ USB/UART
┌───────────────▼─────────────────────────┐
│  Protocol Layer (protocol/)             │
│  - VLQ encoding/decoding                │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│  Core GPIO Logic (core/gpio.go)         │
│  - Command handlers                     │
│  - DigitalOut management                │
│  - Timer-based scheduling               │
│  - PWM generation                       │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│  GPIO HAL (core/gpio_hal.go)            │
│  - Abstract interface                   │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│  Platform Driver (targets/rp2040/)      │
│  - Hardware-specific pin control        │
└─────────────────────────────────────────┘
```

## Implementation Files

### Core Files

- **`core/gpio_hal.go`**: HAL interface definition
  - `GPIODriver` interface for platform abstraction
  - `ConfigureOutput()`, `SetPin()`, `GetPin()` methods

- **`core/gpio.go`**: Main GPIO implementation
  - `DigitalOut` struct for managing GPIO state
  - Command handlers: `config_digital_out`, `queue_digital_out`, `update_digital_out`, `set_digital_out_pwm_cycle`
  - Timer handlers for scheduled updates and PWM toggling
  - Safety features: max_duration enforcement, shutdown behavior

- **`core/gpio_test.go`**: Unit tests
  - Mock GPIO driver for testing
  - Basic functionality tests

### Platform-Specific Files

- **`targets/rp2040/gpio.go`**: RP2040/RP2350 GPIO driver
  - Uses TinyGo's `machine.Pin` API
  - Direct mapping of GPIO numbers to hardware pins

## Klipper Protocol Commands

### config_digital_out

Configures a GPIO pin as a digital output.

**Format**: `config_digital_out oid=%c pin=%u value=%c default_value=%c max_duration=%u`

**Parameters**:
- `oid`: Object ID (0-255) for this digital output
- `pin`: Hardware pin number (platform-specific)
- `value`: Initial value (0=low, 1=high)
- `default_value`: Default value for shutdown/emergency stop (0=low, 1=high)
- `max_duration`: Maximum time (in clock ticks) pin can remain in non-default state (0=unlimited)

**Example**: Configure GPIO25 (LED on Pico) as output, initially high, default low, no timeout
```
config_digital_out oid=1 pin=25 value=1 default_value=0 max_duration=0
```

### queue_digital_out

Schedules a pin state change at a specific time.

**Format**: `queue_digital_out oid=%c clock=%u on_ticks=%u`

**Parameters**:
- `oid`: Object ID of the digital output
- `clock`: System clock time to execute the change
- `on_ticks`: For simple on/off: 0=off, non-zero=on. For PWM: duration to keep pin high per cycle

**Example**: Turn on pin at clock time 1000000
```
queue_digital_out oid=1 clock=1000000 on_ticks=1
```

### update_digital_out

Immediately updates a pin value (not scheduled).

**Format**: `update_digital_out oid=%c value=%c`

**Parameters**:
- `oid`: Object ID of the digital output
- `value`: New value (0=low, 1=high)

**Example**: Immediately turn off the pin
```
update_digital_out oid=1 value=0
```

### set_digital_out_pwm_cycle

Configures software PWM cycle time.

**Format**: `set_digital_out_pwm_cycle oid=%c cycle_ticks=%u`

**Parameters**:
- `oid`: Object ID of the digital output
- `cycle_ticks`: Total PWM cycle duration in clock ticks (recommended ≥10ms)

**Example**: Set 10ms PWM cycle (at 12MHz: 120,000 ticks)
```
set_digital_out_pwm_cycle oid=1 cycle_ticks=120000
```

After setting a PWM cycle, `queue_digital_out` interprets `on_ticks` as the duration to keep the pin high per cycle, enabling PWM control.

## Features

### Timer-Based Scheduling

All GPIO operations can be precisely scheduled using the system timer:
- `queue_digital_out` schedules changes at specific clock times
- Sub-millisecond precision (12MHz timer = 83.3ns resolution)
- Non-blocking operation

### Software PWM

The implementation supports software PWM for analog-like control:
1. Set PWM cycle time with `set_digital_out_pwm_cycle`
2. Use `queue_digital_out` to set duty cycle (on_ticks / cycle_ticks)
3. The timer system automatically toggles the pin at the correct intervals

**Example**: 50% duty cycle PWM at 100Hz
```
# Set 10ms cycle (100Hz)
set_digital_out_pwm_cycle oid=1 cycle_ticks=120000

# Set 50% duty cycle (5ms on, 5ms off)
queue_digital_out oid=1 clock=1000000 on_ticks=60000
```

### Max Duration Safety

The `max_duration` parameter prevents pins from staying in non-default states indefinitely:
- If non-zero, limits how long a pin can differ from its default value
- Automatically returns to default state when time expires
- Provides safety for heaters, fans, and other critical outputs
- Must be explicitly renewed by scheduling new updates

### Shutdown Behavior

On emergency stop or firmware shutdown:
- All GPIO pins return to their default states
- PWM toggling is stopped
- Timer scheduling is halted
- Called from `handleEmergencyStop()` and `TryShutdown()`

## Internal State Management

### DigitalOut Struct

Each configured GPIO output has a `DigitalOut` instance tracking:

```go
type DigitalOut struct {
    OID         uint8   // Object ID
    Pin         GPIOPin // Hardware pin number
    Flags       uint8   // State flags (DF_*)

    Timer       Timer   // Scheduled event timer

    OnDuration  uint32  // PWM on time
    OffDuration uint32  // PWM off time
    CycleTime   uint32  // PWM cycle time
    EndTime     uint32  // Max duration expiry time

    MaxDuration uint32  // Max time in non-default state
}
```

### State Flags

- `DF_ON` (0x01): Current pin state (1=high, 0=low)
- `DF_TOGGLING` (0x02): PWM mode active
- `DF_CHECK_END` (0x04): Monitor max_duration
- `DF_DEFAULT_ON` (0x08): Default state for shutdown

### Timer Handlers

Three timer handlers manage GPIO operations:

1. **`digitalOutLoadEvent`**: Executes scheduled updates
   - Sets pin to new state
   - Starts PWM toggling if configured
   - Schedules max_duration enforcement

2. **`digitalOutToggleEvent`**: Handles PWM toggling
   - Alternates pin between high and low
   - Calculates next toggle time
   - Continues until toggling disabled or max_duration reached

3. **`digitalOutEndEvent`**: Enforces max_duration
   - Returns pin to default state
   - Stops PWM toggling
   - Clears DF_CHECK_END flag

## Platform Integration (RP2040)

The RP2040 implementation uses TinyGo's hardware abstraction:

```go
type RPGPIODriver struct {
    configuredPins map[core.GPIOPin]machine.Pin
}
```

**Pin Mapping**: GPIO numbers map directly to RP2040 pins (GPIO0 = Pin 0, GPIO25 = Pin 25, etc.)

**Configuration**: Pins are configured as outputs using `machine.PinOutput`

**Control**: Uses `Pin.Set(bool)` and `Pin.Get()` for state manipulation

## Usage Example

Complete example of configuring and using a GPIO pin:

```python
# In Klipper config (handled by host)
[output_pin my_led]
pin: gpio25
pwm: True
cycle_time: 0.010
value: 0
shutdown_value: 0
```

This generates the following MCU commands:

```
# 1. Configure pin
config_digital_out oid=1 pin=25 value=0 default_value=0 max_duration=0

# 2. Set PWM cycle to 10ms (120,000 ticks @ 12MHz)
set_digital_out_pwm_cycle oid=1 cycle_ticks=120000

# 3. Set to 25% brightness (3,000 ticks on, 117,000 ticks off)
queue_digital_out oid=1 clock=1000000 on_ticks=30000

# 4. Later, turn off
update_digital_out oid=1 value=0
```

## Testing

### Unit Tests

Run GPIO tests:
```bash
go test -v ./core/gpio_test.go ./core/gpio.go ./core/gpio_hal.go
```

### Hardware Testing

1. Flash firmware with GPIO support
2. Use Klipper console to send commands
3. Observe pin behavior with LED or oscilloscope

Example Klipper console test:
```python
# Connect to MCU
~/klippy-env/bin/python ~/klipper/klippy/console.py -v /dev/ttyACM0

# Send commands (after configuration)
# Blink LED on GPIO25
update_digital_out oid=1 value=1
# Wait...
update_digital_out oid=1 value=0
```

## Performance Considerations

### Timer Overhead

Each GPIO requires timer resources:
- Simple on/off: 1 timer event per scheduled update
- PWM: 2 timer events per cycle (on→off, off→on)
- Max duration: 1 additional timer event

### Memory Usage

Per configured GPIO:
- `DigitalOut` struct: ~48 bytes
- Map entry overhead: ~16 bytes
- Total: ~64 bytes per pin

### Timing Accuracy

- Timer resolution: 83.3ns @ 12MHz
- Software overhead: ~5-10µs per toggle
- PWM frequency limits: Practical max ~10kHz, recommended max ~1kHz

## Future Enhancements

Possible improvements:
- [ ] Hardware PWM support for higher frequencies
- [ ] GPIO input support (digital_in)
- [ ] Interrupt-based edge detection
- [ ] Pin conflict detection
- [ ] GPIO grouping for simultaneous updates

## References

- [Klipper MCU Commands](https://www.klipper3d.org/MCU_Commands.html)
- [Klipper gpiocmds.c](https://github.com/Klipper3d/klipper/blob/master/src/gpiocmds.c)
- [TinyGo machine package](https://tinygo.org/docs/reference/microcontrollers/machine/)
