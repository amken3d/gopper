//go:build tinygo

package core

import "machine"

// ADCPin identifies an ADC pin.
type ADCPin machine.ADC

// ADCChannelID identifies a logical ADC channel.
type ADCChannelID machine.ADCChannel

// ADCValue is the "raw" ADC reading as seen by the rest of the firmware.
// Convention here: 16-bit value, even if underlying hardware is 12 bits.
type ADCValue uint16

// ADCConfig is the high-level config the core cares about.
type ADCConfig machine.ADCConfig

// ADCDriver is the abstract ADC interface that core code uses.
type ADCDriver interface {
	// Init powers up and configures the ADC peripheral.
	Init(cfg ADCConfig) error

	// ConfigureChannel prepares a channel for analog input.
	// For pin-muxed channels, this should set pin to analog mode.
	ConfigureChannel(ch ADCChannelID) error

	// ReadRaw performs a one-shot sample from the given channel.
	// Returns a 16-bit scaled value (e.g. 12-bit HW value left-shifted).
	ReadRaw(ch ADCChannelID) (ADCValue, error)
}

// Global singleton used by core code.
var adcDriver ADCDriver

// SetADCDriver is called by target-specific code to register its driver.
func SetADCDriver(d ADCDriver) {
	adcDriver = d
}

// MustADC returns the configured driver or panics if missing.
func MustADC() ADCDriver {
	if adcDriver == nil {
		panic("ADC driver not configured")
	}
	return adcDriver
}
