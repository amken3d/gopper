// I2C endstop handling for Time-of-Flight (TOF) and other I2C-based sensors
// Supports VL53L0X, VL53L1X, VL53L4CD, and similar distance sensors
package core

import (
	"gopper/protocol"
)

// I2CEndstop represents a configured I2C-based endstop
type I2CEndstop struct {
	OID           uint8        // Object ID
	I2C           *I2CDevice   // I2C instance for communication
	I2CAddr       uint8        // I2C device address
	Flags         uint8        // State flags (ESF_*)
	Timer         Timer        // Timer for sampling
	SampleTime    uint32       // Time between samples (in ticks)
	SampleCount   uint8        // Number of consecutive samples required
	TriggerCount  uint8        // Remaining samples before trigger
	RestTime      uint32       // Rest time between check cycles
	NextWake      uint32       // Next scheduled wake time
	TriggerSync   *TriggerSync // Associated trigger synchronization object
	TriggerReason uint8        // Reason code to report when triggered

	// I2C-specific parameters
	SensorType        uint8  // Sensor type (VL53L0X, VL53L1X, etc.)
	DistanceThreshold uint32 // Trigger distance threshold (in mm)
	TriggerBelow      bool   // True if trigger when distance < threshold
	Hysteresis        uint32 // Hysteresis value to prevent oscillation (in mm)

	// Sensor state
	LastDistance uint32 // Last measured distance (in mm)
	Initialized  bool   // Sensor initialized flag
}

// I2C Endstop sensor types
const (
	I2C_ENDSTOP_VL53L0X  = 0x00
	I2C_ENDSTOP_VL53L1X  = 0x01
	I2C_ENDSTOP_VL53L4CD = 0x02
)

// Global registry of I2C endstops
var i2cEndstops = make(map[uint8]*I2CEndstop)

// InitI2CEndstopCommands registers I2C endstop-related commands
func InitI2CEndstopCommands() {
	// RE-ENABLED: Testing with properly recompiled TinyGo (32KB stack)
	RegisterCommand("config_i2c_endstop", "oid=%c i2c_oid=%c addr=%c sensor_type=%c distance_threshold=%u trigger_below=%c hysteresis=%u", handleConfigI2CEndstop)
	RegisterCommand("i2c_endstop_home", "oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u trsync_oid=%c trigger_reason=%c", handleI2CEndstopHome)
	RegisterCommand("i2c_endstop_query_state", "oid=%c", handleI2CEndstopQueryState)
	RegisterResponse("i2c_endstop_state", "oid=%c homing=%c next_clock=%u distance=%u")
}

// handleConfigI2CEndstop configures an I2C endstop
// Format: config_i2c_endstop oid=%c i2c_oid=%c addr=%c sensor_type=%c distance_threshold=%u trigger_below=%c hysteresis=%u
func handleConfigI2CEndstop(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	i2cOID, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	addr, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	sensorType, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	distanceThreshold, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	triggerBelow, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	hysteresis, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get I2C object
	i2c, exists := GetI2C(uint8(i2cOID))
	if !exists {
		return nil // Silently ignore if I2C not configured
	}

	// Create new I2C endstop instance
	ies := &I2CEndstop{
		OID:               uint8(oid),
		I2C:               i2c,
		I2CAddr:           uint8(addr),
		SensorType:        uint8(sensorType),
		DistanceThreshold: distanceThreshold,
		TriggerBelow:      triggerBelow != 0,
		Hysteresis:        hysteresis,
		Initialized:       false,
	}

	// Initialize the sensor based on type
	if err := initializeI2CSensor(ies); err != nil {
		// Log error but continue - sensor may be initialized later
	}

	// Register in global map
	i2cEndstops[uint8(oid)] = ies

	return nil
}

