//go:build rp2040 || rp2350

// Alternative Multicore Example Using Hardware FIFO
//
// This demonstrates inter-core communication using the RP2040's hardware FIFO,
// which is the same mechanism used by the TinyGo runtime for GC synchronization.
//
// The RP2040 has two 32-bit hardware FIFOs (one per direction) for efficient
// inter-core communication without using shared memory or atomic operations.

package main

import (
	"device/arm"
	"device/rp"
	"machine"
	"time"
)

// Example of using hardware FIFO for inter-core communication
// Note: This is a simplified example. The actual TinyGo runtime uses the FIFO
// for GC synchronization, so in production code you'd want to use higher-level
// primitives like channels or atomic variables instead.

func fifoExample() {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})

	println("FIFO Example: Core 0 sends messages, Core 1 echoes them back")

	// Launch Core 1
	machine.Core1.Start(fifoCore1Main)

	// Wait a bit for Core 1 to start
	time.Sleep(100 * time.Millisecond)

	// Send messages to Core 1
	for i := uint32(100); i < 110; i++ {
		println("Core 0: Sending", i)
		fifoPushBlocking(i)

		// Wait for echo response
		response := fifoPopBlocking()
		println("Core 0: Received echo:", response)

		if response != i {
			println("ERROR: Expected", i, "got", response)
		}

		led.Toggle()
		time.Sleep(500 * time.Millisecond)
	}

	println("FIFO test complete!")
}

func fifoCore1Main() {
	println("Core 1: Started, waiting for FIFO messages...")

	// Echo loop: receive value, add 1, send back
	for {
		// Wait for message from Core 0
		value := fifoPopBlocking()

		println("Core 1: Received", value, ", echoing back")

		// Echo it back
		fifoPushBlocking(value)
	}
}

// FIFO helper functions (simplified from TinyGo runtime)

func fifoValid() bool {
	return rp.SIO.FIFO_ST.Get()&rp.SIO_FIFO_ST_VLD != 0
}

func fifoReady() bool {
	return rp.SIO.FIFO_ST.Get()&rp.SIO_FIFO_ST_RDY != 0
}

func fifoDrain() {
	for fifoValid() {
		rp.SIO.FIFO_RD.Get()
	}
}

func fifoPushBlocking(data uint32) {
	// Wait until FIFO is ready to accept data
	for !fifoReady() {
		// Busy wait or yield
		time.Sleep(1 * time.Microsecond)
	}
	rp.SIO.FIFO_WR.Set(data)
	arm.Asm("sev") // Signal event to wake other core
}

func fifoPopBlocking() uint32 {
	// Wait until FIFO has data
	for !fifoValid() {
		arm.Asm("wfe") // Wait for event
	}
	return rp.SIO.FIFO_RD.Get()
}

/*
// Uncomment to run the FIFO example instead of the main test
func main() {
	fifoExample()
}
*/
