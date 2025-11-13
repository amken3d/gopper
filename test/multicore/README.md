# TinyGo Multicore Test for RP2040

This test program demonstrates and validates TinyGo's dual-core capabilities on the RP2040 (and RP2350) microcontroller.

## What This Tests

The RP2040 features dual ARM Cortex-M0+ cores at 133MHz. This test validates:

1. **Independent Core Operation** - Both cores running simultaneously
2. **Inter-Core Communication** - Message passing between cores
3. **Atomic Operations** - Thread-safe shared memory access
4. **Task Distribution** - Offloading work to the second core
5. **Performance Benchmarking** - Measuring throughput of both cores
6. **Visual Feedback** - LED patterns showing core activity

## Hardware Requirements

- Raspberry Pi Pico (RP2040) or Pico 2 (RP2350)
- USB cable for programming and serial output
- Optional: External LED on GP15 for Core 1 visualization

### LED Connections

- **Core 0 LED**: Onboard LED (GP25) - controlled by Core 0
- **Core 1 LED**: GP15 (optional external LED) - controlled by Core 1

If you don't have an external LED, the test will still work - you just won't see Core 1's visual feedback.

## Building and Flashing

### For Raspberry Pi Pico (RP2040)

```bash
# Build the multicore test
make test-multicore

# Flash to Pico:
# 1. Hold BOOTSEL button on Pico
# 2. Connect USB cable
# 3. Release BOOTSEL (Pico mounts as USB drive)
# 4. Copy firmware:
cp build/multicore-test-rp2040.uf2 /media/$USER/RPI-RP2/

# The Pico will automatically reboot and start the test
```

### For Raspberry Pi Pico 2 (RP2350)

```bash
# Build for Pico 2
make test-multicore-pico2

# Flash using same BOOTSEL procedure
cp build/multicore-test-rp2350.uf2 /media/$USER/RPI-RP2/
```

## Monitoring Output

Connect to the serial console to see test results:

```bash
# Find the device (usually /dev/ttyACM0)
ls /dev/ttyACM*

# Connect with screen
screen /dev/ttyACM0 115200

# Or use minicom
minicom -D /dev/ttyACM0 -b 115200

# Or Python serial
python3 -m serial.tools.miniterm /dev/ttyACM0 115200
```

## Expected Output

```
Core 0: Launching Core 1...
Core 0: Waiting for Core 1 to be ready...
Core 1: Started!
Core 0: Core 1 is ready!

=== Test 1: Independent Counters ===
Running for 1 second...
Core 0 increments: 98547
Core 1 increments: 97832
✓ Both cores incrementing independently

=== Test 2: Inter-Core Messaging ===
PING-PONG tests passed: 10 / 10
✓ Inter-core messaging working

=== Test 3: Task Distribution ===
Tasks requested: 20
Tasks completed: 20
Time elapsed: 342 ms
Avg time per task: 17 ms
✓ Task distribution working

=== Test 4: Performance Benchmark ===
Benchmarking for 2 seconds...

Benchmark Results:
Duration: 2000 ms
Core 0 operations: 195847
Core 1 operations: 197234
Total operations: 393081
Core 0 ops/sec: 97923
Core 1 ops/sec: 98617
✓ Both cores performing work

=== All Tests Complete ===
Core 0 final count: 195847
Core 1 final count: 197234

=== Entering Continuous Operation Mode ===
Both cores running. Watch LEDs for activity.
Core 0 LED: onboard LED (GP25)
Core 1 LED: GP15

Status Update:
  Core 0 count: 245832
  Core 1 count: 248195
  Ratio (C1/C0): 1.009
...
```

## What Each Test Does

### Test 1: Independent Counters

- Both cores increment their own atomic counters for 1 second
- Verifies that both cores can execute code simultaneously
- Typical result: Each core performs 90,000-100,000+ increments

### Test 2: Inter-Core Messaging

- Core 0 sends PING messages to Core 1
- Core 1 responds with PONG messages
- Uses volatile registers for communication
- Tests 10 round-trip messages
- Success: All 10 messages should complete

### Test 3: Task Distribution

- Core 0 submits 20 computational tasks to Core 1
- Each task does 1000 iterations of math operations
- Measures task completion rate and latency
- Demonstrates offloading work to second core
- Typical result: ~15-20ms per task

### Test 4: Performance Benchmark

- Both cores run computational work for 2 seconds
- Measures operations per second for each core
- Typical result: 90,000-100,000+ ops/sec per core
- Total throughput: ~200,000 ops/sec (nearly 2x single core)

