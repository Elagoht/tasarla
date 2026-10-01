package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func violations(t *testing.T, err error) []string {
	t.Helper()
	var re *store.RuleError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want a *store.RuleError", err)
	}
	var out []string
	for _, v := range re.Violations {
		out = append(out, v.Code)
	}
	return out
}

func TestRulesConfiguration(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	limit := 2
	if err := f.s.SetBoardPolicy(ctx, f.board.ID, rules.ModeRestricted, &limit); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetTransitions(ctx, f.board.ID, []rules.Transition{{From: f.cols[0].ID, To: f.cols[1].ID}}); err != nil {
		t.Fatal(err)
	}
	qa, err := f.s.CreateBoardRole(ctx, f.board.ID, "QA")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetBoardRoleMembers(ctx, f.board.ID, qa.ID, []int64{f.member.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: f.cols[2].ID, Subject: rules.SubjectBoardRole, BoardRoleID: &qa.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasLabel, Params: rules.ConditionParams{LabelIDs: []int64{1}}}); err != nil {
		t.Fatal(err)
	}
	r, err := f.s.BoardRules(ctx, f.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Transitions) != 1 || len(r.Roles) != 1 || len(r.Roles[0].MemberIDs) != 1 || len(r.Permissions) != 1 || len(r.Conditions) != 1 ||
		r.Conditions[0].Params.LabelIDs[0] != 1 {
		t.Fatalf("rules = %+v", r)
	}
	board, _ := f.s.Board(ctx, f.board.ID)
	if board.TransitionsMode != rules.ModeRestricted || *board.PersonWIPLimit != 2 {
		t.Errorf("board = %+v", board)
	}

	// Another board's columns and roles are refused.
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X", "Y"})
	oc, _ := f.s.Columns(ctx, other.ID)
	if err := f.s.SetTransitions(ctx, f.board.ID, []rules.Transition{{From: oc[0].ID, To: oc[1].ID}}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("foreign transition: %v", err)
	}
	if err := f.s.AddMovePermission(ctx, other.ID, store.MovePermission{ToColumnID: oc[0].ID, Subject: rules.SubjectBoardRole, BoardRoleID: &qa.ID}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("foreign role: %v", err)
	}
	if err := f.s.AddCondition(ctx, other.ID, store.ColumnCondition{ColumnID: f.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasEstimate}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("foreign column condition: %v", err)
	}
	outsider := mustUpsert(t, f.s, identity("out", "out@example.com"))
	if err := f.s.SetBoardRoleMembers(ctx, f.board.ID, qa.ID, []int64{outsider.ID}); err != nil {
		t.Fatal(err)
	}
	if r, _ := f.s.BoardRules(ctx, f.board.ID); len(r.Roles[0].MemberIDs) != 0 {
		t.Errorf("a non-member joined a board role")
	}

	if err := f.s.DeleteMovePermission(ctx, f.board.ID, r.Permissions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteCondition(ctx, f.board.ID, r.Conditions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteBoardRole(ctx, f.board.ID, qa.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMovesObeyTheRules(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasEstimate}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: f.cols[1].ID, Subject: rules.SubjectTeamLead}); err != nil {
		t.Fatal(err)
	}
	member := rules.Actor{UserID: f.member.ID, TeamRole: rules.RoleMember}
	_, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[1].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version, Actor: member})
	if got := violations(t, err); len(got) != 2 || got[0] != "rules.permission" || got[1] != "rules.condition.has_estimate" {
		t.Fatalf("violations = %v", got)
	}
	if c, _ := f.s.Card(ctx, f.board.ID, a.ID); c.ColumnID != f.cols[0].ID || c.Version != a.Version {
		t.Fatal("a refused move changed the card")
	}
	est := 1.0
	a, _ = f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A", Estimate: &est})
	lead := rules.Actor{UserID: f.lead.ID, TeamRole: rules.RoleLead}
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[1].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version, Actor: lead}); err != nil {
		t.Fatalf("an allowed move: %v", err)
	}
}

