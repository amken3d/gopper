//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/hex"
	"syscall/js"

	"gopper/protocol"
)

// Global transport instance for the UI
var transport *protocol.Transport
var outputBuffer *protocol.ScratchOutput

func main() {
	// Set up the global output buffer
	outputBuffer = protocol.NewScratchOutput()

	// Export functions to JavaScript
	js.Global().Set("gopperWasm", js.ValueOf(map[string]interface{}{
		"encodeVLQ":       js.FuncOf(encodeVLQWrapper),
		"decodeVLQ":       js.FuncOf(decodeVLQWrapper),
		"crc16":           js.FuncOf(crc16Wrapper),
		"encodeMessage":   js.FuncOf(encodeMessageWrapper),
		"parseResponse":   js.FuncOf(parseResponseWrapper),
		"decodeMessage":   js.FuncOf(decodeMessageWrapper),
		"createTransport": js.FuncOf(createTransportWrapper),
		"version":         protocol.Version,
	}))

	// Keep the program running
	select {}
}

// encodeVLQWrapper encodes a signed integer to VLQ format
// Args: value (int32)
// Returns: hex string
func encodeVLQWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf("error: missing value argument")
	}

	value := int32(args[0].Int())
	output := protocol.NewScratchOutput()
	protocol.EncodeVLQInt(output, value)
	result := output.Result()

	return js.ValueOf(hex.EncodeToString(result))
}

// decodeVLQWrapper decodes a VLQ from hex string
// Args: hexString (string)
// Returns: {value: number, consumed: number, error: string}
func decodeVLQWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return makeResult(0, 0, "missing hex string argument")
	}

	hexStr := args[0].String()
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return makeResult(0, 0, "invalid hex string: "+err.Error())
	}

	value, consumed, err := protocol.DecodeVLQ(data)
	if err != nil {
		return makeResult(0, 0, err.Error())
	}

	return makeResult(int(value), consumed, "")
}

// crc16Wrapper calculates CRC16 checksum
// Args: hexString (string)
// Returns: number (uint16)
func crc16Wrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(0)
	}

	hexStr := args[0].String()
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return js.ValueOf(0)
	}

	crc := protocol.CRC16(data)
	return js.ValueOf(int(crc))
}

// encodeMessageWrapper encodes a command into a Klipper protocol message
// Args: cmdID (uint16), argsHex (string) - hex encoded VLQ parameters
// Returns: hex string of complete message
func encodeMessageWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return js.ValueOf("error: missing arguments")
	}

	cmdID := uint16(args[0].Int())
	argsHex := args[1].String()

	// Decode argument bytes
	argBytes := []byte{}
	if argsHex != "" {
		var err error
		argBytes, err = hex.DecodeString(argsHex)
		if err != nil {
			return js.ValueOf("error: invalid args hex: " + err.Error())
		}
	}

	// Create a fresh output buffer for this message
	msgOutput := protocol.NewScratchOutput()

	// Create a temporary transport to encode the message
	tempTransport := protocol.NewTransport(msgOutput, nil)

	// Encode the command
	tempTransport.SendCommand(cmdID, func(output protocol.OutputBuffer) {
		if len(argBytes) > 0 {
			output.Output(argBytes)
		}
	})

	result := msgOutput.Result()
	return js.ValueOf(hex.EncodeToString(result))
}

// parseResponseWrapper parses a received message
// Args: hexString (string)
// Returns: {synchronized: bool, cmdID: number, data: string (hex), error: string}
func parseResponseWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return makeParseResult(false, 0, "", "missing hex string argument")
	}

	hexStr := args[0].String()
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return makeParseResult(false, 0, "", "invalid hex string: "+err.Error())
	}

	// Create input buffer
	input := protocol.NewSliceInputBuffer(data)

	// Track received commands
	var receivedCmdID uint16
	var receivedData []byte
	var handlerError error

	// Create output for responses
	respOutput := protocol.NewScratchOutput()

	// Create transport with handler
	trans := protocol.NewTransport(respOutput, func(cmdID uint16, dataPtr *[]byte) error {
		receivedCmdID = cmdID
		// Copy the data before handler returns
		receivedData = make([]byte, len(*dataPtr))
		copy(receivedData, *dataPtr)
		return nil
	})

	// Process the message
	trans.Receive(input)

	// Check if we got a command
	if handlerError != nil {
		return makeParseResult(false, 0, "", handlerError.Error())
	}

	if receivedCmdID > 0 {
		return makeParseResult(true, int(receivedCmdID), hex.EncodeToString(receivedData), "")
	}

	// No command received (might be just an ACK)
	return makeParseResult(true, 0, "", "")
}

