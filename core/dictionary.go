package core

import (
	"bytes"
	"machine"
	"sync"
	"time"

	"gopper/tinycompress"
)

// Constant represents a firmware constant exposed to the host
type Constant struct {
	Name  string
	Value interface{} // Can be string, int, etc.
}

// Enumeration represents an enumeration of values (like pin names)
type Enumeration struct {
	Name   string
	Values []string
}

// Dictionary manages the data dictionary sent to Klipper host
type Dictionary struct {
	mu            sync.RWMutex
	constants     map[string]*Constant
	enumerations  map[string]*Enumeration
	commandReg    *CommandRegistry
	version       string
	buildVersions string
	clk_freq      uint32
	mcu           string
	cachedDict    []byte // Cached compressed dictionary
}

var globalDictionary = NewDictionary(globalRegistry)

// NewDictionary creates a new dictionary
func NewDictionary(cmdReg *CommandRegistry) *Dictionary {
	return &Dictionary{
		constants:     make(map[string]*Constant),
		enumerations:  make(map[string]*Enumeration),
		commandReg:    cmdReg,
		version:       "gopper-0.1.0",
		buildVersions: "go-tinygo",
		clk_freq:      1000000,
		mcu:           "rp2040",
	}
}

// ledBlink blinks the LED a specific number of times for diagnostics
func ledBlink(count int) {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for i := 0; i < count; i++ {
		led.High()
		time.Sleep(20 * time.Millisecond)
		led.Low()
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // Pause after blink sequence
}

// RegisterConstant registers a constant in the dictionary
func RegisterConstant(name string, value interface{}) {
	globalDictionary.AddConstant(name, value)
}

// RegisterEnumeration registers an enumeration in the dictionary
func RegisterEnumeration(name string, values []string) {
	globalDictionary.AddEnumeration(name, values)
}

// AddConstant adds a constant to the dictionary
func (d *Dictionary) AddConstant(name string, value interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.constants[name] = &Constant{
		Name:  name,
		Value: value,
	}
}

// AddEnumeration adds an enumeration to the dictionary
func (d *Dictionary) AddEnumeration(name string, values []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// CRITICAL: Copy the values slice to prevent TinyGo GC from reclaiming it
	// TinyGo's GC can be aggressive and may reclaim the original slice after
	// the caller's function returns, leading to memory corruption
	valuesCopy := make([]string, len(values))
	copy(valuesCopy, values)

	d.enumerations[name] = &Enumeration{
		Name:   name,
		Values: valuesCopy,
	}
}

// SetVersion sets the firmware version string
func (d *Dictionary) SetVersion(version string) {
	d.mu.Lock()
	ledBlink(1)
	defer d.mu.Unlock()
	d.version = version
}

// SetBuildVersions sets the build versions string
func (d *Dictionary) SetBuildVersions(versions string) {
	d.mu.Lock()
	ledBlink(2)
	defer d.mu.Unlock()
	d.buildVersions = versions
}

// BuildDictionary builds and caches the dictionary (call after all commands registered)
func (d *Dictionary) BuildDictionary() {
	ledBlink(3)
	DebugPrintln("[BuildDict] Starting BuildDictionary")

	// CRITICAL: Get commands/responses BEFORE acquiring dictionary lock
	// This prevents deadlock with cores scheduler where:
	// - Core 0: Dictionary lock → CommandRegistry lock
	// - Core 1: CommandRegistry lock → Dictionary lock
	DebugPrintln("[BuildDict] Calling GetCommandsAndResponses...")
	commands, responses := d.commandReg.GetCommandsAndResponses()
	DebugPrintln("[BuildDict] Got " + itoa(len(commands)) + " commands, " + itoa(len(responses)) + " responses")

	DebugPrintln("[BuildDict] Acquiring dictionary lock...")
	d.mu.Lock()
	defer d.mu.Unlock()
	DebugPrintln("[BuildDict] Dictionary lock acquired")

	// Generate uncompressed JSON (passing pre-fetched commands/responses)
	DebugPrintln("[BuildDict] Building JSON...")
	jsonData := d.buildJSONLockedWithData(commands, responses)
	DebugPrintln("[BuildDict] JSON built, size: " + itoa(len(jsonData)) + " bytes")

	// Re-enable compression with detailed debugging
	DebugPrintln("[BuildDict] Starting compression...")
	var buf bytes.Buffer
	DebugPrintln("[BuildDict] Creating NewWriter...")
	w := tinycompress.NewWriter(&buf)
	DebugPrintln("[BuildDict] Calling Write...")
	_, err := w.Write(jsonData)
	if err != nil {
		DebugPrintln("[BuildDict] ERROR: Compression write failed: " + err.Error())
		d.cachedDict = jsonData
		return
	}
	DebugPrintln("[BuildDict] Calling Close...")
	err = w.Close()
	if err != nil {
		DebugPrintln("[BuildDict] ERROR: Compression close failed: " + err.Error())
		d.cachedDict = jsonData
		return
	}
	DebugPrintln("[BuildDict] Compression complete")

	compressed := buf.Bytes()
	DebugPrintln("[BuildDict] Compressed size: " + itoa(len(compressed)) + " bytes")

	if len(compressed) == 0 {
		DebugPrintln("[BuildDict] ERROR: Compression produced empty output")
		d.cachedDict = jsonData
		return
	}

	d.cachedDict = make([]byte, len(compressed))
	copy(d.cachedDict, compressed)
	DebugPrintln("[BuildDict] Dictionary cached successfully")
}

// Generate generates the complete dictionary in JSON format
// Format follows Klipper's data dictionary format
func (d *Dictionary) Generate() []byte {
	// Return cached dictionary if available
	if d.cachedDict != nil {
		return d.cachedDict
	}

	// Otherwise generate on-the-fly (no lock needed for reading registry)
	return d.generateJSON()
}

// generateJSON builds the JSON dictionary (acquires read lock)
func (d *Dictionary) generateJSON() []byte {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.buildJSONLocked()
}

// buildJSONLocked builds the JSON dictionary (caller must hold lock)
func (d *Dictionary) buildJSONLocked() []byte {
	// Get commands and responses from registry
	commands, responses := d.commandReg.GetCommandsAndResponses()
	return d.buildJSONLockedWithData(commands, responses)
}

// buildJSONLockedWithData builds the JSON dictionary with pre-fetched command data
// This version avoids nested lock acquisition (caller must hold dictionary lock)
func (d *Dictionary) buildJSONLockedWithData(commands map[string]int, responses map[string]int) []byte {
	// Pre-allocate a reasonably sized buffer
	result := make([]byte, 0, 1024)

	// Build JSON manually byte by byte to avoid string concatenation issues
	result = append(result, []byte(`{"version":"`)...)
	result = append(result, []byte(d.version)...)
	result = append(result, []byte(`","build_versions":"`)...)
	result = append(result, []byte(d.buildVersions)...)
	result = append(result, []byte(`","config":{`)...)

	// Add all constants (sorted for consistency)
	constNames := make([]string, 0, len(d.constants))
	for name := range d.constants {
		constNames = append(constNames, name)
	}
	// Simple bubble sort for embedded (no sort package issues)
	for i := 0; i < len(constNames); i++ {
		for j := i + 1; j < len(constNames); j++ {
			if constNames[i] > constNames[j] {
				constNames[i], constNames[j] = constNames[j], constNames[i]
			}
		}
	}

	first := true
	for _, name := range constNames {
		c := d.constants[name]
		if !first {
			result = append(result, ',')
		}
		result = append(result, '"')
		result = append(result, []byte(name)...)
		result = append(result, []byte(`":"`)...)
		result = append(result, []byte(valueToString(c.Value))...)
		result = append(result, '"')
		first = false
	}
	result = append(result, []byte(`},"commands":{`)...)

	// Commands (sorted by ID for consistency)
	cmdIDs := make([]int, 0, len(commands))
	for _, id := range commands {
		cmdIDs = append(cmdIDs, id)
	}
	// Simple bubble sort
	for i := 0; i < len(cmdIDs); i++ {
		for j := i + 1; j < len(cmdIDs); j++ {
			if cmdIDs[i] > cmdIDs[j] {
				cmdIDs[i], cmdIDs[j] = cmdIDs[j], cmdIDs[i]
			}
		}
	}

	firstCmd := true
	for _, id := range cmdIDs {
		// Find the format for this ID
		for cmdFormat, cmdID := range commands {
			if cmdID == id {
				if !firstCmd {
					result = append(result, ',')
				}
				result = append(result, '"')
				result = append(result, []byte(cmdFormat)...)
				result = append(result, []byte(`":`)...)
				result = append(result, []byte(itoa(cmdID))...)
				firstCmd = false
				break
			}
		}
	}
	result = append(result, []byte(`},"responses":{`)...)

	// Responses (sorted by ID for consistency)
	respIDs := make([]int, 0, len(responses))
	for _, id := range responses {
		respIDs = append(respIDs, id)
	}
	// Simple bubble sort
	for i := 0; i < len(respIDs); i++ {
		for j := i + 1; j < len(respIDs); j++ {
			if respIDs[i] > respIDs[j] {
				respIDs[i], respIDs[j] = respIDs[j], respIDs[i]
			}
		}
	}

	firstResp := true
	for _, id := range respIDs {
		// Find the format for this ID
		for respFormat, respID := range responses {
			if respID == id {
				if !firstResp {
					result = append(result, ',')
				}
				result = append(result, '"')
				result = append(result, []byte(respFormat)...)
				result = append(result, []byte(`":`)...)
				result = append(result, []byte(itoa(respID))...)
				firstResp = false
				break
			}
		}
	}
	result = append(result, '}') // Close responses object

	// Add enumerations if any
	if len(d.enumerations) > 0 {
		result = append(result, []byte(`,"enumerations":{`)...)

		// Sort enumeration names
		enumNames := make([]string, 0, len(d.enumerations))
		for name := range d.enumerations {
			enumNames = append(enumNames, name)
		}
		// Simple bubble sort
		for i := 0; i < len(enumNames); i++ {
			for j := i + 1; j < len(enumNames); j++ {
				if enumNames[i] > enumNames[j] {
					enumNames[i], enumNames[j] = enumNames[j], enumNames[i]
				}
			}
		}

		firstEnum := true
		for _, name := range enumNames {
			enum := d.enumerations[name]
			if !firstEnum {
				result = append(result, ',')
			}
			result = append(result, '"')
			result = append(result, []byte(name)...)
			result = append(result, []byte(`":{`)...)

			// Add enumeration values as key-value pairs (name: index)
			// Skip empty values (null) - only include named enumerations
			firstValue := true
			for i, value := range enum.Values {
				if value != "" {
					if !firstValue {
						result = append(result, ',')
					}
					result = append(result, '"')
					result = append(result, []byte(value)...)
					result = append(result, []byte(`":`)...)
					result = append(result, []byte(itoa(i))...)
					firstValue = false
				}
			}
			result = append(result, '}')
			firstEnum = false
		}
		result = append(result, '}')
	}

	result = append(result, '}')

	// Return uncompressed for now - TinyGo's zlib may not be fully functional
	// TODO: Implement compression once we verify it works in TinyGo
	return result
}

// GetChunk returns a chunk of the dictionary starting at offset
func (d *Dictionary) GetChunk(offset uint32, count uint8) []byte {
	// Don't lock here - Generate() handles its own locking
	// Adding a lock here causes deadlock because Generate() -> generateJSON() also locks
	data := d.Generate()

	// Safety check: ensure we have a valid dictionary
	if data == nil || len(data) == 0 {
		return []byte{}
	}

	// Check if offset is beyond data length
	if offset >= uint32(len(data)) {
		return []byte{}
	}

	// Calculate end position with bounds checking
	end := offset + uint32(count)
	if end > uint32(len(data)) {
		end = uint32(len(data))
	}

	// Ensure we don't create invalid slice
	if end <= offset {
		return []byte{}
	}

	// CRITICAL: Return a copy, not a slice
	// TinyGo's GC can be aggressive, and returning a slice that references
	// the cached dictionary can lead to memory corruption if the data is
	// modified or moved during USB transmission
	chunk := make([]byte, end-offset)
	copy(chunk, data[offset:end])
	return chunk
}

// GetGlobalDictionary returns the global dictionary instance
func GetGlobalDictionary() *Dictionary {

	return globalDictionary
}
