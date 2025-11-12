package core

// itoa converts an integer to a string without using fmt package
// This is a lightweight alternative for embedded systems
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	// Count digits
	temp := n
	digits := 0
	for temp > 0 {
		digits++
		temp /= 10
	}

	// Add space for negative sign
	if negative {
		digits++
	}

	// Build string from right to left
	buf := make([]byte, digits)
	pos := digits - 1

	for n > 0 {
		buf[pos] = byte('0' + n%10)
		n /= 10
		pos--
	}

	if negative {
		buf[0] = '-'
	}

	return string(buf)
}

// utoa converts an unsigned integer to a string
func utoa(n uint32) string {
	if n == 0 {
		return "0"
	}

	// Count digits
	temp := n
	digits := 0
	for temp > 0 {
		digits++
		temp /= 10
	}

	// Build string from right to left
	buf := make([]byte, digits)
	pos := digits - 1

	for n > 0 {
		buf[pos] = byte('0' + n%10)
		n /= 10
		pos--
	}

	return string(buf)
}

// valueToString converts a value to string representation
// Handles the most common types used in constants
func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return itoa(val)
	case int32:
		return itoa(int(val))
	case int64:
		return itoa(int(val))
	case uint:
		return utoa(uint32(val))
	case uint32:
		return utoa(val)
	case uint64:
		return utoa(uint32(val))
	default:
		// Fallback for unknown types - return empty string
		// In production firmware, all types should be known
		return ""
	}
}
