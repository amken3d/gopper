//go:build tinygo

package core

// I2CBusID identifies a specific I2C bus (e.g., I2C0, I2C1).
type I2CBusID uint8

// I2CAddress is a 7-bit I2C device address.
type I2CAddress uint8

// I2CDriver is the abstract I2C interface that core code uses.
type I2CDriver interface {
	// ConfigureBus initializes a specific I2C bus with the given frequency.
	// Returns error if bus ID is invalid or configuration fails.
	ConfigureBus(bus I2CBusID, frequencyHz uint32) error

	// Write transmits data to a device at the given address on the specified bus.
	// This is a simple write operation (equivalent to i2c_dev_write in Klipper).
	Write(bus I2CBusID, addr I2CAddress, data []byte) error

	// Read reads data from a device, optionally writing a register address first.
	// If regData is non-empty, it's transmitted before the read (restart in between).
	// This matches Klipper's i2c_dev_read behavior.
	Read(bus I2CBusID, addr I2CAddress, regData []byte, readLen uint8) ([]byte, error)

	// GetMachineBus returns the underlying machine.I2C instance for a bus.
	// This allows direct use of TinyGo drivers that expect machine.I2C.
	// Returns nil if the bus is not configured or doesn't support machine.I2C.
	GetMachineBus(bus I2CBusID) (interface{}, error)
}

// Global singleton used by core code.
var i2cDriver I2CDriver

// SetI2CDriver is called by target-specific code to register its driver.
func SetI2CDriver(d I2CDriver) {
	i2cDriver = d
}

// MustI2C returns the configured driver or panics if missing.
func MustI2C() I2CDriver {
	if i2cDriver == nil {
		panic("I2C driver not configured")
	}
	return i2cDriver
}
