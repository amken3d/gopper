package core

// Timer frequencies for common MCUs
const (
	TimerFreq = 12000000 // 12MHz default timer frequency
)

var (
	systemTicks uint32
	bootTime    uint64 // Time at boot for uptime calculation
)

// GetTime returns the current system time in timer ticks
func GetTime() uint32 {
	return getSystemTicks()
}

// SetTime sets the current system time (for testing/hardware integration)
func SetTime(ticks uint32) {
	setSystemTicks(ticks)
}

// GetUptime returns 64-bit uptime in timer ticks
func GetUptime() uint64 {
	// Return current time as 64-bit value
	// In a real implementation with hardware, this would read a 64-bit counter
	return uint64(GetTime())
}

// TimerFromUS converts microseconds to timer ticks
func TimerFromUS(us uint32) uint32 {
	return (us * TimerFreq) / 1000000
}

// TimerToUS converts timer ticks to microseconds
func TimerToUS(ticks uint32) uint32 {
	return (ticks * 1000000) / TimerFreq
}

// TimerInit initializes the system timer
func TimerInit() {
	// Platform-specific initialization
	// This will be implemented differently for each target
	bootTime = uint64(GetTime())
}

// ProcessTimers processes scheduled timers
func ProcessTimers() {
	currentTime = GetTime()
	TimerDispatch()
}
