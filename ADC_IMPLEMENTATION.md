# ADC Implementation in Gopper

This document describes the Analog-to-Digital Converter (ADC) implementation in Gopper, which provides compatibility with Klipper's `analog_in` protocol.

## Overview

The ADC implementation allows the firmware to read analog sensor values (thermistors, voltage dividers, etc.) and report them to the Klipper host software. This is essential for temperature sensing and other analog measurements in 3D printing.

## Architecture

The ADC implementation follows Gopper's modular architecture:

```
core/adc.go          - Command handlers and sampling logic
core/adc_hal.go      - Hardware abstraction layer interface
core/adc_test.go     - Unit tests
targets/rp2040/adc.go - RP2040-specific ADC implementation
```

### Key Components

1. **AnalogIn Structure**: Represents a configured ADC input with sampling parameters
2. **Command Handlers**: Process config_analog_in and query_analog_in commands
3. **Timer-Based Sampling**: Uses the scheduler for periodic ADC reads
4. **Range Checking**: Safety mechanism to detect out-of-range values

## Protocol Commands

### config_analog_in

Configures a GPIO pin for analog input sampling.

**Format**: `config_analog_in oid=%c pin=%u`

**Parameters**:
- `oid`: Object ID for this analog input (0-255)
- `pin`: GPIO pin number (26-29 for RP2040 ADC0-ADC3)

**Example**:
```
config_analog_in oid=0 pin=26
```

This configures GPIO 26 (ADC0 on RP2040) as analog input with object ID 0.

### query_analog_in

Starts periodic analog sampling with specified parameters.

**Format**: `query_analog_in oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u min_value=%hu max_value=%hu range_check_count=%c`

**Parameters**:
- `oid`: Object ID of the analog input
- `clock`: Start time for sampling (in timer ticks)
- `sample_ticks`: Delay between individual samples when oversampling
- `sample_count`: Number of samples to average (oversampling)
- `rest_ticks`: Interval between reporting cycles
- `min_value`: Minimum acceptable ADC value (for range checking)
- `max_value`: Maximum acceptable ADC value (for range checking)
- `range_check_count`: Number of consecutive out-of-range readings before shutdown

**Example**:
```
query_analog_in oid=0 clock=1000 sample_ticks=100 sample_count=4 rest_ticks=10000 min_value=1000 max_value=3000 range_check_count=3
```

This configures:
- Start at tick 1000
- Take 4 samples with 100 ticks between each
- Report every 10000 ticks
- Valid range: 1000-3000
- Shutdown after 3 consecutive out-of-range readings

### analog_in_state (Response)

Sent by firmware to report ADC values to the host.

**Format**: `analog_in_state oid=%c next_clock=%u value=%hu`

**Parameters**:
- `oid`: Object ID of the analog input
- `next_clock`: When the next sampling cycle will begin
- `value`: Sum of all samples (not averaged)

**Note**: The value is the **sum** of all oversampled readings, not the average. The host divides by `sample_count` to get the average.

## Sampling Flow

1. **Configuration Phase**:
   - Host sends `config_analog_in` to set up the pin
   - Firmware configures the GPIO for ADC use

2. **Query Phase**:
   - Host sends `query_analog_in` with sampling parameters
   - Firmware schedules a timer for the first sample

3. **Sampling Cycle**:
   ```
   Timer fires → Read ADC → Accumulate value → More samples needed?
                                                    ↓ Yes         ↓ No
                                          Reschedule timer    Send response
                                                              Schedule next cycle
   ```

4. **Range Checking**:
   - After all samples collected, average is checked against min/max
   - Out-of-range readings increment violation counter
   - If violations reach `range_check_count`, firmware triggers shutdown
   - In-range readings reset the violation counter

## RP2040 Implementation Details

### ADC Specifications

- **Resolution**: 12-bit (0-4095)
- **Reference Voltage**: 3.3V (or external VREF on ADC3)
- **Channels**: 5 total
  - ADC0: GPIO 26
  - ADC1: GPIO 27
  - ADC2: GPIO 28
  - ADC3: GPIO 29 (can be used for external VREF)
  - ADC4: Internal temperature sensor (not currently exposed)

### Conversion Speed

RP2040 ADC conversions are very fast (~2µs), so the implementation treats them as synchronous. The `adcSampleImpl` function always returns `ready=true`.

### Pin Mapping

The implementation maps GPIO numbers to TinyGo's `machine.ADC` types:

```go
GPIO 26 → machine.ADC0
GPIO 27 → machine.ADC1
GPIO 28 → machine.ADC2
GPIO 29 → machine.ADC3
```

### Value Scaling

TinyGo's `machine.ADC.Get()` returns 16-bit values (0-65535), but RP2040 ADC is 12-bit. The implementation scales back to 12-bit:

```go
value12bit = (value16bit * 4095) / 65535
```

## Safety Features

### Range Checking

The ADC implementation includes safety features inspired by Klipper's thermistor monitoring:

1. **Min/Max Bounds**: Each query specifies acceptable value range
2. **Violation Counter**: Tracks consecutive out-of-range readings
3. **Automatic Shutdown**: Triggers firmware shutdown if violations exceed threshold

**Example Use Case**: Thermistor monitoring
- A disconnected thermistor reads 0 (short) or max (open circuit)
- Range checking detects this immediately
- Firmware shuts down to prevent heater damage

### Shutdown Behavior

