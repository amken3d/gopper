package core

// Timer represents a scheduled event
type Timer struct {
	WakeTime uint32
	Handler  func(*Timer) uint8
	Next     *Timer
}

const (
	SF_DONE       = 0
	SF_RESCHEDULE = 1

	// Timer in past threshold - if timer is more than 100ms behind, report error
	// At 12MHz, 100ms = 1,200,000 ticks
	TimerPastThreshold = 1200000
)

var (
	timerList       *Timer
	currentTime     uint32
	timerPastErrors uint32 // Count of "timer in past" errors
)

// ScheduleTimer adds a timer to the schedule
func ScheduleTimer(t *Timer) {
	state := disableInterrupts()
	defer restoreInterrupts(state)

	// Insert timer in sorted order
	// Implementation similar to Klipper's sched_add_timer
	insertTimer(t)
}

// insertTimer inserts a timer in sorted order by WakeTime
// Uses signed comparison to handle 32-bit wrap-around correctly
func insertTimer(t *Timer) {
	// Use signed comparison: int32(a - b) < 0 means a is before b
	// This handles wrap-around correctly within half the 32-bit range (~35 min at 1MHz)
	if timerList == nil || int32(t.WakeTime-timerList.WakeTime) < 0 {
		t.Next = timerList
		timerList = t
		return
	}

	current := timerList
	for current.Next != nil && int32(current.Next.WakeTime-t.WakeTime) < 0 {
		current = current.Next
	}

	t.Next = current.Next
	current.Next = t
}

// TimerDispatch processes due timers
func TimerDispatch() {
	state := disableInterrupts()
	defer restoreInterrupts(state)

	// Process all timers with WakeTime <= currentTime
	// Use signed comparison to handle 32-bit wrap-around:
	// int32(currentTime - WakeTime) >= 0 means timer is due
	for timerList != nil && int32(currentTime-timerList.WakeTime) >= 0 {
		timer := timerList
		timerList = timer.Next
		timer.Next = nil // Clear Next pointer to avoid circular references

		// Check for "timer in past" condition - timer is too far behind
		// This indicates the MCU can't keep up with requested step rate
		// Use signed comparison to handle 32-bit wrap-around correctly
		timeDiff := int32(currentTime - timer.WakeTime)
		if timeDiff > int32(TimerPastThreshold) {
			timerPastErrors++

			// Debug output BEFORE any other action
			DebugPrintln("[SCHED] TIMER IN PAST! Shutting down...")

			// Record timing event for post-mortem analysis
			RecordTiming(EvtTimerPast, 0, currentTime, timer.WakeTime, uint32(timeDiff))

			// NOTE: Removed "go DumpTimingRing()" - spawning goroutine with
			// interrupts disabled causes crash on TinyGo

			// Trigger shutdown with "Rescheduled timer in the past" error
			TryShutdown("Rescheduled timer in the past")
			return
		}

		// Call handler
		result := timer.Handler(timer)

		// Reschedule if requested
		if result == SF_RESCHEDULE {
			insertTimer(timer)
		}

		// CRITICAL: Re-read current time after each timer handler
		// Timer handlers may block (e.g., PIO FIFO full), advancing real time
		// Without this, all subsequent timers appear "due" even if scheduled for the future
		currentTime = GetTime()
	}
}

// GetTimerPastErrors returns the count of timer-in-past errors
func GetTimerPastErrors() uint32 {
	return timerPastErrors
}

// ResetTimerPastErrors resets the error counter
func ResetTimerPastErrors() {
	timerPastErrors = 0
}
