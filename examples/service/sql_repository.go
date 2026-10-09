package main

import (
	"context"
	"database/sql"
	"errors"
)

// SQLUsers adapts an application-owned *sql.DB to UserRepository. This query
// uses PostgreSQL placeholders; choose the query/schema appropriate to your
// database. The application must register a driver and create the users table.
// No driver is imported here, so this example keeps Jano dependency-free.
type SQLUsers struct{ DB *sql.DB }

var _ UserRepository = SQLUsers{}

// Find retrieves a user, propagating cancellation to the SQL driver.
func (repository SQLUsers) Find(ctx context.Context, id int64) (User, error) {
	if repository.DB == nil {
		return User{}, errors.New("SQLUsers requires a database")
	}
	var user User
	err := repository.DB.QueryRowContext(ctx, "SELECT id, name FROM users WHERE id = $1", id).Scan(&user.ID, &user.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}
