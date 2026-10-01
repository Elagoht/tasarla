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

// columnsForm is the column table as the settings page posts it.
func columnsForm(rows []map[string]string, create, done int) url.Values {
	f := url.Values{"op": {"columns_save"}, "col_count": {strconv.Itoa(len(rows))}}
	for i, r := range rows {
		p := "col_" + strconv.Itoa(i) + "_"
		f.Set(p+"order", strconv.Itoa(i))
		if i == create {
			f.Set(p+"create", "1")
		}
		if i == done {
			f.Set(p+"done", "1")
		}
		for k, v := range r {
			f.Set(p+k, v)
		}
	}
	return f
}

func TestSavingTheColumnTable(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	s := b.path + "/settings?tab=columns"
	rows := []map[string]string{
		{"id": id(b.cols[1].ID), "name": "In progress", "wip": "2", "person": "1"},
		{"id": id(b.cols[0].ID), "name": "Backlog"},
		{"id": "", "name": "Review"},
		{"id": id(b.cols[2].ID), "name": "Done"},
	}
	res := b.lead.Submit(s, b.path+"/settings", columnsForm(rows, 1, 3))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("columns_save = %d:\n%s", res.Status, res.Body)
	}
	cols, _ := b.h.store.Columns(ctx, b.board.ID)
	var names []string
	for _, c := range cols {
		names = append(names, c.Name)
	}
	if strings.Join(names, ",") != "In progress,Backlog,Review,Done" || *cols[0].WIPLimit != 2 || !cols[0].CountsPersonWIP ||
		!cols[1].AllowCreate || cols[0].AllowCreate || !cols[3].IsDone {
		t.Fatalf("columns = %+v", cols)
	}
}

// Review Focus 2: one bad row refuses the whole table and keeps what was typed.
// A table with no done column chosen marks none, rather than the first.
func TestNoDoneColumnTickedMarksNone(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	rows := []map[string]string{
		{"id": id(b.cols[0].ID), "name": "Todo"},
		{"id": id(b.cols[1].ID), "name": "Doing"},
		{"id": id(b.cols[2].ID), "name": "Done"},
	}
	form := columnsForm(rows, 0, -1)
	if res := b.lead.Submit(b.path+"/settings?tab=columns", b.path+"/settings", form); res.Status != http.StatusSeeOther {
		t.Fatalf("columns_save = %d", res.Status)
	}
	cols, _ := b.h.store.Columns(ctx, b.board.ID)
	for _, c := range cols {
		if c.IsDone {
			t.Fatalf("%s is marked done", c.Name)
		}
	}
	page := b.lead.Get(b.path + "/settings?tab=columns").Body
	if strings.Contains(page, `name="col_0_done" value="1" checked`) {
		t.Error("the table shows the first column as done")
	}
}

// Review: a board may have more than one done column, and keeps them.
func TestTwoDoneColumnsStayDone(t *testing.T) {
	b := newBoardSetup(t)
	rows := []map[string]string{
		{"id": id(b.cols[0].ID), "name": "Todo"},
		{"id": id(b.cols[1].ID), "name": "Cancelled"},
		{"id": id(b.cols[2].ID), "name": "Done"},
	}
	form := columnsForm(rows, 0, 2)
	form.Set("col_1_done", "1")
	if res := b.lead.Submit(b.path+"/settings?tab=columns", b.path+"/settings", form); res.Status != http.StatusSeeOther {
		t.Fatalf("columns_save = %d", res.Status)
	}
	cols, _ := b.h.store.Columns(context.Background(), b.board.ID)
	if cols[0].IsDone || !cols[1].IsDone || !cols[2].IsDone || !cols[0].AllowCreate {
		t.Fatalf("columns = %+v", cols)
	}
	page := b.lead.Get(b.path + "/settings?tab=columns").Body
	mustContain(t, page, `name="col_1_done" value="1" checked`, `name="col_2_done" value="1" checked`)
}

