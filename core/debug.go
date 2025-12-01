package core

// DebugWriter is a function type for writing debug messages
type DebugWriter func(string)

// TimingEvent captures a timing-critical event for post-mortem analysis
type TimingEvent struct {
	EventType uint8  // Event type code
	OID       uint8  // Object ID (stepper, etc.)
	Clock     uint32 // System clock at event
	Value1    uint32 // Context-dependent value
	Value2    uint32 // Context-dependent value
}

// Event type codes
const (
	EvtQueueStep     = 1 // queue_step received
	EvtLoadMove      = 2 // Move loaded from queue
	EvtTimerSchedule = 3 // Timer scheduled
	EvtTimerFire     = 4 // Timer fired (step generated)
	EvtTimerPast     = 5 // Timer in past detected
	EvtResetClock    = 6 // reset_step_clock received
)

const (
	TimingRingSize = 32 // Keep last 32 events for post-mortem
)

var (
	// debugPrintln is the global debug print function (can be set by platform code)
	debugPrintln DebugWriter = func(s string) {} // No-op by default

	// debugEnabled controls whether debug output is active
	// Disabled by default for performance; enable with set_debug enable=1
	debugEnabled bool = false

	// Timing capture ring buffer (non-blocking, for post-mortem)
	timingRing     [TimingRingSize]TimingEvent
	timingRingHead uint8        // Next write position
	timingEnabled  bool  = true // Always capture timing events

	// Async debug output channel
	debugChan chan string
)

// SetDebugWriter sets the platform-specific debug output function
// This allows platforms to redirect debug output to UART, USB, etc.
func SetDebugWriter(writer DebugWriter) {
	debugPrintln = writer
}

// SetDebugEnabled enables or disables debug output
// Useful for benchmarks where debug output would affect timing
func SetDebugEnabled(enabled bool) {
	debugEnabled = enabled
}

// IsDebugEnabled returns whether debug output is enabled
func IsDebugEnabled() bool {
	return debugEnabled
}

// InitAsyncDebug starts the async debug output goroutine
// Call this from main() after SetDebugWriter
func InitAsyncDebug() {
	debugChan = make(chan string, 16) // Buffer 16 messages
	go debugOutputWorker()
}

// debugOutputWorker runs in background, drains debug channel
func debugOutputWorker() {
	for msg := range debugChan {
		if debugPrintln != nil {
			debugPrintln(msg)
		}
	}
}

// DebugPrintln writes a debug message using the platform-specific writer
// Blocks if debug is enabled (use DebugAsync for non-blocking)
func DebugPrintln(msg string) {
	if debugEnabled && debugPrintln != nil {
		debugPrintln(msg)
	}
}

// DebugAsync queues a debug message for async output (non-blocking)
// Returns immediately even if channel is full (drops message)
func DebugAsync(msg string) {
	if debugChan != nil {
		select {
		case debugChan <- msg:
		default:
			// Channel full, drop message (non-blocking)
		}
	}
}

// RecordTiming captures a timing event in the ring buffer
// This is always non-blocking and very fast (~20ns)
func RecordTiming(eventType, oid uint8, clock, value1, value2 uint32) {
	if !timingEnabled {
		return
	}
	idx := timingRingHead
	timingRing[idx] = TimingEvent{
		EventType: eventType,
		OID:       oid,
		Clock:     clock,
		Value1:    value1,
		Value2:    value2,
	}
	timingRingHead = (idx + 1) % TimingRingSize
}

// DumpTimingRing outputs the timing ring buffer (call on shutdown/error)
// This should be called from a goroutine or after stopping time-critical code
func DumpTimingRing() {
	if debugPrintln == nil {
		return
	}

	debugPrintln("[TIMING] === Timing Ring Dump ===")
	debugPrintln("[TIMING] Total steps executed: " + itoa(int(GetTotalStepCount())))

	// Read from oldest to newest
	start := timingRingHead
	for i := uint8(0); i < TimingRingSize; i++ {
		idx := (start + i) % TimingRingSize
		evt := &timingRing[idx]
		if evt.EventType == 0 {
			continue // Empty slot
		}

		var name string
		switch evt.EventType {
		case EvtQueueStep:
			name = "QUEUE_STEP"
		case EvtLoadMove:
			name = "LOAD_MOVE"
		case EvtTimerSchedule:
			name = "TIMER_SCHED"
		case EvtTimerFire:
			name = "TIMER_FIRE"
		case EvtTimerPast:
			name = "TIMER_PAST!"
		case EvtResetClock:
			name = "RESET_CLK"
		default:
			name = "UNKNOWN"
		}

		debugPrintln("[TIMING] " + name +
			" oid=" + itoa(int(evt.OID)) +
			" clock=" + itoa(int(evt.Clock)) +
			" v1=" + itoa(int(evt.Value1)) +
			" v2=" + itoa(int(evt.Value2)))
	}
	debugPrintln("[TIMING] === End Dump ===")
}

// ClearTimingRing clears the timing buffer
func ClearTimingRing() {
	for i := range timingRing {
		timingRing[i] = TimingEvent{}
	}
	timingRingHead = 0
}
