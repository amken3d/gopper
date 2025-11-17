// Analog endstop handling for ADC-based sensors
// Supports hall effect sensors, pressure sensors, and other analog sensors
package core

import (
	"gopper/protocol"
)

// AnalogEndstop represents a configured ADC-based endstop
type AnalogEndstop struct {
	OID           uint8        // Object ID
	ADC           *AnalogIn    // ADC instance for reading analog values
	Flags         uint8        // State flags (ESF_*)
	Timer         Timer        // Timer for sampling
	SampleTime    uint32       // Time between samples (in ticks)
	SampleCount   uint8        // Number of consecutive samples required
	TriggerCount  uint8        // Remaining samples before trigger
	RestTime      uint32       // Rest time between check cycles
	NextWake      uint32       // Next scheduled wake time
	TriggerSync   *TriggerSync // Associated trigger synchronization object
	TriggerReason uint8        // Reason code to report when triggered

	// Analog-specific parameters
	Threshold    uint32 // Trigger threshold value (ADC counts)
	TriggerAbove bool   // True if trigger when value > threshold, false if value < threshold
	Hysteresis   uint32 // Hysteresis value to prevent oscillation
}

// Global registry of analog endstops
var analogEndstops = make(map[uint8]*AnalogEndstop)

// InitAnalogEndstopCommands registers analog endstop-related commands
func InitAnalogEndstopCommands() {
	// Command to configure an analog endstop
	RegisterCommand("config_analog_endstop", "oid=%c adc_oid=%c threshold=%u trigger_above=%c hysteresis=%u", handleConfigAnalogEndstop)

	// Command to start homing with an analog endstop
	RegisterCommand("analog_endstop_home", "oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u trsync_oid=%c trigger_reason=%c", handleAnalogEndstopHome)

	// Command to query analog endstop state
	RegisterCommand("analog_endstop_query_state", "oid=%c", handleAnalogEndstopQueryState)

	// Response: analog endstop state report
	RegisterResponse("analog_endstop_state", "oid=%c homing=%c next_clock=%u value=%u")
}

// handleConfigAnalogEndstop configures an analog endstop
// Format: config_analog_endstop oid=%c adc_oid=%c threshold=%u trigger_above=%c hysteresis=%u
func handleConfigAnalogEndstop(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	adcOID, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	threshold, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	triggerAbove, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	hysteresis, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get ADC object
	adc, exists := GetADC(uint8(adcOID))
	if !exists {
		return nil // Silently ignore if ADC not configured
	}

	// Create new analog endstop instance
	aes := &AnalogEndstop{
		OID:          uint8(oid),
		ADC:          adc,
		Threshold:    threshold,
		TriggerAbove: triggerAbove != 0,
		Hysteresis:   hysteresis,
	}

	// Register in global map
	analogEndstops[uint8(oid)] = aes

	return nil
}

// handleAnalogEndstopHome starts homing with an analog endstop
// Format: analog_endstop_home oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u trsync_oid=%c trigger_reason=%c
func handleAnalogEndstopHome(data *[]byte) error {
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

	// Get analog endstop object
	aes, exists := analogEndstops[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Cancel any existing timer
	aes.Timer.Next = nil

	// If sample_count is 0, disable homing
	if sampleCount == 0 {
		aes.TriggerSync = nil
		aes.Flags = 0
		return nil
	}

	// Get trigger sync object
	ts, exists := GetTriggerSync(uint8(trsyncOID))
	if !exists {
		return nil // Silently ignore if trsync not configured
	}

	// Configure homing parameters
	aes.SampleTime = sampleTicks
	aes.SampleCount = uint8(sampleCount)
	aes.TriggerCount = uint8(sampleCount)
	aes.RestTime = restTicks
	aes.TriggerSync = ts
	aes.TriggerReason = uint8(triggerReason)
	aes.Flags = ESF_HOMING

	// Schedule initial timer
	aes.Timer.WakeTime = clock
	aes.Timer.Handler = analogEndstopEvent
	ScheduleTimer(&aes.Timer)

	return nil
}

// handleAnalogEndstopQueryState queries the current analog endstop state
// Format: analog_endstop_query_state oid=%c
func handleAnalogEndstopQueryState(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get analog endstop object
	aes, exists := analogEndstops[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Read current state
	state := disableInterrupts()
	eflags := aes.Flags
	nextwake := aes.NextWake
	restoreInterrupts(state)

	// Read ADC value
	value := uint32(0)
	if aes.ADC != nil {
		value = uint32(aes.ADC.PendingValue)
	}

	// Send response
	homing := uint32(0)
	if (eflags & ESF_HOMING) != 0 {
		homing = 1
	}

	SendResponse("analog_endstop_state", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(oid))
		protocol.EncodeVLQUint(output, homing)
		protocol.EncodeVLQUint(output, nextwake)
		protocol.EncodeVLQUint(output, value)
	})

	return nil
}

