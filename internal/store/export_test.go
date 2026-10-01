package store

import "github.com/jackc/pgx/v5/pgxpool"

// Pool lets tests hold locks the way a concurrent request would.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
