# Gopper
![gopper_gopher.png](gopper_gopher.png)

A Klipper firmware implementation in TinyGo for modern microcontrollers.

Gopper brings the power of Go to 3D printer firmware, providing a modern, type-safe, and maintainable alternative to C-based implementations while maintaining full compatibility with the Klipper protocol.

## Features (Planned)

- Full Klipper protocol compatibility
- Written in TinyGo for optimal embedded performance
- Modular architecture for easy customization
- Type-safe implementation with Go's strong typing
- Support for modern microcontrollers (RP2040, STM32, etc.)

## Status

⚠️ **Early Development** - This project is in active development and not yet ready for production use.

### Current Progress

**Phase 1: Core Protocol (Completed ✅)**
- ✅ VLQ encoding/decoding (Klipper-compatible)
- ✅ CRC16 implementation
- ✅ Buffer management (InputBuffer, OutputBuffer, FifoBuffer)
- ✅ Transport layer with message framing
- ✅ Command dispatch system
- ✅ Unit tests for all components

**Phase 2: Minimum Viable Commands (Completed ✅)**
- ✅ Data dictionary generation system
- ✅ Constants and enumerations registry
- ✅ Core protocol commands:
  - `identify` / `identify_response`
  - `get_uptime` / `uptime`
  - `get_clock` / `clock`
  - `get_config` / `config`
  - `config_reset`
  - `finalize_config`
  - `allocate_oids`
  - `emergency_stop`
- ✅ RP2040 hardware timer integration
- ✅ USB CDC communication (native USB support)
- ✅ Complete main loop with USB I/O

**Phase 3: Stepper Control (Next)**
- ⏳ Stepper driver infrastructure
- ⏳ Step pulse generation
- ⏳ Queue-based motion planning
- ⏳ Endstop handling

## Supported Hardware (Planned)

- RP2040 (Raspberry Pi Pico)
- STM32F4 series
- STM32H7 series
- More to come...

## Communication

Gopper uses **native USB CDC (Communications Device Class)** for communication with the Klipper host. This provides:

When you flash Gopper to your board, it will enumerate as a USB serial device:
- **Linux**: `/dev/ttyACM0`
- **macOS**: `/dev/tty.usbmodem*`
- **Windows**: `COM3` (or similar)

-- If you have UDEV rules setup, then the USB device will enumarate in a predictable fashion with a id or path that is kept constant using the vendor id, device id combination

## Project Structure
```
gopper/
├── protocol/      # Klipper protocol implementation
├── core/          # Core scheduling and command system
├── hardware/      # Hardware abstraction layer
├── motion/        # Motion control and kinematics
├── modules/       # Peripheral modules (steppers, sensors, etc.)
├── targets/       # Target-specific implementations
└─ tinycompress/  # TinyCompress compression library (zlib compatible)
```

## Building

Requirements:
- TinyGo 0.31.0 or later
- Go 1.21 or later

```bash
# Build for RP2040 (Raspberry Pi Pico)
make rp2040

# Build for RP2350 (Raspberry Pi Pico 2)
tinygo build -target=pico2 -o build/gopper-rp2350.uf2 ./targets/rp2040

# Build for STM32F4
make stm32f4

# Run tests
make test

# Clean build artifacts
make clean
```

### Manual Build Commands

```bash
# Build for RP2040
tinygo build -target=pico -o build/gopper-rp2040.uf2 ./targets/rp2040

# Build for RP2350
tinygo build -target=pico2 -o build/gopper-rp2350.uf2 ./targets/rp2040

# Build for STM32F4
tinygo build -target=nucleo-f446re -o build/gopper-stm32f4.hex ./targets/stm32f4

# Run tests
go test ./protocol/... ./core/...
```

## Testing with Klipper


Test protocol without any Klipper installation:

```bash
# 1. Install Klipper (if not using Docker)
git clone https://github.com/Klipper3d/klipper
cd klipper && ./scripts/install-octopi.sh

# 2. Flash firmware
cp build/gopper-rp2040.uf2 /media/$USER/RPI-RP2/
#copy the uf2 to the RP2040 in Bootsel mode

# or use this command to flash directly from the RP2040
tinygo flash -target=pico -port=/dev/ttyACM0 build/gopper-rp2040.uf2

# 3. Configure and run Klipper
cp test/klipper-test.cfg ~/printer.cfg
~/klipper/klippy/klippy.py ~/printer.cfg -l /tmp/klippy.log
```

See [test/README.md](test/README.md) for detailed testing instructions and troubleshooting.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Acknowledgments
(This project is not endorsed by any of the following)
- [Klipper](https://github.com/Klipper3d/klipper) by Kevin O'Connor
- [Anchor](https://github.com/Annex-Engineering/anchor) for inspiration on protocol implementation
- The TinyGo community

## License

MIT License - See [LICENSE](LICENSE) for details
