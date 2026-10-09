package jano

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRoutePrecedenceAndBacktracking(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		app := New()
		patterns := []string{"/users/{id}", "/users/new", "/{category}/posts/{id}", "/users/settings/profile", "/files/{path...}", "/files/{id}", "/files/static"}
		if reverse {
			for i, k := 0, len(patterns)-1; i < k; i, k = i+1, k-1 {
				patterns[i], patterns[k] = patterns[k], patterns[i]
			}
		}
		for _, pattern := range patterns {
			pattern := pattern
			app.Get(pattern, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, pattern+":"+Param(r, "id")+":"+Param(r, "path"))
			})
		}
		for _, tt := range []struct{ path, body string }{
			{"/users/new", "/users/new::"},
			{"/users/42", "/users/{id}:42:"},
			{"/users/posts/7", "/{category}/posts/{id}:7:"},
			{"/files/static", "/files/static::"},
			{"/files/42", "/files/{id}:42:"},
			{"/files/a/b", "/files/{path...}::a/b"},
			{"/files/", "/files/{id}::"},
		} {
			w := httptest.NewRecorder()
			app.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
			if w.Code != 200 || w.Body.String() != tt.body {
				t.Fatalf("reverse %v, %s got (%d, %q)", reverse, tt.path, w.Code, w.Body.String())
			}
		}
	}
}

func TestCatchAllAndParameterNames(t *testing.T) {
	app := New()
	app.Get("/assets/{path...}", func(w http.ResponseWriter, r *http.Request) {
		if Param(r, "path") != r.Context().Value("path") {
			t.Error("legacy parameter differs")
		}
		fmt.Fprint(w, Param(r, "path"))
	})
	app.Get("/a/{first}/x", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, Param(r, "first")) })
	app.Get("/a/{second}/y", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, Param(r, "second")) })
	for _, tt := range []struct {
		path, body string
		status     int
	}{
		{"/assets/a/b/c", "a/b/c", 200}, {"/assets/", "", 200}, {"/assets", "404 page not found\n", 404},
		{"/a/one/x", "one", 200}, {"/a/two/y", "two", 200},
	} {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
		if w.Code != tt.status || w.Body.String() != tt.body {
			t.Fatalf("%s got (%d, %q)", tt.path, w.Code, w.Body.String())
		}
	}
}

func TestParamDoesNotModifyOriginalPathValues(t *testing.T) {
	app := New()
	app.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if Param(r, "id") != "42" || Param(r, "other") != "preserved" {
			t.Error("incorrect path values")
		}
		r.SetPathValue("other", "changed")
	})
	r := httptest.NewRequest("GET", "/users/42", nil)
	r.SetPathValue("id", "original")
	r.SetPathValue("other", "preserved")
	app.ServeHTTP(httptest.NewRecorder(), r)
	if r.PathValue("id") != "original" || r.PathValue("other") != "preserved" {
		t.Fatal("original path values mutated")
	}
}

type nilHandler struct{}

func (*nilHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestRegistrationValidation(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, tt := range []struct {
		method, path string
		handler      http.Handler
		middleware   []Middleware
	}{
		{"", "/", handler, nil}, {"G ET", "/", handler, nil}, {"GET", "relative", handler, nil},
		{"GET", "/query?x=1", handler, nil}, {"GET", "/hash#fragment", handler, nil},
		{"GET", "/x/{}/", handler, nil}, {"GET", "/x/{id}/{id}", handler, nil},
		{"GET", "/x/{path...}/tail", handler, nil}, {"GET", "/prefix{id}", handler, nil},
		{"GET", "/x/{id", handler, nil}, {"GET", "/", nil, nil},
		{"GET", "/", (*nilHandler)(nil), nil}, {"GET", "/", http.HandlerFunc(nil), nil},
		{"GET", "/", handler, []Middleware{nil}},
	} {
		app := New()
		if err := app.Register(tt.method, tt.path, tt.handler, tt.middleware...); err == nil {
			t.Errorf("accepted %q %q with handler %T", tt.method, tt.path, tt.handler)
		}
	}
	app := New()
	app.Get("/a/{id}", handler)
	if err := app.Register("GET", "/a/{name}", handler); err == nil {
		t.Fatal("accepted ambiguous parameter names")
	}
	if err := app.Register("POST", "/a/{name}", handler); err != nil {
		t.Fatal(err)
	}
	if err := app.Register("GET", "/a/{id}", handler); err != nil {
		t.Fatal("replacement rejected", err)
	}
	if err := app.RegisterContext("GET", "/context", nil); err == nil {
		t.Fatal("nil context handler accepted")
	}
}

func TestMethodNotAllowedOptIn(t *testing.T) {
	app := New(WithMethodNotAllowed())
	app.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {})
	app.Post("/users/{name}", func(w http.ResponseWriter, r *http.Request) {})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("PUT", "/users/42", nil))
	if w.Code != 405 || w.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("got %d, Allow %q", w.Code, w.Header().Get("Allow"))
	}
	w = httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/missing", nil))
	if w.Code != 404 || w.Header().Get("Allow") != "" {
		t.Fatal("unknown path did not return 404")
	}
}

