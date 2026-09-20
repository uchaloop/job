package job

import (
	"context"
	"errors"
	"time"
)

// defaultTimeout bounds a single attempt when the configured Timeout is unset.
const defaultTimeout = time.Minute

// Runner executes one attempt per call to Run. It holds the resolved
// configuration and the middleware chain, built once, so a caller that runs the
// same work repeatedly - beat, for instance - pays for the composition once.
//
// A Runner keeps no state between attempts. Concurrent calls to Run are the
// caller's business: the runner serializes nothing, so they are safe only when
// the Func and its middleware are. Separate runners for separate logical jobs
// are simpler than one runner guarding shared state.
type Runner struct {
	fn      Func
	timeout time.Duration
}

// MakeRunner resolves cfg against opts and builds a Runner. fn is required.
func MakeRunner(cfg Config, fn Func, opts ...Option) (*Runner, error) {
	if fn == nil {
		return nil, errors.New("func is required")
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var s settings
	for _, o := range opts {
		if o != nil {
			o.apply(&s)
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Runner{fn: chain(fn, s.middleware), timeout: timeout}, nil
}

// Run performs one attempt and reports how it went.
//
// It calls the chain at most once, synchronously, on the calling goroutine, and
// waits for it to return. There is no retry, no goroutine of its own and no
// delivery to a handler: what to do with the Result, and when to attempt again,
// belong to the caller.
//
// The attempt is bounded by the configured timeout, as far as a context can
// bound anything - the deadline cancels the attempt, it cannot interrupt it. A
// caller's context that is already done skips the work entirely and reports why.
func (r *Runner) Run(ctx context.Context) Result {
	if err := ctx.Err(); err != nil {
		// Nothing ran, so there is nothing to measure: Start and Duration stay
		// zero rather than describing an attempt that never happened.
		return Result{Err: err, Outcome: outcomeOf(ctx, err)}
	}

	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	start := time.Now()
	processed, err := r.fn(runCtx)
	duration := time.Since(start)

	// Classified before the deferred cancel fires, so runCtx still tells the
	// truth about whether the deadline or the caller cut the attempt short.
	return Result{
		Start:     start,
		Duration:  duration,
		Processed: processed,
		Err:       err,
		Outcome:   outcomeOf(runCtx, err),
	}
}

// outcomeOf classifies a finished attempt. A panic outranks the rest because it
// is a defect and always worth surfacing. The deadline and the cancellation
// outrank the work's own error because both mean the attempt was cut short,
// whatever it managed to return on the way out.
func outcomeOf(ctx context.Context, err error) Outcome {
	if _, ok := errors.AsType[*PanicError](err); ok {
		return OutcomePanic
	}

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return OutcomeTimeout
	case errors.Is(ctx.Err(), context.Canceled):
		return OutcomeCanceled
	case err != nil:
		return OutcomeError
	}

	return OutcomeOK
}
