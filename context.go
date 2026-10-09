package jano

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
)

// HandlerFunc processes a request and returns errors to the configured policy.
type HandlerFunc func(*Context) error

// Context holds one request and response. It must not be shared across goroutines
// or retained after a handler returns. Use Context() when calling repositories,
// database clients and other context-aware services.
type Context struct {
	Writer   http.ResponseWriter
	Request  *http.Request
	response *responseWriter
}

// Context returns the standard request context with cancellation and deadlines.
func (c *Context) Context() context.Context { return c.Request.Context() }

// Param returns a path parameter.
func (c *Context) Param(name string) string { return Param(c.Request, name) }

// ParamInt parses an int path parameter; invalid values return an HTTP 400 error.
func (c *Context) ParamInt(name string) (int, error) { return ParamInt(c.Request, name) }

// ParamInt64 parses an int64 path parameter, preserving errors for the error policy.
// Positive-only identifiers require a separate domain check.
func (c *Context) ParamInt64(name string) (int64, error) { return ParamInt64(c.Request, name) }

// Query returns the first query parameter value.
func (c *Context) Query(name string) string { return c.Request.URL.Query().Get(name) }

// Written reports whether final headers have been written or the connection
// has been hijacked. A successful hijack without headers leaves Status at zero.
func (c *Context) Written() bool { return c.response.written }

// Status returns the final written status, or zero before a response is written.
func (c *Context) Status() int { return c.response.status }

// JSON serializes before committing the status, so encoding errors can be handled.
func (c *Context) JSON(status int, value any) error {
	if c.Written() {
		return ErrResponseCommitted
	}
	if status < 200 || status > 599 {
		return fmt.Errorf("jano: invalid final status %d", status)
	}
	if status == http.StatusNoContent || status == http.StatusResetContent || status == http.StatusNotModified {
		return fmt.Errorf("jano: status %d does not allow a JSON body", status)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(status)
	_, err = c.Writer.Write(append(data, '\n'))
	return err
}

// Text writes a UTF-8 plain-text response.
func (c *Context) Text(status int, text string) error {
	if c.Written() {
		return ErrResponseCommitted
	}
	if status < 200 || status > 599 {
		return fmt.Errorf("jano: invalid final status %d", status)
	}
	if status == http.StatusNoContent || status == http.StatusResetContent || status == http.StatusNotModified {
		return fmt.Errorf("jano: status %d does not allow a text body", status)
	}
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(status)
	_, err := io.WriteString(c.Writer, text)
	return err
}

// NoContent writes a status without a response body.
func (c *Context) NoContent(status int) error {
	if c.Written() {
		return ErrResponseCommitted
	}
	if status < 200 || status > 599 {
		return fmt.Errorf("jano: invalid final status %d", status)
	}
	c.Writer.WriteHeader(status)
	return nil
}

// BindJSON decodes exactly one JSON value with a 1 MiB limit and rejects unknown
// object fields. It does not write a response. Use BindJSONLimit for another limit.
// A supplied Content-Type must be application/json or an application/*+json type.
// Validate domain rules separately after decoding.
func (c *Context) BindJSON(value any) error { return c.BindJSONLimit(value, 1<<20) }

// BindJSONLimit decodes a bounded body. The limit must be positive.
func (c *Context) BindJSONLimit(value any, limit int64) error {
	if limit <= 0 {
		return fmt.Errorf("jano: body limit must be positive")
	}
	if contentType := c.Request.Header.Get("Content-Type"); contentType != "" {
		media, _, err := mime.ParseMediaType(contentType)
		if err != nil || (media != "application/json" && !(strings.HasPrefix(media, "application/") && strings.HasSuffix(media, "+json"))) {
			return NewHTTPError(http.StatusUnsupportedMediaType, "Content-Type must be JSON")
		}
	}
	if c.Request.Body == nil {
		return NewHTTPError(http.StatusBadRequest, "Invalid JSON body")
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return jsonBodyError(err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return jsonBodyError(err)
		}
		return NewHTTPError(http.StatusBadRequest, "Body must contain exactly one JSON value")
	}
	return nil
}

func jsonBodyError(err error) error {
	var invalidTarget *json.InvalidUnmarshalError
	if errors.As(err, &invalidTarget) {
		return err
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &HTTPError{Status: http.StatusRequestEntityTooLarge, Message: "Request body too large", Cause: err}
	}
	return &HTTPError{Status: http.StatusBadRequest, Message: "Invalid JSON body", Cause: err}
}

func adapt(handler HandlerFunc, errorHandler ErrorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &responseWriter{ResponseWriter: w}
		c := &Context{Writer: response, Request: r, response: response}
		if err := handler(c); err != nil {
			errorHandler(c, err)
		}
	})
}

// responseWriter tracks final commitment while retaining ResponseController
// access to optional capabilities through Unwrap. Hijack returns ErrNotSupported
// when the underlying writer does not support connection takeover.
type responseWriter struct {
	http.ResponseWriter
	written bool
	status  int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.ResponseWriter.WriteHeader(status)
	if status >= 200 || status == http.StatusSwitchingProtocols {
		w.written = true
		w.status = status
	}
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// Unwrap supports http.ResponseController for streaming and connection control.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// FlushError commits the status before flushing via http.ResponseController.
func (w *responseWriter) FlushError() error {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// Hijack transfers connection ownership and prevents later HTTP error responses.
// Unsupported writers return http.ErrNotSupported without committing a response.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	connection, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.written = true
	}
	return connection, buffer, err
}
