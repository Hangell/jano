package jano

import (
	"fmt"
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
	router.ServeHTTP(response, request) // Compile outside the timed section.
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

// BenchmarkRouteCount measures lookup scaling, excluding registration and compilation.
func BenchmarkRouteCount(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		for _, dynamic := range []bool{false, true} {
			kind := "static"
			if dynamic {
				kind = "parameter"
			}
			b.Run(fmt.Sprintf("%s/%d", kind, count), func(b *testing.B) {
				app := New()
				handler := func(http.ResponseWriter, *http.Request) {}
				for i := 0; i < count; i++ {
					path := fmt.Sprintf("/routes/%d", i)
					if dynamic {
						path += "/{id}"
					}
					app.Get(path, handler)
				}
				path := fmt.Sprintf("/routes/%d", count-1)
				if dynamic {
					path += "/42"
				}
				router := app.Router()
				request := httptest.NewRequest("GET", path, nil)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					router.ServeHTTP(response, request)
				}
			})
		}
	}
}
