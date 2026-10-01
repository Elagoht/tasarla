package store_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func sentenceKeys(t *testing.T, f boardFixture) []string {
	t.Helper()
	ss, err := f.s.BoardSentences(context.Background(), f.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, s := range ss {
		keys = append(keys, s.Key)
	}
	return keys
}

func TestSentencesGroupTheRules(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	todo, doing, done := f.cols[0].ID, f.cols[1].ID, f.cols[2].ID
	qa, _ := f.s.CreateBoardRole(ctx, f.board.ID, "QA")
	f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: done, Subject: rules.SubjectTeamLead})
	f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: done, Subject: rules.SubjectBoardRole, BoardRoleID: &qa.ID})
	f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: done, FromColumnID: &doing, Subject: rules.SubjectAssignee})
	f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: doing, Phase: rules.PhaseEnter, Kind: rules.HasEstimate})
	three := 3
	f.s.SetWIPLimit(ctx, f.board.ID, doing, &three)
	f.s.SetPersonWIP(ctx, f.board.ID, &three, []int64{doing})
	if err := f.s.AddFromSentence(ctx, f.board.ID, done, []int64{doing}); err != nil {
		t.Fatal(err)
	}
	ss, _ := f.s.BoardSentences(ctx, f.board.ID)
	byKey := map[string]store.Sentence{}
	for _, s := range ss {
		byKey[s.Key] = s
	}
	grouped := byKey["permission:"+id64(done)+":0"]
	if len(grouped.Subjects) != 1 || grouped.Subjects[0] != rules.SubjectTeamLead || len(grouped.RoleIDs) != 1 || grouped.RoleIDs[0] != qa.ID {
		t.Errorf("grouped permission = %+v", grouped)
	}
	if s := byKey["permission:"+id64(done)+":"+id64(doing)]; len(s.Subjects) != 1 || s.FromID != doing {
		t.Errorf("from-permission = %+v", s)
	}
	if s := byKey["from:"+id64(done)]; len(s.Columns) != 1 || s.Columns[0] != doing {
		t.Errorf("from sentence = %+v", s)
	}
	if s := byKey["wip:"+id64(doing)]; s.Limit != 3 {
		t.Errorf("wip = %+v", s)
	}
	if s := byKey["person_wip"]; s.Limit != 3 || len(s.Columns) != 1 {
		t.Errorf("person wip = %+v", s)
	}
	conds := 0
	for _, s := range ss {
		if s.Kind == store.SentenceCondition && s.ColumnID == doing && s.ConditionKind == rules.HasEstimate {
			conds++
		}
	}
	if conds != 1 || len(ss) != 6 {
		t.Errorf("sentences = %d (%v)", len(ss), sentenceKeys(t, f))
	}

	// A "from" sentence on one column does not lock the others: todo → doing still moves.
	a := f.card(t, 0, "A")
	est := 1.0
	a, _ = f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A", Estimate: &est}, 0)
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: doing, ExpectedFrom: todo, ExpectedVersion: a.Version}); err != nil {
		t.Fatalf("an unrelated move was locked: %v", err)
	}
}

func TestDeletingSentences(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	doing, done := f.cols[1].ID, f.cols[2].ID
	qa, _ := f.s.CreateBoardRole(ctx, f.board.ID, "QA")
	f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: done, Subject: rules.SubjectTeamLead})
	f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: done, Subject: rules.SubjectBoardRole, BoardRoleID: &qa.ID})
	f.s.AddMovePermission(ctx, f.board.ID, store.MovePermission{ToColumnID: doing, Subject: rules.SubjectTeamLead})
	if err := f.s.DeleteSentence(ctx, f.board.ID, "permission:"+id64(done)+":0"); err != nil {
		t.Fatal(err)
	}
	r, _ := f.s.BoardRules(ctx, f.board.ID)
	if len(r.Permissions) != 1 || r.Permissions[0].ToColumnID != doing {
		t.Fatalf("permissions after deleting one group = %+v", r.Permissions)
	}
	f.s.AddFromSentence(ctx, f.board.ID, done, []int64{doing})
	if b, _ := f.s.Board(ctx, f.board.ID); b.TransitionsMode != rules.ModeRestricted {
		t.Fatal("not restricted")
	}
	if err := f.s.DeleteSentence(ctx, f.board.ID, "from:"+id64(done)); err != nil {
		t.Fatal(err)
	}
	if b, _ := f.s.Board(ctx, f.board.ID); b.TransitionsMode != rules.ModeOpen {
		t.Fatal("the last from sentence left the board restricted")
	}
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	if err := f.s.DeleteSentence(ctx, other.ID, "permission:"+id64(doing)+":0"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("through another board: %v", err)
	}
	if err := f.s.DeleteSentence(ctx, f.board.ID, "nonsense"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("bad key: %v", err)
	}
}

func id64(n int64) string { return strconv.FormatInt(n, 10) }
