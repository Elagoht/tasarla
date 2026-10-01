package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/store"
)

func TestLabels(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	bug, err := f.s.CreateLabel(ctx, f.board.ID, "bug", "#FF0000")
	if err != nil || bug.Color != "#ff0000" {
		t.Fatalf("CreateLabel = %+v, %v", bug, err)
	}
	if _, err := f.s.CreateLabel(ctx, f.board.ID, "bug", "#00ff00"); err == nil {
		t.Error("a duplicate label name was stored")
	}
	if _, err := f.s.CreateLabel(ctx, f.board.ID, "ui", "red"); err == nil {
		t.Error("a colour that is not #rrggbb was stored")
	}
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	foreign, _ := f.s.CreateLabel(ctx, other.ID, "foreign", "#000000")

	a := f.card(t, 0, "A")
	if err := f.s.SetCardLabels(ctx, f.board.ID, a.ID, []int64{bug.ID, foreign.ID}); err != nil {
		t.Fatal(err)
	}
	ids, _ := f.s.CardLabels(ctx, a.ID)
	if len(ids) != 1 || ids[0] != bug.ID {
		t.Errorf("card labels = %v, want only the board's own", ids)
	}
	view, _ := f.s.BoardCards(ctx, f.board.ID)
	if len(view[0].Labels) != 1 || view[0].Labels[0].Name != "bug" {
		t.Errorf("board view labels = %+v", view[0].Labels)
	}
	if err := f.s.DeleteLabel(ctx, other.ID, bug.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleting through another board: %v", err)
	}
	if err := f.s.DeleteLabel(ctx, f.board.ID, bug.ID); err != nil {
		t.Fatal(err)
	}
	if ids, _ := f.s.CardLabels(ctx, a.ID); len(ids) != 0 {
		t.Errorf("a deleted label stayed on the card")
	}
}

func TestChecklist(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	one, err := f.s.AddChecklistItem(ctx, f.board.ID, a.ID, "one")
	if err != nil {
		t.Fatal(err)
	}
	two, _ := f.s.AddChecklistItem(ctx, f.board.ID, a.ID, "two")
	if one.Position != 0 || two.Position != 1 {
		t.Errorf("positions %d %d", one.Position, two.Position)
	}
	if err := f.s.SetChecklistItemDone(ctx, f.board.ID, a.ID, one.ID, true); err != nil {
		t.Fatal(err)
	}
	items, _ := f.s.ChecklistItems(ctx, a.ID)
	if len(items) != 2 || !items[0].Done || items[1].Done {
		t.Errorf("items = %+v", items)
	}
	view, _ := f.s.BoardCards(ctx, f.board.ID)
	if view[0].ChecklistDone != 1 || view[0].ChecklistTotal != 2 {
		t.Errorf("counts = %d/%d", view[0].ChecklistDone, view[0].ChecklistTotal)
	}
	b := f.card(t, 0, "B")
	if err := f.s.SetChecklistItemDone(ctx, f.board.ID, b.ID, one.ID, false); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("toggling another card's item: %v", err)
	}
	if err := f.s.DeleteChecklistItem(ctx, f.board.ID, a.ID, one.ID); err != nil {
		t.Fatal(err)
	}
	if items, _ := f.s.ChecklistItems(ctx, a.ID); len(items) != 1 || items[0].Position != 0 {
		t.Errorf("after delete = %+v", items)
	}
	if c, _ := f.s.Card(ctx, f.board.ID, a.ID); c.Version <= a.Version {
		t.Error("checklist changes did not bump the card's version")
	}
}

func TestDependencies(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a, b, c := f.card(t, 0, "A"), f.card(t, 0, "B"), f.card(t, 0, "C")
	if err := f.s.AddDependency(ctx, f.board.ID, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.AddDependency(ctx, f.board.ID, b.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	// A → B → C: C → A would close a cycle.
	if err := f.s.AddDependency(ctx, f.board.ID, c.ID, a.ID); !errors.Is(err, store.ErrCycle) {
		t.Fatalf("C → A: %v, want ErrCycle", err)
	}
	if err := f.s.AddDependency(ctx, f.board.ID, a.ID, a.ID); !errors.Is(err, store.ErrCycle) {
		t.Errorf("A → A: %v", err)
	}
	if err := f.s.AddDependency(ctx, f.board.ID, a.ID, b.ID); err != nil {
		t.Errorf("adding an existing dependency again: %v", err)
	}
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	otherCols, _ := f.s.Columns(ctx, other.ID)
	foreign, _ := f.s.CreateCard(ctx, other.ID, otherCols[0].ID, "F", f.lead.ID)
	if err := f.s.AddDependency(ctx, f.board.ID, foreign.ID, a.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a blocker from another board: %v", err)
	}

	deps, err := f.s.CardDependencies(ctx, b.ID)
	if err != nil || len(deps.Blockers) != 1 || deps.Blockers[0].ID != a.ID || len(deps.Blocking) != 1 || deps.Blocking[0].ID != c.ID {
		t.Fatalf("CardDependencies(B) = %+v, %v", deps, err)
	}
	view, _ := f.s.BoardCards(ctx, f.board.ID)
	blocked := map[string]bool{}
	for _, v := range view {
		blocked[v.Card.Title] = v.Blocked
	}
	if blocked["A"] || !blocked["B"] || !blocked["C"] {
		t.Errorf("blocked = %v", blocked)
	}
	// A finished: B is no longer blocked.
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[2].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version}); err != nil {
		t.Fatal(err)
	}
	view, _ = f.s.BoardCards(ctx, f.board.ID)
	for _, v := range view {
		if v.Card.Title == "B" && v.Blocked {
			t.Error("B is blocked by a done card")
		}
	}
	if err := f.s.RemoveDependency(ctx, f.board.ID, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.RemoveDependency(ctx, f.board.ID, a.ID, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("removing twice: %v", err)
	}
}

func TestAssignedTo(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	b := f.card(t, 1, "B")
	f.card(t, 0, "C")
	for _, c := range []store.Card{a, b} {
		if _, err := f.s.UpdateCard(ctx, f.board.ID, c.ID, c.Version, store.CardFields{Title: c.Title, AssigneeID: &f.member.ID}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.s.ArchiveCard(ctx, f.board.ID, b.ID, 0); err != nil {
		t.Fatal(err)
	}
	tasks, err := f.s.AssignedTo(ctx, f.member.ID)
	if err != nil || len(tasks) != 1 || tasks[0].Card.Title != "A" || tasks[0].BoardName != "Sprint" || tasks[0].ColumnName != "Todo" {
		t.Fatalf("AssignedTo = %+v, %v", tasks, err)
	}
	// Leaving the team hides the board's cards.
	if err := f.s.RemoveMember(ctx, f.team.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	if tasks, _ := f.s.AssignedTo(ctx, f.member.ID); len(tasks) != 0 {
		t.Errorf("cards of a team the user left: %+v", tasks)
	}
}
