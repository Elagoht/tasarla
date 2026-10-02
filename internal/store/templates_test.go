package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func templateInput(f boardFixture, name string) store.TemplateInput {
	p, d := int16(3), 2
	col := f.cols[1].ID
	return store.TemplateInput{Name: name, Title: "Haftalık rapor", Description: "Ne yapıldı?", Priority: &p,
		AssigneeID: &f.member.ID, ColumnID: &col, DueInDays: &d, Checklist: []string{"Topla", "Yaz"},
		Schedule: store.Schedule{Kind: "weekly", Weekdays: 1, Hour: 9}}
}

func TestTemplateCRUD(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	label, _ := f.s.CreateLabel(ctx, f.board.ID, "rapor", "#228be6")
	in := templateInput(f, "Rapor")
	in.LabelIDs = []int64{label.ID}
	tpl, err := f.s.CreateTemplate(ctx, f.board.ID, in, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.s.Template(ctx, f.board.ID, tpl.ID)
	if err != nil || got.Title != "Haftalık rapor" || !slices.Equal(got.Checklist, []string{"Topla", "Yaz"}) ||
		!slices.Equal(got.LabelIDs, []int64{label.ID}) || got.Schedule.Weekdays != 1 || got.ScheduleSince == nil || got.UpdatedBy != f.lead.ID {
		t.Fatalf("template = %+v, %v", got, err)
	}
	if _, err := f.s.CreateTemplate(ctx, f.board.ID, templateInput(f, "Rapor"), f.lead.ID); !errors.Is(err, store.ErrTemplateName) {
		t.Errorf("duplicate name = %v", err)
	}
	// An edit that leaves the schedule alone keeps schedule_since.
	in.Title = "Rapor (güncel)"
	upd, err := f.s.UpdateTemplate(ctx, f.board.ID, tpl.ID, in, f.member.ID)
	if err != nil || upd.Title != "Rapor (güncel)" || !upd.ScheduleSince.Equal(*got.ScheduleSince) || upd.UpdatedBy != f.member.ID {
		t.Fatalf("update = %+v, %v", upd, err)
	}
	if err := f.s.DeleteLabel(ctx, f.board.ID, label.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteColumn(ctx, f.board.ID, f.cols[1].ID); err != nil {
		t.Fatal(err)
	}
	after, _ := f.s.Template(ctx, f.board.ID, tpl.ID)
	if len(after.LabelIDs) != 0 || after.ColumnID != nil {
		t.Errorf("deleted label/column kept: %+v", after)
	}
	if err := f.s.DeleteTemplate(ctx, f.board.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Template(ctx, f.board.ID, tpl.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleted template = %v", err)
	}
}

func TestTemplateScheduleSince(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	in := templateInput(f, "Rapor")
	tpl, err := f.s.CreateTemplate(ctx, f.board.ID, in, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	in.Schedule.Minute = 30
	moved, err := f.s.UpdateTemplate(ctx, f.board.ID, tpl.ID, in, f.lead.ID)
	if err != nil || !moved.ScheduleSince.After(*tpl.ScheduleSince) || moved.Schedule.Minute != 30 {
		t.Fatalf("changed schedule = %+v, %v", moved, err)
	}
	in.Schedule = store.Schedule{}
	off, err := f.s.UpdateTemplate(ctx, f.board.ID, tpl.ID, in, f.lead.ID)
	if err != nil || off.ScheduleSince != nil || off.Schedule.Kind != "" {
		t.Fatalf("schedule off = %+v, %v", off, err)
	}
	if _, err := f.s.UpdateTemplate(ctx, f.board.ID+999, tpl.ID, in, f.lead.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("template of another board = %v", err)
	}
}

func TestCardFromTemplate(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	label, _ := f.s.CreateLabel(ctx, f.board.ID, "rapor", "#228be6")
	in := templateInput(f, "Rapor")
	in.LabelIDs = []int64{label.ID}
	tpl, err := f.s.CreateTemplate(ctx, f.board.ID, in, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 23, 30, 0, 0, time.UTC)
	got, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, tpl.ID, 0, f.lead.ID, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	c := got.Card
	if c.ColumnID != f.cols[1].ID || c.Title != "Haftalık rapor" || c.Description != "Ne yapıldı?" ||
		c.Priority == nil || *c.Priority != 3 || c.AssigneeID == nil || *c.AssigneeID != f.member.ID ||
		c.CreatedBy != f.lead.ID || got.AssigneeDropped {
		t.Fatalf("card = %+v, dropped %v", c, got.AssigneeDropped)
	}
	if want := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC); c.DueDate == nil || !c.DueDate.Equal(want) {
		t.Errorf("due = %v, want %v", c.DueDate, want)
	}
	labels, err := f.s.CardLabels(ctx, c.ID)
	if err != nil || !slices.Equal(labels, []int64{label.ID}) {
		t.Errorf("labels = %v, %v", labels, err)
	}
	items, err := f.s.ChecklistItems(ctx, c.ID)
	if err != nil || len(items) != 2 || items[0].Text != "Topla" || items[1].Text != "Yaz" || items[0].Done {
		t.Errorf("checklist = %+v, %v", items, err)
	}

	// Today is taken in loc: 23:30 UTC is already the 3rd in Istanbul.
	ist := time.FixedZone("Istanbul", 3*60*60)
	there, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, tpl.ID, f.cols[0].ID, f.lead.ID, ist, now)
	if err != nil {
		t.Fatal(err)
	}
	if there.Card.ColumnID != f.cols[0].ID {
		t.Errorf("column = %d, want %d", there.Card.ColumnID, f.cols[0].ID)
	}
	if want := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC); there.Card.DueDate == nil || !there.Card.DueDate.Equal(want) {
		t.Errorf("due in Istanbul = %v, want %v", there.Card.DueDate, want)
	}

	// A template without a column makes its card in the first creatable one.
	in.Name, in.ColumnID = "Kolonsuz", nil
	loose, err := f.s.CreateTemplate(ctx, f.board.ID, in, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, loose.ID, 0, f.lead.ID, time.UTC, now)
	if err != nil || first.Card.ColumnID != f.cols[0].ID {
		t.Errorf("columnless = %+v, %v", first.Card, err)
	}

	if _, err := f.s.CreateCardFromTemplate(ctx, f.board.ID+999, tpl.ID, 0, f.lead.ID, time.UTC, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("template of another board = %v", err)
	}
	other, err := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"A"})
	if err != nil {
		t.Fatal(err)
	}
	otherCols, _ := f.s.Columns(ctx, other.ID)
	if _, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, tpl.ID, otherCols[0].ID, f.lead.ID, time.UTC, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("column of another board = %v", err)
	}
}

