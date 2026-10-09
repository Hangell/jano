package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
)

// Exercise the adapter through database/sql using a minimal local test driver.
// This does not replace integration testing against a real PostgreSQL schema.
func init() { sql.Register("jano-example-test", repositoryDriver{}) }

type repositoryDriver struct{}

func (repositoryDriver) Open(name string) (driver.Conn, error) {
	return &repositoryConnection{mode: name}, nil
}

type repositoryConnection struct{ mode string }

func (*repositoryConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (*repositoryConnection) Close() error              { return nil }
func (*repositoryConnection) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }
func (c *repositoryConnection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if query != "SELECT id, name FROM users WHERE id = $1" || len(args) != 1 || args[0].Value != int64(42) {
		return nil, errors.New("unexpected query or arguments")
	}
	if c.mode == "error" {
		return nil, errDriverFailure
	}
	return &repositoryRows{empty: c.mode == "missing"}, nil
}

var errDriverFailure = errors.New("database failure")

type repositoryRows struct{ empty, read bool }

func (*repositoryRows) Columns() []string { return []string{"id", "name"} }
func (*repositoryRows) Close() error      { return nil }
func (r *repositoryRows) Next(values []driver.Value) error {
	if r.empty || r.read {
		return io.EOF
	}
	r.read = true
	values[0], values[1] = int64(42), "Jane"
	return nil
}

func TestSQLRepository(t *testing.T) {
	for _, mode := range []string{"found", "missing", "error"} {
		t.Run(mode, func(t *testing.T) {
			db, err := sql.Open("jano-example-test", mode)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := SQLUsers{DB: db}
			user, err := repository.Find(context.Background(), 42)
			switch mode {
			case "found":
				if err != nil || user != (User{ID: 42, Name: "Jane"}) {
					t.Fatalf("got %+v, %v", user, err)
				}
			case "missing":
				if !errors.Is(err, ErrUserNotFound) {
					t.Fatalf("got %v", err)
				}
			case "error":
				if !errors.Is(err, errDriverFailure) {
					t.Fatalf("got %v", err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := repository.Find(ctx, 42); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation not propagated: %v", err)
			}
		})
	}
	if _, err := (SQLUsers{}).Find(context.Background(), 42); err == nil {
		t.Fatal("nil database accepted")
	}
}
