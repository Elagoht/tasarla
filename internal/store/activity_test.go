package store_test

import (
	"context"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

func kinds(as []store.Activity) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.Kind)
	}
	return out
}

func TestActivityRecordsWhoDidWhatAndWhen(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	est := 2.0
	a, err := f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A2", Estimate: &est}, f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Saving with nothing changed is not a change.
	a, _ = f.s.UpdateCard(ctx, f.board.ID, a.ID, a.Version, store.CardFields{Title: "A2", Estimate: &est}, f.member.ID)
	member := rules.Actor{UserID: f.member.ID, TeamRole: rules.RoleMember}
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[1].ID, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version, Actor: member}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.LogActivity(ctx, f.board.ID, &a.ID, f.member.ID, store.ActivityChecklistAdded, store.ActivityPayload{Text: "item"}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ArchiveCard(ctx, f.board.ID, a.ID, f.lead.ID); err != nil {
		t.Fatal(err)
	}

	got, err := f.s.CardActivity(ctx, a.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"card_archived", "checklist_added", "card_moved", "card_updated", "card_created"}
	if join(kinds(got)) != join(want) {
		t.Fatalf("kinds = %v, want %v", kinds(got), want)
	}
	moved, updated := got[2], got[3]
	if moved.ActorName != "User member" || moved.Payload.From != "Todo" || moved.Payload.To != "Doing" || moved.CreatedAt.IsZero() {
		t.Errorf("moved = %+v", moved)
	}
	if join(updated.Payload.Fields) != "title,estimate" {
		t.Errorf("updated fields = %v", updated.Payload.Fields)
	}
	if got[0].ActorName != "User lead" || got[4].ActorName != "User lead" {
		t.Errorf("actors = %q, %q", got[0].ActorName, got[4].ActorName)
	}

	board, _ := f.s.BoardActivity(ctx, f.board.ID, 50)
	if len(board) != 5 || board[0].CardTitle != "A2" {
		t.Fatalf("board activity = %+v", board)
	}
}

func TestReorderingIsNotLogged(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	f.card(t, 0, "B")
	if _, err := f.s.MoveCard(ctx, store.Move{BoardID: f.board.ID, CardID: a.ID, ToColumnID: f.cols[0].ID, ToIndex: 1, ExpectedFrom: f.cols[0].ID, ExpectedVersion: a.Version}); err != nil {
		t.Fatal(err)
	}
	got, _ := f.s.CardActivity(ctx, a.ID, 50)
	if join(kinds(got)) != "card_created" {
		t.Fatalf("kinds = %v", kinds(got))
	}
}
