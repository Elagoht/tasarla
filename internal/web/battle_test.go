package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Battle test: text the database cannot hold is refused, never a 500.
func TestTextWithNULOrBadUTF8IsRefused(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	for _, title := range []string{"a\x00b", "a\xffb", "\xc0\xaf"} {
		if res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "title": {title}}); res.Status != http.StatusBadRequest {
			t.Errorf("create_card %q = %d, want 400", title, res.Status)
		}
		if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "description", title)); res.Status != http.StatusBadRequest {
			t.Errorf("set_field %q = %d, want 400", title, res.Status)
		}
		if res := b.lead.Submit(b.path+"/settings", b.path+"/settings", url.Values{"op": {"label_add"}, "label_name": {title}, "label_color": {"#e03131"}}); res.Status != http.StatusBadRequest {
			t.Errorf("label_add %q = %d, want 400", title, res.Status)
		}
	}
}

// Battle test: an estimate is a plain non-negative number.
func TestAnEstimateIsAPlainNumber(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	for _, v := range []string{"NaN", "Inf", "infinity", "0x1p4", "1_0", "1e3", "-1", "100001"} {
		if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "estimate", v)); res.Status != http.StatusUnprocessableEntity {
			t.Errorf("estimate %q = %d, want 422", v, res.Status)
		}
	}
	for _, v := range []string{"0", "2,5", "13.25"} {
		cur, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID)
		if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(cur, "estimate", v)); res.Status != http.StatusOK {
			t.Errorf("estimate %q = %d, want 200", v, res.Status)
		}
	}
}

// Battle test: limits stay far inside the database's integers.
func TestHugeLimitsAreRefused(t *testing.T) {
	b := newBoardSetup(t)
	s := b.path + "/settings"
	rows := []map[string]string{{"id": id(b.cols[0].ID), "name": "Todo", "wip": "2147483648"},
		{"id": id(b.cols[1].ID), "name": "Doing"}, {"id": id(b.cols[2].ID), "name": "Done"}}
	if res := b.lead.Submit(s+"?tab=columns", s, columnsForm(rows, 0, 2)); res.Status != http.StatusUnprocessableEntity {
		t.Errorf("huge WIP = %d, want 422", res.Status)
	}
	for _, f := range []url.Values{
		{"op": {"rule_add"}, "sentence": {"wip"}, "column": {id(b.cols[0].ID)}, "limit": {"2147483648"}},
		{"op": {"rule_add"}, "sentence": {"person_wip"}, "limit": {"2147483648"}, "counted": {id(b.cols[0].ID)}},
		{"op": {"rule_add"}, "sentence": {"condition"}, "column": {id(b.cols[0].ID)}, "phase": {"enter"}, "kind": {"min_attachments"}, "count": {"2147483648"}},
	} {
		if res := b.lead.Submit(s+"?tab=rules", s, f); res.Status != http.StatusSeeOther {
			t.Errorf("%s = %d, want 303 with a message", f.Get("sentence"), res.Status)
		}
	}
	ss, _ := b.h.store.BoardSentences(context.Background(), b.board.ID)
	if len(ss) != 0 {
		t.Fatalf("a huge limit was saved: %+v", ss)
	}
}

// Battle test: odd input to the column table and the rules is refused.
func TestOddColumnTablesAndRulesAreRefused(t *testing.T) {
	b := newBoardSetup(t)
	s := b.path + "/settings"
	dup := []map[string]string{{"id": id(b.cols[0].ID), "name": "A"}, {"id": id(b.cols[0].ID), "name": "B"},
		{"id": id(b.cols[2].ID), "name": "Done"}}
	if res := b.lead.Submit(s+"?tab=columns", s, columnsForm(dup, 0, 2)); res.Status != http.StatusBadRequest {
		t.Errorf("a column twice = %d, want 400", res.Status)
	}
	none := []map[string]string{{"id": id(b.cols[0].ID), "name": "Todo", "delete": "1"},
		{"id": id(b.cols[1].ID), "name": "Doing", "delete": "1"}, {"id": id(b.cols[2].ID), "name": "Done", "delete": "1"}}
	res := b.lead.Submit(s+"?tab=columns", s, columnsForm(none, -1, -1))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("no column left = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "En az bir kolon kalmalı.")
	b.lead.Submit(s+"?tab=rules", s, url.Values{"op": {"rule_add"}, "sentence": {"permission"}, "column": {id(b.cols[1].ID)},
		"from_column": {id(b.cols[1].ID)}, "subject": {"team_lead"}})
	if ss, _ := b.h.store.BoardSentences(context.Background(), b.board.ID); len(ss) != 0 {
		t.Fatalf("a permission from a column into itself was saved: %+v", ss)
	}
}

// Battle test: a board has one address.
func TestABoardHasOneAddress(t *testing.T) {
	b := newBoardSetup(t)
	for _, p := range []string{"/boards/+" + id(b.board.ID), "/boards/0" + id(b.board.ID)} {
		if res := b.member.Get(p); res.Status != http.StatusNotFound || strings.Contains(res.Body, "Sprint") {
			t.Errorf("GET %s = %d, want 404", p, res.Status)
		}
	}
}
