//go:build tinygo

package core

import (
	"gopper/protocol"
	"sync/atomic"
	"unsafe"
)

// FirmwareState holds the global firmware state
type FirmwareState struct {
	configCRC  uint32 // atomic
	isShutdown uint32 // atomic bool
	moveCount  uint16
}

var globalState = &FirmwareState{
	moveCount: 16, // Command queue size - minimum for Klipper
}

// InitCoreCommands registers all core protocol commands
// IMPORTANT: Command registration order matters!
// Klipper has a hardcoded bootstrap dictionary:
//
//	identify_response = ID 0
//	identify = ID 1
func InitCoreCommands() {
	// Bootstrap messages - MUST be first to match Klipper's DefaultMessages
	RegisterCommand("identify_response", "offset=%u data=%*s", nil)   // ID 0
	RegisterCommand("identify", "offset=%u count=%c", handleIdentify) // ID 1

	// Other commands (order doesn't matter after bootstrap)
	RegisterCommand("get_uptime", "", handleGetUptime)
	RegisterCommand("get_clock", "", handleGetClock)
	RegisterCommand("get_config", "", handleGetConfig)
	RegisterCommand("config_reset", "", handleConfigReset)
	RegisterCommand("finalize_config", "crc=%u", handleFinalizeConfig)
	RegisterCommand("allocate_oids", "count=%c", handleAllocateOids)
	RegisterCommand("emergency_stop", "", handleEmergencyStop)
	RegisterCommand("reset", "", handleReset)

	// Debug commands
	RegisterCommand("debug_read", "order=%c addr=%u", handleDebugRead)
	RegisterCommand("debug_result", "val=%u", nil)

	// Response messages (MCU → Host)
	RegisterCommand("clock", "clock=%u", nil)
	RegisterCommand("uptime", "high=%u clock=%u", nil)
	RegisterCommand("config", "is_config=%c crc=%u is_shutdown=%c move_count=%hu", nil)

	// Register common constants
	// Note: MCU and CLOCK_FREQ are platform-specific and registered in target/*/clock.go
	RegisterConstant("STATS_SUMSQ_BASE", uint32(256))
}

// handleIdentify returns chunks of the data dictionary
func handleIdentify(data *[]byte) error {
	// Decode arguments: offset (uint32), count (uint8)
	offset, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	count8, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}
	count := uint8(count8)

	// Get dictionary chunk
	chunk := GetGlobalDictionary().GetChunk(offset, count)

	// Send identify_response
	SendResponse("identify_response", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, offset)
		protocol.EncodeVLQBytes(output, chunk)
	})

	return nil
}

// handleGetUptime returns the system uptime
func handleGetUptime(data *[]byte) error {
	// Get 64-bit uptime
	uptime := GetUptime()
	high := uint32(uptime >> 32)
	low := uint32(uptime & 0xFFFFFFFF)

	SendResponse("uptime", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, high)
		protocol.EncodeVLQUint(output, low)
	})

	return nil
}

// handleGetClock returns the current clock value
func handleGetClock(data *[]byte) error {
	clock := GetTime()

	SendResponse("clock", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, clock)
	})

	return nil
}

// handleGetConfig returns the configuration state
func handleGetConfig(data *[]byte) error {
	crc := atomic.LoadUint32(&globalState.configCRC)
	isShutdown := atomic.LoadUint32(&globalState.isShutdown) != 0
	isConfig := crc != 0

	SendResponse("config", func(output protocol.OutputBuffer) {
		// is_config (bool)
		if isConfig {
			protocol.EncodeVLQUint(output, 1)
		} else {
			protocol.EncodeVLQUint(output, 0)
		}
		// crc (uint32)
		protocol.EncodeVLQUint(output, crc)
		// is_shutdown (bool)
		if isShutdown {
			protocol.EncodeVLQUint(output, 1)
		} else {
			protocol.EncodeVLQUint(output, 0)
		}
		// move_count (uint16)
		protocol.EncodeVLQUint(output, uint32(globalState.moveCount))
	})

	return nil
}

// handleConfigReset resets the configuration state
func handleConfigReset(data *[]byte) error {
	atomic.StoreUint32(&globalState.configCRC, 0)
	return nil
}

// handleFinalizeConfig finalizes the configuration with a CRC
func handleFinalizeConfig(data *[]byte) error {
	crc, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	atomic.StoreUint32(&globalState.configCRC, crc)
	return nil
}

// handleAllocateOids allocates object IDs (currently a no-op)
func handleAllocateOids(data *[]byte) error {
	count, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}
	_ = count // Currently unused
	return nil
}