// handleI2CEndstopHome starts homing with an I2C endstop
// Format: i2c_endstop_home oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u trsync_oid=%c trigger_reason=%c
func handleI2CEndstopHome(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	clock, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	sampleTicks, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	sampleCount, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	restTicks, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	trsyncOID, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	triggerReason, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get I2C endstop object
	ies, exists := i2cEndstops[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Cancel any existing timer
	ies.Timer.Next = nil

	// If sample_count is 0, disable homing
	if sampleCount == 0 {
		ies.TriggerSync = nil
		ies.Flags = 0
		return nil
	}

	// Get trigger sync object
	ts, exists := GetTriggerSync(uint8(trsyncOID))
	if !exists {
		return nil // Silently ignore if trsync not configured
	}

	// Ensure sensor is initialized
	if !ies.Initialized {
		if err := initializeI2CSensor(ies); err != nil {
			return err
		}
	}

	// Configure homing parameters
	ies.SampleTime = sampleTicks
	ies.SampleCount = uint8(sampleCount)
	ies.TriggerCount = uint8(sampleCount)
	ies.RestTime = restTicks
	ies.TriggerSync = ts
	ies.TriggerReason = uint8(triggerReason)
	ies.Flags = ESF_HOMING

	// Schedule initial timer
	ies.Timer.WakeTime = clock
	ies.Timer.Handler = i2cEndstopEvent
	ScheduleTimer(&ies.Timer)

	return nil
}

// handleI2CEndstopQueryState queries the current I2C endstop state
// Format: i2c_endstop_query_state oid=%c
func handleI2CEndstopQueryState(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get I2C endstop object
	ies, exists := i2cEndstops[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Read current state
	state := disableInterrupts()
	eflags := ies.Flags
	nextwake := ies.NextWake
	distance := ies.LastDistance
	restoreInterrupts(state)

	// Send response
	homing := uint32(0)
	if (eflags & ESF_HOMING) != 0 {
		homing = 1
	}

	SendResponse("i2c_endstop_state", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(oid))
		protocol.EncodeVLQUint(output, homing)
		protocol.EncodeVLQUint(output, nextwake)
		protocol.EncodeVLQUint(output, distance)
	})

	return nil
}

// initializeI2CSensor initializes the I2C sensor based on its type
func initializeI2CSensor(ies *I2CEndstop) error {
	if ies.I2C == nil {
		return nil
	}

	// Initialize based on sensor type
	switch ies.SensorType {
	case I2C_ENDSTOP_VL53L0X:
		// VL53L0X initialization sequence
		// This is a simplified version - full initialization would require more steps
		// Set continuous ranging mode
		// Write to SYSRANGE_START register (0x00) with value 0x02
		if err := i2cWrite(ies.I2C, ies.I2CAddr, []byte{0x00, 0x02}); err != nil {
			return err
		}

	case I2C_ENDSTOP_VL53L1X:
		// VL53L1X initialization sequence
		// Set distance mode and timing budget
		// This is a simplified version
		if err := i2cWrite(ies.I2C, ies.I2CAddr, []byte{0x00, 0x01}); err != nil {
			return err
		}

	case I2C_ENDSTOP_VL53L4CD:
		// VL53L4CD initialization sequence
		// This is a simplified version
		if err := i2cWrite(ies.I2C, ies.I2CAddr, []byte{0x00, 0x01}); err != nil {
			return err
		}
	}

	ies.Initialized = true
	return nil
}

// readI2CDistance reads distance from the I2C sensor
func readI2CDistance(ies *I2CEndstop) (uint32, error) {
	if ies.I2C == nil {
		return 0, nil
	}

	var distance uint32

	switch ies.SensorType {
	case I2C_ENDSTOP_VL53L0X:
		// VL53L0X: Read from RESULT_RANGE_STATUS (0x14)
		// Distance is at offset 10-11 (2 bytes, big-endian)
		buf := make([]byte, 12)
		if err := i2cRead(ies.I2C, ies.I2CAddr, 0x14, buf); err != nil {
			return 0, err
		}
		// Extract distance (bytes 10-11)
		distance = uint32(buf[10])<<8 | uint32(buf[11])

	case I2C_ENDSTOP_VL53L1X:
		// VL53L1X: Read from RESULT__FINAL_CROSSTALK_CORRECTED_RANGE_MM_SD0 (0x0096)
		buf := make([]byte, 2)
		if err := i2cRead(ies.I2C, ies.I2CAddr, 0x0096, buf); err != nil {
			return 0, err
		}
		distance = uint32(buf[0])<<8 | uint32(buf[1])

	case I2C_ENDSTOP_VL53L4CD:
		// VL53L4CD: Similar to VL53L1X
		buf := make([]byte, 2)
		if err := i2cRead(ies.I2C, ies.I2CAddr, 0x0096, buf); err != nil {
			return 0, err
		}
		distance = uint32(buf[0])<<8 | uint32(buf[1])
	}

	return distance, nil
}

