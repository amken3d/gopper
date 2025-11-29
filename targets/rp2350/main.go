//go:build rp2350

package main

import (
	"gopper/core"
	"gopper/protocol"
	"gopper/targets/pio"
	"gopper/tinycompress"
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

// ledBlink blinks the LED a specific number of times for diagnostics
func ledBlink(count int) {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for i := 0; i < count; i++ {
		led.High()
		time.Sleep(10 * time.Millisecond)
		led.Low()
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // Pause after blink sequence
}

func main() {
	// Initialize debug UART FIRST for early diagnostics
	// GPIO36=TX, GPIO37=RX at 115200 baud
	InitDebugUART()
	DebugPrintln("[MAIN] Starting main()")

	// Register debug writer with core and tinycompress packages
	core.SetDebugWriter(DebugPrintln)
	tinycompress.SetDebugWriter(DebugPrintln)
	DebugPrintln("[MAIN] Debug writer registered with core and tinycompress packages")

	// Pin main execution to Core 0 for stability
	// This ensures all initialization happens on a single core
	machine.LockCore(0)
	DebugPrintln("[MAIN] Locked to Core 0")

	// Initialize USB CDC immediately
	InitUSB()
	DebugPrintln("[MAIN] USB initialized")

	// DIAGNOSTIC: 3 blinks = USB initialized
	//ledBlink(3)

	// CRITICAL: Disable watchdog on boot to clear any previous state
	// This prevents issues with watchdog persisting across resets
	DebugPrintln("[MAIN] Configuring watchdog...")
	err := machine.Watchdog.Configure(machine.WatchdogConfig{TimeoutMillis: 0})
	if err != nil {
		DebugPrintln("[MAIN] Watchdog config failed")
		return
	}
	DebugPrintln("[MAIN] Watchdog disabled")

	// Initialize clock
	DebugPrintln("[MAIN] Initializing clock...")
	InitClock()
	DebugPrintln("[MAIN] Clock initialized")
	DebugPrintln("[MAIN] Initializing timer...")
	core.TimerInit()
	DebugPrintln("[MAIN] Timer initialized")

	// Initialize core commands
	DebugPrintln("[MAIN] Initializing core commands...")
	core.InitCoreCommands()
	DebugPrintln("[MAIN] Core commands initialized")

	// Step 1: GPIO and ADC (most basic peripherals)
	DebugPrintln("[MAIN] Initializing ADC commands...")
	core.InitADCCommands()
	DebugPrintln("[MAIN] Initializing GPIO commands...")
	core.InitGPIOCommands()
	DebugPrintln("[MAIN] GPIO/ADC initialized")

	// Step 2: PWM and SPI
	DebugPrintln("[MAIN] Initializing PWM commands...")
	core.InitPWMCommands()
	DebugPrintln("[MAIN] Initializing SPI commands...")
	core.InitSPICommands()
	DebugPrintln("[MAIN] PWM/SPI initialized")

	// Step 3: Trigger sync BEFORE endstops
	DebugPrintln("[MAIN] Initializing trigger sync commands...")
	core.InitTriggerSyncCommands()
	DebugPrintln("[MAIN] Trigger sync initialized")

	// Step 4: I2C
	DebugPrintln("[MAIN] Initializing I2C commands...")
	core.InitI2CCommands()
	DebugPrintln("[MAIN] I2C initialized")

	// Step 5: ALL endstop types
	DebugPrintln("[MAIN] Initializing endstop commands...")
	core.InitEndstopCommands()
	DebugPrintln("[MAIN] Initializing analog endstop commands...")
	core.InitAnalogEndstopCommands()
	DebugPrintln("[MAIN] Initializing I2C endstop commands...")
	core.InitI2CEndstopCommands()
	DebugPrintln("[MAIN] Endstops initialized")

	// Step 6: PIO stepper support
	DebugPrintln("[MAIN] Initializing PIO steppers...")
	pio.InitSteppers()
	DebugPrintln("[MAIN] PIO steppers initialized")

	// Step 7: Driver commands (TMC drivers, etc.)
	DebugPrintln("[MAIN] Initializing driver commands...")
	core.InitDriverCommands()
	DebugPrintln("[MAIN] Driver commands initialized")

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
	spiDriver := Rp2350SPIDriver()
	core.SetSPIDriver(spiDriver)
	softwareSPIDriver := NewRP2040SoftwareSPIDriver()
	core.SetSoftwareSPIDriver(softwareSPIDriver)

	// Build and cache dictionary after all commands registered
	// This compresses the dictionary with zlib
	DebugPrintln("[MAIN] Building dictionary...")
	dict := core.GetGlobalDictionary()
	dict.BuildDictionary()
	DebugPrintln("[MAIN] Dictionary build complete!")

	// Log all registered commands for debugging
	core.LogRegisteredCommands()

	DebugPrintln("[MAIN] Blinking LED 5 times...")
	ledBlink(5)
	DebugPrintln("[MAIN] LED blink complete")

	// Create buffers
	DebugPrintln("[MAIN] Creating buffers...")
	inputBuffer = protocol.NewFifoBuffer(256)
	DebugPrintln("[MAIN] Input buffer created")

	DebugPrintln("[MAIN] Creating output buffer...")
	outputBuffer = protocol.NewScratchOutput()
	DebugPrintln("[MAIN] Output buffer created")

	// Create transport with a command handler and reset callback
	DebugPrintln("[MAIN] Creating transport...")
	transport = protocol.NewTransport(outputBuffer, handleCommand)
	DebugPrintln("[MAIN] Transport created")
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

	// DIAGNOSTIC: 4 blinks = Entering main loop (no goroutine needed)
	ledBlink(4)

	// Main loop - handles USB reading, message processing, and timers
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

			// Read incoming USB data into input buffer
			available := USBAvailable()
			if available > 0 {
				data, err := USBRead()
				if err != nil {
					msgerrors++
				} else {
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
					}
				}
			}

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

		// Yield briefly to avoid busy loop
		time.Sleep(10 * time.Microsecond)
	}
}

// COMMENTED OUT: usbReaderLoop caused goroutine creation failure with large firmware
// USB reading is now handled directly in the main loop above
// Keep this code in case we need to revert to goroutine approach later

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
