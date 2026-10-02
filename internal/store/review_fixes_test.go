package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"kanban/internal/rules"
	"kanban/internal/store"
)

// holdBoard takes the board's row lock in a transaction of its own, as a
// concurrent move would, and returns a function that releases it.
func holdBoard(t *testing.T, f boardFixture) func() {
	t.Helper()
	ctx := context.Background()
	tx, err := f.s.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM boards WHERE id = $1 FOR UPDATE`, f.board.ID); err != nil {
		t.Fatal(err)
	}
	return func() { tx.Rollback(ctx) }
}

// Every write that feeds a rule takes the board's lock first, so a move cannot
// read it half-way (spec §5.3).
func TestRuleInputsWaitForTheBoardLock(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	bug, _ := f.s.CreateLabel(ctx, f.board.ID, "bug", "#e03131")
	att, _ := f.s.AddAttachment(ctx, f.board.ID, a.ID, store.Attachment{UploaderID: f.lead.ID, Filename: "x", ContentType: "text/plain", StorageKey: "0123456789abcdef0123456789abcdef"})
	writes := map[string]func() error{
		"checklist": func() error { _, err := f.s.AddChecklistItem(ctx, f.board.ID, a.ID, "x"); return err },
		"labels":    func() error { return f.s.SetCardLabels(ctx, f.board.ID, a.ID, []int64{bug.ID}) },
		"attach": func() error {
			_, err := f.s.AddAttachment(ctx, f.board.ID, a.ID, store.Attachment{UploaderID: f.lead.ID, Filename: "y", ContentType: "text/plain", StorageKey: "fedcba9876543210fedcba9876543210"})
			return err
		},
		"detach": func() error {
			_, err := f.s.DeleteAttachment(ctx, f.board.ID, a.ID, att.ID, f.lead.ID, true)
			return err
		},
	}
	for name, write := range writes {
		release := holdBoard(t, f)
		done := make(chan error, 1)
		go func() { done <- write() }()
		select {
		case err := <-done:
			release()
			t.Errorf("%s finished while another transaction held the board (err %v)", name, err)
			continue
		case <-time.After(300 * time.Millisecond):
		}
		release()
		if err := <-done; err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDeletingARoleThatPermissionsNameIsRefused(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	qa, _ := f.s.CreateBoardRole(ctx, f.board.ID, "QA")
	if err := f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: f.cols[2].ID, Subject: rules.SubjectBoardRole, BoardRoleID: &qa.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteBoardRole(ctx, f.board.ID, qa.ID); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("delete = %v, want ErrInUse", err)
	}
	from := f.cols[1].ID
	if err := f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: f.cols[2].ID, FromColumnID: &from, Subject: rules.SubjectTeamLead}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteColumn(ctx, f.board.ID, from); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("deleting a column a permission starts from = %v, want ErrInUse", err)
	}
}

func TestLabelConditionsStayHonest(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	bug, _ := f.s.CreateLabel(ctx, f.board.ID, "bug", "#e03131")
	ui, _ := f.s.CreateLabel(ctx, f.board.ID, "ui", "#1971c2")
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	foreign, _ := f.s.CreateLabel(ctx, other.ID, "foreign", "#000000")
	cond := store.ColumnCondition{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasLabel}
	cond.Params.LabelIDs = []int64{bug.ID, foreign.ID}
	if err := f.s.AddCondition(ctx, f.board.ID, cond); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a foreign label in a condition = %v", err)
	}
	cond.Params.LabelIDs = []int64{bug.ID, ui.ID}
	if err := f.s.AddCondition(ctx, f.board.ID, cond); err != nil {
		t.Fatal(err)
	}
	f.s.DeleteLabel(ctx, f.board.ID, bug.ID)
	r, _ := f.s.BoardRules(ctx, f.board.ID)
	if len(r.Conditions) != 1 || len(r.Conditions[0].Params.LabelIDs) != 1 || r.Conditions[0].Params.LabelIDs[0] != ui.ID {
		t.Fatalf("after deleting one label: %+v", r.Conditions)
	}
	f.s.DeleteLabel(ctx, f.board.ID, ui.ID)
	if r, _ := f.s.BoardRules(ctx, f.board.ID); len(r.Conditions) != 0 {
		t.Fatalf("a condition naming only deleted labels survived: %+v", r.Conditions)
	}
}

func TestDeleteColumnChecksTheBoardFirst(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	oc, _ := f.s.Columns(ctx, other.ID)
	f.s.CreateCard(ctx, other.ID, oc[0].ID, "busy", f.lead.ID)
	if err := f.s.DeleteColumn(ctx, f.board.ID, oc[0].ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("another board's busy column = %v, want ErrNotFound", err)
	}
}

func TestOutboxRecordsEachSendAsItGoes(t *testing.T) {
	f := newBoardFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < 3; i++ {
		f.s.CreateNotification(context.Background(), store.NewNotification{UserID: f.member.ID, Kind: store.NotifyAssigned,
			Email: &store.OutboxMessage{To: "member@example.com", Subject: "s", HTML: "h", Text: "t"}})
	}
	calls := 0
	// The database's clock may run a little ahead of ours: due "now" means a moment after the insert.
	_, _, err := f.s.ProcessOutbox(ctx, time.Now().Add(time.Second), 20, func(store.OutboxMessage) error {
		calls++
		if calls == 2 {
			cancel() // shutdown begins while the second e-mail is going out
		}
		return nil
	}, nil)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if n, _ := f.s.OutboxCount(context.Background(), "sent"); n != calls {
		t.Fatalf("%d sent, %d recorded as sent: a restart would send them again", calls, n)
	}
	if calls != 2 {
		t.Errorf("sends after cancellation: %d", calls)
	}
}

func TestOutboxReportsWhatItGivesUpOn(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	f.s.CreateNotification(ctx, store.NewNotification{UserID: f.member.ID, Kind: store.NotifyAssigned,
		Email: &store.OutboxMessage{To: "member@example.com", Subject: "s", HTML: "h", Text: "t"}})
	var gaveUp []string
	// The database's clock may run a little ahead of ours: due "now" means a moment after the insert.
	now := time.Now().Add(time.Second)
	for i := 0; i < 8; i++ {
		f.s.ProcessOutbox(ctx, now, 20, func(store.OutboxMessage) error { return errors.New("down") },
			func(m store.OutboxMessage, err error) { gaveUp = append(gaveUp, m.To+": "+err.Error()) })
		now = now.Add(7 * time.Hour)
	}
	if len(gaveUp) != 1 || gaveUp[0] != "member@example.com: down" {
		t.Fatalf("gave up = %v", gaveUp)
	}
}

func TestCanSeeBoard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if ok, err := f.s.CanSeeBoard(ctx, f.member.ID, f.board.ID); !ok || err != nil {
		t.Fatalf("member: %v %v", ok, err)
	}
	f.s.RemoveMember(ctx, f.team.ID, f.member.ID)
	if ok, _ := f.s.CanSeeBoard(ctx, f.member.ID, f.board.ID); ok {
		t.Fatal("a removed member still sees the board")
	}
	admin := mustUpsert(t, f.s, identity("admin", "admin@example.com"), "admin@example.com")
	if ok, _ := f.s.CanSeeBoard(ctx, admin.ID, f.board.ID); !ok {
		t.Fatal("an admin does not see the board")
	}
}
