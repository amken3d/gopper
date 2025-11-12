
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

### Known Issues
- ⚠️ Connection reliability: Dictionary transfer sometimes fails partway through
  - Intermittent crashes at various offsets during identify_response transmission
  - Likely memory corruption or race condition in USB handling
  - Reconnection after Ctrl+C does not always work cleanly
- ⚠️ USB disconnect detection needs improvement
- ⚠️ No stepper motor control implemented yet
- ⚠️ Timer system not fully tested under load

### Recent Bug Fixes
- Fixed TinyGo GC issue with compressed dictionary buffer (`bytes.Buffer` data being reclaimed)
- Fixed deadlock in dictionary `GetChunk()` function (RWMutex reentrancy issue)
- Added defensive copying to prevent memory corruption during USB transmission
- Improved USB write failure detection and buffer management

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
