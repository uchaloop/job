package job_test

import (
	"context"
	"slices"
	"testing"

	"github.com/uchaloop/job"
)

func TestMultiHandler_FansOutInOrderSkippingNil(t *testing.T) {
	var order []string

	mk := func(name string) job.Handler {
		return job.HandlerFunc(func(context.Context, job.Result) {
			order = append(order, name)
		})
	}

	job.MultiHandler(mk("a"), nil, mk("b")).Handle(context.Background(), job.Result{})

	if want := []string{"a", "b"}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestMultiHandler_Empty(t *testing.T) {
	// Must not panic with no handlers.
	job.MultiHandler().Handle(context.Background(), job.Result{})
}

func TestHandlerFunc_NilIsNoOp(t *testing.T) {
	var h job.HandlerFunc

	h.Handle(context.Background(), job.Result{})
}
