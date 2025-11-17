# Endstop Implementation in Gopper

This document describes the endstop implementation in Gopper, which provides comprehensive support for various types of endstop sensors used in 3D printers.

## Overview

Gopper implements Klipper-compatible endstop functionality with support for:
- **GPIO-based endstops** (mechanical switches, hall effect sensors with digital output)
- **Analog endstops** (ADC-based sensors, analog hall effect sensors)
- **I2C endstops** (Time-of-Flight sensors like VL53L0X, VL53L1X, VL53L4CD)

## Architecture

### Core Components

1. **Trigger Synchronization (trsync)** (`core/trsync.go`)
    - Coordinates multiple endstops during homing operations
    - Manages trigger callbacks and timeouts
    - Reports trigger events to the host

2. **GPIO Endstops** (`core/endstop.go`)
    - Traditional mechanical switches
    - Hall effect sensors with digital output
    - Optical sensors with digital output
    - Uses timer-based sampling with oversampling to prevent false triggers

3. **Analog Endstops** (`core/endstop_analog.go`)
    - Hall effect sensors with analog output
    - Pressure-sensitive sensors
    - Threshold-based triggering with hysteresis

4. **I2C Endstops** (`core/endstop_i2c.go`)
    - Time-of-Flight (TOF) sensors (VL53L0X, VL53L1X, VL53L4CD)
    - Distance-based triggering with hysteresis

## Command Protocol

### Trigger Synchronization Commands

#### `trsync_start`
Format: `trsync_start oid=%c report_clock=%u report_ticks=%u expire_reason=%c`

Starts a trigger synchronization session for coordinated homing.

Parameters:
- `oid`: Object ID of the trigger sync object
- `report_clock`: Initial clock time for status reports
- `report_ticks`: Interval between status reports (in timer ticks)
- `expire_reason`: Reason code to report if timeout expires

#### `trsync_set_timeout`
Format: `trsync_set_timeout oid=%c clock=%u`

Sets a timeout for the trigger synchronization.

Parameters:
- `oid`: Object ID of the trigger sync object
- `clock`: Clock time when timeout expires

#### `trsync_trigger`
Format: `trsync_trigger oid=%c reason=%c`

Manually triggers a trsync object.

Parameters:
- `oid`: Object ID of the trigger sync object
- `reason`: Reason code for the trigger

#### Response: `trsync_state`
Format: `trsync_state oid=%c can_trigger=%c trigger_reason=%c clock=%u`

Reports the current state of a trigger sync object.

### GPIO Endstop Commands

#### `config_endstop`
Format: `config_endstop oid=%c pin=%u pull_up=%c`

Configures a GPIO pin as an endstop input.

Parameters:
- `oid`: Object ID for the endstop
- `pin`: GPIO pin number
- `pull_up`: 1 to enable pull-up resistor, 0 for pull-down

#### `endstop_home`
Format: `endstop_home oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u pin_value=%c trsync_oid=%c trigger_reason=%c`

Starts homing with a GPIO endstop.

Parameters:
- `oid`: Object ID of the endstop
- `clock`: Clock time to start checking
- `sample_ticks`: Time between consecutive samples during oversampling
- `sample_count`: Number of consecutive samples required to confirm trigger (0 to disable)
- `rest_ticks`: Time between check cycles
- `pin_value`: Expected pin value when triggered (1=high, 0=low)
- `trsync_oid`: Object ID of the associated trigger sync
- `trigger_reason`: Reason code to report when triggered

#### `endstop_query_state`
Format: `endstop_query_state oid=%c`

Queries the current state of an endstop.

#### Response: `endstop_state`
Format: `endstop_state oid=%c homing=%c next_clock=%u pin_value=%c`

Reports the current state of a GPIO endstop.

### Analog Endstop Commands

#### `config_analog_endstop`
Format: `config_analog_endstop oid=%c adc_oid=%c threshold=%u trigger_above=%c hysteresis=%u`

Configures an analog (ADC-based) endstop.

Parameters:
- `oid`: Object ID for the endstop
- `adc_oid`: Object ID of the associated ADC channel
- `threshold`: ADC value threshold for triggering
- `trigger_above`: 1 to trigger when value > threshold, 0 when value < threshold
- `hysteresis`: Hysteresis value to prevent oscillation

#### `analog_endstop_home`
Format: `analog_endstop_home oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u trsync_oid=%c trigger_reason=%c`

Starts homing with an analog endstop.

#### `analog_endstop_query_state`
Format: `analog_endstop_query_state oid=%c`

Queries the current state of an analog endstop.

