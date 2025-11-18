# Standalone Mode Implementation

## Overview

This commit adds a standalone mode to Gopper that eliminates the need for a Klipper host. The MCU can now operate autonomously, handling G-code parsing, motion planning, and execution entirely on-chip.

## Architecture

The standalone mode is built on top of the existing firmware infrastructure, reusing:
- **Scheduler**: Timer-based event system for step generation
- **Hardware Abstraction Layer**: GPIO, ADC, PWM, SPI, I2C drivers
- **Timer System**: Precise timing for motion control

New components added:

```
standalone/
├── types.go              # Core data structures
├── manager.go            # Main coordinator
├── config/
│   └── config.go         # JSON configuration system
├── gcode/
│   ├── parser.go         # G-code parser
│   └── interpreter.go    # G-code execution
├── kinematics/
│   ├── kinematics.go     # Interface definition
│   └── cartesian.go      # Cartesian kinematics
├── stepgen/
│   └── stepper.go        # Step pulse generation
└── planner/
    └── planner.go        # Motion planning
```

## Key Features

### 1. G-code Parser
- Parses standard G-code commands
- Supports parameters (X, Y, Z, E, F, S, etc.)
- Handles comments and whitespace
- No stdlib dependencies (embedded-friendly)

### 2. Kinematics System
- Modular architecture for different machine types
- Cartesian kinematics implemented
- Ready for CoreXY, Delta, etc.

### 3. Motion Planner
- Trapezoidal velocity profiles
- Respects axis limits and velocities
- Move queuing and coordination
- Future: Junction deviation and lookahead

### 4. Step Generator
- Integrates with existing scheduler
- Precise step timing using hardware timers
- Per-axis control (direction, enable)
- Non-blocking operation

### 5. Configuration System
- JSON-based configuration
- Defines axes, endstops, heaters
- Example configs provided

## Usage

### Enable Standalone Mode

Edit `targets/rp2040/mode_select.go`:

```go
func GetMode() ModeConfig {
    return ModeConfig{
        Standalone: true,  // Set to true
    }
}
```

### Build and Flash

```bash
make rp2040
# Flash to RP2040
cp build/gopper-rp2040.uf2 /media/[user]/RPI-RP2/
```

### Test

Connect via serial:

```bash
screen /dev/ttyACM0 115200
```

Send G-code:

```gcode
G28           ; Home
G0 X10 Y10    ; Move
G1 X50 F3000  ; Feed move
```

## Supported G-codes

### Motion
- `G0/G1` - Linear move
- `G28` - Home
- `G90` - Absolute positioning
- `G91` - Relative positioning
- `G92` - Set position

### Temperature (Framework in place)
- `M104/M109` - Extruder temp
- `M140/M190` - Bed temp
- `M105` - Report temp

### Extrusion
- `M82` - Absolute extrusion
- `M83` - Relative extrusion

## Implementation Status

### ✅ Complete
- Core architecture
- G-code parser and interpreter
- Cartesian kinematics
- Motion planner with trapezoidal profiles
- Step generation using scheduler
- Configuration system
- Mode selection framework

### 🚧 Next Steps
1. Test on hardware
2. Implement heater PID control
3. Add endstop homing
4. Optimize acceleration profiles
5. Add junction deviation
6. Implement lookahead planning

### 📋 Future Enhancements
- CoreXY/Delta kinematics
- SD card support
- LCD/Display interface
- Advanced motion features (pressure advance, input shaping)
- Web interface (WiFi)

## Memory Footprint

Estimated SRAM usage on RP2040:
- G-code queue: ~8KB
- Motion buffers: ~8KB
- Config/state: ~4KB
- **Total**: ~20KB (~7.5% of 264KB)

## Design Decisions

1. **Dual-Mode Architecture**: Can switch between Klipper protocol and standalone mode
2. **Code Reuse**: Leverages existing scheduler and HAL drivers
3. **Modular Kinematics**: Easy to add new machine types
4. **Embedded-Friendly**: No stdlib dependencies in critical paths
5. **Non-Blocking**: All operations use scheduler/timers

## Comparison: Standalone vs Klipper Mode

| Aspect | Standalone | Klipper Protocol |
|--------|-----------|-----------------|
| Host PC | Not required | Required |
| Latency | Lower | Higher (USB) |
| Setup | Simpler | More complex |
| Features | Basic | Full Klipper |
| Config | JSON | Klipper CFG |
| Memory | ~20KB | ~10KB |

## Files Changed

### New Files
- `standalone/` - Entire standalone module
- `targets/rp2040/standalone_mode.go` - Standalone entry point
- `targets/rp2040/mode_select.go` - Mode selection
- `examples/configs/ender3.json` - Example configuration

### Modified Files
- `targets/rp2040/main.go` - Added mode selection logic

## Testing

The architecture has been designed and implemented. Hardware testing is needed:

1. Build firmware with standalone mode enabled
2. Flash to RP2040
3. Connect steppers and endstops
4. Send G-code commands via serial
5. Verify motion execution

## Documentation

- `standalone/README.md` - Detailed module documentation
- `examples/configs/ender3.json` - Example configuration
- This file - Implementation summary

## Notes

- The standalone mode is **experimental** and not yet production-ready
- Thorough testing on real hardware is required
- Safety features (thermal runaway, limits) need validation
- Performance tuning may be needed

## Future Work

1. **Phase 1** (Next): Hardware testing and debugging
2. **Phase 2**: Heater control and temperature monitoring
3. **Phase 3**: Advanced motion planning (lookahead)
4. **Phase 4**: Additional kinematics (CoreXY, Delta)
5. **Phase 5**: User interfaces (SD, LCD, Web)

## Acknowledgments

This implementation draws inspiration from:
- Klipper's motion planning algorithms
- Marlin's G-code parser
- RepRap firmware architecture

The design maintains compatibility with Klipper while enabling autonomous MCU operation.
