package core

import (
	"gopper/protocol"
	"sync/atomic"
)

// FirmwareState holds the global firmware state
type FirmwareState struct {
	configCRC  uint32 // atomic
	isShutdown uint32 // atomic bool
	moveCount  uint16
}

var globalState = &FirmwareState{}

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
	// TODO: Implement actual emergency stop behavior
	// - Stop all timers
	// - Disable all outputs
	// - Set steppers to idle
	return nil
}

// ResetFirmwareState resets the firmware state for reconnection
// This is called when USB reconnects or firmware restart is requested
func ResetFirmwareState() {
	atomic.StoreUint32(&globalState.configCRC, 0)
	atomic.StoreUint32(&globalState.isShutdown, 0)
	globalState.moveCount = 0
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

// SetResetHandler sets the platform-specific reset handler
func SetResetHandler(handler func()) {
	globalResetHandler = handler
}

// handleReset triggers a hardware reset of the MCU
// This is used by Klipper's FIRMWARE_RESTART command
func handleReset(data *[]byte) error {
	if globalResetHandler != nil {
		globalResetHandler()
		// Should never return - reset handler should reset the MCU
	}
	return nil
}
