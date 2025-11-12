package protocol

import "sync/atomic"

const (
	MessageHeaderSize  = 2
	MessageTrailerSize = 3
	MessageLengthMin   = MessageHeaderSize + MessageTrailerSize
	MessageLengthMax   = 64
	MessagePositionLen = 0
	MessagePositionSeq = 1
	MessageTrailerCRC  = 3
	MessageTrailerSync = 1
	MessageValueSync   = 0x7E
	MessageDest        = 0x10
	// MessageSeqMask is already defined in protocol.go
)

// CommandHandler is a function type for handling decoded commands
type CommandHandler func(cmdID uint16, data *[]byte) error

// Transport handles the Klipper protocol transport layer
type Transport struct {
	isSynchronized uint32 // atomic bool (0 = false, 1 = true)
	nextSequence   uint32 // atomic uint8 stored as uint32
	// For receiving: expected sequence from host (0x10-0x1F)
	// For sending: sequence for responses/ACKs (same value, but 0x00-0x0F range)
	output        OutputBuffer
	handler       CommandHandler
	resetCallback func() // Called when host reset is detected
	flushCallback func() // Called to immediately flush ACK to USB
}

// NewTransport creates a new Transport instance
func NewTransport(output OutputBuffer, handler CommandHandler) *Transport {
	return &Transport{
		isSynchronized: 1,           // Start synchronized
		nextSequence:   MessageDest, // Expected sequence from host (0x10)
		// Also used for ACK/response sequence (as 0x00)
		output:  output,
		handler: handler,
	}
}

// Receive processes incoming data from the input buffer
// This is the main entry point for handling received messages
func (t *Transport) Receive(input InputBuffer) {
	data := input.Data()

	for len(data) > 0 {
		if !t.getSynchronized() {
			// Look for sync byte to resynchronize
			syncPos := -1
			for i, b := range data {
				if b == MessageValueSync {
					syncPos = i
					break
				}
			}

			if syncPos >= 0 {
				// Found sync byte - skip garbage before it and resync
				data = data[syncPos+1:]
				t.setSynchronized(true)
				t.encodeAckNak()
				// Continue processing in synchronized mode
			} else {
				// No sync byte found - discard all data
				data = nil
			}
		} else {
			// Skip leading sync bytes
			if data[0] == MessageValueSync {
				data = data[1:]
				continue
			}

			// Need at least minimum message length
			if len(data) < MessageLengthMin {
				break
			}

			// Extract message length
			msgLen := int(data[MessagePositionLen])
			if msgLen < MessageLengthMin || msgLen > MessageLengthMax {
				t.setSynchronized(false)
				continue
			}

			// Check sequence/destination byte
			seq := data[MessagePositionSeq]
			if seq&^MessageSeqMask != MessageDest {
				t.setSynchronized(false)
				continue
			}

			// Wait for full message
			if len(data) < msgLen {
				break
			}

			// Verify trailing sync byte
			if data[msgLen-MessageTrailerSync] != MessageValueSync {
				t.setSynchronized(false)
				continue
			}

			// Verify CRC
			frameCRC := uint16(data[msgLen-MessageTrailerCRC])<<8 |
				uint16(data[msgLen-MessageTrailerCRC+1])
			actualCRC := CRC16(data[:msgLen-MessageTrailerSize])

			if frameCRC != actualCRC {
				t.setSynchronized(false)
				continue
			}

			// Extract frame data (between header and trailer)
			frame := data[MessageHeaderSize : msgLen-MessageTrailerSize]
			data = data[msgLen:]

			// Check for host reset (sequence back to MESSAGE_DEST)
			expectedSeq := uint8(atomic.LoadUint32(&t.nextSequence))
			if seq == MessageDest && expectedSeq != MessageDest {
				// Host reset detected - clear our state
				atomic.StoreUint32(&t.nextSequence, MessageDest)
				expectedSeq = MessageDest
				// Call reset callback if set
				if t.resetCallback != nil {
					t.resetCallback()
				}
			}

			// Process frame if sequence matches (exactly matching Anchor's logic)
			if seq == expectedSeq {
				// Sequence matches - increment and process
				nextSeq := ((seq + 1) & MessageSeqMask) | MessageDest
				atomic.StoreUint32(&t.nextSequence, uint32(nextSeq))
				_ = t.parseFrame(frame)
			}
			// Always send ACK/NAK after processing (or not processing) the frame
			// This matches Anchor's behavior - ACK is sent even if sequence didn't match
			// If sequence didn't match, this acts as a NAK with expected sequence
			t.encodeAckNak()
		}
	}

	// Remove consumed bytes from input
	consumed := input.Available() - len(data)
	if consumed > 0 {
		input.Pop(consumed)
	}
}