func TestCardFromTemplateDropsTheAssignee(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	tpl, err := f.s.CreateTemplate(ctx, f.board.ID, templateInput(f, "Rapor"), f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	one := 1
	if err := f.s.SetPersonWIP(ctx, f.board.ID, &one, []int64{f.cols[1].ID}); err != nil {
		t.Fatal(err)
	}
	busy, err := f.s.CreateCard(ctx, f.board.ID, f.cols[1].ID, "Meşgul", f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.UpdateCardField(ctx, f.board.ID, busy.ID, busy.Version, store.FieldAssignee,
		store.CardFields{AssigneeID: &f.member.ID}, f.lead.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	got, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, tpl.ID, 0, f.lead.ID, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Card.AssigneeID != nil || !got.AssigneeDropped {
		t.Errorf("over WIP: assignee %v, dropped %v", got.Card.AssigneeID, got.AssigneeDropped)
	}

	if err := f.s.SetPersonWIP(ctx, f.board.ID, nil, []int64{}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.RemoveMember(ctx, f.team.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	gone, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, tpl.ID, 0, f.lead.ID, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	if gone.Card.AssigneeID != nil || !gone.AssigneeDropped {
		t.Errorf("not in the team: assignee %v, dropped %v", gone.Card.AssigneeID, gone.AssigneeDropped)
	}
}

func TestCardFromTemplateRefused(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	in := templateInput(f, "Tarihsiz")
	in.DueInDays = nil
	undated, err := f.s.CreateTemplate(ctx, f.board.ID, in, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	_, err = f.s.CreateCardFromTemplate(ctx, f.board.ID, undated.ID, 0, f.lead.ID, time.UTC, now)
	var re *store.RuleError
	if !errors.As(err, &re) || len(re.Violations) != 1 {
		t.Fatalf("no due date = %v", err)
	}
	if cards, err := f.s.BoardCards(ctx, f.board.ID); err != nil || len(cards) != 0 {
		t.Fatalf("refused card was made: %d cards, %v", len(cards), err)
	}

	// The rules see the card as the template makes it, due date included.
	dated, err := f.s.CreateTemplate(ctx, f.board.ID, templateInput(f, "Tarihli"), f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, dated.ID, 0, f.lead.ID, time.UTC, now); err != nil {
		t.Errorf("dated template = %v", err)
	}
}

// A board with no columns has nowhere to make the card; a nil location is UTC.
func TestCardFromTemplateEdges(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	empty, err := f.s.CreateBoard(ctx, f.team.ID, "Boş", nil)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := f.s.CreateTemplate(ctx, empty.ID, store.TemplateInput{Name: "T", Title: "T"}, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.CreateCardFromTemplate(ctx, empty.ID, tpl.ID, 0, f.lead.ID, time.UTC, time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("no columns = %v, want ErrNotFound", err)
	}

	days := 1
	in := templateInput(f, "UTC")
	in.DueInDays, in.AssigneeID = &days, nil
	dated, err := f.s.CreateTemplate(ctx, f.board.ID, in, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 23, 30, 0, 0, time.UTC)
	got, err := f.s.CreateCardFromTemplate(ctx, f.board.ID, dated.ID, 0, f.lead.ID, nil, now)
	if err != nil || got.Card.DueDate == nil || got.Card.DueDate.Format(time.DateOnly) != "2026-10-03" {
		t.Fatalf("nil location: %+v, %v", got.Card.DueDate, err)
	}
}
