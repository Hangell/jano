package jano

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContextBinding(t *testing.T) {
	for _, tt := range []struct {
		name, body, contentType string
		limit                   int64
		status                  int
	}{
		{"valid", `{"name":"Jane"}`, "application/json", 1024, 201},
		{"JSON suffix", `{"name":"Jane"}`, "application/problem+json", 1024, 201},
		{"missing media type", `{"name":"Jane"}`, "", 1024, 201},
		{"empty", "", "application/json", 1024, 400},
		{"malformed", `{`, "application/json", 1024, 400},
		{"unknown field", `{"name":"Jane","unexpected":1}`, "application/json", 1024, 400},
		{"trailing value", `{"name":"Jane"} {}`, "application/json", 1024, 400},
		{"trailing garbage", `{"name":"Jane"} nope`, "application/json", 1024, 400},
		{"wrong type", `{"name":3}`, "application/json", 1024, 400},
		{"wrong media", `{"name":"Jane"}`, "text/plain", 1024, 415},
		{"invalid media", `{"name":"Jane"}`, "invalid;", 1024, 415},
		{"too large", `{"name":"Jane"}`, "application/json", 4, 413},
		{"oversized trailing whitespace", `{"name":"Jane"}` + strings.Repeat(" ", 20), "application/json", 20, 413},
		{"invalid limit", `{}`, "application/json", 0, 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app := New()
			app.HandleContext("POST", "/users/{id}", func(c *Context) error {
				var input struct {
					Name string `json:"name"`
				}
				if err := c.BindJSONLimit(&input, tt.limit); err != nil {
					return err
				}
				if c.Param("id") != "42" || c.Query("sort") != "name" {
					t.Error("context helpers missing values")
				}
				return c.JSON(201, input)
			})
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/users/42?sort=name", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			app.ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("got (%d, %q), want %d", w.Code, w.Body.String(), tt.status)
			}
			if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal("missing JSON media type")
			}
			if tt.status == 201 && w.Body.String() != "{\"name\":\"Jane\"}\n" {
				t.Fatalf("incorrect JSON response %q", w.Body.String())
			}
		})
	}
}

func TestContextErrors(t *testing.T) {
	cause := errors.New("private database error")
	for _, tt := range []struct {
		name    string
		handler HandlerFunc
		status  int
		body    string
	}{
		{"unexpected", func(*Context) error { return cause }, 500, "Internal Server Error"},
		{"wrapped HTTP", func(*Context) error {
			return fmt.Errorf("wrapped: %w", &HTTPError{Status: 404, Message: "User not found", Cause: cause})
		}, 404, "User not found"},
		{"invalid HTTP status", func(*Context) error { return NewHTTPError(200, "bad") }, 500, "Internal Server Error"},
		{"default message", func(*Context) error { return NewHTTPError(403, "") }, 403, "Forbidden"},
		{"JSON encoding", func(c *Context) error { return c.JSON(201, make(chan int)) }, 500, "Internal Server Error"},
		{"invalid binding target", func(c *Context) error { return c.BindJSON(nil) }, 500, "Internal Server Error"},
		{"invalid status", func(c *Context) error { return c.JSON(0, "value") }, 500, "Internal Server Error"},
		{"JSON no-body status", func(c *Context) error { return c.JSON(204, "value") }, 500, "Internal Server Error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app := New()
			app.HandleContext("POST", "/", tt.handler)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{}`)))
			if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.body) || strings.Contains(w.Body.String(), "private") {
				t.Fatalf("got (%d, %q)", w.Code, w.Body.String())
			}
		})
	}
	if !errors.Is(&HTTPError{Cause: cause}, cause) {
		t.Fatal("HTTPError did not unwrap cause")
	}
}

func TestCommittedResponseAndCustomErrorHandler(t *testing.T) {
	app := New()
	called := false
	app.SetErrorHandler(func(c *Context, err error) {
		called = true
		if !c.Written() || c.Status() != 202 || err.Error() != "late failure" {
			t.Error("error handler did not receive committed response")
		}
		DefaultErrorHandler(c, err)
	})
	app.HandleContext("GET", "/", func(c *Context) error {
		if c.Written() || c.Status() != 0 {
			t.Error("response starts committed")
		}
		if err := c.Text(202, "accepted"); err != nil {
			return err
		}
		if err := c.JSON(200, "again"); !errors.Is(err, ErrResponseCommitted) {
			t.Error("second response accepted")
		}
		return errors.New("late failure")
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if !called || w.Code != 202 || w.Body.String() != "accepted" {
		t.Fatalf("got (%d, %q), called %v", w.Code, w.Body.String(), called)
	}
}

func TestContextNoContentAndCancellation(t *testing.T) {
	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.HandleContext("DELETE", "/", func(c *Context) error {
		if !errors.Is(c.Context().Err(), context.Canceled) {
			t.Error("context cancellation lost")
		}
		return c.NoContent(204)
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("DELETE", "/", nil).WithContext(ctx))
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("got (%d, %q)", w.Code, w.Body.String())
	}
}

func TestResponseControllerFlushAndWrite(t *testing.T) {
	app := New()
	app.HandleContext("GET", "/", func(c *Context) error {
		if err := http.NewResponseController(c.Writer).Flush(); err != nil {
			t.Error(err)
		}
		if !c.Written() || c.Status() != 200 {
			t.Error("flush did not commit response")
		}
		c.Writer.WriteHeader(500)
		_, err := c.Writer.Write([]byte("stream"))
		return err
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if !w.Flushed || w.Code != 200 || w.Body.String() != "stream" {
		t.Fatalf("got (%d, %q), flushed %v", w.Code, w.Body.String(), w.Flushed)
	}
}

func TestJSONEncodingFailureDoesNotCommit(t *testing.T) {
	app := New()
	app.HandleContext("GET", "/", func(c *Context) error {
		if err := c.JSON(201, func() {}); err == nil {
			t.Error("expected encoding error")
		}
		if c.Written() {
			t.Error("encoding failure committed response")
		}
		return c.JSON(200, map[string]bool{"ok": true})
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	var result map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || !result["ok"] {
		t.Fatalf("got (%d, %q)", w.Code, w.Body.String())
	}
}

func TestContextResponseContracts(t *testing.T) {
	for name, handler := range map[string]HandlerFunc{
		"invalid text status":  func(c *Context) error { return c.Text(0, "body") },
		"text body forbidden":  func(c *Context) error { return c.Text(204, "body") },
		"invalid empty status": func(c *Context) error { return c.NoContent(0) },
	} {
		t.Run(name, func(t *testing.T) {
			app := New()
			app.HandleContext("GET", "/", handler)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			if w.Code != 500 {
				t.Fatalf("got %d", w.Code)
			}
		})
	}
	app := New()
	app.HandleContext("GET", "/", func(c *Context) error {
		_, err := c.Writer.Write([]byte("implicit status"))
		if err != nil {
			return err
		}
		if !c.Written() || c.Status() != 200 {
			t.Error("raw write did not commit")
		}
		if !errors.Is(c.Text(200, "again"), ErrResponseCommitted) || !errors.Is(c.NoContent(204), ErrResponseCommitted) {
			t.Error("committed helpers accepted")
		}
		return nil
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || w.Body.String() != "implicit status" {
		t.Fatalf("got (%d,%q)", w.Code, w.Body.String())
	}
}
