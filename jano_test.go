package jano

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestHTTPMethods(t *testing.T) {
	app := New()
	methods := []struct {
		method   string
		register func(string, http.HandlerFunc)
	}{
		{http.MethodGet, app.Get}, {http.MethodPost, app.Post},
		{http.MethodPut, app.Put}, {http.MethodDelete, app.Delete},
		{http.MethodPatch, app.Patch}, {http.MethodOptions, app.Options},
		{http.MethodHead, app.Head},
	}
	for _, method := range methods {
		name := method.method
		method.register("/resource", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Method", name)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	for _, method := range methods {
		t.Run(method.method, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, httptest.NewRequest(method.method, "/resource", nil))
			if w.Code != http.StatusNoContent || w.Header().Get("X-Method") != method.method {
				t.Fatalf("got status %d, method %q", w.Code, w.Header().Get("X-Method"))
			}
		})
	}
}

func TestRouting(t *testing.T) {
	app := New()
	app.Get("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "root") })
	app.Get("/people/{id}/posts/{post}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s:%s", r.Context().Value("id"), r.Context().Value("post"))
	})
	app.Get("/replace", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "old") })
	app.Get("/replace", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "new") })
	tests := []struct {
		name, method, path string
		status             int
		body               string
	}{
		{"root", "GET", "/", 200, "root"},
		{"parameters", "GET", "/people/42/posts/7", 200, "42:7"},
		{"query ignored", "GET", "/people/42/posts/7?id=99", 200, "42:7"},
		{"replacement", "GET", "/replace", 200, "new"},
		{"unknown path", "GET", "/missing", 404, "404 page not found\n"},
		{"unknown method", "POST", "/replace", 404, "404 page not found\n"},
		{"no implicit head", "HEAD", "/replace", 404, "404 page not found\n"},
		{"no implicit options", "OPTIONS", "/replace", 404, "404 page not found\n"},
		{"trailing slash", "GET", "/replace/", 404, "404 page not found\n"},
		{"case sensitive", "GET", "/Replace", 404, "404 page not found\n"},
		{"missing segment", "GET", "/people/42/posts", 404, "404 page not found\n"},
		{"different literal", "GET", "/users/42/posts/7", 404, "404 page not found\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.status || w.Body.String() != tt.body {
				t.Fatalf("got (%d, %q), want (%d, %q)", w.Code, w.Body.String(), tt.status, tt.body)
			}
		})
	}
}

func TestMiddlewareOrderAndParameters(t *testing.T) {
	app := New()
	var calls []string
	for _, name := range []string{"first", "second"} {
		name := name
		app.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Context().Value("id") != "42" {
					t.Error("middleware did not receive route parameter")
				}
				calls = append(calls, name+" before")
				next.ServeHTTP(w, r)
				calls = append(calls, name+" after")
			})
		})
	}
	app.Get("/people/{id}", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "handler")
		w.WriteHeader(http.StatusNoContent)
	})
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, httptest.NewRequest("GET", "/people/42", nil))
	want := []string{"first before", "second before", "handler", "second after", "first after"}
	if !reflect.DeepEqual(calls, want) || w.Code != http.StatusNoContent {
		t.Fatalf("got calls %v and status %d", calls, w.Code)
	}
}

func TestMiddlewareCanStopRequest(t *testing.T) {
	app := New()
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	})
	app.Get("/private", func(w http.ResponseWriter, r *http.Request) { t.Error("blocked handler was called") })
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, httptest.NewRequest("GET", "/private", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d", w.Code)
	}
}

func TestCustomNotFoundBypassesMiddleware(t *testing.T) {
	app := New()
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("middleware ran for an unmatched request")
			next.ServeHTTP(w, r)
		})
	})
	app.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "custom fallback", http.StatusNotFound)
	})
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, httptest.NewRequest("GET", "/missing", nil))
	if w.Code != 404 || w.Body.String() != "custom fallback\n" {
		t.Fatalf("got (%d, %q)", w.Code, w.Body.String())
	}
}

func TestParametersPreserveContextAndRequest(t *testing.T) {
	type contextKey struct{}
	request := httptest.NewRequest("GET", "/people/42", nil)
	ctx, cancel := context.WithCancel(context.WithValue(request.Context(), contextKey{}, "existing"))
	defer cancel()
	request = request.WithContext(ctx)
	app := New()
	app.Get("/people/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(contextKey{}) != "existing" || r.Context().Value("id") != "42" {
			t.Error("context values were not preserved")
		}
		cancel()
		if r.Context().Err() != context.Canceled {
			t.Error("context cancellation was not preserved")
		}
	})
	app.Router().ServeHTTP(httptest.NewRecorder(), request)
	if request.Context().Value("id") != nil {
		t.Fatal("original request was modified")
	}
}

func TestConcurrentRequestsIsolateParameters(t *testing.T) {
	app := New()
	app.Get("/people/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Context().Value("id"))
	})
	router := app.Router()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			want := fmt.Sprint(id)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/people/"+want, nil))
			if w.Code != 200 || w.Body.String() != want {
				t.Errorf("id %s got (%d, %q)", want, w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()
}

func FuzzMatchRoute(f *testing.F) {
	for _, seed := range [][2]string{
		{"/", "/"}, {"/people/{id}", "/people/42"},
		{"/people/{id}", "/people/"}, {"/people/{id}", "/users/42"},
		{"/people/{id}", "/people/42/posts"}, {"/{id:[0-9]+}", "/text"},
		{"/files/{path...}", "/files/a/b"}, {"/files/{path...}", "/files/"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, pattern, path string) {
		segments, _, err := parsePattern(pattern)
		if err != nil {
			return
		}
		state := compileRouter([]route{{key: routeKey{"GET", pattern}, segments: segments, handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}}, nil, http.HandlerFunc(http.NotFound), DefaultErrorHandler, false)
		entry, parts := state.find("GET", path)
		pathParts := strings.Split(path, "/")
		catchAll := segments[len(segments)-1].catchAll
		expected := len(pathParts) == len(segments) || (catchAll && len(pathParts) >= len(segments))
		if expected {
			for i, segment := range segments {
				if !segment.parameter && segment.value != pathParts[i] {
					expected = false
					break
				}
			}
		}
		if (entry != nil) != expected {
			t.Fatal("compiled lookup disagrees with reference matching")
		}
		if entry == nil {
			return
		}
		if len(entry.params) == 0 {
			if pattern != path {
				t.Fatal("static route matched a different path")
			}
			return
		}
		r := withParams(httptest.NewRequest("GET", "/", nil), entry.params, parts)
		var reconstructed []string
		for _, segment := range segments {
			if segment.parameter {
				reconstructed = append(reconstructed, Param(r, segment.value))
			} else {
				reconstructed = append(reconstructed, segment.value)
			}
		}
		if strings.Join(reconstructed, "/") != path {
			t.Fatal("parameters did not reconstruct path")
		}

	})
}
