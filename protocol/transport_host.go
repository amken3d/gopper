package protocol

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ResponseHandler is a function type for handling received responses from MCU
type ResponseHandler func(cmdID uint16, data *[]byte) error

// HostTransport handles the Klipper protocol from the host side
// It inverts the Transport logic: sends commands, waits for ACKs, receives responses
type HostTransport struct {
	// Serial I/O
	port io.ReadWriteCloser

	// Sequence tracking (0x10-0x1F for host messages)
	currentSeq uint32 // atomic uint8 stored as uint32

	// Synchronization state
	isSynchronized uint32 // atomic bool (0 = false, 1 = true)

	// Buffers
	inputBuffer  *FifoBuffer
	outputBuffer *bytes.Buffer

	// Channel for ACK/NAK messages
	ackChan chan *Message

	// Channels for response messages
	responseChan chan *Message

	// Response handler (optional callback for async responses)
	responseHandler ResponseHandler

	// Mutex for thread-safe operations
	writeMutex sync.Mutex
	readMutex  sync.Mutex

	// Stop channel for graceful shutdown
	stopChan chan struct{}
	doneChan chan struct{}
}

// Message represents a parsed Klipper message
type Message struct {
	Length   uint8
	Sequence uint8
	Payload  []byte // Frame data without header/trailer
	CRC      uint16
}

// NewHostTransport creates a new host-side transport
func NewHostTransport(port io.ReadWriteCloser) *HostTransport {
	t := &HostTransport{
		port:         port,
		currentSeq:   MessageDest, // Start at 0x10
		inputBuffer:  NewFifoBuffer(512),
		outputBuffer: bytes.NewBuffer(make([]byte, 0, 256)),
		ackChan:      make(chan *Message, 1),     // Buffered for ACK
		responseChan: make(chan *Message, 16),    // Buffered for responses
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}

	atomic.StoreUint32(&t.isSynchronized, 1) // Start synchronized

	// Start background reader
	go t.readLoop()

	return t
}

// SendCommand sends a command to the MCU and waits for ACK
func (t *HostTransport) SendCommand(cmdID uint16, args func(output OutputBuffer)) error {
	return t.SendCommandWithTimeout(cmdID, args, 2*time.Second)
}

