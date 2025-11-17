// Endstop handling for GPIO-based sensors
// Implements Klipper's endstop protocol for mechanical switches, hall effect sensors, etc.
package core

import (
	"gopper/protocol"
)

// Endstop flags
const (
	ESF_PIN_HIGH = 1 << 0 // Expected pin state when triggered (1=high, 0=low)
	ESF_HOMING   = 1 << 1 // Currently homing
)

// Endstop represents a configured GPIO endstop
type Endstop struct {
	OID           uint8        // Object ID
	Pin           GPIOPin      // GPIO pin for endstop input
	Flags         uint8        // State flags (ESF_*)
	Timer         Timer        // Timer for sampling
	SampleTime    uint32       // Time between samples (in ticks)
	SampleCount   uint8        // Number of consecutive samples required
	TriggerCount  uint8        // Remaining samples before trigger
	RestTime      uint32       // Rest time between check cycles
	NextWake      uint32       // Next scheduled wake time
	TriggerSync   *TriggerSync // Associated trigger synchronization object
	TriggerReason uint8        // Reason code to report when triggered
}

// Global registry of endstops
var endstops = make(map[uint8]*Endstop)

// InitEndstopCommands registers endstop-related commands
func InitEndstopCommands() {
	// Command to configure an endstop
	RegisterCommand("config_endstop", "oid=%c pin=%u pull_up=%c", handleConfigEndstop)

	// Command to start homing with an endstop
	RegisterCommand("endstop_home", "oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u pin_value=%c trsync_oid=%c trigger_reason=%c", handleEndstopHome)

	// Command to query endstop state
	RegisterCommand("endstop_query_state", "oid=%c", handleEndstopQueryState)

	// Response: endstop state report
	RegisterResponse("endstop_state", "oid=%c homing=%c next_clock=%u pin_value=%c")
}

// handleConfigEndstop configures a GPIO endstop
// Format: config_endstop oid=%c pin=%u pull_up=%c
func handleConfigEndstop(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	pin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	pullUp, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Create new endstop instance
	es := &Endstop{
		OID: uint8(oid),
		Pin: GPIOPin(pin),
	}

	// Configure GPIO pin as input via HAL
	// Pull-up/pull-down configuration
	if pullUp != 0 {
		if err := MustGPIO().ConfigureInputPullUp(es.Pin); err != nil {
			return err
		}
	} else {
		if err := MustGPIO().ConfigureInputPullDown(es.Pin); err != nil {
			return err
		}
	}

	// Register in global map
	endstops[uint8(oid)] = es

	return nil
}

// handleEndstopHome starts homing with an endstop
// Format: endstop_home oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u pin_value=%c trsync_oid=%c trigger_reason=%c
func handleEndstopHome(data *[]byte) error {
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

	pinValue, err := protocol.DecodeVLQUint(data)
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

	// Get endstop object
	es, exists := endstops[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Cancel any existing timer
	// Note: In a real implementation, we'd need sched_del_timer
	// For now, we clear the Next pointer
	es.Timer.Next = nil

	// If sample_count is 0, disable homing
	if sampleCount == 0 {
		es.TriggerSync = nil
		es.Flags = 0
		return nil
	}

	// Get trigger sync object
	ts, exists := GetTriggerSync(uint8(trsyncOID))
	if !exists {
		return nil // Silently ignore if trsync not configured
	}

	// Configure homing parameters
	es.SampleTime = sampleTicks
	es.SampleCount = uint8(sampleCount)
	es.TriggerCount = uint8(sampleCount)
	es.RestTime = restTicks
	es.TriggerSync = ts
	es.TriggerReason = uint8(triggerReason)
	es.Flags = ESF_HOMING

	// Set expected pin value flag
	if pinValue != 0 {
		es.Flags |= ESF_PIN_HIGH
	}

	// Schedule initial timer
	es.Timer.WakeTime = clock
	es.Timer.Handler = endstopEvent
	ScheduleTimer(&es.Timer)

	return nil
}

// handleEndstopQueryState queries the current endstop state
// Format: endstop_query_state oid=%c
func handleEndstopQueryState(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get endstop object
	es, exists := endstops[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Read current pin state
	state := disableInterrupts()
	eflags := es.Flags
	nextwake := es.NextWake
	restoreInterrupts(state)

	// Read pin value
	pinValue := uint32(0)
	if MustGPIO().ReadPin(es.Pin) {
		pinValue = 1
	}

	// Send response
	homing := uint32(0)
	if (eflags & ESF_HOMING) != 0 {
		homing = 1
	}

	SendResponse("endstop_state", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(oid))
		protocol.EncodeVLQUint(output, homing)
		protocol.EncodeVLQUint(output, nextwake)
		protocol.EncodeVLQUint(output, pinValue)
	})

	return nil
}

// endstopEvent is the timer callback for endstop checking
// This is the first-stage check that looks for a potential trigger
func endstopEvent(t *Timer) uint8 {
	// Find the Endstop instance that owns this timer
	var es *Endstop
	for _, esPtr := range endstops {
		if esPtr != nil && &esPtr.Timer == t {
			es = esPtr
			break
		}
	}

	if es == nil {
		return SF_DONE
	}

	// Read pin state
	pinHigh := MustGPIO().ReadPin(es.Pin)

	// Check if pin matches expected trigger state
	// If ESF_PIN_HIGH is set, we expect high (true)
	// If ESF_PIN_HIGH is clear, we expect low (false)
	expectHigh := (es.Flags & ESF_PIN_HIGH) != 0
	triggered := (pinHigh && expectHigh) || (!pinHigh && !expectHigh)

	nextWake := t.WakeTime + es.RestTime

	if !triggered {
		// No match - reschedule for the next attempt
		t.WakeTime = nextWake
		return SF_RESCHEDULE
	}

	// Potential trigger detected - start oversampling
	es.NextWake = nextWake
	t.Handler = endstopOversampleEvent
	return endstopOversampleEvent(t)
}

// endstopOversampleEvent is the timer callback for oversampling
// This confirms the trigger by taking multiple consecutive samples
func endstopOversampleEvent(t *Timer) uint8 {
	// Find the Endstop instance that owns this timer
	var es *Endstop
	for _, esPtr := range endstops {
		if esPtr != nil && &esPtr.Timer == t {
			es = esPtr
			break
		}
	}

	if es == nil {
		return SF_DONE
	}

	// Read pin state
	pinHigh := MustGPIO().ReadPin(es.Pin)

	// Check if pin still matches expected trigger state
	expectHigh := (es.Flags & ESF_PIN_HIGH) != 0
	triggered := (pinHigh && expectHigh) || (!pinHigh && !expectHigh)

	if !triggered {
		// No longer matching - reschedule for the next attempt
		t.Handler = endstopEvent
		t.WakeTime = es.NextWake
		es.TriggerCount = es.SampleCount
		return SF_RESCHEDULE
	}

	// Decrement trigger count
	count := es.TriggerCount - 1
	if count == 0 {
		// All samples confirmed - trigger!
		if es.TriggerSync != nil {
			TriggerSyncDoTrigger(es.TriggerSync, es.TriggerReason)
		}
		return SF_DONE
	}

	// Continue oversampling
	es.TriggerCount = count
	t.WakeTime += es.SampleTime
	return SF_RESCHEDULE
}