// analogEndstopEvent is the timer callback for analog endstop checking
func analogEndstopEvent(t *Timer) uint8 {
	// Find the AnalogEndstop instance that owns this timer
	var aes *AnalogEndstop
	for _, aesPtr := range analogEndstops {
		if aesPtr != nil && &aesPtr.Timer == t {
			aes = aesPtr
			break
		}
	}

	if aes == nil {
		return SF_DONE
	}

	// Read ADC value
	if aes.ADC == nil {
		return SF_DONE
	}

	value := uint32(aes.ADC.PendingValue)

	// Check if value crosses threshold
	var triggered bool
	if aes.TriggerAbove {
		// Trigger when value rises above threshold
		triggered = value > aes.Threshold
	} else {
		// Trigger when value falls below threshold
		triggered = value < aes.Threshold
	}

	nextWake := t.WakeTime + aes.RestTime

	if !triggered {
		// No match - reschedule for the next attempt
		t.WakeTime = nextWake
		return SF_RESCHEDULE
	}

	// Potential trigger detected - start oversampling
	aes.NextWake = nextWake
	t.Handler = analogEndstopOversampleEvent
	return analogEndstopOversampleEvent(t)
}

// analogEndstopOversampleEvent is the timer callback for oversampling
func analogEndstopOversampleEvent(t *Timer) uint8 {
	// Find the AnalogEndstop instance that owns this timer
	var aes *AnalogEndstop
	for _, aesPtr := range analogEndstops {
		if aesPtr != nil && &aesPtr.Timer == t {
			aes = aesPtr
			break
		}
	}

	if aes == nil {
		return SF_DONE
	}

	// Read ADC value
	if aes.ADC == nil {
		return SF_DONE
	}

	value := uint32(aes.ADC.PendingValue)

	// Check if value still crosses threshold (with hysteresis)
	var triggered bool
	if aes.TriggerAbove {
		// Trigger when value rises above threshold
		// Use hysteresis to prevent oscillation
		triggered = value > (aes.Threshold - aes.Hysteresis)
	} else {
		// Trigger when value falls below threshold
		// Use hysteresis to prevent oscillation
		triggered = value < (aes.Threshold + aes.Hysteresis)
	}

	if !triggered {
		// No longer matching - reschedule for the next attempt
		t.Handler = analogEndstopEvent
		t.WakeTime = aes.NextWake
		aes.TriggerCount = aes.SampleCount
		return SF_RESCHEDULE
	}

	// Decrement trigger count
	count := aes.TriggerCount - 1
	if count == 0 {
		// All samples confirmed - trigger!
		if aes.TriggerSync != nil {
			TriggerSyncDoTrigger(aes.TriggerSync, aes.TriggerReason)
		}
		return SF_DONE
	}

	// Continue oversampling
	aes.TriggerCount = count
	t.WakeTime += aes.SampleTime
	return SF_RESCHEDULE
}
