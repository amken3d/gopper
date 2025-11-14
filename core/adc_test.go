package core

import (
	"gopper/protocol"
	"testing"
)

// Mock ADC functions for testing
func setupMockADC() {
	ADCSetup = func(pin uint32) error {
		// Mock implementation - always succeeds
		return nil
	}

	ADCSample = func(pin uint32) (uint16, bool) {
		// Mock implementation - returns a fixed value
		return 2048, true // Mid-range 12-bit value
	}

	ADCCancel = func(pin uint32) {
		// Mock implementation - no-op
	}
}

func TestADCCommandRegistration(t *testing.T) {
	// Reset registry for clean test
	globalRegistry = NewCommandRegistry()

	// Register ADC commands
	InitADCCommands()

	// Verify commands were registered
	commands := []string{"config_analog_in", "query_analog_in", "analog_in_state"}
	for _, cmdName := range commands {
		cmd, ok := globalRegistry.GetCommandByName(cmdName)
		if !ok {
			t.Errorf("Command %s not registered", cmdName)
		} else {
			t.Logf("Command %s registered with ID %d", cmdName, cmd.ID)
		}
	}
}

func TestConfigAnalogIn(t *testing.T) {
	// Setup mock ADC
	setupMockADC()

	// Reset registry and register commands
	globalRegistry = NewCommandRegistry()
	InitADCCommands()

	// Create command data: oid=0, pin=26
	output := protocol.NewScratchOutput()
	protocol.EncodeVLQUint(output, 0)  // oid
	protocol.EncodeVLQUint(output, 26) // pin (GPIO 26 = ADC0 on RP2040)

	data := output.Result()

	// Execute command
	err := handleConfigAnalogIn(&data)
	if err != nil {
		t.Errorf("handleConfigAnalogIn failed: %v", err)
	}

	// Verify analog input was created
	ain, exists := analogInputs[0]
	if !exists {
		t.Error("AnalogIn not created")
	} else {
		if ain.OID != 0 {
			t.Errorf("Expected OID 0, got %d", ain.OID)
		}
		if ain.Pin != 26 {
			t.Errorf("Expected pin 26, got %d", ain.Pin)
		}
		if ain.State != ADCStateReady {
			t.Errorf("Expected state ADCStateReady, got %d", ain.State)
		}
		t.Logf("AnalogIn configured: OID=%d, Pin=%d, State=%d", ain.OID, ain.Pin, ain.State)
	}
}

func TestQueryAnalogIn(t *testing.T) {
	// Setup mock ADC
	setupMockADC()

	// Reset registry and register commands
	globalRegistry = NewCommandRegistry()
	InitADCCommands()

	// First configure an analog input
	configData := protocol.NewScratchOutput()
	protocol.EncodeVLQUint(configData, 0)  // oid
	protocol.EncodeVLQUint(configData, 26) // pin
	data := configData.Result()
	handleConfigAnalogIn(&data)

	// Now query it
	queryData := protocol.NewScratchOutput()
	protocol.EncodeVLQUint(queryData, 0)     // oid
	protocol.EncodeVLQUint(queryData, 1000)  // clock (start time)
	protocol.EncodeVLQUint(queryData, 100)   // sample_ticks
	protocol.EncodeVLQUint(queryData, 4)     // sample_count (oversample 4x)
	protocol.EncodeVLQUint(queryData, 10000) // rest_ticks
	protocol.EncodeVLQUint(queryData, 1000)  // min_value
	protocol.EncodeVLQUint(queryData, 3000)  // max_value
	protocol.EncodeVLQUint(queryData, 3)     // range_check_count

	data = queryData.Result()
	err := handleQueryAnalogIn(&data)
	if err != nil {
		t.Errorf("handleQueryAnalogIn failed: %v", err)
	}

	// Verify analog input parameters
	ain := analogInputs[0]
	if ain.SampleTime != 100 {
		t.Errorf("Expected SampleTime 100, got %d", ain.SampleTime)
	}
	if ain.SampleCount != 4 {
		t.Errorf("Expected SampleCount 4, got %d", ain.SampleCount)
	}
	if ain.RestTime != 10000 {
		t.Errorf("Expected RestTime 10000, got %d", ain.RestTime)
	}
	if ain.MinValue != 1000 {
		t.Errorf("Expected MinValue 1000, got %d", ain.MinValue)
	}
	if ain.MaxValue != 3000 {
		t.Errorf("Expected MaxValue 3000, got %d", ain.MaxValue)
	}
	if ain.RangeCheckCount != 3 {
		t.Errorf("Expected RangeCheckCount 3, got %d", ain.RangeCheckCount)
	}
	if ain.State != ADCStateSampling {
		t.Errorf("Expected state ADCStateSampling, got %d", ain.State)
	}

	t.Logf("AnalogIn query configured successfully")
}

func TestADCVLQEncoding(t *testing.T) {
	// Test VLQ encoding/decoding for ADC command parameters
	testCases := []struct {
		name  string
		value uint32
	}{
		{"small value", 0},
		{"pin number", 26},
		{"sample ticks", 100},
		{"rest ticks", 10000},
		{"min value", 1000},
		{"max value", 3000},
		{"ADC max", 4095},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode
			output := protocol.NewScratchOutput()
			protocol.EncodeVLQUint(output, tc.value)
			encoded := output.Result()

			// Decode
			decoded, err := protocol.DecodeVLQUint(&encoded)
			if err != nil {
				t.Errorf("Failed to decode %s: %v", tc.name, err)
			}

			if decoded != tc.value {
				t.Errorf("Value mismatch for %s: expected %d, got %d", tc.name, tc.value, decoded)
			}

			t.Logf("%s: %d -> %d bytes -> %d", tc.name, tc.value, len(output.Result()), decoded)
		})
	}
}
