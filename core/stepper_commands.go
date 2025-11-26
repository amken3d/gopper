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

	// move_to_position: Position-based move command (for hardware ramp drivers like TMC5240)
	RegisterCommand("move_to_position",
		"oid=%c target_pos=%i start_vel=%u end_vel=%u accel=%u",
		cmdMoveToPosition)

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
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	stepPin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	dirPin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	invertStep, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	minStopInterval, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Create stepper
	_, err = NewStepper(uint8(oid), uint8(stepPin), uint8(dirPin), invertStep != 0, minStopInterval)
	if err != nil {
		return err
	}

	return nil
}

// cmdQueueStep handles queue_step command
// Format: oid=%c interval=%u count=%hu add=%hi
func cmdQueueStep(data *[]byte) error {
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

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	return stepper.QueueMove(interval, uint16(count), int16(add))
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

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	stepper.SetNextDir(uint8(dir))
	return nil
}

func cmdResetStepClock(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	clockTime, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	stepper.ResetClock(clockTime)
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

// cmdMoveToPosition handles move_to_position command
// Format: oid=%c target_pos=%i start_vel=%u end_vel=%u accel=%u
// This command enables hardware ramp generation for drivers like TMC5240
func cmdMoveToPosition(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	targetPos, err := protocol.DecodeVLQInt(data)
	if err != nil {
		return err
	}

	startVel, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	endVel, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	accel, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	return stepper.MoveToPosition(int64(targetPos), startVel, endVel, accel)
}
