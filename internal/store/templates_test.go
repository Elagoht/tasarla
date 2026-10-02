package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"

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
