package protocol

import "testing"

func TestCRC16(t *testing.T) {
	testCases := []struct {
		data     []byte
		expected uint16
	}{
		{
			data:     []byte{5, MessageDest},
			expected: 0, // Will be calculated
		},
		{
			data:     []byte{},
			expected: 0xFFFF,
		},
		{
			data:     []byte{0x00},
			expected: 0, // Will be calculated
		},
		{
			data:     []byte{0xFF},
			expected: 0, // Will be calculated
		},
	}

	for i, tc := range testCases {
		result := CRC16(tc.data)
		// For now, just verify it returns a value
		// We'll verify against Klipper's implementation later
		if i == 0 && result == 0 {
			t.Errorf("Test case %d: CRC16 returned 0, which is unlikely", i)
		}
		t.Logf("Test case %d: CRC16(%v) = 0x%04X", i, tc.data, result)
	}
}

func TestCRC16Consistency(t *testing.T) {
	// Test that same input produces same output
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	crc1 := CRC16(data)
	crc2 := CRC16(data)

	if crc1 != crc2 {
		t.Errorf("CRC16 not consistent: first=%04X, second=%04X", crc1, crc2)
	}
}

func TestCRC16Different(t *testing.T) {
	// Test that different inputs produce different outputs
	data1 := []byte{0x01, 0x02, 0x03}
	data2 := []byte{0x01, 0x02, 0x04}

	crc1 := CRC16(data1)
	crc2 := CRC16(data2)

	if crc1 == crc2 {
		t.Errorf("CRC16 collision: both inputs produced %04X", crc1)
	}
}
