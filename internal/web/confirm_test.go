package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
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

// Review: the confirm page goes back only to a page of this site.
func TestTheConfirmPageGoesBackOnlyToThisSite(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	page := b.member.Get(b.cardPath(c))
	form := url.Values{"op": {"archive"}, "_csrf": {webtest.CSRFToken(t, page.Body)}}
	for _, ref := range []string{"//evil.example/x", "/\\evil.example/x"} {
		res := b.member.PostWith(b.cardPath(c), form, http.Header{"Referer": {ref}})
		if res.Status != http.StatusOK || strings.Contains(res.Body, "evil.example") {
			t.Errorf("Referer %q: %d, back link kept it", ref, res.Status)
		}
	}
	res := b.member.PostWith(b.cardPath(c), form, http.Header{"Referer": {webtest.Origin + b.path}})
	mustContain(t, res.Body, `href="`+webtest.Origin+b.path+`"`)
}
