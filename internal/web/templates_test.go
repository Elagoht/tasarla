package web_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"kanban/internal/store"
)

// templateForm is a complete template form as the settings page posts it:
// a weekly schedule on Monday and Wednesday at 09:30, into Doing.
func (b boardSetup) templateForm(t *testing.T, name string) url.Values {
	t.Helper()
	labels, err := b.h.store.Labels(context.Background(), b.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	f := url.Values{
		"op": {"template_save"}, "template_id": {""},
		"template_name": {name}, "template_title": {"Weekly report"}, "template_description": {"Write it up."},
		"template_priority": {"3"}, "template_assignee": {id(b.h.user("member@example.com").ID)},
		"template_column": {id(b.cols[1].ID)}, "template_estimate": {"2,5"}, "due_in_days": {"2"},
		"checklist":     {"Gather numbers\r\n\r\n  Send mail  \n"},
		"schedule_kind": {"weekly"}, "weekday": {"0", "2"}, "monthday": {"1"}, "schedule_time": {"09:30"},
	}
	for _, l := range labels {
		f.Add("template_label", id(l.ID))
	}
	return f
}

func (b boardSetup) templatesPath() string { return b.path + "/settings?tab=templates" }

func (b boardSetup) newTemplatePath() string { return b.templatesPath() + "&edit=new" }

func TestLeadSavesATemplate(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	label, err := b.h.store.CreateLabel(ctx, b.board.ID, "report", "#1971c2")
	if err != nil {
		t.Fatal(err)
	}
	page := b.lead.Get(b.templatesPath())
	if page.Status != http.StatusOK {
		t.Fatalf("templates tab = %d", page.Status)
	}
	mustContain(t, page.Body, "Şablonlar", "Yeni şablon", `href="`+b.newTemplatePath()+`"`)
	mustContain(t, b.lead.Get(b.newTemplatePath()).Body, `name="template_name"`, `name="weekday" value="6"`, "Paz")

	res := b.lead.Submit(b.newTemplatePath(), b.newTemplatePath(), b.templateForm(t, "Weekly"))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("template_save = %d:\n%s", res.Status, res.Body)
	}
	ts, err := b.h.store.Templates(ctx, b.board.ID)
	if err != nil || len(ts) != 1 {
		t.Fatalf("templates = %+v, %v", ts, err)
	}
	tpl := ts[0]
	if tpl.Name != "Weekly" || tpl.Title != "Weekly report" || tpl.Description != "Write it up." ||
		tpl.Priority == nil || *tpl.Priority != 3 || tpl.AssigneeID == nil || *tpl.AssigneeID != b.h.user("member@example.com").ID ||
		tpl.ColumnID == nil || *tpl.ColumnID != b.cols[1].ID || tpl.Estimate == nil || *tpl.Estimate != 2.5 || tpl.DueInDays == nil || *tpl.DueInDays != 2 ||
		len(tpl.LabelIDs) != 1 || tpl.LabelIDs[0] != label.ID ||
		strings.Join(tpl.Checklist, "|") != "Gather numbers|Send mail" {
		t.Fatalf("template = %+v", tpl)
	}
	want := store.Schedule{Kind: "weekly", Weekdays: 1<<0 | 1<<2, MonthDay: 1, Hour: 9, Minute: 30}
	if tpl.Schedule != want {
		t.Fatalf("schedule = %+v, want %+v", tpl.Schedule, want)
	}
	mustContain(t, b.lead.Get(b.templatesPath()).Body, "Weekly")

	// The edit form shows what was saved; saving it unchanged keeps the schedule running.
	edit := b.templatesPath() + "&edit=" + id(tpl.ID)
	mustContain(t, b.lead.Get(edit).Body, `value="Weekly"`, `name="template_estimate" value="2.5"`, `name="weekday" value="2" checked`, `value="`+id(label.ID)+`" checked`, "Send mail")
	form := b.templateForm(t, "Weekly")
	form.Set("template_id", id(tpl.ID))
	form.Set("template_estimate", "2.5") // as the edit form shows it
	form.Set("monthday", "17")           // hidden for a weekly schedule: not part of it
	if res := b.lead.Submit(edit, edit, form); res.Status != http.StatusSeeOther {
		t.Fatalf("template update = %d:\n%s", res.Status, res.Body)
	}
	again, _ := b.h.store.Template(ctx, b.board.ID, tpl.ID)
	if again.Estimate == nil || *again.Estimate != 2.5 {
		t.Fatalf("estimate after an edit = %v", again.Estimate)
	}
	if again.Schedule != want || again.ScheduleSince == nil || !again.ScheduleSince.Equal(*tpl.ScheduleSince) {
		t.Fatalf("an unchanged schedule moved: %+v since %v, was %v", again.Schedule, again.ScheduleSince, tpl.ScheduleSince)
	}
}

