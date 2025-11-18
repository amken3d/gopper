# Gopper Standalone Mode

This module enables Gopper to run entirely on the microcontroller without requiring a Klipper host. In standalone mode, the MCU handles G-code parsing, motion planning, and execution autonomously.

## Architecture

```
┌─────────────────────────────────────┐
│         Gopper MCU                  │
│  ┌──────────────────────────────┐  │
│  │   Standalone Module          │  │
│  │  ┌────────────┬───────────┐  │  │
│  │  │   G-code   │  Motion   │  │  │
│  │  │   Parser   │  Planner  │  │  │
│  │  └────────────┴───────────┘  │  │
│  │  ┌────────────┬───────────┐  │  │
│  │  │ Kinematics │ Step Gen  │  │  │
│  │  └────────────┴───────────┘  │  │
│  └──────────────────────────────┘  │
│  ┌──────────────────────────────┐  │
│  │  Hardware Abstraction Layer  │  │
│  │  (Scheduler, Timers, HAL)    │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
```

## Components

### G-code Parser (`gcode/parser.go`)
- Parses G-code commands (G0, G1, G28, G90, G91, G92, etc.)
- Extracts parameters (X, Y, Z, E, F, S, etc.)
- Handles comments and whitespace

### G-code Interpreter (`gcode/interpreter.go`)
- Executes parsed G-code commands
- Maintains machine state (position, mode, feedrate)
- Converts G-code to motion planner calls

### Kinematics (`kinematics/`)
- Converts XYZ coordinates to stepper positions
- Implementations:
  - **Cartesian**: 1:1 mapping (default)
  - CoreXY: Future implementation
  - Delta: Future implementation

### Motion Planner (`planner/planner.go`)
- Calculates trapezoidal velocity profiles
- Enforces axis limits and maximum velocities
- Queues moves for execution
- Coordinates multiple steppers

### Step Generator (`stepgen/stepper.go`)
- Generates step pulses using the existing scheduler
- Controls individual stepper motors
- Handles direction and enable signals

### Manager (`manager.go`)
- Coordinates all components
- Handles serial I/O
- Provides high-level interface

## Configuration

Machines are configured using JSON files. See `examples/configs/ender3.json` for an example.

### Configuration Structure

```json
{
  "mode": "standalone",
  "kinematics": "cartesian",
  "axes": {
    "x": {
      "step_pin": "gpio0",
      "dir_pin": "gpio1",
      "enable_pin": "gpio8",
      "steps_per_mm": 80.0,
      "max_velocity": 300.0,
      "max_accel": 3000.0,
      "homing_vel": 50.0,
      "min_position": 0.0,
      "max_position": 220.0
    }
    // ... y, z, e axes
  },
  "endstops": {
    "x": { "pin": "gpio20", "invert": false }
    // ... y, z endstops
  },
  "heaters": {
    "extruder": {
      "sensor_pin": "ADC0",
      "heater_pin": "gpio10",
      "pid": [0.1, 0.5, 0.05],
      "max_temp": 300.0
    }
    // ... bed heater
  }
}
```

## Usage

### Enabling Standalone Mode

Edit `targets/rp2040/mode_select.go`:

```go
func GetMode() ModeConfig {
    return ModeConfig{
        Standalone: true,  // Change to true
    }
}
```

### Building

```bash
make rp2040
```

### Flashing

1. Hold BOOTSEL button and plug in USB
2. Copy firmware:
```bash
cp build/gopper-rp2040.uf2 /media/[user]/RPI-RP2/
```

### Testing

Connect via serial terminal:

```bash
screen /dev/ttyACM0 115200
```

Send G-code commands:

```gcode
G28           ; Home all axes
G0 X10 Y10    ; Move to X10, Y10
G1 X50 Y50 F3000  ; Move at 3000mm/min
M114          ; Get position
```

## Supported G-codes

### Motion Commands
- `G0/G1` - Linear move
- `G28` - Home axis (X, Y, Z, or all)
- `G90` - Absolute positioning
- `G91` - Relative positioning
- `G92` - Set position

### Temperature Commands
- `M104 S[temp]` - Set extruder temperature
- `M109 S[temp]` - Set extruder temperature and wait
- `M140 S[temp]` - Set bed temperature
- `M190 S[temp]` - Set bed temperature and wait
- `M105` - Get temperature (future)

### Extrusion
- `M82` - Absolute extrusion mode
- `M83` - Relative extrusion mode

### Information
- `M114` - Get current position (future)

## Implementation Status

### ✅ Implemented
- G-code parser (basic commands)
- Cartesian kinematics
- Motion planner with trapezoidal profiles
- Step generation using scheduler
- Configuration system
- Manager integration

### 🚧 In Progress
- Acceleration ramping
- Junction speed optimization
- Lookahead planning

### 📋 TODO
- Heater PID control
- Temperature reading
- Endstop handling during homing
- CoreXY/Delta kinematics
- SD card support
- LCD/Display support
- Advanced features (pressure advance, input shaping)

## Memory Usage

Estimated memory footprint on RP2040 (264KB SRAM):

- G-code queue: ~8KB (32 moves)
- Motion planner: ~4KB (lookahead buffer)
- Step buffers: ~4KB (4 steppers)
- Config/state: ~2KB
- **Total**: ~20KB (< 8% of available SRAM)

## Performance Considerations

### Timer Resolution
- Default: 12MHz timer (83ns resolution)
- Sufficient for step rates up to ~100kHz

### Step Generation
- Uses existing scheduler (interrupt-safe)
- Constant-time step scheduling
- No blocking operations

### G-code Parsing
- Integer and float parsing without stdlib
- Minimal allocations
- Suitable for streaming execution

## Future Enhancements

1. **Advanced Motion Control**
   - Junction deviation
   - Lookahead planning
   - Pressure advance
   - Input shaping

2. **Additional Kinematics**
   - CoreXY
   - Delta
   - Polar
   - SCARA

3. **User Interfaces**
   - SD card printing
   - LCD/TFT displays
   - Web interface (via WiFi module)

4. **Safety Features**
   - Thermal runaway protection
   - Position limits enforcement
   - Power loss recovery

5. **Advanced Features**
   - Multi-extruder support
   - Automatic bed leveling
   - Filament sensors
   - Resume after pause

## Comparison: Standalone vs Klipper Mode

| Feature | Standalone Mode | Klipper Mode |
|---------|----------------|--------------|
| Host PC Required | No | Yes |
| G-code Source | USB/UART/SD | Klipper Host |
| Motion Planning | On MCU | On Host |
| Config Format | JSON | Klipper CFG |
| Memory Usage | ~20KB | ~10KB |
| Latency | Lower | Higher (USB) |
| Complexity | Higher (MCU) | Lower (MCU) |
| Advanced Features | Limited | Full Klipper |

## Contributing

When adding features to standalone mode:

1. Keep memory usage minimal
2. Avoid blocking operations
3. Use the scheduler for timing
4. Test on real hardware
5. Document configuration options

## License

Same as Gopper main project.
