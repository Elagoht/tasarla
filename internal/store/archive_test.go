package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func TestRestoringACard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Old 50% done", f.lead.ID)
	b, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Other", f.lead.ID)
	if err := f.s.ArchiveCard(ctx, f.board.ID, a.ID, f.lead.ID); err != nil {
		t.Fatal(err)
	}
	got, err := f.s.ArchivedCards(ctx, f.board.ID, "")
	if err != nil || len(got) != 1 || got[0].Card.ID != a.ID || got[0].ColumnName != f.cols[0].Name || got[0].ArchivedBy != f.lead.Name {
		t.Fatalf("archive = %+v, %v", got, err)
	}
	// The search matches % as itself.
	if got, _ := f.s.ArchivedCards(ctx, f.board.ID, "50%"); len(got) != 1 {
		t.Fatalf("search 50%% = %d", len(got))
	}
	if got, _ := f.s.ArchivedCards(ctx, f.board.ID, "5_"); len(got) != 0 {
		t.Fatalf("search 5_ matched by wildcard")
	}
	// Coming back is entering the column: its conditions apply.
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasEstimate}); err != nil {
		t.Fatal(err)
	}
	var re *store.RuleError
	if err := f.s.RestoreCard(ctx, f.board.ID, a.ID, f.lead.ID); !errors.As(err, &re) {
		t.Fatalf("restore against a condition = %v", err)
	}
	ss, _ := f.s.BoardSentences(ctx, f.board.ID)
	for _, s := range ss {
		_ = f.s.DeleteSentence(ctx, f.board.ID, s.Key)
	}
	if err := f.s.RestoreCard(ctx, f.board.ID, a.ID, f.lead.ID); err != nil {
		t.Fatal(err)
	}
	cards, _ := f.s.BoardCards(ctx, f.board.ID)
	if len(cards) != 2 || cards[0].Card.ID != b.ID || cards[1].Card.ID != a.ID {
		t.Fatalf("after restore: %+v", cards)
	}
	if err := f.s.RestoreCard(ctx, f.board.ID, a.ID, f.lead.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("restoring a card not archived = %v", err)
	}
}

func TestRestoringABoard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if err := f.s.ArchiveBoard(ctx, f.board.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.s.ArchivedBoards(ctx, f.team.ID); len(got) != 1 {
		t.Fatalf("archived boards = %d", len(got))
	}
	if err := f.s.RestoreBoard(ctx, f.team.ID+1000, f.board.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("restore through another team = %v", err)
	}
	if err := f.s.RestoreBoard(ctx, f.team.ID, f.board.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.s.BoardsOfTeam(ctx, f.team.ID); len(got) != 1 {
		t.Fatalf("boards after restore = %d", len(got))
	}
}
