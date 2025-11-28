//go:build rp2350

package main

import (
	"gopper/core"
	"gopper/protocol"
	// piostepper "gopper/targets/pio" // DISABLED: Crashes on RP2350 during InitSteppers()
	"machine"
	"time"
)

var (
	// Buffers for communication
	inputBuffer  *protocol.FifoBuffer
	outputBuffer *protocol.ScratchOutput
	transport    *protocol.Transport

	// Debug counters
	messagesReceived         uint32
	messagesSent             uint32
	msgerrors                uint32
	usbWasDisconnected       bool
	consecutiveWriteFailures uint32
)

func main() {
	// Initialize USB CDC immediately
	InitUSB()

	// CRITICAL: Disable watchdog on boot to clear any previous state
	// This prevents issues with watchdog persisting across resets
	err := machine.Watchdog.Configure(machine.WatchdogConfig{TimeoutMillis: 0})
	if err != nil {
		return
	}

	// Initialize clock
	InitClock()
	core.TimerInit()

	// Initialize core commands
	core.InitCoreCommands()

	// Step 1: GPIO and ADC (most basic peripherals) - WORKING ✓
	core.InitADCCommands()
	core.InitGPIOCommands()

	// Step 2: PWM, SPI, and I2C - WORKING ✓
	core.InitPWMCommands()
	core.InitSPICommands()
	core.InitI2CCommands()

	// Step 3: Disable endstops for now - compression crashes with them
	// The crash happens at dictionary offset 240 during transmission
	// This suggests the compressed dictionary is corrupted or too large
	// TODO: Investigate tinycompress library on RP2350
	// core.InitEndstopCommands()
	// core.InitAnalogEndstopCommands()
	// core.InitI2CEndstopCommands()
	// core.InitTriggerSyncCommands()

	// Step 4: Disable ALL stepper-related code for RP2350
	// piostepper.InitSteppers()
	// core.RegisterStepperCommands()
	// core.InitDriverCommands()

	// Register combined pin enumeration for RP2350
	// This must happen before BuildDictionary()
	// Indices 0-47: GPIO pins (gpio0-gpio47)
	// Indices 48-52: ADC channels (ADC0-ADC3, ADC_TEMPERATURE)
	registerRP2350Pins()

	// Step 1: GPIO and ADC drivers - WORKING ✓
	adcDriver := NewRPAdcDriver()
	core.SetADCDriver(adcDriver)
	gpioDriver := NewRPGPIODriver()
	core.SetGPIODriver(gpioDriver)

	// Step 2: Add PWM and SPI drivers
	pwmDriver := NewRP2040PWMDriver()
	core.SetPWMDriver(pwmDriver)
	spiDriver := NewRP2040SPIDriver()
	core.SetSPIDriver(spiDriver)
	softwareSPIDriver := NewRP2040SoftwareSPIDriver()
	core.SetSoftwareSPIDriver(softwareSPIDriver)

	// Build and cache dictionary after all commands registered
	// This compresses the dictionary with zlib
	core.GetGlobalDictionary().BuildDictionary()
	// Create buffers
	inputBuffer = protocol.NewFifoBuffer(256)
	outputBuffer = protocol.NewScratchOutput()

	// Create transport with a command handler and reset callback
	transport = protocol.NewTransport(outputBuffer, handleCommand)
	transport.SetResetCallback(func() {
		// Clear buffers on host reset
		inputBuffer.Reset()
		outputBuffer.Reset()

		core.ResetFirmwareState() // Clear the shutdown flag and config state
	})
	// Set flush callback to immediately send ACKs to USB
	// This is critical - serialqueue expects ACK before response
	transport.SetFlushCallback(func() {
		writeUSB()
	})
	core.SetGlobalTransport(transport)

	// Set reset handler to trigger watchdog reset (recommended for RP2040/RP2350)
	// This is used by Klipper's FIRMWARE_RESTART command
	core.SetResetHandler(func() {
		// Use watchdog reset instead of ARM SYSRESETREQ
		// This is more reliable on RP2040/RP2350 and handles USB re-enumeration better
		err = machine.Watchdog.Configure(machine.WatchdogConfig{TimeoutMillis: 1})
		if err != nil {
			return
		}
		err = machine.Watchdog.Start()
		if err != nil {
			return
		}
		// Wait for reset (should happen in ~1ms)
		for {
			time.Sleep(1 * time.Millisecond)
		}
	})
	// Start USB reader goroutine
	go usbReaderLoop()

	// Main loop - start immediately
	for {
		// Recover from panics in the main loop to prevent a firmware crash
		func() {
			defer func() {
				if r := recover(); r != nil {
					msgerrors++
					// Clear buffers and continue
					inputBuffer.Reset()
					outputBuffer.Reset()
				}
			}()

			// Update system time from hardware
			UpdateSystemTime()

			// Process incoming messages
			if inputBuffer.Available() > 0 {
				// Create InputBuffer from FIFO data
				data := inputBuffer.Data()
				originalLen := len(data)
				inputBuf := protocol.NewSliceInputBuffer(data)

				// Process messages
				transport.Receive(inputBuf)
				messagesReceived++

				// Remove consumed bytes from FIFO
				consumed := originalLen - inputBuf.Available()
				if consumed > 0 {
					inputBuffer.Pop(consumed)
				}

			}

			// Write outgoing USB data
			result := outputBuffer.Result()
			if len(result) > 0 {
				writeUSB()
				messagesSent++
			}

			// Check for pending reset after all messages sent
			// This ensures the ACK has been transmitted before reset
			core.CheckPendingReset()

			// Process scheduled timers
			core.ProcessTimers()

			// Run an analog-in task to send any pending analog_in_state reports.
			core.AnalogInTask()
		}()

		// Yield to other goroutines
		time.Sleep(10 * time.Microsecond)
	}
}