// parseFrame extracts and dispatches commands from a frame
func (t *Transport) parseFrame(frame []byte) (err error) {
	// Recover from any panics in command handlers to prevent firmware crash
	defer func() {
		if r := recover(); r != nil {
			// Panic occurred - set synchronized to false to trigger resync
			t.setSynchronized(false)
		}
	}()

	for len(frame) > 0 {
		// Decode command ID
		cmdID, err := DecodeVLQUint(&frame)
		if err != nil {
			// Malformed VLQ - desync and return
			t.setSynchronized(false)
			return err
		}

		// Call command handler
		if t.handler != nil {
			if err := t.handler(uint16(cmdID), &frame); err != nil {
				// Handler error - log but continue processing
				// Don't desync on handler errors
				return err
			}
		}
	}
	return nil
}

// encodeAckNak sends an ACK/NAK message
// CRITICAL: ACK must be sent immediately, not buffered with responses
// This matches Klipper's expectation that ACK arrives before response
func (t *Transport) encodeAckNak() {
	ns := uint8(atomic.LoadUint32(&t.nextSequence))
	crc := CRC16([]byte{5, ns})

	ackMsg := []byte{
		5,
		ns,
		uint8((crc & 0xFF00) >> 8),
		uint8(crc & 0xFF),
		MessageValueSync,
	}

	t.output.Output(ackMsg)

	// Force immediate flush of ACK - don't wait for main loop
	// This is critical for serialqueue which waits for ACK before accepting responses
	if t.flushCallback != nil {
		t.flushCallback()
	}
}

// EncodeFrame encodes and sends a frame with the given data
func (t *Transport) EncodeFrame(frameData func(output OutputBuffer)) {
	cursor := t.output.CurPosition()

	// Write header (length placeholder and sequence)
	// CRITICAL: Per Klipper protocol docs, both ACK and responses use the SAME sequence
	// "The high-order bits always contain 0x10" applies to BOTH directions
	// So if we received 0x10, we send ACK and response with 0x11 (NOT 0x01!)
	seq := uint8(atomic.LoadUint32(&t.nextSequence))
	t.output.Output([]byte{0, seq})

	// Write frame contents
	frameData(t.output)

	// Update length field
	changed := len(t.output.DataSince(cursor))
	t.output.Update(cursor, uint8(changed+MessageTrailerSize))

	// Calculate and write CRC
	crc := CRC16(t.output.DataSince(cursor))
	t.output.Output([]byte{
		uint8((crc & 0xFF00) >> 8),
		uint8(crc & 0xFF),
		MessageValueSync,
	})

	// Don't increment sequence - nextSequence is already correct
	// Multiple responses can be sent with the same sequence number
}

// SendCommand sends a command with arguments
func (t *Transport) SendCommand(cmdID uint16, args func(output OutputBuffer)) {
	t.EncodeFrame(func(output OutputBuffer) {
		EncodeVLQUint(output, uint32(cmdID))
		if args != nil {
			args(output)
		}
	})
}

// Reset resets the transport state (useful after USB disconnect/reconnect)
func (t *Transport) Reset() {
	atomic.StoreUint32(&t.isSynchronized, 1)
	atomic.StoreUint32(&t.nextSequence, MessageDest)

	// Call reset callback if set
	if t.resetCallback != nil {
		t.resetCallback()
	}
}

// SetResetCallback sets a callback to be called when host reset is detected
func (t *Transport) SetResetCallback(callback func()) {
	t.resetCallback = callback
}

// SetFlushCallback sets a callback to immediately flush ACK messages to USB
// This is critical for Klipper's serialqueue which expects ACK before response
func (t *Transport) SetFlushCallback(callback func()) {
	t.flushCallback = callback
}

// Helper methods for atomic operations
func (t *Transport) getSynchronized() bool {
	return atomic.LoadUint32(&t.isSynchronized) != 0
}

func (t *Transport) setSynchronized(val bool) {
	if val {
		atomic.StoreUint32(&t.isSynchronized, 1)
	} else {
		atomic.StoreUint32(&t.isSynchronized, 0)
	}
}
