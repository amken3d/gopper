//go:build tinygo

// ADC (Analog to Digital Converter) support
// Implements Klipper's analog_in protocol for reading analog sensors
package core

import (
	"gopper/protocol"
)

// ADC states
const (
	ADCStateIdle     = 0
	ADCStateReady    = 1
	ADCStateSampling = 2
	// ADCStateReportPending indicates that a sample cycle has completed
	// and an analog_in_state message needs to be sent from the task context.
	ADCStateReportPending = 3
)

// AnalogIn represents a configured ADC input channel
type AnalogIn struct {
	OID   uint8  // Object ID
	Pin   uint32 // Hardware pin number (interpreted as ADCChannelID by HAL)
	State uint8  // Current state (idle, ready, sampling)

	// Timer for periodic sampling
	Timer Timer

	// Timing parameters
	RestTime      uint32 // Ticks between reporting cycles
	SampleTime    uint32 // Ticks between individual samples
	NextBeginTime uint32 // When next sampling cycle begins

	// Sampling parameters
	SampleCount   uint8 // Number of samples to oversample
	CurrentSample uint8 // Current sample index

	// Value tracking
	Value uint32 // Accumulated ADC value (sum of samples)

	// Range checking
	MinValue        uint16 // Minimum acceptable value
	MaxValue        uint16 // Maximum acceptable value
	RangeCheckCount uint8  // Number of violations before shutdown
	InvalidCount    uint8  // Current violation count
	// Pending report (value that analogInTask will send)
	PendingValue uint16
}

// Global registry of analog inputs
var analogInputs = make(map[uint8]*AnalogIn)

// Wake flag for analog-in task
var analogInWake bool

// InitADCCommands registers ADC-related commands with the command registry
func InitADCCommands() {
	// Command to configure an analog input pin
	RegisterCommand("config_analog_in", "oid=%c pin=%u", handleConfigAnalogIn)

	// Command to start periodic sampling
	RegisterCommand("query_analog_in", "oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u min_value=%hu max_value=%hu range_check_count=%c", handleQueryAnalogIn)

	// Response message: analog value update (MCU → Host)
	RegisterCommand("analog_in_state", "oid=%c next_clock=%u value=%hu", nil)

	// Register ADC constants
	// ADC_MAX: Maximum ADC value for 12-bit ADC (0-4095)
	RegisterConstant("ADC_MAX", uint32(4095))

}

// handleConfigAnalogIn configures a pin for analog input sampling
func handleConfigAnalogIn(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	pin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Create new analog input instance
	ain := &AnalogIn{
		OID:   uint8(oid),
		Pin:   pin,
		State: ADCStateReady,
	}

	// Initialize the ADC hardware for this pin via HAL
	// We treat the Klipper "pin" value as a logical ADCChannelID understood by the target.
	if err := MustADC().ConfigureChannel(ADCChannelID(pin)); err != nil {
		return err
	}

	// Register in global map
	analogInputs[uint8(oid)] = ain

	return nil
}

// handleQueryAnalogIn starts periodic analog sampling
func handleQueryAnalogIn(data *[]byte) error {
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

	minValue, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	maxValue, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	rangeCheckCount, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the analog input object
	ain, exists := analogInputs[uint8(oid)]
	if !exists {
		// Invalid OID - analog input not configured
		return nil
	}

	// Configure sampling parameters
	ain.SampleTime = sampleTicks
	ain.SampleCount = uint8(sampleCount)
	ain.RestTime = restTicks
	ain.MinValue = uint16(minValue)
	ain.MaxValue = uint16(maxValue)
	ain.RangeCheckCount = uint8(rangeCheckCount)
	ain.NextBeginTime = clock

	// Reset state
	ain.Value = 0
	ain.CurrentSample = 0
	ain.InvalidCount = 0

	// If sample count is zero, match Klipper semantics: do not schedule sampling.
	if ain.SampleCount == 0 {
		ain.State = ADCStateReady
		// Optionally ensure the timer is not queued anymore
		ain.Timer.Next = nil
		return nil
	}

	// Enable sampling
	ain.State = ADCStateSampling

	// Schedule first sample
	// IMPORTANT: Clear Next pointer to avoid issues if timer was previously scheduled
	ain.Timer.Next = nil
	ain.Timer.WakeTime = clock
	ain.Timer.Handler = analogInTimerHandler
	ScheduleTimer(&ain.Timer)

	return nil
}

// wakeAnalogInTask marks the analog-in task as needing to run.
func wakeAnalogInTask() {
	state := disableInterrupts()
	analogInWake = true
	restoreInterrupts(state)
}

