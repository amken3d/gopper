package core

import (
	"gopper/protocol"
	"testing"
)

func TestCommandRegistry(t *testing.T) {
	registry := NewCommandRegistry()

	// Register a command
	var called bool
	handler := func(data *[]byte) error {
		called = true
		return nil
	}

	id := registry.Register("test_command", "arg=%u", handler)

	if id != 0 {
		t.Errorf("Expected first command to have ID 0, got %d", id)
	}

	// Verify command can be retrieved
	cmd, ok := registry.GetCommand(id)
	if !ok {
		t.Error("Failed to retrieve registered command")
	}

	if cmd.Name != "test_command" {
		t.Errorf("Expected command name 'test_command', got '%s'", cmd.Name)
	}

	// Test dispatch
	var data []byte
	err := registry.Dispatch(id, &data)
	if err != nil {
		t.Errorf("Dispatch failed: %v", err)
	}

	if !called {
		t.Error("Command handler was not called")
	}

	// Test unknown command
	err = registry.Dispatch(999, &data)
	if err == nil {
		t.Error("Expected error for unknown command ID")
	}
}

func TestCommandRegistryMultiple(t *testing.T) {
	registry := NewCommandRegistry()

	id1 := registry.Register("command1", "arg1=%u", func(data *[]byte) error { return nil })
	id2 := registry.Register("command2", "arg2=%u", func(data *[]byte) error { return nil })
	id3 := registry.Register("command3", "arg3=%u", func(data *[]byte) error { return nil })

	if id1 != 0 || id2 != 1 || id3 != 2 {
		t.Errorf("Command IDs not sequential: %d, %d, %d", id1, id2, id3)
	}

	// Verify all commands exist
	for i := uint16(0); i < 3; i++ {
		if _, ok := registry.GetCommand(i); !ok {
			t.Errorf("Command %d not found", i)
		}
	}
}

func TestCommandRegistryDictionary(t *testing.T) {
	registry := NewCommandRegistry()

	registry.Register("get_uptime", "", func(data *[]byte) error { return nil })
	registry.Register("get_config", "", func(data *[]byte) error { return nil })

	dict := registry.GetDictionary()

	if dict == "" {
		t.Error("Dictionary is empty")
	}

	t.Logf("Dictionary:\n%s", dict)
}

func TestCommandWithArguments(t *testing.T) {
	registry := NewCommandRegistry()

	var receivedValue uint32

	handler := func(data *[]byte) error {
		val, err := protocol.DecodeVLQUint(data)
		if err != nil {
			return err
		}
		receivedValue = val
		return nil
	}

	id := registry.Register("test_args", "value=%u", handler)

	// Create test data
	output := protocol.NewScratchOutput()
	protocol.EncodeVLQUint(output, 12345)
	data := output.Result()

	err := registry.Dispatch(id, &data)
	if err != nil {
		t.Errorf("Dispatch failed: %v", err)
	}

	if receivedValue != 12345 {
		t.Errorf("Expected value 12345, got %d", receivedValue)
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Test the global registry functions
	RegisterCommand("global_test", "arg=%u", func(data *[]byte) error {
		return nil
	})

	dict := GetGlobalRegistry().GetDictionary()
	if dict == "" {
		t.Error("Global registry dictionary is empty")
	}
}