func TestMembersCannotSaveTemplates(t *testing.T) {
	b := newBoardSetup(t)
	// Every settings operation tells a non-manager the board does not exist.
	res := b.member.Submit(b.path, b.path+"/settings?tab=templates&edit=new", b.templateForm(t, "Weekly"))
	if res.Status != http.StatusNotFound {
		t.Fatalf("member template_save = %d, want 404", res.Status)
	}
	if ts, _ := b.h.store.Templates(context.Background(), b.board.ID); len(ts) != 0 {
		t.Fatalf("a member saved %+v", ts)
	}
}

func TestBadTemplatesAreShownAgain(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	if res := b.lead.Submit(b.newTemplatePath(), b.newTemplatePath(), b.templateForm(t, "Taken")); res.Status != http.StatusSeeOther {
		t.Fatalf("first template = %d", res.Status)
	}
	cases := []struct {
		name  string
		set   map[string]string
		wants []string
	}{
		{"empty name", map[string]string{"template_name": " "}, []string{"Bu alan zorunludur."}},
		{"name too long", map[string]string{"template_name": strings.Repeat("ş", 61)}, []string{"En fazla 60 karakter olabilir."}},
		{"same name", map[string]string{"template_name": "Taken"}, []string{"Bu adla bir şablon zaten var."}},
		{"due too far", map[string]string{"due_in_days": "400"}, []string{"0 ile 365 arasında"}},
		{"weekly without a day", map[string]string{"weekday": ""}, []string{"En az bir gün seçin."}},
		{"no such hour", map[string]string{"schedule_time": "25:00"}, []string{"Saati SS:DD biçiminde girin."}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			form := b.templateForm(t, "Fresh")
			for k, v := range c.set {
				if v == "" {
					form.Del(k)
				} else {
					form.Set(k, v)
				}
			}
			res := b.lead.Submit(b.newTemplatePath(), b.newTemplatePath(), form)
			if res.Status != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422:\n%s", res.Status, res.Body)
			}
			// The form comes back with what was typed and the field's message.
			mustContain(t, res.Body, `name="template_title" value="Weekly report"`, "Send mail", `class="field__error"`)
			mustContain(t, res.Body, c.wants...)
			if _, ok := c.set["weekday"]; !ok {
				mustContain(t, res.Body, `name="weekday" value="2" checked`)
			}
		})
	}
	if ts, _ := b.h.store.Templates(ctx, b.board.ID); len(ts) != 1 {
		t.Fatalf("templates = %d, want only the first", len(ts))
	}
}

func TestDeletingATemplateAsksFirst(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	tpl, err := b.h.store.CreateTemplate(ctx, b.board.ID, store.TemplateInput{Name: "Old", Title: "Old card"}, b.h.user("lead@example.com").ID)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"op": {"template_delete"}, "template_id": {id(tpl.ID)}}
	res := b.lead.Submit(b.templatesPath(), b.templatesPath(), form)
	if res.Status != http.StatusOK {
		t.Fatalf("delete without confirm = %d, want the confirmation page", res.Status)
	}
	mustContain(t, res.Body, "Şablon silinsin mi?", `name="confirm" value="1"`)
	if _, err := b.h.store.Template(ctx, b.board.ID, tpl.ID); err != nil {
		t.Fatalf("deleted before confirmation: %v", err)
	}
	form.Set("confirm", "1")
	if res := b.lead.Submit(b.templatesPath(), b.templatesPath(), form); res.Status != http.StatusSeeOther {
		t.Fatalf("confirmed delete = %d", res.Status)
	}
	if _, err := b.h.store.Template(ctx, b.board.ID, tpl.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("template after delete: %v", err)
	}
}

