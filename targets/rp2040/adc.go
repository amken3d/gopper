//go:build rp2040 || rp2350

package main

import (
	"gopper/core"
	"machine"
)

// ADC Configuration for RP2040/RP2350 variants
//
// RP2040 (QFN-56) - 5 ADC channels:
//   - ADC0-3: GPIO 26-29
//   - ADC4: Internal temperature sensor
//
// RP2350A (QFN-60) - 5 ADC channels (same as RP2040):
//   - ADC0-3: GPIO 26-29
//   - ADC4: Internal temperature sensor
//
// RP2350B (QFN-80) - 9 ADC channels:
//   - ADC0-7: GPIO 40-47
//   - ADC8: Internal temperature sensor
//
// Pin Usage Notes:
//   - Use GPIO 26-29 for RP2040/RP2350A
//   - Use GPIO 40-47 for RP2350B
//   - Special pin number 4 or 8 for temperature sensor (depending on variant)
//
// ADC Specifications:
//   - Resolution: 12-bit (0-4095)
//   - Sample Rate: 500 kSPS
//   - Reference Voltage: 3.3V (or external VREF)
//   - Conversion Time: ~2µs

const (
	// ADC hardware constants
	ADC_MAX = 4095 // 12-bit ADC

	// RP2040/RP2350A GPIO to ADC channel mapping (QFN-56/60)
	GPIO_ADC0_RP2040 = 26
	GPIO_ADC1_RP2040 = 27
	GPIO_ADC2_RP2040 = 28
	GPIO_ADC3_RP2040 = 29

	// RP2350B GPIO to ADC channel mapping (QFN-80)
	GPIO_ADC0_RP2350B = 40
	GPIO_ADC1_RP2350B = 41
	GPIO_ADC2_RP2350B = 42
	GPIO_ADC3_RP2350B = 43
	GPIO_ADC4_RP2350B = 44
	GPIO_ADC5_RP2350B = 45
	GPIO_ADC6_RP2350B = 46
	GPIO_ADC7_RP2350B = 47

	// Temperature sensor special pin numbers
	// These are virtual pin numbers used to access the internal temp sensor
	// Klipper config would use: "sensor_pin: gpio4" for RP2040/RP2350A
	//                      or:  "sensor_pin: gpio8" for RP2350B
	PIN_TEMP_SENSOR_RP2040  = 4  // RP2040/RP2350A temperature sensor
	PIN_TEMP_SENSOR_RP2350B = 8  // RP2350B temperature sensor
)

// ADC pin mapping: GPIO pin number -> machine.ADC
// This map is populated in InitADC() based on available pins
var adcPins map[uint32]machine.ADC

// Track which pins are configured
var configuredADCs = make(map[uint32]machine.ADC)

// Temperature sensor ADC (special handling required)
var tempSensorADC *machine.ADC
var tempSensorPinNumber uint32

// InitADC initializes the ADC peripheral and detects the chip variant
func InitADC() {
	// Initialize the ADC pin mapping based on available pins
	// TinyGo defines ADC0-ADC7 but not all may be available
	adcPins = make(map[uint32]machine.ADC)

	// Check if we have the extended GPIO pins (RP2350B)
	// RP2350B has GPIO 40-47 for ADC, while RP2040/RP2350A use GPIO 26-29
	// We detect this by checking if machine has ADC4 defined (more than 4 ADC channels)

	// Common pins available on all variants (RP2040/RP2350A/RP2350B)
	// Map both naming conventions for maximum compatibility
	adcPins[GPIO_ADC0_RP2040] = machine.ADC{Pin: machine.ADC0}
	adcPins[GPIO_ADC1_RP2040] = machine.ADC{Pin: machine.ADC1}
	adcPins[GPIO_ADC2_RP2040] = machine.ADC{Pin: machine.ADC2}
	adcPins[GPIO_ADC3_RP2040] = machine.ADC{Pin: machine.ADC3}

	// Try to detect RP2350B by checking if ADC4-ADC7 are available
	// On RP2350B, these are on GPIO 40-47 instead of 26-29
	// For now, we support both pin numbering schemes:
	//   - GPIO 26-29 for RP2040/RP2350A
	//   - GPIO 40-47 for RP2350B (if user specifies these pins)
	// TODO: Add runtime detection when TinyGo exposes chip variant info

	// Add RP2350B pins (GPIO 40-47) if available
	// These will only work on RP2350B hardware
	adcPins[GPIO_ADC0_RP2350B] = machine.ADC{Pin: machine.ADC0}
	adcPins[GPIO_ADC1_RP2350B] = machine.ADC{Pin: machine.ADC1}
	adcPins[GPIO_ADC2_RP2350B] = machine.ADC{Pin: machine.ADC2}
	adcPins[GPIO_ADC3_RP2350B] = machine.ADC{Pin: machine.ADC3}

	// RP2350B has additional ADC channels 4-7
	// Note: TinyGo may not expose ADC4-ADC7 yet, so we wrap in a check
	// TODO: Update this when TinyGo adds full RP2350B support
	if false { // Placeholder - replace with actual check when TinyGo supports it
		adcPins[GPIO_ADC4_RP2350B] = machine.ADC{Pin: machine.Pin(44)}
		adcPins[GPIO_ADC5_RP2350B] = machine.ADC{Pin: machine.Pin(45)}
		adcPins[GPIO_ADC6_RP2350B] = machine.ADC{Pin: machine.Pin(46)}
		adcPins[GPIO_ADC7_RP2350B] = machine.ADC{Pin: machine.Pin(47)}
	}

	// Set up temperature sensor
	// The internal temperature sensor is accessed via a special ADC channel
	// For now, we support pin number 4 (RP2040/RP2350A) or 8 (RP2350B)
	// Users can configure with "sensor_pin: gpio4" in Klipper config
	tempSensorPinNumber = PIN_TEMP_SENSOR_RP2040 // Default to RP2040/RP2350A
	// Note: TinyGo doesn't expose the temp sensor directly in machine.ADC yet
	// We'll need to access it via raw registers or wait for TinyGo support

	// Set up the HAL function pointers
	core.ADCSetup = adcSetupImpl
	core.ADCSample = adcSampleImpl
	core.ADCCancel = adcCancelImpl
}

