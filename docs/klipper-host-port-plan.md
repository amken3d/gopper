# Klipper Host Software Port to Go - Implementation Plan

**Created**: 2025-11-25
**Status**: Planning Phase
**Target**: Port Klipper's Python host software (klippy) to Go (Standard Go or TinyGo WASM)

---

## Executive Summary

This document outlines a comprehensive plan to port the Klipper host software (klippy) from Python to Go. The existing Gopper project provides a significant head start with its complete Klipper protocol implementation in Go, which can be directly reused for the host-side implementation.

**Recommendation**: Start with **Standard Go** for native host implementation, then add **TinyGo WASM** for browser-based control as a secondary target.

---

## 1. Architecture Analysis: Klipper Host (klippy)

### 1.1 Core Components to Port

Based on the Klipper codebase analysis, the klippy host software consists of:

#### **Main Orchestration**
- **klippy.py**: Entry point, config parsing, printer object instantiation
- **reactor.py**: Event loop for I/O and timer callbacks (Python greenlet-based)
- **configfile.py**: Configuration file parsing (INI-like format)

#### **MCU Communication**
- **mcu.py**: MCU interface and command/response management
- **serialhdl.py**: Serial communication handler (runs in dedicated thread)
- **msgproto.py**: Message protocol (command encoding/decoding)
- **clocksync.py**: Clock synchronization between host and MCU
- **chelper/serialqueue.c**: C helper for low-latency serial I/O and message buffering

