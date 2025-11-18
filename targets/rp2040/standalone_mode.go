//go:build rp2040 || rp2350

package main

import (
	"gopper/core"
	"gopper/standalone"
	"gopper/standalone/config"
	"machine"
	"time"
)

// RunStandaloneMode runs the MCU in standalone mode (no Klipper host required)
func RunStandaloneMode() {
	// Get default configuration
	cfg := config.DefaultCartesianConfig()

	// Create manager
	manager, err := standalone.NewManagerWithConfig(cfg)
	if err != nil {
		// Flash LED rapidly to indicate error
		led := machine.LED
		led.Configure(machine.PinConfig{Mode: machine.PinOutput})
		for {
			led.High()
			time.Sleep(100 * time.Millisecond)
			led.Low()
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Get GPIO driver (already initialized in main)
	gpioDriver := core.GetGPIODriver()
	if gpioDriver == nil {
		// Error - GPIO not initialized
		return
	}

	// Initialize manager
	err = manager.Initialize(gpioDriver)
	if err != nil {
		// Flash LED rapidly to indicate error
		led := machine.LED
		led.Configure(machine.PinConfig{Mode: machine.PinOutput})
		for {
			led.High()
			time.Sleep(100 * time.Millisecond)
			led.Low()
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Start standalone mode
	err = manager.Start()
	if err != nil {
		return
	}

	// Flash LED 3 times to indicate standalone mode started
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for i := 0; i < 3; i++ {
		led.High()
		time.Sleep(200 * time.Millisecond)
		led.Low()
		time.Sleep(200 * time.Millisecond)
	}

	// Main loop for standalone mode
	for {
		// Process USB input
		available := USBAvailable()
		if available > 0 {
			data, err := USBRead()
			if err == nil {
				// Process byte
				err = manager.ProcessByte(data)
				if err != nil {
					// Send error response
					manager.SendResponse("Error: ")
					manager.SendResponse(err.Error())
					manager.SendResponse("\n")
				}
			}
		}

		// Send any pending output
		output := manager.GetOutput()
		if len(output) > 0 {
			USBWriteBytes(output)
		}

		// Update system time
		UpdateSystemTime()

		// Process scheduled timers
		core.ProcessTimers()

		// Yield
		time.Sleep(10 * time.Microsecond)
	}
}
