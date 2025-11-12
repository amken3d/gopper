package protocol

import (
	"testing"
)

func TestVLQEncodeDecodeInt(t *testing.T) {
	testCases := []int32{
		0,
		1,
		-1,
		127,
		-127,
		128,
		-128,
		255,
		-255,
		1000,
		-1000,
		65535,
		-65535,
		1000000,
		-1000000,
	}

	for _, expected := range testCases {
		output := NewScratchOutput()
		EncodeVLQInt(output, expected)
		encoded := output.Result()

		data := encoded
		decoded, err := DecodeVLQInt(&data)
		if err != nil {
			t.Errorf("Failed to decode VLQ for value %d: %v", expected, err)
			continue
		}

		if decoded != expected {
			t.Errorf("VLQ mismatch: expected %d, got %d (encoded as %v)", expected, decoded, encoded)
		}

		if len(data) != 0 {
			t.Errorf("VLQ decode didn't consume all bytes for value %d: %d bytes remaining", expected, len(data))
		}
	}
}

func TestVLQEncodeDecodeUint(t *testing.T) {
	testCases := []uint32{
		0,
		1,
		127,
		128,
		255,
		1000,
		65535,
		1000000,
	}

	for _, expected := range testCases {
		output := NewScratchOutput()
		EncodeVLQUint(output, expected)
		encoded := output.Result()

		data := encoded
		decoded, err := DecodeVLQUint(&data)
		if err != nil {
			t.Errorf("Failed to decode VLQ for value %d: %v", expected, err)
			continue
		}

		if decoded != expected {
			t.Errorf("VLQ mismatch: expected %d, got %d (encoded as %v)", expected, decoded, encoded)
		}
	}
}

func TestVLQBytes(t *testing.T) {
	testCases := [][]byte{
		{},
		{0x01},
		{0x01, 0x02, 0x03},
		{0xFF, 0xFE, 0xFD},
		make([]byte, 50), // Moderate array (within 64-byte message limit)
	}

	for i, expected := range testCases {
		output := NewScratchOutput()
		EncodeVLQBytes(output, expected)
		encoded := output.Result()

		data := encoded
		decoded, err := DecodeVLQBytes(&data)
		if err != nil {
			t.Errorf("Test case %d: Failed to decode bytes: %v", i, err)
			continue
		}

		if len(decoded) != len(expected) {
			t.Errorf("Test case %d: Length mismatch: expected %d, got %d", i, len(expected), len(decoded))
			continue
		}

		for j := range expected {
			if decoded[j] != expected[j] {
				t.Errorf("Test case %d: Byte mismatch at index %d: expected %d, got %d", i, j, expected[j], decoded[j])
			}
		}
	}
}

func TestVLQString(t *testing.T) {
	testCases := []string{
		"",
		"hello",
		"Hello, World!",
		"Special chars: !@#$%^&*()",
	}

	for _, expected := range testCases {
		output := NewScratchOutput()
		EncodeVLQString(output, expected)
		encoded := output.Result()

		data := encoded
		decoded, err := DecodeVLQString(&data)
		if err != nil {
			t.Errorf("Failed to decode string '%s': %v", expected, err)
			continue
		}

		if decoded != expected {
			t.Errorf("String mismatch: expected '%s', got '%s'", expected, decoded)
		}
	}
}

func TestVLQBufferTooSmall(t *testing.T) {
	// Test decoding with insufficient data
	data := []byte{0x80} // Continuation byte but no following byte
	_, err := DecodeVLQInt(&data)
	if err != ErrBufferTooSmall {
		t.Errorf("Expected ErrBufferTooSmall, got %v", err)
	}
}
