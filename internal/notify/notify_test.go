package notify_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/db/dbtest"
	"kanban/internal/notify"
	"kanban/internal/store"
)

type fixture struct {
	s       *store.Store
	n       *notify.Notifier
	tags    []string
	ada     store.User // Turkish
	bob     store.User // English
	board   store.Board
	cols    []store.Column
	invoked int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{s: store.New(dbtest.New(t))}
	ctx := context.Background()
	tr := i18n.New(i18n.Options{FS: os.DirFS("../.."), Dir: "locales"})
	app, err := collage.New(&collage.Config{
		Template: collage.TemplateConfig{FS: fstest.MapFS{"t/p.html": {Data: []byte("<p></p>")}}, Root: "t", Extension: ".html"},
		Locale:   collage.LocaleConfig{Default: "tr", Supported: []string{"tr", "en"}},
		Security: collage.SecurityConfig{CSRFKey: []byte(strings.Repeat("k", 32))},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Plugins:  []collage.Plugin{tr},
	})
	if err != nil {
		t.Fatal(err)
	}
	card := collage.NewPage("card").WithContent(collage.NewFragment("c", "p.html").Build()).
		WithPath("tr", "/boards/{id}/cards/{card}").WithPath("en", "/boards/{id}/cards/{card}").Build()
	settings := collage.NewPage("me-settings").WithContent(collage.NewFragment("s", "p.html").Build()).
		WithPath("tr", "/me/settings").WithPath("en", "/me/settings").Build()
	app.RegisterPage(card)
	app.RegisterPage(settings)
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	f.n = &notify.Notifier{
		Store: f.s, I18n: tr, BaseURL: "https://pano.example.com", URL: app.URL,
		Invalidate: func(_ context.Context, tags ...string) error { f.tags = append(f.tags, tags...); return nil },
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	f.ada, _ = f.s.UpsertIdentity(ctx, store.Identity{Issuer: "i", Subject: "ada", Email: "ada@example.com", Name: "Ada"}, nil, "tr")
	f.bob, _ = f.s.UpsertIdentity(ctx, store.Identity{Issuer: "i", Subject: "bob", Email: "bob@example.com", Name: "Bob"}, nil, "en")
	team, _ := f.s.CreateTeam(ctx, "T")
	f.s.AddMember(ctx, team.ID, f.ada.ID, store.RoleLead)
	f.s.AddMember(ctx, team.ID, f.bob.ID, store.RoleMember)
	f.board, _ = f.s.CreateBoard(ctx, team.ID, "Sprint", []string{"Todo", "Done"})
	f.cols, _ = f.s.Columns(ctx, f.board.ID)
	return f
}

func (f *fixture) outbox(t *testing.T) []store.OutboxMessage {
	t.Helper()
	var got []store.OutboxMessage
	if _, _, err := f.s.ProcessOutbox(context.Background(), time.Now().Add(time.Minute), 100, func(m store.OutboxMessage) error {
		got = append(got, m)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestEmitWritesInTheRecipientsLanguage(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	card, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Payments", f.ada.ID)
	err := f.n.Emit(ctx,
		notify.Event{Kind: store.NotifyAssigned, To: f.bob.ID, Actor: f.ada.ID, ActorName: "Ada", Card: card, BoardName: "Sprint"},
		notify.Event{Kind: store.NotifyAssigned, To: f.ada.ID, Actor: f.ada.ID, ActorName: "Ada", Card: card, BoardName: "Sprint"},
	)
	if err != nil {
		t.Fatal(err)
	}
	mails := f.outbox(t)
	if len(mails) != 1 {
		t.Fatalf("mails = %d, want 1 (no mail for your own action)", len(mails))
	}
	m := mails[0]
	if m.To != "bob@example.com" || !strings.Contains(m.Subject, "Ada assigned you to a card: Payments") {
		t.Fatalf("mail = %+v", m)
	}
	link := "https://pano.example.com/en/boards/" + itoa(f.board.ID) + "/cards/" + itoa(card.ID)
	for _, body := range []string{m.Text, m.HTML} {
		if !strings.Contains(body, "Hello Bob,") || !strings.Contains(body, link) || !strings.Contains(body, "/en/me/settings") {
			t.Errorf("body lacks the greeting, the link or the settings link:\n%s", body)
		}
	}
	if n, _ := f.s.UnreadCount(ctx, f.bob.ID); n != 1 {
		t.Errorf("bob unread = %d", n)
	}
	if n, _ := f.s.UnreadCount(ctx, f.ada.ID); n != 0 {
		t.Errorf("ada was notified of her own action")
	}
	if len(f.tags) != 1 || f.tags[0] != "notifications:"+itoa(f.bob.ID) {
		t.Errorf("invalidated = %v", f.tags)
	}
}

func TestEmailFollowsThePreference(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	card, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "C", f.ada.ID)
	f.n.Emit(ctx, notify.Event{Kind: store.NotifyCommented, To: f.bob.ID, Actor: f.ada.ID, ActorName: "Ada", Card: card, Text: "hi"})
	if mails := f.outbox(t); len(mails) != 0 {
		t.Fatalf("commented is off by default, got %d mails", len(mails))
	}
	if n, _ := f.s.UnreadCount(ctx, f.bob.ID); n != 1 {
		t.Fatalf("the in-app notification is still made: %d", n)
	}
	f.s.SetNotificationPrefs(ctx, f.bob.ID, map[string]bool{store.NotifyCommented: true})
	f.n.Emit(ctx, notify.Event{Kind: store.NotifyCommented, To: f.bob.ID, Actor: f.ada.ID, ActorName: "Ada", Card: card, Text: "again"})
	if mails := f.outbox(t); len(mails) != 1 {
		t.Fatalf("mails after opting in = %d", len(mails))
	}
}

func TestDueSoonComesOnceAndAgainWhenTheDateChanges(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	card, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Report", f.ada.ID)
	due, _ := time.ParseInLocation(time.DateOnly, "2026-10-02", time.Local)
	card, err := f.s.UpdateCard(ctx, f.board.ID, card.ID, card.Version, store.CardFields{Title: "Report", AssigneeID: &f.ada.ID, DueDate: &due}, f.bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-01 09:00", time.Local)
	s := notify.Scheduler{Store: f.s, Notifier: f.n, Now: func() time.Time { return now }}
	for i := 0; i < 3; i++ {
		if err := s.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	mails := f.outbox(t)
	if len(mails) != 1 || !strings.Contains(mails[0].Subject, "Report kartının son tarihi yaklaşıyor (2026-10-02)") {
		t.Fatalf("mails = %+v", mails)
	}
	later, _ := time.ParseInLocation(time.DateOnly, "2026-10-03", time.Local)
	f.s.UpdateCard(ctx, f.board.ID, card.ID, card.Version, store.CardFields{Title: "Report", AssigneeID: &f.ada.ID, DueDate: &later}, f.bob.ID)
	now = now.Add(24 * time.Hour)
	s.Tick(ctx)
	s.Tick(ctx)
	if mails := f.outbox(t); len(mails) != 1 || !strings.Contains(mails[0].Subject, "2026-10-03") {
		t.Fatalf("after the date changed: %+v", mails)
	}
}

func TestWorkerSendsTheOutbox(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	card, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "C", f.ada.ID)
	f.n.Emit(ctx, notify.Event{Kind: store.NotifyAssigned, To: f.bob.ID, Actor: f.ada.ID, ActorName: "Ada", Card: card})
	var sent []string
	w := notify.Worker{Store: f.s, Send: func(_ context.Context, m store.OutboxMessage) error {
		sent = append(sent, m.To)
		return nil
	}, Log: f.n.Log}
	if err := w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0] != "bob@example.com" {
		t.Fatalf("sent = %v", sent)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// A user who left the team hears nothing more about its cards (spec §6).
func TestNoNotificationsAcrossTheTeamBoundary(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	card, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Secret plan", f.ada.ID)
	board, _ := f.s.Board(ctx, f.board.ID)
	f.s.RemoveMember(ctx, board.TeamID, f.bob.ID)
	f.n.Emit(ctx, notify.Event{Kind: store.NotifyAssigned, To: f.bob.ID, Actor: f.ada.ID, ActorName: "Ada", Card: card, BoardName: "Sprint"})
	if n, _ := f.s.UnreadCount(ctx, f.bob.ID); n != 0 {
		t.Fatalf("a former member was notified: %d", n)
	}
	if mails := f.outbox(t); len(mails) != 0 {
		t.Fatalf("a former member was mailed: %+v", mails)
	}
}

// The reminder is per person: a card handed to someone else reminds them too.
func TestDueSoonReachesANewAssignee(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	card, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Report", f.ada.ID)
	due, _ := time.ParseInLocation(time.DateOnly, "2026-10-02", time.Local)
	card, _ = f.s.UpdateCard(ctx, f.board.ID, card.ID, card.Version, store.CardFields{Title: "Report", AssigneeID: &f.ada.ID, DueDate: &due}, 0)
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-01 09:00", time.Local)
	s := notify.Scheduler{Store: f.s, Notifier: f.n, Now: func() time.Time { return now }}
	s.Tick(ctx)
	f.s.UpdateCard(ctx, f.board.ID, card.ID, card.Version, store.CardFields{Title: "Report", AssigneeID: &f.bob.ID, DueDate: &due}, 0)
	s.Tick(ctx)
	if n, _ := f.s.UnreadCount(ctx, f.bob.ID); n != 1 {
		t.Fatalf("the new assignee got %d reminders, want 1", n)
	}
}
