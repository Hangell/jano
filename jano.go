package jano

import (
	"context"
	"net/http"
	"strings"
)

// Jano struct to hold the routes and middlewares
type Jano struct {
	routes       map[string]map[string]http.HandlerFunc
	middlewares  []func(http.Handler) http.Handler
	notFound     http.HandlerFunc
}

// New creates a new instance of Jano
func New() *Jano {
	return &Jano{
		routes:      make(map[string]map[string]http.HandlerFunc),
		middlewares: []func(http.Handler) http.Handler{},
		notFound:    http.NotFound,
	}
}

// Get registers a GET handler
func (j *Jano) Get(path string, handler http.HandlerFunc) {
	j.addRoute("GET", path, handler)
}

// Post registers a POST handler
func (j *Jano) Post(path string, handler http.HandlerFunc) {
	j.addRoute("POST", path, handler)
}

// Put registers a PUT handler
func (j *Jano) Put(path string, handler http.HandlerFunc) {
	j.addRoute("PUT", path, handler)
}

// Delete registers a DELETE handler
func (j *Jano) Delete(path string, handler http.HandlerFunc) {
	j.addRoute("DELETE", path, handler)
}

// Patch registers a PATCH handler
func (j *Jano) Patch(path string, handler http.HandlerFunc) {
	j.addRoute("PATCH", path, handler)
}

// Options registers an OPTIONS handler
func (j *Jano) Options(path string, handler http.HandlerFunc) {
	j.addRoute("OPTIONS", path, handler)
}

// Head registers a HEAD handler
func (j *Jano) Head(path string, handler http.HandlerFunc) {
	j.addRoute("HEAD", path, handler)
}

// Use adds a middleware to the chain
func (j *Jano) Use(middleware func(http.Handler) http.Handler) {
	j.middlewares = append(j.middlewares, middleware)
}

// addRoute adds a handler for a specific method and path
func (j *Jano) addRoute(method, path string, handler http.HandlerFunc) {
	if j.routes[path] == nil {
		j.routes[path] = make(map[string]http.HandlerFunc)
	}
	j.routes[path][method] = handler
}

// Router returns the http.Handler to be used by http.Server
func (j *Jano) Router() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, params, ok := j.findHandler(r.Method, r.URL.Path)
		if !ok {
			j.notFound(w, r)
			return
		}
		r = setParams(r, params)
		wrappedHandler := http.Handler(handler)
		for i := len(j.middlewares) - 1; i >= 0; i-- {
			wrappedHandler = j.middlewares[i](wrappedHandler)
		}
		wrappedHandler.ServeHTTP(w, r)
	})
}

// findHandler finds the handler for the given method and path
func (j *Jano) findHandler(method, path string) (http.HandlerFunc, map[string]string, bool) {
	for route, methodHandlers := range j.routes {
		if params, ok := matchRoute(route, path); ok {
			if handler, ok := methodHandlers[method]; ok {
				return handler, params, true
			}
		}
	}
	return nil, nil, false
}

// NotFound sets the custom 404 handler
func (j *Jano) NotFound(handler http.HandlerFunc) {
	j.notFound = handler
}

// matchRoute checks if the route matches the path and extracts parameters
func matchRoute(route, path string) (map[string]string, bool) {
	routeParts := strings.Split(route, "/")
	pathParts := strings.Split(path, "/")
	if len(routeParts) != len(pathParts) {
		return nil, false
	}
	params := make(map[string]string)
	for i := range routeParts {
		if strings.HasPrefix(routeParts[i], "{") && strings.HasSuffix(routeParts[i], "}") {
			paramName := routeParts[i][1:len(routeParts[i])-1]
			params[paramName] = pathParts[i]
		} else if routeParts[i] != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

// setParams adds the parameters to the request context
func setParams(r *http.Request, params map[string]string) *http.Request {
	ctx := r.Context()
	for key, value := range params {
		ctx = context.WithValue(ctx, key, value)
	}
	return r.WithContext(ctx)
}
