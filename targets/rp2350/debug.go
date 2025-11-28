//go:build rp2350

package main

import (
	"machine"
)

var (
	debugUART    *machine.UART
	debugEnabled bool
)

// InitDebugUART initializes UART1 on GPIO36 (TX) and GPIO37 (RX) for debugging
// Baud rate: 115200
func InitDebugUART() {
	debugUART = machine.UART1

	err := debugUART.Configure(machine.UARTConfig{
		BaudRate: 115200,
		TX:       machine.GPIO36, // UART1 TX
		RX:       machine.GPIO37, // UART1 RX
	})

	if err != nil {
		debugEnabled = false
		return
	}

	debugEnabled = true

	// Send a startup message
	DebugPrintln("=== RP2350 Debug UART Initialized ===")
	DebugPrintln("Baud: 115200, TX=GPIO36, RX=GPIO37")
	DebugPrintln("")
}

// DebugPrint writes a string to the debug UART (no newline)
func DebugPrint(s string) {
	if !debugEnabled || debugUART == nil {
		return
	}
	debugUART.Write([]byte(s))
}

// DebugPrintln writes a string to the debug UART with newline
func DebugPrintln(s string) {
	if !debugEnabled || debugUART == nil {
		return
	}
	debugUART.Write([]byte(s))
	debugUART.Write([]byte("\r\n"))
}

// DebugPrintf provides formatted printing to debug UART
// Limited format support: %s (string), %d/%u (int), %x (hex)
func DebugPrintf(format string, args ...interface{}) {
	if !debugEnabled || debugUART == nil {
		return
	}

	result := make([]byte, 0, 256)
	argIndex := 0

	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && argIndex < len(args) {
			switch format[i+1] {
			case 's': // String
				if str, ok := args[argIndex].(string); ok {
					result = append(result, []byte(str)...)
				}
				argIndex++
				i++ // Skip format character
			case 'd', 'u': // Integer
				if num, ok := args[argIndex].(int); ok {
					result = append(result, []byte(itoa(num))...)
				}
				argIndex++
				i++
			case 'x': // Hex
				if num, ok := args[argIndex].(int); ok {
					result = append(result, []byte(itoaHex(num))...)
				}
				argIndex++
				i++
			default:
				result = append(result, format[i])
			}
		} else {
			result = append(result, format[i])
		}
	}

	debugUART.Write(result)
}

// itoaHex converts int to hex string
func itoaHex(i int) string {
	if i == 0 {
		return "0x0"
	}

	const hexDigits = "0123456789abcdef"
	var buf [20]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = hexDigits[i&0xf]
		i >>= 4
	}

	// Add 0x prefix
	pos -= 2
	buf[pos] = '0'
	buf[pos+1] = 'x'

	return string(buf[pos:])
}
