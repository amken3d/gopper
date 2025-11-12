//go:build !tinygo

package core

// getSystemTicks returns the current system ticks (regular Go implementation)
func getSystemTicks() uint32 {
	return systemTicks
}

// setSystemTicks sets the system ticks (regular Go implementation)
func setSystemTicks(ticks uint32) {
	systemTicks = ticks
}
