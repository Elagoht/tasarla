package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func TestNotificationsAndDedupe(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	n := store.NewNotification{UserID: f.member.ID, Kind: store.NotifyDueSoon, CardID: &a.ID,
		Payload: store.NotificationPayload{CardTitle: "A", BoardID: f.board.ID}, DedupeKey: "due_soon:1:2026-10-02",
		Email: &store.OutboxMessage{To: "member@example.com", Subject: "s", HTML: "<p>h</p>", Text: "t"}}
	created, err := f.s.CreateNotification(ctx, n)
	if err != nil || !created {
		t.Fatalf("first = %v, %v", created, err)
	}
	created, err = f.s.CreateNotification(ctx, n)
	if err != nil || created {
		t.Fatalf("duplicate = %v, %v; want not created", created, err)
	}
	n.DedupeKey = ""
	n.Email = nil
	if _, err := f.s.CreateNotification(ctx, n); err != nil {
		t.Fatal(err)
	}
	list, err := f.s.Notifications(ctx, f.member.ID, 10)
	if err != nil || len(list) != 2 || list[0].Payload.CardTitle != "A" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if c, _ := f.s.UnreadCount(ctx, f.member.ID); c != 2 {
		t.Errorf("unread = %d", c)
	}
	if err := f.s.MarkRead(ctx, f.lead.ID, list[0].ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("marking someone else's: %v", err)
	}
	if err := f.s.MarkRead(ctx, f.member.ID, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if c, _ := f.s.UnreadCount(ctx, f.member.ID); c != 1 {
		t.Errorf("unread after one = %d", c)
	}
	if err := f.s.MarkAllRead(ctx, f.member.ID); err != nil {
		t.Fatal(err)
	}
	if c, _ := f.s.UnreadCount(ctx, f.member.ID); c != 0 {
		t.Errorf("unread after all = %d", c)
	}
	if pending, _ := f.s.OutboxCount(ctx, "pending"); pending != 1 {
		t.Errorf("outbox pending = %d, want 1 (only the first had an email)", pending)
	}
}

func TestNotificationPrefs(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	prefs, err := f.s.NotificationPrefs(ctx, f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"assigned": true, "mentioned": true, "commented": false, "due_soon": true, "overdue": true, "unblocked": false}
	for k, v := range want {
		if prefs[k] != v {
			t.Errorf("default %s = %v", k, prefs[k])
		}
	}
	if err := f.s.SetNotificationPrefs(ctx, f.member.ID, map[string]bool{"assigned": false, "commented": true}); err != nil {
		t.Fatal(err)
	}
	prefs, _ = f.s.NotificationPrefs(ctx, f.member.ID)
	if prefs["assigned"] || !prefs["commented"] || !prefs["mentioned"] {
		t.Errorf("prefs = %v", prefs)
	}
	if err := f.s.SetLocale(ctx, f.member.ID, "en"); err != nil {
		t.Fatal(err)
	}
	if u, _ := f.s.UserByID(ctx, f.member.ID); u.Locale != "en" {
		t.Errorf("locale = %q", u.Locale)
	}
}

func TestOutboxRetriesThenGivesUp(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	f.s.CreateNotification(ctx, store.NewNotification{UserID: f.member.ID, Kind: store.NotifyAssigned, CardID: &a.ID,
		Email: &store.OutboxMessage{To: "member@example.com", Subject: "s", HTML: "h", Text: "t"}})
	down := errors.New("smtp down")
	now := time.Now()
	for i := 1; i <= 8; i++ {
		sent, failed, err := f.s.ProcessOutbox(ctx, now, 20, func(store.OutboxMessage) error { return down })
		if err != nil {
			t.Fatal(err)
		}
		if sent != 0 || failed != 1 {
			t.Fatalf("attempt %d: sent %d failed %d", i, sent, failed)
		}
		// Not due again until the backoff has passed.
		if s, f2, _ := f.s.ProcessOutbox(ctx, now, 20, func(store.OutboxMessage) error { return nil }); s+f2 != 0 {
			t.Fatalf("attempt %d: retried before its time", i)
		}
		now = now.Add(7 * time.Hour)
	}
	if n, _ := f.s.OutboxCount(ctx, "failed"); n != 1 {
		t.Fatalf("failed = %d, want 1 after 8 attempts", n)
	}
	if n, _ := f.s.OutboxCount(ctx, "pending"); n != 0 {
		t.Fatalf("pending = %d", n)
	}
}

func TestOutboxSendsWhenSMTPComesBack(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	f.s.CreateNotification(ctx, store.NewNotification{UserID: f.member.ID, Kind: store.NotifyAssigned,
		Email: &store.OutboxMessage{To: "member@example.com", Subject: "Hello", HTML: "h", Text: "t"}})
	f.s.ProcessOutbox(ctx, time.Now(), 20, func(store.OutboxMessage) error { return errors.New("down") })
	var got []store.OutboxMessage
	sent, _, err := f.s.ProcessOutbox(ctx, time.Now().Add(time.Hour), 20, func(m store.OutboxMessage) error {
		got = append(got, m)
		return nil
	})
	if err != nil || sent != 1 || got[0].Subject != "Hello" || got[0].To != "member@example.com" {
		t.Fatalf("sent %d, %v, %+v", sent, err, got)
	}
	if n, _ := f.s.OutboxCount(ctx, "sent"); n != 1 {
		t.Errorf("sent rows = %d", n)
	}
}

func TestDueCandidates(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	day := func(s string) *time.Time { d, _ := time.ParseInLocation(time.DateOnly, s, time.Local); return &d }
	mk := func(title string, col int, due *time.Time) store.Card {
		c := f.card(t, col, title)
		c, err := f.s.UpdateCard(ctx, f.board.ID, c.ID, c.Version, store.CardFields{Title: title, DueDate: due, AssigneeID: &f.member.ID}, 0)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	mk("soon", 0, day("2026-10-02"))
	mk("late", 0, day("2026-09-28"))
	mk("far", 0, day("2026-10-20"))
	mk("done", 2, day("2026-09-28"))
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-01 09:00", time.Local)
	got, err := f.s.DueCandidates(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]string{}
	for _, d := range got {
		byTitle[d.Card.Title] = d.Kind
	}
	if len(byTitle) != 2 || byTitle["soon"] != store.NotifyDueSoon || byTitle["late"] != store.NotifyOverdue {
		t.Fatalf("candidates = %v", byTitle)
	}
}

func TestNewlyUnblocked(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a, b, c := f.card(t, 0, "A"), f.card(t, 0, "B"), f.card(t, 0, "C")
	f.s.AddDependency(ctx, f.board.ID, a.ID, c.ID)
	f.s.AddDependency(ctx, f.board.ID, b.ID, c.ID)
	a, _ = f.s.Card(ctx, f.board.ID, a.ID)
	f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[2].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version, Actor: rules.Actor{}})
	if got, _ := f.s.NewlyUnblocked(ctx, a.ID); len(got) != 0 {
		t.Fatalf("C still waits on B: %+v", got)
	}
	f.s.ArchiveCard(ctx, f.board.ID, b.ID, 0)
	got, err := f.s.NewlyUnblocked(ctx, b.ID)
	if err != nil || len(got) != 1 || got[0].ID != c.ID {
		t.Fatalf("unblocked = %+v, %v", got, err)
	}
}
