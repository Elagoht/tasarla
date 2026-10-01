package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func TestUpdateCardFieldChangesOnlyThatField(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	a, _ = f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A", Description: "keep me"}, 0)

	got, err := f.s.UpdateCardField(ctx, f.board.ID, a.ID, a.Version, store.FieldTitle, store.CardFields{Title: "A2", Description: "ignored"}, f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "A2" || got.Description != "keep me" || got.Version != a.Version+1 {
		t.Fatalf("card = %+v", got)
	}
	if _, err := f.s.UpdateCardField(ctx, f.board.ID, a.ID, a.Version, store.FieldDescription, store.CardFields{Description: "stale"}, 0); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}
	act, _ := f.s.CardActivity(ctx, a.ID, 1)
	if len(act) != 1 || join(act[0].Payload.Fields) != "title" {
		t.Errorf("activity = %+v", act)
	}
	if _, err := f.s.UpdateCardField(ctx, f.board.ID, a.ID, got.Version, store.CardField("version"), store.CardFields{}, 0); err == nil {
		t.Error("an unknown field was accepted")
	}
}

func TestUpdateCardFieldChecksPersonalWIP(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	limit := 1
	f.s.SetBoardPolicy(ctx, f.board.ID, rules.ModeOpen, &limit)
	f.s.UpdateColumn(ctx, f.board.ID, f.cols[0].ID, store.ColumnUpdate{Name: "Todo", AllowCreate: true, CountsPersonWIP: true})
	a, b := f.card(t, 0, "A"), f.card(t, 0, "B")
	if _, err := f.s.UpdateCardField(ctx, f.board.ID, a.ID, a.Version, store.FieldAssignee, store.CardFields{AssigneeID: &f.member.ID}, 0); err != nil {
		t.Fatal(err)
	}
	_, err := f.s.UpdateCardField(ctx, f.board.ID, b.ID, b.Version, store.FieldAssignee, store.CardFields{AssigneeID: &f.member.ID}, 0)
	if got := violations(t, err); len(got) != 1 || got[0] != "rules.wip_person" {
		t.Fatalf("violations = %v", got)
	}
}

func colNames(t *testing.T, f boardFixture) string {
	t.Helper()
	cols, _ := f.s.Columns(context.Background(), f.board.ID)
	var names []string
	for i, c := range cols {
		if c.Position != i {
			t.Errorf("column %q has position %d, want %d", c.Name, c.Position, i)
		}
		names = append(names, c.Name)
	}
	return join(names)
}

func TestSaveColumns(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	three := 3
	rows := []store.ColumnRow{
		{ID: f.cols[1].ID, Name: "In progress", WIPLimit: &three, CountsPersonWIP: true},
		{ID: f.cols[0].ID, Name: "Backlog", AllowCreate: true},
		{Name: "Review"},
		{ID: f.cols[2].ID, Name: "Done", IsDone: true},
	}
	if err := f.s.SaveColumns(ctx, f.board.ID, rows); err != nil {
		t.Fatal(err)
	}
	if got := colNames(t, f); got != "In progress,Backlog,Review,Done" {
		t.Fatalf("columns = %s", got)
	}
	cols, _ := f.s.Columns(ctx, f.board.ID)
	if c := cols[0]; c.WIPLimit == nil || *c.WIPLimit != 3 || !c.CountsPersonWIP || c.AllowCreate {
		t.Errorf("in progress = %+v", c)
	}
	// Delete the empty Review column.
	rows = []store.ColumnRow{{ID: cols[0].ID, Name: cols[0].Name}, {ID: cols[1].ID, Name: cols[1].Name, AllowCreate: true},
		{ID: cols[2].ID, Name: "Review", Delete: true}, {ID: cols[3].ID, Name: "Done", IsDone: true}}
	if err := f.s.SaveColumns(ctx, f.board.ID, rows); err != nil {
		t.Fatal(err)
	}
	if got := colNames(t, f); got != "In progress,Backlog,Done" {
		t.Fatalf("after delete = %s", got)
	}
}