When ADC values go out of range:
1. `TryShutdown("ADC out of range")` is called
2. Firmware enters shutdown state
3. Host detects shutdown via `get_config` response
4. Host can stop all dangerous operations

## Testing

### Unit Tests

Run unit tests with:
```bash
go test ./core/adc_test.go ./core/adc.go ./core/adc_hal.go ./core/command.go ./core/commands.go
```

Tests cover:
- Command registration
- VLQ encoding/decoding
- Configuration handling
- Query parameter parsing

### Hardware Testing

**Prerequisites**:
- RP2040 board (Raspberry Pi Pico)
- Voltage divider or potentiometer connected to GPIO 26-29
- Klipper host software

**Test Procedure**:

1. Build and flash firmware:
   ```bash
   make rp2040
   # Hold BOOTSEL and plug in USB
   cp build/gopper-rp2040.uf2 /media/[user]/RPI-RP2/
   ```

2. Connect analog sensor:
   ```
   3.3V ──┬── 10kΩ ──┬── GPIO 26 (ADC0)
          │          │
          │          └── 10kΩ ──┬── GND
          │                     │
          └── (Thermistor)     (or potentiometer)
   ```

3. Use Klipper console to test:
   ```bash
   ~/klippy-env/bin/python ~/klipper/klippy/console.py -v /dev/ttyACM0
   ```

4. Send test commands:
   ```
   config_analog_in oid=0 pin=26
   query_analog_in oid=0 clock=1000 sample_ticks=100 sample_count=4 rest_ticks=50000 min_value=100 max_value=4000 range_check_count=5
   ```

5. Observe `analog_in_state` responses in console

**Expected Results**:
- Regular `analog_in_state` messages every `rest_ticks`
- Values change when adjusting potentiometer
- Firmware shuts down if value goes out of range

## Common Use Cases

### 1. Thermistor Reading (Hotend)

Typical thermistor configuration for 3D printer hotend:

```python
# In Klipper config
[extruder]
sensor_type: Generic 3950
sensor_pin: gpio26
min_temp: 0
max_temp: 300
```

Klipper translates this to:
```
config_analog_in oid=1 pin=26
query_analog_in oid=1 clock=... sample_ticks=... sample_count=8 rest_ticks=... min_value=... max_value=... range_check_count=4
```

### 2. Heated Bed Thermistor

Similar to hotend, but typically:
- Lower max temperature (120°C)
- Same oversampling for accuracy
- Wider range tolerance

### 3. Voltage Monitoring

Can be used to monitor power supply voltage:
- Voltage divider to scale voltage into 0-3.3V range
- No range checking (set range_check_count=0)
- Lower sample rate (larger rest_ticks)

## Future Enhancements

Potential improvements for the ADC implementation:

1. **Shutdown Messages**: Send detailed shutdown reason to host
2. **ADC4 Support**: Expose internal temperature sensor
3. **Calibration**: Support for ADC calibration data
4. **DMA**: Use DMA for rapid multi-channel sampling
5. **Filtering**: Add digital filtering for noisy signals
6. **Dynamic Ranges**: Allow runtime range updates

## Performance Considerations

### Timer Overhead

Each active analog input uses one timer in the scheduler:
- Minimal overhead (~100 bytes per AnalogIn)
- Timer events scheduled precisely
- No busy-waiting or polling

### Sample Rate Limits

**RP2040 ADC**: ~500 ksamples/sec maximum

**Practical Limits**:
- Thermistor: 10-100 Hz (sample_count=4-16, rest_ticks=12000-120000)
- Fast signals: 1-10 kHz possible but not typical for 3D printing

### Oversampling Benefits

Oversampling (sample_count > 1) provides:
- Noise reduction
- Improved effective resolution
- Better thermal stability readings

**Rule of thumb**: 4x oversampling increases effective resolution by 1 bit

## Troubleshooting

### Issue: No analog_in_state messages

**Possible Causes**:
1. ADC pin not configured (missing `config_analog_in`)
2. Timer not scheduled (check `clock` parameter is valid)
3. Firmware in shutdown state

**Solution**: Check Klipper logs, verify commands sent in correct order

### Issue: Erratic ADC readings

**Possible Causes**:
1. Floating input (no pull-up/pull-down)
2. Noisy signal
3. Incorrect voltage range (exceeds 3.3V)

**Solution**:
- Add proper pull resistors
- Increase sample_count for more averaging
- Check sensor wiring

### Issue: Unexpected shutdowns

**Possible Causes**:
1. Range too tight (min_value/max_value)
2. Intermittent connection
3. Sensor out of spec

**Solution**:
- Widen acceptable range
- Increase range_check_count for more tolerance
- Check sensor and wiring

## Related Files

- `core/adc.go` - Main ADC implementation
- `core/adc_hal.go` - Hardware abstraction layer
- `core/adc_test.go` - Unit tests
- `targets/rp2040/adc.go` - RP2040 ADC driver
- `core/commands.go` - Shutdown handling
- `core/scheduler.go` - Timer scheduling

## References

- [Klipper MCU Commands](https://www.klipper3d.org/MCU_Commands.html)
- [Klipper ADC Source](https://github.com/Klipper3d/klipper/blob/master/src/adccmds.c)
- [RP2040 Datasheet](https://datasheets.raspberrypi.com/rp2040/rp2040-datasheet.pdf) - Section 4.9 (ADC)
- [TinyGo ADC Documentation](https://tinygo.org/docs/reference/microcontrollers/machine/adc/)
