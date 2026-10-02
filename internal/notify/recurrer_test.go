package notify_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"kanban/internal/notify"
	"kanban/internal/rules"
	"kanban/internal/store"
)

var istanbul = mustLocation("Europe/Istanbul")

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, istanbul)
	if err != nil {
		panic(err)
	}
	return t
}

// scheduled makes a template of the board that runs on Mondays at 09:00 in
// the first column, edited last by actor, its schedule on since since.
func (f *fixture) scheduled(t *testing.T, actor int64, dueInDays *int, since time.Time) store.Template {
	t.Helper()
	ctx := context.Background()
	col := f.cols[0].ID
	tpl, err := f.s.CreateTemplate(ctx, f.board.ID, store.TemplateInput{
		Name: "Haftalık", Title: "Haftalık rapor", ColumnID: &col, DueInDays: dueInDays,
		Schedule: store.Schedule{Kind: "weekly", Weekdays: 1, Hour: 9},
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE card_templates SET schedule_since = $2 WHERE id = $1`, tpl.ID, since); err != nil {
		t.Fatal(err)
	}
	return tpl
}

func (f *fixture) recurrer(now time.Time) notify.Recurrer {
	return notify.Recurrer{Store: f.s, Notifier: f.n, Location: istanbul, Now: func() time.Time { return now }}
}

func (f *fixture) counts(t *testing.T, templateID int64) (cards, runs int) {
	t.Helper()
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM cards WHERE board_id = $1`, f.board.ID).Scan(&cards); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM template_runs WHERE template_id = $1`, templateID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	return cards, runs
}

func TestRecurrerMakesOneCardPerMoment(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	days := 2
	tpl := f.scheduled(t, f.ada.ID, &days, at("2026-09-01 00:00"))

	// Two recurrers on the same moment, at once: one card.
	r := f.recurrer(at("2026-10-05 10:00"))
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- r.Tick(ctx) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 1 || runs != 1 {
		t.Fatalf("concurrent ticks: %d cards, %d runs; want 1, 1", cards, runs)
	}
	// And again, one after the other: still one.
	for range 2 {
		if err := r.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 1 || runs != 1 {
		t.Fatalf("repeated ticks: %d cards, %d runs; want 1, 1", cards, runs)
	}
	got, _ := f.s.Template(ctx, f.board.ID, tpl.ID)
	if got.LastRun == nil || got.LastRun.Status != "created" || got.LastRun.CardID == nil || !got.LastRun.ScheduledFor.Equal(at("2026-10-05 09:00")) {
		t.Fatalf("last run = %+v", got.LastRun)
	}
	card, err := f.s.Card(ctx, f.board.ID, *got.LastRun.CardID)
	if err != nil || card.ColumnID != f.cols[0].ID || card.Title != "Haftalık rapor" || card.CreatedBy != f.ada.ID ||
		card.DueDate == nil || card.DueDate.Format(time.DateOnly) != "2026-10-07" {
		t.Fatalf("card = %+v, %v", card, err)
	}

	// Three weeks on, the two missed Mondays are not made up for: one card more.
	if err := f.recurrer(at("2026-10-26 10:00")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 2 || runs != 2 {
		t.Fatalf("after three weeks: %d cards, %d runs; want 2, 2", cards, runs)
	}
	got, _ = f.s.Template(ctx, f.board.ID, tpl.ID)
	if !got.LastRun.ScheduledFor.Equal(at("2026-10-26 09:00")) {
		t.Errorf("latest run is for %v", got.LastRun.ScheduledFor)
	}
	if n, _ := f.s.UnreadCount(ctx, f.ada.ID); n != 0 {
		t.Errorf("a card made is not a notification: %d", n)
	}
}

// A refused card is recorded as a failed run, in the same transaction that
// the rules refused, and the template's last editor hears of it.
func TestRecurrerRecordsARefusal(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	tpl := f.scheduled(t, f.ada.ID, nil, at("2026-09-01 00:00"))
	if err := f.recurrer(at("2026-10-05 10:00")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 0 || runs != 1 {
		t.Fatalf("%d cards, %d runs; want 0, 1", cards, runs)
	}
	got, _ := f.s.Template(ctx, f.board.ID, tpl.ID)
	if run := got.LastRun; run == nil || run.Status != "failed" || run.CardID != nil ||
		len(run.Violations) != 1 || run.Violations[0].Code != "rules.condition.has_due_date" {
		t.Fatalf("last run = %+v", got.LastRun)
	}
	ns, err := f.s.Notifications(ctx, f.ada.ID, 10)
	if err != nil || len(ns) != 1 || ns[0].Kind != store.NotifyTemplateFailed || ns[0].CardID != nil ||
		ns[0].Payload.CardTitle != "Haftalık" || ns[0].Payload.BoardName != "Sprint" {
		t.Fatalf("notifications = %+v, %v", ns, err)
	}
	mails := f.outbox(t)
	if len(mails) != 1 || mails[0].To != "ada@example.com" || !strings.Contains(mails[0].Subject, "Sprint board'unda Haftalık şablonu kart açamadı.") {
		t.Fatalf("mails = %+v", mails)
	}
	for _, body := range []string{mails[0].Text, mails[0].HTML} {
		if strings.Contains(body, "/cards/") || strings.Contains(body, `href=""`) {
			t.Errorf("a mail with no card links to one:\n%s", body)
		}
	}
	// The same moment again tells nobody twice.
	if err := f.recurrer(at("2026-10-05 10:30")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if n, _ := f.s.UnreadCount(ctx, f.ada.ID); n != 1 {
		t.Errorf("unread = %d, want 1", n)
	}
}

// When the template's last editor has left the team, its leads hear of it.
func TestRecurrerTellsTheLeadsWhenTheOwnerLeft(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	days := 1
	tpl := f.scheduled(t, f.bob.ID, &days, at("2026-09-01 00:00"))
	if err := f.s.RemoveMember(ctx, f.board.TeamID, f.bob.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.recurrer(at("2026-10-05 10:00")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 0 || runs != 1 {
		t.Fatalf("%d cards, %d runs; want 0, 1", cards, runs)
	}
	got, _ := f.s.Template(ctx, f.board.ID, tpl.ID)
	if run := got.LastRun; run == nil || run.Status != "failed" ||
		len(run.Violations) != 1 || run.Violations[0].Code != store.ViolationOwnerGone {
		t.Fatalf("last run = %+v", got.LastRun)
	}
	ns, _ := f.s.Notifications(ctx, f.ada.ID, 10)
	if len(ns) != 1 || ns[0].Kind != store.NotifyTemplateFailed {
		t.Fatalf("lead's notifications = %+v", ns)
	}
	if n, _ := f.s.UnreadCount(ctx, f.bob.ID); n != 0 {
		t.Errorf("the former member was notified: %d", n)
	}
}

func TestRecurrerSkipsArchivedBoards(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	days := 1
	tpl := f.scheduled(t, f.ada.ID, &days, at("2026-09-01 00:00"))
	if err := f.s.ArchiveBoard(ctx, f.board.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.recurrer(at("2026-10-05 10:00")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if _, runs := f.counts(t, tpl.ID); runs != 0 {
		t.Fatalf("runs = %d on an archived board", runs)
	}
}

// A schedule set after its latest moment waits for the next one.
func TestRecurrerWaitsForANewSchedulesFirstMoment(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	days := 1
	tpl := f.scheduled(t, f.ada.ID, &days, at("2026-10-05 09:30"))
	if err := f.recurrer(at("2026-10-05 10:00")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 0 || runs != 0 {
		t.Fatalf("%d cards, %d runs before the first moment; want 0, 0", cards, runs)
	}
	if err := f.recurrer(at("2026-10-12 10:00")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 1 || runs != 1 {
		t.Fatalf("%d cards, %d runs at the first moment; want 1, 1", cards, runs)
	}
}

// A template whose column was deleted records why and tells its editor.
func TestRecurrerRecordsAMissingColumn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	days := 1
	tpl := f.scheduled(t, f.ada.ID, &days, at("2026-09-01 00:00"))
	if err := f.s.DeleteColumn(ctx, f.board.ID, f.cols[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.recurrer(at("2026-10-05 10:00")).Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if cards, runs := f.counts(t, tpl.ID); cards != 0 || runs != 1 {
		t.Fatalf("%d cards, %d runs; want 0, 1 (no card in another column)", cards, runs)
	}
	got, _ := f.s.Template(ctx, f.board.ID, tpl.ID)
	if run := got.LastRun; run == nil || run.Status != "failed" ||
		len(run.Violations) != 1 || run.Violations[0].Code != store.ViolationNoColumn {
		t.Fatalf("last run = %+v", got.LastRun)
	}
	ns, _ := f.s.Notifications(ctx, f.ada.ID, 10)
	if len(ns) != 1 || ns[0].Kind != store.NotifyTemplateFailed {
		t.Fatalf("editor's notifications = %+v", ns)
	}
}

// RunTemplate acts on the template as it is in its transaction, not as the
// caller read it: the editor it reports is the current one, and a schedule
// turned off meanwhile records nothing.
func TestRunTemplateReadsTheTemplateAgain(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tpl := f.scheduled(t, f.ada.ID, nil, at("2026-09-01 00:00"))
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	stale, err := f.s.ScheduledTemplates(ctx)
	if err != nil || len(stale) != 1 || stale[0].Template.UpdatedBy != f.ada.ID {
		t.Fatalf("scheduled = %+v, %v", stale, err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE card_templates SET updated_by = $2 WHERE id = $1`, tpl.ID, f.bob.ID); err != nil {
		t.Fatal(err)
	}
	out, err := f.s.RunTemplate(ctx, stale[0], at("2026-10-05 09:00"), istanbul)
	if err != nil || !out.Ran || out.Card != nil || out.Owner != f.bob.ID {
		t.Fatalf("run = %+v, %v; want a refusal owned by bob", out, err)
	}

	if _, err := f.pool.Exec(ctx, `UPDATE card_templates SET schedule_kind = '', schedule_since = NULL WHERE id = $1`, tpl.ID); err != nil {
		t.Fatal(err)
	}
	out, err = f.s.RunTemplate(ctx, stale[0], at("2026-10-12 09:00"), istanbul)
	if err != nil || out.Ran {
		t.Fatalf("run after the schedule was turned off = %+v, %v", out, err)
	}
	if _, runs := f.counts(t, tpl.ID); runs != 1 {
		t.Errorf("runs = %d, want 1 (nothing recorded once off)", runs)
	}
}
