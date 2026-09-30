package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"kanban/internal/store"
)

func (f boardFixture) card(t *testing.T, col int, title string) store.Card {
	t.Helper()
	c, err := f.s.CreateCard(context.Background(), f.board.ID, f.cols[col].ID, title, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// order returns the titles of a column's cards, top to bottom.
func (f boardFixture) order(t *testing.T, col int) string {
	t.Helper()
	view, err := f.s.BoardCards(context.Background(), f.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for i, c := range view {
		if c.Card.ColumnID == f.cols[col].ID {
			if c.Card.Position != len(titles) {
				t.Errorf("card %d %q has position %d, want %d", i, c.Card.Title, c.Card.Position, len(titles))
			}
			titles = append(titles, c.Card.Title)
		}
	}
	return join(titles)
}

func TestCreateCardAppendsToTheColumn(t *testing.T) {
	f := newBoardFixture(t)
	a := f.card(t, 0, "A")
	b := f.card(t, 0, "B")
	if a.Position != 0 || b.Position != 1 || a.Version != 1 {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
	other, _ := f.s.CreateBoard(context.Background(), f.team.ID, "Other", []string{"X"})
	if _, err := f.s.CreateCard(context.Background(), other.ID, f.cols[0].ID, "C", f.lead.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a card in another board's column: %v", err)
	}
}

func TestMoveCard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a, b, c := f.card(t, 0, "A"), f.card(t, 0, "B"), f.card(t, 0, "C")
	f.card(t, 1, "X")

	// Reorder in place: C to the top.
	moved, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: c.ID, ToColumnID: f.cols[0].ID, ToIndex: 0, ExpectedFrom: f.cols[0].ID, ExpectedVersion: c.Version})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Version != c.Version+1 {
		t.Errorf("version = %d, want %d", moved.Version, c.Version+1)
	}
	if got := f.order(t, 0); got != "C,A,B" {
		t.Errorf("column 0 = %s", got)
	}

	// Across columns, into the middle.
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[1].ID, ToIndex: 1, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version}); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, 0); got != "C,B" {
		t.Errorf("column 0 = %s", got)
	}
	if got := f.order(t, 1); got != "X,A" {
		t.Errorf("column 1 = %s", got)
	}

	// An index past the end lands at the end; a negative one at the top.
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: b.ID, ToColumnID: f.cols[1].ID, ToIndex: 99, ExpectedFrom: f.cols[0].ID, ExpectedVersion: b.Version}); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, 1); got != "X,A,B" {
		t.Errorf("column 1 = %s", got)
	}
}

func TestMovingAStaleCardIsAConflict(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[1].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version}); err != nil {
		t.Fatal(err)
	}
	// Someone saw the card in column 0 at version 1.
	_, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[2].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale column: %v", err)
	}
	_, err = f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[2].ID, ExpectedFrom: f.cols[1].ID, ExpectedVersion: a.Version})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}
}

func TestMoveStaysInsideTheBoard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	otherCols, _ := f.s.Columns(ctx, other.ID)
	_, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: otherCols[0].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("into another board's column: %v", err)
	}
	_, err = f.s.MoveCard(ctx, store.Move{BoardID: other.ID, CardID: a.ID, ToColumnID: otherCols[0].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a card of another board: %v", err)
	}
}

func TestUpdateCard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	est := 3.5
	due := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	prio := int16(2)
	fields := store.CardFields{Title: "A2", Description: "body", AssigneeID: &f.member.ID, Estimate: &est, DueDate: &due, Priority: &prio}
	got, err := f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, fields)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "A2" || got.Description != "body" || *got.AssigneeID != f.member.ID || *got.Estimate != 3.5 ||
		!got.DueDate.Equal(due) || *got.Priority != 2 || got.Version != a.Version+1 {
		t.Errorf("updated = %+v", got)
	}
	if _, err := f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, fields); !errors.Is(err, store.ErrConflict) {
		t.Errorf("stale update: %v", err)
	}
	if _, err := f.s.UpdateCard(ctx, f.board.ID, 99999, 1, fields); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown card: %v", err)
	}
}

func TestArchiveCardClosesTheGap(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	f.card(t, 0, "A")
	b := f.card(t, 0, "B")
	f.card(t, 0, "C")
	if err := f.s.ArchiveCard(ctx, f.board.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, 0); got != "A,C" {
		t.Errorf("column 0 = %s", got)
	}
	archived, _ := f.s.Card(ctx, f.board.ID, b.ID)
	if archived.ArchivedAt == nil || archived.Version != b.Version+1 {
		t.Errorf("archived = %+v", archived)
	}
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: b.ID, ToColumnID: f.cols[1].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: archived.Version}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("moving an archived card: %v", err)
	}
}

func TestBoardCardsCarriesWhatTheBoardShows(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	a, _ = f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A", AssigneeID: &f.member.ID})
	view, err := f.s.BoardCards(ctx, f.board.ID)
	if err != nil || len(view) != 1 {
		t.Fatalf("BoardCards = %v, %v", view, err)
	}
	if view[0].AssigneeName != "User member" {
		t.Errorf("assignee name = %q", view[0].AssigneeName)
	}
}
