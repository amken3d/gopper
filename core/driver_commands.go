//go:build tinygo

package core

import (
	"gopper/protocol"
)

// InitDriverCommands registers the standard driver-related Klipper commands.
// This should be called during firmware initialization.
func InitDriverCommands() {
	// Command to configure a registered driver
	RegisterCommand("config_driver", "oid=%c", handleConfigDriver)

	// Command to read data from a driver
	RegisterCommand("driver_read", "oid=%c params=%*s", handleDriverRead)

	// Command to write data to a driver
	RegisterCommand("driver_write", "oid=%c data=%*s", handleDriverWrite)

	// Command to start polling for a driver
	RegisterCommand("driver_start_poll", "oid=%c poll_ticks=%u", handleDriverStartPoll)

	// Command to stop polling for a driver
	RegisterCommand("driver_stop_poll", "oid=%c", handleDriverStopPoll)

	// Command to query driver state
	RegisterCommand("driver_query_state", "oid=%c", handleDriverQueryState)

	// Command to unregister a driver
	RegisterCommand("driver_unregister", "oid=%c", handleDriverUnregister)

	// Response messages
	RegisterResponse("driver_data", "oid=%c data=%*s")
	RegisterResponse("driver_state", "oid=%c configured=%c active=%c error_code=%c")
	RegisterResponse("driver_poll_data", "oid=%c data=%*s")
}

// handleConfigDriver configures a registered driver
// Format: config_driver oid=%c
func handleConfigDriver(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get driver instance
	instance, exists := GetDriver(uint8(oid))
	if !exists {
		return nil // Silently ignore if driver not registered
	}

	// Skip if already configured
	if instance.State.Configured {
		return nil
	}

	// Call configure function if provided
	if instance.Config.ConfigureFunc != nil {
		if err := instance.Config.ConfigureFunc(instance.Device, instance.Config); err != nil {
			instance.State.LastError = err
			return err
		}
	}

	instance.State.Configured = true
	return nil
}

// handleDriverRead reads data from a driver
// Format: driver_read oid=%c params=%*s
func handleDriverRead(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get driver instance
	instance, exists := GetDriver(uint8(oid))
	if !exists {
		return nil // Silently ignore if driver not registered
	}

	// Check if driver supports reading
	if instance.Config.ReadFunc == nil {
		return nil // Silently ignore if read not supported
	}

	// Decode parameters (remaining bytes)
	params := *data
	*data = (*data)[len(*data):] // Consume all remaining bytes

	// Read from driver
	readData, err := instance.Config.ReadFunc(instance.Device, params)
	if err != nil {
		instance.State.LastError = err
		return err
	}

	// Send response
	SendResponse("driver_data", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(oid))
		for _, b := range readData {
			protocol.EncodeVLQUint(output, uint32(b))
		}
	})

	return nil
}

// handleDriverWrite writes data to a driver
// Format: driver_write oid=%c data=%*s
func handleDriverWrite(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get driver instance
	instance, exists := GetDriver(uint8(oid))
	if !exists {
		return nil // Silently ignore if driver not registered
	}

	// Check if driver supports writing
	if instance.Config.WriteFunc == nil {
		return nil // Silently ignore if write not supported
	}

	// Decode data (remaining bytes)
	writeData := *data
	*data = (*data)[len(*data):] // Consume all remaining bytes

	// Write to driver
	if err := instance.Config.WriteFunc(instance.Device, writeData); err != nil {
		instance.State.LastError = err
		return err
	}

	return nil
}

// handleDriverStartPoll starts periodic polling for a driver
// Format: driver_start_poll oid=%c poll_ticks=%u
func handleDriverStartPoll(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	pollTicks, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get driver instance
	instance, exists := GetDriver(uint8(oid))
	if !exists {
		return nil // Silently ignore if driver not registered
	}

	// Start polling
	if err := StartPolling(instance, pollTicks); err != nil {
		instance.State.LastError = err
		return err
	}

	return nil
}

// handleDriverStopPoll stops periodic polling for a driver
// Format: driver_stop_poll oid=%c
func handleDriverStopPoll(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get driver instance
	instance, exists := GetDriver(uint8(oid))
	if !exists {
		return nil // Silently ignore if driver not registered
	}

	// Stop polling
	StopPolling(instance)

	return nil
}

// handleDriverQueryState queries the state of a driver
// Format: driver_query_state oid=%c
func handleDriverQueryState(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get driver instance
	instance, exists := GetDriver(uint8(oid))
	if !exists {
		return nil // Silently ignore if driver not registered
	}

	// Get state
	configured := uint32(0)
	if instance.State.Configured {
		configured = 1
	}

	active := uint32(0)
	if instance.State.Active {
		active = 1
	}

	errorCode := uint32(0)
	if instance.State.LastError != nil {
		errorCode = 1 // Simple error flag
	}

	// Send response
	SendResponse("driver_state", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(oid))
		protocol.EncodeVLQUint(output, configured)
		protocol.EncodeVLQUint(output, active)
		protocol.EncodeVLQUint(output, errorCode)
	})
	return nil
}

// handleDriverUnregister unregisters a driver
// Format: driver_unregister oid=%c
func handleDriverUnregister(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Unregister driver
	return UnregisterDriver(uint8(oid))
}
