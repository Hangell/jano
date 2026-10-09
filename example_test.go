package jano_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Hangell/jano"
)

func ExampleJano_Router() {
	app := jano.New()
	app.Get("/people/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Person: %s", jano.Param(r, "id"))
	})
	response := httptest.NewRecorder()
	app.Router().ServeHTTP(response, httptest.NewRequest("GET", "/people/42", nil))
	fmt.Println(response.Code)
	fmt.Println(response.Body.String())
	// Output:
	// 200
	// Person: 42
}
