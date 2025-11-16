# PWM Implementation in Gopper

This document describes the hardware PWM (Pulse Width Modulation) implementation in Gopper, which provides compatibility with Klipper's `pwm_out` protocol for efficient analog-like control.

## Overview

The PWM implementation allows Klipper to control analog outputs (fans, heaters, LEDs, etc.) using hardware PWM peripherals. This provides more efficient and precise control compared to software PWM (available through GPIO), especially for high-frequency applications.

**Key Differences from GPIO Software PWM**:
- **Hardware PWM** (this implementation): Uses dedicated PWM peripherals, higher frequency capability (up to 125 MHz), lower CPU overhead
- **GPIO Software PWM** (see [gpio.md](gpio.md)): Timer-based toggling, more flexible but limited to ~1kHz practical maximum

## Architecture

The PWM implementation follows Gopper's modular architecture:

```
┌─────────────────────────────────────────┐
│  Klipper Host (Python)                  │
│  - Sends PWM commands via protocol      │
└───────────────┬─────────────────────────┘
                │ USB/UART
┌───────────────▼─────────────────────────┐
│  Protocol Layer (protocol/)             │
│  - VLQ encoding/decoding                │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│  Core PWM Logic (core/pwm.go)           │
│  - Command handlers                     │
│  - HardwarePWM management               │
│  - Timer-based scheduling               │
│  - Max duration safety                  │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│  PWM HAL (core/pwm_hal.go)              │
│  - Abstract PWMDriver interface         │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│  Platform Driver (targets/rp2040/)      │
│  - RP2040PWMDriver                      │
│  - Hardware PWM slice management        │
└─────────────────────────────────────────┘
```

## Implementation Files

### Core Files

- **`core/pwm_hal.go`**: HAL interface definition
  - `PWMDriver` interface for platform abstraction
  - `ConfigureHardwarePWM()`, `SetDutyCycle()`, `GetMaxValue()`, `DisablePWM()` methods
  - Type definitions: `PWMPin`, `PWMValue`

- **`core/pwm.go`**: Main PWM implementation
  - `HardwarePWM` struct for managing PWM state
  - Command handlers: `config_pwm_out`, `queue_pwm_out`, `set_pwm_out`
  - Timer handlers for scheduled updates and max_duration enforcement
  - Safety features: max_duration monitoring, shutdown behavior

### Platform-Specific Files

- **`targets/rp2040/pwm.go`**: RP2040/RP2350 hardware PWM driver
  - `RP2040PWMDriver` implementation
  - 8 PWM slices with 2 channels each (16 total PWM outputs)
  - Uses TinyGo's `machine.PWM` API
  - Automatic pin-to-slice mapping

### Test Files

- **`test/pwm/`**: Standalone PWM test program
  - Isolated driver testing without Klipper protocol
  - Tests basic configuration, duty cycle sweeps, and continuous patterns
  - Uses built-in LED for visual feedback

## Klipper Protocol Commands

### config_pwm_out

Configures a pin for hardware PWM output.

**Format**: `config_pwm_out oid=%c pin=%u cycle_ticks=%u value=%hu default_value=%hu max_duration=%u`

**Parameters**:
- `oid`: Object ID (0-255) for this PWM output
- `pin`: Hardware pin number (must support PWM)
- `cycle_ticks`: PWM period in timer ticks (determines frequency)
- `value`: Initial duty cycle (0-255, where 0=0%, 255=100%)
- `default_value`: Default duty cycle for shutdown/emergency stop
- `max_duration`: Maximum time (in clock ticks) pin can remain in non-default state (0=unlimited)

**Example**: Configure GPIO25 (LED on Pico) for 1kHz PWM at 50% duty cycle
```
# At 12MHz timer: 1kHz = 12,000 ticks/cycle
config_pwm_out oid=1 pin=25 cycle_ticks=12000 value=127 default_value=0 max_duration=0
```

**Notes**:
- Frequency calculation: `frequency = timer_freq / cycle_ticks`
- RP2040 timer frequency: 12 MHz (83.3ns resolution)
- Pins sharing a PWM slice must use the same frequency

### queue_pwm_out

Schedules a PWM duty cycle change at a specific time.

**Format**: `queue_pwm_out oid=%c clock=%u value=%hu`

**Parameters**:
- `oid`: Object ID of the PWM output
- `clock`: System clock time to execute the change
- `value`: New duty cycle (0-255)

