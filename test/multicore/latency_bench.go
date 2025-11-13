//go:build rp2040 || rp2350

// Inter-Core Latency Benchmarking
//
// This module provides precise latency measurements for inter-core communication
// on RP2040/RP2350. Critical for designing real-time systems like Gopper.

package main

import (
	"machine"
	"runtime/volatile"
	"sync/atomic"
	"time"
)

// Latency measurement infrastructure
var (
	// Timing data
	latencyStart     atomic.Uint64  // Timestamp when message was sent
	latencyEnd       atomic.Uint64  // Timestamp when response received
	latencyTrigger   atomic.Uint32  // Signal to start latency test
	latencyResponse  atomic.Uint32  // Response from Core 1
	latencyComplete  atomic.Bool    // Test complete flag
	latencyIteration atomic.Uint32  // Current iteration number

	// Results storage (for statistical analysis)
	latencySamples     [1000]uint32 // Store up to 1000 samples (in microseconds)
	latencySampleCount atomic.Uint32

	// Volatile register latency test
	volatileLatencyTrigger volatile.Register32
	volatileLatencyResponse volatile.Register32
)

// Get current time in microseconds
func micros() uint64 {
	return uint64(time.Now().UnixMicro())
}

// latencyBenchmarkCore1 runs on Core 1 and responds to latency test requests
func latencyBenchmarkCore1() {
	println("Core 1: Latency benchmark handler started")

	for {
		// Check for atomic latency test trigger
		if latencyTrigger.Load() != 0 {
			// Respond immediately
			latencyResponse.Store(latencyTrigger.Load())
			latencyTrigger.Store(0)
		}

		// Check for volatile latency test trigger
		if volatileLatencyTrigger.Get() != 0 {
			// Respond immediately
			volatileLatencyResponse.Set(volatileLatencyTrigger.Get())
			volatileLatencyTrigger.Set(0)
		}

		// Minimal delay to avoid busy-wait
		time.Sleep(1 * time.Microsecond)
	}
}

// runLatencyBenchmarks executes all latency tests
func runLatencyBenchmarks() {
	println("\n" + "="*60)
	println("=== Inter-Core Latency Benchmarks ===")
	println("="*60)
	println("\nThese measurements are critical for real-time system design.")
	println("Lower latency = faster response to time-critical events")
	println("Lower jitter = more predictable real-time behavior\n")

	// Launch Core 1 latency handler
	machine.Core1.Start(latencyBenchmarkCore1)
	time.Sleep(100 * time.Millisecond) // Wait for Core 1 to start

	// Test 1: Round-trip latency with atomic variables
	println("\n--- Test 1: Atomic Variable Round-Trip Latency ---")
	testAtomicRoundTripLatency(100)

	time.Sleep(500 * time.Millisecond)

	// Test 2: Round-trip latency with volatile registers
	println("\n--- Test 2: Volatile Register Round-Trip Latency ---")
	testVolatileRoundTripLatency(100)

	time.Sleep(500 * time.Millisecond)

	// Test 3: Latency under load
	println("\n--- Test 3: Latency Under Load ---")
	testLatencyUnderLoad(50)

	time.Sleep(500 * time.Millisecond)

	// Test 4: Jitter analysis
	println("\n--- Test 4: Jitter Analysis (Critical for Real-Time) ---")
	testJitter(200)

	println("\n" + "="*60)
	println("=== Latency Benchmark Complete ===")
	println("="*60)
	printRecommendations()
}

