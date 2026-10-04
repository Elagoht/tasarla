package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"kanban/internal/rules"
	"kanban/internal/store"
)

// A card made with a due date enters a column that requires one; the same
// card without the date is refused.
func TestCreateCardDueMeetsTheEntryRule(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	var re *store.RuleError
	if _, err := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Undated", f.lead.ID); !errors.As(err, &re) {
		t.Fatalf("undated card = %v, want a rule error", err)
	}
	due := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	c, err := f.s.CreateCardDue(ctx, f.board.ID, f.cols[0].ID, "Dated", due, f.lead.ID)
	if err != nil {
		t.Fatalf("dated card: %v", err)
	}
	got, err := f.s.Card(ctx, f.board.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Dated" || got.DueDate == nil || got.DueDate.Format(time.DateOnly) != "2026-10-14" || got.StartDate != nil {
		t.Fatalf("card = %+v", got)
	}
}

func TestCreateCardDueInAnotherBoardsColumn(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	other, err := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"A"})
	if err != nil {
		t.Fatal(err)
	}
	cols, err := f.s.Columns(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.CreateCardDue(ctx, f.board.ID, cols[0].ID, "X", time.Now(), f.lead.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign column = %v, want ErrNotFound", err)
	}
}
