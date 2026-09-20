package recovery_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/uchaloop/job"
	"github.com/uchaloop/job/middleware/recovery"
)

func TestMiddleware_TurnsAPanicIntoAResult(t *testing.T) {
	runner, err := job.MakeRunner(
		job.Config{},
		func(context.Context) (int, error) { panic("boom") },
		job.WithMiddleware(recovery.Middleware()),
	)
	if err != nil {
		t.Fatalf("MakeRunner: %v", err)
	}

	result := runner.Run(context.Background())

	if result.Outcome != job.OutcomePanic {
		t.Errorf("Outcome = %q, want %q", result.Outcome, job.OutcomePanic)
	}

	panicErr, ok := errors.AsType[*job.PanicError](result.Err)
	if !ok {
		t.Fatalf("Err = %v, want a *job.PanicError", result.Err)
	}
	if panicErr.Value != "boom" {
		t.Errorf("Value = %v, want boom", panicErr.Value)
	}
	// The stack is captured where the panic happened, which is the only place
	// it is still complete.
	if !strings.Contains(string(panicErr.Stack), "recovery_test") {
		t.Error("the stack does not reach the panicking function")
	}
}

func TestMiddleware_LogsWhenAsked(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	runner, err := job.MakeRunner(
		job.Config{},
		func(context.Context) (int, error) { panic("boom") },
		job.WithMiddleware(recovery.Middleware(recovery.WithLogger(logger))),
	)
	if err != nil {
		t.Fatalf("MakeRunner: %v", err)
	}

	runner.Run(context.Background())

	if !strings.Contains(buf.String(), "job panic recovered") {
		t.Errorf("nothing was logged: %q", buf.String())
	}
}

func TestMiddleware_LetsOrdinaryResultsThrough(t *testing.T) {
	runner, err := job.MakeRunner(
		job.Config{},
		func(context.Context) (int, error) { return 5, nil },
		job.WithMiddleware(recovery.Middleware(), nil),
	)
	if err != nil {
		t.Fatalf("MakeRunner: %v", err)
	}

	if result := runner.Run(context.Background()); result.Processed != 5 || result.Outcome != job.OutcomeOK {
		t.Errorf("result = %+v, want 5 processed and ok", result)
	}
}
