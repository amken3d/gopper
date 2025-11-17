//go:build tinygo

// SPI (Serial Peripheral Interface) support
// Implements Klipper's SPI protocol for hardware and software SPI communication
package core

import (
	"gopper/protocol"
)

// SPI device flags
const (
	SF_HARDWARE       = 0x00 // Hardware SPI
	SF_SOFTWARE       = 0x01 // Software SPI (bit-banged)
	SF_CS_ACTIVE_HIGH = 0x02 // Chip select active high (default is active low)
	SF_HAVE_PIN       = 0x04 // Has chip select pin
)

// SPIDevice represents a configured SPI device
type SPIDevice struct {
	OID   uint8  // Object ID
	Flags uint8  // Device flags (hardware/software, CS polarity, etc.)
	Pin   uint32 // Chip select pin (if SF_HAVE_PIN is set)

	// Bus configuration (set by spi_set_bus)
	BusHandle interface{} // Opaque handle from ConfigureBus
	BusID     SPIBusID    // Hardware bus ID
	Mode      SPIMode     // SPI mode (0-3)
	Rate      uint32      // Clock rate in Hz

	// Shutdown safety
	ShutdownMsg []byte // Message to send on shutdown
}

// Global registry of SPI devices
var spiDevices = make(map[uint8]*SPIDevice)

// InitSPICommands registers SPI-related commands with the command registry
func InitSPICommands() {
	// Command to configure an SPI device with chip select pin
	RegisterCommand("config_spi", "oid=%c pin=%u cs_active_high=%c", handleConfigSPI)

	// Command to configure an SPI device without chip select
	RegisterCommand("config_spi_without_cs", "oid=%c", handleConfigSPIWithoutCS)

	// Command to set SPI bus parameters
	RegisterCommand("spi_set_bus", "oid=%c spi_bus=%u mode=%u rate=%u", handleSPISetBus)

	// Command to configure shutdown message for safety
	RegisterCommand("config_spi_shutdown", "oid=%c spi_oid=%c shutdown_msg=%*s", handleConfigSPIShutdown)

	// Command to send and receive SPI data
	RegisterCommand("spi_transfer", "oid=%c data=%*s", handleSPITransfer)

	// Command to send SPI data without receiving
	RegisterCommand("spi_send", "oid=%c data=%*s", handleSPISend)

	// Response message: SPI transfer response (MCU → Host)
	RegisterCommand("spi_transfer_response", "oid=%c response=%*s", nil)
}

// handleConfigSPI configures an SPI device with a chip select pin
// Format: config_spi oid=%c pin=%u cs_active_high=%c
func handleConfigSPI(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	pin, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	csActiveHigh, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Create new SPI device instance
	dev := &SPIDevice{
		OID:   uint8(oid),
		Flags: SF_HAVE_PIN,
		Pin:   pin,
	}

	// Set CS polarity flag
	if csActiveHigh != 0 {
		dev.Flags |= SF_CS_ACTIVE_HIGH
	}

	// Configure CS pin as output via GPIO HAL
	if err := MustGPIO().ConfigureOutput(GPIOPin(pin)); err != nil {
		return err
	}

	// Set CS to deasserted (inactive) state
	csInactive := true // Default: inactive high
	if csActiveHigh != 0 {
		csInactive = false // Active high means inactive is low
	}

	if err := MustGPIO().SetPin(GPIOPin(pin), csInactive); err != nil {
		return err
	}

	// Register in global map
	spiDevices[uint8(oid)] = dev

	return nil
}

// handleConfigSPIWithoutCS configures an SPI device without a chip select pin
// Format: config_spi_without_cs oid=%c
func handleConfigSPIWithoutCS(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Create new SPI device instance without CS pin
	dev := &SPIDevice{
		OID:   uint8(oid),
		Flags: 0, // No flags set
	}

	// Register in global map
	spiDevices[uint8(oid)] = dev

	return nil
}

// handleSPISetBus configures the SPI bus parameters for a device
// Format: spi_set_bus oid=%c spi_bus=%u mode=%u rate=%u
func handleSPISetBus(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	spiBus, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	mode, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	rate, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get the SPI device
	dev, exists := spiDevices[uint8(oid)]
	if !exists {
		// Invalid OID - device not configured
		return nil
	}

	// Store bus configuration
	dev.BusID = SPIBusID(spiBus)
	dev.Mode = SPIMode(mode)
	dev.Rate = rate

	// Check if this is a software SPI bus (high bit set in Klipper)
	// Software SPI uses bus IDs >= 0x80
	if spiBus >= 0x80 {
		dev.Flags |= SF_SOFTWARE
		// Software SPI configuration is deferred until first transfer
		// This matches Klipper's behavior
		return nil
	}

	// Configure hardware SPI bus via HAL
	config := SPIConfig{
		BusID: SPIBusID(spiBus),
		Mode:  SPIMode(mode),
		Rate:  rate,
	}

	busHandle, err := MustSPI().ConfigureBus(config)
	if err != nil {
		return err
	}

	dev.BusHandle = busHandle

	return nil
}

