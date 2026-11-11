// Package clock provides time abstractions for FUI.
//
// By routing all time queries through this package, the rest of the codebase
// remains deterministic and easily testable without depending on the host's
// system clock.
package clock

import "time"

// Clock abstracts access to the current instant in time.
type Clock interface {
	// Now returns the current instant in time.
	Now() time.Time
}

// System provides real wall-clock time from the operating system.
// This is the only type in the application permitted to call time.Now.
type System struct{}

// NewSystem returns a Clock backed by the operating system clock.
func NewSystem() System {
	return System{}
}

// Now returns the current operating system time.
func (System) Now() time.Time {
	return time.Now()
}

// Fixed provides a deterministic, frozen instant in time for testing.
type Fixed struct {
	t time.Time
}

// NewFixed creates a Fixed clock initialized to the given time.
func NewFixed(t time.Time) *Fixed {
	return &Fixed{t: t}
}

// Now returns the fixed clock's current frozen time.
func (f *Fixed) Now() time.Time {
	return f.t
}

// Set updates the frozen time to t.
func (f *Fixed) Set(t time.Time) {
	f.t = t
}

// Advance shifts the frozen time forward by d.
func (f *Fixed) Advance(d time.Duration) {
	f.t = f.t.Add(d)
}
