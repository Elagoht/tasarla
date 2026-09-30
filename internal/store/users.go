package store

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// User is a person who has signed in at least once.
type User struct {
	ID         int64
	Issuer     string
	Subject    string
	Email      string
	Name       string
	Locale     string
	IsAdmin    bool
	DisabledAt *time.Time
	CreatedAt  time.Time
}

// Disabled reports whether the user may no longer sign in.
func (u User) Disabled() bool { return u.DisabledAt != nil }

// Identity is who an identity provider says signed in.
type Identity struct {
	Issuer  string
	Subject string
	Email   string
	Name    string
}

const userColumns = `id, issuer, subject, email, name, locale, is_admin, disabled_at, created_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.Name, &u.Locale, &u.IsAdmin, &u.DisabledAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func collectUsers(rows pgx.Rows, err error) ([]User, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpsertIdentity finds the user an identity provider signed in by (issuer,
// subject) and refreshes their email and name, or creates them. Only a user
// created here can be made admin by adminEmails; after that, adminship is the
// application's to manage.
func (s *Store) UpsertIdentity(ctx context.Context, id Identity, adminEmails []string, locale string) (User, error) {
	name := strings.TrimSpace(id.Name)
	if name == "" {
		name = id.Email
	}
	return scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO users (issuer, subject, email, name, locale, is_admin)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (issuer, subject) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name
		RETURNING `+userColumns,
		id.Issuer, id.Subject, id.Email, name, locale, isAdminEmail(id.Email, adminEmails)))
}

func isAdminEmail(email string, adminEmails []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	return email != "" && slices.ContainsFunc(adminEmails, func(a string) bool {
		return strings.ToLower(strings.TrimSpace(a)) == email
	})
}

// UserByID returns one user.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// UserByEmail returns the one active user with email, ignoring case.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	users, err := collectUsers(s.pool.Query(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE lower(email) = lower($1) AND disabled_at IS NULL
		LIMIT 2`, strings.TrimSpace(email)))
	switch {
	case err != nil:
		return User{}, err
	case len(users) == 0:
		return User{}, ErrNotFound
	case len(users) > 1:
		return User{}, ErrAmbiguousEmail
	}
	return users[0], nil
}

// Users returns every user, by name.
func (s *Store) Users(ctx context.Context) ([]User, error) {
	return collectUsers(s.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY lower(name), id`))
}

// SetAdmin grants or revokes adminship.
func (s *Store) SetAdmin(ctx context.Context, userID int64, admin bool) error {
	return s.changeAdmins(ctx, `UPDATE users SET is_admin = $2 WHERE id = $1`, userID, admin)
}

// SetDisabled disables or re-enables a user.
func (s *Store) SetDisabled(ctx context.Context, userID int64, disabled bool) error {
	return s.changeAdmins(ctx, `
		UPDATE users SET disabled_at = CASE WHEN $2::boolean THEN coalesce(disabled_at, now()) ELSE NULL END
		WHERE id = $1`, userID, disabled)
}

// changeAdmins runs update with the active admins locked, and refuses it when
// there were active admins before and none would remain.
func (s *Store) changeAdmins(ctx context.Context, update string, userID int64, value bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	countAdmins := func() (int, error) {
		var n int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL`).Scan(&n)
		return n, err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE is_admin AND disabled_at IS NULL FOR UPDATE`); err != nil {
		return err
	}
	before, err := countAdmins()
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, update, userID, value)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	after, err := countAdmins()
	if err != nil {
		return err
	}
	if before > 0 && after == 0 {
		return ErrLastAdmin
	}
	return tx.Commit(ctx)
}
