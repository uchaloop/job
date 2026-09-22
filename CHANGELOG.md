# Changelog

## [0.3.0] - 2026-09-22

### Breaking changes

- Change `Func` and `Result.Processed` to `int64` processed-item counts. The
  runner preserves returned values unchanged; callers should report nonnegative counts.
- Change `assignment.Decision.Slot` to `uint64`. Replace `OwnedBetween` with
  `OwnedAfter(after, count uint64) uint64`, using overflow-safe constant-time
  counting without adding a slot number to a count. The supported time range
  of `Decide` is unchanged.

### Added

- Add `Result.ErrorHandlerCalled` to distinguish an invoked, zero-duration
  error callback from an attempt without error processing.

### Documentation

- Clarify that topology changes require stopping all old participants and
  waiting for their attempts before starting the new configuration.
- Clarify that `PanicError` retains its stack without requiring immediate logging.
- Refresh the README logo with an SVG.

## [0.2.1] - 2026-09-20

- Add logo

## [0.2.0] - 2026-09-20

- Add optional WithErrorHandler with an independent ErrorHandlerTimeout
  (JOB_ERROR_HANDLER_TIMEOUT, default 1m). Only returned work errors trigger it.
- Preserve work Err/Outcome and expose ErrorHandlerErr separately, including
  expired handler contexts. Duration now includes both stages; WorkDuration and
  ErrorHandlerDuration provide their individual timings.

- **Breaking:** the Fx adapter now lives in `github.com/uchaloop/jobfx`.
  Use that independent module; this repository contains only the core library.
- Updated documentation and CI for independent core and adapter releases.

## [0.1.0] - 2026-09-20

Initial release. Execution split out of beat so one attempt means the same
whether a scheduler or a one-shot process runs it.

### Added

- `Func`, `Middleware` and `Runner`: one attempt per `Run`, synchronous, with
  the middleware chain composed once. No retry, no goroutine of its own, no
  handler call - the caller owns the result and the next attempt.
- `Config.Timeout`, read as `JOB_TIMEOUT`, defaulting to one minute. It is
  cooperative: it cancels the attempt's context and cannot interrupt it.
- `Result` and `Outcome` (`ok`, `error`, `panic`, `timeout`, `canceled`).
  Outcome is authoritative, so a Func cut short by the deadline is not a success
  just because it returned no error. A context that is already done skips the
  work and measures nothing.
- `Handler`, `HandlerFunc` and `MultiHandler`, which the Runner never calls
  itself, so there is no hidden second delivery.
- `PanicError` and `job/middleware/recovery`, which turns a panic in the wrapped
  work into a result rather than letting it reach the caller.
- `job/assignment`: an optional, pure cluster rotation that names one owner per
  scheduled point. It reads no clock, keeps no state and decides from the
  scheduled point rather than the current time, so a late start or a retry keeps
  the original owner. `OwnedBetween` counts a range of slots arithmetically, so
  a clock that jumped a day does not cost a day of iterations.
- `job/jobfx`, the Fx wiring, kept to one package so nothing else imports Fx. It
  provides the Runner from the container and runs nothing: a one-shot process
  owns the explicit sequence of start, run, observe, clean up and exit. Its
  `ExampleModule` walks through the whole of it.

[0.1.0]: https://github.com/uchaloop/job/releases/tag/v0.1.0

[0.2.0]: https://github.com/uchaloop/job/compare/v0.1.0...v0.2.0

[0.2.1]: https://github.com/uchaloop/job/compare/v0.2.0...v0.2.1

[0.3.0]: https://github.com/uchaloop/job/compare/v0.2.1...v0.3.0
