package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func move(t *testing.T, f boardFixture, c store.Card, to int64) (store.Card, error) {
	t.Helper()
	cur, _ := f.s.Card(context.Background(), f.board.ID, c.ID)
	return f.s.MoveCard(context.Background(), store.Move{BoardID: f.board.ID, CardID: c.ID, ToColumnID: to, ToIndex: 0,
		ExpectedFrom: cur.ColumnID, ExpectedVersion: cur.Version, Actor: rules.Actor{UserID: f.lead.ID, TeamRole: "lead"}})
}

// A card entering a done column is completed and leaves the board; moved out
// again it is reopened.
func TestEnteringADoneColumnCompletesTheCard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	done := f.cols[len(f.cols)-1]
	if !done.IsDone {
		t.Fatalf("the fixture's last column is not done: %+v", done)
	}
	c, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Ship it", f.lead.ID)
	blocked, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "After", f.lead.ID)
	if err := f.s.AddDependency(ctx, f.board.ID, c.ID, blocked.ID); err != nil {
		t.Fatal(err)
	}
	moved, err := move(t, f, c, done.ID)
	if err != nil || moved.CompletedAt == nil || moved.CompletedFrom == nil || *moved.CompletedFrom != f.cols[0].ID {
		t.Fatalf("moved = %+v, %v", moved, err)
	}
	cards, _ := f.s.BoardCards(ctx, f.board.ID)
	if len(cards) != 1 || cards[0].Card.ID != blocked.ID || cards[0].Blocked {
		t.Fatalf("board after completing = %+v", cards)
	}
	list, _ := f.s.DoneCards(ctx, f.board.ID, "")
	if len(list) != 1 || list[0].Card.ID != c.ID || list[0].CompletedBy != f.lead.Name {
		t.Fatalf("done = %+v", list)
	}
	deps, _ := f.s.CardDependencies(ctx, blocked.ID)
	if len(deps.Blockers) != 1 || !deps.Blockers[0].Done {
		t.Fatalf("blockers = %+v", deps.Blockers)
	}
	// Reopening goes through the rules of the column it enters.
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasEstimate}); err != nil {
		t.Fatal(err)
	}
	var re *store.RuleError
	if _, err := move(t, f, c, f.cols[1].ID); !errors.As(err, &re) {
		t.Fatalf("reopen against a condition = %v", err)
	}
	reopened, err := move(t, f, c, f.cols[0].ID)
	if err != nil || reopened.CompletedAt != nil {
		t.Fatalf("reopened = %+v, %v", reopened, err)
	}
	if cards, _ := f.s.BoardCards(ctx, f.board.ID); len(cards) != 2 {
		t.Fatalf("board after reopening = %d cards", len(cards))
	}
}

// Completed cards do not count against a WIP limit.
func TestCompletedCardsLeaveTheWIPCount(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	done := f.cols[len(f.cols)-1]
	one := 1
	if err := f.s.SetWIPLimit(ctx, f.board.ID, done.ID, &one); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"A", "B", "C"} {
		c, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, title, f.lead.ID)
		if _, err := move(t, f, c, done.ID); err != nil {
			t.Fatalf("completing %s: %v", title, err)
		}
	}
}
