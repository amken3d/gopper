package core

// GPIOPin identifies a hardware GPIO pin number
type GPIOPin uint32

// GPIODriver is the abstract GPIO interface that core code uses.
// Platform-specific implementations handle actual hardware control.
type GPIODriver interface {
	// ConfigureOutput configures a pin as a digital output
	// Returns error if pin is invalid or already in use
	ConfigureOutput(pin GPIOPin) error

	// ConfigureInputPullUp configures a pin as a digital input with pull-up resistor
	ConfigureInputPullUp(pin GPIOPin) error

	// ConfigureInputPullDown configures a pin as a digital input with pull-down resistor
	ConfigureInputPullDown(pin GPIOPin) error

	// SetPin sets the pin to high (true) or low (false)
	SetPin(pin GPIOPin, value bool) error

	// GetPin reads the current pin state
	GetPin(pin GPIOPin) (bool, error)

	// ReadPin reads the current pin state (alias for GetPin for convenience)
	ReadPin(pin GPIOPin) bool
}

// Global singleton used by core code.
var gpioDriver GPIODriver

// SetGPIODriver is called by target-specific code to register its driver.
func SetGPIODriver(d GPIODriver) {
	gpioDriver = d
}

// MustGPIO returns the configured driver or panics if missing.
func MustGPIO() GPIODriver {
	if gpioDriver == nil {
		panic("GPIO driver not configured")
	}
	return gpioDriver
}
