# job

One attempt at a piece of work, and nothing about when the next one happens.

A short-lived process started by a Kubernetes CronJob runs one attempt and
exits. [beat](https://github.com/uchaloop/beat) runs the same work repeatedly
inside a long-lived process. Both drive the same `Runner`, so the work, its
middleware, its timeout and its classification are written once.

- **One attempt per call** - no retry, no goroutine of its own, no handler call.
- **A cooperative timeout** - it cancels the attempt's context and says so even
  when the work stayed quiet about it.
- **No dependencies** beyond the standard library and `uchaloop/validate`.

```bash
go get github.com/uchaloop/job
```

## Quick start

```go
runner, err := job.MakeRunner(
    job.Config{Timeout: 3 * time.Minute},
    func(ctx context.Context) (int, error) {
        return queue.ProcessBatch(ctx, 1000)
    },
    job.WithMiddleware(recovery.Middleware()),
)
if err != nil {
    return err
}

result := runner.Run(workCtx)
handler.Handle(observeCtx, result)
// The exit code follows result.Outcome, after cleanup.
```

## The contract

`Run` calls the chain at most once, synchronously, and waits for it to return.
A caller's context that is already done skips the work entirely and reports why,
measuring nothing.

| Field | Meaning |
|---|---|
| `Start`, `Duration` | When the chain ran and for how long. Handlers are not part of it |
| `Processed` | What the work reported, kept even when it then failed |
| `Err` | The chain's error, which stays nil when a timeout cut a quiet Func short |
| `Outcome` | `ok`, `error`, `panic`, `timeout` or `canceled` - authoritative |

The timeout is cooperative, as every deadline in Go is: it cancels the context
and cannot interrupt the function. Work must return when its context is done and
must wait for its own goroutines first. Work that ignores cancellation holds its
caller for as long as it likes, and no result is reported until it returns.

Outcome, not `Err`, decides whether an attempt succeeded: a Func cut short by the
deadline may still return a nil error, and that is not a success.

## Panics

They propagate by default. To turn one into a `*job.PanicError` on the result
instead, add `job/middleware/recovery`:

```go
job.WithMiddleware(recovery.Middleware(recovery.WithLogger(logger)))
```

It protects only the wrapped work - not a handler, not goroutines the work
started.

## Choosing which process runs the work

`job/assignment` is an optional, pure policy for a deployment that runs in
several clusters: one cluster owns each scheduled point, and all of its replicas
run it.

```go
rotation, err := assignment.MakeRotation(assignment.Config{
    Clusters: []string{"el", "xc", "dm"},
    Current:  os.Getenv("CLUSTER"),
    Period:   4 * time.Hour,
})

decision, err := rotation.Decide(assignment.Invocation{ScheduledFor: point})
```

It reads no clock and keeps no state, so every process computes the same answer
from the same configuration - nothing is coordinated at run time. It decides who
should try, not that anyone did: a cluster that is down leaves its points
unserved, and no other cluster takes over. Work must survive a skipped attempt.

The caller passes the **scheduled point**, never the current time, so a pod that
starts late and a retry of the same scheduled Job decide as the original point
did.

Changing the cluster list or the period changes who owns which point, and
nothing checks that every process agrees: a rolling update produces duplicates
and gaps at once. Change the topology by restarting every pod in every cluster
and accept the pause - there is no hot reconfiguration, by choice.

## Configuration

| Field | Default env name | Default |
|---|---|---|
| `Timeout` | `JOB_TIMEOUT` | `1m` |

`Config` names its default instance `job`, so `confx.Provide[job.Config]()`
reads `JOB_TIMEOUT`.

## Uber Fx

`job/jobfx` is the only package here that imports Fx, so a process that builds
its Runner by hand never compiles it in. It ships in this module for now - the
Fx requirement is therefore in `go.mod` and in your build list; a split into its
own module is left for a later release.

It provides the Runner from the container and runs nothing: a one-shot process
owns its own sequence.

```go
app.Start(startupCtx)          // dependencies first
result := runner.Run(workCtx)  // one attempt
handler.Handle(observeCtx, result)
app.Stop(cleanupCtx)           // a fresh budget, not the cancelled work context
os.Exit(code(result))
```

Running the work inside `OnStart` would fold its duration into the startup
timeout, and `fx.Shutdowner` would report an ordinary completion as a
termination signal. See the package's `ExampleModule` for the whole sequence,
including the cluster decision and the exit code.

## Documentation

The contract, the outcomes and the reasons behind them are in the package
documentation:
**[pkg.go.dev/github.com/uchaloop/job](https://pkg.go.dev/github.com/uchaloop/job)**.

## License

[MIT](LICENSE)