func TestABadColumnTableSavesNothing(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	b.card(t, 0, "busy")
	rows := []map[string]string{
		{"id": id(b.cols[0].ID), "name": "Typed name", "delete": "1"},
		{"id": id(b.cols[1].ID), "name": "Doing", "wip": "zero"},
		{"id": id(b.cols[2].ID), "name": ""},
	}
	res := b.lead.Submit(b.path+"/settings?tab=columns", b.path+"/settings", columnsForm(rows, 0, 2))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("bad table = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, `value="Typed name"`, `value="zero"`, "Kartı olan bir kolon silinemez.", "WIP limiti pozitif bir tam sayı olmalı.", "Ad boş olamaz.")
	cols, _ := b.h.store.Columns(ctx, b.board.ID)
	if cols[0].Name != "Todo" || len(cols) != 3 {
		t.Fatalf("columns changed: %+v", cols)
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
	b.lead.Submit(s, s, url.Values{"confirm": {"1"}, "op": {"label_delete"}, "label_id": {id(labels[0].ID)}})
	if labels, _ := b.h.store.Labels(ctx, b.board.ID); len(labels) != 0 {
		t.Fatal("label not deleted")
	}
	b.lead.Submit(s, s, url.Values{"op": {"rename"}, "board_name": {"Sprint 42"}})
	mustContain(t, b.member.Get(b.path).Body, "Sprint 42")
	res = b.lead.Submit(s, s, url.Values{"confirm": {"1"}, "op": {"archive_board"}})
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/teams/") {
		t.Fatalf("archive = %d %q", res.Status, res.Location())
	}
	if res := b.member.Get(b.path); res.Status != http.StatusNotFound {
		t.Fatalf("archived board = %d, want 404", res.Status)
	}
}

func TestEditingRoles(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	s := b.path + "/settings"
	b.lead.Submit(s, s, url.Values{"op": {"role_add"}, "role_name": {"QA"}})
	r, _ := b.h.store.BoardRules(ctx, b.board.ID)
	role := id(r.Roles[0].ID)
	if res := b.lead.Submit(s, s, url.Values{"op": {"role_rename"}, "role_id": {role}, "role_name": {"Quality"}}); res.Status != http.StatusSeeOther {
		t.Fatalf("role_rename = %d", res.Status)
	}
	b.lead.Submit(s, s, url.Values{"op": {"role_members"}, "role_id": {role}, "member": {id(b.h.user("member@example.com").ID)}})
	r, _ = b.h.store.BoardRules(ctx, b.board.ID)
	if r.Roles[0].Name != "Quality" || len(r.Roles[0].MemberIDs) != 1 {
		t.Fatalf("roles = %+v", r.Roles)
	}
	mustContain(t, b.lead.Get(s+"?tab=roles").Body, "Quality")
}

func TestRuleSentences(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	s := b.path + "/settings"
	todo, doing, done := id(b.cols[0].ID), id(b.cols[1].ID), id(b.cols[2].ID)
	qa, _ := b.h.store.CreateBoardRole(ctx, b.board.ID, "QA")
	adds := []url.Values{
		{"sentence": {"permission"}, "column": {done}, "subject": {"team_lead"}, "role": {id(qa.ID)}},
		{"sentence": {"from"}, "column": {done}, "from": {doing}},
		{"sentence": {"condition"}, "column": {doing}, "phase": {"enter"}, "kind": {"min_attachments"}, "count": {"2"}},
		{"sentence": {"wip"}, "column": {doing}, "limit": {"3"}},
		{"sentence": {"person_wip"}, "limit": {"2"}, "counted": {doing}},
	}
	for _, form := range adds {
		form.Set("op", "rule_add")
		if res := b.lead.Submit(s, s, form); res.Status != http.StatusSeeOther {
			t.Fatalf("rule_add %s = %d:\n%s", form.Get("sentence"), res.Status, res.Body)
		}
	}
	ss, _ := b.h.store.BoardSentences(ctx, b.board.ID)
	if len(ss) != 5 {
		t.Fatalf("sentences = %+v", ss)
	}
	page := b.lead.Get(s + "?tab=rules").Body
	mustContain(t, page, "Done kolonuna yalnızca", "QA", "en az 2 dosya eki", "en fazla 3 kart")
	_ = todo

	// Nonsense never reaches the store.
	for _, form := range []url.Values{
		{"op": {"rule_add"}, "sentence": {"condition"}, "column": {doing}, "phase": {"enter"}, "kind": {"tarot"}},
		{"op": {"rule_add"}, "sentence": {"permission"}, "column": {done}},
		{"op": {"rule_add"}, "sentence": {"wip"}, "column": {doing}, "limit": {"-1"}},
		{"op": {"rule_add"}, "sentence": {"poetry"}},
	} {
		res := b.lead.Submit(s, s, form)
		if res.Status == http.StatusInternalServerError {
			t.Errorf("%v = 500", form)
		}
	}
	if ss2, _ := b.h.store.BoardSentences(ctx, b.board.ID); len(ss2) != 5 {
		t.Fatalf("bad input changed the rules: %d", len(ss2))
	}
	for _, sen := range ss {
		res := b.lead.Submit(s, s, url.Values{"confirm": {"1"}, "op": {"rule_delete"}, "key": {sen.Key}})
		if res.Status != http.StatusSeeOther {
			t.Fatalf("rule_delete %s = %d", sen.Key, res.Status)
		}
	}
	if ss, _ := b.h.store.BoardSentences(ctx, b.board.ID); len(ss) != 0 {
		t.Fatalf("left over: %+v", ss)
	}
	if bd, _ := b.h.store.Board(ctx, b.board.ID); bd.TransitionsMode != rules.ModeOpen {
		t.Fatal("still restricted")
	}
	_ = store.SentenceWIP
}
