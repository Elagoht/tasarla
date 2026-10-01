package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

// Battle test: an archived card takes no labels, checklist items, comments
// or dependencies.
func TestAnArchivedCardTakesNoWrites(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	c, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Card", f.lead.ID)
	other, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Other", f.lead.ID)
	l, _ := f.s.CreateLabel(ctx, f.board.ID, "bug", "#e03131")
	if err := f.s.ArchiveCard(ctx, f.board.ID, c.ID, f.lead.ID); err != nil {
		t.Fatal(err)
	}
	checks := map[string]error{
		"labels":  f.s.SetCardLabels(ctx, f.board.ID, c.ID, []int64{l.ID}),
		"dep":     f.s.AddDependency(ctx, f.board.ID, other.ID, c.ID),
		"blocker": f.s.AddDependency(ctx, f.board.ID, c.ID, other.ID),
	}
	_, checks["checklist"] = f.s.AddChecklistItem(ctx, f.board.ID, c.ID, "x")
	_, checks["comment"] = f.s.AddComment(ctx, f.board.ID, c.ID, f.lead.ID, "x", nil)
	for name, err := range checks {
		if !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s on an archived card = %v, want ErrNotFound", name, err)
		}
	}
}

// Battle test: an archived board takes no writes.
func TestAnArchivedBoardTakesNoWrites(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	c, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Card", f.lead.ID)
	if err := f.s.ArchiveBoard(ctx, f.board.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Late", f.lead.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("create on an archived board = %v", err)
	}
	if _, err := f.s.UpdateCardField(ctx, f.board.ID, c.ID, c.Version, store.FieldTitle, store.CardFields{Title: "Late"}, f.lead.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("edit on an archived board = %v", err)
	}
}

// Battle test: a role deleted while a permission naming it is added either
// refuses (in use) or leaves the add failing — the permission is never
// silently dropped.
func TestARoleDeleteNeverDropsAPermission(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	for range 60 {
		r, err := f.s.CreateBoardRole(ctx, f.board.ID, "QA")
		if err != nil {
			t.Fatal(err)
		}
		var addErr, delErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			addErr = f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: f.cols[1].ID, Subject: rules.SubjectBoardRole, BoardRoleID: &r.ID})
		}()
		go func() { defer wg.Done(); delErr = f.s.DeleteBoardRole(ctx, f.board.ID, r.ID) }()
		wg.Wait()
		rs, _ := f.s.BoardRules(ctx, f.board.ID)
		kept := len(rs.Permissions) > 0
		switch {
		case addErr == nil && delErr == nil:
			t.Fatalf("both succeeded; permission kept = %v", kept)
		case addErr != nil && !errors.Is(addErr, store.ErrNotFound):
			t.Fatalf("add = %v, want ErrNotFound when the role went first", addErr)
		case delErr != nil && !errors.Is(delErr, store.ErrInUse):
			t.Fatalf("delete = %v, want ErrInUse when the permission came first", delErr)
		}
		for _, p := range rs.Permissions {
			_ = f.s.DeleteMovePermission(ctx, f.board.ID, p.ID)
		}
		_ = f.s.DeleteBoardRole(ctx, f.board.ID, r.ID)
	}
}

// Battle test: a column table from a form older than a new column keeps
// positions distinct.
func TestAStaleColumnTableKeepsPositionsDistinct(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	stale := []store.ColumnRow{{ID: f.cols[0].ID, Name: "Todo", AllowCreate: true}, {ID: f.cols[1].ID, Name: "Doing"},
		{ID: f.cols[2].ID, Name: "Done", IsDone: true}}
	added := append([]store.ColumnRow{{Name: "New"}}, stale...)
	if err := f.s.SaveColumns(ctx, f.board.ID, added); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SaveColumns(ctx, f.board.ID, stale); err != nil {
		t.Fatal(err)
	}
	cols, _ := f.s.Columns(ctx, f.board.ID)
	seen := map[int]bool{}
	for i, c := range cols {
		if seen[c.Position] || c.Position != i {
			t.Fatalf("positions = %+v", cols)
		}
		seen[c.Position] = true
	}
}