**Example**: Set to 75% duty cycle at clock time 1000000
```
queue_pwm_out oid=1 clock=1000000 value=191
```

**Notes**:
- Allows precise timing control for synchronized operations
- If `max_duration` is set, it's reset when value differs from default

### set_pwm_out

Immediately updates the PWM duty cycle (not scheduled).

**Format**: `set_pwm_out oid=%c value=%hu`

**Parameters**:
- `oid`: Object ID of the PWM output
- `value`: New duty cycle (0-255)

**Example**: Immediately set to 25% duty cycle
```
set_pwm_out oid=1 value=63
```

## RP2040 Hardware PWM Architecture

### PWM Slices and Channels

The RP2040 has **8 PWM slices** (PWM0-PWM7), each with **2 channels** (A and B):

```
PWM Slice 0:  Channel A (even pins: 0, 16)   Channel B (odd pins: 1, 17)
PWM Slice 1:  Channel A (even pins: 2, 18)   Channel B (odd pins: 3, 19)
PWM Slice 2:  Channel A (even pins: 4, 20)   Channel B (odd pins: 5, 21)
PWM Slice 3:  Channel A (even pins: 6, 22)   Channel B (odd pins: 7, 23)
PWM Slice 4:  Channel A (even pins: 8, 24)   Channel B (odd pins: 9, 25)  ← LED
PWM Slice 5:  Channel A (even pins: 10, 26)  Channel B (odd pins: 11, 27)
PWM Slice 6:  Channel A (even pins: 12, 28)  Channel B (odd pins: 13, 29)
PWM Slice 7:  Channel A (even pins: 14)      Channel B (odd pins: 15)
```

**Pin Mapping Formula**:
- Slice number: `(pin >> 1) & 0x7` (divide by 2, modulo 8)
- Channel: `pin & 1` (0=A for even pins, 1=B for odd pins)

**Example**: GPIO 25 (built-in LED)
- Slice: `(25 >> 1) & 0x7` = `12 & 0x7` = **4**
- Channel: `25 & 1` = **1** (B)
- Result: **PWM Slice 4, Channel B**

### Frequency and Period

PWM frequency is determined by:
```
frequency = timer_freq / cycle_ticks
```

Where `timer_freq` = 12 MHz (configurable in `core/timer.go`)

**Common Frequencies**:
- 1 kHz (fans, LEDs): `cycle_ticks = 12000`
- 10 kHz (heaters): `cycle_ticks = 1200`
- 100 Hz (servos): `cycle_ticks = 120000`

**Hardware Limits**:
- Minimum frequency: ~183 Hz (max cycle_ticks = 65535)
- Maximum frequency: 125 MHz (RP2040 system clock)
- Practical range: 100 Hz - 100 kHz

### Duty Cycle Resolution

The duty cycle is specified as 0-255 (8-bit) for Klipper compatibility, but internally converted to the hardware's counter resolution:

```go
// Convert 0-255 to hardware counter value
dutyCycle = (value * top) / 255
```

Where `top` is the hardware counter maximum (varies with frequency).

## Features

### Hardware PWM Benefits

1. **High Frequency**: Up to 125 MHz (vs ~1kHz for software PWM)
2. **Low CPU Overhead**: Hardware handles toggling
3. **Precise Duty Cycle**: Hardware counter resolution
4. **Independent Operation**: Runs without CPU intervention

### Timer-Based Scheduling

PWM updates can be precisely scheduled:
- `queue_pwm_out` schedules changes at specific clock times
- Sub-millisecond precision (12MHz timer = 83.3ns resolution)
- Non-blocking operation
- Integrated with Gopper's timer system

### Max Duration Safety

The `max_duration` parameter prevents outputs from staying in non-default states indefinitely:

```
config_pwm_out oid=1 pin=25 cycle_ticks=12000 value=255 default_value=0 max_duration=120000
```

- After 120,000 ticks (~10ms @ 12MHz) at non-default value, automatically returns to default
- Must be explicitly renewed by scheduling new updates
- Provides safety for heaters, fans, and other outputs
- Zero means no timeout (runs indefinitely)

**Workflow**:
1. Set PWM to non-default value → max_duration timer starts
2. Timer expires → PWM returns to default_value
3. Flag `PWM_CHECK_END` cleared

### Shutdown Behavior

On emergency stop or firmware shutdown:
- All PWM outputs return to their default duty cycles
- Timer scheduling is halted
- Called from `ShutdownAllHardwarePWM()`

