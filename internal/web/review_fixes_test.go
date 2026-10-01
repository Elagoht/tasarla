package web_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"kanban/internal/rules"
	"kanban/internal/store"
)

// Any file over 5 MB is refused on the form, not with a bare 413 (§9, §14).
func TestAFileWellOverTheLimitIsRefusedOnTheForm(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	res := b.member.Upload(b.cardPath(c), b.cardPath(c), url.Values{"op": {"attachment_add"}}, "file", "six.bin", bytes.Repeat([]byte("x"), 6<<20))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("6 MB upload = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Dosya 5 MB&#39;ı aşıyor.")
}

func TestMentioningSomeoneInAnEditNotifiesThem(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	b.lead.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_add"}, "comment": {"draft"}})
	comments, _ := b.h.store.CardComments(ctx, c.ID)
	b.lead.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_edit"}, "comment_id": {id(comments[0].ID)}, "comment_body": {"now with @member"}})
	mustContain(t, b.member.Get("/notifications").Body, "Lead sizi bir yorumda andı: Card")
	// Saving the same text again tells nobody twice.
	b.lead.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_edit"}, "comment_id": {id(comments[0].ID)}, "comment_body": {"now with @member!"}})
	if n, _ := b.h.store.UnreadCount(ctx, b.h.user("member@example.com").ID); n != 1 {
		t.Fatalf("unread = %d, want 1", n)
	}
}

func TestUnblockedIsSaidOnce(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	blocker, blocked := b.card(t, 0, "Blocker"), b.card(t, 0, "Blocked")
	memberID := b.h.user("member@example.com").ID
	b.h.store.UpdateCard(ctx, b.board.ID, blocked.ID, blocked.Version, store.CardFields{Title: "Blocked", AssigneeID: &memberID}, 0)
	b.h.store.AddDependency(ctx, b.board.ID, blocker.ID, blocked.ID)
	move := func(to, from int64) {
		cur, _ := b.h.store.Card(ctx, b.board.ID, blocker.ID)
		if res := b.lead.SubmitFetch(b.path, b.path, moveForm(cur, to, 0, from, cur.Version)); res.Status != http.StatusOK {
			t.Fatalf("move = %d", res.Status)
		}
	}
	move(b.cols[2].ID, b.cols[0].ID)
	move(b.cols[1].ID, b.cols[2].ID)
	move(b.cols[2].ID, b.cols[1].ID)
	cur, _ := b.h.store.Card(ctx, b.board.ID, blocker.ID)
	b.lead.Submit(b.cardPath(cur), b.cardPath(cur), url.Values{"confirm": {"1"}, "op": {"archive"}})
	if n, _ := b.h.store.UnreadCount(ctx, memberID); n != 1 {
		t.Fatalf("unblocked notifications = %d, want 1", n)
	}
}

func TestDeletingARoleInUseSaysWhy(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	qa, _ := b.h.store.CreateBoardRole(ctx, b.board.ID, "QA")
	b.h.store.AddMovePermission(ctx, b.board.ID, store.MovePermission{ToColumnID: b.cols[2].ID, Subject: rules.SubjectBoardRole, BoardRoleID: &qa.ID})
	s := b.path + "/settings"
	if res := b.lead.Submit(s, s, url.Values{"confirm": {"1"}, "op": {"role_delete"}, "role_id": {id(qa.ID)}}); res.Status != http.StatusSeeOther {
		t.Fatalf("role_delete = %d", res.Status)
	}
	page := b.lead.Get(s).Body
	mustContain(t, page, "Bu rolü kullanan bir yetki var; önce yetkiyi silin.")
	if !strings.Contains(page, "QA") {
		t.Fatal("the role was deleted")
	}
}
