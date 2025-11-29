//go:build rp2040 || rp2350

package pio

import (
	"gopper/core"
	"machine"

	piolib "github.com/tinygo-org/pio/rp2-pio"
)

// PIO program storage - loaded once per PIO block
var (
	pio0ProgramOffset uint8 = 0xFF // 0xFF = not loaded
	pio1ProgramOffset uint8 = 0xFF
	stepperProgram    []uint16
)

// StepperPIO handles PIO-based stepper pulse generation
// Implements core.StepperBackend interface
type StepperPIO struct {
	pio       *piolib.PIO
	sm        piolib.StateMachine
	offset    uint8
	stepPin   machine.Pin
	dirPin    machine.Pin
	direction bool
	pioNum    uint8
	smNum     uint8
}

// NewStepperPIO creates a new PIO stepper controller
// pioNum: 0 for PIO0, 1 for PIO1
// smNum: 0-3 for state machine number
func NewStepperPIO(pioNum, smNum uint8) *StepperPIO {
	var p *piolib.PIO
	if pioNum == 0 {
		p = piolib.PIO0
	} else {
		p = piolib.PIO1
	}

	return &StepperPIO{
		pio:    p,
		sm:     p.StateMachine(smNum),
		pioNum: pioNum,
		smNum:  smNum,
	}
}

// buildStepperProgram creates the PIO stepper program using AssemblerV0
// Simple version: direction controlled via GPIO, PIO only generates step pulses
// Based on known-working minimal test pattern
func buildStepperProgram() []uint16 {
	asm := piolib.AssemblerV0{SidesetBits: 0}

	return []uint16{
		// Wait for step count from TX FIFO
		asm.Pull(false, true).Encode(),        // 0: pull block
		asm.Out(piolib.OutDestX, 32).Encode(), // 1: X = step count
		// Step loop
		asm.Set(piolib.SetDestPins, 1).Delay(7).Encode(), // 2: step HIGH [7]
		asm.Set(piolib.SetDestPins, 0).Delay(7).Encode(), // 3: step LOW [7]
		asm.Jmp(2, piolib.JmpXNZeroDec).Encode(),         // 4: jmp x--, 2
		// Wraps back to 0 (pull) when X reaches 0
	}
}

// Init initializes the stepper hardware
func (s *StepperPIO) Init(stepPin, dirPin uint8, invertStep, invertDir bool) error {
	core.DebugPrintln("[PIO] Init: stepPin=" + itoa(int(stepPin)) + " dirPin=" + itoa(int(dirPin)))

	s.stepPin = machine.Pin(stepPin)
	s.dirPin = machine.Pin(dirPin)

	// Claim state machine
	core.DebugPrintln("[PIO] Claiming state machine...")
	s.sm.TryClaim()

	// Build program if not already built
	if stepperProgram == nil {
		core.DebugPrintln("[PIO] Building stepper program...")
		stepperProgram = buildStepperProgram()
	}

	// Load program once per PIO block
	var offset uint8
	var err error

	core.DebugPrintln("[PIO] Loading program to PIO" + itoa(int(s.pioNum)) + "...")
	if s.pioNum == 0 {
		if pio0ProgramOffset == 0xFF {
			offset, err = s.pio.AddProgram(stepperProgram, 0)
			if err != nil {
				core.DebugPrintln("[PIO] ERROR: AddProgram failed: " + err.Error())
				return err
			}
			pio0ProgramOffset = offset
		}
		offset = pio0ProgramOffset
	} else {
		if pio1ProgramOffset == 0xFF {
			offset, err = s.pio.AddProgram(stepperProgram, 0)
			if err != nil {
				core.DebugPrintln("[PIO] ERROR: AddProgram failed: " + err.Error())
				return err
			}
			pio1ProgramOffset = offset
		}
		offset = pio1ProgramOffset
	}
	s.offset = offset
	core.DebugPrintln("[PIO] Program loaded at offset " + itoa(int(offset)))

	// Configure state machine
	cfg := piolib.DefaultStateMachineConfig()

	// SET pins = step pin (for pulse generation)
	cfg.SetSetPins(s.stepPin, 1)

	// Shift control: shift right, no autopull, 32-bit threshold
	cfg.SetOutShift(true, false, 32)

	// Wrap points (5 instruction program: 0-4)
	cfg.SetWrap(offset+4, offset)

	// Clock divider - slow for testing (125kHz PIO clock)
	cfg.SetClkDivIntFrac(1000, 0)

	// Configure step pin for PIO control
	core.DebugPrintln("[PIO] Configuring step pin " + itoa(int(stepPin)) + " for PIO mode...")
	s.stepPin.Configure(machine.PinConfig{Mode: s.pio.PinMode()})

	// Configure direction pin as regular GPIO output (not PIO)
	core.DebugPrintln("[PIO] Configuring dir pin " + itoa(int(dirPin)) + " as GPIO output...")
	s.dirPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	s.dirPin.Low()

	// Set step pin direction BEFORE init (critical!)
	s.sm.SetPindirsConsecutive(s.stepPin, 1, true)

	// Initialize and enable state machine
	core.DebugPrintln("[PIO] Initializing state machine...")
	s.sm.Init(offset, cfg)
	s.sm.SetEnabled(true)

	core.DebugPrintln("[PIO] Init complete")
	return nil
}

