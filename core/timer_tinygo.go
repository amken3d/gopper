//go:build tinygo

package core

import "sync/atomic"

var (
	systemTicksValue uint32
	// hardwareTimerFunc is set by platform code to read the actual hardware timer
	// When set, getSystemTicks() reads hardware directly instead of cached value
	hardwareTimerFunc func() uint32
)

// getSystemTicks returns the current system ticks
// If a hardware timer function is registered, reads hardware directly for accuracy
// Otherwise falls back to cached value (for testing or platforms without direct access)
func getSystemTicks() uint32 {
	if hardwareTimerFunc != nil {
		return hardwareTimerFunc()
	}
	return atomic.LoadUint32(&systemTicksValue)
}

// setSystemTicks sets the system ticks (for cached mode)
func setSystemTicks(ticks uint32) {
	atomic.StoreUint32(&systemTicksValue, ticks)
}

// SetHardwareTimerFunc registers a function to read the hardware timer directly
// This should be called during platform initialization before any timer operations
// Once set, GetTime() will always return the actual hardware time
func SetHardwareTimerFunc(f func() uint32) {
	hardwareTimerFunc = f
}
