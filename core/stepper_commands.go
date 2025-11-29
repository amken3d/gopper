package core

import (
	"errors"
	"gopper/protocol"
)

// Stepper command handlers for Klipper protocol
// Implements: config_stepper, queue_step, set_next_step_dir, reset_step_clock, stepper_get_position

// RegisterStepperCommands registers all stepper-related commands
func RegisterStepperCommands() {
	// NOTE: RegisterCommand now takes (name, format, handler) directly.
	// The Command struct is still used internally for the dictionary,
	// but registration is via this helper.

	// config_stepper: Initialize a stepper motor
	RegisterCommand("config_stepper",
		"oid=%c step_pin=%c dir_pin=%c invert_step=%c step_pulse_ticks=%u",
		cmdConfigStepper)

	// queue_step: Add a move to the stepper queue
	RegisterCommand("queue_step",
		"oid=%c interval=%u count=%hu add=%hi",
		cmdQueueStep)

	// set_next_step_dir: Set direction for next move
	RegisterCommand("set_next_step_dir",
		"oid=%c dir=%c",
		cmdSetNextStepDir)

	// reset_step_clock: Synchronize step timing
	RegisterCommand("reset_step_clock",
		"oid=%c clock=%u",
		cmdResetStepClock)

	// stepper_get_position: Query current position
	RegisterCommand("stepper_get_position",
		"oid=%c",
		cmdStepperGetPosition)

	// Debug command to get stepper info
	RegisterCommand("stepper_get_info",
		"oid=%c",
		cmdStepperGetInfo)

	RegisterCommand("stepper_stop_on_trigger",
		"oid=%c trsync_oid=%c",
		cmdStepperStopOnTrigger)

	// Response: stepper position query result
	RegisterResponse("stepper_position", "oid=%c pos=%i")
}

func cmdStepperStopOnTrigger(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}
	trsyncOID, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the stepper
	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	// Get the trsync
	ts, exists := GetTriggerSync(uint8(trsyncOID))
	if !exists {
		return errors.New("trsync not found")
	}

	// Register callback to stop stepper when trigger fires
	TriggerSyncAddSignal(ts, func(reason uint8) {
		// Stop the stepper by clearing its queue and canceling the timer
		stepper.QueueHead = 0
		stepper.QueueTail = 0
		stepper.CurrentCount = 0
		// The timer will naturally stop when it finds no more moves
	})

	return nil
}

// cmdConfigStepper handles config_stepper command
// Format: oid=%c step_pin=%c dir_pin=%c invert_step=%c min_stop_interval=%u
func cmdConfigStepper(data *[]byte) error {
	DebugPrintln("[STEPPER] config_stepper called")

	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: failed to decode oid")
		return err
	}

	stepPin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: failed to decode step_pin")
		return err
	}

	dirPin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: failed to decode dir_pin")
		return err
	}

	invertStep, err := protocol.DecodeVLQUint(data)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: failed to decode invert_step")
		return err
	}

	minStopInterval, err := protocol.DecodeVLQUint(data)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: failed to decode min_stop_interval")
		return err
	}

	DebugPrintln("[STEPPER] config_stepper oid=" + itoa(int(oid)) + " step=" + itoa(int(stepPin)) + " dir=" + itoa(int(dirPin)))

	// Create stepper
	_, err = NewStepper(uint8(oid), uint8(stepPin), uint8(dirPin), invertStep != 0, minStopInterval)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: NewStepper failed: " + err.Error())
		return err
	}

	DebugPrintln("[STEPPER] config_stepper complete")
	return nil
}

// cmdQueueStep handles queue_step command
// Format: oid=%c interval=%u count=%hu add=%hi
func cmdQueueStep(data *[]byte) error {
	DebugPrintln("[STEPPER] queue_step called")

	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	interval, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	count, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	add, err := protocol.DecodeVLQInt(data)
	if err != nil {
		return err
	}

	DebugPrintln("[STEPPER] queue_step oid=" + itoa(int(oid)) + " interval=" + itoa(int(interval)) + " count=" + itoa(int(count)))

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		DebugPrintln("[STEPPER] ERROR: stepper not found for oid=" + itoa(int(oid)))
		return errors.New("stepper not found")
	}

	err = stepper.QueueMove(interval, uint16(count), int16(add))
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: QueueMove failed: " + err.Error())
		return err
	}

	DebugPrintln("[STEPPER] queue_step complete")
	return nil
}

func cmdSetNextStepDir(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	dir, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	DebugPrintln("[STEPPER] set_next_step_dir oid=" + itoa(int(oid)) + " dir=" + itoa(int(dir)))

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		DebugPrintln("[STEPPER] ERROR: stepper not found for oid=" + itoa(int(oid)))
		return errors.New("stepper not found")
	}

	stepper.SetNextDir(uint8(dir))
	return nil
}

func cmdResetStepClock(data *[]byte) error {
	DebugPrintln("[STEPPER] reset_step_clock called")

	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: failed to decode oid in reset_step_clock")
		return err
	}

	clockTime, err := protocol.DecodeVLQUint(data)
	if err != nil {
		DebugPrintln("[STEPPER] ERROR: failed to decode clock in reset_step_clock")
		return err
	}

	DebugPrintln("[STEPPER] reset_step_clock oid=" + itoa(int(oid)) + " clock=" + itoa(int(clockTime)))

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		DebugPrintln("[STEPPER] ERROR: stepper not found for oid=" + itoa(int(oid)) + " (stepperCount=" + itoa(int(stepperCount)) + ")")
		return errors.New("stepper not found")
	}

	stepper.ResetClock(clockTime)
	DebugPrintln("[STEPPER] reset_step_clock complete")
	return nil
}

func cmdStepperGetPosition(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	position := stepper.GetPosition()

	// Send stepper_position response
	SendResponse("stepper_position", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, oid)
		protocol.EncodeVLQInt(output, int32(position))
	})

	return nil
}

func cmdStepperGetInfo(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	// Debug info available via stepper struct but not logged to reduce bloat
	_ = stepper

	return nil
}
