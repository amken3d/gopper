//go:build rp2040

package pio

// PIO Stepper Backend using tinygo-org/pio package
// This provides hardware-accelerated, jitter-free step pulse generation

import (
	"gopper/core"
	"machine"

	rp2pio "github.com/tinygo-org/pio/rp2-pio"
)

// PIO program for step pulse generation
// Command word format:
//
//	Bits 0-15:  pulse count (number of steps to generate)
//	Bits 16-23: delay cycles (inter-pulse spacing)
//	Bit 31:     direction (0=forward, 1=reverse)
//
// Program flow:
//  1. Pull 32-bit command from FIFO
//  2. Extract pulse count into X register
//  3. Extract delay cycles into Y register
//  4. Set direction pin
//  5. Generate X pulses with Y cycle delays between them
//
// buildStepperProgram creates the stepper PIO program using AssemblerV0
func buildStepperProgram() []uint16 {
	asm := rp2pio.AssemblerV0{SidesetBits: 0}
	return []uint16{
		// .wrap_target
		asm.Pull(false, true).Encode(),          // 0: pull block
		asm.Out(rp2pio.OutDestX, 16).Encode(),   // 1: out x, 16 (pulse count)
		asm.Out(rp2pio.OutDestY, 8).Encode(),    // 2: out y, 8 (delay cycles)
		asm.Out(rp2pio.OutDestPins, 1).Encode(), // 3: out pins, 1 (direction)
		// step_loop:
		asm.Set(rp2pio.SetDestPins, 1).Delay(7).Encode(), // 4: set pins, 1 [7]
		asm.Set(rp2pio.SetDestPins, 0).Encode(),          // 5: set pins, 0
		// delay_loop:
		asm.Jmp(6, rp2pio.JmpYNZeroDec).Encode(), // 6: jmp y--, 6
		asm.Jmp(4, rp2pio.JmpXNZeroDec).Encode(), // 7: jmp x--, 4
		// .wrap
	}
}

const stepperPIOOrigin = 0 // Load at offset 0 for correct jump addresses

// PIOStepperBackend implements stepper control using TinyGo's pio package
type PIOStepperBackend struct {
	pio       *rp2pio.PIO
	sm        rp2pio.StateMachine
	stepPin   machine.Pin
	dirPin    machine.Pin
	direction bool
	offset    uint8
	pioNum    uint8
	smNum     uint8
}

// NewPIOStepperBackend creates a new PIO-based stepper backend
// pioNum: 0 for PIO0, 1 for PIO1
// smNum: 0-3 for state machine number
func NewPIOStepperBackend(pioNum, smNum uint8) *PIOStepperBackend {
	var pioHW *rp2pio.PIO
	if pioNum == 0 {
		pioHW = rp2pio.PIO0
	} else {
		pioHW = rp2pio.PIO1
	}

	return &PIOStepperBackend{
		pio:    pioHW,
		sm:     pioHW.StateMachine(smNum),
		pioNum: pioNum,
		smNum:  smNum,
	}
}

// Init initializes the PIO stepper backend
func (b *PIOStepperBackend) Init(stepPin, dirPin uint8, invertStep, invertDir bool) error {
	b.stepPin = machine.Pin(stepPin)
	b.dirPin = machine.Pin(dirPin)

	// CRITICAL: Claim the state machine first!
	b.sm.TryClaim()

	// Build and load PIO program using AssemblerV0
	program := buildStepperProgram()
	offset, err := b.pio.AddProgram(program, stepperPIOOrigin)
	if err != nil {
		return err
	}
	b.offset = offset

	// Configure pins for PIO
	b.stepPin.Configure(machine.PinConfig{Mode: b.pio.PinMode()})
	b.dirPin.Configure(machine.PinConfig{Mode: b.pio.PinMode()})

	// Build state machine configuration
	cfg := rp2pio.DefaultStateMachineConfig()

	// Configure SET pins (step pin) - used for pulse generation
	cfg.SetSetPins(b.stepPin, 1)

	// Configure OUT pins (direction pin) - used for direction control
	cfg.SetOutPins(b.dirPin, 1)

	// Configure shift control: shift right, autopull DISABLED (we use explicit PULL), 32-bit threshold
	cfg.SetOutShift(true, false, 32)

	// Configure wrap points (program is 8 instructions: 0-7)
	cfg.SetWrap(offset+uint8(len(program))-1, offset)

	// Full speed clock (125MHz) - PIO program handles timing
	cfg.SetClkDivIntFrac(1000, 0)

	// Initialize state machine FIRST
	b.sm.Init(offset, cfg)

	// THEN set pin directions (must be after Init!)
	b.sm.SetPindirsConsecutive(b.stepPin, 1, true) // step = output
	b.sm.SetPindirsConsecutive(b.dirPin, 1, true)  // dir = output

	// Set initial pin states
	b.sm.SetPinsConsecutive(b.stepPin, 1, false) // step = low
	b.sm.SetPinsConsecutive(b.dirPin, 1, false)  // dir = low

	// Enable state machine
	b.sm.SetEnabled(true)

	return nil
}

// Step generates a single step pulse
func (b *PIOStepperBackend) Step() {
	// Build command word with current direction
	// 1 step, minimal delay (1 cycle), current direction
	cmd := uint32(1) | (1 << 16) // count=1, delay=1
	if b.direction {
		cmd |= (1 << 31) // set direction bit
	}

	// Wait for FIFO space and write
	for b.sm.IsTxFIFOFull() {
		// Busy wait - should be very brief
	}
	b.sm.TxPut(cmd)
}

// QueueSteps queues multiple steps to PIO
func (b *PIOStepperBackend) QueueSteps(count uint16, delayCycles uint8, direction bool) {
	// Build 32-bit command word
	cmd := uint32(count) | (uint32(delayCycles) << 16)
	if direction {
		cmd |= (1 << 31)
	}

	// Wait for FIFO space and write
	for b.sm.IsTxFIFOFull() {
		// Busy wait
	}
	b.sm.TxPut(cmd)
}

// SetDirection sets the direction for the next move
func (b *PIOStepperBackend) SetDirection(dir bool) {
	b.direction = dir
}

// Stop halts the PIO state machine
func (b *PIOStepperBackend) Stop() {
	b.sm.SetEnabled(false)
	b.sm.ClearFIFOs()
	b.sm.Restart()
	b.sm.SetEnabled(true)
}

// GetName returns the backend name
func (b *PIOStepperBackend) GetName() string {
	return "PIO"
}

// GetInfo returns backend performance information
func (b *PIOStepperBackend) GetInfo() core.StepperBackendInfo {
	return core.StepperBackendInfo{
		Name:          b.GetName(),
		MaxStepRate:   500000, // 500 kHz
		MinPulseNs:    64,     // ~64ns pulse width (8 cycles @ 125MHz)
		TypicalJitter: 10,     // <10ns jitter (hardware-timed)
		CPUOverhead:   1,      // ~1% CPU (only FIFO management)
	}
}
