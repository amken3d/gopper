package tinycompress

import (
	"hash"
	"hash/adler32"
	"io"
)

// ZlibEncoder handles zlib-format compatible compression
type ZlibEncoder struct {
	buf    []byte
	output []byte
}

// ZlibStream handles streaming compression for multiple small messages
type ZlibStream struct {
	encoder  *ZlibEncoder
	adler    hash.Hash32
	totalIn  int
	totalOut int
}

// NewZlib creates a new zlib-compatible encoder
func NewZlib(bufferSize int) *ZlibEncoder {
	return &ZlibEncoder{
		buf:    make([]byte, bufferSize),
		output: nil, // Allocate on demand
	}
}

// NewStream creates a streaming encoder for multiple small messages
func (z *ZlibEncoder) NewStream() *ZlibStream {
	return &ZlibStream{
		encoder: z,
		adler:   adler32.New(),
	}
}

// Compress implements minimal DEFLATE compression in zlib format
func (z *ZlibEncoder) Compress(input []byte) ([]byte, int, error) {
	if len(input) == 0 {
		return nil, 0, nil
	}

	requiredSize := len(input) + 7 // 2 (zlib) + 1 (deflate) + 4 (len+nlen) + 4 (adler32) = 11, but data starts at 7
	if cap(z.output) < requiredSize+4 {
		z.output = make([]byte, requiredSize+4)
	}
	z.output = z.output[:requiredSize+4]

	pos := 0

	// Zlib header
	z.output[pos] = 0x78
	z.output[pos+1] = 0x9C
	pos += 2

	// DEFLATE block header (final block, no compression)
	z.output[pos] = 0x01
	pos++

	// Length of data (16-bit little endian)
	length := uint16(len(input))
	z.output[pos] = byte(length)
	z.output[pos+1] = byte(length >> 8)
	pos += 2

	// NLEN (one's complement of length)
	nlength := uint16(^length)
	z.output[pos] = byte(nlength)
	z.output[pos+1] = byte(nlength >> 8)
	pos += 2

	// Copy raw data
	copy(z.output[pos:], input)
	pos += len(input)

	// Calculate and append Adler-32 checksum (big-endian)
	checksum := adler32.Checksum(input)
	z.output[pos] = byte(checksum >> 24)
	z.output[pos+1] = byte(checksum >> 16)
	z.output[pos+2] = byte(checksum >> 8)
	z.output[pos+3] = byte(checksum)
	pos += 4

	return z.output[:pos], pos, nil
}

// WriteBlock writes a block to the stream (for batching multiple messages)
func (s *ZlibStream) WriteBlock(input []byte, isFinal bool) ([]byte, int, error) {
	if len(input) == 0 {
		return nil, 0, nil
	}

	// Update running checksum
	s.adler.Write(input)
	s.totalIn += len(input)

	requiredSize := len(input) + 5 // 1 (block header) + 4 (len+nlen)
	if s.totalOut == 0 {
		requiredSize += 2 // Add zlib header for first block
	}
	if isFinal {
		requiredSize += 4 // Add checksum for final block
	}

	if cap(s.encoder.output) < requiredSize {
		s.encoder.output = make([]byte, requiredSize)
	}
	s.encoder.output = s.encoder.output[:requiredSize]

	pos := 0

	// Zlib header (only on first block)
	if s.totalOut == 0 {
		s.encoder.output[pos] = 0x78
		s.encoder.output[pos+1] = 0x9C
		pos += 2
	}

	// DEFLATE block header
	if isFinal {
		s.encoder.output[pos] = 0x01 // Final block, no compression
	} else {
		s.encoder.output[pos] = 0x00 // Non-final block, no compression
	}
	pos++

	// Length of data (16-bit little endian)
	length := uint16(len(input))
	s.encoder.output[pos] = byte(length)
	s.encoder.output[pos+1] = byte(length >> 8)
	pos += 2

	// NLEN (one's complement of length)
	nlength := uint16(^length)
	s.encoder.output[pos] = byte(nlength)
	s.encoder.output[pos+1] = byte(nlength >> 8)
	pos += 2

	// Copy raw data
	copy(s.encoder.output[pos:], input)
	pos += len(input)

	// Add checksum if final block
	if isFinal {
		checksum := s.adler.Sum32()
		s.encoder.output[pos] = byte(checksum >> 24)
		s.encoder.output[pos+1] = byte(checksum >> 16)
		s.encoder.output[pos+2] = byte(checksum >> 8)
		s.encoder.output[pos+3] = byte(checksum)
		pos += 4
	}

	s.totalOut += pos

	return s.encoder.output[:pos], pos, nil
}