func TestATemplateWhoseColumnWasDeletedIsFlagged(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	doing := b.cols[1].ID
	if _, err := b.h.store.CreateTemplate(ctx, b.board.ID, store.TemplateInput{Name: "Orphan", Title: "Card", ColumnID: &doing},
		b.h.user("lead@example.com").ID); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.lead.Get(b.templatesPath()).Body, "Hedef kolon silinmiş") {
		t.Fatal("flagged while its column is there")
	}
	rows := []store.ColumnRow{{ID: b.cols[0].ID, Name: "Todo", AllowCreate: true}, {ID: doing, Delete: true}, {ID: b.cols[2].ID, Name: "Done", IsDone: true}}
	if err := b.h.store.SaveColumns(ctx, b.board.ID, rows); err != nil {
		t.Fatal(err)
	}
	mustContain(t, b.lead.Get(b.templatesPath()).Body, "Orphan", "Hedef kolon silinmiş; şablon zamanlamayla kart açamaz.")
}

func TestTheLastRunIsShown(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	lead := b.h.user("lead@example.com").ID
	doing := b.cols[1].ID
	in := store.TemplateInput{Name: "Daily", Title: "Standup", ColumnID: &doing, Schedule: store.Schedule{Kind: "daily", MonthDay: 1, Hour: 9}}
	tpl, err := b.h.store.CreateTemplate(ctx, b.board.ID, in, lead)
	if err != nil {
		t.Fatal(err)
	}
	run := func(at time.Time) {
		t.Helper()
		if _, err := b.h.store.RunTemplate(ctx, store.ScheduledTemplate{Template: tpl, TeamID: b.team.ID}, at, time.UTC); err != nil {
			t.Fatal(err)
		}
	}
	run(time.Now().Add(-time.Hour))
	mustContain(t, b.lead.Get(b.templatesPath()).Body, "Son çalışma: kart açıldı")

	// Doing is now full: the next run is refused by the board's WIP limit.
	limit := 1
	if err := b.h.store.SetWIPLimit(ctx, b.board.ID, doing, &limit); err != nil {
		t.Fatal(err)
	}
	run(time.Now())
	page := b.lead.Get(b.templatesPath()).Body
	mustContain(t, page, "Son çalışma başarısız:", "Doing kolonunun WIP limiti (1) dolu.")
	if strings.Contains(page, "Son çalışma: kart açıldı") {
		t.Error("the earlier run is shown, not the latest")
	}

	b.h.speaks("lead@example.com", "en")
	mustContain(t, b.lead.Get("/en"+b.templatesPath()).Body, "The last run failed:", "Doing is at its WIP limit (1).")

	// A reason no rule gives is shown in the reader's language too.
	member := b.h.user("member@example.com").ID
	gone, err := b.h.store.CreateTemplate(ctx, b.board.ID, store.TemplateInput{Name: "Gone", Title: "Card", ColumnID: &doing,
		Schedule: store.Schedule{Kind: "daily", MonthDay: 1, Hour: 9}}, member)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.h.store.RemoveMember(ctx, b.team.ID, member); err != nil {
		t.Fatal(err)
	}
	if _, err := b.h.store.RunTemplate(ctx, store.ScheduledTemplate{Template: gone, TeamID: b.team.ID}, time.Now(), time.UTC); err != nil {
		t.Fatal(err)
	}
	mustContain(t, b.lead.Get("/en"+b.templatesPath()).Body, "The person who last saved the template is no longer in the team.")
}

// templateOnBoard makes a template titled "Weekly report" named "Weekly".
func (b boardSetup) templateOnBoard(t *testing.T, in store.TemplateInput) store.Template {
	t.Helper()
	if in.Name == "" {
		in.Name = "Weekly"
	}
	if in.Title == "" {
		in.Title = "Weekly report"
	}
	tpl, err := b.h.store.CreateTemplate(context.Background(), b.board.ID, in, b.h.user("lead@example.com").ID)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func TestEveryAddCardFormOffersTheTemplates(t *testing.T) {
	b := newBoardSetup(t)
	if strings.Contains(b.lead.Get(b.path).Body, `name="template"`) {
		t.Fatal("a template choice without any template")
	}
	tpl := b.templateOnBoard(t, store.TemplateInput{})
	for _, lane := range []string{"", "?lane=assignee"} {
		body := b.lead.Get(b.path + lane).Body
		mustContain(t, body, `name="template"`, `<option value="">Şablonsuz</option>`, `<option value="`+id(tpl.ID)+`">Weekly</option>`)
		if n := strings.Count(body, `name="template"`); n != strings.Count(body, `class="add-card__form"`) {
			t.Errorf("lane %q: %d template choices for %d add-card forms", lane, n, strings.Count(body, `class="add-card__form"`))
		}
	}
	if strings.Contains(b.lead.Get(b.path).Body, `name="title" rows="2" required`) {
		t.Error("the title is still required in the browser, which blocks a template")
	}
}

func TestACardIsMadeFromATemplateInTheFormsColumn(t *testing.T) {
	b := newBoardSetup(t)
	doing := b.cols[1].ID
	tpl := b.templateOnBoard(t, store.TemplateInput{ColumnID: &doing})
	todo := b.cols[0].ID

	res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "column": {id(todo)}, "title": {""}, "template": {id(tpl.ID)}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("create from template = %d:\n%s", res.Status, res.Body)
	}
	cards, err := b.h.store.BoardCards(context.Background(), b.board.ID)
	if err != nil || len(cards) != 1 {
		t.Fatalf("cards = %+v, %v", cards, err)
	}
	if c := cards[0].Card; c.Title != "Weekly report" || c.ColumnID != todo {
		t.Fatalf("card = %q in column %d, want the template's title in the form's column %d", c.Title, c.ColumnID, todo)
	}
}