## Internal State Management

### HardwarePWM Struct

Each configured PWM output has a `HardwarePWM` instance:

```go
type HardwarePWM struct {
    OID   uint8  // Object ID
    Pin   PWMPin // Hardware pin number
    Flags uint8  // State flags

    Timer Timer // For scheduled updates and max_duration

    CycleTicks uint32   // PWM period in ticks
    Value      PWMValue // Current duty cycle (0-255)

    DefaultValue PWMValue // Default for shutdown
    MaxDuration  uint32   // Max time in non-default state
    EndTime      uint32   // When max_duration expires
}
```

### State Flags

- `PWM_CHECK_END` (0x01): Monitor max_duration

### Timer Handlers

Two timer handlers manage PWM operations:

1. **`pwmLoadEvent`**: Executes scheduled duty cycle updates
   - Applies new PWM value to hardware
   - Starts max_duration monitoring if needed
   - Schedules `pwmEndEvent` if `PWM_CHECK_END` is set

2. **`pwmEndEvent`**: Enforces max_duration
   - Returns PWM to default value
   - Clears `PWM_CHECK_END` flag

## Platform Integration (RP2040)

The RP2040 implementation manages hardware PWM slices:

```go
type RP2040PWMDriver struct {
    slices      map[uint8]uint64      // Slice -> period (ns)
    channels    map[uint32]uint8      // Pin -> channel
    peripherals map[uint8]pwmPeripheral // Slice -> PWM peripheral
}
```

### Key Methods

**ConfigureHardwarePWM**:
1. Calculate slice and channel from pin number
2. Convert cycle_ticks to nanoseconds
3. Configure PWM peripheral with period
4. Get and store channel mapping
5. Return actual cycle_ticks (may be adjusted by hardware)

**SetDutyCycle**:
1. Lookup channel for pin
2. Get PWM peripheral for slice
3. Scale 0-255 value to hardware counter range
4. Set hardware duty cycle

**getPWMPeripheral**:
- Returns appropriate `machine.PWMx` peripheral for slice number
- RP2040 has PWM0-PWM7

## Usage Example

### Klipper Configuration

```ini
[output_pin fan]
pin: gpio25
pwm: True
hardware_pwm: True
cycle_time: 0.001  # 1kHz
value: 0
shutdown_value: 0
```

### Generated MCU Commands

```
# 1. Configure PWM at 1kHz, initially off
config_pwm_out oid=1 pin=25 cycle_ticks=12000 value=0 default_value=0 max_duration=0

# 2. Set to 50% speed
set_pwm_out oid=1 value=127

# 3. Schedule ramp to 100% at future time
queue_pwm_out oid=1 clock=5000000 value=255

# 4. Turn off immediately
set_pwm_out oid=1 value=0
```

### Console Testing

```bash
# Connect to MCU
~/klippy-env/bin/python ~/klipper/klippy/console.py -v /dev/ttyACM0

# Test PWM on GPIO25 (LED)
config_pwm_out oid=0 pin=25 cycle_ticks=12000 value=127 default_value=0 max_duration=0

# Observe LED at 50% brightness

# Change to 25%
set_pwm_out oid=0 value=63

# Full brightness
set_pwm_out oid=0 value=255

# Off
set_pwm_out oid=0 value=0
```

## Testing

### Standalone PWM Test

A dedicated test program validates the PWM driver in isolation:

```bash
# Build test
make test-pwm

# Flash to Pico (hold BOOTSEL, plug USB)
cp build/pwm-test-rp2040.uf2 /media/[user]/RPI-RP2/
```

**Test Sequence**:
1. **Basic Config** (2 sec): LED at 50% brightness
2. **Sweep** (2 sec): Gradual brighten then dim
3. **Continuous Patterns**: Breathing, stepping, rapid blinking

See [test/pwm/README.md](../test/pwm/README.md) for details.

### Hardware Requirements

- **No external hardware needed**: Uses built-in LED (GPIO25)
- **Optional**: Oscilloscope on GPIO25 to verify 1kHz signal
- **Alternative test pin**: Any GPIO with PWM capability

## Pin Compatibility

### RP2040 (Raspberry Pi Pico)

**PWM-capable pins**: GPIO 0-29 (all GPIO pins support PWM)

**Common uses**:
- **GPIO 25**: Built-in LED (PWM4B) - great for testing
- **GPIO 15**: Pin 20 on Pico (PWM7B)
- **GPIO 16-17**: Slice 0 - often used for fans
- **GPIO 20-21**: Slice 2 - heater control

