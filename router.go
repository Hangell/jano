package jano

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
)

type routeKey struct{ method, path string }
type patternKey struct{ method, signature string }
type segment struct {
	value               string
	parameter, catchAll bool
}
type route struct {
	key            routeKey
	segments       []segment
	handler        http.Handler
	contextHandler HandlerFunc
	middleware     []Middleware
}
type parameter struct {
	name     string
	index    int
	catchAll bool
}
type compiledRoute struct {
	handler http.Handler
	params  []parameter
}
type routeNode struct {
	static    map[string]*routeNode
	parameter *routeNode
	catchAll  *compiledRoute
	entry     *compiledRoute
}
type methodTree struct {
	root  *routeNode
	exact map[string]*compiledRoute
}
type routerState struct {
	methods          map[string]*methodTree
	notFound         http.Handler
	methodNotAllowed bool
}

func (j *Jano) register(method, path string, handler http.Handler, contextual HandlerFunc, middleware []Middleware) error {
	if !validMethod(method) {
		return fmt.Errorf("jano: invalid HTTP method %q", method)
	}
	if isNilHandler(handler) && contextual == nil {
		return fmt.Errorf("jano: nil handler for %s %s", method, path)
	}
	for _, m := range middleware {
		if m == nil {
			return fmt.Errorf("jano: nil middleware for %s %s", method, path)
		}
	}
	segments, signature, err := parsePattern(path)
	if err != nil {
		return err
	}
	key := routeKey{method, path}
	j.mu.Lock()
	defer j.mu.Unlock()
	shape := patternKey{method, signature}
	if existingPath, ok := j.patterns[shape]; ok && existingPath != path {
		return fmt.Errorf("jano: ambiguous routes %q and %q for %s", existingPath, path, method)
	}
	j.patterns[shape] = path
	j.routes[key] = route{key: key, segments: segments, handler: handler, contextHandler: contextual, middleware: append([]Middleware(nil), middleware...)}
	j.invalidate()
	return nil
}

func isNilHandler(handler http.Handler) bool {
	if handler == nil {
		return true
	}
	v := reflect.ValueOf(handler)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func validMethod(method string) bool {
	if method == "" {
		return false
	}
	for _, c := range method {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			continue
		}
		return false
	}
	return true
}

func parsePattern(path string) ([]segment, string, error) {
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#\r\n") {
		return nil, "", fmt.Errorf("jano: invalid route path %q", path)
	}
	parts := strings.Split(path, "/")
	segments := make([]segment, len(parts))
	var signature strings.Builder
	names := make(map[string]bool)
	for i, part := range parts {
		s := segment{value: part}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			s.value = part[1 : len(part)-1]
			s.parameter = true
			if strings.HasSuffix(s.value, "...") {
				s.catchAll = true
				s.value = strings.TrimSuffix(s.value, "...")
				if i != len(parts)-1 {
					return nil, "", fmt.Errorf("jano: catch-all must be the final segment in %q", path)
				}
			}
			if s.value == "" || strings.ContainsAny(s.value, "{}") || names[s.value] {
				return nil, "", fmt.Errorf("jano: invalid or repeated parameter %q in %q", s.value, path)
			}
			names[s.value] = true
		} else if strings.ContainsAny(part, "{}") {
			return nil, "", fmt.Errorf("jano: parameters must occupy an entire segment in %q", path)
		}
		segments[i] = s
		if s.catchAll {
			signature.WriteString("C;")
		} else if s.parameter {
			signature.WriteString("P;")
		} else {
			fmt.Fprintf(&signature, "S%d:%s;", len(part), part)
		}
	}
	return segments, signature.String(), nil
}

func compileRouter(routes []route, global []Middleware, fallback http.Handler, errorHandler ErrorHandler, methodNotAllowed bool) *routerState {
	state := &routerState{methods: make(map[string]*methodTree), notFound: fallback, methodNotAllowed: methodNotAllowed}
	// Sorting makes factory invocation order reproducible, independent of maps.
	sort.Slice(routes, func(i, k int) bool {
		if routes[i].key.method == routes[k].key.method {
			return routes[i].key.path < routes[k].key.path
		}
		return routes[i].key.method < routes[k].key.method
	})
	for _, route := range routes {
		handler := route.handler
		if route.contextHandler != nil {
			handler = adapt(route.contextHandler, errorHandler)
		}
		for i := len(route.middleware) - 1; i >= 0; i-- {
			handler = route.middleware[i](handler)
		}
		for i := len(global) - 1; i >= 0; i-- {
			handler = global[i](handler)
		}
		entry := &compiledRoute{handler: handler}
		tree := state.methods[route.key.method]
		if tree == nil {
			tree = &methodTree{root: &routeNode{}, exact: make(map[string]*compiledRoute)}
			state.methods[route.key.method] = tree
		}
		node := tree.root
		for i, s := range route.segments {
			if s.parameter {
				entry.params = append(entry.params, parameter{s.value, i, s.catchAll})
			}
			if s.catchAll {
				node.catchAll = entry
				break
			}
			if s.parameter {
				if node.parameter == nil {
					node.parameter = &routeNode{}
				}
				node = node.parameter
			} else {
				if node.static == nil {
					node.static = make(map[string]*routeNode)
				}
				if node.static[s.value] == nil {
					node.static[s.value] = &routeNode{}
				}
				node = node.static[s.value]
			}
			if i == len(route.segments)-1 {
				node.entry = entry
			}
		}
		if len(entry.params) == 0 {
			tree.exact[route.key.path] = entry
		}
	}
	return state
}

func (node *routeNode) find(parts []string, index int) *compiledRoute {
	if index == len(parts) {
		return node.entry
	}
	if next := node.static[parts[index]]; next != nil {
		if entry := next.find(parts, index+1); entry != nil {
			return entry
		}
	}
	if node.parameter != nil {
		if entry := node.parameter.find(parts, index+1); entry != nil {
			return entry
		}
	}
	return node.catchAll
}

func (state *routerState) find(method, path string) (*compiledRoute, []string) {
	tree := state.methods[method]
	if tree == nil {
		return nil, nil
	}
	if entry := tree.exact[path]; entry != nil {
		return entry, nil
	}
	parts := strings.Split(path, "/")
	return tree.root.find(parts, 0), parts
}

func (state *routerState) allowedMethods(path string) string {
	var methods []string
	for method := range state.methods {
		if entry, _ := state.find(method, path); entry != nil {
			methods = append(methods, method)
		}
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}

// Param returns a route parameter from a Jano request. It works in standard
// handlers and middleware using Go's Request.PathValue API.
func Param(r *http.Request, name string) string { return r.PathValue(name) }

// parameterContext preserves legacy string-key lookups without allocating a
// nested context node for every parameter. The values are request-local.
type parameterContext struct {
	context.Context
	params []parameter
	values []string
}

func (c *parameterContext) Value(key any) any {
	if name, ok := key.(string); ok {
		for i, param := range c.params {
			if param.name == name {
				return c.values[i]
			}
		}
	}
	return c.Context.Value(key)
}

func withParams(r *http.Request, params []parameter, parts []string) *http.Request {
	ctx := &parameterContext{Context: r.Context(), params: params, values: make([]string, len(params))}
	for i, param := range params {
		value := parts[param.index]
		if param.catchAll {
			value = strings.Join(parts[param.index:], "/")
		}
		ctx.values[i] = value
	}
	// Clone isolates PathValue's mutable map and preserves caller context values.
	r = r.Clone(ctx)
	for i, param := range params {
		r.SetPathValue(param.name, ctx.values[i])
	}
	return r
}
