//go:build tinygo

package core

// PWMPin identifies a hardware pin capable of PWM output
type PWMPin uint32

// PWMValue is the duty cycle value (0 to PWM_MAX)
type PWMValue uint32

// PWMDriver is the abstract PWM interface that core code uses.
// Platform-specific implementations handle actual hardware control.
type PWMDriver interface {
	// ConfigureHardwarePWM configures a pin for hardware PWM output
	// cycleTicks: PWM period in timer ticks
	// Returns the actual cycle ticks used (may be adjusted for hardware constraints)
	ConfigureHardwarePWM(pin PWMPin, cycleTicks uint32) (uint32, error)

	// SetDutyCycle sets the PWM duty cycle for a pin
	// value: 0 (fully off) to GetMaxValue() (fully on)
	SetDutyCycle(pin PWMPin, value PWMValue) error

	// GetMaxValue returns the maximum PWM value (e.g., 255 for 8-bit)
	// This matches Klipper's PWM_MAX constant
	GetMaxValue() uint32

	// DisablePWM disables PWM on a pin and returns it to GPIO mode
	DisablePWM(pin PWMPin) error
}

// Global singleton used by core code.
var pwmDriver PWMDriver

// SetPWMDriver is called by target-specific code to register its driver.
func SetPWMDriver(d PWMDriver) {
	pwmDriver = d
}

// MustPWM returns the configured driver or panics if missing.
func MustPWM() PWMDriver {
	if pwmDriver == nil {
		panic("PWM driver not configured")
	}
	return pwmDriver
}
