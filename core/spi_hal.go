//go:build tinygo

package core

// SPIBusID identifies a hardware SPI bus configuration
type SPIBusID uint8

// SPIMode represents SPI clock polarity and phase (0-3)
// Mode 0: CPOL=0, CPHA=0 (clock idle low, sample on rising edge)
// Mode 1: CPOL=0, CPHA=1 (clock idle low, sample on falling edge)
// Mode 2: CPOL=1, CPHA=0 (clock idle high, sample on falling edge)
// Mode 3: CPOL=1, CPHA=1 (clock idle high, sample on rising edge)
type SPIMode uint8

// SPIConfig holds the configuration for an SPI bus
type SPIConfig struct {
	BusID SPIBusID // Hardware bus identifier
	Mode  SPIMode  // SPI mode (0-3)
	Rate  uint32   // Clock rate in Hz
}

// SPIDriver is the abstract SPI interface that core code uses.
// Platform-specific implementations handle actual hardware control.
type SPIDriver interface {
	// ConfigureBus sets up a hardware SPI bus with specified parameters
	// Returns an opaque bus handle and any error
	ConfigureBus(config SPIConfig) (interface{}, error)

	// Transfer performs a bidirectional SPI transfer
	// Sends txData and receives rxData simultaneously
	// The busHandle is the value returned by ConfigureBus
	Transfer(busHandle interface{}, txData []byte, rxData []byte) error

	// GetBusInfo returns information about available SPI buses
	// Returns a map of bus IDs to human-readable descriptions
	GetBusInfo() map[SPIBusID]string
}

// SoftwareSPIDriver is the interface for software (bit-banged) SPI
// This is separate from hardware SPI and used as a fallback
type SoftwareSPIDriver interface {
	// ConfigureSoftwareSPI sets up GPIO pins for software SPI
	// sclk: clock pin, mosi: master out slave in, miso: master in slave out
	ConfigureSoftwareSPI(sclk, mosi, miso uint32, mode SPIMode, rate uint32) (interface{}, error)

	// Transfer performs a software SPI transfer
	Transfer(handle interface{}, txData []byte, rxData []byte) error
}

// Global singletons used by core code
var (
	spiDriver         SPIDriver
	softwareSPIDriver SoftwareSPIDriver
)

// SetSPIDriver is called by target-specific code to register its hardware SPI driver
func SetSPIDriver(d SPIDriver) {
	spiDriver = d
}

// SetSoftwareSPIDriver is called by target-specific code to register its software SPI driver
func SetSoftwareSPIDriver(d SoftwareSPIDriver) {
	softwareSPIDriver = d
}

// MustSPI returns the configured hardware SPI driver or panics if missing
func MustSPI() SPIDriver {
	if spiDriver == nil {
		panic("SPI driver not configured")
	}
	return spiDriver
}

// GetSoftwareSPI returns the software SPI driver or nil if not available
func GetSoftwareSPI() SoftwareSPIDriver {
	return softwareSPIDriver
}