// testAtomicRoundTripLatency measures round-trip time using atomic variables
func testAtomicRoundTripLatency(iterations int) {
	println("Measuring round-trip latency using atomic.Uint32...")
	println("Iterations:", iterations)

	var samples []uint32
	var minLatency uint32 = 0xFFFFFFFF
	var maxLatency uint32 = 0
	var totalLatency uint64 = 0

	for i := 0; i < iterations; i++ {
		// Clear previous response
		latencyResponse.Store(0)

		// Start timing
		startTime := micros()

		// Send trigger
		testValue := uint32(i + 1)
		latencyTrigger.Store(testValue)

		// Wait for response
		timeout := time.Now().Add(10 * time.Millisecond)
		for latencyResponse.Load() != testValue {
			if time.Now().After(timeout) {
				println("  WARNING: Timeout on iteration", i)
				break
			}
			time.Sleep(1 * time.Microsecond)
		}

		// End timing
		endTime := micros()
		latency := uint32(endTime - startTime)

		// Record sample
		samples = append(samples, latency)
		totalLatency += uint64(latency)

		if latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}

		// Small delay between tests
		time.Sleep(100 * time.Microsecond)
	}

	// Calculate statistics
	avgLatency := uint32(totalLatency / uint64(len(samples)))

	println("\nResults:")
	println("  Samples:", len(samples))
	println("  Min latency:", minLatency, "μs")
	println("  Max latency:", maxLatency, "μs")
	println("  Avg latency:", avgLatency, "μs")
	println("  Jitter (max-min):", maxLatency-minLatency, "μs")

	// Calculate percentiles
	sortedSamples := make([]uint32, len(samples))
	copy(sortedSamples, samples)
	bubbleSort(sortedSamples)

	p50 := sortedSamples[len(sortedSamples)*50/100]
	p90 := sortedSamples[len(sortedSamples)*90/100]
	p99 := sortedSamples[len(sortedSamples)*99/100]

	println("\nPercentiles:")
	println("  P50 (median):", p50, "μs")
	println("  P90:", p90, "μs")
	println("  P99:", p99, "μs")

	// Interpretation for real-time systems
	println("\nInterpretation:")
	if avgLatency < 50 {
		println("  ✓ Excellent - suitable for high-frequency stepper control")
	} else if avgLatency < 100 {
		println("  ✓ Good - suitable for most real-time tasks")
	} else if avgLatency < 200 {
		println("  ⚠ Moderate - may limit highest frequency operations")
	} else {
		println("  ⚠ High - consider optimization for time-critical tasks")
	}

	if maxLatency-minLatency < 50 {
		println("  ✓ Low jitter - very predictable timing")
	} else if maxLatency-minLatency < 100 {
		println("  ✓ Acceptable jitter - good for most applications")
	} else {
		println("  ⚠ High jitter - may cause timing variability")
	}
}

// testVolatileRoundTripLatency measures latency using volatile registers
func testVolatileRoundTripLatency(iterations int) {
	println("Measuring round-trip latency using volatile.Register32...")
	println("Iterations:", iterations)

	var samples []uint32
	var minLatency uint32 = 0xFFFFFFFF
	var maxLatency uint32 = 0
	var totalLatency uint64 = 0

	for i := 0; i < iterations; i++ {
		// Clear previous response
		volatileLatencyResponse.Set(0)

		// Start timing
		startTime := micros()

		// Send trigger
		testValue := uint32(i + 1)
		volatileLatencyTrigger.Set(testValue)

		// Wait for response
		timeout := time.Now().Add(10 * time.Millisecond)
		for volatileLatencyResponse.Get() != testValue {
			if time.Now().After(timeout) {
				println("  WARNING: Timeout on iteration", i)
				break
			}
			time.Sleep(1 * time.Microsecond)
		}

		// End timing
		endTime := micros()
		latency := uint32(endTime - startTime)

		samples = append(samples, latency)
		totalLatency += uint64(latency)

		if latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}

		time.Sleep(100 * time.Microsecond)
	}

	avgLatency := uint32(totalLatency / uint64(len(samples)))

	println("\nResults:")
	println("  Samples:", len(samples))
	println("  Min latency:", minLatency, "μs")
	println("  Max latency:", maxLatency, "μs")
	println("  Avg latency:", avgLatency, "μs")
	println("  Jitter (max-min):", maxLatency-minLatency, "μs")

	println("\nComparison to atomic variables:")
	println("  Volatile registers may have slightly different performance")
	println("  characteristics depending on compiler optimizations")
}