#### **Motion System**
- **toolhead.py**: Motion planning, lookahead, and timing coordination
- **trapq.py**: Trapezoidal motion queue
- **chelper/kin_*.c**: C helpers for kinematics calculations
- **chelper/itersolve.c**: Iterative kinematic solver
- **kinematics/**: Robot kinematics implementations
  - `cartesian.py`, `corexy.py`, `delta.py`, `polar.py`, etc.

#### **G-code Processing**
- **gcode.py**: G-code parser and command dispatcher
- **extras/gcode_move.py**: G1/G0 movement commands
- **extras/gcode_macro.py**: Macro support
- **extras/virtual_sdcard.py**: SD card file printing

#### **Hardware Modules** (extras/)
- Temperature sensors (thermistors, MAX31855, etc.)
- Heaters and PID control
- Fans, LEDs, servos
- Bed leveling (bed_mesh, bltouch, probe)
- Display support (LCD, TFT)
- Input shaping for resonance compensation

#### **Threading Model**
Klipper uses 4 threads:
1. **Main thread**: Reactor event loop and G-code processing
2. **Serial I/O thread**: Low-level serial port reading/writing (C code)
3. **Response handler thread**: Process MCU responses (Python)
4. **Logger thread**: Async log writing

---

## 2. Technology Choice: TinyGo WASM vs Standard Go

### 2.1 Comparison Matrix

| Aspect | Standard Go | TinyGo WASM |
|--------|-------------|-------------|
| **Binary Size** | ~10-20 MB native | ~400 KB (optimized) |
| **Garbage Collection** | Excellent (concurrent, low-latency) | Poor (naive, 1+ second pauses) |
| **Memory Allocation** | Fast | Very slow for large allocations |
| **Language Support** | 100% (full stdlib) | ~95% (missing some stdlib) |
| **Concurrency** | Full goroutines, channels | Limited (no recover in WASM) |
| **Performance** | Native speed | Near-native (WASM overhead) |
| **Deployment** | Native binary, requires OS | Browser-based, no install |
| **Serial Access** | Direct (github.com/tarm/serial) | Via WebSerial API (limited) |
| **File System** | Full OS access | Browser sandboxed |
| **Dependencies** | All Go packages | Limited (no cgo, no net) |
| **Maturity** | Production-ready | Experimental for complex apps |

### 2.2 Recommended Approach: Dual Strategy

**Phase 1: Standard Go (Primary Target)**
- Build full-featured host software as native Go binary
- Direct serial port access for reliability
- Full motion planning with standard Go's excellent GC
- Can run on Raspberry Pi, x86 Linux, macOS, Windows
- Use existing ecosystem (serial libs, web servers, etc.)

**Phase 2: TinyGo WASM (Secondary Target)**
- Browser-based UI for monitoring and basic control
- WebSerial API for direct browser-to-MCU communication
- Offload heavy processing to server or accept limited functionality
- Leverage existing Gopper `ui/wasm/` work

**Why this order?**
- TinyGo's GC limitations make real-time motion planning problematic
- Standard Go provides better performance for lookahead and kinematics
- WASM is excellent for UI but challenging for core host logic
- Can share 80% of code between native and WASM builds (use build tags)

---

## 3. Code Reuse from Gopper

### 3.1 Directly Reusable (100%)

The Gopper project already provides production-ready implementations of:

#### **protocol/ package** - Complete Klipper Protocol
```go
protocol/
├── vlq.go           // VLQ encoding/decoding (signed, unsigned, bytes, strings)
├── crc16.go         // Klipper-compatible CRC16 checksum
├── buffers.go       // FifoBuffer, SliceInputBuffer, ScratchOutput
├── transport.go     // Message framing, sequencing, ACK/NAK, sync
└── protocol.go      // Constants (MESSAGE_DEST, MESSAGE_SEQ_MASK, etc.)
```

**Proven compatibility**: Already used in Gopper firmware and WASM module.

#### **core/dictionary.go** - Dictionary Handling
- JSON dictionary generation and parsing
- Command/response format strings
- Compression/decompression (via tinycompress/)
- Chunked dictionary transmission

**Proven compatibility**: Can parse dictionaries from both Gopper and original Klipper MCUs.

#### **tinycompress/** - Custom zlib Implementation
- TinyGo-compatible zlib compression
- Works in both TinyGo and standard Go
- Required because standard lib `compress/zlib` doesn't work in TinyGo

### 3.2 Adaptable with Inversion

#### **Transport Layer Inversion**

Current `protocol/transport.go` is MCU-centric:
- **MCU mode**: Receives commands → Sends ACKs → Sends responses
- **Host mode** (needed): Sends commands → Waits for ACKs → Receives responses

**Adaptation strategy**:
```go
// Add a mode flag to Transport
type Transport struct {
    mode TransportMode  // HostMode or MCUMode
    // ... existing fields
}

// Host-specific methods
func (t *Transport) SendCommand(cmdID uint32, args []byte) error
func (t *Transport) WaitForAck(timeout time.Duration) error
func (t *Transport) ReceiveResponse() (*Response, error)
```

#### **Command Registry**

Current `core/command.go` registers MCU commands (handlers firmware executes).

**For host**: Need to track MCU's available commands from dictionary.

**Adaptation strategy**:
```go
// Host-side command builder
type HostCommandRegistry struct {
    commands map[string]*CommandDefinition  // From MCU dictionary
}

func (r *HostCommandRegistry) BuildCommand(name string, params map[string]interface{}) ([]byte, error)
```

### 3.3 Reference Implementation: ui/wasm/

The existing WASM module demonstrates:
- Clean API design for host-side protocol usage
- JavaScript bindings for `encodeVLQ`, `decodeVLQ`, `encodeMessage`, `parseResponse`
- Can be used as blueprint for Go host API

---

## 4. Phased Implementation Plan

### Phase 1: Foundation (2-3 weeks)

**Goal**: Basic host-MCU communication and command execution

#### 1.1 Project Setup
- [ ] Create `gopper-host/` directory or `cmd/host/` in existing repo
- [ ] Decide on repo structure: monorepo (recommended) vs separate repo
- [ ] Set up build system for both standard Go and TinyGo targets

#### 1.2 Core Communication
- [ ] Invert Transport layer for host mode
  - [ ] Add `SendCommand()` and `WaitForAck()`
  - [ ] Implement command queueing
  - [ ] Handle response parsing
- [ ] Implement serial port abstraction
  ```go
  // For standard Go: use github.com/tarm/serial
  // For TinyGo WASM: use WebSerial API via syscall/js
  type SerialPort interface {
      Read([]byte) (int, error)
      Write([]byte) (int, error)
      Close() error
  }
  ```
- [ ] Port clock synchronization (`clocksync.py`)
  - [ ] Send `get_clock` commands periodically
  - [ ] Calculate clock drift and offset
  - [ ] Convert host timestamps to MCU time domain

#### 1.3 MCU Interface
- [ ] Port `mcu.py` functionality
  - [ ] Connection management (connect, reset, disconnect)
  - [ ] Dictionary retrieval and parsing
  - [ ] Command builders from format strings
  - [ ] Response handlers by command ID
- [ ] Implement basic commands
  - [ ] `identify` and dictionary retrieval
  - [ ] `get_config`, `get_uptime`, `get_clock`
  - [ ] `config_reset`, `finalize_config`, `allocate_oids`

**Milestone 1 Deliverable**: CLI tool that connects to MCU, retrieves dictionary, and sends basic commands.

---

### Phase 2: Motion Planning Core (4-5 weeks)

**Goal**: Implement G-code processing and basic motion planning

#### 2.1 G-code Parser
- [ ] Port `gcode.py`
  - [ ] Parse G-code lines (G0, G1, G28, G90, G91, etc.)
  - [ ] Parameter extraction (X, Y, Z, E, F)
  - [ ] Command dispatch
- [ ] Implement G-code state machine
  - [ ] Absolute vs relative positioning
  - [ ] Units (mm vs inches)
  - [ ] Feed rate tracking
- [ ] Port `gcode_move.py`
  - [ ] G1/G0 movement command handling
  - [ ] Coordinate transformation
  - [ ] Extrusion tracking

#### 2.2 Kinematics
- [ ] Port kinematics interface
  ```go
  type Kinematics interface {
      CalcPosition(stepper_positions) (x, y, z float64)
      MoveToPosition(x, y, z, speed float64) ([]StepperMove, error)
      CheckMove(move *Move) error
  }
  ```
- [ ] Implement Cartesian kinematics first (simplest)
  - [ ] X/Y/Z axis mapping
  - [ ] Homing sequences
  - [ ] Limit checking
- [ ] Add CoreXY kinematics (common for modern printers)
- [ ] (Later) Delta, Polar, etc.

#### 2.3 Toolhead and Motion Queue
- [ ] Port `toolhead.py`
  - [ ] Move queue (lookahead buffer)
  - [ ] Velocity planning (trapezoidal motion)
  - [ ] Junction speed calculation (lookahead)
  - [ ] Step timing calculation
- [ ] Implement trapezoidal motion planning
  - [ ] Acceleration/deceleration calculations
  - [ ] Maximum velocity constraints
  - [ ] Lookahead optimization
- [ ] Port trapq (trapezoidal queue)

**Milestone 2 Deliverable**: Host that can process G-code and generate stepper commands for basic moves.

---

### Phase 3: Hardware Integration (3-4 weeks)

**Goal**: Full peripheral support and PID control

#### 3.1 Temperature Control
- [ ] Port thermistor/temperature sensor support
  - [ ] ADC reading and conversion
  - [ ] Multiple sensor types (thermistor, thermocouple, PT100)
- [ ] Implement PID controller
  - [ ] Heater control loop
  - [ ] Auto-tuning (PID_CALIBRATE)
  - [ ] Safety checks (thermal runaway protection)

#### 3.2 Bed Leveling
- [ ] Port bed mesh leveling
  - [ ] Probing sequence
  - [ ] Mesh generation and interpolation
  - [ ] Z-offset compensation during printing
- [ ] Implement probe support (BLTouch, inductive, etc.)

#### 3.3 Other Peripherals
- [ ] Fans (part cooling, hotend, controller)
- [ ] Endstops and homing
- [ ] Filament sensors
- [ ] LEDs and status indicators

**Milestone 3 Deliverable**: Host with full hardware control (heating, probing, fans).

---

### Phase 4: Advanced Features (2-3 weeks)

**Goal**: Production-ready feature set

#### 4.1 Input Shaping
- [ ] Port input shaping for resonance compensation
- [ ] Accelerometer integration (ADXL345)
- [ ] Shaper calibration tools

#### 4.2 Macros and Scripting
- [ ] G-code macro support
- [ ] Variable substitution
- [ ] Conditional execution

#### 4.3 File Printing
- [ ] Virtual SD card implementation
- [ ] File upload and management
- [ ] Print job state machine (start, pause, resume, cancel)

#### 4.4 Web Interface
- [ ] REST API for printer control
- [ ] WebSocket for real-time updates
- [ ] Web UI (can reuse existing Mainsail/Fluidd if API-compatible)

**Milestone 4 Deliverable**: Feature-complete host software.

---

### Phase 5: TinyGo WASM Port (2-3 weeks)

**Goal**: Browser-based control interface

#### 5.1 WASM Adaptation
- [ ] Port host code with build tags
  ```go
  //go:build wasm
  // WebSerial implementation

  //go:build !wasm
  // Native serial implementation
  ```
- [ ] Reduce memory footprint (minimize GC pressure)
- [ ] Optimize for WASM binary size

#### 5.2 WebSerial Integration
- [ ] Implement SerialPort interface using WebSerial API
- [ ] Handle browser security constraints
- [ ] Connection management UI

#### 5.3 Limitations
- [ ] Identify features to disable in WASM (heavy processing)
- [ ] Consider hybrid approach: WASM frontend + Go backend

**Milestone 5 Deliverable**: Browser-based Klipper host controller.

---

## 5. Technical Challenges and Solutions

### 5.1 Real-Time Motion Planning

**Challenge**: Motion planning requires precise timing and low-latency GC.

**Solutions**:
- Use standard Go (not TinyGo) for motion planning
- Pre-allocate buffers to minimize GC pressure
- Use `GOGC` environment variable to tune GC frequency
- Consider `runtime.LockOSThread()` for timing-critical goroutines

### 5.2 Clock Synchronization

**Challenge**: Host and MCU clocks drift over time.

**Solutions**:
- Implement continuous clock synchronization (every 1-2 seconds)
- Use round-trip time (RTT) compensation
- Maintain clock drift estimation with moving average
- **Reuse algorithm from Klipper's `clocksync.py`**

### 5.3 Serial Communication

**Challenge**: Low-latency serial I/O is critical for responsiveness.

**Solutions**:
- Dedicated goroutine for serial reading (equivalent to Klipper's thread)
- Buffered channels for command/response queuing
- Use `github.com/tarm/serial` for standard Go
- For WASM: Accept higher latency of WebSerial API

### 5.4 C Helper Modules

**Challenge**: Klipper uses C helpers for performance (`chelper/`).

**Solutions**:
- **Option 1**: Port to pure Go (easier, ~10-20% slower)
  - Go's math performance is excellent for kinematics
  - JIT compilation makes Go competitive with C
- **Option 2**: Use cgo for critical paths (more complex)
  - Not available in TinyGo WASM
  - Adds build complexity
- **Recommendation**: Start with pure Go, profile, optimize hotspots

### 5.5 Kinematics Calculations

**Challenge**: Iterative kinematic solving is compute-intensive.

**Solutions**:
- Port `itersolve.c` to Go (it's not as complex as it seems)
- Use Go's `math` package (well-optimized)
- Pre-compute lookup tables where possible
- Consider SIMD optimizations later (with `golang.org/x/sys/cpu`)

### 5.6 Configuration File Format

**Challenge**: Klipper uses custom INI-like format with includes and expressions.

**Solutions**:
- Port `configfile.py` to Go
- Use `github.com/go-ini/ini` as base, extend with Klipper specifics
- Support `[include]` directives
- Support arithmetic expressions in config values

---

## 6. Development Roadmap

### Month 1: Foundation
- Week 1-2: Project setup, transport inversion, serial communication
- Week 3-4: MCU interface, dictionary handling, basic commands

### Month 2: Motion Planning
- Week 5-6: G-code parser, kinematics interface, Cartesian implementation
- Week 7-8: Toolhead, motion queue, trapezoidal planning

### Month 3: Hardware
- Week 9-10: Temperature control, PID, heaters
- Week 11-12: Bed leveling, probes, fans, endstops

### Month 4: Polish
- Week 13-14: Input shaping, macros, file printing
- Week 15-16: Web interface, API, testing

### Month 5 (Optional): WASM
- Week 17-18: TinyGo WASM port
- Week 19-20: Browser UI, WebSerial, optimization

**Total estimated time**: 4-5 months for full-featured standard Go host, +1 month for WASM.

---

## 7. Testing Strategy

### 7.1 Unit Tests
- Test VLQ encoding/decoding (already done in Gopper)
- Test kinematics calculations against known positions
- Test motion planning with synthetic moves
- Test PID controller with simulated temperature curves

### 7.2 Integration Tests
- Connect to Gopper MCU and exchange commands
- Verify clock synchronization accuracy
- Test complete G-code command sequences
- Validate stepper timing against oscilloscope

### 7.3 Hardware Tests
- Test with real 3D printer (Cartesian first)
- Print test models (calibration cube, benchy)
- Verify temperature stability under load
- Test homing and bed leveling accuracy

### 7.4 Compatibility Tests
- Test with original Klipper MCU firmware
- Verify protocol compatibility
- Test with different MCU types (RP2040, STM32, AVR)

---

## 8. Dependencies and Libraries

### Standard Go Host

```
Required:
- github.com/tarm/serial             // Serial port communication
- github.com/go-ini/ini              // Config file parsing (extend for Klipper format)
- github.com/gorilla/websocket       // WebSocket for web UI
- github.com/gin-gonic/gin           // Web server (or net/http)

Optional:
- github.com/shirou/gopsutil         // System monitoring
- github.com/spf13/cobra             // CLI framework
- github.com/spf13/viper             // Configuration management
```

### TinyGo WASM

```
Required:
- syscall/js                         // JavaScript interop
- github.com/tinygo-org/tinyfs       // Virtual filesystem (if needed)

Limitations:
- No cgo
- No net package (use WebSocket via syscall/js)
- No os/exec
- Limited reflect package
```

---

## 9. Comparison with Python Klipper

### Advantages of Go Implementation

1. **Type Safety**: Compile-time error catching vs runtime errors
2. **Performance**: Native code vs interpreted Python
3. **Concurrency**: Lightweight goroutines vs Python threads/greenlets
4. **Memory**: Deterministic GC vs Python reference counting + GC
5. **Deployment**: Single binary vs Python + dependencies
6. **Cross-platform**: Easy cross-compilation vs Python virtualenvs
7. **Maintainability**: Strong typing and interfaces
8. **Debugging**: Better profiling tools, race detector

### Challenges vs Python

1. **Development Speed**: Go is more verbose than Python
2. **Ecosystem**: Python has more scientific/numeric libraries
3. **Dynamic Features**: Python's metaprogramming is more flexible
4. **Community**: Klipper community is Python-centric

---

## 10. Migration Path for Users

### Compatibility Layer

To ease adoption, provide:

1. **Config file compatibility**: Parse existing Klipper configs
2. **API compatibility**: Match Klipper's Moonraker API
3. **G-code compatibility**: Support all Klipper G-code extensions
4. **Plugin system**: Allow extending functionality

### Dual-boot Setup

Users could run:
- **Development**: Go host with Gopper firmware
- **Production**: Python Klipper as fallback

### Gradual Migration

1. **Phase 1**: Use Go host with original Klipper MCU firmware (prove host works)
2. **Phase 2**: Use Python Klipper with Gopper MCU firmware (prove MCU works)
3. **Phase 3**: Use Go host with Gopper MCU firmware (full Go stack)

---

## 11. Performance Considerations

### Memory Usage

- **Python Klipper**: ~100-200 MB on Raspberry Pi
- **Go Host** (expected): ~50-100 MB (better GC, static binary)
- **TinyGo WASM**: ~10-20 MB in browser

### CPU Usage

- **Python**: ~20-40% on Raspberry Pi 3 during printing
- **Go** (expected): ~10-20% (native code, better concurrency)

### Latency

- **Python**: ~1-2ms for command processing
- **Go** (expected): ~0.5-1ms (lower GC pauses)

---

## 12. Next Steps

### Immediate Actions

1. **Decide on strategy**: Standard Go first (recommended) or TinyGo WASM
2. **Set up repository**: Extend Gopper or create separate repo
3. **Prototype transport inversion**: Modify `protocol/transport.go` for host mode
4. **Test with Gopper**: Build simple CLI to send commands to Gopper MCU

### First Milestone Goal

Create a working prototype that:
- Connects to Gopper MCU via USB serial
- Retrieves and parses dictionary
- Sends configuration commands
- Receives and displays responses
- Demonstrates clock synchronization

**Estimated time**: 1-2 weeks

---

## 13. References and Resources

### Klipper Documentation
- [Code Overview](https://www.klipper3d.org/Code_Overview.html)
- [Klipper Architecture - DeepWiki](https://deepwiki.com/Klipper3d/klipper/1.1-system-architecture)
- [Rethinking Firmware: Klipper's Host-MCU Architecture](https://medium.com/@hrkeni/rethinking-firmware-what-klippers-host-mcu-architecture-means-for-embedded-systems-2f419cbdc419)

### Klipper Source Code
- [klipper/klippy/toolhead.py](https://github.com/Klipper3d/klipper/blob/master/klippy/toolhead.py)
- [klipper/klippy/mcu.py](https://github.com/Klipper3d/klipper/blob/master/klippy/mcu.py)
- [klipper/klippy/reactor.py](https://github.com/Klipper3d/klipper/blob/master/klippy/reactor.py)

### TinyGo Resources
- [TinyGo WebAssembly Guide](https://tinygo.org/docs/guides/webassembly/)
- [Optimizing TinyGo WASM](https://www.fermyon.com/blog/optimizing-tinygo-wasm)
- [TinyGo Language Support](https://tinygo.org/docs/reference/lang-support/)

### Go Libraries
- [github.com/tarm/serial](https://github.com/tarm/serial)
- [github.com/go-ini/ini](https://github.com/go-ini/ini)
- [github.com/gorilla/websocket](https://github.com/gorilla/websocket)

---

## Appendix A: Klipper Protocol Quick Reference

### Message Format
```
[Length: 1 byte]
[Sequence: 1 byte]  // 0x10-0x1F for host messages
[Command ID: VLQ]
[Arguments: VLQ encoded]
[CRC16: 2 bytes]
[Sync: 0x7E]
```

### Command Flow
1. Host sends command message with sequence N
2. MCU sends ACK with sequence N (immediate)
3. MCU sends response(s) with sequence N (if applicable)
4. Host moves to sequence N+1

### Critical Timing
- Commands within 2^31 timer ticks (~3 minutes at 12 MHz) are buffered
- Clock synchronization needed every 1-2 seconds
- ACKs must be sent immediately (not batched)

---

## Appendix B: Code Snippets

### Host-Side Transport Inversion

```go
// protocol/transport_host.go

type HostTransport struct {
    *Transport
    pendingAck     chan *Message
    responses      chan *Message
    currentSeq     byte
}

func NewHostTransport(serial io.ReadWriter) *HostTransport {
    return &HostTransport{
        Transport:  NewTransport(serial, HostMode),
        pendingAck: make(chan *Message, 1),
        responses:  make(chan *Message, 16),
        currentSeq: 0x10,
    }
}

func (t *HostTransport) SendCommand(cmdID uint32, args []byte) error {
    msg := t.BuildMessage(cmdID, args, t.currentSeq)
    if err := t.WriteMessage(msg); err != nil {
        return err
    }
    return t.WaitForAck(2 * time.Second)
}

func (t *HostTransport) WaitForAck(timeout time.Duration) error {
    select {
    case ack := <-t.pendingAck:
        if ack.Sequence != t.currentSeq {
            return fmt.Errorf("sequence mismatch: expected %02x, got %02x",
                t.currentSeq, ack.Sequence)
        }
        t.currentSeq = (t.currentSeq + 1) & 0x0F | 0x10
        return nil
    case <-time.After(timeout):
        return fmt.Errorf("ACK timeout")
    }
}

func (t *HostTransport) ReceiveResponse() (*Message, error) {
    select {
    case resp := <-t.responses:
        return resp, nil
    case <-time.After(5 * time.Second):
        return nil, fmt.Errorf("response timeout")
    }
}
```

### Serial Port Abstraction

```go
// host/serial.go

type SerialPort interface {
    Read([]byte) (int, error)
    Write([]byte) (int, error)
    Close() error
}

// Standard Go implementation
type NativeSerial struct {
    port *serial.Port
}

func OpenNativeSerial(device string, baud int) (*NativeSerial, error) {
    c := &serial.Config{Name: device, Baud: baud}
    port, err := serial.OpenPort(c)
    if err != nil {
        return nil, err
    }
    return &NativeSerial{port: port}, nil
}

// TinyGo WASM implementation (build tag: wasm)
type WebSerial struct {
    js.Value  // WebSerial API handle
}

func OpenWebSerial() (*WebSerial, error) {
    navigator := js.Global().Get("navigator")
    serial := navigator.Get("serial")
    if serial.IsUndefined() {
        return nil, fmt.Errorf("WebSerial API not available")
    }
    // Request port selection from user
    // ...
}
```

### Command Builder

```go
// host/mcu.go

type MCU struct {
    transport  *HostTransport
    dictionary *Dictionary
    commands   map[string]*CommandBuilder
}

type CommandBuilder struct {
    cmdID  uint32
    format string
}

func (m *MCU) BuildCommand(name string, params map[string]interface{}) ([]byte, error) {
    builder, ok := m.commands[name]
    if !ok {
        return nil, fmt.Errorf("unknown command: %s", name)
    }

    // Parse format string and encode parameters as VLQ
    var buf bytes.Buffer
    protocol.EncodeVLQUint(&buf, builder.cmdID)

    // Example: format = "oid=%c step_pin=%c dir_pin=%c"
    // Extract param types and encode values
    // ...

    return buf.Bytes(), nil
}

func (m *MCU) SendCommand(name string, params map[string]interface{}) error {
    cmdBytes, err := m.BuildCommand(name, params)
    if err != nil {
        return err
    }
    return m.transport.SendCommand(cmdBytes)
}
```

---

## Appendix C: Project Structure

```
gopper/                            # Monorepo (recommended)
├── protocol/                      # SHARED: MCU and Host
│   ├── vlq.go
│   ├── crc16.go
│   ├── buffers.go
│   ├── transport.go
│   └── transport_host.go          # NEW: Host-specific transport
│
├── core/                          # MCU firmware core
│   └── ...
│
├── targets/                       # MCU targets
│   └── ...
│
├── host/                          # NEW: Host software
│   ├── cmd/
│   │   └── gopper-host/
│   │       └── main.go            # CLI entry point
│   ├── mcu/
│   │   ├── mcu.go                 # MCU interface
│   │   ├── clocksync.go           # Clock synchronization
│   │   └── commands.go            # Command builders
│   ├── gcode/
│   │   ├── parser.go              # G-code parser
│   │   ├── interpreter.go         # G-code execution
│   │   └── commands.go            # G-code command handlers
│   ├── motion/
│   │   ├── toolhead.go            # Toolhead and move queue
│   │   ├── trapq.go               # Trapezoidal motion queue
│   │   ├── kinematics.go          # Kinematics interface
│   │   └── kin_cartesian.go       # Cartesian kinematics
│   ├── hardware/
│   │   ├── heater.go              # Heater control
│   │   ├── pid.go                 # PID controller
│   │   ├── fan.go                 # Fan control
│   │   └── probe.go               # Bed probe
│   ├── config/
│   │   └── parser.go              # Config file parser
│   ├── web/
│   │   ├── api.go                 # REST API
│   │   └── websocket.go           # WebSocket handler
│   └── serial/
│       ├── serial.go              # Serial port abstraction
│       ├── serial_native.go       # Standard Go (//go:build !wasm)
│       └── serial_wasm.go         # TinyGo WASM (//go:build wasm)
│
├── ui/                            # Web UI
│   ├── wasm/
│   │   └── main.go                # Existing WASM module (enhance)
│   └── web/
│       ├── index.html
│       └── app.js
│
├── docs/
│   └── klipper-host-port-plan.md  # This document
│
└── Makefile
    └── (add host build targets)
```

---

## Appendix D: Build Commands

```makefile
# Makefile additions

# Standard Go host
.PHONY: host
host:
	go build -o build/gopper-host ./host/cmd/gopper-host

# TinyGo WASM
.PHONY: wasm-host
wasm-host:
	tinygo build -o build/gopper-host.wasm -target wasm ./host/cmd/gopper-host

# Run host with mock MCU
.PHONY: test-host
test-host:
	go run ./host/cmd/gopper-host --device /dev/ttyACM0 --verbose

# Integration test: host + firmware
.PHONY: integration-test
integration-test: rp2040 host
	./scripts/flash-and-test.sh
```

---

**End of Plan**

This document provides a comprehensive roadmap for porting Klipper's host software to Go. The existing Gopper protocol implementation significantly reduces the effort required, and the phased approach allows for incremental development and testing.

**Recommended first step**: Prototype the host-side transport inversion and test with the existing Gopper MCU firmware. This will validate the approach and provide a foundation for building the rest of the host software.
