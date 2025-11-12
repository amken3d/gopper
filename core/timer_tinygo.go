//go:build tinygo

package core

import "sync/atomic"

var systemTicksValue uint32

// getSystemTicks returns the current system ticks
func getSystemTicks() uint32 {
	return atomic.LoadUint32(&systemTicksValue)
}

// setSystemTicks sets the system ticks
func setSystemTicks(ticks uint32) {
	atomic.StoreUint32(&systemTicksValue, ticks)
}
