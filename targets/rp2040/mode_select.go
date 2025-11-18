//go:build rp2040 || rp2350

package main

// ModeConfig determines which mode to run
type ModeConfig struct {
	// Set to true to run in standalone mode
	// Set to false to run in Klipper protocol mode
	Standalone bool
}

// GetMode returns the current mode configuration
// This can be modified at compile time or runtime
func GetMode() ModeConfig {
	// Default: Klipper mode for backward compatibility
	// To enable standalone mode, change this to true
	return ModeConfig{
		Standalone: false,
	}
}

// To enable standalone mode, you can:
// 1. Change the Standalone value above to true
// 2. Add a build tag: go build -tags standalone
// 3. Read from a config file or GPIO pin state
