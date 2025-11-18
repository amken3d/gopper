si# Gopper Klipper UI

A modern web-based interface for Gopper firmware using **WebSerial API** and **TinyGo WebAssembly**.

## Architecture: Option A - WebSerial Direct Connection

This UI implements direct browser-to-MCU communication with zero backend requirements:

```
┌─────────────────────────────────────┐
│   Browser (Chrome/Edge)             │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  HTML/CSS/JavaScript         │  │
│  │  - WebSerial API             │  │
│  │  - UI Controls               │  │
│  └──────────┬───────────────────┘  │
│             │                       │
│  ┌──────────▼───────────────────┐  │
│  │  Gopper Wasm Module          │  │
│  │  (TinyGo compiled)           │  │
│  │  - VLQ encode/decode         │  │
│  │  - CRC16 calculation         │  │
│  │  - Protocol transport        │  │
│  └──────────┬───────────────────┘  │
│             │                       │
└─────────────┼───────────────────────┘
              │
       WebSerial API
              │
              ▼
     ┌────────────────┐
     │  Gopper MCU    │
     │  (USB CDC)     │
     └────────────────┘
```

## Features

- **Direct USB Communication**: Browser talks directly to MCU via WebSerial
- **Shared Protocol Logic**: Same Go code runs in firmware and browser
- **Zero Backend**: No server or proxy needed
- **Real-time**: Low latency (~1-5ms) for commands and responses
- **Protocol Testing**: Built-in VLQ encoder/decoder testing
- **Command Console**: Send any Klipper protocol command

## Requirements

### Browser
- **Chrome** 89+ or **Edge** 89+ (WebSerial API required)
- Firefox and Safari do not support WebSerial yet

### Build Tools
- **TinyGo** 0.31.0 or later
- **Make** (for build commands)
- **Python** 3.x (for local web server, optional)

## Building

### Quick Start

```bash
# From the gopper root directory
make wasm

# Build and serve the UI
make wasm-serve
```

This will:
1. Compile `ui/wasm/main.go` to WebAssembly
2. Copy `wasm_exec.js` from TinyGo
3. Start a web server at `http://localhost:8080`

### Manual Build

```bash
# Build Wasm module
tinygo build -o ui/web/gopper.wasm -target=wasm ./ui/wasm

# Copy JavaScript glue code
cp $(tinygo env TINYGOROOT)/targets/wasm_exec.js ui/web/

# Serve with any web server (Python example)
cd ui/web
python3 -m http.server 8080
```

## Usage

### 1. Build the Wasm Module

```bash
make wasm
```

### 2. Flash Gopper Firmware to MCU

```bash
# Build firmware for RP2040
make rp2040

# Flash to Pico (hold BOOTSEL while plugging in)
cp build/gopper-rp2040.uf2 /media/[user]/RPI-RP2/
```

### 3. Open the UI

```bash
# Start web server
make wasm-serve

# Open browser to http://localhost:8080
```

### 4. Connect to Gopper

1. Click **"Connect to Gopper"** button
2. Select your Gopper device from the serial port picker
3. Connection status will show "Connected" when successful
4. MCU identification will appear after handshake

### 5. Send Commands

**Built-in Commands:**
- **Identify** - Request MCU identification
- **Get Uptime** - Query MCU uptime
- **Get Clock** - Get current clock value
- **Get Config** - Request configuration

**Custom Commands:**
- Enter command ID (number)
- Enter arguments as hex string (optional)
- Click "Send Custom Command"

**Protocol Testing:**
- Test VLQ encoding with any integer value
- Test VLQ decoding from hex strings
- All operations use the Wasm module

## Project Structure

```
ui/
├── wasm/
│   └── main.go          # TinyGo Wasm module
│                        # Exports: encodeVLQ, decodeVLQ,
│                        #          encodeMessage, parseResponse
│
├── web/
│   ├── index.html       # UI layout
│   ├── style.css        # Styling
│   ├── app.js           # WebSerial + Wasm integration
│   ├── gopper.wasm      # Built Wasm module (generated)
│   └── wasm_exec.js     # TinyGo JS glue (generated)
│
└── README.md            # This file
```

