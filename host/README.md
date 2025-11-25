# Gopper Host - Klipper Protocol Host Implementation

This is the host-side implementation of the Klipper protocol in Go. It's the first step toward porting the full Klipper host software (klippy) from Python to Go.

## Status: Prototype/Proof of Concept

This is an early prototype demonstrating host-to-MCU communication. Currently implemented:

- ✅ Host-side transport layer (inverted from MCU-side)
- ✅ Serial port abstraction (native Go implementation)
- ✅ MCU connection management
- ✅ Dictionary retrieval and parsing
- ✅ Basic command sending (get_uptime, get_clock, get_config)
- ✅ Sequence number management
- ✅ ACK/NAK handling

## Architecture

```
host/
├── cmd/
│   └── gopper-host/     # CLI tool for testing
│       └── main.go
├── mcu/
│   └── mcu.go           # MCU interface and dictionary handling
├── serial/
│   ├── serial.go        # Serial port interface
│   └── serial_native.go # Native implementation (build tag: !wasm)
└── README.md            # This file
```

## Building

```bash
# Build the host software
make host

# This creates: build/gopper-host
```

## Usage

### Basic Connection Test

```bash
# Connect to MCU and retrieve dictionary
./build/gopper-host -device /dev/ttyACM0
```

### Command-Line Options

```bash
./build/gopper-host -help
  -device string
        Serial device path (default "/dev/ttyACM0")
  -baud int
        Baud rate (ignored for USB CDC) (default 250000)
  -verbose
        Enable verbose output
```

### Interactive Commands

Once connected, you can use these commands:

- `help` - Show available commands
- `dict` - Print dictionary summary
- `raw` - Print raw dictionary data (JSON)
- `get_uptime` - Send get_uptime command to MCU
- `get_clock` - Send get_clock command to MCU
- `get_config` - Send get_config command to MCU
- `quit` / `exit` / `q` - Exit the program

### Example Session

```
$ ./build/gopper-host -device /dev/ttyACM0

Gopper Host - Klipper Protocol Host Implementation
===================================================

Connecting to MCU on /dev/ttyACM0...
Connected successfully!
Retrieving dictionary from MCU...
  Retrieved 0 bytes...
Dictionary retrieved: 1234 bytes

=== MCU Dictionary ===
Version: gopper-0.1.0
Build: go-tinygo

Config:
  CLOCK_FREQ = 12000000
  MCU = rp2040

Commands (41):
  [0] identify_response offset=%u data=%*s
  [1] identify offset=%u count=%c
  ... and 31 more

Responses (10):
  [0] identify_response offset=%u data=%*s
  ... and 9 more
======================

Enter commands (type 'help' for available commands, 'quit' to exit):
> get_uptime
Sending get_uptime command...
Command sent successfully!
(Note: Response handling not yet implemented - check MCU logs)

> dict
<prints dictionary again>

> quit
Goodbye!
```

## Testing with Gopper MCU

1. **Flash Gopper firmware to RP2040:**
   ```bash
   make rp2040
   # Hold BOOTSEL and plug in USB, then:
   cp build/gopper-rp2040.uf2 /media/<user>/RPI-RP2/
   ```

2. **Wait for MCU to enumerate** (usually appears as `/dev/ttyACM0`)

3. **Run the host:**
   ```bash
   ./build/gopper-host
   ```

4. **Expected behavior:**
   - MCU LED should flash on connection
   - Dictionary should be retrieved successfully
   - Commands should be acknowledged by MCU
   - You can see MCU responses if you monitor serial separately

## Architecture Details

### Transport Layer (`protocol/transport_host.go`)

The host transport is an inversion of the MCU transport:

- **MCU Transport**: Receives commands → Sends ACKs → Sends responses
- **Host Transport**: Sends commands → Waits for ACKs → Receives responses

Key features:
- Sequence number management (0x10-0x1F, wrapping)
- Separate channels for ACKs and responses
- Background reader goroutine for continuous serial reading
- Timeout handling for ACKs and responses
- Thread-safe operations with mutexes

### MCU Interface (`host/mcu/mcu.go`)

Provides high-level MCU interaction:
- Connection management
- Dictionary retrieval (chunked identify commands)
- Dictionary parsing (JSON)
- Command builders from dictionary format strings
- Response handler registration

### Serial Abstraction (`host/serial/`)

Platform-independent serial interface:
- Uses `github.com/tarm/serial` for native Go
- Can be extended for TinyGo WASM (WebSerial API)
- Mock implementation possible for testing

## What's Next

This prototype demonstrates the feasibility of the host port. Next steps:

1. **Response Handling**: Properly decode and handle MCU responses
2. **Clock Synchronization**: Implement clock sync protocol
3. **G-code Parser**: Parse and execute G-code commands
4. **Motion Planning**: Implement toolhead and kinematics
5. **Configuration**: Parse Klipper config files
6. **Web Interface**: Add REST API and WebSocket

See the full roadmap in: `docs/klipper-host-port-plan.md`

## Dependencies

- `github.com/tarm/serial` - Serial port communication (native Go)
- Standard Go library (encoding/json, io, sync, etc.)

## Build Tags

- Default (no tags): Native Go with `github.com/tarm/serial`
- `wasm`: TinyGo WASM with WebSerial API (TODO)

## Contributing

This is an early prototype. Feedback and contributions welcome!

## License

Same as Gopper project (check main README.md)
