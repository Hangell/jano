package jano

import (
	"fmt"
	"net/http"
	"strconv"
)

// Param returns a route parameter using Go's Request.PathValue API. It works
// with Jano, http.ServeMux and routers that populate path values.
func Param(r *http.Request, name string) string { return r.PathValue(name) }

// ParamInt parses a decimal path parameter within the platform's int range.
// A missing, malformed or overflowing value returns zero and an HTTP 400 error.
// Signed values are accepted; validate domain rules such as positive IDs separately.
func ParamInt(r *http.Request, name string) (int, error) {
	value, err := integerParam(r, name, strconv.IntSize)
	return int(value), err
}

// ParamInt64 parses a decimal path parameter within the signed 64-bit range.
// It returns zero and an HTTP 400 error for missing, malformed or overflowing
// values. It does not write a response or impose positive-only ID semantics.
func ParamInt64(r *http.Request, name string) (int64, error) {
	return integerParam(r, name, 64)
}

func integerParam(r *http.Request, name string, bits int) (int64, error) {
	value, err := strconv.ParseInt(Param(r, name), 10, bits)
	if err != nil {
		return 0, &HTTPError{Status: http.StatusBadRequest, Message: fmt.Sprintf("Invalid integer parameter %q", name), Cause: err}
	}
	return value, nil
}
