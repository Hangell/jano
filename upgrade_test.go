package jano_test

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hangell/jano"
	"github.com/Hangell/jano/middleware"
)

type connectionWriter struct {
	*httptest.ResponseRecorder
	connection net.Conn
	deadline   time.Time
}

func (w *connectionWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.connection, bufio.NewReadWriter(bufio.NewReader(w.connection), bufio.NewWriter(w.connection)), nil
}
func (w *connectionWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func TestHijackCommitsContextAndRecovery(t *testing.T) {
	for _, panicAfter := range []bool{false, true} {
		app := jano.New()
		reported := false
		app.Use(middleware.Recovery(func(*http.Request, any) { reported = true }))
		app.HandleContext("GET", "/", func(c *jano.Context) error {
			connection, _, err := http.NewResponseController(c.Writer).Hijack()
			if err != nil {
				t.Error(err)
				return err
			}
			defer connection.Close()
			if !c.Written() || c.Status() != 0 {
				t.Error("hijack did not commit context")
			}
			if err := c.JSON(500, "unexpected"); !errors.Is(err, jano.ErrResponseCommitted) {
				t.Error("JSON accepted after hijack")
			}
			if panicAfter {
				panic("after hijack")
			}
			return errors.New("after hijack")
		})
		server, client := net.Pipe()
		defer client.Close()
		w := &connectionWriter{ResponseRecorder: httptest.NewRecorder(), connection: server}
		app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Body.Len() != 0 || w.Result().Header.Get("Content-Type") != "" || reported != panicAfter {
			t.Fatalf("response written after hijack: %q, reported %v", w.Body.String(), reported)
		}
	}
}

func TestUnsupportedHijackDoesNotCommit(t *testing.T) {
	app := jano.New()
	app.Use(middleware.Recovery(nil))
	app.HandleContext("GET", "/", func(c *jano.Context) error {
		_, _, err := http.NewResponseController(c.Writer).Hijack()
		if !errors.Is(err, http.ErrNotSupported) || c.Written() {
			t.Error("unsupported hijack changed response")
		}
		return c.Text(200, "normal response")
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || w.Body.String() != "normal response" {
		t.Fatalf("got (%d,%q)", w.Code, w.Body.String())
	}
}

func TestSwitchingProtocolsIsFinal(t *testing.T) {
	for _, panicAfter := range []bool{false, true} {
		app := jano.New()
		app.Use(middleware.Recovery(nil))
		app.HandleContext("GET", "/", func(c *jano.Context) error {
			c.Writer.WriteHeader(http.StatusSwitchingProtocols)
			if !c.Written() || c.Status() != 101 {
				t.Error("101 was not treated as final")
			}
			if panicAfter {
				panic("after 101")
			}
			return errors.New("after 101")
		})
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 101 || w.Body.Len() != 0 {
			t.Fatalf("upgrade overwritten: (%d,%q)", w.Code, w.Body.String())
		}
	}
}

func TestResponseControllerDeadlineThroughWrappers(t *testing.T) {
	app := jano.New()
	app.Use(middleware.Recovery(nil))
	deadline := time.Now().Add(time.Minute)
	app.HandleContext("GET", "/", func(c *jano.Context) error {
		if err := http.NewResponseController(c.Writer).SetWriteDeadline(deadline); err != nil {
			t.Error(err)
		}
		return c.NoContent(204)
	})
	w := &connectionWriter{ResponseRecorder: httptest.NewRecorder()}
	app.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if !w.deadline.Equal(deadline) || w.Code != 204 {
		t.Fatal("deadline did not cross wrappers")
	}
}
