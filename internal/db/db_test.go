package db_test

import (
	"context"
	"testing"

	"kanban/internal/db"
	"kanban/internal/db/dbtest"
)

func TestMigrateCreatesTablesAndIsIdempotent(t *testing.T) {
	pool := dbtest.New(t) // already migrated once
	ctx := context.Background()

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	for _, table := range []string{"users", "teams", "team_members"} {
		var exists bool
		err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("table %s missing (err=%v)", table, err)
		}
	}
}

func TestUsersAreUniqueByIssuerAndSubject(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	insert := `INSERT INTO users (issuer, subject, email, name, locale) VALUES ('https://idp', 'sub-1', 'a@x', 'A', 'tr')`
	if _, err := pool.Exec(ctx, insert); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := pool.Exec(ctx, insert); err == nil {
		t.Fatal("second insert with the same (issuer, subject) succeeded")
	}
}
