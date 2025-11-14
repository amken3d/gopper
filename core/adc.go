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
)

// AnalogIn represents a configured ADC input channel
type AnalogIn struct {
	OID   uint8  // Object ID
	Pin   uint32 // Hardware pin number
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
}

// Global registry of analog inputs
var analogInputs = make(map[uint8]*AnalogIn)

// InitADCCommands registers ADC-related commands with the command registry
func InitADCCommands() {
	// Command to configure an analog input pin
	RegisterCommand("config_analog_in", "oid=%c pin=%u", handleConfigAnalogIn)

	// Command to start periodic sampling
	RegisterCommand("query_analog_in", "oid=%c clock=%u sample_ticks=%u sample_count=%c rest_ticks=%u min_value=%hu max_value=%hu range_check_count=%c", handleQueryAnalogIn)

	// Response message: analog value update (MCU → Host)
	RegisterCommand("analog_in_state", "oid=%c next_clock=%u value=%hu", nil)
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

	// Initialize the ADC hardware for this pin
	err = ADCSetup(pin)
	if err != nil {
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
	ain.State = ADCStateSampling

	// Schedule first sample
	ain.Timer.WakeTime = clock
	ain.Timer.Handler = analogInTimerHandler
	ScheduleTimer(&ain.Timer)

	return nil
}

// analogInTimerHandler is the timer callback for ADC sampling
func analogInTimerHandler(t *Timer) uint8 {
	// Find the AnalogIn instance that owns this timer
	var ain *AnalogIn
	for _, a := range analogInputs {
		if &a.Timer == t {
			ain = a
			break
		}
	}

	if ain == nil || ain.State != ADCStateSampling {
		return SF_DONE
	}

	// Try to read ADC sample
	value, ready := ADCSample(ain.Pin)

	if !ready {
		// Sample not ready yet, reschedule soon
		t.WakeTime = GetTime() + 100 // Small delay (adjust based on ADC conversion time)
		return SF_RESCHEDULE
	}

	// Accumulate sample value
	ain.Value += uint32(value)
	ain.CurrentSample++

	// Check if we've collected all samples
	if ain.CurrentSample >= ain.SampleCount {
		// All samples collected, send report

		// Range checking (if enabled)
		if ain.RangeCheckCount > 0 {
			// Average the samples for range check
			avgValue := uint16(ain.Value / uint32(ain.SampleCount))

			if avgValue < ain.MinValue || avgValue > ain.MaxValue {
				ain.InvalidCount++

				if ain.InvalidCount >= ain.RangeCheckCount {
					// Trigger shutdown - ADC out of range
					TryShutdown("ADC out of range")
					ain.InvalidCount = 0
				}
			} else {
				// Value in range, reset invalid count
				ain.InvalidCount = 0
			}
		}

		// Calculate next reporting cycle
		ain.NextBeginTime += ain.RestTime

		// Send analog_in_state response
		SendResponse("analog_in_state", func(output protocol.OutputBuffer) {
			protocol.EncodeVLQUint(output, uint32(ain.OID))
			protocol.EncodeVLQUint(output, ain.NextBeginTime)
			// Send accumulated value (sum of all samples, not average)
			protocol.EncodeVLQUint(output, ain.Value)
		})

		// Reset for next cycle
		ain.Value = 0
		ain.CurrentSample = 0

		// Schedule next sampling cycle
		t.WakeTime = ain.NextBeginTime
		return SF_RESCHEDULE
	} else {
		// More samples needed, schedule next sample
		t.WakeTime = GetTime() + ain.SampleTime
		return SF_RESCHEDULE
	}
}

// ShutdownAnalogIn stops sampling for an analog input (called during shutdown)
func ShutdownAnalogIn(ain *AnalogIn) {
	if ain.State == ADCStateSampling {
		// Cancel any pending ADC operation
		ADCCancel(ain.Pin)
		ain.State = ADCStateReady
	}
}
