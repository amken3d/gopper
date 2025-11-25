package mcu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"gopper/host/serial"
	"gopper/protocol"
)

// MCU represents a connection to a Klipper microcontroller
type MCU struct {
	// Transport layer
	transport *protocol.HostTransport

	// Serial port
	port serial.Port

	// Dictionary data
	dictionary     *Dictionary
	dictionaryData []byte

	// Connection state
	connected bool
}

// Dictionary represents the parsed MCU dictionary
type Dictionary struct {
	Version       string                 `json:"version"`
	BuildVersions string                 `json:"build_versions"`
	Config        map[string]string      `json:"config"`
	Commands      map[string]int         `json:"commands"`
	Responses     map[string]int         `json:"responses"`
	Enumerations  map[string]map[string]int `json:"enumerations,omitempty"`
}

// NewMCU creates a new MCU instance (not yet connected)
func NewMCU() *MCU {
	return &MCU{
		connected: false,
	}
}

// Connect connects to an MCU via serial port
func (m *MCU) Connect(device string) error {
	return m.ConnectWithConfig(serial.DefaultConfig(device))
}

// ConnectWithConfig connects to an MCU with a custom serial config
func (m *MCU) ConnectWithConfig(cfg *serial.Config) error {
	// Open serial port
	port, err := serial.Open(cfg)
	if err != nil {
		return fmt.Errorf("failed to open serial port: %w", err)
	}

	m.port = port
	m.transport = protocol.NewHostTransport(port)
	m.connected = true

	// Set up response handler for identify responses
	m.transport.SetResponseHandler(m.handleResponse)

	// Give MCU time to initialize (if it just powered on)
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Close closes the connection to the MCU
func (m *MCU) Close() error {
	if m.transport != nil {
		if err := m.transport.Close(); err != nil {
			return err
		}
	}
	m.connected = false
	return nil
}

// RetrieveDictionary retrieves the complete dictionary from the MCU
func (m *MCU) RetrieveDictionary() error {
	if !m.connected {
		return fmt.Errorf("not connected to MCU")
	}

	fmt.Println("Retrieving dictionary from MCU...")

	// Dictionary will be retrieved in chunks
	// Start with offset 0, count 40 (typical chunk size)
	var dictBuffer bytes.Buffer
	offset := uint32(0)
	chunkSize := uint8(40)
	maxIterations := 1000 // Safety limit

	for i := 0; i < maxIterations; i++ {
		// Send identify command
		chunk, err := m.sendIdentify(offset, chunkSize)
		if err != nil {
			return fmt.Errorf("failed to retrieve dictionary chunk at offset %d: %w", offset, err)
		}

		if len(chunk) == 0 {
			// No more data
			break
		}

		// Append chunk to buffer
		dictBuffer.Write(chunk)
		offset += uint32(len(chunk))

		// Progress indicator
		if i%10 == 0 {
			fmt.Printf("  Retrieved %d bytes...\n", offset)
		}

		// If we got less than requested, we're done
		if len(chunk) < int(chunkSize) {
			break
		}
	}

	m.dictionaryData = dictBuffer.Bytes()
	fmt.Printf("Dictionary retrieved: %d bytes\n", len(m.dictionaryData))

	// Try to decompress if it's compressed
	// (Gopper uses tinycompress/zlib, but we can use standard zlib for host)
	decompressed, err := m.tryDecompress(m.dictionaryData)
	if err == nil && len(decompressed) > 0 {
		fmt.Printf("Dictionary decompressed: %d -> %d bytes\n", len(m.dictionaryData), len(decompressed))
		m.dictionaryData = decompressed
	}

	// Parse dictionary JSON
	if err := m.parseDictionary(); err != nil {
		return fmt.Errorf("failed to parse dictionary: %w", err)
	}

	return nil
}

// sendIdentify sends an identify command and waits for response
func (m *MCU) sendIdentify(offset uint32, count uint8) ([]byte, error) {
	// Build identify command: cmdID=1, offset (VLQ uint), count (VLQ uint)
	err := m.transport.SendCommand(1, func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, offset)
		protocol.EncodeVLQUint(output, uint32(count))
	})

	if err != nil {
		return nil, fmt.Errorf("failed to send identify command: %w", err)
	}

	// Wait for response (identify_response has cmdID=0)
	resp, err := m.transport.ReceiveResponse(1 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to receive identify response: %w", err)
	}

	// Parse response payload: cmdID (VLQ), offset (VLQ), data (VLQ bytes)
	payload := resp.Payload

	// Decode command ID (should be 0 for identify_response)
	cmdID, err := protocol.DecodeVLQUint(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response command ID: %w", err)
	}

	if cmdID != 0 {
		return nil, fmt.Errorf("unexpected response command ID: %d (expected 0)", cmdID)
	}

	// Decode offset (should match our request)
	respOffset, err := protocol.DecodeVLQUint(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response offset: %w", err)
	}

	if respOffset != offset {
		return nil, fmt.Errorf("offset mismatch: expected %d, got %d", offset, respOffset)
	}

	// Decode data (VLQ-encoded byte array)
	data, err := protocol.DecodeVLQBytes(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response data: %w", err)
	}

	return data, nil
}

