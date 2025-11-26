
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gopper is a Klipper firmware implementation written in TinyGo for modern microcontrollers. It aims to provide full compatibility with the Klipper protocol while bringing Go's type safety and modern development practices to embedded 3D printer firmware.

**Status**: Early development, not production-ready.

## Current Status (2025-11)

### Working Features
- ✅ USB CDC communication on RP2040 (Raspberry Pi Pico)
- ✅ Klipper protocol handshake and identification
- ✅ Dictionary compression using custom TinyGo-compatible zlib
- ✅ Command registration and dispatch system
- ✅ VLQ encoding/decoding
- ✅ Basic command handlers (identify, get_uptime, get_clock, get_config)
- ✅ ADC (Analog-to-Digital Converter) support
- ✅ Stepper motor control (config_stepper, queue_step, set_next_step_dir, reset_step_clock)
- ✅ Position-based stepper moves (move_to_position command)
- ✅ TMC5240 stepper driver support (both step/dir and SPI hardware ramp modes)
- ✅ Stepper position reporting (stepper_get_position, stepper_position response)
- ✅ Trigger synchronization (trsync) for coordinated homing
- ✅ Endstop support for homing operations
- ✅ GPIO digital input/output
- ✅ PWM support

### Known Issues
- ⚠️ Stepper PIO backend not fully tested (GPIO fallback works)
- ⚠️ Timer system not fully tested under load
- ⚠️ Actual motor movement not yet verified (protocol communication confirmed working)

### Recent Bug Fixes (2025-11-12)
- **Fixed circular buffer wrap-around bug** causing timeouts after ~6 seconds of communication
  - `FifoBuffer.Data()` was only returning first segment when wrapped, causing message corruption
  - Added proper contiguous data copy for wrapped buffers
- **Fixed firmware restart hanging/crashing issue**
  - `FIRMWARE_RESTART` command was hanging because MCU reset occurred before ACK was sent
  - Root cause: `machine.CPUReset()` (ARM SYSRESETREQ) doesn't properly reset RP2040
  - **Solution**: Use RP2040 watchdog timer for reset instead
    - Watchdog reset properly resets all hardware including USB controller
    - Must call `machine.Watchdog.Start()` after configuring timeout
    - Must disable watchdog on boot to clear previous state
  - Deferred reset until after protocol ACK is transmitted to host
  - Added `resetPending` flag and `CheckPendingReset()` function
  - Klipper now properly receives ACK and reconnects after firmware restart
- **Implemented proper firmware restart with hardware reset**
  - Added `reset` command that triggers watchdog reset for complete MCU reset
  - Uses `machine.Watchdog` API with 1ms timeout for reliable RP2040 reset
  - Watchdog disabled on boot (`TimeoutMillis: 0`) to prevent issues after reset
  - Matches behavior of traditional Klipper firmwares (AVR, STM32)
  - Ensures all hardware peripherals are properly reset, not just software state
- Fixed TinyGo GC issue with compressed dictionary buffer (`bytes.Buffer` data being reclaimed)
- Fixed deadlock in dictionary `GetChunk()` function (RWMutex reentrancy issue)
- Added defensive copying to prevent memory corruption during USB transmission
- Improved USB write failure detection and buffer management
- ADC support now uses `machine.ADC` API instead of custom implementation

## Build Commands

```bash
# Build for RP2040 (Raspberry Pi Pico) - default target
make rp2040

# Build for STM32F4
make stm32f4

# Run tests (protocol and core packages)
make test

# Run tests for specific package
go test ./protocol/...
go test ./core/...

# Clean build artifacts
make clean
```

## Testing Commands

### Unit Tests

```bash
# Run all tests with verbose output
go test -v ./...

# Run tests for a single package
go test -v ./protocol

# Run a specific test
go test -v ./protocol -run TestDecodeVLQ
```

### Hardware Testing (RP2040)

```bash
# Flash firmware to RP2040
# 1. Hold BOOTSEL button and plug in USB
# 2. Copy firmware to mounted drive:
cp build/gopper-rp2040.uf2 /media/[user]/RPI-RP2/

# Test with Klipper console (requires Klipper installed)
~/klippy-env/bin/python ~/klipper/klippy/console.py -v /dev/ttyACM0

# Expected LED flash patterns on boot:
# - 5 flashes: Starting firmware
# - 3 flashes: JSON dictionary generation complete
# - 2 flashes: Compression successful
# - 4 flashes: USB reconnection detected (after disconnect)

# Monitor serial directly (for debugging)
screen /dev/ttyACM0 250000
# or
picocom /dev/ttyACM0 -b 250000
```

## Requirements

- **TinyGo**: 0.31.0 or later (for embedded builds)
- **Go**: 1.21 or later (for tests and development)

## TMC5240 Hardware Ramp Support

Gopper supports TMC5240 stepper drivers with hardware ramp generation, enabling smoother motion and reduced MCU load.

### SPI-Only Control

**Important:** TMC5240 is SPI-controlled only and does NOT support traditional step/dir input mode like TMC2209 or A4988. The TMC5240's strength is its internal motion controller accessed via SPI.

**How TMC5240 works:**
- TMC5240 internal motion controller generates steps autonomously
- MCU sends position targets via SPI
- Uses `move_to_position` command (position-based protocol)
- Hardware ramp generator handles acceleration/deceleration
- Requires Klipper host modifications (Python side) to send position commands

### Position-Based Move Protocol

New `move_to_position` command enables hardware ramp:
```
move_to_position oid=%c target_pos=%i start_vel=%u end_vel=%u accel=%u
```

