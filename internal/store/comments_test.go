package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/store"
)

func TestResolveMentions(t *testing.T) {
	members := []store.Member{
		{User: store.User{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com"}},
		{User: store.User{ID: 2, Name: "Grace", Email: "grace.hopper@example.com"}},
	}
	got := store.ResolveMentions("Hi @ada and @Grace.Hopper, also @AdaLovelace and @nobody, mail me at x@ada", members)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("mentions = %v, want [1 2]", got)
	}
	if got := store.ResolveMentions("no mentions here", members); len(got) != 0 {
		t.Fatalf("mentions = %v", got)
	}
}

func TestComments(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	c, err := f.s.AddComment(ctx, f.board.ID, a.ID, f.member.ID, "Looks good @lead", []int64{f.lead.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.AddComment(ctx, f.board.ID, a.ID, f.lead.ID, "Thanks", nil); err != nil {
		t.Fatal(err)
	}
	list, err := f.s.CardComments(ctx, a.ID)
	if err != nil || len(list) != 2 || list[0].AuthorName != "User member" || list[0].Body != "Looks good @lead" || len(list[0].MentionIDs) != 1 {
		t.Fatalf("comments = %+v, %v", list, err)
	}
	if err := f.s.EditComment(ctx, f.board.ID, a.ID, c.ID, f.lead.ID, "hijack", nil); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("editing someone else's comment: %v", err)
	}
	if err := f.s.EditComment(ctx, f.board.ID, a.ID, c.ID, f.member.ID, "Edited", nil); err != nil {
		t.Fatal(err)
	}
	list, _ = f.s.CardComments(ctx, a.ID)
	if list[0].Body != "Edited" || list[0].EditedAt == nil || len(list[0].MentionIDs) != 0 {
		t.Errorf("edited = %+v", list[0])
	}
	if err := f.s.DeleteComment(ctx, f.board.ID, a.ID, c.ID, f.lead.ID, false); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleting someone else's comment without managing the board: %v", err)
	}
	if err := f.s.DeleteComment(ctx, f.board.ID, a.ID, c.ID, f.lead.ID, true); err != nil {
		t.Fatalf("a manager deleting: %v", err)
	}
	list, _ = f.s.CardComments(ctx, a.ID)
	if list[0].DeletedAt == nil || list[0].Body != "" {
		t.Errorf("deleted = %+v", list[0])
	}
	other, _ := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"X"})
	if _, err := f.s.AddComment(ctx, other.ID, a.ID, f.member.ID, "x", nil); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("commenting through another board: %v", err)
	}
	act, _ := f.s.CardActivity(ctx, a.ID, 10)
	if act[0].Kind != store.ActivityCommentAdded {
		t.Errorf("activity = %v", kinds(act))
	}
}
