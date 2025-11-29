//go:build rp2350

package main

import (
	"gopper/core"
	"runtime/volatile"
	"unsafe"
)

// RP2350 Timer peripheral memory map
// NOTE: RP2350 timer is at a DIFFERENT address than RP2040!
// - RP2040 TIMER: 0x40054000
// - RP2350 TIMER0: 0x400B0000
//
// Timer register offsets (from timerType struct in TinyGo):
// timeHW   @ 0x00 - Write to upper 32b
// timeLW   @ 0x04 - Write to lower 32b
// timeHR   @ 0x08 - Latched read from upper 32b
// timeLR   @ 0x0C - Latched read from lower 32b (latches timeHR)
// alarm[4] @ 0x10-0x1C
// armed    @ 0x20
// timeRawH @ 0x24 - Raw read from upper 32b
// timeRawL @ 0x28 - Raw read from lower 32b (what TinyGo uses)
const (
	timerBase     = 0x400B0000       // RP2350 TIMER0 base address
	timerTimeRawH = timerBase + 0x24 // Raw timer high (no latching)
	timerTimeRawL = timerBase + 0x28 // Raw timer low (no latching)
)

var (
	timerRawH = (*volatile.Register32)(unsafe.Pointer(uintptr(timerTimeRawH)))
	timerRawL = (*volatile.Register32)(unsafe.Pointer(uintptr(timerTimeRawL)))
)

// InitClock initializes the RP2350 hardware timer
// The RP2350 has a 64-bit microsecond timer at 1MHz (same as RP2040)
// Note: TinyGo's runtime already initializes the tick generators via clks.initTicks()
func InitClock() {
	// Wait for timer to stabilize after TinyGo's clock initialization
	// Read and discard a few values to ensure we get stable readings
	_ = timerRawL.Get()
	_ = timerRawL.Get()
	_ = timerRawL.Get()

	// Register MCU-specific constant
	core.RegisterConstant("MCU", "rp2350")
	core.RegisterConstant("CLOCK_FREQ", uint32(1000000)) // 1MHz
}

// GetHardwareTime reads the RP2350 hardware timer
// Returns the low 32 bits of the microsecond counter
func GetHardwareTime() uint32 {
	// Read the low 32 bits of the raw timer (same as TinyGo's timeRawL)
	return timerRawL.Get()
}

// GetHardwareUptime reads the full 64-bit RP2350 hardware timer
func GetHardwareUptime() uint64 {
	// Read both high and low parts
	// Must read high first, then low, then high again to detect rollover
	for {
		high1 := timerRawH.Get()
		low := timerRawL.Get()
		high2 := timerRawH.Get()

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
