package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"kanban/internal/store"
)

func fieldForm(c store.Card, field, value string) url.Values {
	return url.Values{"op": {"set_field"}, "field": {field}, "value": {value}, "expected_version": {strconv.Itoa(c.Version)}}
}

func TestSetFieldSavesOneField(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	c, _ = b.h.store.UpdateCard(ctx, b.board.ID, c.ID, c.Version, store.CardFields{Title: "Card", Description: "keep"}, 0)
	memberID := id(b.h.user("member@example.com").ID)
	steps := []struct{ field, value string }{
		{"title", "Renamed"}, {"assignee", memberID}, {"due_date", "2026-10-20"}, {"priority", "2"}, {"estimate", "1,5"},
	}
	for _, s := range steps {
		cur, _ := b.h.store.Card(ctx, b.board.ID, c.ID)
		res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(cur, s.field, s.value))
		if res.Status != http.StatusOK || strings.Contains(res.Body, "<html") {
			t.Fatalf("set %s = %d:\n%s", s.field, res.Status, res.Body)
		}
	}
	got, _ := b.h.store.Card(ctx, b.board.ID, c.ID)
	if got.Title != "Renamed" || got.Description != "keep" || got.AssigneeID == nil || *got.Priority != 2 || *got.Estimate != 1.5 ||
		got.DueDate.Format("2006-01-02") != "2026-10-20" {
		t.Fatalf("card = %+v", got)
	}
	// Clearing a field.
	res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(got, "assignee", ""))
	if res.Status != http.StatusOK {
		t.Fatalf("clear assignee = %d", res.Status)
	}
	if got, _ := b.h.store.Card(ctx, b.board.ID, c.ID); got.AssigneeID != nil {
		t.Fatal("assignee not cleared")
	}
}

// Review Focus 1: a stale save of one field does not overwrite another.
func TestAStaleFieldSaveIsRefusedNotMerged(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	b.lead.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "description", "lead's text"))
	res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "title", "member's title"))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("stale save = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Bu kart başka biri tarafından değiştirildi.")
	got, _ := b.h.store.Card(ctx, b.board.ID, c.ID)
	if got.Title != "Card" || got.Description != "lead's text" {
		t.Fatalf("card = %+v", got)
	}
}

func TestSetFieldValidates(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	out := b.h.signedIn("out", "out@example.com")
	_ = out
	cases := map[string]string{
		"title": "", "due_date": "20.10.2026", "estimate": "-2", "priority": "9",
		"assignee": id(b.h.user("out@example.com").ID), "version": "3",
	}
	for field, value := range cases {
		res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, field, value))
		if res.Status != http.StatusUnprocessableEntity && res.Status != http.StatusBadRequest {
			t.Errorf("%s=%q = %d, want 422 or 400", field, value, res.Status)
		}
	}
	if got, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID); got.Version != c.Version {
		t.Fatal("an invalid save changed the card")
	}
	// Without a script: back to the card with the message.
	res := b.member.Submit(b.cardPath(c), b.cardPath(c), fieldForm(c, "title", "Plain"))
	if res.Status != http.StatusSeeOther || res.Location() != b.cardPath(c) {
		t.Fatalf("plain post = %d %q", res.Status, res.Location())
	}
}

func TestMovingFromThePanel(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	form := url.Values{"op": {"move_to"}, "to_column": {id(b.cols[1].ID)}, "expected_from": {id(b.cols[0].ID)}, "expected_version": {strconv.Itoa(c.Version)}}
	res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), form)
	if res.Status != http.StatusOK || strings.Contains(res.Body, "<html") || !strings.Contains(res.Body, "Card") {
		t.Fatalf("move_to = %d:\n%s", res.Status, res.Body)
	}
	if got, _ := b.h.store.Card(ctx, b.board.ID, c.ID); got.ColumnID != b.cols[1].ID {
		t.Fatal("not moved")
	}
	// Stale: refused with the reason, card stays.
	res = b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), form)
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("stale move_to = %d", res.Status)
	}
	mustContain(t, res.Body, "Bu kart başka biri tarafından değiştirildi.")
}
