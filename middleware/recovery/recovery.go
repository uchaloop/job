/*
Package recovery provides a job.Middleware that recovers from a panic in the
work and turns it into a *job.PanicError, so one bad attempt does not take the
process with it.

The job runner deliberately does not recover panics: a panic that propagates is
the honest outcome for code that misbehaved, and a supervisor restarts what it
kills. That default is wrong for work that touches input it does not control,
where a single malformed record should not stop every later attempt. This
middleware is how that choice is made explicitly rather than by default.

	job.WithMiddleware(recovery.Middleware(recovery.WithLogger(logger)))

The recovered value and its stack are reported as a *job.PanicError in the
attempt's error, and the Result reads job.OutcomePanic, so a handler tells a
panic from an ordinary failure without unwrapping anything. The value itself is
reachable with errors.AsType[*job.PanicError](result.Err). [WithLogger]
additionally logs it where it happened, with the stack, which is the only place
the stack is still complete.
*/
package recovery

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/uchaloop/job"
)

type settings struct {
	log *slog.Logger
}

// Option configures the middleware built by Middleware.
type Option func(*settings)

// WithLogger logs each recovered panic, with its stack, at Error level. A nil
// logger (the default) disables logging.
func WithLogger(l *slog.Logger) Option {
	return func(s *settings) { s.log = l }
}

// Middleware recovers from a panic in the wrapped work so the caller keeps
// going. The panic becomes the attempt's error - always a *job.PanicError
// carrying the value and stack, so downstream handlers can classify it - and,
// if WithLogger is set, is logged with its stack.
func Middleware(opts ...Option) job.Middleware {
	var s settings
	for _, o := range opts {
		if o != nil {
			o(&s)
		}
	}

	return func(next job.Func) job.Func {
		return func(ctx context.Context) (processed int, err error) {
			defer func() {
				if p := recover(); p != nil {
					stack := debug.Stack()

					if s.log != nil {
						s.log.Error("job panic recovered", "panic", p, "stack", string(stack))
					}

					err = &job.PanicError{Value: p, Stack: stack}
				}
			}()

			return next(ctx)
		}
	}
}