func TestSnapshotReadsTheCard(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a, blocker := f.card(t, 0, "A"), f.card(t, 0, "Blocker")
	bug, _ := f.s.CreateLabel(ctx, f.board.ID, "bug", "#e03131")
	for _, c := range []store.ColumnCondition{
		{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.HasLabel, Params: rules.ConditionParams{LabelIDs: []int64{bug.ID}}},
		{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.ChecklistComplete},
		{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.BlockersDone},
		{ColumnID: f.cols[1].ID, Phase: rules.PhaseEnter, Kind: rules.MinAttachments, Params: rules.ConditionParams{Count: 1}},
	} {
		if err := f.s.AddCondition(ctx, f.board.ID, c); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.s.AddChecklistItem(ctx, f.board.ID, a.ID, "todo"); err != nil {
		t.Fatal(err)
	}
	if err := f.s.AddDependency(ctx, f.board.ID, blocker.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	a, _ = f.s.Card(ctx, f.board.ID, a.ID)
	_, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[1].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version})
	if got := violations(t, err); len(got) != 4 {
		t.Fatalf("violations = %v", got)
	}
	var re *store.RuleError
	errors.As(err, &re)
	if re.Violations[0].Params["labels"] != "bug" {
		t.Errorf("label names = %v", re.Violations[0].Params)
	}
}

func TestCreateAndAssignObeyTheRules(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	limit := 1
	if err := f.s.UpdateColumn(ctx, f.board.ID, f.cols[0].ID, store.ColumnUpdate{Name: "Todo", WIPLimit: &limit, AllowCreate: true, CountsPersonWIP: true}); err != nil {
		t.Fatal(err)
	}
	a := f.card(t, 0, "A")
	_, err := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "B", f.lead.ID)
	if got := violations(t, err); len(got) != 1 || got[0] != "rules.wip_column" {
		t.Fatalf("create over the limit: %v", got)
	}

	if err := f.s.SetBoardPolicy(ctx, f.board.ID, rules.ModeOpen, &limit); err != nil {
		t.Fatal(err)
	}
	a, err = f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A", AssigneeID: &f.member.ID})
	if err != nil {
		t.Fatalf("first assignment: %v", err)
	}
	// Raise the column limit so a second card fits, then give it to the same person.
	limit2 := 5
	f.s.UpdateColumn(ctx, f.board.ID, f.cols[0].ID, store.ColumnUpdate{Name: "Todo", WIPLimit: &limit2, AllowCreate: true, CountsPersonWIP: true})
	b := f.card(t, 0, "B")
	_, err = f.s.UpdateCard(ctx, f.board.ID, b.ID, b.Version, store.CardFields{Title: "B", AssigneeID: &f.member.ID})
	if got := violations(t, err); len(got) != 1 || got[0] != "rules.wip_person" {
		t.Fatalf("second assignment: %v", got)
	}
	// Editing the first card again, same assignee, is not a new assignment.
	if _, err := f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A again", AssigneeID: &f.member.ID}); err != nil {
		t.Fatalf("re-saving the same assignee: %v", err)
	}
}

// Spec §5.4: with one free place left, two moves at once — exactly one wins.
func TestConcurrentMovesRespectWIP(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	limit := 1
	if err := f.s.UpdateColumn(ctx, f.board.ID, f.cols[1].ID, store.ColumnUpdate{Name: "Doing", WIPLimit: &limit}); err != nil {
		t.Fatal(err)
	}
	a, b := f.card(t, 0, "A"), f.card(t, 0, "B")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})
	for i, c := range []store.Card{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: c.ID, ToColumnID: f.cols[1].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: c.Version})
		}()
	}
	close(start)
	wg.Wait()
	ok, refused := 0, 0
	for _, err := range errs {
		var re *store.RuleError
		switch {
		case err == nil:
			ok++
		case errors.As(err, &re):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || refused != 1 {
		t.Fatalf("accepted %d, refused %d; want exactly one of each", ok, refused)
	}
}