// testLatencyUnderLoad measures latency while both cores are doing work
func testLatencyUnderLoad(iterations int) {
	println("Measuring latency while Core 1 is under computational load...")
	println("This simulates real-world conditions where cores are busy.")
	println("Iterations:", iterations)

	// Add load to Core 1 by incrementing a counter rapidly
	var loadCounter atomic.Uint32
	loadActive := atomic.Bool{}
	loadActive.Store(true)

	// Create artificial load (this would run on Core 1 via scheduler)
	go func() {
		for loadActive.Load() {
			loadCounter.Add(1)
			// Busy work
			for j := 0; j < 100; j++ {
				_ = j * j
			}
		}
	}()

	time.Sleep(100 * time.Millisecond) // Let load stabilize

	var samples []uint32
	var minLatency uint32 = 0xFFFFFFFF
	var maxLatency uint32 = 0
	var totalLatency uint64 = 0

	for i := 0; i < iterations; i++ {
		latencyResponse.Store(0)
		startTime := micros()

		testValue := uint32(i + 1000) // Different range to avoid conflicts
		latencyTrigger.Store(testValue)

		timeout := time.Now().Add(20 * time.Millisecond) // Longer timeout under load
		for latencyResponse.Load() != testValue {
			if time.Now().After(timeout) {
				println("  WARNING: Timeout on iteration", i)
				break
			}
			time.Sleep(1 * time.Microsecond)
		}

		endTime := micros()
		latency := uint32(endTime - startTime)

		samples = append(samples, latency)
		totalLatency += uint64(latency)

		if latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}

		time.Sleep(200 * time.Microsecond)
	}

	// Stop load
	loadActive.Store(false)
	time.Sleep(50 * time.Millisecond)

	avgLatency := uint32(totalLatency / uint64(len(samples)))

	println("\nResults under load:")
	println("  Samples:", len(samples))
	println("  Min latency:", minLatency, "μs")
	println("  Max latency:", maxLatency, "μs")
	println("  Avg latency:", avgLatency, "μs")
	println("  Jitter (max-min):", maxLatency-minLatency, "μs")
	println("  Load counter reached:", loadCounter.Load())

	println("\nInterpretation:")
	println("  This represents worst-case latency when Core 1 is busy.")
	println("  In Gopper, this would be stepper control or sensor processing.")
	println("  Higher latency under load is expected and acceptable if predictable.")
}

// testJitter measures timing jitter in detail
func testJitter(iterations int) {
	println("Detailed jitter analysis over", iterations, "iterations...")
	println("Jitter = variation in latency, critical for real-time control")

	var samples []uint32
	var totalLatency uint64 = 0

	for i := 0; i < iterations; i++ {
		latencyResponse.Store(0)
		startTime := micros()

		testValue := uint32(i + 2000)
		latencyTrigger.Store(testValue)

		for latencyResponse.Load() != testValue {
			time.Sleep(1 * time.Microsecond)
		}

		endTime := micros()
		latency := uint32(endTime - startTime)

		samples = append(samples, latency)
		totalLatency += uint64(latency)

		time.Sleep(50 * time.Microsecond)
	}

	// Sort for percentile calculation
	sortedSamples := make([]uint32, len(samples))
	copy(sortedSamples, samples)
	bubbleSort(sortedSamples)

	avgLatency := uint32(totalLatency / uint64(len(samples)))
	minLatency := sortedSamples[0]
	maxLatency := sortedSamples[len(sortedSamples)-1]

	// Calculate standard deviation (approximation)
	var sumSquaredDiff uint64 = 0
	for _, sample := range samples {
		diff := int32(sample) - int32(avgLatency)
		sumSquaredDiff += uint64(diff * diff)
	}
	variance := sumSquaredDiff / uint64(len(samples))
	stdDev := uint32(sqrt(variance))

	println("\nJitter Statistics:")
	println("  Mean:", avgLatency, "μs")
	println("  Std Dev:", stdDev, "μs")
	println("  Min:", minLatency, "μs")
	println("  Max:", maxLatency, "μs")
	println("  Range (jitter):", maxLatency-minLatency, "μs")

	// Percentiles
	p1 := sortedSamples[len(sortedSamples)*1/100]
	p5 := sortedSamples[len(sortedSamples)*5/100]
	p50 := sortedSamples[len(sortedSamples)*50/100]
	p95 := sortedSamples[len(sortedSamples)*95/100]
	p99 := sortedSamples[len(sortedSamples)*99/100]

	println("\nPercentile Distribution:")
	println("  P1 :", p1, "μs")
	println("  P5 :", p5, "μs")
	println("  P50:", p50, "μs (median)")
	println("  P95:", p95, "μs")
	println("  P99:", p99, "μs")

	// Calculate coefficient of variation (CV = stddev/mean)
	cv := float32(stdDev) / float32(avgLatency)

	println("\nPredictability Metrics:")
	println("  Coefficient of Variation:", formatFloat(cv))
	if cv < 0.1 {
		println("  ✓ Excellent - very predictable timing (CV < 10%)")
	} else if cv < 0.2 {
		println("  ✓ Good - acceptable for real-time (CV < 20%)")
	} else if cv < 0.3 {
		println("  ⚠ Moderate - some timing variability (CV < 30%)")
	} else {
		println("  ⚠ High - significant jitter, may need optimization")
	}

	println("\nFor Gopper stepper control:")
	if maxLatency < 100 {
		println("  ✓ Suitable for high-speed stepper control (>10 kHz)")
	} else if maxLatency < 500 {
		println("  ✓ Suitable for normal stepper speeds (>2 kHz)")
	} else {
		println("  ⚠ May limit maximum stepper speeds")
	}
}

