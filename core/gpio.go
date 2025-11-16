// GPIO (General Purpose Input/Output) support
// Implements Klipper's digital_out protocol for controlling GPIO pins
package core

import (
	"gopper/protocol"
)

// DigitalOut flags
const (
	DF_ON         = 1 << 0 // Current pin state (1=high, 0=low)
	DF_TOGGLING   = 1 << 1 // PWM mode active
	DF_CHECK_END  = 1 << 2 // Monitor max_duration
	DF_DEFAULT_ON = 1 << 3 // Default state for shutdown/power-loss
)

// DigitalOut represents a configured GPIO output pin
type DigitalOut struct {
	OID   uint8   // Object ID
	Pin   GPIOPin // Hardware pin
	Flags uint8   // State flags (DF_*)

	// Timers for scheduled operations
	Timer Timer // Main timer for scheduled updates and PWM

	// PWM timing
	OnDuration  uint32 // PWM on time in ticks
	OffDuration uint32 // PWM off time in ticks
	CycleTime   uint32 // Total PWM cycle time in ticks
	EndTime     uint32 // Time when max_duration expires

	// Safety parameters
	MaxDuration uint32 // Maximum time pin can be in non-default state
}

// Global registry of digital outputs
var digitalOutputs = make(map[uint8]*DigitalOut)

// InitGPIOCommands registers GPIO-related commands with the command registry
func InitGPIOCommands() {
	// Command to configure a digital output pin
	RegisterCommand("config_digital_out", "oid=%c pin=%u value=%c default_value=%c max_duration=%u", handleConfigDigitalOut)

	// Command to queue a scheduled pin change
	RegisterCommand("queue_digital_out", "oid=%c clock=%u on_ticks=%u", handleQueueDigitalOut)

	// Command to immediately update a pin value
	RegisterCommand("update_digital_out", "oid=%c value=%c", handleUpdateDigitalOut)

	// Command to set PWM cycle time
	RegisterCommand("set_digital_out_pwm_cycle", "oid=%c cycle_ticks=%u", handleSetDigitalOutPWMCycle)
}

// handleConfigDigitalOut configures a pin for digital output
// Format: config_digital_out oid=%c pin=%u value=%c default_value=%c max_duration=%u
func handleConfigDigitalOut(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	pin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	value, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	defaultValue, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	maxDuration, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Create new digital output instance
	dout := &DigitalOut{
		OID:         uint8(oid),
		Pin:         GPIOPin(pin),
		MaxDuration: maxDuration,
		Flags:       0,
	}

	// Set default value flag
	if defaultValue != 0 {
		dout.Flags |= DF_DEFAULT_ON
	}

	// Configure GPIO pin via HAL
	if err := MustGPIO().ConfigureOutput(dout.Pin); err != nil {
		return err
	}

	// Set initial value
	initialState := value != 0
	if err := MustGPIO().SetPin(dout.Pin, initialState); err != nil {
		return err
	}

	// Set current state flag
	if initialState {
		dout.Flags |= DF_ON
	}

	// Register in global map
	digitalOutputs[uint8(oid)] = dout

	return nil
}

// handleQueueDigitalOut schedules a pin state change
// Format: queue_digital_out oid=%c clock=%u on_ticks=%u
func handleQueueDigitalOut(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	clock, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	onTicks, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the digital output object
	dout, exists := digitalOutputs[uint8(oid)]
	if !exists {
		// Invalid OID - digital output not configured
		return nil
	}

	// If PWM cycle is configured, use it for PWM mode
	if dout.CycleTime != 0 {
		dout.OnDuration = onTicks
		dout.OffDuration = dout.CycleTime - onTicks

		// Validate on_ticks doesn't exceed cycle time
		if dout.OnDuration > dout.CycleTime {
			dout.OnDuration = dout.CycleTime
			dout.OffDuration = 0
		}

		// Enable toggling if we have both on and off periods
		if dout.OnDuration > 0 && dout.OffDuration > 0 {
			dout.Flags |= DF_TOGGLING
		} else {
			dout.Flags &^= DF_TOGGLING
			// Pure on or pure off
			if dout.OnDuration > 0 {
				dout.Flags |= DF_ON
			} else {
				dout.Flags &^= DF_ON
			}
		}
	} else {
		// No PWM cycle - simple on/off
		if onTicks > 0 {
			dout.Flags |= DF_ON
		} else {
			dout.Flags &^= DF_ON
		}
		dout.Flags &^= DF_TOGGLING
	}

	// Update max_duration end time if needed
	if dout.MaxDuration != 0 {
		// Check if new state differs from default
		newStateOn := (dout.Flags & DF_ON) != 0
		defaultOn := (dout.Flags & DF_DEFAULT_ON) != 0

		if newStateOn != defaultOn {
			dout.EndTime = clock + dout.MaxDuration
			dout.Flags |= DF_CHECK_END
		} else {
			dout.Flags &^= DF_CHECK_END
		}
	}

	// Schedule the timer to execute at the specified clock time
	// Clear Next pointer to avoid issues if timer was previously scheduled
	dout.Timer.Next = nil
	dout.Timer.WakeTime = clock
	dout.Timer.Handler = digitalOutLoadEvent
	ScheduleTimer(&dout.Timer)

	return nil
}

// handleUpdateDigitalOut immediately updates a pin value
// Format: update_digital_out oid=%c value=%c
func handleUpdateDigitalOut(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	value, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the digital output object
	dout, exists := digitalOutputs[uint8(oid)]
	if !exists {
		// Invalid OID - digital output not configured
		return nil
	}

	// Update pin state immediately
	state := value != 0
	if err := MustGPIO().SetPin(dout.Pin, state); err != nil {
		return err
	}

	// Update flags
	if state {
		dout.Flags |= DF_ON
	} else {
		dout.Flags &^= DF_ON
	}

	// Disable toggling mode
	dout.Flags &^= DF_TOGGLING

	return nil
}

