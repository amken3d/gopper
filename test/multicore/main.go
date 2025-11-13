//go:build rp2040 || rp2350

// Multicore Test for TinyGo on RP2040
//
// This program demonstrates TinyGo's dual-core capabilities on the RP2040.
// It showcases:
// - Launching code on Core 1
// - Inter-core communication using shared memory
// - Atomic operations for thread-safe counters
// - Independent LED control from each core
// - Performance benchmarking across cores

package main

import (
	"machine"
	"runtime/volatile"
	"sync/atomic"
	"time"
)

// Shared data between cores (must use atomic operations or volatile)
var (
	core0Counter atomic.Uint32 // Counter incremented by Core 0
	core1Counter atomic.Uint32 // Counter incremented by Core 1

	// Communication channel between cores using volatile registers
	core0ToCore1Message volatile.Register32
	core1ToCore0Message volatile.Register32

	// Task queue for Core 1
	taskRequests atomic.Uint32 // Number of tasks requested
	tasksComplete atomic.Uint32 // Number of tasks completed

	// LED pins
	ledCore0 machine.Pin // LED controlled by Core 0
	ledCore1 machine.Pin // LED controlled by Core 1 (if available)

	// Startup sync
	core1Ready atomic.Bool
)

// Message types for inter-core communication
const (
	MSG_NONE           uint32 = 0
	MSG_PING           uint32 = 1
	MSG_PONG           uint32 = 2
	MSG_COMPUTE_TASK   uint32 = 3
	MSG_TASK_COMPLETE  uint32 = 4
	MSG_GET_COUNTER    uint32 = 5
	MSG_COUNTER_RESULT uint32 = 6
)

func main() {
	// Configure LED for Core 0 (onboard LED)
	ledCore0 = machine.LED
	ledCore0.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Configure second LED for Core 1 if available (GP15)
	ledCore1 = machine.GP15
	ledCore1.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Flash startup sequence
	flashStartup()

	// Initialize shared memory
	core0Counter.Store(0)
	core1Counter.Store(0)
	core0ToCore1Message.Set(MSG_NONE)
	core1ToCore0Message.Set(MSG_NONE)
	taskRequests.Store(0)
	tasksComplete.Store(0)
	core1Ready.Store(false)

	// Launch Core 1
	println("Core 0: Launching Core 1...")
	machine.Core1.Start(core1Main)

	// Wait for Core 1 to be ready
	println("Core 0: Waiting for Core 1 to be ready...")
	for !core1Ready.Load() {
		time.Sleep(10 * time.Millisecond)
	}
	println("Core 0: Core 1 is ready!")

	// Flash both LEDs to indicate dual-core operation started
	for i := 0; i < 5; i++ {
		ledCore0.Set(!ledCore0.Get())
		ledCore1.Set(!ledCore1.Get())
		time.Sleep(100 * time.Millisecond)
	}

	// Test 1: Basic counter increment
	println("\n=== Test 1: Independent Counters ===")
	testIndependentCounters()

	// Test 2: Inter-core messaging
	println("\n=== Test 2: Inter-Core Messaging ===")
	testInterCoreMessaging()

	// Test 3: Task distribution
	println("\n=== Test 3: Task Distribution ===")
	testTaskDistribution()

	// Test 4: Performance benchmark
	println("\n=== Test 4: Performance Benchmark ===")
	testPerformanceBenchmark()

	// Success - flash both LEDs in sync
	println("\n=== All Tests Complete ===")
	println("Core 0 final count:", core0Counter.Load())
	println("Core 1 final count:", core1Counter.Load())

	// Enter continuous operation mode
	println("\n=== Entering Continuous Operation Mode ===")
	continuousOperation()
}

// core1Main runs on Core 1
func core1Main() {
	println("Core 1: Started!")

	// Signal that Core 1 is ready
	core1Ready.Store(true)

	// Core 1 main loop
	for {
		// Increment counter
		core1Counter.Add(1)

		// Check for messages from Core 0
		msg := core0ToCore1Message.Get()
		if msg != MSG_NONE {
			handleCore1Message(msg)
			core0ToCore1Message.Set(MSG_NONE) // Clear message
		}

		// Toggle LED periodically
		if core1Counter.Load()%100000 == 0 {
			ledCore1.Set(!ledCore1.Get())
		}

		// Small delay to prevent busy-wait
		time.Sleep(10 * time.Microsecond)
	}
}

// handleCore1Message processes messages received by Core 1
func handleCore1Message(msg uint32) {
	switch msg {
	case MSG_PING:
		// Respond with PONG
		core1ToCore0Message.Set(MSG_PONG)

	case MSG_COMPUTE_TASK:
		// Simulate computational work
		result := uint32(0)
		for i := 0; i < 1000; i++ {
			result += uint32(i * i)
		}
		tasksComplete.Add(1)
		core1ToCore0Message.Set(MSG_TASK_COMPLETE)

	case MSG_GET_COUNTER:
		// Send counter value back
		core1ToCore0Message.Set(core1Counter.Load())
	}
}

// flashStartup shows a startup LED sequence
func flashStartup() {
	for i := 0; i < 10; i++ {
		ledCore0.High()
		time.Sleep(50 * time.Millisecond)
		ledCore0.Low()
		time.Sleep(50 * time.Millisecond)
	}
}

