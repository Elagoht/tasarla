package store_test

import (
	"context"
	"testing"
	"time"

	"kanban/internal/store"
)

func intp(n int) *int { return &n }

func fullPlan() store.BoardPlan {
	high := int16(3)
	return store.BoardPlan{
		Columns: []store.PlanColumn{
			{Name: "Backlog", AllowCreate: true},
			{Name: "Doing", WIP: intp(3), CountsPersonWIP: true},
			{Name: "Done", Done: true},
		},
		Labels: []store.PlanLabel{{Name: "Bug", Color: "#E03131"}, {Name: "Story", Color: "#1971c2"}},
		Templates: []store.PlanTemplate{{
			Name: "Bug report", Title: "New bug", Description: "**Steps**", Priority: &high,
			ColumnIndex: 0, LabelIndexes: []int{0}, Checklist: []string{"Reproduced", "Fixed"},
			Schedule: store.Schedule{Kind: "weekly", Weekdays: 1 << 4, Hour: 16},
		}},
		PersonWIP:   intp(2),
		Permissions: []store.PlanPermission{{ToIndex: 2, Subject: "team_lead"}},
		Conditions:  []store.PlanCondition{{ColumnIndex: 1, Phase: "enter", Kind: "has_assignee"}},
	}
}

func TestCreateBoardFromPlan(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	before := time.Now().Add(-time.Second)
	b, err := f.s.CreateBoardFromPlan(ctx, f.team.ID, "Bugs", fullPlan(), f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	cols, err := f.s.Columns(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 || cols[0].Name != "Backlog" || !cols[0].AllowCreate || cols[1].AllowCreate ||
		cols[1].WIPLimit == nil || *cols[1].WIPLimit != 3 || !cols[1].CountsPersonWIP || !cols[2].IsDone || cols[0].IsDone {
		t.Fatalf("columns = %+v", cols)
	}
	labels, _ := f.s.Labels(ctx, b.ID)
	if len(labels) != 2 || labels[0].Name != "Bug" || labels[0].Color != "#e03131" {
		t.Fatalf("labels = %+v", labels)
	}
	tpls, err := f.s.Templates(ctx, b.ID)
	if err != nil || len(tpls) != 1 {
		t.Fatalf("templates = %+v, %v", tpls, err)
	}
	tp := tpls[0]
	if tp.ColumnID == nil || *tp.ColumnID != cols[0].ID || len(tp.LabelIDs) != 1 || tp.LabelIDs[0] != labels[0].ID ||
		len(tp.Checklist) != 2 || tp.Priority == nil || *tp.Priority != 3 || tp.Schedule.Kind != "weekly" ||
		tp.ScheduleSince == nil || tp.ScheduleSince.Before(before) || tp.UpdatedBy != f.lead.ID {
		t.Fatalf("template = %+v", tp)
	}
	r, err := f.s.BoardRules(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Permissions) != 1 || r.Permissions[0].ToColumnID != cols[2].ID || r.Permissions[0].Subject != "team_lead" ||
		len(r.Conditions) != 1 || r.Conditions[0].ColumnID != cols[1].ID || r.Conditions[0].Kind != "has_assignee" {
		t.Fatalf("rules = %+v", r)
	}
	got, err := f.s.Board(ctx, b.ID)
	if err != nil || got.PersonWIPLimit == nil || *got.PersonWIPLimit != 2 {
		t.Fatalf("board = %+v, %v", got, err)
	}
}

// A plan that fails half way leaves nothing: no board, no columns.
func TestCreateBoardFromPlanRollsBack(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	p := fullPlan()
	p.Conditions = append(p.Conditions, store.PlanCondition{ColumnIndex: 1, Phase: "enter", Kind: "nonsense"})
	if _, err := f.s.CreateBoardFromPlan(ctx, f.team.ID, "Broken", p, f.lead.ID); err == nil {
		t.Fatal("a plan with a bad condition was built")
	}
	boards, err := f.s.BoardsOfTeam(ctx, f.team.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range boards {
		if b.Name == "Broken" {
			t.Fatal("the failed board is left behind")
		}
	}
}

// An index past the plan's columns or labels is refused, not silently dropped.
func TestCreateBoardFromPlanRefusesBadIndexes(t *testing.T) {
	f := newBoardFixture(t)
	p := fullPlan()
	p.Permissions = []store.PlanPermission{{ToIndex: 9, Subject: "team_lead"}}
	if _, err := f.s.CreateBoardFromPlan(context.Background(), f.team.ID, "X", p, f.lead.ID); err == nil {
		t.Fatal("a permission into column 9 of 3 was built")
	}
}

func TestColumnsPlanKeepsTodaysDefaults(t *testing.T) {
	p := store.ColumnsPlan([]string{"A", "B", "C"})
	if len(p.Columns) != 3 || !p.Columns[0].AllowCreate || p.Columns[1].AllowCreate || !p.Columns[2].Done || p.Columns[0].Done {
		t.Fatalf("plan = %+v", p.Columns)
	}
	if one := store.ColumnsPlan([]string{"Only"}); one.Columns[0].Done {
		t.Fatal("a one-column board's only column is done")
	}
}
