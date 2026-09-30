package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/store"
)

// boardFixture is a team with a lead, a member and a board.
type boardFixture struct {
	s      *store.Store
	lead   store.User
	member store.User
	team   store.Team
	board  store.Board
	cols   []store.Column
}

func newBoardFixture(t *testing.T) boardFixture {
	t.Helper()
	s := newStore(t)
	ctx := context.Background()
	f := boardFixture{s: s}
	f.lead = mustUpsert(t, s, identity("lead", "lead@example.com"))
	f.member = mustUpsert(t, s, identity("member", "member@example.com"))
	var err error
	if f.team, err = s.CreateTeam(ctx, "Platform"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, f.team.ID, f.lead.ID, store.RoleLead); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, f.team.ID, f.member.ID, store.RoleMember); err != nil {
		t.Fatal(err)
	}
	if f.board, err = s.CreateBoard(ctx, f.team.ID, "Sprint", []string{"Todo", "Doing", "Done"}); err != nil {
		t.Fatal(err)
	}
	if f.cols, err = s.Columns(ctx, f.board.ID); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCreateBoardMakesItsColumns(t *testing.T) {
	f := newBoardFixture(t)
	if len(f.cols) != 3 {
		t.Fatalf("columns = %d, want 3", len(f.cols))
	}
	for i, c := range f.cols {
		if c.Position != i || c.BoardID != f.board.ID {
			t.Errorf("column %d = %+v", i, c)
		}
	}
	if !f.cols[0].AllowCreate || f.cols[0].IsDone || !f.cols[2].IsDone || f.cols[2].AllowCreate {
		t.Errorf("flags: first=%+v last=%+v", f.cols[0], f.cols[2])
	}
	if f.board.TransitionsMode != "open" || f.board.PersonWIPLimit != nil {
		t.Errorf("board = %+v", f.board)
	}
}

func TestBoardLookups(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	got, err := f.s.Board(ctx, f.board.ID)
	if err != nil || got.Name != "Sprint" || got.TeamID != f.team.ID {
		t.Fatalf("Board = %+v, %v", got, err)
	}
	if _, err := f.s.Board(ctx, 99999); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown board: %v", err)
	}
	boards, err := f.s.BoardsOfTeam(ctx, f.team.ID)
	if err != nil || len(boards) != 1 {
		t.Fatalf("BoardsOfTeam = %v, %v", boards, err)
	}
	if err := f.s.RenameBoard(ctx, f.board.ID, "Sprint 2"); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ArchiveBoard(ctx, f.board.ID); err != nil {
		t.Fatal(err)
	}
	if boards, _ := f.s.BoardsOfTeam(ctx, f.team.ID); len(boards) != 0 {
		t.Errorf("an archived board is listed")
	}
}

func TestColumnEditing(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	review, err := f.s.AddColumn(ctx, f.board.ID, "Review")
	if err != nil || review.Position != 3 {
		t.Fatalf("AddColumn = %+v, %v", review, err)
	}
	if err := f.s.MoveColumn(ctx, f.board.ID, review.ID, -1); err != nil {
		t.Fatal(err)
	}
	cols, _ := f.s.Columns(ctx, f.board.ID)
	names := []string{}
	for _, c := range cols {
		names = append(names, c.Name)
	}
	if got := join(names); got != "Todo,Doing,Review,Done" {
		t.Errorf("order = %s", got)
	}
	limit := 3
	upd := store.ColumnUpdate{Name: "In review", WIPLimit: &limit, IsDone: false, AllowCreate: true, CountsPersonWIP: true}
	if err := f.s.UpdateColumn(ctx, f.board.ID, review.ID, upd); err != nil {
		t.Fatal(err)
	}
	cols, _ = f.s.Columns(ctx, f.board.ID)
	if c := cols[2]; c.Name != "In review" || c.WIPLimit == nil || *c.WIPLimit != 3 || !c.AllowCreate || !c.CountsPersonWIP {
		t.Errorf("updated column = %+v", c)
	}
	// Another board's column is not this board's to change.
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"A"})
	otherCols, _ := f.s.Columns(ctx, other.ID)
	if err := f.s.UpdateColumn(ctx, f.board.ID, otherCols[0].ID, upd); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("cross-board UpdateColumn: %v", err)
	}
	if err := f.s.DeleteColumn(ctx, f.board.ID, review.ID); err != nil {
		t.Fatalf("DeleteColumn(empty): %v", err)
	}
	cols, _ = f.s.Columns(ctx, f.board.ID)
	for i, c := range cols {
		if c.Position != i {
			t.Errorf("positions not renumbered after delete: %+v", cols)
		}
	}
}

func TestAColumnWithCardsCannotBeDeleted(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if _, err := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Write docs", f.lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteColumn(ctx, f.board.ID, f.cols[0].ID); !errors.Is(err, store.ErrColumnNotEmpty) {
		t.Errorf("DeleteColumn(with cards): %v", err)
	}
}

func join(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}
