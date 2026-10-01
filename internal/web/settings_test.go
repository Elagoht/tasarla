package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOnlyManagersSeeBoardSettings(t *testing.T) {
	b := newBoardSetup(t)
	settings := b.path + "/settings"
	if res := b.lead.Get(settings); res.Status != http.StatusOK {
		t.Fatalf("lead GET settings = %d", res.Status)
	}
	mustContain(t, b.lead.Get(b.path).Body, `href="`+settings+`"`)
	if res := b.member.Get(settings); res.Status != http.StatusNotFound {
		t.Fatalf("member GET settings = %d, want 404", res.Status)
	}
	if strings.Contains(b.member.Get(b.path).Body, `href="`+settings+`"`) {
		t.Error("a member is shown the settings link")
	}
	if res := b.member.Submit(b.path, settings, url.Values{"op": {"rename"}, "board_name": {"Mine"}}); res.Status != http.StatusNotFound {
		t.Fatalf("member POST settings = %d, want 404", res.Status)
	}
	out := b.h.signedIn("out", "out@example.com")
	if res := out.Get(settings); res.Status != http.StatusNotFound {
		t.Fatalf("outsider GET settings = %d, want 404", res.Status)
	}
}

func TestEditingColumns(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	s := b.path + "/settings"
	if res := b.lead.Submit(s, s, url.Values{"op": {"column_add"}, "new_column": {"Review"}}); res.Status != http.StatusSeeOther {
		t.Fatalf("column_add = %d:\n%s", res.Status, res.Body)
	}
	cols, _ := b.h.store.Columns(ctx, b.board.ID)
	review := cols[3]
	if review.Name != "Review" {
		t.Fatalf("columns = %+v", cols)
	}
	res := b.lead.Submit(s, s, url.Values{"op": {"column_update"}, "column_id": {id(review.ID)}, "name": {"In review"},
		"wip_limit": {"2"}, "is_done": {"1"}, "counts_person_wip": {"1"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("column_update = %d", res.Status)
	}
	b.lead.Submit(s, s, url.Values{"op": {"column_move"}, "column_id": {id(review.ID)}, "dir": {"-1"}})
	cols, _ = b.h.store.Columns(ctx, b.board.ID)
	if c := cols[2]; c.Name != "In review" || c.WIPLimit == nil || *c.WIPLimit != 2 || !c.IsDone || !c.CountsPersonWIP || c.AllowCreate {
		t.Fatalf("updated column = %+v", c)
	}
	res = b.lead.Submit(s, s, url.Values{"op": {"column_update"}, "column_id": {id(review.ID)}, "name": {"x"}, "wip_limit": {"zero"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("bad limit = %d", res.Status)
	}
	mustContain(t, b.lead.Get(s).Body, "WIP limiti pozitif bir tam sayı olmalı.")

	b.card(t, 0, "Busy")
	b.lead.Submit(s, s, url.Values{"op": {"column_delete"}, "column_id": {id(b.cols[0].ID)}})
	mustContain(t, b.lead.Get(s).Body, "Kartı olan bir kolon silinemez.")
	if res := b.lead.Submit(s, s, url.Values{"op": {"column_delete"}, "column_id": {id(review.ID)}}); res.Status != http.StatusSeeOther {
		t.Fatalf("delete = %d", res.Status)
	}
	if cols, _ := b.h.store.Columns(ctx, b.board.ID); len(cols) != 3 {
		t.Fatalf("columns after delete = %d", len(cols))
	}
}

func TestEditingLabelsAndTheBoard(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	s := b.path + "/settings"
	if res := b.lead.Submit(s, s, url.Values{"op": {"label_add"}, "label_name": {"bug"}, "label_color": {"#e03131"}}); res.Status != http.StatusSeeOther {
		t.Fatalf("label_add = %d:\n%s", res.Status, res.Body)
	}
	res := b.lead.Submit(s, s, url.Values{"op": {"label_add"}, "label_name": {"x"}, "label_color": {"#123456"}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("a colour outside the palette = %d, want 422", res.Status)
	}
	labels, _ := b.h.store.Labels(ctx, b.board.ID)
	if len(labels) != 1 {
		t.Fatalf("labels = %+v", labels)
	}
	b.lead.Submit(s, s, url.Values{"op": {"label_delete"}, "label_id": {id(labels[0].ID)}})
	if labels, _ := b.h.store.Labels(ctx, b.board.ID); len(labels) != 0 {
		t.Fatal("label not deleted")
	}
	b.lead.Submit(s, s, url.Values{"op": {"rename"}, "board_name": {"Sprint 42"}})
	mustContain(t, b.member.Get(b.path).Body, "Sprint 42")
	res = b.lead.Submit(s, s, url.Values{"op": {"archive_board"}})
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/teams/") {
		t.Fatalf("archive = %d %q", res.Status, res.Location())
	}
	if res := b.member.Get(b.path); res.Status != http.StatusNotFound {
		t.Fatalf("archived board = %d, want 404", res.Status)
	}
}
