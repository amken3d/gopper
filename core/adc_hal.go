// ADC Hardware Abstraction Layer
// Defines the interface for platform-specific ADC implementations
package core

// ADCSetup initializes an ADC pin for sampling
// Returns error if pin is not ADC-capable
// Platform-specific implementation required in targets/*/adc.go
var ADCSetup func(pin uint32) error

// ADCSample attempts to read an ADC value from the specified pin
// Returns (value, ready) where:
//
//	value: the ADC reading (0 if not ready)
//	ready: true if conversion complete, false if still in progress
//
// Platform-specific implementation required in targets/*/adc.go
var ADCSample func(pin uint32) (uint16, bool)

// ADCCancel cancels any pending ADC conversion on the specified pin
// Platform-specific implementation required in targets/*/adc.go
var ADCCancel func(pin uint32)
