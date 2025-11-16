package core

//
//import (
//	"log"
//	"strings"
//	"testing"
//)
//
//func TestDictionary(t *testing.T) {
//	dict := NewDictionary(NewCommandRegistry())
//	log.Printf("Dictionary: %s", dict.Generate())
//
//	// Add test constants
//	dict.AddConstant("TEST_CONST", uint32(42))
//	dict.AddConstant("TEST_STR", "hello")
//
//	// Add test enumeration
//	dict.AddEnumeration("test_pins", []string{"PA0", "PA1", "PB0"})
//
//	// Register test command
//	dict.commandReg.Register("test_cmd", "arg=%u", func(data *[]byte) error {
//		return nil
//	})
//
//	// Generate dictionary
//	output := string(dict.Generate())
//
//	t.Log("Generated dictionary:\n" + output)
//
//	// Verify version present (JSON format)
//	if !strings.Contains(output, `"version":"gopper-0.1.0"`) {
//		t.Error("Dictionary missing version")
//	}
//
//	// Verify constants present (JSON format)
//	if !strings.Contains(output, `"TEST_CONST":"42"`) {
//		t.Error("Dictionary missing TEST_CONST")
//	}
//	if !strings.Contains(output, `"TEST_STR":"hello"`) {
//		t.Error("Dictionary missing TEST_STR")
//	}
//
//	// Verify enumeration present (JSON format)
//	if !strings.Contains(output, `"test_pins"`) {
//		t.Error("Dictionary missing test_pins enumeration")
//	}
//	if !strings.Contains(output, `"PA0":0`) && !strings.Contains(output, `"PA1":1`) {
//		t.Error("Dictionary missing test_pins values")
//	}
//
//	// Verify command present (JSON format)
//	if !strings.Contains(output, `"test_cmd arg=%u"`) {
//		t.Error("Dictionary missing test_cmd")
//	}
//}
//
//func TestDictionaryChunks(t *testing.T) {
//	dict := NewDictionary(NewCommandRegistry())
//	dict.AddConstant("TEST", uint32(123))
//
//	// Generate full dictionary
//	full := dict.Generate()
//
//	// Test getting chunks
//	chunk1 := dict.GetChunk(0, 10)
//	if len(chunk1) == 0 {
//		t.Error("First chunk is empty")
//	}
//	if len(chunk1) > 10 {
//		t.Errorf("First chunk too large: %d bytes", len(chunk1))
//	}
//
//	// Test offset beyond end
//	chunkEnd := dict.GetChunk(uint32(len(full)+100), 10)
//	if len(chunkEnd) != 0 {
//		t.Error("Chunk beyond end should be empty")
//	}
//
//	// Test chunk at exact end
//	chunkAtEnd := dict.GetChunk(uint32(len(full)), 10)
//	if len(chunkAtEnd) != 0 {
//		t.Error("Chunk at end should be empty")
//	}
//}
//
//func TestInitCoreCommands(t *testing.T) {
//	// Create a new registry for testing
//	oldRegistry := globalRegistry
//	log.Printf("Old registry: %p", oldRegistry)
//	globalRegistry = NewCommandRegistry()
//	log.Printf("New registry: %p", globalRegistry)
//	defer func() { globalRegistry = oldRegistry }()
//
//	// Initialize core commands
//	InitCoreCommands()
//	log.Printf("New registry: %p", globalRegistry)
//
//	// Verify commands are registered
//	requiredCommands := []string{
//		"identify",
//		"get_uptime",
//		"get_clock",
//		"get_config",
//		"config_reset",
//		"finalize_config",
//		"allocate_oids",
//		"emergency_stop",
//	}
//
//	for _, cmdName := range requiredCommands {
//		cmd, ok := globalRegistry.GetCommandByName(cmdName)
//		if !ok {
//			t.Errorf("Required command not registered: %s", cmdName)
//		}
//		if cmd == nil {
//			t.Errorf("Command %s is nil", cmdName)
//		}
//	}
//
//	// Verify constants are registered (JSON format)
//	dict := GetGlobalDictionary().Generate()
//	dictStr := string(dict)
//
//	// Check for core constants (platform-specific constants like MCU and CLOCK_FREQ
//	// are registered in target packages, not in InitCoreCommands)
//	if !strings.Contains(dictStr, `"STATS_SUMSQ_BASE"`) {
//		t.Error("STATS_SUMSQ_BASE constant not registered")
//	}
//
//	// ADC_MAX is registered by InitADCCommands which is called in InitCoreCommands
//	if !strings.Contains(dictStr, `"ADC_MAX"`) {
//		t.Error("ADC_MAX constant not registered")
//	}
//
//	t.Logf("Dictionary:\n%s", dictStr)
//}
