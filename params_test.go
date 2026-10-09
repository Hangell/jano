package jano_test

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Hangell/jano"
)

func TestParamWithNativeServeMux(t *testing.T) {
	for _, tt := range []struct{ path, id string }{
		{"/users/42", "42"},
		{"/users/550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"/users/a%2Fb", "a/b"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
				if got := jano.Param(r, "id"); got != tt.id {
					t.Errorf("got id %q, want %q", got, tt.id)
				}
				if jano.Param(r, "missing") != "" {
					t.Error("missing parameter should be empty")
				}
				fmt.Fprint(w, jano.Param(r, "id"))
			})
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
			if w.Code != 200 || w.Body.String() != tt.id {
				t.Fatalf("got (%d, %q)", w.Code, w.Body.String())
			}
		})
	}
}

func ExampleParam() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, jano.Param(r, "id"))
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/users/42", nil))
	fmt.Println(w.Body.String())
	// Output: 42
}

func TestIntegerParams(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  int64
		valid bool
	}{
		{"42", 42, true}, {"0", 0, true}, {"-42", -42, true}, {"+42", 42, true}, {"0042", 42, true},
		{"9223372036854775807", math.MaxInt64, true}, {"-9223372036854775808", math.MinInt64, true},
		{"", 0, false}, {"abc", 0, false}, {"1.5", 0, false}, {" 42", 0, false}, {"0x2a", 0, false},
		{"9223372036854775808", 0, false}, {"-9223372036854775809", 0, false},
	} {
		t.Run(tt.value, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.SetPathValue("id", tt.value)
			got, err := jano.ParamInt64(r, "id")
			if got != tt.want || (err == nil) != tt.valid {
				t.Fatalf("got (%d,%v), want %d, valid %v", got, err, tt.want, tt.valid)
			}
			if !tt.valid {
				var httpError *jano.HTTPError
				var numberError *strconv.NumError
				if !errors.As(err, &httpError) || httpError.Status != 400 || !errors.As(err, &numberError) {
					t.Fatalf("incorrect error contract: %v", err)
				}
			}
		})
	}
	for _, value := range []string{strconv.FormatInt(int64(math.MaxInt), 10), strconv.FormatInt(int64(math.MinInt), 10), "42"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", value)
		want, _ := strconv.Atoi(value)
		got, err := jano.ParamInt(r, "id")
		if err != nil || got != want {
			t.Fatalf("platform int %s got (%d,%v)", value, got, err)
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	if value, err := jano.ParamInt(r, "missing"); err == nil || value != 0 {
		t.Fatal("missing int accepted")
	}
	r.SetPathValue("id", strconv.FormatUint(uint64(math.MaxInt)+1, 10))
	if value, err := jano.ParamInt(r, "id"); err == nil || value != 0 {
		t.Fatal("platform int overflow accepted")
	}
}

func TestIntegerParamThroughBothRouters(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, tt := range []struct {
			id     string
			status int
			body   string
		}{{"42", 200, "42"}, {"abc", 400, "invalid"}, {"9223372036854775808", 400, "invalid"}} {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id, err := jano.ParamInt64(r, "id")
				if err != nil {
					http.Error(w, "invalid", 400)
					return
				}
				fmt.Fprint(w, id)
			})
			var router http.Handler
			if native {
				mux := http.NewServeMux()
				mux.Handle("GET /users/{id}", handler)
				router = mux
			} else {
				app := jano.New()
				app.Get("/users/{id}", handler)
				router = app
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/users/"+tt.id, nil))
			if w.Code != tt.status || strings.TrimSpace(w.Body.String()) != tt.body {
				t.Fatalf("native %v, id %s got (%d,%q)", native, tt.id, w.Code, w.Body.String())
			}
		}
	}
}

func TestContextIntegerParams(t *testing.T) {
	for _, wide := range []bool{false, true} {
		for _, id := range []string{"42", "invalid", "9223372036854775808"} {
			app := jano.New()
			app.HandleContext("GET", "/users/{id}", func(c *jano.Context) error {
				var value int64
				var err error
				if wide {
					value, err = c.ParamInt64("id")
				} else {
					var n int
					n, err = c.ParamInt("id")
					value = int64(n)
				}
				if err != nil {
					return err
				}
				return c.JSON(200, map[string]int64{"id": value})
			})
			w := httptest.NewRecorder()
			app.ServeHTTP(w, httptest.NewRequest("GET", "/users/"+id, nil))
			want := 400
			if id == "42" {
				want = 200
			}
			if w.Code != want {
				t.Fatalf("wide %v, id %s got %d", wide, id, w.Code)
			}
			if want == 400 && (!strings.Contains(w.Body.String(), `"error"`) || strings.Contains(w.Body.String(), id)) {
				t.Fatalf("invalid error payload %q", w.Body.String())
			}
		}
	}
}
