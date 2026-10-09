package middleware_test

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hangell/jano"
	"github.com/Hangell/jano/middleware"
)

func TestRecovery(t *testing.T) {
	for _, committed := range []bool{false, true} {
		reported := false
		handler := middleware.Recovery(func(r *http.Request, value any) {
			reported = true
			if value != "private panic" || r.URL.Path != "/" {
				t.Error("incorrect panic report")
			}
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if committed {
				w.WriteHeader(202)
				_, _ = io.WriteString(w, "accepted")
			}
			panic("private panic")
		}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		want := 500
		if committed {
			want = 202
		}
		if !reported || w.Code != want || strings.Contains(w.Body.String(), "private") {
			t.Fatalf("got (%d, %q), reported %v", w.Code, w.Body.String(), reported)
		}
		if committed && w.Body.String() != "accepted" {
			t.Fatal("committed response was overwritten")
		}
	}
}

func TestRecoveryPreservesAbortHandler(t *testing.T) {
	defer func() {
		if value := recover(); value != http.ErrAbortHandler {
			t.Fatalf("expected ErrAbortHandler, got %v", value)
		}
	}()
	handler := middleware.Recovery(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

func TestRecoveryPreservesFlush(t *testing.T) {
	app := jano.New()
	app.Use(middleware.Recovery(nil))
	app.HandleContext("GET", "/", func(c *jano.Context) error {
		if err := http.NewResponseController(c.Writer).Flush(); err != nil {
			t.Error(err)
		}
		panic("late panic")
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if !w.Flushed || w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("flushed response was replaced")
	}
}

func TestBodyLimit(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		called := false
		handler := middleware.BodyLimit(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			_, err := io.ReadAll(r.Body)
			var tooLarge *http.MaxBytesError
			if !errors.As(err, &tooLarge) {
				t.Fatalf("expected MaxBytesError, got %v", err)
			}
			w.WriteHeader(413)
		}))
		r := httptest.NewRequest("POST", "/", strings.NewReader("12345"))
		if streamed {
			r.ContentLength = -1
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 413 || called != streamed {
			t.Fatalf("streamed %v: status %d, called %v", streamed, w.Code, called)
		}
	}
	handler := middleware.BodyLimit(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "1234" {
			t.Errorf("exact limit failed: %q, %v", body, err)
		}
		w.WriteHeader(204)
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader("1234")))
	if w.Code != 204 {
		t.Fatal("exact limit rejected")
	}
}

func TestContextTimeout(t *testing.T) {
	type key struct{}
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "parent"))
	defer cancel()
	var received context.Context
	handler := middleware.ContextTimeout(time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Context()
		deadline, ok := received.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > time.Minute {
			t.Error("missing or incorrect deadline")
		}
		if received.Value(key{}) != "parent" {
			t.Error("parent values lost")
		}
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(parent))
	if !errors.Is(received.Err(), context.Canceled) {
		t.Fatal("child context not canceled after handler")
	}
	cancel()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(parent))
	if !errors.Is(received.Err(), context.Canceled) {
		t.Fatal("parent cancellation lost")
	}
}

func TestRequestID(t *testing.T) {
	var last string
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Request-ID", "untrusted-input")
		middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := middleware.RequestIDFromContext(r.Context())
			if id != w.Header().Get("X-Request-ID") || len(id) != 32 || id == last {
				t.Errorf("invalid request ID %q", id)
			}
			if _, err := hex.DecodeString(id); err != nil {
				t.Error(err)
			}
			last = id
		})).ServeHTTP(w, r)
	}
	if middleware.RequestIDFromContext(context.Background()) != "" {
		t.Fatal("unexpected ID without middleware")
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for name, configure := range map[string]func(){
		"body limit": func() { middleware.BodyLimit(0) },
		"timeout":    func() { middleware.ContextTimeout(0) },
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
