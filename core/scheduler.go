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
)

var (
	timerList   *Timer
	currentTime uint32
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
func insertTimer(t *Timer) {
	if timerList == nil || t.WakeTime < timerList.WakeTime {
		t.Next = timerList
		timerList = t
		return
	}

	current := timerList
	for current.Next != nil && current.Next.WakeTime < t.WakeTime {
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
	for timerList != nil && timerList.WakeTime <= currentTime {
		timer := timerList
		timerList = timer.Next
		timer.Next = nil // Clear Next pointer to avoid circular references

		// Call handler
		result := timer.Handler(timer)

		// Reschedule if requested
		if result == SF_RESCHEDULE {
			insertTimer(timer)
		}
	}
}
