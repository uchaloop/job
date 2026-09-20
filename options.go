package job

// settings holds the resolved options for one Runner.
type settings struct {
	middleware []Middleware
}

// Option customizes a Runner built by MakeRunner.
type Option interface{ apply(*settings) }

type optionFunc func(*settings)

func (f optionFunc) apply(s *settings) { f(s) }

// WithMiddleware wraps the work. The first middleware is the outermost layer
// and therefore runs first, which is the order the call reads in. Nil entries
// are skipped, and the chain is composed once, when the Runner is built.
func WithMiddleware(mw ...Middleware) Option {
	return optionFunc(func(s *settings) { s.middleware = append(s.middleware, mw...) })
}
