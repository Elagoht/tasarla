package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/db/dbtest"
	"kanban/internal/store"
)

func newStore(t *testing.T) *store.Store { return store.New(dbtest.New(t)) }

func identity(sub, email string) store.Identity {
	return store.Identity{Issuer: "https://idp.test", Subject: sub, Email: email, Name: "User " + sub}
}

func mustUpsert(t *testing.T, s *store.Store, id store.Identity, admins ...string) store.User {
	t.Helper()
	u, err := s.UpsertIdentity(context.Background(), id, admins, "tr")
	if err != nil {
		t.Fatalf("UpsertIdentity(%s): %v", id.Subject, err)
	}
	return u
}

func TestUpsertIdentityCreatesThenRefreshes(t *testing.T) {
	s := newStore(t)
	first := mustUpsert(t, s, identity("s1", "old@example.com"))
	if first.Locale != "tr" || first.IsAdmin || first.Disabled() {
		t.Fatalf("new user = %+v", first)
	}
	second := mustUpsert(t, s, store.Identity{Issuer: "https://idp.test", Subject: "s1", Email: "new@example.com", Name: "New Name"})
	if second.ID != first.ID {
		t.Fatalf("same (issuer, subject) produced a new user: %d vs %d", second.ID, first.ID)
	}
	if second.Email != "new@example.com" || second.Name != "New Name" {
		t.Errorf("email/name not refreshed: %+v", second)
	}
}

func TestUpsertIdentityEmptyNameFallsBackToEmail(t *testing.T) {
	s := newStore(t)
	u := mustUpsert(t, s, store.Identity{Issuer: "https://idp.test", Subject: "s1", Email: "a@example.com"})
	if u.Name != "a@example.com" {
		t.Errorf("Name = %q, want the email", u.Name)
	}
}

func TestEmailIsNotIdentity(t *testing.T) {
	s := newStore(t)
	a := mustUpsert(t, s, identity("s1", "same@example.com"))
	b := mustUpsert(t, s, identity("s2", "same@example.com"))
	if a.ID == b.ID {
		t.Fatal("two subjects with one email became one user")
	}
}

func TestAdminEmailsGrantAdminOnlyOnFirstSignIn(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	admins := []string{" alice@example.com "}

	alice := mustUpsert(t, s, identity("alice", "Alice@Example.COM"), admins...)
	if !alice.IsAdmin {
		t.Fatal("ADMIN_EMAILS match (case/space-insensitive) did not grant admin")
	}
	bob := mustUpsert(t, s, identity("bob", "bob@example.com"), admins...)
	if bob.IsAdmin {
		t.Fatal("a non-listed email became admin")
	}

	// Revoked in the app, alice must not get it back by signing in again.
	if err := s.SetAdmin(ctx, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdmin(ctx, alice.ID, false); err != nil {
		t.Fatal(err)
	}
	again := mustUpsert(t, s, identity("alice", "alice@example.com"), admins...)
	if again.IsAdmin {
		t.Fatal("admin was granted again on a later sign-in")
	}
}

func TestUserByEmail(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := mustUpsert(t, s, identity("s1", "Ada@Example.com"))

	got, err := s.UserByEmail(ctx, " ada@example.com ")
	if err != nil || got.ID != a.ID {
		t.Fatalf("UserByEmail = %+v, %v", got, err)
	}
	if _, err := s.UserByEmail(ctx, "nobody@example.com"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown email: err = %v, want ErrNotFound", err)
	}

	mustUpsert(t, s, identity("s2", "ada@example.com"))
	if _, err := s.UserByEmail(ctx, "ada@example.com"); !errors.Is(err, store.ErrAmbiguousEmail) {
		t.Errorf("two users: err = %v, want ErrAmbiguousEmail", err)
	}

	if err := s.SetDisabled(ctx, a.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserByEmail(ctx, "ada@example.com"); err != nil {
		t.Errorf("a disabled user must not count: err = %v", err)
	}
}

func TestTheLastActiveAdminStays(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	alice := mustUpsert(t, s, identity("alice", "alice@example.com"), "alice@example.com")
	bob := mustUpsert(t, s, identity("bob", "bob@example.com"))

	if err := s.SetAdmin(ctx, alice.ID, false); !errors.Is(err, store.ErrLastAdmin) {
		t.Errorf("demoting the only admin: err = %v, want ErrLastAdmin", err)
	}
	if err := s.SetDisabled(ctx, alice.ID, true); !errors.Is(err, store.ErrLastAdmin) {
		t.Errorf("disabling the only admin: err = %v, want ErrLastAdmin", err)
	}
	if err := s.SetDisabled(ctx, bob.ID, true); err != nil {
		t.Errorf("disabling a regular user: %v", err)
	}

	if err := s.SetAdmin(ctx, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDisabled(ctx, bob.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdmin(ctx, alice.ID, false); err != nil {
		t.Errorf("demoting one of two admins: %v", err)
	}
	got, _ := s.UserByID(ctx, alice.ID)
	if got.IsAdmin {
		t.Error("alice is still admin")
	}
}

func TestNoAdminsAtAllDoesNotBlockOtherChanges(t *testing.T) {
	s := newStore(t)
	u := mustUpsert(t, s, identity("s1", "a@example.com"))
	if err := s.SetDisabled(context.Background(), u.ID, true); err != nil {
		t.Fatalf("SetDisabled with no admins: %v", err)
	}
}

func TestUnknownUser(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.UserByID(ctx, 999); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("UserByID: %v", err)
	}
	if err := s.SetDisabled(ctx, 999, true); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetDisabled: %v", err)
	}
	if err := s.SetAdmin(ctx, 999, true); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetAdmin: %v", err)
	}
}