// handleSetDigitalOutPWMCycle sets the PWM cycle time
// Format: set_digital_out_pwm_cycle oid=%c cycle_ticks=%u
func handleSetDigitalOutPWMCycle(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	cycleTicks, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the digital output object
	dout, exists := digitalOutputs[uint8(oid)]
	if !exists {
		// Invalid OID - digital output not configured
		return nil
	}

	// Set cycle time
	dout.CycleTime = cycleTicks

	return nil
}

// digitalOutLoadEvent is the timer handler for loading scheduled pin updates
// This executes at the scheduled time and sets up PWM toggling if needed
func digitalOutLoadEvent(t *Timer) uint8 {
	// Find the DigitalOut instance that owns this timer
	var dout *DigitalOut
	for _, dPtr := range digitalOutputs {
		if dPtr != nil && &dPtr.Timer == t {
			dout = dPtr
			break
		}
	}

	if dout == nil {
		// Timer fired but no DigitalOut found - should not happen
		return SF_DONE
	}

	// Check if we're in toggling (PWM) mode
	if (dout.Flags & DF_TOGGLING) != 0 {
		// Start PWM cycle
		// Set pin to ON and schedule toggle
		if err := MustGPIO().SetPin(dout.Pin, true); err != nil {
			// On error, stop toggling
			dout.Flags &^= DF_TOGGLING
			return SF_DONE
		}

		// Schedule next toggle (to turn OFF)
		t.WakeTime = GetTime() + dout.OnDuration
		t.Handler = digitalOutToggleEvent
		return SF_RESCHEDULE
	}

	// Not toggling - simple on/off
	state := (dout.Flags & DF_ON) != 0
	if err := MustGPIO().SetPin(dout.Pin, state); err != nil {
		return SF_DONE
	}

	// Check if we need to monitor max_duration
	if (dout.Flags & DF_CHECK_END) != 0 {
		// Schedule a timer to enforce max_duration
		t.WakeTime = dout.EndTime
		t.Handler = digitalOutEndEvent
		return SF_RESCHEDULE
	}

	return SF_DONE
}

// digitalOutToggleEvent is the timer handler for PWM toggling
func digitalOutToggleEvent(t *Timer) uint8 {
	// Find the DigitalOut instance that owns this timer
	var dout *DigitalOut
	for _, dPtr := range digitalOutputs {
		if dPtr != nil && &dPtr.Timer == t {
			dout = dPtr
			break
		}
	}

	if dout == nil {
		return SF_DONE
	}

	// Stop toggling if flag is cleared
	if (dout.Flags & DF_TOGGLING) == 0 {
		return SF_DONE
	}

	// Toggle pin state
	currentState := (dout.Flags & DF_ON) != 0
	newState := !currentState

	if err := MustGPIO().SetPin(dout.Pin, newState); err != nil {
		// On error, stop toggling
		dout.Flags &^= DF_TOGGLING
		return SF_DONE
	}

	// Update state flag
	if newState {
		dout.Flags |= DF_ON
	} else {
		dout.Flags &^= DF_ON
	}

	// Schedule next toggle
	var nextDuration uint32
	if newState {
		// Just turned ON, schedule turn OFF
		nextDuration = dout.OnDuration
	} else {
		// Just turned OFF, schedule turn ON
		nextDuration = dout.OffDuration
	}

	// Check if we're approaching end time
	currentTime := GetTime()
	if (dout.Flags&DF_CHECK_END) != 0 && (currentTime+nextDuration >= dout.EndTime) {
		// Switch to load event handler to check end time
		t.WakeTime = dout.EndTime
		t.Handler = digitalOutLoadEvent
		return SF_RESCHEDULE
	}

	// Continue toggling
	t.WakeTime = currentTime + nextDuration
	return SF_RESCHEDULE
}

// digitalOutEndEvent is the timer handler for max_duration enforcement
func digitalOutEndEvent(t *Timer) uint8 {
	// Find the DigitalOut instance that owns this timer
	var dout *DigitalOut
	for _, dPtr := range digitalOutputs {
		if dPtr != nil && &dPtr.Timer == t {
			dout = dPtr
			break
		}
	}

	if dout == nil {
		return SF_DONE
	}

	// Max duration expired - return to default state
	defaultState := (dout.Flags & DF_DEFAULT_ON) != 0
	if err := MustGPIO().SetPin(dout.Pin, defaultState); err != nil {
		return SF_DONE
	}

	// Update flags
	if defaultState {
		dout.Flags |= DF_ON
	} else {
		dout.Flags &^= DF_ON
	}

	// Clear toggling and check_end flags
	dout.Flags &^= DF_TOGGLING | DF_CHECK_END

	return SF_DONE
}

// ShutdownDigitalOut returns a pin to its default state (called during shutdown)
func ShutdownDigitalOut(dout *DigitalOut) {
	// Return to default state
	defaultState := (dout.Flags & DF_DEFAULT_ON) != 0
	_ = MustGPIO().SetPin(dout.Pin, defaultState)

	// Update flags
	if defaultState {
		dout.Flags |= DF_ON
	} else {
		dout.Flags &^= DF_ON
	}

	// Clear toggling and check_end flags
	dout.Flags &^= DF_TOGGLING | DF_CHECK_END

	// Stop any scheduled timers
	dout.Timer.Next = nil
}

// ShutdownAllDigitalOut returns all pins to their default states
// Call this from the global shutdown handler to mirror Klipper's behavior
func ShutdownAllDigitalOut() {
	for _, dout := range digitalOutputs {
		if dout != nil {
			ShutdownDigitalOut(dout)
		}
	}
}
