package jano

// Option configures a new Jano instance.
type Option func(*Jano)

// WithMethodNotAllowed enables HTTP 405 responses with an Allow header.
// By default, unregistered methods use the not-found handler for compatibility.
func WithMethodNotAllowed() Option {
	return func(j *Jano) { j.methodNotAllowed = true }
}