// SendCommandWithTimeout sends a command with a custom timeout
func (t *HostTransport) SendCommandWithTimeout(cmdID uint16, args func(output OutputBuffer), timeout time.Duration) error {
	// Build command message
	msg, err := t.buildCommandMessage(cmdID, args)
	if err != nil {
		return fmt.Errorf("failed to build command: %w", err)
	}

	// Send message
	if err := t.writeMessage(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Wait for ACK
	if err := t.waitForAck(timeout); err != nil {
		return fmt.Errorf("ACK timeout or error: %w", err)
	}

	return nil
}

// buildCommandMessage constructs a complete message with header, payload, CRC, and sync
func (t *HostTransport) buildCommandMessage(cmdID uint16, args func(output OutputBuffer)) ([]byte, error) {
	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	t.outputBuffer.Reset()

	// Write header placeholders (length and sequence)
	seq := uint8(atomic.LoadUint32(&t.currentSeq))
	t.outputBuffer.Write([]byte{0, seq}) // Length placeholder, sequence

	// Create scratch output for payload
	scratch := NewScratchOutput()

	// Encode command ID
	EncodeVLQUint(scratch, uint32(cmdID))

	// Encode arguments if provided
	if args != nil {
		args(scratch)
	}

	// Write payload
	payload := scratch.Result()
	t.outputBuffer.Write(payload)

	// Calculate message length (including trailer)
	msgLen := MessageHeaderSize + len(payload) + MessageTrailerSize

	if msgLen > MessageLengthMax {
		return nil, fmt.Errorf("message too long: %d bytes (max %d)", msgLen, MessageLengthMax)
	}

	// Get the full message data
	data := t.outputBuffer.Bytes()

	// Update length field
	data[MessagePositionLen] = uint8(msgLen)

	// Calculate CRC over header + payload
	crc := CRC16(data[:MessageHeaderSize+len(payload)])

	// Append CRC and sync byte
	crcHigh := uint8((crc & 0xFF00) >> 8)
	crcLow := uint8(crc & 0xFF)
	t.outputBuffer.Write([]byte{crcHigh, crcLow, MessageValueSync})

	// Make a copy to return
	msgCopy := make([]byte, t.outputBuffer.Len())
	copy(msgCopy, t.outputBuffer.Bytes())

	return msgCopy, nil
}

// writeMessage sends a message to the serial port
func (t *HostTransport) writeMessage(msg []byte) error {
	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	n, err := t.port.Write(msg)
	if err != nil {
		return err
	}
	if n != len(msg) {
		return fmt.Errorf("incomplete write: %d/%d bytes", n, len(msg))
	}

	return nil
}

// waitForAck waits for an ACK message with timeout
func (t *HostTransport) waitForAck(timeout time.Duration) error {
	select {
	case ack := <-t.ackChan:
		// Verify sequence matches
		expectedSeq := uint8(atomic.LoadUint32(&t.currentSeq))
		if ack.Sequence != expectedSeq {
			return fmt.Errorf("sequence mismatch: expected 0x%02x, got 0x%02x", expectedSeq, ack.Sequence)
		}

		// Advance sequence (0x10-0x1F, wrapping)
		nextSeq := ((expectedSeq + 1) & MessageSeqMask) | MessageDest
		atomic.StoreUint32(&t.currentSeq, uint32(nextSeq))

		return nil

	case <-time.After(timeout):
		return fmt.Errorf("ACK timeout after %v", timeout)

	case <-t.stopChan:
		return fmt.Errorf("transport stopped")
	}
}

// ReceiveResponse receives a response message with timeout
func (t *HostTransport) ReceiveResponse(timeout time.Duration) (*Message, error) {
	select {
	case resp := <-t.responseChan:
		return resp, nil

	case <-time.After(timeout):
		return nil, fmt.Errorf("response timeout after %v", timeout)

	case <-t.stopChan:
		return nil, fmt.Errorf("transport stopped")
	}
}

// SetResponseHandler sets a callback for handling responses asynchronously
func (t *HostTransport) SetResponseHandler(handler ResponseHandler) {
	t.responseHandler = handler
}

// readLoop continuously reads from serial port and processes messages
func (t *HostTransport) readLoop() {
	defer close(t.doneChan)

	buffer := make([]byte, 256)

	for {
		select {
		case <-t.stopChan:
			return
		default:
		}

		// Read from serial port
		n, err := t.port.Read(buffer)
		if err != nil {
			if err == io.EOF {
				return
			}
			// Log error but continue
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if n > 0 {
			// Add data to input buffer
			t.inputBuffer.Write(buffer[:n])

			// Process messages
			t.processMessages()
		}
	}
}

// processMessages parses and dispatches messages from the input buffer
func (t *HostTransport) processMessages() {
	t.readMutex.Lock()
	defer t.readMutex.Unlock()

	data := t.inputBuffer.Data()

	for len(data) > 0 {
		if !t.getSynchronized() {
			// Look for sync byte
			syncPos := -1
			for i, b := range data {
				if b == MessageValueSync {
					syncPos = i
					break
				}
			}

			if syncPos >= 0 {
				// Found sync - skip to after sync byte
				data = data[syncPos+1:]
				t.setSynchronized(true)
			} else {
				// No sync found - discard all
				data = nil
			}
		} else {
			// Skip leading sync bytes
			if data[0] == MessageValueSync {
				data = data[1:]
				continue
			}

			// Need minimum message length
			if len(data) < MessageLengthMin {
				break
			}

			// Extract message length
			msgLen := int(data[MessagePositionLen])
			if msgLen < MessageLengthMin || msgLen > MessageLengthMax {
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

			// Extract message components
			seq := data[MessagePositionSeq]
			payload := make([]byte, msgLen-MessageHeaderSize-MessageTrailerSize)
			copy(payload, data[MessageHeaderSize:msgLen-MessageTrailerSize])

			msg := &Message{
				Length:   data[MessagePositionLen],
				Sequence: seq,
				Payload:  payload,
				CRC:      frameCRC,
			}

			// Advance data pointer
			data = data[msgLen:]

			// Dispatch message
			t.dispatchMessage(msg)
		}
	}

	// Remove consumed bytes from input buffer
	consumed := t.inputBuffer.Available() - len(data)
	if consumed > 0 {
		t.inputBuffer.Pop(consumed)
	}
}

// dispatchMessage routes a message to the appropriate channel
func (t *HostTransport) dispatchMessage(msg *Message) {
	// Check if this is an ACK (minimal message with just header + trailer)
	if len(msg.Payload) == 0 {
		// This is an ACK/NAK
		select {
		case t.ackChan <- msg:
		default:
			// ACK channel full, drop (shouldn't happen with buffered channel)
		}
		return
	}

	// This is a response message
	// Call response handler if set
	if t.responseHandler != nil {
		// Decode command ID from payload
		payloadCopy := make([]byte, len(msg.Payload))
		copy(payloadCopy, msg.Payload)
		cmdID, err := DecodeVLQUint(&payloadCopy)
		if err == nil {
			_ = t.responseHandler(uint16(cmdID), &payloadCopy)
		}
	}

	// Also send to response channel for synchronous retrieval
	select {
	case t.responseChan <- msg:
	default:
		// Response channel full, drop oldest
		select {
		case <-t.responseChan:
		default:
		}
		t.responseChan <- msg
	}
}

// Close stops the transport and closes the serial port
func (t *HostTransport) Close() error {
	close(t.stopChan)
	<-t.doneChan // Wait for read loop to finish

	if t.port != nil {
		return t.port.Close()
	}
	return nil
}

// Reset resets the transport state (useful after errors)
func (t *HostTransport) Reset() {
	atomic.StoreUint32(&t.isSynchronized, 1)
	atomic.StoreUint32(&t.currentSeq, MessageDest)

	// Drain channels
	for len(t.ackChan) > 0 {
		<-t.ackChan
	}
	for len(t.responseChan) > 0 {
		<-t.responseChan
	}

	// Clear input buffer
	if t.inputBuffer.Available() > 0 {
		t.inputBuffer.Pop(t.inputBuffer.Available())
	}
}

// Helper methods for atomic operations
func (t *HostTransport) getSynchronized() bool {
	return atomic.LoadUint32(&t.isSynchronized) != 0
}

func (t *HostTransport) setSynchronized(val bool) {
	if val {
		atomic.StoreUint32(&t.isSynchronized, 1)
	} else {
		atomic.StoreUint32(&t.isSynchronized, 0)
	}
}

// GetCurrentSequence returns the current sequence number (for debugging)
func (t *HostTransport) GetCurrentSequence() uint8 {
	return uint8(atomic.LoadUint32(&t.currentSeq))
}
