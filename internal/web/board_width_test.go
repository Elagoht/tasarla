package web_test

import (
	"net/http"
	"strings"
	"testing"
)

// The board keeps the reading width unless its viewer widened it; the choice
// comes back in a cookie, so the page is drawn at the right width at once.
func TestBoardWidthFollowsTheCookie(t *testing.T) {
	b := newBoardSetup(t)
	page := b.member.Get(b.path)
	if page.Status != http.StatusOK {
		t.Fatalf("board = %d", page.Status)
	}
	mustContain(t, page.Body, `class="main main--wide" id="main"`, `data-board-width aria-pressed="false" hidden`)
	b.member.SetCookie("board_wide", "1")
	wide := b.member.Get(b.path).Body
	mustContain(t, wide, `class="main main--wide is-expanded" id="main"`, `data-board-width aria-pressed="true" hidden`)
	b.member.SetCookie("board_wide", "yes")
	if strings.Contains(b.member.Get(b.path).Body, "is-expanded") {
		t.Error("a cookie other than 1 widens the board")
	}
}
