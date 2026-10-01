package web_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"kanban/internal/webtest"
)

// Without a script, a destructive action asks first, on a page of its own
// that posts the same form again with confirm=1.
func TestDestructiveActionsAskFirst(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Keep me")
	res := b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"archive"}})
	if res.Status != http.StatusOK {
		t.Fatalf("archive without confirm = %d, want the confirmation page", res.Status)
	}
	mustContain(t, res.Body, "Kart arşivlensin mi?", `name="confirm" value="1"`, `name="op" value="archive"`, `action="`+b.cardPath(c)+`"`)
	if got, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID); got.ArchivedAt != nil {
		t.Fatal("archived without confirmation")
	}
	// Posting the page's form archives.
	form := url.Values{"op": {"archive"}, "confirm": {"1"}, "_csrf": {webtest.CSRFToken(t, res.Body)}}
	if res := b.member.Post(b.cardPath(c), form); res.Status != http.StatusSeeOther {
		t.Fatalf("confirmed archive = %d", res.Status)
	}
	if got, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID); got.ArchivedAt == nil {
		t.Fatal("not archived after confirmation")
	}
}