// Reset resets the stream for reuse
func (s *ZlibStream) Reset() {
	s.adler.Reset()
	s.totalIn = 0
	s.totalOut = 0
}

// Decompress decompresses zlib-formatted data
func (z *ZlibEncoder) Decompress(compressed []byte, compressedSize int) ([]byte, int, error) {
	if compressedSize < 7 {
		return nil, 0, nil
	}

	// Verify zlib header
	if compressed[0] != 0x78 {
		return nil, 0, nil
	}

	dataStart := 7
	dataLength := int(compressed[3]) | (int(compressed[4]) << 8)

	if dataStart+dataLength+4 > compressedSize {
		return nil, 0, nil
	}

	if dataLength > len(z.buf) {
		dataLength = len(z.buf)
	}

	// Copy uncompressed data
	copy(z.buf, compressed[dataStart:dataStart+dataLength])

	// Verify Adler-32 checksum (big-endian)
	checksumStart := compressedSize - 4
	expectedChecksum := uint32(compressed[checksumStart])<<24 |
		uint32(compressed[checksumStart+1])<<16 |
		uint32(compressed[checksumStart+2])<<8 |
		uint32(compressed[checksumStart+3])

	actualChecksum := adler32.Checksum(z.buf[:dataLength])
	if actualChecksum != expectedChecksum {
		return nil, 0, nil // Checksum mismatch
	}

	return z.buf[:dataLength], dataLength, nil
}

// DecompressStream decompresses a stream with multiple blocks
func (z *ZlibEncoder) DecompressStream(compressed []byte, compressedSize int) ([]byte, int, error) {
	if compressedSize < 7 {
		return nil, 0, nil
	}

	// Verify and skip zlib header
	if compressed[0] != 0x78 {
		return nil, 0, nil
	}
	pos := 2
	outPos := 0

	adler := adler32.New()

	// Process DEFLATE blocks
	for pos < compressedSize-4 { // Leave room for final checksum
		// Read block header
		blockHeader := compressed[pos]
		pos++

		isFinal := (blockHeader & 0x01) != 0
		blockType := (blockHeader >> 1) & 0x03

		if blockType != 0 { // Only support uncompressed blocks
			return nil, 0, nil
		}

		if pos+4 > compressedSize {
			return nil, 0, nil
		}

		// Read length
		length := int(compressed[pos]) | (int(compressed[pos+1]) << 8)
		nlength := int(compressed[pos+2]) | (int(compressed[pos+3]) << 8)
		pos += 4

		// Verify one's complement
		if length != (^nlength & 0xFFFF) {
			return nil, 0, nil
		}

		if pos+length > compressedSize-4 {
			return nil, 0, nil
		}

		// Copy data
		if outPos+length > len(z.buf) {
			return nil, 0, nil // Output buffer too small
		}
		copy(z.buf[outPos:], compressed[pos:pos+length])
		adler.Write(z.buf[outPos : outPos+length])
		outPos += length
		pos += length

		if isFinal {
			break
		}
	}

	// Verify final Adler-32 checksum
	if pos+4 != compressedSize {
		return nil, 0, nil
	}

	expectedChecksum := uint32(compressed[pos])<<24 |
		uint32(compressed[pos+1])<<16 |
		uint32(compressed[pos+2])<<8 |
		uint32(compressed[pos+3])

	if adler.Sum32() != expectedChecksum {
		return nil, 0, nil
	}

	return z.buf[:outPos], outPos, nil
}

// Writer provides io.WriteCloser interface for zlib compression
type Writer struct {
	output     io.Writer
	inputBuf   []byte
	inputPos   int
	adler      hash.Hash32
	headerDone bool
}

