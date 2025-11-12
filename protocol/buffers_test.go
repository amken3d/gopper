package protocol

import "testing"

func TestSliceInputBuffer(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	buf := NewSliceInputBuffer(data)

	if buf.Available() != 5 {
		t.Errorf("Expected 5 bytes available, got %d", buf.Available())
	}

	bufData := buf.Data()
	if len(bufData) != 5 {
		t.Errorf("Expected 5 bytes in data, got %d", len(bufData))
	}

	buf.Pop(2)
	if buf.Available() != 3 {
		t.Errorf("After popping 2, expected 3 bytes available, got %d", buf.Available())
	}

	bufData = buf.Data()
	if len(bufData) != 3 || bufData[0] != 3 {
		t.Errorf("After popping 2, expected first byte to be 3, got %d", bufData[0])
	}
}

func TestScratchOutput(t *testing.T) {
	scratch := NewScratchOutput()

	data1 := []byte{1, 2, 3}
	scratch.Output(data1)

	if scratch.CurPosition() != 3 {
		t.Errorf("Expected position 3, got %d", scratch.CurPosition())
	}

	result := scratch.Result()
	if len(result) != 3 {
		t.Errorf("Expected 3 bytes in result, got %d", len(result))
	}

	data2 := []byte{4, 5}
	scratch.Output(data2)

	if scratch.CurPosition() != 5 {
		t.Errorf("Expected position 5, got %d", scratch.CurPosition())
	}

	// Test Update
	scratch.Update(0, 99)
	result = scratch.Result()
	if result[0] != 99 {
		t.Errorf("Expected first byte to be 99, got %d", result[0])
	}

	// Test DataSince
	since := scratch.DataSince(2)
	if len(since) != 3 || since[0] != 3 {
		t.Errorf("DataSince(2) failed: expected [3 4 5], got %v", since)
	}

	// Test Reset
	scratch.Reset()
	if scratch.CurPosition() != 0 {
		t.Errorf("After reset, expected position 0, got %d", scratch.CurPosition())
	}
}

func TestFifoBuffer(t *testing.T) {
	fifo := NewFifoBuffer(10)

	if !fifo.IsEmpty() {
		t.Error("New FIFO should be empty")
	}

	if fifo.Available() != 0 {
		t.Errorf("Empty FIFO should have 0 available, got %d", fifo.Available())
	}

	// Write some data
	data := []byte{1, 2, 3, 4, 5}
	written := fifo.Write(data)

	if written != 5 {
		t.Errorf("Expected to write 5 bytes, wrote %d", written)
	}

	if fifo.Available() != 5 {
		t.Errorf("Expected 5 bytes available, got %d", fifo.Available())
	}

	// Read some data
	readBuf := make([]byte, 3)
	read := fifo.Read(readBuf)

	if read != 3 {
		t.Errorf("Expected to read 3 bytes, read %d", read)
	}

	if readBuf[0] != 1 || readBuf[1] != 2 || readBuf[2] != 3 {
		t.Errorf("Read data mismatch: got %v", readBuf)
	}

	if fifo.Available() != 2 {
		t.Errorf("After reading 3, expected 2 available, got %d", fifo.Available())
	}

	// Test Pop
	fifo.Pop(1)
	if fifo.Available() != 1 {
		t.Errorf("After popping 1, expected 1 available, got %d", fifo.Available())
	}

	// Test wrap-around
	fifo.Reset()
	bigData := make([]byte, 12)
	for i := range bigData {
		bigData[i] = byte(i)
	}
	written = fifo.Write(bigData)
	if written != 9 { // Buffer size is 10, can only store 9 (one slot reserved)
		t.Errorf("Expected to write 9 bytes to size-10 FIFO, wrote %d", written)
	}
}

func TestFifoBufferWrapAround(t *testing.T) {
	fifo := NewFifoBuffer(5)

	// Fill buffer
	fifo.Write([]byte{1, 2, 3, 4})

	// Read some
	readBuf := make([]byte, 2)
	fifo.Read(readBuf)

	// Write more (will wrap around)
	written := fifo.Write([]byte{5, 6})
	if written != 2 {
		t.Errorf("Expected to write 2 bytes, wrote %d", written)
	}

	// Verify order
	allData := make([]byte, 4)
	read := fifo.Read(allData)
	if read != 4 {
		t.Errorf("Expected to read 4 bytes, read %d", read)
	}
	if allData[0] != 3 || allData[1] != 4 || allData[2] != 5 || allData[3] != 6 {
		t.Errorf("Wrap-around data mismatch: got %v", allData)
	}
}

// TestFifoBufferDataWrapped tests the Data() method with wrapped buffer
// This is critical for the protocol layer which uses Data() to inspect messages
func TestFifoBufferDataWrapped(t *testing.T) {
	fifo := NewFifoBuffer(10)

	// Write initial data that will be partially consumed
	fifo.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8})

	// Pop some data to advance read pointer
	fifo.Pop(5) // Now read=5, write=8

	// Write more data to wrap around
	// Buffer has capacity 10, so write can go from 8 to 9, then wrap to 0, 1, 2, 3
	written := fifo.Write([]byte{10, 11, 12, 13, 14, 15})
	if written != 6 { // Should write 6 bytes: positions 8,9,0,1,2,3
		t.Errorf("Expected to write 6 bytes, wrote %d", written)
	}

	// Now buffer contains:
	// - Positions 5-9: [6, 7, 8, 10, 11]
	// - Positions 0-3: [12, 13, 14, 15]
	// So Data() should return [6, 7, 8, 10, 11, 12, 13, 14, 15]

	data := fifo.Data()
	expected := []byte{6, 7, 8, 10, 11, 12, 13, 14, 15}

	if len(data) != len(expected) {
		t.Errorf("Expected %d bytes from Data(), got %d", len(expected), len(data))
	}

	for i, val := range expected {
		if i >= len(data) || data[i] != val {
			t.Errorf("Data mismatch at index %d: expected %d, got %d (full data: %v)", i, val, data[i], data)
			break
		}
	}

	// Verify Available() matches Data() length
	if fifo.Available() != len(data) {
		t.Errorf("Available() returned %d but Data() has length %d", fifo.Available(), len(data))
	}
}
