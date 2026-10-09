package jano

import (
	"net/http"
	"sync"
	"sync/atomic"
)

// Middleware wraps an HTTP handler. The first middleware is the outermost.
type Middleware = func(http.Handler) http.Handler

// Jano is an HTTP router. Configuration changes are synchronized and published
// as immutable routing snapshots. A request in progress may use the previous
// snapshot. Handlers and middleware must synchronize their own shared state.
// A Jano must be created with New and must not be copied after first use.
type Jano struct {
	mu               sync.RWMutex
	compileMu        sync.Mutex
	routes           map[routeKey]route
	patterns         map[patternKey]string
	middlewares      []Middleware
	notFound         http.Handler
	errorHandler     ErrorHandler
	methodNotAllowed bool
	generation       uint64
	compiled         atomic.Pointer[routerState]
}

// New creates a router without mandatory middleware or external dependencies.
func New(options ...Option) *Jano {
	j := &Jano{
		routes:       make(map[routeKey]route),
		patterns:     make(map[patternKey]string),
		notFound:     http.HandlerFunc(http.NotFound),
		errorHandler: DefaultErrorHandler,
	}
	for _, option := range options {
		if option != nil {
			option(j)
		}
	}
	return j
}

// Get registers a GET handler, replacing an identical method and path.
func (j *Jano) Get(path string, handler http.HandlerFunc) { j.Handle(http.MethodGet, path, handler) }

// Post registers a POST handler.
func (j *Jano) Post(path string, handler http.HandlerFunc) { j.Handle(http.MethodPost, path, handler) }

// Put registers a PUT handler.
func (j *Jano) Put(path string, handler http.HandlerFunc) { j.Handle(http.MethodPut, path, handler) }

// Delete registers a DELETE handler.
func (j *Jano) Delete(path string, handler http.HandlerFunc) {
	j.Handle(http.MethodDelete, path, handler)
}

// Patch registers a PATCH handler.
func (j *Jano) Patch(path string, handler http.HandlerFunc) {
	j.Handle(http.MethodPatch, path, handler)
}

// Options registers an OPTIONS handler.
func (j *Jano) Options(path string, handler http.HandlerFunc) {
	j.Handle(http.MethodOptions, path, handler)
}

// Head registers a HEAD handler. GET does not implicitly register HEAD.
func (j *Jano) Head(path string, handler http.HandlerFunc) { j.Handle(http.MethodHead, path, handler) }

// Handle registers any HTTP handler and optional route-specific middleware.
// Invalid or ambiguous patterns panic; use Register to receive an error instead.
func (j *Jano) Handle(method, path string, handler http.Handler, middleware ...Middleware) {
	if err := j.Register(method, path, handler, middleware...); err != nil {
		panic(err)
	}
}

// Register validates and registers a standard HTTP handler. Identical method and
// path registrations replace the previous route. Structurally identical patterns
// with different parameter names are rejected. Patterns use {name} for a segment
// and {name...} for a final catch-all. Method names must be valid HTTP tokens.
func (j *Jano) Register(method, path string, handler http.Handler, middleware ...Middleware) error {
	return j.register(method, path, handler, nil, middleware)
}

// HandleContext registers an error-returning handler with request helpers.
// Invalid registrations panic; use RegisterContext to receive an error instead.
func (j *Jano) HandleContext(method, path string, handler HandlerFunc, middleware ...Middleware) {
	if err := j.RegisterContext(method, path, handler, middleware...); err != nil {
		panic(err)
	}
}

// RegisterContext validates and registers an error-returning handler.
func (j *Jano) RegisterContext(method, path string, handler HandlerFunc, middleware ...Middleware) error {
	return j.register(method, path, nil, handler, middleware)
}

// Use adds global middleware in registration order. Middleware runs on matched
// routes only, preserving the legacy fallback behavior. Middleware factories are
// applied when compiling a snapshot, not on every request. Factories should be
// side-effect free: configuration changes can invoke them again.
func (j *Jano) Use(middleware Middleware) {
	if middleware == nil {
		panic("jano: nil middleware")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.middlewares = append(j.middlewares, middleware)
	j.invalidate()
}

// NotFound sets the fallback for unmatched paths and methods. Global middleware
// is bypassed. The handler must write its own HTTP status.
func (j *Jano) NotFound(handler http.HandlerFunc) {
	if handler == nil {
		panic("jano: nil not-found handler")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.notFound = handler
	j.invalidate()
}

// SetErrorHandler replaces the error policy for all context handlers.
func (j *Jano) SetErrorHandler(handler ErrorHandler) {
	if handler == nil {
		panic("jano: nil error handler")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.errorHandler = handler
	j.invalidate()
}

// Router returns the router as an http.Handler, including later registrations.
func (j *Jano) Router() http.Handler { return j }

// ServeHTTP implements http.Handler using an immutable routing snapshot.
func (j *Jano) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state := j.snapshot()
	entry, parts := state.find(r.Method, r.URL.Path)
	if entry == nil {
		if state.methodNotAllowed {
			if allow := state.allowedMethods(r.URL.Path); allow != "" {
				w.Header().Set("Allow", allow)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
		}
		state.notFound.ServeHTTP(w, r)
		return
	}
	if len(entry.params) > 0 {
		r = withParams(r, entry.params, parts)
	}
	entry.handler.ServeHTTP(w, r)
}

func (j *Jano) invalidate() {
	j.generation++
	j.compiled.Store(nil)
}

func (j *Jano) snapshot() *routerState {
	if state := j.compiled.Load(); state != nil {
		return state
	}
	// Prevent concurrent first requests from rebuilding the same snapshot.
	j.compileMu.Lock()
	defer j.compileMu.Unlock()
	if state := j.compiled.Load(); state != nil {
		return state
	}
	j.mu.RLock()
	generation := j.generation
	routes := make([]route, 0, len(j.routes))
	for _, route := range j.routes {
		routes = append(routes, route)
	}
	middleware := append([]Middleware(nil), j.middlewares...)
	fallback, errorHandler, methodNotAllowed := j.notFound, j.errorHandler, j.methodNotAllowed
	j.mu.RUnlock()
	// Invoke application-supplied factories outside the configuration lock.
	state := compileRouter(routes, middleware, fallback, errorHandler, methodNotAllowed)
	j.mu.Lock()
	if generation == j.generation {
		j.compiled.Store(state)
	}
	j.mu.Unlock()
	return state
}