### Continuous Operation Mode

- Both cores run indefinitely
- LEDs blink to show activity (different rates per core)
- Status printed every 5 seconds
- Ratio (C1/C0) should be close to 1.0, showing balanced performance

## LED Patterns

### During Tests

- **Startup**: Onboard LED flashes rapidly (10x fast blinks)
- **Sync**: Both LEDs alternate 5 times when both cores ready
- **Activity**: LEDs blink during tests

### Continuous Mode

- **Core 0 LED (onboard)**: Toggles every ~500ms
- **Core 1 LED (GP15)**: Toggles every ~1000ms
- If both LEDs are blinking at different rates, both cores are running

## Understanding the Results

### Good Results

- ✓ All tests pass
- ✓ Core 0 and Core 1 counters both increase
- ✓ Both cores perform similar amounts of work (ratio ~0.95-1.05)
- ✓ No timeouts or message failures
- ✓ LEDs show consistent activity

### Potential Issues

If tests fail or cores don't run independently:

1. **Core 1 not starting**: Check TinyGo version (need 0.31.0+)
2. **Message timeouts**: May indicate timing issues or synchronization problems
3. **Unbalanced counters**: One core running slower (usually normal, but should be within 10%)
4. **No serial output**: Check baud rate (115200) and USB connection

## Technical Details

### Inter-Core Communication

The test uses two methods for inter-core communication:

1. **Volatile Registers** (`volatile.Register32`)
   - Hardware memory locations visible to both cores
   - Used for message passing
   - No caching issues

2. **Atomic Operations** (`atomic.Uint32`, `atomic.Bool`)
   - Thread-safe counters and flags
   - Prevents race conditions
   - Ensures memory consistency

### Memory Safety

- All shared variables use atomic operations or volatile memory
- No mutexes needed (atomic ops are lock-free)
- Safe for real-time operation

### Core 1 Limitations

Per TinyGo and RP2040 documentation:

- Core 1 has limited access to some peripherals
- USB communication should be done from Core 0
- SPI/I2C/UART typically better on Core 0
- Core 1 is ideal for: computation, sensors, tight timing loops

## Use Cases for Dual-Core in Gopper

This test demonstrates potential uses for Core 1 in the Gopper firmware:

1. **Stepper Pulse Generation** - Core 1 handles precise timing
2. **Sensor Reading** - Core 1 reads encoders/endstops without blocking
3. **Motion Planning** - Core 1 computes kinematics while Core 0 handles comms
4. **Thermal Management** - Core 1 monitors temperatures and manages PID loops
5. **Real-Time Tasks** - Core 1 runs high-priority timing-critical code

## Performance Notes

### Expected Performance

- **Single Core**: ~100,000 operations/sec
- **Dual Core**: ~200,000 operations/sec (nearly 2x)
- **Message Latency**: <100μs for inter-core messages
- **Task Overhead**: ~15-20ms including computation

### Factors Affecting Performance

- Clock speed (default 133MHz on RP2040)
- Memory access patterns (both cores share same RAM)
- Cache contention (RP2040 has small instruction cache)
- Task complexity

## Troubleshooting

### No Serial Output

```bash
# Check device enumeration
ls -l /dev/ttyACM*

# Check permissions
sudo chmod 666 /dev/ttyACM0

# Or add user to dialout group
sudo usermod -a -G dialout $USER
# Then log out and back in
```

### Build Errors

```bash
# Check TinyGo version (need 0.31.0+)
tinygo version

# Update TinyGo if needed
# See: https://tinygo.org/getting-started/install/

# Clean and rebuild
make clean
make test-multicore
```

### Core 1 Not Starting

If you see "Waiting for Core 1 to be ready..." indefinitely:

- TinyGo version too old (need 0.31.0+)
- RP2040 target not properly supported
- Try power cycling the Pico

### Message Timeouts

If Test 2 shows failures:

- Normal if 1-2 failures out of 10
- Many failures may indicate timing issues
- Try reducing workload or increasing timeouts

## Further Reading

- [TinyGo Machine Package](https://tinygo.org/docs/reference/microcontrollers/machine/)
- [RP2040 Datasheet](https://datasheets.raspberrypi.com/rp2040/rp2040-datasheet.pdf) - Chapter 2.3 (Multicore)
- [TinyGo Atomic Package](https://pkg.go.dev/sync/atomic)
- [RP2040 Multicore Example](https://github.com/tinygo-org/tinygo/tree/release/src/examples/multicore)

## License

Same as Gopper project - see main repository LICENSE file.