// handleConfigSPIShutdown configures a message to send on MCU shutdown
// Format: config_spi_shutdown oid=%c spi_oid=%c shutdown_msg=%*s
func handleConfigSPIShutdown(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	spiOID, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Decode shutdown message (variable length)
	msgLen := len(*data)
	shutdownMsg := make([]byte, msgLen)
	copy(shutdownMsg, *data)
	*data = (*data)[msgLen:] // Consume all remaining data

	// Get the SPI device
	dev, exists := spiDevices[uint8(spiOID)]
	if !exists {
		// Invalid OID - device not configured
		return nil
	}

	// Store shutdown message
	dev.ShutdownMsg = shutdownMsg

	// Note: oid parameter is for a shutdown object that could trigger multiple messages
	// For simplicity, we just store the message on the SPI device itself
	_ = oid

	return nil
}

// spiDeviceTransfer performs an SPI transfer with CS management
func spiDeviceTransfer(dev *SPIDevice, txData []byte, rxData []byte) error {
	// Assert chip select if device has CS pin
	if dev.Flags&SF_HAVE_PIN != 0 {
		csActive := false // Default: active low
		if dev.Flags&SF_CS_ACTIVE_HIGH != 0 {
			csActive = true // Active high
		}
		if err := MustGPIO().SetPin(GPIOPin(dev.Pin), csActive); err != nil {
			return err
		}
	}

	var err error

	// Perform transfer based on device type
	if dev.Flags&SF_SOFTWARE != 0 {
		// Software SPI transfer
		softSPI := GetSoftwareSPI()
		if softSPI == nil {
			return nil // Software SPI not available
		}
		err = softSPI.Transfer(dev.BusHandle, txData, rxData)
	} else {
		// Hardware SPI transfer
		err = MustSPI().Transfer(dev.BusHandle, txData, rxData)
	}

	// Deassert chip select if device has CS pin
	if dev.Flags&SF_HAVE_PIN != 0 {
		csInactive := true // Default: inactive high
		if dev.Flags&SF_CS_ACTIVE_HIGH != 0 {
			csInactive = false // Inactive low for active-high CS
		}
		if gpioErr := MustGPIO().SetPin(GPIOPin(dev.Pin), csInactive); gpioErr != nil {
			return gpioErr
		}
	}

	return err
}

// handleSPITransfer sends and receives SPI data
// Format: spi_transfer oid=%c data=%*s
// Response: spi_transfer_response oid=%c response=%*s
func handleSPITransfer(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Remaining data is the transfer payload
	txLen := len(*data)
	txData := make([]byte, txLen)
	copy(txData, *data)
	*data = (*data)[txLen:] // Consume all remaining data

	// Get the SPI device
	dev, exists := spiDevices[uint8(oid)]
	if !exists {
		// Invalid OID - device not configured
		return nil
	}

	// Allocate receive buffer
	rxData := make([]byte, txLen)

	// Perform SPI transfer
	if err := spiDeviceTransfer(dev, txData, rxData); err != nil {
		return err
	}

	// Send response message with received data
	SendResponse("spi_transfer_response", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(oid))
		// Encode response data as variable-length byte array
		output.Output(rxData)
	})

	return nil
}

// handleSPISend sends SPI data without receiving
// Format: spi_send oid=%c data=%*s
func handleSPISend(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Remaining data is the transfer payload
	txLen := len(*data)
	txData := make([]byte, txLen)
	copy(txData, *data)
	*data = (*data)[txLen:] // Consume all remaining data

	// Get the SPI device
	dev, exists := spiDevices[uint8(oid)]
	if !exists {
		// Invalid OID - device not configured
		return nil
	}

	// Perform SPI transfer (discard received data)
	// We still need to read during SPI transfer for proper clocking
	rxData := make([]byte, txLen)
	if err := spiDeviceTransfer(dev, txData, rxData); err != nil {
		return err
	}

	// No response message for spi_send

	return nil
}

// ShutdownSPI sends shutdown messages to all SPI devices
// This is called during MCU shutdown for safety
func ShutdownSPI() {
	for _, dev := range spiDevices {
		if dev != nil && len(dev.ShutdownMsg) > 0 {
			// Send shutdown message
			rxData := make([]byte, len(dev.ShutdownMsg))
			_ = spiDeviceTransfer(dev, dev.ShutdownMsg, rxData)
		}
	}
}
