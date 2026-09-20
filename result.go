package job

import (
	"fmt"
	"time"
)

// Outcome classifies how an attempt ended. It is the one label an
// observability adapter needs; Result.Err carries the detail behind it.
type Outcome string

const (
	// OutcomeOK is an attempt that finished and returned no error.
	OutcomeOK Outcome = "ok"

	// OutcomeError is an attempt that returned an error of its own.
	OutcomeError Outcome = "error"

	// OutcomePanic is an attempt whose panic the recovery middleware turned
	// into a *PanicError. Without that middleware a panic never reaches a
	// Result - it propagates to the caller.
	OutcomePanic Outcome = "panic"

	// OutcomeTimeout is an attempt cut off by Config.Timeout. It is reported
	// even when the Func swallowed the cancellation and returned no error,
	// because the deadline belongs to the runner and the runner can see it
	// fire.
	OutcomeTimeout Outcome = "timeout"

	// OutcomeCanceled is an attempt cut off by the caller's context - a
	// shutdown, most often. It is not a failure of the work, so it carries its
	// own label rather than adding to the error count every time a process
	// stops.
	OutcomeCanceled Outcome = "canceled"
)

// Result describes one completed attempt.
type Result struct {
	// Start is the wall-clock time the attempt began. It is the zero Time when
	// the caller's context was already done and the work was never called.
	Start time.Time

	// Duration is how long the chain took: the Func and its middleware, and
	// nothing else. Result handlers are not part of it.
	Duration time.Duration

	// Processed is the number of items the Func reported.
	Processed int

	// Err is the error the chain returned, or nil. It stays nil when a timeout
	// or a cancellation cut a Func short that chose to report no error, which
	// is why Outcome and not Err decides whether an attempt succeeded.
	Err error

	// Outcome classifies the attempt and is authoritative.
	Outcome Outcome
}

// PanicError wraps a value recovered from a panic in the work. The recovery
// middleware returns it as the attempt's error, so a handler can tell a panic
// from an ordinary error - Result.Outcome reads OutcomePanic - and inspect the
// recovered value and its stack.
//
// The runner does not recover panics; without the recovery middleware a panic
// propagates to whoever called Run.
type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("recovered from panic: %v", e.Value)
}