**How it works:**
1. Host still does motion planning and kinematics
2. Host calculates target positions and velocities for each stepper
3. Host sends position-based moves instead of step counts
4. TMC5240 firmware programs hardware ramp registers
5. TMC5240 generates steps autonomously

**Backward compatibility:**
- Non-TMC backends automatically convert to step-based moves
- Existing `queue_step` command still works for all drivers
- No breaking changes to existing code

### Benefits
- Lower MCU load (no timer callbacks per step)
- Smoother motion (hardware-generated pulses, no jitter)
- Advanced features: StallGuard homing, load detection, thermal management
- Multi-stepper coordination maintained via `reset_step_clock`

### Usage Example

**Initialize TMC5240 in firmware:**
```go
// In targets/rp2040/main.go or similar
spi := machine.SPI0
spi.Configure(machine.SPIConfig{
    Frequency: 4000000, // 4 MHz (TMC5240 max is 4 MHz)
    Mode:      3,        // SPI Mode 3 for TMC5240
})

csPin := machine.Pin(17)
tmc5240 := core.NewTMC5240Backend(spi, csPin)

// Register as stepper backend
core.SetStepperBackendFactory(func() core.StepperBackend {
    return tmc5240
})
```

**From Klipper console:**
```python
# Position-based commands (native TMC5240 mode)
>>> config_stepper oid=0 step_pin=0 dir_pin=0 invert_step=0 step_pulse_ticks=0
>>> move_to_position oid=0 target_pos=10000 start_vel=5000 end_vel=8000 accel=500

# Traditional queue_step also works (automatically converted to position-based)
>>> queue_step oid=0 interval=5000 count=100 add=0
# This is internally converted to move_to_position by the firmware
```

### Future Enhancements
- Klipper host Python module for automatic TMC5240 detection and position-based commands
- StallGuard-based sensorless homing integration
- Real-time load monitoring and adaptive speed control
- S-curve acceleration support (TMC5240 hardware feature)
- Automatic velocity/acceleration calibration

## Architecture Overview

### Core System Design

Gopper uses a **command-scheduler-timer** architecture that mirrors Klipper's design for real-time operation:

1. **Protocol Layer** (`protocol/`): Handles Klipper wire protocol communication
   - Variable Length Quantity (VLQ) encoding/decoding for efficient parameter passing
   - Message blocks with sequence numbers and CRC checking
   - Compatible with Klipper's communication protocol

2. **Command System** (`core/command.go`): Command registration and dispatch
   - Commands are registered similar to Klipper's `DECL_COMMAND` macro
   - Each command has an ID, name, format string, and handler function
   - CommandRegistry maintains a dictionary sent to the host for command mapping

3. **Scheduler** (`core/scheduler.go`): Real-time event scheduling
   - Maintains a sorted linked list of Timer structs
   - Timers can be one-shot (`SF_DONE`) or rescheduling (`SF_RESCHEDULE`)
   - Uses interrupt disable/restore for thread-safe timer manipulation
   - Critical for real-time motion control accuracy

4. **Timer System** (`core/timer.go`): System time management
   - Default timer frequency: 12MHz (configurable per platform)
   - Provides microsecond-to-tick conversions for precise timing
   - Platform-specific initialization in `TimerInit()`

5. **Target Layer** (`targets/`): Platform-specific implementations
   - Each target (rp2040, stm32f4) has its own main entry point
   - RP2040: Uses USB CDC for communication (TinyGo's `machine.Serial`)
   - STM32F4: Uses UART at 250000 baud (Klipper standard)
   - Main loop: check serial input → process commands → run timers

### Key Concepts

**Interrupt-Safe Operations**: The scheduler uses `interrupt.Disable()` and `interrupt.Restore()` to ensure atomic timer list modifications, essential for real-time guarantees.

**Timer-Based Execution**: Unlike traditional firmware with blocking delays, Gopper schedules all operations as timer callbacks. This allows precise timing for stepper pulses and coordinated motion.

**VLQ Encoding**: Commands use VLQ encoding for compact parameter passing over the serial link, supporting both positive and negative integers efficiently.

**Command Dictionary**: The firmware sends a dictionary of available commands to the host at startup, allowing the host to map command names to IDs dynamically.

## Development Notes

### General Guidelines

- All embedded builds must use TinyGo, not standard Go
- The main loop in `targets/*/main.go` must never block for long periods
- Timer handlers should execute quickly and return `SF_DONE` or `SF_RESCHEDULE`
- Use build tags (e.g., `//go:build rp2040`) for platform-specific code
- Serial communication is at 250000 baud for UART, USB CDC for RP2040
- System time is tracked in timer ticks (default 12MHz), not wall-clock time

### TinyGo-Specific Considerations

TinyGo's garbage collector and runtime have different characteristics than standard Go:

1. **Memory Management**:
   - TinyGo uses a conservative mark-and-sweep GC
   - GC can be more aggressive in reclaiming memory
   - Always explicitly copy data from temporary buffers (like `bytes.Buffer.Bytes()`)
   - Example: `data := make([]byte, len(buf.Bytes())); copy(data, buf.Bytes())`

2. **Concurrency**:
   - Goroutines are supported but have different scheduling characteristics
   - Use `time.Sleep()` to yield control in busy loops
   - Atomic operations (`sync/atomic`) work but channels may behave differently

3. **Standard Library**:
   - Not all standard library packages are available
   - Some packages are partially implemented (e.g., `compress/zlib` doesn't work)
   - Check TinyGo compatibility before using new packages
   - Custom implementations may be needed (see `tinycompress/` package)

4. **Debugging**:
   - Printf debugging via USB/UART is primary debugging method
   - LED flash patterns can indicate firmware state
   - Use panic recovery to prevent complete firmware crashes
   - Memory corruption often manifests as intermittent crashes
