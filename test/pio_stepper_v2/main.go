//go:build rp2040 || rp2350

package main

// PIO Stepper Speed Test - Cycles through different speeds
// Watch on oscilloscope to see frequency changes

import (
	piostepper "gopper/targets/pio"
	"machine"
	"time"
)

const (
	stepPin = machine.STEP7
	dirPin  = machine.DIR7
)

// Speed test configurations: clock divider and expected approximate frequency
var speedTests = []struct {
	divider uint16
	name    string
}{
	//{10000, "Very Slow (~1.2 kHz)"},
	//{5000, "Slow (~2.4 kHz)"},
	//{2000, "Medium-Slow (~6 kHz)"},
	//{1000, "Medium (~12 kHz)"},
	//{500, "Medium-Fast (~24 kHz)"},
	//{200, "Fast (~60 kHz)"},
	{100, "Very Fast (~120 kHz)"},
	{50, "Ultra Fast (~240 kHz)"},
	{25, "~500 Khz"},
	//{12, "~1Mhz"},
	//{6, "~2Mhz"},
	//{2, "Guess"},
}

func main() {
	time.Sleep(3 * time.Second)

	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Flash LED to indicate start
	for i := 0; i < 3; i++ {
		led.High()
		time.Sleep(100 * time.Millisecond)
		led.Low()
		time.Sleep(100 * time.Millisecond)
	}

	println("=== PIO Stepper Speed Test ===")
	println("Step: GP2, Dir: GP3")
	println("Cycling through speeds - watch oscilloscope!")

	// Create and init stepper
	stepper := piostepper.NewStepperPIO(0, 0)
	err := stepper.Init(uint8(stepPin), uint8(dirPin), false, false)
	if err != nil {
		println("Init error:", err.Error())
		for {
			led.High()
			time.Sleep(100 * time.Millisecond)
			led.Low()
			time.Sleep(100 * time.Millisecond)
		}
	}
	println("Init OK!")

	stepper.SetDirection(false)

	// Main loop - cycle through speeds
	cycle := 0
	for {
		cycle++
		println("\n=== Cycle", cycle, "===")

		for _, test := range speedTests {
			// Stop clears FIFO and restarts SM
			stepper.Stop()

			// Set new speed
			stepper.SetClockDiv(test.divider, 0)
			println("Speed:", test.name, "- Divider:", test.divider)

			led.High()

			// Generate pulses for 3 seconds at this speed
			// Only queue when FIFO has space (non-blocking approach)
			startTime := time.Now()
			for time.Since(startTime) < 3*time.Second {
				// Only queue if not busy (FIFO has space)
				if !stepper.IsBusy() {
					stepper.QueueSteps(100)
				}
				time.Sleep(1 * time.Millisecond)
			}

			led.Low()
			println("  (changing speed...)")
			time.Sleep(500 * time.Millisecond)
		}

		// Toggle direction each cycle
		if cycle%2 == 0 {
			stepper.SetDirection(false)
			println("\nDirection: FORWARD (GP3 LOW)")
		} else {
			stepper.SetDirection(true)
			println("\nDirection: REVERSE (GP3 HIGH)")
		}

		println("\n--- Restarting cycle ---")
		time.Sleep(1 * time.Second)
	}
}
