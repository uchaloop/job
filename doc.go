// Package job runs one attempt at a piece of work and reports how it went.
//
// It is the execution half of a scheduled job, with no opinion about when the
// next attempt happens. A short-lived process started by a Kubernetes CronJob
// runs one attempt and exits; github.com/uchaloop/beat runs the same work
// repeatedly inside a long-lived process. Both use the same Runner, so the work,
// its middleware, its timeout and its classification are written once.
//
//	runner, err := job.MakeRunner(
//		job.Config{Timeout: 3 * time.Minute},
//		func(ctx context.Context) (int, error) {
//			return queue.ProcessBatch(ctx, 1000)
//		},
//		job.WithMiddleware(recovery.Middleware()),
//	)
//
//	result := runner.Run(ctx)
//
// # What one attempt means
//
// [Runner.Run] calls the chain at most once, synchronously, and waits for it to
// return. It never retries, never starts a goroutine of its own and never calls
// a [Handler]: the caller decides what to do with the [Result] and when to try
// again. Two attempts never overlap because the caller does not overlap them.
//
// # The timeout is cooperative
//
// Config.Timeout bounds the attempt's context. Like every deadline in Go it
// cancels; it cannot interrupt a running function. A [Func] must return when
// its context is done, and must wait for its own goroutines first. One that
// ignores cancellation holds its caller for as long as it likes, and no
// [Result] is reported until it returns.
//
// [Result.Outcome], not [Result.Err], says whether an attempt succeeded: a Func
// cut short by the deadline may still return a nil error, and that is not a
// success. The runner classifies the attempt before releasing its own context,
// so a deadline or a cancellation is visible even when the work stayed quiet
// about it.
//
// # Panics
//
// The runner does not recover them. A panic propagates to whoever called Run -
// the honest default for work that misbehaved. To turn one into a [PanicError]
// on the Result instead, add job/middleware/recovery.
//
// # Choosing which process runs the work
//
// job/assignment is an optional, pure policy that picks one cluster to own a
// scheduled point when the same deployment runs in several of them. It is off
// unless an application asks for it, and it decides nothing about execution.
package job
