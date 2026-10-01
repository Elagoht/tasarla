package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func lead(f boardFixture) rules.Actor {
	return rules.Actor{UserID: f.lead.ID, TeamRole: rules.RoleLead}
}

func assign(t *testing.T, f boardFixture, c store.Card, to *int64) store.Card {
	t.Helper()
	out, err := f.s.UpdateCardField(context.Background(), f.board.ID, c.ID, c.Version, store.FieldAssignee,
		store.CardFields{AssigneeID: to}, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func laneMove(f boardFixture, c store.Card, toCol int, target store.LaneTarget) store.Move {
	return store.Move{BoardID: f.board.ID, CardID: c.ID, ToColumnID: f.cols[toCol].ID,
		ExpectedFrom: c.ColumnID, ExpectedVersion: c.Version, Actor: lead(f), Lane: &target}
}

// Dropped on the member's lane of the next column: moved and given to them, in
// one change, with both written to the history.
func TestLaneMoveChangesColumnAndAssignee(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	c := f.card(t, 0, "A")
	res, err := f.s.MoveCardWith(ctx, laneMove(f, c, 1, store.LaneTarget{Field: store.FieldAssignee, AssigneeID: &f.member.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if res.After.ColumnID != f.cols[1].ID || res.After.AssigneeID == nil || *res.After.AssigneeID != f.member.ID {
		t.Fatalf("after = %+v", res.After)
	}
	if res.Before.AssigneeID != nil || res.Before.ColumnID != f.cols[0].ID {
		t.Fatalf("before = %+v", res.Before)
	}
	acts, err := f.s.CardActivity(ctx, c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, a := range acts {
		kinds[a.Kind] = true
	}
	if !kinds[store.ActivityCardMoved] || !kinds[store.ActivityCardUpdated] {
		t.Errorf("activity kinds = %v", kinds)
	}
}

// A refused part refuses the whole: the card stays where it was, as it was.
func TestLaneMoveRefusedChangesNothing(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	one := 1
	if err := f.s.SetPersonWIP(ctx, f.board.ID, &one, []int64{f.cols[1].ID}); err != nil {
		t.Fatal(err)
	}
	assign(t, f, f.card(t, 1, "Busy"), &f.member.ID)
	c := f.card(t, 0, "A")
	_, err := f.s.MoveCardWith(ctx, laneMove(f, c, 1, store.LaneTarget{Field: store.FieldAssignee, AssigneeID: &f.member.ID}))
	var re *store.RuleError
	if !errors.As(err, &re) || len(re.Violations) != 1 || re.Violations[0].Code != "rules.wip_person" {
		t.Fatalf("err = %v, want one rules.wip_person", err)
	}
	now, err := f.s.Card(ctx, f.board.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if now.ColumnID != c.ColumnID || now.AssigneeID != nil || now.Version != c.Version || now.Position != c.Position {
		t.Errorf("refused move changed the card: %+v", now)
	}
}

// Both columns count the person's WIP: the lane drop itself checks it, since
// Evaluate (which covers only a card coming from an uncounted column) does not.
func TestLaneMoveBetweenCountedColumnsChecksPersonWIP(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	one := 1
	if err := f.s.SetPersonWIP(ctx, f.board.ID, &one, []int64{f.cols[0].ID, f.cols[1].ID}); err != nil {
		t.Fatal(err)
	}
	assign(t, f, f.card(t, 1, "Busy"), &f.member.ID)
	c := f.card(t, 0, "A")
	_, err := f.s.MoveCardWith(ctx, laneMove(f, c, 1, store.LaneTarget{Field: store.FieldAssignee, AssigneeID: &f.member.ID}))
	var re *store.RuleError
	if !errors.As(err, &re) || len(re.Violations) != 1 || re.Violations[0].Code != "rules.wip_person" {
		t.Fatalf("err = %v, want one rules.wip_person", err)
	}
	now, err := f.s.Card(ctx, f.board.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if now.ColumnID != c.ColumnID || now.AssigneeID != nil || now.Version != c.Version || now.Position != c.Position {
		t.Errorf("refused move changed the card: %+v", now)
	}
}

// Same column, another person's lane: no column rule runs, the person's WIP does.
func TestLaneChangeInOneColumnChecksOnlyTheAssignment(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	c := f.card(t, 0, "A") // before the condition: creating is entering
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	res, err := f.s.MoveCardWith(ctx, laneMove(f, c, 0, store.LaneTarget{Field: store.FieldAssignee, AssigneeID: &f.member.ID}))
	if err != nil {
		t.Fatalf("an entry condition ran for a move inside the column: %v", err)
	}
	if res.After.AssigneeID == nil || *res.After.AssigneeID != f.member.ID {
		t.Errorf("not assigned: %+v", res.After)
	}
}

// The new value is what entry conditions see.
func TestLaneMoveEntryConditionSeesTheNewAssignee(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasAssignee}); err != nil {
		t.Fatal(err)
	}
	c := f.card(t, 0, "A")
	if _, err := f.s.MoveCardWith(ctx, laneMove(f, c, 1, store.LaneTarget{Field: store.FieldAssignee, AssigneeID: &f.member.ID})); err != nil {
		t.Fatalf("has_assignee did not see the lane's assignee: %v", err)
	}
	d := f.card(t, 0, "B")
	_, err := f.s.MoveCardWith(ctx, laneMove(f, d, 1, store.LaneTarget{Field: store.FieldAssignee}))
	var re *store.RuleError
	if !errors.As(err, &re) {
		t.Fatalf("unassigned into a has_assignee column: err = %v", err)
	}
}

func TestLaneMoveToSomeoneOutsideTheTeam(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	outsider := mustUpsert(t, f.s, identity("out", "out@example.com"))
	c := f.card(t, 0, "A")
	_, err := f.s.MoveCardWith(ctx, laneMove(f, c, 0, store.LaneTarget{Field: store.FieldAssignee, AssigneeID: &outsider.ID}))
	var re *store.RuleError
	if !errors.As(err, &re) || re.Violations[0].Code != "rules.assignee_not_member" {
		t.Fatalf("err = %v, want rules.assignee_not_member", err)
	}
}

// The index: above the card dropped on, at the end of the lane without one,
// at the end of the lane when the card named is in another lane.
func TestLaneMovePlacesByTheCardBelow(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	m1 := assign(t, f, f.card(t, 1, "M1"), &f.member.ID)
	f.card(t, 1, "U1")
	assign(t, f, f.card(t, 1, "M2"), &f.member.ID)
	f.card(t, 1, "U2")
	member := store.LaneTarget{Field: store.FieldAssignee, AssigneeID: &f.member.ID}

	a := f.card(t, 0, "A")
	above := member
	above.BeforeCardID = m1.ID
	if _, err := f.s.MoveCardWith(ctx, laneMove(f, a, 1, above)); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, 1); got != "A,M1,U1,M2,U2" {
		t.Errorf("above M1: %s", got)
	}

	b := f.card(t, 0, "B")
	if _, err := f.s.MoveCardWith(ctx, laneMove(f, b, 1, member)); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, 1); got != "A,M1,U1,M2,B,U2" {
		t.Errorf("end of lane: %s", got)
	}

	cc := f.card(t, 0, "C")
	wrong := member
	u1 := findCard(t, f, "U1")
	wrong.BeforeCardID = u1.ID // in the unassigned lane
	if _, err := f.s.MoveCardWith(ctx, laneMove(f, cc, 1, wrong)); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, 1); got != "A,M1,U1,M2,B,C,U2" {
		t.Errorf("another lane's card: %s", got)
	}

	// A lane with no card in the column: the bottom of the column.
	d := f.card(t, 0, "D")
	urgent := int16(4)
	if _, err := f.s.MoveCardWith(ctx, laneMove(f, d, 1, store.LaneTarget{Field: store.FieldPriority, Priority: &urgent})); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, 1); got != "A,M1,U1,M2,B,C,U2,D" {
		t.Errorf("empty cell: %s", got)
	}
}

func TestLaneMoveStaleIsAConflict(t *testing.T) {
	f := newBoardFixture(t)
	c := f.card(t, 0, "A")
	m := laneMove(f, c, 1, store.LaneTarget{Field: store.FieldPriority})
	m.ExpectedVersion--
	if _, err := f.s.MoveCardWith(context.Background(), m); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func findCard(t *testing.T, f boardFixture, title string) store.Card {
	t.Helper()
	cards, err := f.s.BoardCards(context.Background(), f.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cards {
		if c.Card.Title == title {
			return c.Card
		}
	}
	t.Fatalf("no card %q", title)
	return store.Card{}
}
