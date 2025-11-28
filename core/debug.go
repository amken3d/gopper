package core

// DebugWriter is a function type for writing debug messages
type DebugWriter func(string)

var (
	// debugPrintln is the global debug print function (can be set by platform code)
	debugPrintln DebugWriter = func(s string) {} // No-op by default
)

// SetDebugWriter sets the platform-specific debug output function
// This allows platforms to redirect debug output to UART, USB, etc.
func SetDebugWriter(writer DebugWriter) {
	debugPrintln = writer
}

// DebugPrintln writes a debug message using the platform-specific writer
func DebugPrintln(msg string) {
	if debugPrintln != nil {
		debugPrintln(msg)
	}
}