## Code Sharing

The beauty of this architecture is **code reuse**:

```go
// protocol/vlq.go - Used in BOTH firmware and Wasm!
func EncodeVLQInt(output OutputBuffer, v int32) {
    // Same code runs on MCU and in browser
}

func DecodeVLQInt(data *[]byte) (int32, error) {
    // Same code runs on MCU and in browser
}
```

**Shared packages:**
- `protocol/vlq.go` - VLQ encoding/decoding
- `protocol/crc16.go` - CRC calculation
- `protocol/buffers.go` - Buffer abstractions
- `protocol/transport.go` - Protocol transport layer

## Performance

**WebSerial Direct Connection (Option A):**
- Round-trip latency: ~1-5ms
- Bandwidth: Limited only by USB (~12 Mbps Full Speed)
- Command streaming: 100-500 Hz easily achievable
- No network overhead
- No server bottleneck

This is **orders of magnitude faster** than traditional networked solutions!

## Development

### Testing Wasm Functions

Open browser console after loading the UI:

```javascript
// Test VLQ encoding
gopperWasm.encodeVLQ(42)  // Returns "2a"

// Test VLQ decoding
gopperWasm.decodeVLQ("2a")  // Returns {value: 42, consumed: 1}

// Test CRC16
gopperWasm.crc16("050a")  // Returns CRC value

// Access the app instance
gopperApp.log("Test message", "info")
```

### Debugging

1. **Browser DevTools**: F12 in Chrome/Edge
2. **Console logs**: All TX/RX shown in UI console
3. **Wasm debugging**: Use `console.log` in Go code (prints to browser console)
4. **Protocol inspection**: All messages shown as hex in console

### Adding New Commands

1. Define command in firmware (`core/commands.go`)
2. Add handler in `ui/web/app.js`:

```javascript
handleCommand(cmdID, dataHex) {
    switch (cmdID) {
        case 42: // Your new command
            this.log('Received my command!', 'info');
            // Decode dataHex and process
            break;
    }
}
```

3. Add UI button if needed

## Troubleshooting

### "WebSerial not supported"
- Use Chrome 89+ or Edge 89+
- WebSerial not available in Firefox/Safari yet

### "Wasm module not ready"
- Check browser console for errors
- Ensure `gopper.wasm` and `wasm_exec.js` are in `ui/web/`
- Rebuild with `make wasm`

### "Connection error"
- Make sure Gopper firmware is flashed to MCU
- Check USB connection
- Try unplugging and replugging the device
- Check browser console for detailed error

### No response from MCU
- Verify firmware is running (check LED patterns)
- Ensure correct command IDs
- Check console for protocol errors
- Try disconnecting and reconnecting

## Browser Compatibility

| Browser | WebSerial | Wasm | Status |
|---------|-----------|------|--------|
| Chrome 89+ | ✅ | ✅ | **Supported** |
| Edge 89+ | ✅ | ✅ | **Supported** |
| Firefox | ❌ | ✅ | Not supported |
| Safari | ❌ | ✅ | Not supported |

## Next Steps

### Potential Enhancements

1. **G-code Parser in Wasm**
    - Parse and visualize toolpaths in browser
    - Share motion planning code with firmware

2. **Configuration Editor**
    - Parse `printer.cfg` in Wasm
    - Validate settings
    - Generate configs from UI

3. **Real-time Plotting**
    - Temperature graphs
    - Position tracking
    - Live performance metrics

4. **File Management**
    - Upload G-code files
    - Store in browser LocalStorage or IndexedDB
    - Stream to printer

5. **Macro Support**
    - Define custom command sequences
    - Save/load macros
    - Keyboard shortcuts

## Contributing

When adding features:
1. Keep protocol code in `protocol/` package (shared with firmware)
2. Add Wasm exports in `ui/wasm/main.go`
3. Update UI in `ui/web/app.js`
4. Test with real hardware!

## License

Same as Gopper main project (see root LICENSE file).
    