// i2cEndstopEvent is the timer callback for I2C endstop checking
func i2cEndstopEvent(t *Timer) uint8 {
	// Find the I2CEndstop instance that owns this timer
	var ies *I2CEndstop
	for _, iesPtr := range i2cEndstops {
		if iesPtr != nil && &iesPtr.Timer == t {
			ies = iesPtr
			break
		}
	}

	if ies == nil {
		return SF_DONE
	}

	// Read distance from sensor
	distance, err := readI2CDistance(ies)
	if err != nil {
		// On error, reschedule and try again
		t.WakeTime = t.WakeTime + ies.RestTime
		return SF_RESCHEDULE
	}

	ies.LastDistance = distance

	// Check if distance crosses threshold
	var triggered bool
	if ies.TriggerBelow {
		// Trigger when distance falls below threshold
		triggered = distance < ies.DistanceThreshold
	} else {
		// Trigger when distance rises above threshold
		triggered = distance > ies.DistanceThreshold
	}

	nextWake := t.WakeTime + ies.RestTime

	if !triggered {
		// No match - reschedule for the next attempt
		t.WakeTime = nextWake
		return SF_RESCHEDULE
	}

	// Potential trigger detected - start oversampling
	ies.NextWake = nextWake
	t.Handler = i2cEndstopOversampleEvent
	return i2cEndstopOversampleEvent(t)
}

// i2cEndstopOversampleEvent is the timer callback for oversampling
func i2cEndstopOversampleEvent(t *Timer) uint8 {
	// Find the I2CEndstop instance that owns this timer
	var ies *I2CEndstop
	for _, iesPtr := range i2cEndstops {
		if iesPtr != nil && &iesPtr.Timer == t {
			ies = iesPtr
			break
		}
	}

	if ies == nil {
		return SF_DONE
	}

	// Read distance from sensor
	distance, err := readI2CDistance(ies)
	if err != nil {
		// On error, go back to main event
		t.Handler = i2cEndstopEvent
		t.WakeTime = ies.NextWake
		ies.TriggerCount = ies.SampleCount
		return SF_RESCHEDULE
	}

	ies.LastDistance = distance

	// Check if distance still crosses threshold (with hysteresis)
	var triggered bool
	if ies.TriggerBelow {
		// Trigger when distance falls below threshold
		// Use hysteresis to prevent oscillation
		triggered = distance < (ies.DistanceThreshold + ies.Hysteresis)
	} else {
		// Trigger when distance rises above threshold
		// Use hysteresis to prevent oscillation
		triggered = distance > (ies.DistanceThreshold - ies.Hysteresis)
	}

	if !triggered {
		// No longer matching - reschedule for the next attempt
		t.Handler = i2cEndstopEvent
		t.WakeTime = ies.NextWake
		ies.TriggerCount = ies.SampleCount
		return SF_RESCHEDULE
	}

	// Decrement trigger count
	count := ies.TriggerCount - 1
	if count == 0 {
		// All samples confirmed - trigger!
		if ies.TriggerSync != nil {
			TriggerSyncDoTrigger(ies.TriggerSync, ies.TriggerReason)
		}
		return SF_DONE
	}

	// Continue oversampling
	ies.TriggerCount = count
	t.WakeTime += ies.SampleTime
	return SF_RESCHEDULE
}

// Helper functions for I2C communication

func i2cWrite(i2c *I2CDevice, addr uint8, data []byte) error {
	if i2c == nil {
		return nil
	}
	// Use the I2C HAL to write data
	return MustI2C().Write(i2c.Bus, I2CAddress(addr), data)
}

func i2cRead(i2c *I2CDevice, addr uint8, reg uint8, buf []byte) error {
	if i2c == nil {
		return nil
	}
	// Use the I2C HAL combined write-then-read operation:
	// reg is sent as regData, then readLen bytes are read into a temporary buffer.
	readLen := uint8(len(buf))
	data, err := MustI2C().Read(i2c.Bus, I2CAddress(addr), []byte{reg}, readLen)
	if err != nil {
		return err
	}
	copy(buf, data)
	return nil
}