// NewWriter creates a new zlib Writer compatible with io.WriteCloser
func NewWriter(w io.Writer) *Writer {
	debugPrint("[ZLIB] NewWriter: Creating writer...")
	// CRITICAL: Pre-allocate large buffer upfront to avoid ANY allocation during Write()
	// The cores scheduler hangs on memory allocation during Write phase
	// With all features enabled (stepper, I2C, drivers), dictionary can be ~6KB
	writer := &Writer{
		output:   w,
		inputBuf: make([]byte, 0, 8192),
		adler:    adler32.New(),
	}
	debugPrint("[ZLIB] NewWriter: Writer created")
	return writer
}

// debugPrint is a placeholder for debug output (will be set by platform)
var debugPrint = func(s string) {}

// Write implements io.Writer
func (w *Writer) Write(p []byte) (n int, err error) {
	debugPrint("[ZLIB] Write: Appending data...")

	// CRITICAL FIX for cores scheduler: Pre-allocate exact size to avoid reallocation
	// The append() operation can trigger memory reallocation which hangs with cores scheduler
	if cap(w.inputBuf) < len(w.inputBuf)+len(p) {
		debugPrint("[ZLIB] Write: Pre-allocating buffer...")
		newBuf := make([]byte, len(w.inputBuf), len(w.inputBuf)+len(p))
		copy(newBuf, w.inputBuf)
		w.inputBuf = newBuf
		debugPrint("[ZLIB] Write: Buffer pre-allocated")
	}

	// Accumulate input data (now guaranteed to fit without reallocation)
	debugPrint("[ZLIB] Write: Appending to buffer...")
	w.inputBuf = append(w.inputBuf, p...)
	w.inputPos += len(p)
	debugPrint("[ZLIB] Write: Complete")
	return len(p), nil
}

// Close implements io.Closer and writes the compressed data
func (w *Writer) Close() error {
	debugPrint("[ZLIB] Close: Starting...")

	// Write zlib header
	debugPrint("[ZLIB] Close: Writing header...")
	header := []byte{0x78, 0x9C}
	_, err := w.output.Write(header)
	if err != nil {
		debugPrint("[ZLIB] Close: Header write FAILED")
		return err
	}

	// Write DEFLATE block header (final block, no compression)
	debugPrint("[ZLIB] Close: Writing block header...")
	blockHeader := []byte{0x01}
	_, err = w.output.Write(blockHeader)
	if err != nil {
		debugPrint("[ZLIB] Close: Block header write FAILED")
		return err
	}

	// Write length (16-bit little endian)
	debugPrint("[ZLIB] Close: Writing length...")
	length := uint16(len(w.inputBuf))
	lengthBytes := []byte{byte(length), byte(length >> 8)}
	_, err = w.output.Write(lengthBytes)
	if err != nil {
		debugPrint("[ZLIB] Close: Length write FAILED")
		return err
	}

	// Write NLEN (one's complement of length)
	debugPrint("[ZLIB] Close: Writing NLEN...")
	nlength := ^length
	nlengthBytes := []byte{byte(nlength), byte(nlength >> 8)}
	_, err = w.output.Write(nlengthBytes)
	if err != nil {
		debugPrint("[ZLIB] Close: NLEN write FAILED")
		return err
	}

	// Write raw data
	debugPrint("[ZLIB] Close: Writing raw data...")
	_, err = w.output.Write(w.inputBuf)
	if err != nil {
		debugPrint("[ZLIB] Close: Raw data write FAILED")
		return err
	}

	// Calculate and write Adler-32 checksum (big-endian)
	debugPrint("[ZLIB] Close: Calculating checksum...")
	checksum := adler32.Checksum(w.inputBuf)
	debugPrint("[ZLIB] Close: Writing checksum...")
	checksumBytes := []byte{
		byte(checksum >> 24),
		byte(checksum >> 16),
		byte(checksum >> 8),
		byte(checksum),
	}
	_, err = w.output.Write(checksumBytes)
	if err != nil {
		debugPrint("[ZLIB] Close: Checksum write FAILED")
		return err
	}

	debugPrint("[ZLIB] Close: Complete")
	return nil
}

// SetDebugWriter sets the debug output function
func SetDebugWriter(fn func(string)) {
	debugPrint = fn
}