func TestGroupsAndMiddlewareScope(t *testing.T) {
	app := New()
	var calls []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, name+" before")
				next.ServeHTTP(w, r)
				calls = append(calls, name+" after")
			})
		}
	}
	app.Use(mark("global"))
	parent := app.Group("/api/", mark("parent"))
	child := parent.Group("/v1", mark("child"))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "handler")
		fmt.Fprint(w, Param(r, "id"))
	})
	child.Handle("GET", "/users/{id}", handler, mark("route"))
	parent.Use(mark("late"))
	child.HandleContext("GET", "/new/{id}", func(c *Context) error { return c.Text(200, c.Param("id")) })
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/users/42", nil))
	want := []string{"global before", "parent before", "child before", "route before", "handler", "route after", "child after", "parent after", "global after"}
	if !reflect.DeepEqual(calls, want) || w.Body.String() != "42" {
		t.Fatalf("got calls %v and body %q", calls, w.Body.String())
	}
	calls = nil
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/new/7", nil))
	want = []string{"global before", "parent before", "late before", "child before", "child after", "late after", "parent after", "global after"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("late parent middleware missing: %v", calls)
	}
	calls = nil
	app.Get("/outside", handler)
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/outside", nil))
	if !reflect.DeepEqual(calls, []string{"global before", "handler", "global after"}) {
		t.Fatalf("group middleware escaped: %v", calls)
	}
}

func TestMiddlewareCompilationAndLateRegistration(t *testing.T) {
	app := New()
	var builds atomic.Int64
	app.Use(func(next http.Handler) http.Handler { builds.Add(1); return next })
	app.Get("/first", func(w http.ResponseWriter, r *http.Request) {})
	router := app.Router()
	for i := 0; i < 10; i++ {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/first", nil))
	}
	if builds.Load() != 1 {
		t.Fatalf("factory rebuilt on requests: %d", builds.Load())
	}
	app.Get("/late", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/late", nil))
	if w.Code != 201 {
		t.Fatal("existing Router did not see registration")
	}
	if builds.Load() != 3 {
		t.Fatalf("unexpected snapshot compilation: %d", builds.Load())
	}
}

func TestConcurrentConfigurationAndRequests(t *testing.T) {
	app := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, Param(r, "id")) })
	app.Get("/users/{id}", handler)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for n := 0; n < 50; n++ {
				w := httptest.NewRecorder()
				app.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/users/%d", id), nil))
				if w.Code != 200 || w.Body.String() != fmt.Sprint(id) {
					t.Errorf("invalid concurrent response (%d, %q)", w.Code, w.Body.String())
				}
			}
		}(i)
	}
	for n := 0; n < 30; n++ {
		app.Get("/users/{id}", handler)
		app.Get(fmt.Sprintf("/route/%d", n), handler)
		app.NotFound(http.NotFound)
		app.SetErrorHandler(DefaultErrorHandler)
		app.Use(func(next http.Handler) http.Handler { return next })
	}
	wg.Wait()
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/route/29", nil))
	if w.Code != 200 {
		t.Fatal("final configuration was not published")
	}
}

func TestConfigurationPanics(t *testing.T) {
	for name, configure := range map[string]func(){
		"nil middleware":              func() { New().Use(nil) },
		"nil fallback":                func() { New().NotFound(nil) },
		"nil error handler":           func() { New().SetErrorHandler(nil) },
		"invalid route":               func() { New().Get("invalid", func(http.ResponseWriter, *http.Request) {}) },
		"nil context handler":         func() { New().HandleContext("GET", "/", nil) },
		"invalid group":               func() { New().Group("invalid") },
		"nil group middleware":        func() { New().Group("/api", nil) },
		"nil group use":               func() { New().Group("/api").Use(nil) },
		"invalid group route":         func() { New().Group("/api").Get("invalid", func(http.ResponseWriter, *http.Request) {}) },
		"invalid group context route": func() { New().Group("/api").HandleContext("GET", "invalid", func(*Context) error { return nil }) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected panic")
				}
			}()
			configure()
		})
	}
}

func TestGroupMethodHelpers(t *testing.T) {
	app := New()
	group := app.Group("/api")
	methods := []struct {
		method   string
		register func(string, http.HandlerFunc)
	}{
		{"GET", group.Get}, {"POST", group.Post}, {"PUT", group.Put}, {"DELETE", group.Delete},
		{"PATCH", group.Patch}, {"OPTIONS", group.Options}, {"HEAD", group.Head},
	}
	for _, method := range methods {
		name := method.method
		method.register("/resource", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-Method", name); w.WriteHeader(204) })
	}
	for _, method := range methods {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest(method.method, "/api/resource", nil))
		if w.Code != 204 || w.Header().Get("X-Method") != method.method {
			t.Fatalf("incorrect group method %s", method.method)
		}
	}
}

func TestConcurrentFirstRequestsCompileOnce(t *testing.T) {
	app := New()
	var builds atomic.Int64
	app.Use(func(next http.Handler) http.Handler { builds.Add(1); return next })
	app.Get("/", func(http.ResponseWriter, *http.Request) {})
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		}()
	}
	close(start)
	wg.Wait()
	if builds.Load() != 1 {
		t.Fatalf("snapshot compiled %d times", builds.Load())
	}
}