// handleEmergencyStop triggers an emergency stop
func handleEmergencyStop(data *[]byte) error {
	atomic.StoreUint32(&globalState.isShutdown, 1)
	// Stop ADC sampling and other safety‑critical activity.
	ShutdownAllAnalogIn()
	// Return all GPIO pins to default state
	ShutdownAllDigitalOut()
	// Stop all I2C operations
	ShutdownAllI2C()
	// Send shutdown messages to SPI devices
	ShutdownSPI()
	// TODO: Implement additional emergency stop behavior:
	// - Stop all timers
	// - Disable all outputs
	// - Set steppers to idle
	return nil
}

// TryShutdown triggers a firmware shutdown with a reason message
// This is used by safety mechanisms like ADC range checking
func TryShutdown(reason string) {
	atomic.StoreUint32(&globalState.isShutdown, 1)
	// Stop ADC sampling to prevent further activity after shutdown.
	ShutdownAllAnalogIn()
	// Return all GPIO pins to default state
	ShutdownAllDigitalOut()
	// Stop all I2C operations
	ShutdownAllI2C()
	// TODO: Send shutdown message to host with reason
	// For now, just set the shutdown flag
	_ = reason
}

// IsShutdown returns true if the firmware is in shutdown state
func IsShutdown() bool {
	return atomic.LoadUint32(&globalState.isShutdown) != 0
}

// ResetFirmwareState resets the firmware state for reconnection
// This is called when USB reconnects or firmware restart is requested
func ResetFirmwareState() {
	atomic.StoreUint32(&globalState.configCRC, 0)
	atomic.StoreUint32(&globalState.isShutdown, 0)
	// moveCount is not reset - it's a firmware constant
}

// SendResponse sends a response message using the global transport
func SendResponse(responseName string, args func(output protocol.OutputBuffer)) {
	if globalTransport != nil {
		// Look up response command ID
		cmd, ok := globalRegistry.GetCommandByName(responseName)
		if !ok {
			// Response not found - this is an error, all responses should be pre-registered
			panic("Response not registered: " + responseName)
		}

		globalTransport.SendCommand(cmd.ID, args)
	}
}

// GetCommandByName retrieves a command by name
func (r *CommandRegistry) GetCommandByName(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.nameToID[name]
	if !ok {
		return nil, false
	}
	return r.commands[id], true
}

// Global transport for sending responses (set by main)
var globalTransport *protocol.Transport

// SetGlobalTransport sets the global transport for sending responses
func SetGlobalTransport(transport *protocol.Transport) {
	globalTransport = transport
}

// Global reset handler (set by target-specific code)
var globalResetHandler func()

// resetPending is set when a reset command is received
// The actual reset happens in the main loop after ACK is sent
var resetPending uint32 // atomic bool

// SetResetHandler sets the platform-specific reset handler
func SetResetHandler(handler func()) {
	globalResetHandler = handler
}

// handleDebugRead reads a value from a memory address
// This is used by Klipper's temperature_mcu to read calibration values
// Format: debug_read order=%c addr=%u
//
//	order: 1 = read 16-bit (uint16), 2 = read 32-bit (uint32)
//	addr: memory address to read from
//
// Response: debug_result val=%u
func handleDebugRead(data *[]byte) error {
	// Decode arguments: order (uint8), addr (uint32)
	order, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	addr, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Read value from memory address based on order
	var val uint32
	switch order {
	case 1: // 16-bit read
		ptr := (*uint16)(unsafe.Pointer(uintptr(addr)))
		val = uint32(*ptr)
	case 2: // 32-bit read
		ptr := (*uint32)(unsafe.Pointer(uintptr(addr)))
		val = *ptr
	default:
		// Unknown order, return 0
		val = 0
	}

	// Send debug_result response
	SendResponse("debug_result", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, val)
	})

	return nil
}

// handleReset triggers a hardware reset of the MCU
// This is used by Klipper's FIRMWARE_RESTART command
// NOTE: The actual reset is deferred until after the ACK is sent to the host
func handleReset(_ *[]byte) error {
	// Set flag to trigger reset in main loop
	// Don't reset immediately - we need to send ACK first!
	atomic.StoreUint32(&resetPending, 1)
	return nil
}

// CheckPendingReset checks if a reset was requested and executes it
// This should be called from the main loop after all pending messages are sent
func CheckPendingReset() {
	if atomic.LoadUint32(&resetPending) != 0 {
		// Trigger the reset immediately
		// The reset handler (watchdog) has its own built-in delay
		if globalResetHandler != nil {
			globalResetHandler()
			// Should never return - reset handler should reset the MCU
		}
	}
}
