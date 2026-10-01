package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
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
	en := b.member.SubmitFetch("/en"+b.path, "/en"+b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	mustContain(t, en.Body, "You may not move cards into Doing.", "A card needs an estimate to enter Doing.")

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
	res := b.member.SubmitFetch(b.cardPath(second), b.cardPath(second), updateForm(second, map[string]string{"assignee_id": id(memberID)}))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("assign = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Atanan kişinin WIP limiti (1) dolu.")
}

func TestLeadConfiguresRules(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	s := b.path + "/settings"
	todo, doing, done := id(b.cols[0].ID), id(b.cols[1].ID), id(b.cols[2].ID)
	steps := []url.Values{
		{"op": {"policy"}, "transitions_mode": {"restricted"}, "person_wip_limit": {"3"}},
		{"op": {"transitions"}, "t": {todo + "-" + doing, doing + "-" + done}},
		{"op": {"role_add"}, "role_name": {"QA"}},
	}
	for _, form := range steps {
		if res := b.lead.Submit(s, s, form); res.Status != http.StatusSeeOther {
			t.Fatalf("%s = %d:\n%s", form.Get("op"), res.Status, res.Body)
		}
	}
	r, _ := b.h.store.BoardRules(ctx, b.board.ID)
	role := id(r.Roles[0].ID)
	more := []url.Values{
		{"op": {"role_members"}, "role_id": {role}, "member": {id(b.h.user("member@example.com").ID)}},
		{"op": {"permission_add"}, "to_column": {done}, "from_column": {""}, "subject": {"board_role"}, "board_role": {role}},
		{"op": {"condition_add"}, "column_id": {doing}, "phase": {"enter"}, "kind": {"min_attachments"}, "count": {"2"}},
		{"op": {"condition_add"}, "column_id": {doing}, "phase": {"exit"}, "kind": {"has_label"}},
	}
	for _, form := range more {
		if res := b.lead.Submit(s, s, form); res.Status != http.StatusSeeOther {
			t.Fatalf("%s = %d:\n%s", form.Get("op"), res.Status, res.Body)
		}
	}
	r, _ = b.h.store.BoardRules(ctx, b.board.ID)
	if len(r.Transitions) != 2 || len(r.Roles[0].MemberIDs) != 1 || len(r.Permissions) != 1 || len(r.Conditions) != 2 || r.Conditions[0].Params.Count != 2 {
		t.Fatalf("rules = %+v", r)
	}
	board, _ := b.h.store.Board(ctx, b.board.ID)
	if board.TransitionsMode != "restricted" || board.PersonWIPLimit == nil || *board.PersonWIPLimit != 3 {
		t.Fatalf("board = %+v", board)
	}
	page := b.lead.Get(s).Body
	mustContain(t, page, "QA", "En az 2 dosya eki")

	// Bad input does not reach the store.
	for _, form := range []url.Values{
		{"op": {"condition_add"}, "column_id": {doing}, "phase": {"enter"}, "kind": {"tarot"}},
		{"op": {"permission_add"}, "to_column": {done}, "subject": {"board_role"}},
		{"op": {"policy"}, "transitions_mode": {"chaos"}},
	} {
		res := b.lead.Submit(s, s, form)
		if res.Status != http.StatusBadRequest && res.Status != http.StatusSeeOther {
			t.Errorf("%v = %d", form, res.Status)
		}
	}
	if r2, _ := b.h.store.BoardRules(ctx, b.board.ID); len(r2.Conditions) != 2 || len(r2.Permissions) != 1 {
		t.Fatalf("bad input changed the rules: %+v", r2)
	}

	for _, form := range []url.Values{
		{"op": {"permission_delete"}, "permission_id": {id(r.Permissions[0].ID)}},
		{"op": {"condition_delete"}, "condition_id": {id(r.Conditions[0].ID)}},
		{"confirm": {"1"}, "op": {"role_delete"}, "role_id": {role}},
	} {
		if res := b.lead.Submit(s, s, form); res.Status != http.StatusSeeOther {
			t.Fatalf("%s = %d", form.Get("op"), res.Status)
		}
	}
	r, _ = b.h.store.BoardRules(ctx, b.board.ID)
	if len(r.Roles) != 0 || len(r.Permissions) != 0 || len(r.Conditions) != 1 {
		t.Fatalf("after deletes = %+v", r)
	}
	_ = strconv.Itoa
}
