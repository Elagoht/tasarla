// Package store reads and writes the application's data in PostgreSQL.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound reports that the row asked for does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrLastAdmin refuses a change that would leave no active admin.
	ErrLastAdmin = errors.New("store: the last active admin cannot be removed")
	// ErrAmbiguousEmail reports that more than one active user has an email.
	ErrAmbiguousEmail = errors.New("store: more than one active user has this email")
)

// Store is the application's data access.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store over pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// exactlyOne turns a statement that touched no row into ErrNotFound.
func exactlyOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
