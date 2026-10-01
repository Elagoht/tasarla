package web_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestSearchPage(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Rapor <b>kalın</b>")
	if err := b.h.store.ArchiveCard(context.Background(), b.board.ID, c.ID, b.h.user("lead@example.com").ID); err != nil {
		t.Fatal(err)
	}
	page := b.member.Get("/search?q=rapor")
	if page.Status != http.StatusOK {
		t.Fatalf("search = %d", page.Status)
	}
	mustContain(t, page.Body, "<mark>Rapor</mark>", "&lt;b&gt;kalın&lt;/b&gt;", "Sprint", "Arşivde",
		`href="`+b.cardPath(c)+`"`)
	if strings.Contains(page.Body, "<b>kalın</b>") {
		t.Error("a title's markup reached the page")
	}
	if empty := b.member.Get("/search?q=r"); empty.Status != http.StatusOK || strings.Contains(empty.Body, "<mark>") {
		t.Errorf("one letter = %d", empty.Status)
	}
	b.h.speaks("member@example.com", "en")
	mustContain(t, b.member.Get("/en/search?q=rapor").Body, "Archived")
}

func TestSearchPageIsPrivate(t *testing.T) {
	b := newBoardSetup(t)
	if r := b.h.browser().Get("/search?q=rapor"); r.Status != http.StatusSeeOther {
		t.Fatalf("signed out = %d, want a redirect to sign in", r.Status)
	}
}
