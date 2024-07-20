package jano

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkJano(b *testing.B) {
	app := New()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	app.Get("/v1/{v1}", handler)

	request, _ := http.NewRequest("GET", "/v1/anything", nil)
	for i := 0; i < b.N; i++ {
		app.Router().ServeHTTP(nil, request)
	}
}

func BenchmarkJanoSimple(b *testing.B) {
	app := New()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	app.Get("/status", handler)

	request, _ := http.NewRequest("GET", "/status", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Router().ServeHTTP(nil, request)
	}
}

func BenchmarkJanoAlternativeInRegexp(b *testing.B) {
	app := New()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	app.Get("/v1/{v1:(?:a|b)}", handler)

	requestA, _ := http.NewRequest("GET", "/v1/a", nil)
	requestB, _ := http.NewRequest("GET", "/v1/b", nil)
	for i := 0; i < b.N; i++ {
		app.Router().ServeHTTP(nil, requestA)
		app.Router().ServeHTTP(nil, requestB)
	}
}

func BenchmarkManyPathVariables(b *testing.B) {
	app := New()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	app.Get("/v1/{v1}/{v2}/{v3}/{v4}/{v5}", handler)

	matchingRequest, _ := http.NewRequest("GET", "/v1/1/2/3/4/5", nil)
	notMatchingRequest, _ := http.NewRequest("GET", "/v1/1/2/3/4", nil)
	recorder := httptest.NewRecorder()
	for i := 0; i < b.N; i++ {
		app.Router().ServeHTTP(nil, matchingRequest)
		app.Router().ServeHTTP(recorder, notMatchingRequest)
	}
}
