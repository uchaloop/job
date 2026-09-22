package job

import (
	"context"
	"errors"
	"time"
)

// Default budgets for the two sequential execution stages.
const (
	defaultTimeout             = time.Minute
	defaultErrorHandlerTimeout = time.Minute
)

// Runner executes one attempt per call to Run. It holds the resolved
// configuration and the middleware chain, built once, so a caller that runs the
// same work repeatedly - beat, for instance - pays for the composition once.
//
// A Runner keeps no state between attempts. Concurrent calls to Run are the
// caller's business: the runner serializes nothing, so they are safe only when
// the Func, its middleware and ErrorHandler are. Separate runners for logical jobs
// are simpler than one runner guarding shared state.
type Runner struct {
	fn                  Func
	timeout             time.Duration
	errorHandler        ErrorHandler
	errorHandlerTimeout time.Duration
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

	handlerTimeout := cfg.ErrorHandlerTimeout
	if handlerTimeout == 0 {
		handlerTimeout = defaultErrorHandlerTimeout
	}

	return &Runner{
		fn:                  chain(fn, s.middleware),
		timeout:             timeout,
		errorHandler:        s.errorHandler,
		errorHandlerTimeout: handlerTimeout,
	}, nil
}

// Run performs one attempt and reports how it went.
//
// It calls the chain at most once, synchronously, on the calling goroutine, and
// waits for it to return. There is no retry, no goroutine of its own and no
// delivery to an observability Handler: the caller owns the Result. An optional
// ErrorHandler processes a returned error before Run returns.
//
// The work is bounded by Timeout, and error processing by ErrorHandlerTimeout.
// Both deadlines cancel contexts without interrupting execution. A
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
	workDuration := time.Since(start)

	result := Result{
		Start:        start,
		WorkDuration: workDuration,
		Processed:    processed,
		Err:          err,
		Outcome:      outcomeOf(runCtx, err),
	}

	// Freeze the work outcome before error processing and release its timer.
	cancel()

	if err != nil && r.errorHandler != nil {
		result.ErrorHandlerCalled = true
		handlerCtx, cancelHandler := context.WithTimeout(context.WithoutCancel(ctx), r.errorHandlerTimeout)
		defer cancelHandler()
		handlerStart := time.Now()
		handlerErr := r.errorHandler(handlerCtx, err)
		result.ErrorHandlerDuration = time.Since(handlerStart)
		result.ErrorHandlerErr = errors.Join(handlerErr, handlerCtx.Err())
	}

	result.Duration = time.Since(start)

	return result
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
