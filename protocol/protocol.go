// Package protocol implements the Klipper communication protocol
package protocol

// Version represents the Gopper firmware version
const Version = "0.0.1-alpha"

// Protocol constants
const (
	MessageMax     = 512 // Maximum output buffer size (was 64, increased to handle multiple messages)
	MessageMin     = 5   // Minimum message size (header + CRC)
	MessageHeader  = 2   // Message header size
	MessageTrailer = 3   // Message trailer size (CRC)

	// Message sequence masks
	MessageSeqMask  = 0x0F
	MessageSeqShift = 4
)

// MessageBlock represents a Klipper message block
type MessageBlock struct {
	Length   uint8
	Sequence uint8
	Data     []byte
	CRC      uint16
}
