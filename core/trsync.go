// Trigger synchronization for multi-axis homing
// Implements Klipper's trsync protocol for coordinated endstop triggers
package core

import (
	"gopper/protocol"
)

// TriggerSync flags
const (
	TSF_CAN_TRIGGER = 1 << 0 // Trigger is enabled
	TSF_TRIGGERED   = 1 << 1 // Trigger has fired
)

// TriggerSignal represents a callback registered with a TriggerSync
type TriggerSignal struct {
	Callback func(reason uint8) // Called when trigger fires
	Next     *TriggerSignal
}

// TriggerSync coordinates multiple endstops during homing
type TriggerSync struct {
	OID           uint8          // Object ID
	Flags         uint8          // State flags (TSF_*)
	TriggerReason uint8          // Reason code for the trigger
	ExpireReason  uint8          // Reason code if timeout expires
	ReportTicks   uint32         // Interval for status reports
	ReportTimer   Timer          // Timer for periodic reports
	ExpireTimer   Timer          // Timer for timeout
	Signals       *TriggerSignal // Linked list of registered callbacks
}

// Global registry of trigger sync objects
var triggerSyncs = make(map[uint8]*TriggerSync)

// InitTriggerSyncCommands registers trsync-related commands
func InitTriggerSyncCommands() {
	// Command to set timeout for trigger synchronization
	RegisterCommand("trsync_start", "oid=%c report_clock=%u report_ticks=%u expire_reason=%c", handleTriggerSyncStart)

	// Command to set timeout for a trigger sync
	RegisterCommand("trsync_set_timeout", "oid=%c clock=%u", handleTriggerSyncSetTimeout)

	// Command to manually trigger a trsync
	RegisterCommand("trsync_trigger", "oid=%c reason=%c", handleTriggerSyncTrigger)

	// Response: trsync report sent to host
	RegisterResponse("trsync_state", "oid=%c can_trigger=%c trigger_reason=%c clock=%u")
}

// handleTriggerSyncStart starts a trigger synchronization session
// Format: trsync_start oid=%c report_clock=%u report_ticks=%u expire_reason=%c
func handleTriggerSyncStart(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	reportClock, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	reportTicks, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	expireReason, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get or create trigger sync object
	ts, exists := triggerSyncs[uint8(oid)]
	if !exists {
		ts = &TriggerSync{
			OID: uint8(oid),
		}
		triggerSyncs[uint8(oid)] = ts
	}

	// Reset state
	ts.Flags = TSF_CAN_TRIGGER
	ts.TriggerReason = 0
	ts.ExpireReason = uint8(expireReason)
	ts.ReportTicks = reportTicks

	// Schedule report timer
	if reportTicks > 0 {
		ts.ReportTimer.WakeTime = reportClock
		ts.ReportTimer.Handler = triggerSyncReportEvent
		ScheduleTimer(&ts.ReportTimer)
	}

	return nil
}

// handleTriggerSyncSetTimeout sets a timeout for trigger synchronization
// Format: trsync_set_timeout oid=%c clock=%u
func handleTriggerSyncSetTimeout(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	clock, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get trigger sync object
	ts, exists := triggerSyncs[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Schedule expire timer
	ts.ExpireTimer.WakeTime = clock
	ts.ExpireTimer.Handler = triggerSyncExpireEvent
	ScheduleTimer(&ts.ExpireTimer)

	return nil
}

// handleTriggerSyncTrigger manually triggers a trsync
// Format: trsync_trigger oid=%c reason=%c
func handleTriggerSyncTrigger(data *[]byte) error {
	oid, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	reason, err := protocol.DecodeVLQUint(data)
	if err != nil {
		return err
	}

	// Get trigger sync object
	ts, exists := triggerSyncs[uint8(oid)]
	if !exists {
		return nil // Silently ignore if not configured
	}

	// Trigger it
	TriggerSyncDoTrigger(ts, uint8(reason))

	return nil
}

// TriggerSyncDoTrigger fires a trigger synchronization event
// This is called by endstops when they detect a trigger condition
func TriggerSyncDoTrigger(ts *TriggerSync, reason uint8) {
	state := disableInterrupts()
	defer restoreInterrupts(state)

	// Check if we can trigger
	if (ts.Flags & TSF_CAN_TRIGGER) == 0 {
		return
	}

	// Mark as triggered
	ts.Flags &^= TSF_CAN_TRIGGER
	ts.Flags |= TSF_TRIGGERED
	ts.TriggerReason = reason

	// Call all registered signal callbacks
	signal := ts.Signals
	for signal != nil {
		if signal.Callback != nil {
			signal.Callback(reason)
		}
		signal = signal.Next
	}
}

// TriggerSyncAddSignal registers a callback with a trigger sync
func TriggerSyncAddSignal(ts *TriggerSync, callback func(reason uint8)) *TriggerSignal {
	state := disableInterrupts()
	defer restoreInterrupts(state)

	signal := &TriggerSignal{
		Callback: callback,
		Next:     ts.Signals,
	}
	ts.Signals = signal

	return signal
}

// triggerSyncReportEvent is the timer handler for periodic status reports
func triggerSyncReportEvent(t *Timer) uint8 {
	// Find the TriggerSync instance that owns this timer
	var ts *TriggerSync
	for _, tsPtr := range triggerSyncs {
		if tsPtr != nil && &tsPtr.ReportTimer == t {
			ts = tsPtr
			break
		}
	}

	if ts == nil {
		return SF_DONE
	}

	// Send report to host
	triggerSyncReport(ts)

	// Reschedule if still active
	if (ts.Flags & TSF_CAN_TRIGGER) != 0 {
		t.WakeTime = GetTime() + ts.ReportTicks
		return SF_RESCHEDULE
	}

	return SF_DONE
}

// triggerSyncExpireEvent is the timer handler for timeout expiration
func triggerSyncExpireEvent(t *Timer) uint8 {
	// Find the TriggerSync instance that owns this timer
	var ts *TriggerSync
	for _, tsPtr := range triggerSyncs {
		if tsPtr != nil && &tsPtr.ExpireTimer == t {
			ts = tsPtr
			break
		}
	}

	if ts == nil {
		return SF_DONE
	}

	// Trigger with expire reason
	TriggerSyncDoTrigger(ts, ts.ExpireReason)

	// Send final report
	triggerSyncReport(ts)

	return SF_DONE
}

// triggerSyncReport sends a status report to the host
func triggerSyncReport(ts *TriggerSync) {
	canTrigger := uint32(0)
	if (ts.Flags & TSF_CAN_TRIGGER) != 0 {
		canTrigger = 1
	}

	clock := GetTime()

	// Send trsync_state response
	SendResponse("trsync_state", func(output protocol.OutputBuffer) {
		protocol.EncodeVLQUint(output, uint32(ts.OID))
		protocol.EncodeVLQUint(output, canTrigger)
		protocol.EncodeVLQUint(output, uint32(ts.TriggerReason))
		protocol.EncodeVLQUint(output, clock)
	})
}

// GetTriggerSync retrieves a trigger sync by OID
func GetTriggerSync(oid uint8) (*TriggerSync, bool) {
	ts, exists := triggerSyncs[oid]
	return ts, exists
}