### PWM Slice Sharing

**Important**: Pins sharing a PWM slice **must use the same frequency**:
- GPIO 0 and GPIO 1 share PWM Slice 0
- GPIO 2 and GPIO 3 share PWM Slice 1
- ... and so on

**Example conflict**:
```
# This won't work well - both on same slice!
config_pwm_out oid=0 pin=0 cycle_ticks=12000  # 1kHz
config_pwm_out oid=1 pin=1 cycle_ticks=1200   # 10kHz (reconfigures slice!)
```

**Solution**: Use pins on different slices:
```
config_pwm_out oid=0 pin=0 cycle_ticks=12000  # 1kHz on Slice 0
config_pwm_out oid=1 pin=2 cycle_ticks=1200   # 10kHz on Slice 1
```

## Performance Considerations

### Memory Usage

Per configured PWM:
- `HardwarePWM` struct: ~48 bytes
- Map entry overhead: ~16 bytes
- Total: ~64 bytes per PWM output

### CPU Overhead

- **Configuration**: One-time setup cost
- **Duty cycle updates**: Minimal (hardware register write)
- **Scheduled updates**: One timer event per `queue_pwm_out`
- **Max duration**: One timer event per timeout

### Timing Accuracy

- **Hardware PWM**: Extremely precise, no jitter
- **Duty cycle resolution**: Depends on frequency (higher freq = lower resolution)
- **Scheduling precision**: 83.3ns @ 12MHz timer

### Frequency vs Resolution Trade-off

Higher PWM frequency = lower duty cycle resolution:
- **1 kHz**: `top = 12000` → high resolution (256 effective levels)
- **10 kHz**: `top = 1200` → medium resolution (256 → ~256 levels)
- **100 kHz**: `top = 120` → lower resolution (256 → ~120 levels)

## Troubleshooting

### PWM Not Working

**Check**:
1. Pin supports PWM (all RP2040 GPIO do)
2. Correct pin number in command
3. PWM driver initialized: `core.SetPWMDriver(pwmDriver)`
4. Commands registered: `core.InitPWMCommands()`

### Unexpected Frequency

**Causes**:
- Multiple pins on same slice configured with different cycle_ticks
- Last configuration wins for the shared slice

**Solution**: Verify pin mapping and use different slices

### Visible Flickering

**Causes**:
- PWM frequency too low (< 100 Hz for LEDs)
- `cycle_ticks` too large

**Solution**: Increase frequency (reduce cycle_ticks)

### Max Duration Not Working

**Check**:
1. `max_duration` > 0 in `config_pwm_out`
2. Value differs from `default_value`
3. Timer system operational

## Comparison: Hardware PWM vs GPIO Software PWM

| Feature | Hardware PWM | GPIO Software PWM |
|---------|-------------|-------------------|
| **Max Frequency** | 125 MHz | ~1 kHz |
| **CPU Overhead** | Minimal | High at high frequencies |
| **Precision** | Hardware counter | Timer resolution |
| **Pin Flexibility** | All GPIO pins | All GPIO pins |
| **Slice Sharing** | Yes (same freq required) | N/A |
| **Best For** | High-freq, efficient | Legacy, simple control |

**When to use Hardware PWM**:
- Fans requiring > 1kHz frequency
- Efficient heater control
- Multiple PWM outputs
- Precise duty cycle control

**When to use GPIO Software PWM**:
- Frequency < 1kHz is sufficient
- Need different frequencies on adjacent pins
- Simpler configuration

## Future Enhancements

Possible improvements:
- [ ] Automatic slice conflict detection
- [ ] Phase-aligned PWM for multiple outputs
- [ ] Dead-time insertion for H-bridge control
- [ ] PWM capture for frequency measurement
- [ ] Dynamic frequency adjustment

## References

- [Klipper MCU Commands](https://www.klipper3d.org/MCU_Commands.html)
- [Klipper pwmcmds.c](https://github.com/Klipper3d/klipper/blob/master/src/pwmcmds.c)
- [RP2040 Datasheet - PWM](https://datasheets.raspberrypi.com/rp2040/rp2040-datasheet.pdf#page=542)
- [TinyGo machine.PWM](https://tinygo.org/docs/reference/microcontrollers/machine/pwm/)
- [GPIO Software PWM Documentation](gpio.md)