func TestADroppedAssigneeIsReported(t *testing.T) {
	b := newBoardSetup(t)
	member := b.h.user("member@example.com").ID
	tpl := b.templateOnBoard(t, store.TemplateInput{AssigneeID: &member})
	if err := b.h.store.RemoveMember(context.Background(), b.team.ID, member); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"op": {"create_card"}, "column": {id(b.cols[0].ID)}, "template": {id(tpl.ID)}}
	res := b.lead.SubmitFetch(b.path, b.path, form)
	if res.Status != http.StatusOK {
		t.Fatalf("fetch = %d:\n%s", res.Status, res.Body)
	}
	mustContain(t, res.Body, "Şablonun atanan kişisi kartı alamadı; kart atanmamış açıldı.", "Weekly report")
	cards, _ := b.h.store.BoardCards(context.Background(), b.board.ID)
	if len(cards) != 1 || cards[0].Card.AssigneeID != nil {
		t.Fatalf("cards = %+v, want one unassigned card", cards)
	}

	res = b.lead.Submit(b.path, b.path, form)
	if res.Status != http.StatusSeeOther {
		t.Fatalf("form post = %d", res.Status)
	}
	mustContain(t, b.lead.Get(b.path).Body, "Şablonun atanan kişisi kartı alamadı")
}

func TestATemplateDoesNotExcuseAMissingTitleWithoutOne(t *testing.T) {
	b := newBoardSetup(t)
	b.templateOnBoard(t, store.TemplateInput{})
	res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "title": {" "}, "template": {""}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("no title, no template = %d, want 422", res.Status)
	}
	if cards, _ := b.h.store.BoardCards(context.Background(), b.board.ID); len(cards) != 0 {
		t.Fatalf("cards = %+v", cards)
	}
}

func TestAnotherBoardsTemplateIsRefused(t *testing.T) {
	b := newBoardSetup(t)
	other, err := b.h.store.CreateBoard(context.Background(), b.team.ID, "Other", []string{"Todo"})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := b.h.store.CreateTemplate(context.Background(), other.ID, store.TemplateInput{Name: "Theirs", Title: "Theirs"}, b.h.user("lead@example.com").ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{id(foreign.ID), "999999", "abc"} {
		res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "title": {"x"}, "template": {bad}})
		if res.Status != http.StatusBadRequest {
			t.Errorf("template %q = %d, want 400", bad, res.Status)
		}
	}
	if cards, _ := b.h.store.BoardCards(context.Background(), b.board.ID); len(cards) != 0 {
		t.Fatalf("cards = %+v", cards)
	}
}

func TestAFailedTemplateRunIsToldWithoutACardLink(t *testing.T) {
	b := newBoardSetup(t)
	member := b.h.user("member@example.com").ID
	_, err := b.h.store.CreateNotification(context.Background(), store.NewNotification{
		UserID: member, Kind: store.NotifyTemplateFailed,
		Payload: store.NotificationPayload{CardTitle: "Weekly", BoardID: b.board.ID, BoardName: "Sprint"},
	})
	if err != nil {
		t.Fatal(err)
	}
	page := b.member.Get("/notifications").Body
	mustContain(t, page, "Sprint board&#39;unda Weekly şablonu kart açamadı.")
	if strings.Contains(page, `class="notification__text" href=`) {
		t.Error("a notification without a card links to one")
	}
	b.h.speaks("member@example.com", "en")
	mustContain(t, b.member.Get("/en/notifications").Body, "The template Weekly on Sprint could not make its card.")
	mustContain(t, b.member.Get("/en/me/settings").Body, "When a scheduled template cannot make its card", `name="email_template_failed"`)
}
