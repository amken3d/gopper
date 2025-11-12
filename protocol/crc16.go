package protocol

// CRC16 calculates the CRC16 checksum for Klipper protocol messages
// This matches the implementation in Klipper and Anchor
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		b = b ^ uint8(crc&0xFF)
		b = b ^ (b << 4)
		b16 := uint16(b)
		crc = (b16<<8 | crc>>8) ^ (b16 >> 4) ^ (b16 << 3)
	}
	return crc
}
