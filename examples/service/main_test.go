package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type repositoryFunc func(context.Context, int64) (User, error)

func (f repositoryFunc) Find(ctx context.Context, id int64) (User, error) { return f(ctx, id) }

func TestRepositoryIntegration(t *testing.T) {
	type key struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "request"))
	defer cancel()
	called := false
	app := NewApplication(repositoryFunc(func(received context.Context, id int64) (User, error) {
		called = true
		if id != 42 || received.Value(key{}) != "request" {
			t.Error("request data did not reach repository")
		}
		if _, ok := received.Deadline(); !ok {
			t.Error("deadline did not reach repository")
		}
		cancel()
		if !errors.Is(received.Err(), context.Canceled) {
			t.Error("cancellation did not reach repository")
		}
		return User{ID: id, Name: "Jane"}, nil
	}))
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/users/42", nil).WithContext(ctx))
	if !called || w.Code != 200 || !strings.Contains(w.Body.String(), `"id":42`) {
		t.Fatalf("got (%d, %q), called %v", w.Code, w.Body.String(), called)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
}

func TestRepositoryErrors(t *testing.T) {
	for _, tt := range []struct {
		name, path string
		err        error
		status     int
	}{
		{"not found", "/api/v1/users/42", ErrUserNotFound, 404},
		{"timeout", "/api/v1/users/42", context.DeadlineExceeded, 504},
		{"internal", "/api/v1/users/42", errors.New("private database password"), 500},
		{"invalid ID", "/api/v1/users/nope", nil, 400},
		{"negative ID", "/api/v1/users/-1", nil, 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApplication(repositoryFunc(func(context.Context, int64) (User, error) {
				if tt.status == 400 {
					t.Error("repository called for invalid ID")
				}
				return User{}, tt.err
			}))
			w := httptest.NewRecorder()
			app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != tt.status || strings.Contains(w.Body.String(), "password") {
				t.Fatalf("got (%d, %q)", w.Code, w.Body.String())
			}
		})
	}
}

func TestShutdownWaitsForActiveRequest(t *testing.T) {
	started, release, shutdownStarted := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "finished")
	}))
	defer func() { releaseOnce.Do(func() { close(release) }); server.Close() }()
	server.Config.RegisterOnShutdown(func() { close(shutdownStarted) })
	clientDone := make(chan error, 1)
	go func() {
		response, err := server.Client().Get(server.URL)
		if err == nil {
			defer response.Body.Close()
			var body []byte
			body, err = io.ReadAll(response.Body)
			if err == nil && string(body) != "finished" {
				err = fmt.Errorf("incomplete body %q", body)
			}
		}
		clientDone <- err
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := shutdownOnCancel(ctx, server.Config)
	cancel()
	<-shutdownStarted
	select {
	case err := <-done:
		t.Fatalf("shutdown returned while request was active: %v", err)
	default:
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-clientDone; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
