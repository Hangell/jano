package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hangell/jano"
	"github.com/Hangell/jano/examples/api/handlers"
	"github.com/Hangell/jano/examples/api/routes"
)

func TestPeopleAPI(t *testing.T) {
	app := jano.New()
	routes.SetupRoutes(app)
	router := app.Router()
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got status %d, want %d; body %q", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	decodePerson := func(w *httptest.ResponseRecorder) handlers.Person {
		t.Helper()
		var person handlers.Person
		if err := json.Unmarshal(w.Body.Bytes(), &person); err != nil {
			t.Fatal(err)
		}
		return person
	}

	created := decodePerson(request("POST", "/people", `{"name":"Jane","age":30,"student":false}`, http.StatusCreated))
	if created.ID <= 0 || created.Name != "Jane" || created.Age != 30 || created.Student {
		t.Fatalf("unexpected created person: %+v", created)
	}
	path := fmt.Sprintf("/people/%d", created.ID)
	t.Cleanup(func() { router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("DELETE", path, nil)) })
	if got := decodePerson(request("GET", path, "", 200)); got != created {
		t.Fatalf("retrieved %+v, want %+v", got, created)
	}
	var people []handlers.Person
	if err := json.Unmarshal(request("GET", "/people", "", 200).Body.Bytes(), &people); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, person := range people {
		if person == created {
			found = true
		}
	}
	if !found {
		t.Fatalf("created person missing from list: %+v", people)
	}

	updated := decodePerson(request("PUT", path, `{"name":"Jane Smith","age":31,"student":true}`, 200))
	want := handlers.Person{ID: created.ID, Name: "Jane Smith", Age: 31, Student: true}
	if updated != want {
		t.Fatalf("updated %+v, want %+v", updated, want)
	}
	if got := decodePerson(request("GET", path, "", 200)); got != want {
		t.Fatalf("update was not persisted: %+v", got)
	}

	for _, method := range []string{"GET", "PUT", "DELETE"} {
		request(method, "/people/not-a-number", `{}`, 400)
		request(method, "/people/0", `{}`, 400)
		request(method, "/people/-1", `{}`, 400)
		request(method, "/people/9223372036854775808", `{}`, 400)
	}
	request("POST", "/people", `{`, 400)
	request("PUT", path, `{`, 400)
	if got := decodePerson(request("GET", path, "", 200)); got != want {
		t.Fatalf("invalid update modified person: %+v", got)
	}
	deleted := request("DELETE", path, "", 204)
	if deleted.Body.Len() != 0 {
		t.Fatal("delete response must be empty")
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		request(method, path, `{}`, 404)
	}
	fallback := request("GET", "/missing", "", 404)
	if !strings.Contains(fallback.Body.String(), "Custom 404") {
		t.Fatalf("unexpected fallback: %q", fallback.Body.String())
	}
}
