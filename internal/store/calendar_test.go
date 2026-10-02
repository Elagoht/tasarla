package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"kanban/internal/store"
)

func TestCalendarTokenLifecycle(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if has, err := f.s.HasCalendarToken(ctx, f.member.ID); err != nil || has {
		t.Fatalf("a new user has a token: %v, %v", has, err)
	}
	first, err := f.s.NewCalendarToken(ctx, f.member.ID)
	if err != nil || len(first) != 43 {
		t.Fatalf("token %q, %v", first, err)
	}
	if u, err := f.s.UserByCalendarToken(ctx, first); err != nil || u.ID != f.member.ID {
		t.Fatalf("lookup = %+v, %v", u, err)
	}
	second, err := f.s.NewCalendarToken(ctx, f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if has, err := f.s.HasCalendarToken(ctx, f.member.ID); err != nil || !has {
		t.Fatalf("has = %v, %v after a token", has, err)
	}
	if _, err := f.s.UserByCalendarToken(ctx, first); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a reset left the old token working: %v", err)
	}
	if err := f.s.ClearCalendarToken(ctx, f.member.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.UserByCalendarToken(ctx, second); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a cleared token works: %v", err)
	}
	if _, err := f.s.UserByCalendarToken(ctx, "not-a-token"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("garbage = %v", err)
	}
}

func TestCalendarCards(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	due := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	dated := func(title string, assignee *int64) store.Card {
		c := f.card(t, 0, title)
		c, err := f.s.UpdateCardField(ctx, f.board.ID, c.ID, c.Version, store.FieldDueDate, store.CardFields{DueDate: &due}, f.lead.ID)
		if err != nil {
			t.Fatal(err)
		}
		if assignee != nil {
			if c, err = f.s.UpdateCardField(ctx, f.board.ID, c.ID, c.Version, store.FieldAssignee, store.CardFields{AssigneeID: assignee}, f.lead.ID); err != nil {
				t.Fatal(err)
			}
		}
		return c
	}
	mine := dated("Mine", &f.member.ID)
	dated("Lead's", &f.lead.ID)
	f.card(t, 0, "Undated")
	archived := dated("Archived", &f.member.ID)
	if err := f.s.ArchiveCard(ctx, f.board.ID, archived.ID, f.lead.ID); err != nil {
		t.Fatal(err)
	}
	done := dated("Done", &f.member.ID)
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: done.ID, ToColumnID: f.cols[2].ID,
		ExpectedFrom: done.ColumnID, ExpectedVersion: done.Version, Actor: lead(f)}); err != nil {
		t.Fatal(err)
	}

	personal, err := f.s.CalendarCards(ctx, f.member.ID, 0)
	if err != nil || len(personal) != 1 || personal[0].Card.ID != mine.ID || personal[0].BoardName != "Sprint" || personal[0].Modified.IsZero() {
		t.Fatalf("personal = %+v, %v", personal, err)
	}
	board, err := f.s.CalendarCards(ctx, f.member.ID, f.board.ID)
	if err != nil || len(board) != 2 {
		t.Fatalf("board feed = %d cards, %v; want Mine and Lead's", len(board), err)
	}

	if err := f.s.RemoveMember(ctx, f.team.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.CalendarCards(ctx, f.member.ID, f.board.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a removed member reads the board feed: %v", err)
	}
	if personal, _ := f.s.CalendarCards(ctx, f.member.ID, 0); len(personal) != 0 {
		t.Errorf("a removed member's feed keeps the board's cards: %d", len(personal))
	}
}