// usbReaderLoop runs in a goroutine to continuously read USB data
func usbReaderLoop() {
	// Recover from panics to prevent a firmware crash
	defer func() {
		if r := recover(); r != nil {
			msgerrors++
			// Restart the reader loop
			time.Sleep(100 * time.Millisecond)
			go usbReaderLoop()
		}
	}()

	for {
		available := USBAvailable()
		if available > 0 {
			data, err := USBRead()
			if err != nil {
				msgerrors++
				time.Sleep(1 * time.Millisecond)
				continue
			}

			// If we were disconnected and now receiving data, reset the state for reconnection
			if usbWasDisconnected {
				usbWasDisconnected = false
				// Reset all state for fresh connection
				inputBuffer.Reset()
				outputBuffer.Reset()
				transport.Reset()
				core.ResetFirmwareState() // Clear the shutdown flag and config state
				messagesReceived = 0
				messagesSent = 0
				consecutiveWriteFailures = 0
			}

			written := inputBuffer.Write([]byte{data})
			if written == 0 {
				// Buffer full - error condition
				msgerrors++
				time.Sleep(10 * time.Millisecond)
			}
		}
		// Yield to avoid a busy loop
		time.Sleep(100 * time.Microsecond)
	}
}

// handleCommand dispatches received commands to the command registry
func handleCommand(cmdID uint16, data *[]byte) error {
	return core.DispatchCommand(cmdID, data)
}

// registerRP2350Pins registers all pin names for the RP2350
// Combines GPIO pins (0-47) and ADC channels (48-52) into a single enumeration
func registerRP2350Pins() {
	// Total: 48 GPIO pins + 5 ADC channels = 53 total pins
	pinNames := make([]string, 53)

	// Indices 0-47: GPIO pins (gpio0-gpio47)
	for i := 0; i < 48; i++ {
		pinNames[i] = "gpio" + itoa(i)
	}

	// Indices 48-52: ADC channels
	pinNames[48] = "ADC0"
	pinNames[49] = "ADC1"
	pinNames[50] = "ADC2"
	pinNames[51] = "ADC3"
	pinNames[52] = "ADC_TEMPERATURE"

	// Register the combined enumeration
	core.RegisterEnumeration("pin", pinNames)
}

// itoa converts int to string without importing strconv (for embedded)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	// Handle negative numbers
	negative := i < 0
	if negative {
		i = -i
	}

	// Convert to string
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

// writeUSB writes available data from output buffer to USB
func writeUSB() {
	result := outputBuffer.Result()
	if len(result) > 0 {
		// Write all data, handling partial writes
		written := 0
		for written < len(result) {
			n, err := USBWriteBytes(result[written:])
			if err != nil {
				// Write error - likely disconnect
				consecutiveWriteFailures++
				// After several failures, mark as disconnected and clear stale data
				if consecutiveWriteFailures > 10 {
					usbWasDisconnected = true
					consecutiveWriteFailures = 0
					// Clear output buffer - don't keep trying to send stale data
					outputBuffer.Reset()
					// Also clear input buffer for a clean state
					inputBuffer.Reset()
				}
				return
			}
			if n == 0 {
				// No progress - likely disconnect
				consecutiveWriteFailures++
				if consecutiveWriteFailures > 10 {
					usbWasDisconnected = true
					consecutiveWriteFailures = 0
					outputBuffer.Reset()
					inputBuffer.Reset()
				}
				return
			}
			written += n
		}
		// Successfully wrote everything
		if written == len(result) {
			consecutiveWriteFailures = 0 // Reset failure counter on success
			outputBuffer.Reset()
		}
	}
}
