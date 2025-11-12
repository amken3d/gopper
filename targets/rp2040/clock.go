//go:build rp2040 || rp2350

package main

import (
	"gopper/core"
	"runtime/volatile"
	"unsafe"
)

// RP2040/RP2350 Timer peripheral memory map
const (
	timerBase     = 0x40054000
	timerTIMERAWH = timerBase + 0x08 // Raw timer high word
	timerTIMERAWL = timerBase + 0x0C // Raw timer low word
)

var (
	timerRAWH = (*volatile.Register32)(unsafe.Pointer(uintptr(timerTIMERAWH)))
	timerRAWL = (*volatile.Register32)(unsafe.Pointer(uintptr(timerTIMERAWL)))
)

// InitClock initializes the RP2040 hardware timer
// The RP2040 has a 64-bit microsecond timer at 1MHz
func InitClock() {
	// RP2040 timer runs at 1MHz by default
	// Register MCU-specific constant
	core.RegisterConstant("MCU", "rp2040")
	core.RegisterConstant("CLOCK_FREQ", uint32(1000000)) // 1MHz
}

// GetHardwareTime reads the RP2040 hardware timer
// Returns the low 32 bits of the microsecond counter
func GetHardwareTime() uint32 {
	// Read the low 32 bits of the timer
	return timerRAWL.Get()
}

// GetHardwareUptime reads the full 64-bit RP2040 hardware timer
func GetHardwareUptime() uint64 {
	// Read both high and low parts
	// Must read high first, then low, then high again to detect rollover
	for {
		high1 := timerRAWH.Get()
		low := timerRAWL.Get()
		high2 := timerRAWH.Get()

		// If high didn't change, we got a consistent reading
		if high1 == high2 {
			return (uint64(high1) << 32) | uint64(low)
		}
		// Otherwise retry (rollover happened during read)
	}
}

// UpdateSystemTime updates the core timer with hardware time
// Called from main loop or timer interrupt
func UpdateSystemTime() {
	core.SetTime(GetHardwareTime())
}
