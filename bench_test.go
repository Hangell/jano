package jano

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func benchmarkRoute(b *testing.B, route, path string) {
	app := New()
	app.Get(route, func(w http.ResponseWriter, r *http.Request) {})
	router := app.Router()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(response, request)
	}
}

func BenchmarkJano(b *testing.B) {
	benchmarkRoute(b, "/v1/{v1}", "/v1/anything")
}

func BenchmarkJanoSimple(b *testing.B) {
	benchmarkRoute(b, "/status", "/status")
}

func BenchmarkManyPathVariables(b *testing.B) {
	benchmarkRoute(b, "/v1/{v1}/{v2}/{v3}/{v4}/{v5}", "/v1/1/2/3/4/5")
}

func BenchmarkJanoNotFound(b *testing.B) {
	app := New()
	app.Get("/status", func(w http.ResponseWriter, r *http.Request) {})
	router := app.Router()
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// A fresh recorder avoids accumulating error bodies across iterations.
		router.ServeHTTP(httptest.NewRecorder(), request)
	}
}
