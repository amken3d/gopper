//go:build tinygo

// PWM (Pulse Width Modulation) support
// Implements Klipper's hardware PWM protocol for controlling PWM outputs
package core

import (
	"gopper/protocol"
)

// HardwarePWM flags
const (
	PWM_CHECK_END = 1 << 0 // Monitor max_duration
)

// HardwarePWM represents a configured hardware PWM output
type HardwarePWM struct {
	OID   uint8  // Object ID
	Pin   PWMPin // Hardware pin
	Flags uint8  // State flags

	// Timer for scheduled operations
	Timer Timer // Timer for scheduled PWM updates and max_duration

	// PWM configuration
	CycleTicks uint32   // PWM cycle time in ticks
	Value      PWMValue // Current PWM value (0-255)

	// Safety parameters
	DefaultValue PWMValue // Default value for shutdown/power-loss
	MaxDuration  uint32   // Maximum time pin can be in non-default state
	EndTime      uint32   // Time when max_duration expires
}

// Global registry of hardware PWM outputs
var hardwarePWMs = make(map[uint8]*HardwarePWM)

// InitPWMCommands registers PWM-related commands with the command registry
func InitPWMCommands() {
	// Command to configure a hardware PWM output pin
	RegisterCommand("config_pwm_out", "oid=%c pin=%u cycle_ticks=%u value=%hu default_value=%hu max_duration=%u", handleConfigPWMOut)

	// Command to queue a scheduled PWM value change
	RegisterCommand("queue_pwm_out", "oid=%c clock=%u value=%hu", handleQueuePWMOut)

	// Command to immediately set a PWM value
	RegisterCommand("set_pwm_out", "oid=%c value=%hu", handleSetPWMOut)

	// Register PWM constant
	// PWM_MAX: Maximum PWM value (matches Klipper's 255)
	// Note: We use 255 directly instead of calling MustPWM() because
	// InitPWMCommands() is called before the PWM driver is set
	RegisterConstant("PWM_MAX", 255)
}

// handleConfigPWMOut configures a pin for hardware PWM output
// Format: config_pwm_out oid=%c pin=%u cycle_ticks=%u value=%hu default_value=%hu max_duration=%u
func handleConfigPWMOut(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	pin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	cycleTicks, err := protocol.DecodeVLQUint(data)
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

	// Configure hardware PWM via HAL
	actualCycleTicks, err := MustPWM().ConfigureHardwarePWM(PWMPin(pin), cycleTicks)
	if err != nil {
		return err
	}

	// Create new hardware PWM instance
	pwm := &HardwarePWM{
		OID:          uint8(oid),
		Pin:          PWMPin(pin),
		CycleTicks:   actualCycleTicks,
		Value:        PWMValue(value),
		DefaultValue: PWMValue(defaultValue),
		MaxDuration:  maxDuration,
		Flags:        0,
	}

	// Set initial PWM value
	if err := MustPWM().SetDutyCycle(pwm.Pin, pwm.Value); err != nil {
		return err
	}

	// Register in global map
	hardwarePWMs[uint8(oid)] = pwm

	return nil
}

// handleQueuePWMOut schedules a PWM value change
// Format: queue_pwm_out oid=%c clock=%u value=%hu
func handleQueuePWMOut(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	clock, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	value, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the hardware PWM object
	pwm, exists := hardwarePWMs[uint8(oid)]
	if !exists {
		// Invalid OID - PWM not configured
		return nil
	}

	// Store the new value to apply at scheduled time
	pwm.Value = PWMValue(value)

	// Update max_duration end time if needed
	if pwm.MaxDuration != 0 {
		// Check if new value differs from default
		if pwm.Value != pwm.DefaultValue {
			pwm.EndTime = clock + pwm.MaxDuration
			pwm.Flags |= PWM_CHECK_END
		} else {
			pwm.Flags &^= PWM_CHECK_END
		}
	}

	// Schedule the timer to execute at the specified clock time
	// Clear Next pointer to avoid issues if timer was previously scheduled
	pwm.Timer.Next = nil
	pwm.Timer.WakeTime = clock
	pwm.Timer.Handler = pwmLoadEvent
	ScheduleTimer(&pwm.Timer)

	return nil
}

// handleSetPWMOut immediately sets a PWM value
// Format: set_pwm_out oid=%c value=%hu
func handleSetPWMOut(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	value, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the hardware PWM object
	pwm, exists := hardwarePWMs[uint8(oid)]
	if !exists {
		// Invalid OID - PWM not configured
		return nil
	}

	// Set PWM value immediately
	pwm.Value = PWMValue(value)
	if err := MustPWM().SetDutyCycle(pwm.Pin, pwm.Value); err != nil {
		return err
	}

	return nil
}

// pwmLoadEvent is the timer handler for loading scheduled PWM updates
// This executes at the scheduled time and applies the PWM value
func pwmLoadEvent(t *Timer) uint8 {
	// Find the HardwarePWM instance that owns this timer
	var pwm *HardwarePWM
	for _, pPtr := range hardwarePWMs {
		if pPtr != nil && &pPtr.Timer == t {
			pwm = pPtr
			break
		}
	}

	if pwm == nil {
		// Timer fired but no HardwarePWM found - should not happen
		return SF_DONE
	}

	// Apply the scheduled PWM value to hardware
	if err := MustPWM().SetDutyCycle(pwm.Pin, pwm.Value); err != nil {
		// On error, stop further operations
		return SF_DONE
	}

	// Check if we need to monitor max_duration
	if (pwm.Flags & PWM_CHECK_END) != 0 {
		// Schedule a timer to enforce max_duration
		t.WakeTime = pwm.EndTime
		t.Handler = pwmEndEvent
		return SF_RESCHEDULE
	}

	return SF_DONE
}

// pwmEndEvent is the timer handler for max_duration enforcement
func pwmEndEvent(t *Timer) uint8 {
	// Find the HardwarePWM instance that owns this timer
	var pwm *HardwarePWM
	for _, pPtr := range hardwarePWMs {
		if pPtr != nil && &pPtr.Timer == t {
			pwm = pPtr
			break
		}
	}

	if pwm == nil {
		return SF_DONE
	}

	// Max duration expired - return to default value
	pwm.Value = pwm.DefaultValue
	if err := MustPWM().SetDutyCycle(pwm.Pin, pwm.Value); err != nil {
		return SF_DONE
	}

	// Clear check_end flag
	pwm.Flags &^= PWM_CHECK_END

	return SF_DONE
}

// ShutdownHardwarePWM returns a PWM output to its default value (called during shutdown)
func ShutdownHardwarePWM(pwm *HardwarePWM) {
	// Return to default value
	pwm.Value = pwm.DefaultValue
	_ = MustPWM().SetDutyCycle(pwm.Pin, pwm.Value)

	// Clear flags
	pwm.Flags &^= PWM_CHECK_END

	// Stop any scheduled timers
	pwm.Timer.Next = nil
}

// ShutdownAllHardwarePWM returns all PWM outputs to their default values
// Call this from the global shutdown handler to mirror Klipper's behavior
func ShutdownAllHardwarePWM() {
	for _, pwm := range hardwarePWMs {
		if pwm != nil {
			ShutdownHardwarePWM(pwm)
		}
	}
}
