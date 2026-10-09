package jano

import (
	"net/http"
	"strings"
)

// RouteGroup scopes a path prefix and middleware. Group middleware is captured
// when a route is registered; Use affects future registrations, including child
// groups. Global Jano middleware remains dynamic. Do not copy a RouteGroup.
type RouteGroup struct {
	jano       *Jano
	parent     *RouteGroup
	prefix     string
	middleware []Middleware
}

// Group creates a group. Prefixes must be empty or begin with a slash.
func (j *Jano) Group(prefix string, middleware ...Middleware) *RouteGroup {
	return newGroup(j, nil, prefix, middleware)
}

// Group creates a nested group inheriting the parent prefix and middleware.
func (g *RouteGroup) Group(prefix string, middleware ...Middleware) *RouteGroup {
	return newGroup(g.jano, g, prefix, middleware)
}

func newGroup(j *Jano, parent *RouteGroup, prefix string, middleware []Middleware) *RouteGroup {
	if prefix != "" {
		if _, _, err := parsePattern(prefix); err != nil {
			panic(err)
		}
	}
	for _, m := range middleware {
		if m == nil {
			panic("jano: nil group middleware")
		}
	}
	if parent != nil {
		prefix = strings.TrimSuffix(parent.prefix, "/") + prefix
	}
	return &RouteGroup{jano: j, parent: parent, prefix: strings.TrimSuffix(prefix, "/"), middleware: append([]Middleware(nil), middleware...)}
}

// Use adds middleware for subsequently registered routes in this group.
func (g *RouteGroup) Use(middleware Middleware) {
	if middleware == nil {
		panic("jano: nil group middleware")
	}
	g.jano.mu.Lock()
	defer g.jano.mu.Unlock()
	g.middleware = append(g.middleware, middleware)
}

func (g *RouteGroup) inheritedMiddleware() []Middleware {
	var groups []*RouteGroup
	for current := g; current != nil; current = current.parent {
		groups = append(groups, current)
	}
	g.jano.mu.RLock()
	defer g.jano.mu.RUnlock()
	var middleware []Middleware
	for i := len(groups) - 1; i >= 0; i-- {
		middleware = append(middleware, groups[i].middleware...)
	}
	return middleware
}

// Register registers a standard handler under the group's prefix.
func (g *RouteGroup) Register(method, path string, handler http.Handler, middleware ...Middleware) error {
	if _, _, err := parsePattern(path); err != nil {
		return err
	}
	all := append(g.inheritedMiddleware(), middleware...)
	return g.jano.Register(method, g.prefix+path, handler, all...)
}

// Handle registers a standard handler and panics on invalid configuration.
func (g *RouteGroup) Handle(method, path string, handler http.Handler, middleware ...Middleware) {
	if err := g.Register(method, path, handler, middleware...); err != nil {
		panic(err)
	}
}

// RegisterContext registers an error-returning handler under the group's prefix.
func (g *RouteGroup) RegisterContext(method, path string, handler HandlerFunc, middleware ...Middleware) error {
	if _, _, err := parsePattern(path); err != nil {
		return err
	}
	all := append(g.inheritedMiddleware(), middleware...)
	return g.jano.RegisterContext(method, g.prefix+path, handler, all...)
}

// HandleContext registers a context handler and panics on invalid configuration.
func (g *RouteGroup) HandleContext(method, path string, handler HandlerFunc, middleware ...Middleware) {
	if err := g.RegisterContext(method, path, handler, middleware...); err != nil {
		panic(err)
	}
}

// Get registers a GET handler.
func (g *RouteGroup) Get(path string, handler http.HandlerFunc) {
	g.Handle(http.MethodGet, path, handler)
}

// Post registers a POST handler.
func (g *RouteGroup) Post(path string, handler http.HandlerFunc) {
	g.Handle(http.MethodPost, path, handler)
}

// Put registers a PUT handler.
func (g *RouteGroup) Put(path string, handler http.HandlerFunc) {
	g.Handle(http.MethodPut, path, handler)
}

// Delete registers a DELETE handler.
func (g *RouteGroup) Delete(path string, handler http.HandlerFunc) {
	g.Handle(http.MethodDelete, path, handler)
}

// Patch registers a PATCH handler.
func (g *RouteGroup) Patch(path string, handler http.HandlerFunc) {
	g.Handle(http.MethodPatch, path, handler)
}

// Options registers an OPTIONS handler.
func (g *RouteGroup) Options(path string, handler http.HandlerFunc) {
	g.Handle(http.MethodOptions, path, handler)
}

// Head registers a HEAD handler.
func (g *RouteGroup) Head(path string, handler http.HandlerFunc) {
	g.Handle(http.MethodHead, path, handler)
}
