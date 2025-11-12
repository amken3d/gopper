package protocol

import "errors"

var (
	ErrInvalidVLQ     = errors.New("invalid VLQ encoding")
	ErrBufferTooSmall = errors.New("buffer too small for VLQ")
)

// EncodeVLQInt encodes a signed integer to VLQ format
// This matches Klipper's encoding as implemented in Anchor
func EncodeVLQInt(output OutputBuffer, v int32) {
	// Check ranges and output bytes from most significant to least
	// This matches Anchor's encode_vlq_int function
	if !(-(1<<26) <= v && v < (3<<26)) {
		output.Output([]byte{byte((v>>28)&0x7F) | 0x80})
	}
	if !(-(1<<19) <= v && v < (3<<19)) {
		output.Output([]byte{byte((v>>21)&0x7F) | 0x80})
	}
	if !(-(1<<12) <= v && v < (3<<12)) {
		output.Output([]byte{byte((v>>14)&0x7F) | 0x80})
	}
	if !(-(1<<5) <= v && v < (3<<5)) {
		output.Output([]byte{byte((v>>7)&0x7F) | 0x80})
	}
	output.Output([]byte{byte(v & 0x7F)})
}

// EncodeVLQUint encodes an unsigned integer to VLQ format
func EncodeVLQUint(output OutputBuffer, v uint32) {
	EncodeVLQInt(output, int32(v))
}

// DecodeVLQInt decodes a VLQ signed integer from the data slice
// This matches Klipper's decoding as implemented in Anchor
// The data slice is advanced past the consumed bytes
func DecodeVLQInt(data *[]byte) (int32, error) {
	if len(*data) == 0 {
		return 0, ErrBufferTooSmall
	}

	c := uint32((*data)[0])
	*data = (*data)[1:]

	v := c & 0x7F
	// Sign extension for negative numbers
	if (c & 0x60) == 0x60 {
		// Sign extend: convert -32 to uint32
		v |= ^uint32(0x1F)
	}

	// Read continuation bytes
	for c&0x80 != 0 {
		if len(*data) == 0 {
			return 0, ErrBufferTooSmall
		}
		c = uint32((*data)[0])
		*data = (*data)[1:]
		v = (v << 7) | (c & 0x7F)
	}

	return int32(v), nil
}

// DecodeVLQUint decodes a VLQ unsigned integer from the data slice
func DecodeVLQUint(data *[]byte) (uint32, error) {
	val, err := DecodeVLQInt(data)
	return uint32(val), err
}

// EncodeVLQ is a helper that returns encoded bytes (for backward compatibility)
func EncodeVLQ(v int32) []byte {
	output := NewScratchOutput()
	EncodeVLQInt(output, v)
	return output.Result()
}

// DecodeVLQ decodes a VLQ from byte slice without modifying the slice
// Returns the decoded value and number of bytes consumed
func DecodeVLQ(data []byte) (int32, int, error) {
	original := len(data)
	val, err := DecodeVLQInt(&data)
	if err != nil {
		return 0, 0, err
	}
	consumed := original - len(data)
	return val, consumed, nil
}

// EncodeVLQBytes encodes a byte array with length prefix
func EncodeVLQBytes(output OutputBuffer, data []byte) {
	EncodeVLQUint(output, uint32(len(data)))
	output.Output(data)
}

// DecodeVLQBytes decodes a length-prefixed byte array
func DecodeVLQBytes(data *[]byte) ([]byte, error) {
	length, err := DecodeVLQUint(data)
	if err != nil {
		return nil, err
	}
	if len(*data) < int(length) {
		return nil, ErrBufferTooSmall
	}
	result := (*data)[:length]
	*data = (*data)[length:]
	return result, nil
}

// EncodeVLQString encodes a string with length prefix
func EncodeVLQString(output OutputBuffer, s string) {
	bytes := []byte(s)
	EncodeVLQUint(output, uint32(len(bytes)))
	output.Output(bytes)
}

// DecodeVLQString decodes a length-prefixed string
func DecodeVLQString(data *[]byte) (string, error) {
	bytes, err := DecodeVLQBytes(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