#### Response: `analog_endstop_state`
Format: `analog_endstop_state oid=%c homing=%c next_clock=%u value=%u`

Reports the current state of an analog endstop, including the latest ADC value.

### I2C Endstop Commands

#### `config_i2c_endstop`
Format: `config_i2c_endstop oid=%c i2c_oid=%c addr=%c sensor_type=%c distance_threshold=%u trigger_below=%c hysteresis=%u`

Configures an I2C-based endstop (e.g., TOF sensor).

Parameters:
- `oid`: Object ID for the endstop
- `i2c_oid`: Object ID of the associated I2C device
- `addr`: I2C device address
- `sensor_type`: Sensor type (0=VL53L0X, 1=VL53L1X, 2=VL53L4CD)
- `distance_threshold`: Distance threshold for triggering (in mm)
- `trigger_below`: 1 to trigger when distance < threshold, 0 when distance > threshold
- `hysteresis`: Hysteresis value to prevent oscillation (in mm)

#### `i2c_endstop_home`
Format: `i2c_endstop_home oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u trsync_oid=%c trigger_reason=%c`

Starts homing with an I2C endstop.

#### `i2c_endstop_query_state`
Format: `i2c_endstop_query_state oid=%c`

Queries the current state of an I2C endstop.

#### Response: `i2c_endstop_state`
Format: `i2c_endstop_state oid=%c homing=%c next_clock=%u distance=%u`

Reports the current state of an I2C endstop, including the latest distance reading (in mm).

## Implementation Details

### Oversampling for Noise Rejection

All endstop types implement a two-stage detection mechanism to prevent false triggers:

1. **Initial Detection**: The endstop is checked periodically (every `rest_ticks`)
2. **Oversampling**: When a potential trigger is detected, the endstop is sampled multiple times consecutively (every `sample_ticks`) to confirm the trigger

This approach, borrowed from Klipper, prevents false triggers caused by electrical noise or mechanical bounce.

### Trigger Synchronization

The `trsync` system coordinates multiple endstops during homing operations:

1. Multiple endstops can be registered with the same `trsync` object
2. When any endstop triggers, all registered callbacks are invoked
3. The first trigger wins - subsequent triggers are ignored
4. Timeout mechanism provides fallback if no endstop triggers

### Platform Support

The endstop implementation is platform-agnostic and relies on HAL (Hardware Abstraction Layer) interfaces:

- **GPIO HAL**: Provides pin configuration and reading
- **ADC HAL**: Provides analog-to-digital conversion
- **I2C HAL**: Provides I2C communication

Currently supported platforms:
- RP2040 (Raspberry Pi Pico)
- RP2350 (Raspberry Pi Pico 2)

## Usage Examples

### Basic Mechanical Switch

```python
# Klipper configuration example
[stepper_x]
endstop_pin: ^gpio25  # Pull-up enabled
```

The firmware will:
1. Configure GPIO25 as input with pull-up
2. During homing, sample the pin multiple times to confirm trigger
3. Report trigger to the host via trsync

### Hall Effect Sensor (Analog)

```python
# Klipper configuration example
[stepper_y]
endstop_pin: analog_endstop:ADC0
```

The firmware will:
1. Configure ADC0 for analog sampling
2. Monitor ADC value against threshold
3. Use hysteresis to prevent oscillation
4. Report trigger when threshold is crossed consistently

### TOF Sensor (I2C)

```python
# Klipper configuration example
[stepper_z]
endstop_pin: i2c_endstop:VL53L0X
```

The firmware will:
1. Initialize VL53L0X sensor via I2C
2. Periodically read distance measurements
3. Trigger when distance crosses threshold
4. Use hysteresis to prevent oscillation

## Future Enhancements

Potential improvements for future releases:

1. **Sensorless Homing**: Detect motor stall current for endstop detection
2. **Encoder-based Endstops**: Use rotary encoders for position detection
3. **Multiple Sensor Fusion**: Combine data from multiple sensor types
4. **Dynamic Threshold Adjustment**: Auto-tune thresholds based on environmental conditions
5. **Advanced Filtering**: Implement Kalman filtering for noisy sensors

## References

- Klipper endstop implementation: [src/endstop.c](https://github.com/Klipper3d/klipper/blob/master/src/endstop.c)
- Klipper trsync implementation: [src/trsync.c](https://github.com/Klipper3d/klipper/blob/master/src/trsync.c)
- Klipper endstop phase: [docs/Endstop_Phase.md](https://github.com/Klipper3d/klipper/blob/master/docs/Endstop_Phase.md)
