//go:build rp2040 || rp2350

package main

import (
	"gopper/core"
	"gopper/protocol"
	"machine"
	"time"
)

var (
	// Buffers for communication
	inputBuffer  *protocol.FifoBuffer
	outputBuffer *protocol.ScratchOutput
	transport    *protocol.Transport

	// LED for debugging
	debugLED machine.Pin

	// Debug counters
	messagesReceived uint32
	messagesSent     uint32
	errors           uint32

	// USB connection state tracking
	lastUSBActivity          uint64 // Last time we successfully read/wrote USB data
	lastWriteSuccess         uint64 // Last time we successfully wrote USB data
	usbWasDisconnected       bool
	consecutiveWriteFailures uint32
)

func main() {
	// CRITICAL: Disable watchdog on boot to clear any previous state
	// This prevents issues with watchdog persisting across resets
	machine.Watchdog.Configure(machine.WatchdogConfig{TimeoutMillis: 0})

	// Setup LED for status indication
	debugLED = machine.LED
	debugLED.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Initialize USB CDC immediately
	InitUSB()

	// Initialize clock
	InitClock()
	core.TimerInit()

	// Initialize core commands
	core.InitCoreCommands()

	// Build and cache dictionary after all commands registered
	// This compresses the dictionary with zlib
	core.GetGlobalDictionary().BuildDictionary()

	// Flash LED to show we're creating buffers
	FlashLED(2, 100)

	// Create buffers
	inputBuffer = protocol.NewFifoBuffer(256)
	outputBuffer = protocol.NewScratchOutput()

	// Create transport with command handler and reset callback
	transport = protocol.NewTransport(outputBuffer, handleCommand)
	transport.SetResetCallback(func() {
		// Clear buffers on host reset
		inputBuffer.Reset()
		outputBuffer.Reset()

		core.ResetFirmwareState() // Clear shutdown flag and config state
		// Flash LED rapidly on reset
		FlashLED(3, 50)
	})
	// Set flush callback to immediately send ACKs to USB
	// This is critical - serialqueue expects ACK before response
	transport.SetFlushCallback(func() {
		writeUSB()
	})
	core.SetGlobalTransport(transport)

	// Set reset handler to trigger watchdog reset (recommended for RP2040)
	// This is used by Klipper's FIRMWARE_RESTART command
	core.SetResetHandler(func() {
		// Use watchdog reset instead of ARM SYSRESETREQ
		// This is more reliable on RP2040 and handles USB re-enumeration better
		machine.Watchdog.Configure(machine.WatchdogConfig{TimeoutMillis: 1})
		machine.Watchdog.Start()
		// Wait for reset (should happen in ~1ms)
		for {
			time.Sleep(1 * time.Millisecond)
		}
	})

	// Flash LED to show we're starting
	FlashLED(5, 100)
	debugLED.High()

	// Start USB reader goroutine
	go usbReaderLoop()

	// Start LED heartbeat goroutine
	go ledHeartbeat()

	// Main loop - start immediately
	for {
		// Recover from panics in main loop to prevent firmware crash
		func() {
			defer func() {
				if r := recover(); r != nil {
					errors++
					FlashLED(10, 50)
					// Clear buffers and continue
					inputBuffer.Reset()
					outputBuffer.Reset()
				}
			}()

			// Update system time from hardware
			UpdateSystemTime()

			// Process incoming messages
			if inputBuffer.Available() > 0 {
				// Flash LED to show activity
				debugLED.Low()

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

				debugLED.High()
			}

			// Write outgoing USB data
			result := outputBuffer.Result()
			if len(result) > 0 {
				// Flash LED to show sending
				debugLED.Low()

				writeUSB()
				messagesSent++

				debugLED.High()
			}

			// Check for pending reset after all messages sent
			// This ensures the ACK has been transmitted before reset
			core.CheckPendingReset()

			// Process scheduled timers
			core.ProcessTimers()
		}()

		// Yield to other goroutines
		time.Sleep(10 * time.Microsecond)
	}
}

// usbReaderLoop runs in a goroutine to continuously read USB data
func usbReaderLoop() {
	// Recover from panics to prevent firmware crash
	defer func() {
		if r := recover(); r != nil {
			errors++
			FlashLED(10, 50)
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
				errors++
				time.Sleep(1 * time.Millisecond)
				continue
			}

			// If we were disconnected and now receiving data, reset state for reconnection
			if usbWasDisconnected {
				usbWasDisconnected = false
				// Reset all state for fresh connection
				inputBuffer.Reset()
				outputBuffer.Reset()
				transport.Reset()
				core.ResetFirmwareState() // Clear shutdown flag and config state
				messagesReceived = 0
				messagesSent = 0
				consecutiveWriteFailures = 0
				// Flash to indicate reconnection
				FlashLED(4, 100)
			}

			// Update activity timestamp
			lastUSBActivity = core.GetUptime()

			written := inputBuffer.Write([]byte{data})
			if written == 0 {
				// Buffer full - error condition
				errors++
				FlashLED(5, 50)
				time.Sleep(10 * time.Millisecond)
			}
		}
		// Yield to avoid busy loop
		time.Sleep(100 * time.Microsecond)
	}
}

// ledHeartbeat shows the firmware is alive with slow blinks
func ledHeartbeat() {
	ticker := 0
	for {
		time.Sleep(1 * time.Second)
		ticker++
		// Every 5 seconds, do a quick double-blink
		if ticker%5 == 0 {
			debugLED.Low()
			time.Sleep(50 * time.Millisecond)
			debugLED.High()
			time.Sleep(50 * time.Millisecond)
			debugLED.Low()
			time.Sleep(50 * time.Millisecond)
			debugLED.High()
		}
	}
}

// FlashLED flashes the LED count times with the given delay
func FlashLED(count int, delayMs int) {
	state := debugLED.Get()
	for i := 0; i < count; i++ {
		debugLED.Low()
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		debugLED.High()
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
	debugLED.Set(state)
}

// handleCommand dispatches received commands to the command registry
func handleCommand(cmdID uint16, data *[]byte) error {
	return core.DispatchCommand(cmdID, data)
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
					// Also clear input buffer for clean state
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
			lastWriteSuccess = core.GetUptime()
			outputBuffer.Reset()
		}
	}
}