// testIndependentCounters verifies both cores can increment independently
func testIndependentCounters() {
	startCore0 := core0Counter.Load()
	startCore1 := core1Counter.Load()

	// Let both cores run for 1 second
	println("Running for 1 second...")
	startTime := time.Now()

	for time.Since(startTime) < 1*time.Second {
		core0Counter.Add(1)
		time.Sleep(10 * time.Microsecond)
	}

	endCore0 := core0Counter.Load()
	endCore1 := core1Counter.Load()

	core0Delta := endCore0 - startCore0
	core1Delta := endCore1 - startCore1

	println("Core 0 increments:", core0Delta)
	println("Core 1 increments:", core1Delta)

	if core0Delta > 0 && core1Delta > 0 {
		println("✓ Both cores incrementing independently")
	} else {
		println("✗ ERROR: One or both cores not working")
	}
}

// testInterCoreMessaging tests message passing between cores
func testInterCoreMessaging() {
	successCount := 0
	testCount := 10

	for i := 0; i < testCount; i++ {
		// Send PING to Core 1
		core1ToCore0Message.Set(MSG_NONE) // Clear previous response
		core0ToCore1Message.Set(MSG_PING)

		// Wait for PONG response (with timeout)
		timeout := time.Now().Add(100 * time.Millisecond)
		for time.Now().Before(timeout) {
			if core1ToCore0Message.Get() == MSG_PONG {
				successCount++
				core1ToCore0Message.Set(MSG_NONE) // Clear message
				break
			}
			time.Sleep(1 * time.Millisecond)
		}

		time.Sleep(10 * time.Millisecond)
	}

	println("PING-PONG tests passed:", successCount, "/", testCount)

	if successCount == testCount {
		println("✓ Inter-core messaging working")
	} else {
		println("✗ ERROR: Some messages failed")
	}
}

// testTaskDistribution tests distributing work to Core 1
func testTaskDistribution() {
	numTasks := uint32(20)

	startTime := time.Now()
	taskRequests.Store(0)
	tasksComplete.Store(0)

	// Submit tasks to Core 1
	for i := uint32(0); i < numTasks; i++ {
		taskRequests.Add(1)
		core1ToCore0Message.Set(MSG_NONE)
		core0ToCore1Message.Set(MSG_COMPUTE_TASK)

		// Wait for task completion
		timeout := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(timeout) {
			if core1ToCore0Message.Get() == MSG_TASK_COMPLETE {
				core1ToCore0Message.Set(MSG_NONE)
				break
			}
			time.Sleep(100 * time.Microsecond)
		}
	}

	elapsed := time.Since(startTime)
	completed := tasksComplete.Load()

	println("Tasks requested:", numTasks)
	println("Tasks completed:", completed)
	println("Time elapsed:", elapsed.Milliseconds(), "ms")
	println("Avg time per task:", elapsed.Milliseconds()/int64(completed), "ms")

	if completed == numTasks {
		println("✓ Task distribution working")
	} else {
		println("✗ ERROR: Not all tasks completed")
	}
}

// testPerformanceBenchmark measures performance of both cores
func testPerformanceBenchmark() {
	println("Benchmarking for 2 seconds...")

	// Reset counters
	benchmarkStart0 := core0Counter.Load()
	benchmarkStart1 := core1Counter.Load()

	startTime := time.Now()

	// Core 0 does work
	for time.Since(startTime) < 2*time.Second {
		// Do some computational work
		for i := 0; i < 100; i++ {
			core0Counter.Add(1)
		}
		time.Sleep(100 * time.Microsecond)
	}

	elapsed := time.Since(startTime)

	benchmarkEnd0 := core0Counter.Load()
	benchmarkEnd1 := core1Counter.Load()

	opsCore0 := benchmarkEnd0 - benchmarkStart0
	opsCore1 := benchmarkEnd1 - benchmarkStart1

	println("\nBenchmark Results:")
	println("Duration:", elapsed.Milliseconds(), "ms")
	println("Core 0 operations:", opsCore0)
	println("Core 1 operations:", opsCore1)
	println("Total operations:", opsCore0+opsCore1)
	println("Core 0 ops/sec:", uint64(opsCore0)*1000/uint64(elapsed.Milliseconds()))
	println("Core 1 ops/sec:", uint64(opsCore1)*1000/uint64(elapsed.Milliseconds()))

	if opsCore0 > 0 && opsCore1 > 0 {
		println("✓ Both cores performing work")
	}
}

// continuousOperation runs both cores continuously with LED indicators
func continuousOperation() {
	println("Both cores running. Watch LEDs for activity.")
	println("Core 0 LED:", "onboard LED (GP25)")
	println("Core 1 LED:", "GP15")

	ticker := 0
	for {
		// Core 0 work
		core0Counter.Add(1)

		// Toggle Core 0 LED every 500ms
		if core0Counter.Load()%50000 == 0 {
			ledCore0.Set(!ledCore0.Get())
		}

		// Print status every 5 seconds
		ticker++
		if ticker >= 500000 {
			ticker = 0
			println("\nStatus Update:")
			println("  Core 0 count:", core0Counter.Load())
			println("  Core 1 count:", core1Counter.Load())
			println("  Ratio (C1/C0):", float32(core1Counter.Load())/float32(core0Counter.Load()))
		}

		time.Sleep(10 * time.Microsecond)
	}
}
