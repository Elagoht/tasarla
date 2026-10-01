package web_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"kanban/internal/store"
)

func laneForm(c store.Card, to, from int64, lane, value, before string) url.Values {
	f := moveForm(c, to, 0, from, c.Version)
	f.Set("lane", lane)
	f.Set("lane_value", value)
	f.Set("before_card_id", before)
	return f
}

func TestDropOnALaneAssignsAndNotifies(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Lane card")
	member := b.h.user("member@example.com")
	page := b.path + "?lane=assignee"
	r := b.lead.SubmitFetch(page, page, laneForm(c, b.cols[1].ID, b.cols[0].ID, "assignee", id(member.ID), ""))
	if r.Status != http.StatusOK {
		t.Fatalf("lane drop = %d:\n%s", r.Status, r.Body)
	}
	now, err := b.h.store.Card(context.Background(), b.board.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if now.AssigneeID == nil || *now.AssigneeID != member.ID || now.ColumnID != b.cols[1].ID {
		t.Fatalf("card = %+v", now)
	}
	mustContain(t, b.member.Get("/notifications").Body, "Lane card")
}

func TestDropOnTheUnassignedLaneClears(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Given card")
	member := b.h.user("member@example.com").ID
	c, err := b.h.store.UpdateCardField(context.Background(), b.board.ID, c.ID, c.Version, store.FieldAssignee,
		store.CardFields{AssigneeID: &member}, b.h.user("lead@example.com").ID)
	if err != nil {
		t.Fatal(err)
	}
	page := b.path + "?lane=assignee"
	if r := b.lead.SubmitFetch(page, page, laneForm(c, b.cols[0].ID, b.cols[0].ID, "assignee", "none", "")); r.Status != http.StatusOK {
		t.Fatalf("drop on unassigned = %d", r.Status)
	}
	now, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID)
	if now.AssigneeID != nil {
		t.Errorf("still assigned: %+v", now)
	}
}

func TestLaneDropRefusalsAndBadValues(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	b.h.signedIn("out", "out@example.com")
	page := b.path + "?lane=assignee"
	r := b.lead.SubmitFetch(page, page, laneForm(c, b.cols[0].ID, b.cols[0].ID, "assignee", id(b.h.user("out@example.com").ID), ""))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("outsider = %d, want 422", r.Status)
	}
	mustContain(t, r.Body, "takımın üyesi olmayan")
	for _, bad := range [][2]string{{"label", "1"}, {"priority", "9"}, {"assignee", "abc"}, {"priority", "x"}} {
		if r := b.lead.SubmitFetch(page, page, laneForm(c, b.cols[0].ID, b.cols[0].ID, bad[0], bad[1], "")); r.Status != http.StatusBadRequest {
			t.Errorf("lane %s=%s: %d, want 400", bad[0], bad[1], r.Status)
		}
	}
}