func TestSaveColumnsIsAllOrNothing(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	f.card(t, 0, "busy")
	from := f.cols[1].ID
	f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: f.cols[2].ID, FromColumnID: &from, Subject: rules.SubjectTeamLead})
	rows := []store.ColumnRow{
		{ID: f.cols[0].ID, Name: "Todo renamed", Delete: true},
		{ID: f.cols[1].ID, Name: "Doing", Delete: true},
		{ID: f.cols[2].ID, Name: "Done renamed"},
	}
	err := f.s.SaveColumns(ctx, f.board.ID, rows)
	var ce *store.ColumnsError
	if !errors.As(err, &ce) || len(ce.Rows) != 2 || ce.Rows[0].Index != 0 || !errors.Is(ce.Rows[0].Err, store.ErrColumnNotEmpty) ||
		ce.Rows[1].Index != 1 || !errors.Is(ce.Rows[1].Err, store.ErrInUse) {
		t.Fatalf("err = %#v", err)
	}
	if got := colNames(t, f); got != "Todo,Doing,Done" {
		t.Fatalf("a refused save changed the columns: %s", got)
	}
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	oc, _ := f.s.Columns(ctx, other.ID)
	if err := f.s.SaveColumns(ctx, f.board.ID, []store.ColumnRow{{ID: oc[0].ID, Name: "mine"}}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("another board's column: %v", err)
	}
	empty := []store.ColumnRow{{ID: f.cols[2].ID, Name: "Done", Delete: true}}
	f2 := newBoardFixture(t)
	if err := f2.s.SaveColumns(ctx, f2.board.ID, []store.ColumnRow{
		{ID: f2.cols[0].ID, Delete: true, Name: "a"}, {ID: f2.cols[1].ID, Delete: true, Name: "b"}, {ID: f2.cols[2].ID, Delete: true, Name: "c"},
	}); err == nil {
		t.Fatal("deleting every column was accepted")
	}
	_ = empty
}

func TestRenameBoardRole(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	qa, _ := f.s.CreateBoardRole(ctx, f.board.ID, "QA")
	if err := f.s.RenameBoardRole(ctx, f.board.ID, qa.ID, "Quality"); err != nil {
		t.Fatal(err)
	}
	r, _ := f.s.BoardRules(ctx, f.board.ID)
	if r.Roles[0].Name != "Quality" {
		t.Fatalf("roles = %+v", r.Roles)
	}
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	if err := f.s.RenameBoardRole(ctx, other.ID, qa.ID, "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("through another board: %v", err)
	}
}

// Review: a column added to a board with "only from" sentences is reachable
// from everywhere and reaches every column not restricted, so the sentences
// stay as they were.
func TestANewColumnUnderRestrictedTransitionsIsFree(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	todo, doing, done := f.cols[0].ID, f.cols[1].ID, f.cols[2].ID
	if err := f.s.AddFromSentence(ctx, f.board.ID, done, []int64{doing}); err != nil {
		t.Fatal(err)
	}
	rows := []store.ColumnRow{{ID: todo, Name: "Todo", AllowCreate: true}, {ID: doing, Name: "Doing"},
		{Name: "Review"}, {ID: done, Name: "Done", IsDone: true}}
	if err := f.s.SaveColumns(ctx, f.board.ID, rows); err != nil {
		t.Fatal(err)
	}
	ss, err := f.s.BoardSentences(ctx, f.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	var from []store.Sentence
	for _, s := range ss {
		if s.Kind == store.SentenceFrom {
			from = append(from, s)
		}
	}
	if len(from) != 1 || from[0].ColumnID != done || len(from[0].Columns) != 1 || from[0].Columns[0] != doing {
		t.Fatalf("from sentences = %+v", from)
	}
}
