//go:build tinygo

package core

import "runtime/interrupt"

// disableInterrupts disables interrupts and returns the previous state
func disableInterrupts() interrupt.State {
	return interrupt.Disable()
}

// restoreInterrupts restores the interrupt state
func restoreInterrupts(state interrupt.State) {
	interrupt.Restore(state)
}
