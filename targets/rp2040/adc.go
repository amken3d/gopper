//go:build rp2040 || rp2350

package main

import (
	"gopper/core"
	"machine"
)

// RP2040 ADC configuration
// The RP2040 has 5 ADC inputs:
// - ADC0 (GPIO 26)
// - ADC1 (GPIO 27)
// - ADC2 (GPIO 28)
// - ADC3 (GPIO 29) / ADC_VREF
// - ADC4 (Temperature sensor - internal)
//
// ADC resolution: 12-bit (0-4095)
// Reference voltage: 3.3V (or external VREF on ADC3)

const (
	// ADC hardware constants
	ADC_MAX = 4095 // 12-bit ADC

	// GPIO to ADC channel mapping
	GPIO_ADC0 = 26
	GPIO_ADC1 = 27
	GPIO_ADC2 = 28
	GPIO_ADC3 = 29
)

// ADC pin mapping: GPIO pin number -> machine.ADC
var adcPins = map[uint32]machine.ADC{
	GPIO_ADC0: machine.ADC{Pin: machine.ADC0},
	GPIO_ADC1: machine.ADC{Pin: machine.ADC1},
	GPIO_ADC2: machine.ADC{Pin: machine.ADC2},
	GPIO_ADC3: machine.ADC{Pin: machine.ADC3},
	// Note: ADC4 (temperature sensor) can be added if needed
}

// Track which pins are configured
var configuredADCs = make(map[uint32]machine.ADC)

// InitADC initializes the ADC peripheral on RP2040
func InitADC() {
	// TinyGo's machine.ADC handles most initialization automatically
	// when Configure() is called on individual pins

	// Set up the HAL function pointers
	core.ADCSetup = adcSetupImpl
	core.ADCSample = adcSampleImpl
	core.ADCCancel = adcCancelImpl
}

// adcSetupImpl configures a GPIO pin for ADC sampling
func adcSetupImpl(pin uint32) error {
	// Check if this is a valid ADC pin
	adcPin, exists := adcPins[pin]
	if !exists {
		// Not an ADC-capable pin
		return errInvalidADCPin
	}

	// Configure the ADC pin
	adcPin.Configure(machine.ADCConfig{})

	// Store configured ADC
	configuredADCs[pin] = adcPin

	return nil
}

// adcSampleImpl reads an ADC value from the specified pin
// Returns (value, ready) where ready is always true for RP2040
// (RP2040 ADC conversions are very fast, ~2µs)
func adcSampleImpl(pin uint32) (uint16, bool) {
	// Get the configured ADC pin
	adcPin, exists := configuredADCs[pin]
	if !exists {
		return 0, false
	}

	// Read ADC value
	// machine.ADC.Get() returns a 16-bit value, but RP2040 is 12-bit
	// TinyGo scales it to 16-bit, so we need to scale back to 12-bit
	value := adcPin.Get()

	// Convert from 16-bit (0-65535) back to 12-bit (0-4095)
	value12bit := uint16((uint32(value) * ADC_MAX) / 65535)

	// RP2040 ADC is fast enough that we can consider it always ready
	return value12bit, true
}

// adcCancelImpl cancels any pending ADC conversion
// For RP2040, conversions are synchronous and very fast, so this is a no-op
func adcCancelImpl(pin uint32) {
	// No-op for RP2040 - conversions are immediate
}

// Error type for invalid ADC pin
type adcError string

func (e adcError) Error() string {
	return string(e)
}

const errInvalidADCPin = adcError("invalid ADC pin")
