// Package middleware provides optional net/http middleware for Jano and other routers.
package middleware

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"time"
)

// Recovery converts panics into generic HTTP 500 responses. An optional callback
// receives the panic and request for logging. It must not panic or write a response.
// A committed response is left intact. Wrap the complete app to recover fallback
// panics too: middleware.Recovery(nil)(app).
func Recovery(report func(*http.Request, any)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &recoveryWriter{ResponseWriter: w}
			defer func() {
				if value := recover(); value != nil {
					if value == http.ErrAbortHandler {
						panic(value)
					}
					if report != nil {
						report(r, value)
					}
					if !response.written {
						http.Error(response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}()
			next.ServeHTTP(response, r)
		})
	}
}

type recoveryWriter struct {
	http.ResponseWriter
	written bool
}

func (w *recoveryWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.ResponseWriter.WriteHeader(status)
	if status >= 200 || status == http.StatusSwitchingProtocols {
		w.written = true
	}
}
func (w *recoveryWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func (w *recoveryWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *recoveryWriter) FlushError() error {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// BodyLimit bounds a request body. A known Content-Length exceeding the limit
// receives 413 immediately; streamed bodies return *http.MaxBytesError when read
// beyond the limit. Handlers must handle read errors. The limit must be positive.
func BodyLimit(limit int64) func(http.Handler) http.Handler {
	if limit <= 0 {
		panic("jano/middleware: body limit must be positive")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r = r.WithContext(r.Context())
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ContextTimeout sets a deadline for downstream context-aware operations. It does
// not forcibly stop handlers or write a timeout response. The duration must be
// positive. Use request context in database calls so cancellation can propagate.
func ContextTimeout(duration time.Duration) func(http.Handler) http.Handler {
	if duration <= 0 {
		panic("jano/middleware: timeout must be positive")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type requestIDKey struct{}

// RequestID assigns a fresh random ID to every request, ignoring caller-supplied
// IDs, and writes it to X-Request-ID. It is available through RequestIDFromContext.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			http.Error(w, "Unable to create request ID", http.StatusInternalServerError)
			return
		}
		id := hex.EncodeToString(bytes[:])
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the generated request ID, or an empty string.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Hijack transfers connection ownership and prevents later HTTP error responses.
// Unsupported writers return http.ErrNotSupported without committing a response.
func (w *recoveryWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	connection, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.written = true
	}
	return connection, buffer, err
}
