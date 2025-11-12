package protocol

// InputBuffer provides an abstraction for reading incoming protocol data
type InputBuffer interface {
	// Data returns the available data slice
	Data() []byte

	// Available returns the number of bytes available
	Available() int

	// Pop removes n bytes from the front of the buffer
	Pop(n int)
}

// OutputBuffer provides an abstraction for writing outgoing protocol data
type OutputBuffer interface {
	// Output writes data to the buffer
	Output(data []byte)

	// CurPosition returns the current write position
	CurPosition() int

	// Update modifies a byte at a specific position
	Update(pos int, val byte)

	// DataSince returns data from a specific position to current
	DataSince(pos int) []byte
}

// SliceInputBuffer implements InputBuffer using a byte slice
type SliceInputBuffer struct {
	data []byte
}

// NewSliceInputBuffer creates a new SliceInputBuffer
func NewSliceInputBuffer(data []byte) *SliceInputBuffer {
	return &SliceInputBuffer{data: data}
}

func (s *SliceInputBuffer) Data() []byte {
	return s.data
}

func (s *SliceInputBuffer) Available() int {
	return len(s.data)
}

func (s *SliceInputBuffer) Pop(n int) {
	if n > len(s.data) {
		n = len(s.data)
	}
	s.data = s.data[n:]
}

// ScratchOutput implements OutputBuffer using a fixed-size scratch buffer
type ScratchOutput struct {
	buf [MessageMax]byte
	pos int
}

// NewScratchOutput creates a new ScratchOutput
func NewScratchOutput() *ScratchOutput {
	return &ScratchOutput{pos: 0}
}

func (s *ScratchOutput) Output(data []byte) {
	n := copy(s.buf[s.pos:], data)
	s.pos += n
}

func (s *ScratchOutput) CurPosition() int {
	return s.pos
}

func (s *ScratchOutput) Update(pos int, val byte) {
	if pos < len(s.buf) {
		s.buf[pos] = val
	}
}

func (s *ScratchOutput) DataSince(pos int) []byte {
	if pos > s.pos {
		return nil
	}
	return s.buf[pos:s.pos]
}

// Result returns the accumulated output data
func (s *ScratchOutput) Result() []byte {
	return s.buf[:s.pos]
}

// Reset clears the buffer
func (s *ScratchOutput) Reset() {
	s.pos = 0
}

// FifoBuffer is a circular buffer for serial I/O
type FifoBuffer struct {
	buf   []byte
	read  int
	write int
	size  int
}

// NewFifoBuffer creates a new FifoBuffer with the specified capacity
func NewFifoBuffer(capacity int) *FifoBuffer {
	return &FifoBuffer{
		buf:  make([]byte, capacity),
		size: capacity,
	}
}

// Write appends data to the FIFO buffer
func (f *FifoBuffer) Write(data []byte) int {
	written := 0
	for _, b := range data {
		nextWrite := (f.write + 1) % f.size
		if nextWrite == f.read {
			// Buffer full
			break
		}
		f.buf[f.write] = b
		f.write = nextWrite
		written++
	}
	return written
}

// Read reads up to len(data) bytes from the FIFO buffer
func (f *FifoBuffer) Read(data []byte) int {
	read := 0
	for i := range data {
		if f.read == f.write {
			// Buffer empty
			break
		}
		data[i] = f.buf[f.read]
		f.read = (f.read + 1) % f.size
		read++
	}
	return read
}

// Available returns the number of bytes available for reading
func (f *FifoBuffer) Available() int {
	if f.write >= f.read {
		return f.write - f.read
	}
	return f.size - f.read + f.write
}

// Free returns the number of bytes available for writing
func (f *FifoBuffer) Free() int {
	return f.size - f.Available() - 1
}

// Data returns available data as a slice
// When wrapped, this copies data into a contiguous slice for protocol processing
func (f *FifoBuffer) Data() []byte {
	if f.read <= f.write {
		// Simple case: data is contiguous
		return f.buf[f.read:f.write]
	}
	// Wrapped case: copy both segments into contiguous slice
	// This is critical for correct message parsing
	avail := f.Available()
	result := make([]byte, avail)

	// Copy first segment (read to end of buffer)
	firstLen := f.size - f.read
	copy(result, f.buf[f.read:])

	// Copy second segment (start of buffer to write)
	copy(result[firstLen:], f.buf[:f.write])

	return result
}

// Pop removes n bytes from the front
func (f *FifoBuffer) Pop(n int) {
	for i := 0; i < n && f.read != f.write; i++ {
		f.read = (f.read + 1) % f.size
	}
}

// IsEmpty returns true if the buffer is empty
func (f *FifoBuffer) IsEmpty() bool {
	return f.read == f.write
}

// Reset clears the buffer
func (f *FifoBuffer) Reset() {
	f.read = 0
	f.write = 0
}
