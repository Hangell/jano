package jano

import (
	"errors"
	"net/http"
)

// ErrResponseCommitted is returned when a response helper would write a second response.
var ErrResponseCommitted = errors.New("jano: response already committed")

// HTTPError describes an intentional client-visible HTTP error. Cause is retained
// for errors.Is/As and logging but is never serialized by the default policy.
type HTTPError struct {
	Status  int
	Message string
	Cause   error
}

// NewHTTPError creates an HTTP error with a client-safe message.
func NewHTTPError(status int, message string) *HTTPError {
	return &HTTPError{Status: status, Message: message}
}

// Error implements error.
func (e *HTTPError) Error() string { return e.Message }

// Unwrap exposes the underlying cause.
func (e *HTTPError) Unwrap() error { return e.Cause }

// ErrorHandler receives handler errors, including errors after response commitment.
// Custom handlers can log such errors but must not write a second response.
type ErrorHandler func(*Context, error)

// DefaultErrorHandler renders JSON errors before response commitment. Unexpected
// errors become a generic 500 response. Explicit HTTPError messages are public.
func DefaultErrorHandler(c *Context, err error) {
	if c.Written() {
		return
	}
	status, message := http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)
	var httpError *HTTPError
	if errors.As(err, &httpError) && httpError != nil && httpError.Status >= 400 && httpError.Status <= 599 {
		status, message = httpError.Status, httpError.Message
		if message == "" {
			message = http.StatusText(status)
		}
	}
	// The payload contains strings only; an encoding failure is impossible.
	_ = c.JSON(status, map[string]string{"error": message})
}