// AnalogInTask mirrors Klipper's analog_in_task:
// it runs in task context, sends analog_in_state messages for any
// AnalogIn that has completed a sample cycle, and advances its state.
func AnalogInTask() {
	// Fast check with interrupt protection to avoid races with the timer.
	state := disableInterrupts()
	if !analogInWake {
		restoreInterrupts(state)
		return
	}
	analogInWake = false
	restoreInterrupts(state)

	// Iterate all configured analog inputs and send any pending reports.
	for oid, ain := range analogInputs {
		if ain == nil {
			continue
		}

		// We only send when the timer has marked this channel as report pending.
		if ain.State != ADCStateReportPending {
			continue
		}

		// Take a snapshot of the fields that the timer may also touch.
		state = disableInterrupts()
		if ain.State != ADCStateReportPending {
			// State changed while we were waiting; skip.
			restoreInterrupts(state)
			continue
		}
		value := ain.PendingValue
		nextClock := ain.NextBeginTime

		// Mark that the report has been sent; the timer will later
		// drive the state back into ADCStateSampling when the next
		// sampling cycle starts.
		ain.State = ADCStateReady
		restoreInterrupts(state)

		// Send analog_in_state response from task context.
		SendResponse("analog_in_state", func(output protocol.OutputBuffer) {
			protocol.EncodeVLQUint(output, uint32(oid))
			protocol.EncodeVLQUint(output, nextClock)
			protocol.EncodeVLQUint(output, uint32(value))
		})
	}
}

// analogInTimerHandler is the timer callback for ADC sampling
func analogInTimerHandler(t *Timer) uint8 {
	// Find the AnalogIn instance that owns this timer
	// Note: analogInputs is map[uint8]*AnalogIn, so we need to compare timer addresses correctly
	var ain *AnalogIn
	for _, aPtr := range analogInputs {
		if aPtr != nil && &aPtr.Timer == t {
			ain = aPtr
			break
		}
	}

	if ain == nil {
		// Timer fired but no AnalogIn found - should not happen
		// Return SF_DONE to remove timer from schedule
		return SF_DONE
	}

	if ain.State != ADCStateSampling {
		return SF_DONE
	}

	// If sample count is zero, match Klipper semantics: do not sample or report
	// and return to the ready state.
	if ain.SampleCount == 0 {
		ain.State = ADCStateReady
		return SF_DONE
	}

	// Read ADC sample synchronously via HAL
	value, err := MustADC().ReadRaw(ADCChannelID(ain.Pin))
	if err != nil {
		// On read failure, stop sampling this input
		ain.State = ADCStateReady
		return SF_DONE
	}

	// Accumulate sample value (sum, as in Klipper)
	ain.Value += uint32(value)
	ain.CurrentSample++

	// Check if we've collected all samples
	if ain.CurrentSample >= ain.SampleCount {
		// All samples collected, perform range check

		// Range checking on the accumulated 16-bit sum (Klipper uses uint16_t sum)
		sum16 := uint16(ain.Value) // truncation matches Klipper's uint16 accumulator

		if sum16 < ain.MinValue || sum16 > ain.MaxValue {
			ain.InvalidCount++

			// Match Klipper semantics:
			//  - RangeCheckCount == 0 => shutdown on first violation
			//  - RangeCheckCount > 0  => shutdown after N consecutive violations
			if ain.RangeCheckCount == 0 || ain.InvalidCount >= ain.RangeCheckCount {
				TryShutdown("ADC out of range")
				ain.InvalidCount = 0
			}
		} else {
			// Value in range, reset invalid count
			ain.InvalidCount = 0
		}

		// Calculate next reporting cycle
		ain.NextBeginTime += ain.RestTime

		// Stash the value and mark report pending; the task will send it.
		ain.PendingValue = sum16
		ain.State = ADCStateReportPending

		// Reset for next cycle's accumulation
		ain.Value = 0
		ain.CurrentSample = 0

		// Schedule next sampling cycle
		t.WakeTime = ain.NextBeginTime

		// Wake the analog-in task to send the report from task context
		wakeAnalogInTask()

		return SF_RESCHEDULE
	}

	// More samples needed, schedule next sample
	t.WakeTime = GetTime() + ain.SampleTime
	return SF_RESCHEDULE
}

// ShutdownAnalogIn stops sampling for an analog input (called during shutdown)
func ShutdownAnalogIn(ain *AnalogIn) {
	if ain.State == ADCStateSampling || ain.State == ADCStateReportPending {
		// Stop any further activity on this input.
		ain.State = ADCStateReady
	}
	ain.PendingValue = 0
	// Ensure its timer is no longer scheduled
	ain.Timer.Next = nil
}

// ShutdownAllAnalogIn stops sampling on all configured analog inputs.
// Call this from your global shutdown handler to mirror Klipper's behavior.
func ShutdownAllAnalogIn() {
	for _, ain := range analogInputs {
		if ain != nil {
			ShutdownAnalogIn(ain)
		}
	}
}
