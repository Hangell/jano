// The service example demonstrates dependency injection without a framework ORM.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hangell/jano"
	"github.com/Hangell/jano/middleware"
)

// User is the domain response, independent of HTTP and the database driver.
type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

var ErrUserNotFound = errors.New("user not found")

// UserRepository is implemented by memory, SQL or ORM-backed adapters.
// Every operation receives the request context for cancellation and deadlines.
type UserRepository interface {
	Find(context.Context, int64) (User, error)
}

// NewApplication injects a repository once, rather than putting database handles
// in a global map or opening connections on each request.
func NewApplication(repository UserRepository) *jano.Jano {
	app := jano.New(jano.WithMethodNotAllowed())
	app.Use(middleware.Recovery(func(r *http.Request, value any) { log.Printf("request panic: %v", value) }))
	app.Use(middleware.RequestID)
	app.Use(middleware.ContextTimeout(2 * time.Second))
	api := app.Group("/api").Group("/v1")
	api.HandleContext(http.MethodGet, "/users/{id}", func(c *jano.Context) error {
		id, err := c.ParamInt64("id")
		if err != nil || id <= 0 {
			return jano.NewHTTPError(http.StatusBadRequest, "Invalid user ID")
		}
		user, err := repository.Find(c.Context(), id)
		switch {
		case errors.Is(err, ErrUserNotFound):
			return jano.NewHTTPError(http.StatusNotFound, "User not found")
		case errors.Is(err, context.DeadlineExceeded):
			return jano.NewHTTPError(http.StatusGatewayTimeout, "Repository timeout")
		case err != nil:
			return err
		}
		return c.JSON(http.StatusOK, user)
	})
	return app
}

type memoryUsers struct{}

func (memoryUsers) Find(ctx context.Context, id int64) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	if id != 1 {
		return User{}, ErrUserNotFound
	}
	return User{ID: 1, Name: "Jane"}, nil
}

func main() {
	// Replace memoryUsers with SQLUsers{DB: db} for PostgreSQL, or a repository
	// using another database/ORM. Open and close the database in main, not in Jano.
	app := NewApplication(memoryUsers{})
	server := &http.Server{Addr: ":9001", Handler: app, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdown := shutdownOnCancel(ctx, server)
	log.Printf("Service example listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	// ListenAndServe returns as soon as listeners close; wait for active requests.
	if err := <-shutdown; err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func shutdownOnCancel(ctx context.Context, server *http.Server) <-chan error {
	done := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		done <- err
		close(done)
	}()
	return done
}
