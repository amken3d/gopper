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
		"oid=%c step_pin=%c dir_pin=%c invert_step=%c min_stop_interval=%u",
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
}

// cmdConfigStepper handles config_stepper command
// Format: oid=%c step_pin=%c dir_pin=%c invert_step=%c min_stop_interval=%u
//func cmdConfigStepper(args []interface{}) error {
//	if len(args) < 5 {
//		return fmt.Errorf("config_stepper: insufficient arguments")
//	}
//
//	oid := args[0].(uint8)
//	stepPin := args[1].(uint8)
//	dirPin := args[2].(uint8)
//	invertStep := args[3].(uint8) != 0
//	minStopInterval := args[4].(uint32)
//
//	// Create stepper
//	stepper, err := NewStepper(oid, stepPin, dirPin, invertStep, minStopInterval)
//	if err != nil {
//		return fmt.Errorf("config_stepper: %v", err)
//	}
//
//	// Initialize backend (will be set by platform-specific code)
//	if stepper.Backend == nil {
//		// Backend will be initialized by target-specific code
//		// For now, just create the stepper object
//		debugLog(fmt.Sprintf("Stepper %d created: step=%d dir=%d invert=%v min_interval=%d",
//			oid, stepPin, dirPin, invertStep, minStopInterval))
//	}
//
//	return nil
//}

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
//
//	func cmdQueueStep(args []interface{}) error {
//		if len(args) < 4 {
//			return fmt.Errorf("queue_step: insufficient arguments")
//		}
//
//		oid := args[0].(uint8)
//		interval := args[1].(uint32)
//		count := args[2].(uint16)
//		add := args[3].(int16)
//
//		stepper := GetStepper(oid)
//		if stepper == nil {
//			return fmt.Errorf("queue_step: stepper %d not found", oid)
//		}
//
//		return stepper.QueueMove(interval, count, add)
//	}
//
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

// cmdSetNextStepDir handles set_next_step_dir command
// Format: oid=%c dir=%c
//
//	func cmdSetNextStepDir(args []interface{}) error {
//		if len(args) < 2 {
//			return fmt.Errorf("set_next_step_dir: insufficient arguments")
//		}
//
//		oid := args[0].(uint8)
//		dir := args[1].(uint8)
//
//		stepper := GetStepper(oid)
//		if stepper == nil {
//			return fmt.Errorf("set_next_step_dir: stepper %d not found", oid)
//		}
//
//		stepper.SetNextDir(dir)
//		return nil
//	}
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

// cmdResetStepClock handles reset_step_clock command
// Format: oid=%c clock=%u
//
//	func cmdResetStepClock(args []interface{}) error {
//		if len(args) < 2 {
//			return errors.New("reset_step_clock: insufficient arguments")
//		}
//
//		oid := args[0].(uint8)
//		clockTime := args[1].(uint32)
//
//		stepper := GetStepper(oid)
//		if stepper == nil {
//			return errors.New("reset_step_clock: stepper not found")
//		}
//
//		stepper.ResetClock(clockTime)
//		return nil
//	}
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

// cmdStepperGetPosition handles stepper_get_position command
// Format: oid=%c
// Response: stepper_position oid=%c pos=%i
//
//	func cmdStepperGetPosition(args []interface{}) error {
//		if len(args) < 1 {
//			return errors.New("stepper_get_position: insufficient arguments")
//		}
//
//		oid := args[0].(uint8)
//
//		stepper := GetStepper(oid)
//		if stepper == nil {
//			return errors.New("stepper_get_position: stepper not found")
//		}
//
//		position := stepper.GetPosition()
//
//		// Send response (will be implemented in protocol layer)
//		// For now, just log it
//		debugLog("stepper_get_position")
//
//		// TODO: Send stepper_position response via protocol
//		// SendResponse("stepper_position", oid, position)
//
//		return nil
//	}
func cmdStepperGetPosition(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	stepper := GetStepper(uint8(oid))
	if stepper == nil {
		return errors.New("stepper not found")
	}

	_ = stepper.GetPosition()

	// TODO: Send stepper_position response via protocol
	// SendResponse("stepper_position", oid, position)

	return nil
}

// cmdStepperGetInfo handles stepper_get_info debug command
// Format: oid=%c
//
//	func cmdStepperGetInfo(args []interface{}) error {
//		if len(args) < 1 {
//			return errors.New("stepper_get_info: insufficient arguments")
//		}
//
//		oid := args[0].(uint8)
//
//		stepper := GetStepper(oid)
//		if stepper == nil {
//			return errors.New("stepper_get_info: stepper not found")
//		}
//
//		debugLog("stepper_get_info")
//
//		return nil
//	}
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