// printRecommendations provides guidance based on all measurements
func printRecommendations() {
	println("\n=== Recommendations for Gopper Real-Time Design ===\n")

	println("1. Core Assignment:")
	println("   - Core 0: Protocol handling, USB, command dispatch")
	println("   - Core 1: Stepper pulse generation, timing-critical tasks")

	println("\n2. Communication Strategy:")
	println("   - Use atomic variables for control signals (fast, predictable)")
	println("   - Use channels for complex data (easier to program)")
	println("   - Avoid blocking operations on Core 1")

	println("\n3. Stepper Control Implications:")
	println("   - With ~50-100μs latency, can achieve >10 kHz step rates")
	println("   - Use dedicated timer interrupts for step pulses if needed")
	println("   - Core 1 can handle tight timing loops independently")

	println("\n4. Jitter Mitigation:")
	println("   - Keep Core 1 workload consistent")
	println("   - Avoid dynamic memory allocation in hot paths")
	println("   - Use fixed-size buffers and lock-free data structures")

	println("\n5. Testing Requirements:")
	println("   - Always test under realistic load conditions")
	println("   - Monitor worst-case latency, not just average")
	println("   - Validate timing with oscilloscope for critical paths")

	println("\n6. Fallback Strategies:")
	println("   - If jitter too high, use hardware timers for critical pulses")
	println("   - Consider DMA for repetitive tasks")
	println("   - Profile and optimize hot paths on Core 1")
}

// Helper: Simple bubble sort for small arrays
func bubbleSort(arr []uint32) {
	n := len(arr)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if arr[j] > arr[j+1] {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
}

// Helper: Integer square root (Babylonian method)
func sqrt(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}

// Helper: Format float for printing
func formatFloat(f float32) string {
	// Simple float formatting (2 decimal places)
	intPart := int32(f)
	fracPart := int32((f - float32(intPart)) * 100)
	if fracPart < 0 {
		fracPart = -fracPart
	}

	result := ""
	if intPart < 0 || f < 0 {
		result += "-"
		if intPart < 0 {
			intPart = -intPart
		}
	}

	// Convert int to string
	if intPart == 0 {
		result += "0"
	} else {
		digits := []byte{}
		temp := intPart
		for temp > 0 {
			digits = append([]byte{byte('0' + temp%10)}, digits...)
			temp /= 10
		}
		result += string(digits)
	}

	result += "."

	// Add fractional part
	if fracPart < 10 {
		result += "0"
	}
	if fracPart == 0 {
		result += "0"
	} else {
		digits := []byte{}
		temp := fracPart
		for temp > 0 {
			digits = append([]byte{byte('0' + temp%10)}, digits...)
			temp /= 10
		}
		result += string(digits)
	}

	return result
}
