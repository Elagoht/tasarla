package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestEditingACard(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	memberID := id(b.h.user("member@example.com").ID)
	steps := []struct{ field, value string }{
		{"title", "Renamed"}, {"description", "Details"}, {"assignee", memberID},
		{"estimate", "2,5"}, {"due_date", "2026-10-20"}, {"priority", "3"},
	}
	for _, s := range steps {
		cur, _ := b.h.store.Card(ctx, b.board.ID, c.ID)
		res := b.member.Submit(b.cardPath(c), b.cardPath(c), fieldForm(cur, s.field, s.value))
		if res.Status != http.StatusSeeOther || res.Location() != b.cardPath(c) {
			t.Fatalf("set %s = %d %q:\n%s", s.field, res.Status, res.Location(), res.Body)
		}
	}
	got, _ := b.h.store.Card(ctx, b.board.ID, c.ID)
	if got.Title != "Renamed" || got.Description != "Details" || got.AssigneeID == nil || *got.Estimate != 2.5 ||
		got.DueDate.Format("2006-01-02") != "2026-10-20" || *got.Priority != 3 {
		t.Fatalf("card = %+v", got)
	}
	page := b.member.Get(b.cardPath(c)).Body
	mustContain(t, page, "Kaydedildi.", "Renamed", `value="2.5"`)
	mustContain(t, b.member.Get(b.path).Body, "Member", "2026-10-20", "Yüksek")
	mustContain(t, b.member.Get("/me/tasks").Body, "Renamed", "Sprint")
}

func TestASavedFieldIsMarkedAndARefusedOneExplained(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "priority", "2"))
	mustContain(t, res.Body, `data-saved-for="priority" data-just-saved`)
	res = b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "estimate", "-1"))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid estimate = %d", res.Status)
	}
	// Beside the field, not above the panel.
	i := strings.Index(res.Body, `value="estimate"`)
	j := strings.Index(res.Body, "Sıfır ya da pozitif bir sayı girin.")
	if i < 0 || j < i || strings.Contains(res.Body[:i], `role="alert"`) {
		t.Fatalf("the message is not beside the estimate:\n%s", res.Body)
	}
}

func TestCardPartsThroughThePanel(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	blocker := b.card(t, 0, "Blocker")
	bug, _ := b.h.store.CreateLabel(ctx, b.board.ID, "bug", "#e03131")

	res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), url.Values{"op": {"labels"}, "label": {id(bug.ID)}})
	if res.Status != http.StatusOK || strings.Contains(res.Body, "<html") {
		t.Fatalf("labels = %d:\n%s", res.Status, res.Body)
	}
	res = b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), url.Values{"op": {"checklist_add"}, "item_text": {"Write tests"}})
	mustContain(t, res.Body, "Write tests")
	items, _ := b.h.store.ChecklistItems(ctx, c.ID)
	res = b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), url.Values{"op": {"checklist_toggle"}, "item_id": {id(items[0].ID)}, "done": {"1"}})
	mustContain(t, res.Body, "checklist__item--done")
	res = b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), url.Values{"op": {"dep_add"}, "blocker_id": {id(blocker.ID)}})
	mustContain(t, res.Body, "Blocker")
	mustContain(t, b.member.Get(b.path).Body, "card--blocked", `data-color="#e03131"`, "1/1")

	res = b.member.SubmitFetch(b.cardPath(blocker), b.cardPath(blocker), url.Values{"op": {"dep_add"}, "blocker_id": {id(c.ID)}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("cycle = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Bu bağımlılık bir döngü oluşturur.")

	res = b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), url.Values{"op": {"checklist_delete"}, "item_id": {id(items[0].ID)}})
	// The activity log still names the item; the checklist must not.
	if strings.Contains(res.Body, `class="checklist__item`) {
		t.Error("the deleted item is still in the checklist")
	}
	res = b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), url.Values{"op": {"dep_remove"}, "blocker_id": {id(blocker.ID)}})
	if res.Status != http.StatusOK {
		t.Fatalf("dep_remove = %d", res.Status)
	}
}

func TestArchivingACard(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Old card")
	res := b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"confirm": {"1"}, "op": {"archive"}})
	if res.Status != http.StatusSeeOther || res.Location() != b.path {
		t.Fatalf("archive = %d %q", res.Status, res.Location())
	}
	board := b.member.Get(b.path).Body
	mustContain(t, board, "Kart arşivlendi.")
	if strings.Contains(board, "Old card") {
		t.Error("an archived card is on the board")
	}
	page := b.member.Get(b.cardPath(c)).Body
	mustContain(t, page, "Bu kart arşivde.")
	if strings.Contains(page, `value="set_field"`) {
		t.Error("an archived card can still be edited")
	}
}

func TestACardOfAnotherBoardIsNotFound(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	other, _ := b.h.store.CreateBoard(ctx, b.team.ID, "Other", []string{"X"})
	cols, _ := b.h.store.Columns(ctx, other.ID)
	foreign, _ := b.h.store.CreateCard(ctx, other.ID, cols[0].ID, "Foreign", b.h.user("lead@example.com").ID)
	// The card exists, but not on this board.
	if res := b.member.Get(b.path + "/cards/" + id(foreign.ID)); res.Status != http.StatusNotFound {
		t.Fatalf("cross-board card = %d, want 404", res.Status)
	}
}

func TestPanelFragment(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	res := b.member.Get(b.cardPath(c) + "/panel")
	if res.Status != http.StatusOK || strings.Contains(res.Body, "<html") || !strings.Contains(res.Body, "Card") {
		t.Fatalf("panel = %d:\n%s", res.Status, res.Body)
	}
}

func TestMyTasksIsEmptyAtFirst(t *testing.T) {
	h := newHarness(t, "")
	b := h.signedIn("ada", "ada@example.com")
	mustContain(t, b.Get("/me/tasks").Body, "Size atanmış kart yok.")
	mustContain(t, b.Get("/en/me/tasks").Body, "No cards are assigned to you.")
}