// tryDecompress attempts to decompress the dictionary data
func (m *MCU) tryDecompress(data []byte) ([]byte, error) {
	// Check if data looks like zlib (starts with 0x78)
	if len(data) < 2 || data[0] != 0x78 {
		return nil, fmt.Errorf("not zlib compressed")
	}

	// TODO: Implement zlib decompression for compressed dictionaries
	// For now, just try to parse as JSON directly
	// Most MCUs send uncompressed for simplicity
	return nil, fmt.Errorf("decompression not yet implemented")
}

// parseDictionary parses the dictionary JSON
func (m *MCU) parseDictionary() error {
	dict := &Dictionary{}
	if err := json.Unmarshal(m.dictionaryData, dict); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	m.dictionary = dict
	return nil
}

// handleResponse handles responses from the MCU (async callback)
func (m *MCU) handleResponse(cmdID uint16, data *[]byte) error {
	// For now, just log responses
	// In a full implementation, this would dispatch to specific handlers
	return nil
}

// GetDictionary returns the parsed dictionary
func (m *MCU) GetDictionary() *Dictionary {
	return m.dictionary
}

// GetDictionaryRaw returns the raw dictionary data
func (m *MCU) GetDictionaryRaw() []byte {
	return m.dictionaryData
}

// PrintDictionary prints a summary of the dictionary
func (m *MCU) PrintDictionary() {
	if m.dictionary == nil {
		fmt.Println("No dictionary loaded")
		return
	}

	fmt.Println("\n=== MCU Dictionary ===")
	fmt.Printf("Version: %s\n", m.dictionary.Version)
	fmt.Printf("Build: %s\n", m.dictionary.BuildVersions)

	fmt.Println("\nConfig:")
	for k, v := range m.dictionary.Config {
		fmt.Printf("  %s = %s\n", k, v)
	}

	fmt.Printf("\nCommands (%d):\n", len(m.dictionary.Commands))
	for name, id := range m.dictionary.Commands {
		if id < 10 { // Only show first few
			fmt.Printf("  [%d] %s\n", id, name)
		}
	}
	if len(m.dictionary.Commands) > 10 {
		fmt.Printf("  ... and %d more\n", len(m.dictionary.Commands)-10)
	}

	fmt.Printf("\nResponses (%d):\n", len(m.dictionary.Responses))
	for name, id := range m.dictionary.Responses {
		if id < 10 { // Only show first few
			fmt.Printf("  [%d] %s\n", id, name)
		}
	}
	if len(m.dictionary.Responses) > 10 {
		fmt.Printf("  ... and %d more\n", len(m.dictionary.Responses)-10)
	}

	if len(m.dictionary.Enumerations) > 0 {
		fmt.Printf("\nEnumerations (%d):\n", len(m.dictionary.Enumerations))
		for name, values := range m.dictionary.Enumerations {
			fmt.Printf("  %s: %d values\n", name, len(values))
		}
	}

	fmt.Println("======================\n")
}

// SendCommand sends a generic command to the MCU
func (m *MCU) SendCommand(name string, args func(output protocol.OutputBuffer)) error {
	if !m.connected {
		return fmt.Errorf("not connected to MCU")
	}

	if m.dictionary == nil {
		return fmt.Errorf("dictionary not loaded")
	}

	// Look up command ID
	cmdID, ok := m.dictionary.Commands[name]
	if !ok {
		return fmt.Errorf("unknown command: %s", name)
	}

	return m.transport.SendCommand(uint16(cmdID), args)
}

// IsConnected returns whether the MCU is connected
func (m *MCU) IsConnected() bool {
	return m.connected
}
