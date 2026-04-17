// Package lasterror owns the centralized error sink consumed by the tray's
// "Last error" menu slot. Subsystems publish their most recent error via
// Set; the tray's periodic refresher reads it via Get and formats it for
// display. This is the only shared-mutable state between subsystems and
// the UI thread, so reads and writes go through atomic.Value to keep the
// UI thread lock-free (per architecture §Error Handling — tray surfaces
// "Last error" as a grayed-out menu item when non-nil).
package lasterror

import (
	"sync/atomic"
)

// LastError holds the most recent error from any subsystem (tray, hotkey,
// overlay, etc.). Stored as an atomic.Value so concurrent writes from
// subsystem goroutines do not race the tray's 100ms refresher goroutine.
// Contains error or nil.
var LastError atomic.Value

// Set stores err into LastError. Safe for concurrent use from any goroutine.
// Passing nil explicitly clears the slot — callers that want to un-set the
// error (e.g., after a successful Undo) should Set(nil).
func Set(err error) {
	// atomic.Value cannot Store a nil typed value directly (it would panic
	// with "inconsistently typed value"). Wrap as an interface{} holder:
	// storing a typed-nil error is still a non-nil interface{}, so Load
	// will later return an interface holding a nil error value. Get()
	// below unwraps that safely.
	LastError.Store(errorHolder{err: err})
}

// Get returns the most recently Set error, or nil if none has been set or
// the last Set was Set(nil). Safe for concurrent use from any goroutine.
func Get() error {
	v := LastError.Load()
	if v == nil {
		return nil
	}
	return v.(errorHolder).err
}

// errorHolder wraps error in a concrete struct type so atomic.Value's
// "must Store the same concrete type on every call" contract is satisfied
// even when callers pass typed-nil errors (raw `error(nil)` Store would
// race with non-nil error Stores on some architectures).
type errorHolder struct {
	err error
}