// decodeMessageWrapper decodes a complete Klipper protocol message
// Args: hexString (string)
// Returns: {length, sequence, cmdID, params: [{value, type}], crc, crcValid, error}
func decodeMessageWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return makeDecodeResult(0, 0, 0, nil, 0, false, "missing hex string argument")
	}

	hexStr := args[0].String()
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return makeDecodeResult(0, 0, 0, nil, 0, false, "invalid hex string: "+err.Error())
	}

	// Need at least: len(1) + seq(1) + crc(2) + sync(1) = 5 bytes
	if len(data) < 5 {
		return makeDecodeResult(0, 0, 0, nil, 0, false, "message too short")
	}

	// Check for trailing sync byte (0x7e)
	if data[len(data)-1] != 0x7e {
		return makeDecodeResult(0, 0, 0, nil, 0, false, "missing sync byte")
	}

	msgLen := int(data[0])
	seq := data[1]

	// Verify CRC
	frameCRC := uint16(data[msgLen-3])<<8 | uint16(data[msgLen-2])
	actualCRC := protocol.CRC16(data[:msgLen-3])
	crcValid := frameCRC == actualCRC

	// Extract payload (between header and trailer)
	payload := data[2 : msgLen-3]

	// Decode command ID and parameters
	var cmdID int32
	var params []map[string]interface{}

	if len(payload) > 0 {
		// First VLQ is command ID
		var consumed int
		var decodeErr error
		cmdID, consumed, decodeErr = protocol.DecodeVLQ(payload)
		if decodeErr != nil {
			return makeDecodeResult(msgLen, int(seq), 0, nil, int(frameCRC), crcValid, "failed to decode command ID: "+decodeErr.Error())
		}
		payload = payload[consumed:]

		// Decode remaining parameters as VLQ values
		for len(payload) > 0 {
			val, consumed, decodeErr := protocol.DecodeVLQ(payload)
			if decodeErr != nil {
				// Not all parameters may be VLQ, stop on error
				break
			}
			params = append(params, map[string]interface{}{
				"value": int(val),
				"bytes": consumed,
			})
			payload = payload[consumed:]
		}
	}

	return makeDecodeResult(msgLen, int(seq), int(cmdID), params, int(frameCRC), crcValid, "")
}

// createTransportWrapper creates a new transport instance
// This would be used for full bidirectional communication
func createTransportWrapper(this js.Value, args []js.Value) interface{} {
	return js.ValueOf("Transport created")
}

// Helper to create result objects
func makeResult(value int, consumed int, errMsg string) js.Value {
	result := make(map[string]interface{})
	result["value"] = value
	result["consumed"] = consumed
	if errMsg != "" {
		result["error"] = errMsg
	}
	return js.ValueOf(result)
}

func makeParseResult(synchronized bool, cmdID int, dataHex string, errMsg string) js.Value {
	result := make(map[string]interface{})
	result["synchronized"] = synchronized
	result["cmdID"] = cmdID
	result["data"] = dataHex
	if errMsg != "" {
		result["error"] = errMsg
	}
	return js.ValueOf(result)
}

func makeDecodeResult(length int, seq int, cmdID int, params []map[string]interface{}, crc int, crcValid bool, errMsg string) js.Value {
	result := make(map[string]interface{})
	result["length"] = length
	result["sequence"] = seq
	result["cmdID"] = cmdID
	result["crc"] = crc
	result["crcValid"] = crcValid

	// Convert params to JS array
	if params != nil {
		jsParams := make([]interface{}, len(params))
		for i, p := range params {
			jsParams[i] = p
		}
		result["params"] = jsParams
	} else {
		result["params"] = []interface{}{}
	}

	if errMsg != "" {
		result["error"] = errMsg
	}
	return js.ValueOf(result)
}