// Step generates a single step pulse
// Implements core.StepperBackend interface
func (s *StepperPIO) Step() {
	// Queue single step with current direction
	s.QueueSteps(1)
}

// SetDirection sets the direction for the next step(s)
// Implements core.StepperBackend interface
// Direction is controlled via GPIO (not PIO)
func (s *StepperPIO) SetDirection(dir bool) {
	s.direction = dir
	if dir {
		s.dirPin.High()
	} else {
		s.dirPin.Low()
	}
}

// Stop immediately halts the stepper
// Implements core.StepperBackend interface
func (s *StepperPIO) Stop() {
	s.sm.SetEnabled(false)
	s.sm.ClearFIFOs()
	s.sm.Restart()
	s.sm.SetEnabled(true)
}

// GetName returns the backend name
// Implements core.StepperBackend interface
func (s *StepperPIO) GetName() string {
	return "PIO"
}

// GetInfo returns backend performance information
func (s *StepperPIO) GetInfo() core.StepperBackendInfo {
	return core.StepperBackendInfo{
		Name:          s.GetName(),
		MaxStepRate:   500000, // 500 kHz
		MinPulseNs:    64,     // ~64ns @ 125MHz with 8 cycle delay
		TypicalJitter: 10,     // <10ns jitter (hardware-timed)
		CPUOverhead:   1,      // ~1% CPU (only FIFO management)
	}
}

// QueueSteps queues multiple steps to PIO
// This is the efficient way to send steps - queue many at once
// Direction must be set via SetDirection() before calling this
func (s *StepperPIO) QueueSteps(count uint16) {
	// Wait for FIFO space and send count
	for s.sm.IsTxFIFOFull() {
	}
	s.sm.TxPut(uint32(count))
}

// SendSteps queues steps with explicit direction
// Convenience method for batch stepping
func (s *StepperPIO) SendSteps(steps uint16, direction bool) {
	s.SetDirection(direction)
	s.QueueSteps(steps)
}

// IsBusy returns true if the stepper has pending steps
func (s *StepperPIO) IsBusy() bool {
	return !s.sm.IsTxFIFOEmpty()
}

// SetClockDiv sets the clock divider to control step rate
// Higher values = slower steps
// With 125MHz CPU: divider 125 = 1MHz = 1us per PIO cycle
// Each step takes ~18 PIO cycles, so 1MHz / 18 ≈ 55kHz max step rate
func (s *StepperPIO) SetClockDiv(whole uint16, frac uint8) {
	s.sm.SetClkDiv(whole, frac)
}

// itoa converts int to string without importing strconv (for embedded)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
