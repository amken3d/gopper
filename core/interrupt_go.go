//go:build !tinygo

package core

// State is a placeholder for interrupt state on regular Go
type State uintptr

// disableInterrupts is a no-op on regular Go (for testing)
func disableInterrupts() State {
	return 0
}

// restoreInterrupts is a no-op on regular Go (for testing)
func restoreInterrupts(state State) {
	// No-op
}