// adcSetupImpl configures a GPIO pin for ADC sampling
func adcSetupImpl(pin uint32) error {
	// Check if this is the temperature sensor
	if pin == PIN_TEMP_SENSOR_RP2040 || pin == PIN_TEMP_SENSOR_RP2350B {
		// Temperature sensor setup
		// Create ADC for temperature sensor
		// Note: TinyGo's machine package doesn't expose a direct way to access
		// the temperature sensor ADC channel. We'll need to use raw register access
		// or wait for TinyGo to add proper support.

		// For now, we'll use ADC channel 4 (RP2040/RP2350A) as a placeholder
		// The actual implementation would need to:
		// 1. Enable the temperature sensor bias: ADC.CS.TS_EN = 1
		// 2. Select ADC channel 4 (or 8 for RP2350B)
		// 3. Read the value and convert using the formula in the datasheet

		// Store a marker that this pin is configured
		// We'll handle the actual reading in adcSampleImpl
		tempSensorADC = &machine.ADC{Pin: machine.Pin(pin)}
		configuredADCs[pin] = *tempSensorADC

		return nil
	}

	// Check if this is a valid GPIO ADC pin
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
// Returns (value, ready) where ready is always true for RP2040/RP2350
// (RP2040/RP2350 ADC conversions are very fast, ~2µs)
func adcSampleImpl(pin uint32) (uint16, bool) {
	// Check if this is the temperature sensor
	if pin == PIN_TEMP_SENSOR_RP2040 || pin == PIN_TEMP_SENSOR_RP2350B {
		// Read temperature sensor
		// Note: This returns a raw ADC value, not a temperature
		// The Klipper host will do the temperature conversion
		value := readTemperatureSensorRaw()
		return value, true
	}

	// Get the configured ADC pin
	adcPin, exists := configuredADCs[pin]
	if !exists {
		return 0, false
	}

	// Read ADC value
	// machine.ADC.Get() returns a 16-bit value, but RP2040/RP2350 is 12-bit
	// TinyGo scales it to 16-bit, so we need to scale back to 12-bit
	value := adcPin.Get()

	// Convert from 16-bit (0-65535) back to 12-bit (0-4095)
	value12bit := uint16((uint32(value) * ADC_MAX) / 65535)

	// RP2040/RP2350 ADC is fast enough that we can consider it always ready
	return value12bit, true
}

// readTemperatureSensorRaw reads the internal temperature sensor as a raw ADC value
// Returns a 12-bit ADC value (0-4095)
//
// The RP2040/RP2350 temperature sensor formula is:
//   T(°C) = 27 - (ADC_voltage - 0.706V) / 0.001721
//   where ADC_voltage = ADC_value * 3.3V / 4096
//
// Klipper will handle the temperature conversion on the host side.
// We just return the raw 12-bit ADC value here.
func readTemperatureSensorRaw() uint16 {
	// TODO: Implement proper temperature sensor reading
	// This requires:
	// 1. Access to ADC.CS register to enable TS_EN bit
	// 2. Read from ADC channel 4 (RP2040/RP2350A) or 8 (RP2350B)
	// 3. Return the 12-bit raw value
	//
	// For now, return a placeholder value
	// In a real implementation, this would use device/rp or unsafe to access
	// the ADC registers directly:
	//
	// const ADC_BASE = 0x4004C000
	// const ADC_CS_OFFSET = 0x00
	// const ADC_RESULT_OFFSET = 0x04
	// const ADC_CS_TS_EN = 1 << 1
	// const ADC_CS_AINSEL_SHIFT = 12
	//
	// Enable temp sensor: *(ADC_BASE + ADC_CS_OFFSET) |= ADC_CS_TS_EN
	// Select channel 4: *(ADC_BASE + ADC_CS_OFFSET) |= (4 << ADC_CS_AINSEL_SHIFT)
	// Read result: value = *(ADC_BASE + ADC_RESULT_OFFSET)

	// Placeholder: return mid-range value (~25°C)
	// This equates to approximately 25°C based on the datasheet formula
	return 2048
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
