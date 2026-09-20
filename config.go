package job

import (
	"time"

	"github.com/uchaloop/validate"
)

// Config is the configuration job declares. The env tags are inert strings: an
// application loads them with github.com/uchaloop/confmaker and provides the
// filled Config. job itself never reads the environment.
type Config struct {
	// Timeout bounds the work and middleware. Zero defaults to one minute;
	// negative values are invalid.
	//
	// The bound is cooperative: it cancels the attempt's context and cannot
	// interrupt the function. A Func that ignores cancellation holds its caller
	// for as long as it likes, and no outcome is reported until it returns.
	Timeout time.Duration `env:"TIMEOUT"`

	// ErrorHandlerTimeout bounds error processing after the work returns.
	// Zero defaults to one minute; negative values are invalid. The handler's
	// context retains caller values but not its deadline or cancellation.
	// This budget is additional to Timeout and matters during shutdown too.
	ErrorHandlerTimeout time.Duration `env:"ERROR_HANDLER_TIMEOUT"`
}

// SetDefaults establishes the values a deployment does not have to think about.
// confmaker calls it before the environment is applied, so a variable left
// unset keeps what is set here, and a generated .env.example carries the real
// default rather than a blank.
//
// A Config built in Go by hand does not go through it, which is why MakeRunner
// still treats a zero Timeout as the default. Both paths apply one constant.
func (c *Config) SetDefaults() {
	c.Timeout = defaultTimeout
	c.ErrorHandlerTimeout = defaultErrorHandlerTimeout
}

// ConfigName is the default instance name, "job": a loader such as confmaker
// reads JOB_TIMEOUT and JOB_ERROR_HANDLER_TIMEOUT by default.
func (Config) ConfigName() string { return "job" }

// Validate reports whether the Config is usable.
func (c Config) Validate() error {
	var errs validate.Errors

	errs.Require(c.Timeout >= 0, "timeout must be >= 0")
	errs.Require(c.ErrorHandlerTimeout >= 0, "error_handler_timeout must be >= 0")

	return errs.Err()
}
