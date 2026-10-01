package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func TestARuleBreakingMoveIsRefusedWithEveryReason(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	s := b.h.store
	if err := s.AddCondition(ctx, b.board.ID, store.ColumnCondition{ColumnID: b.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasEstimate}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMovePermission(ctx, b.board.ID, store.MovePermission{ToColumnID: b.cols[1].ID, Subject: rules.SubjectTeamLead}); err != nil {
		t.Fatal(err)
	}
	res := b.member.SubmitFetch(b.path, b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	if res.Status != http.StatusUnprocessableEntity || strings.Contains(res.Body, "<html") {
		t.Fatalf("move = %d, want the columns with 422:\n%s", res.Status, res.Body)
	}
	mustContain(t, res.Body, "Doing kolonuna kart taşıma yetkiniz yok.", "Doing kolonuna girmek için kartın bir tahmini olmalı.")
	moved, _ := s.Card(ctx, b.board.ID, c.ID)
	if moved.ColumnID != b.cols[0].ID {
		t.Fatal("the refused move was applied")
	}
	b.h.speaks("member@example.com", "en")
	en := b.member.SubmitFetch("/en"+b.path, "/en"+b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	mustContain(t, en.Body, "You may not move cards into Doing.", "A card needs an estimate to enter Doing.")
	b.h.speaks("member@example.com", "tr")

	// Without a script: back to the card, with the reasons as messages.
	res = b.member.Submit(b.cardPath(c), b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("fallback = %d", res.Status)
	}
	mustContain(t, b.member.Get(res.Location()).Body, "Doing kolonuna kart taşıma yetkiniz yok.")
}

func TestCreatingIntoAFullColumnIsRefused(t *testing.T) {
	b := newBoardSetup(t)
	limit := 1
	if err := b.h.store.UpdateColumn(context.Background(), b.board.ID, b.cols[0].ID, store.ColumnUpdate{Name: "Todo", WIPLimit: &limit, AllowCreate: true}); err != nil {
		t.Fatal(err)
	}
	b.card(t, 0, "First")
	res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "title": {"Second"}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("create = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Todo kolonunun WIP limiti (1) dolu.")
}

func TestAssigningOverPersonalWIPIsRefused(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	limit := 1
	b.h.store.SetBoardPolicy(ctx, b.board.ID, rules.ModeOpen, &limit)
	b.h.store.UpdateColumn(ctx, b.board.ID, b.cols[0].ID, store.ColumnUpdate{Name: "Todo", AllowCreate: true, CountsPersonWIP: true})
	memberID := b.h.user("member@example.com").ID
	first, second := b.card(t, 0, "First"), b.card(t, 0, "Second")
	if _, err := b.h.store.UpdateCard(ctx, b.board.ID, first.ID, first.Version, store.CardFields{Title: "First", AssigneeID: &memberID}, 0); err != nil {
		t.Fatal(err)
	}
	res := b.member.SubmitFetch(b.cardPath(second), b.cardPath(second), fieldForm(second, "assignee", id(memberID)))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("assign = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Atanan kişinin WIP limiti (1) dolu.")
}
