package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestCardDescriptionIsMarkdown(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	cur, _ := b.h.store.Card(t.Context(), b.board.ID, c.ID)
	res := b.member.Submit(b.cardPath(c), b.cardPath(c), fieldForm(cur, "description", "# Plan\n- [x] **yapıldı**\nbkz. https://x.io"))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("set description = %d:\n%s", res.Status, res.Body)
	}
	panel := b.member.Get(b.cardPath(c) + "/panel").Body
	mustContain(t, panel, "<h2>Plan</h2>", "<strong>yapıldı</strong>", `type="checkbox"`,
		`href="https://x.io" rel="nofollow noreferrer"`, `class="md-help"`, "Markdown desteklenir")
}

func TestCommentIsMarkdownWithoutRawHTML(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	res := b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_add"}, "comment": {"@member <script>x</script>"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("comment = %d:\n%s", res.Status, res.Body)
	}
	panel := b.member.Get(b.cardPath(c) + "/panel").Body
	body := panel[strings.Index(panel, "comment__body"):]
	body = body[:strings.Index(body, "</div>")]
	if strings.Contains(body, "<script") || strings.Contains(body, "&lt;script") {
		t.Errorf("raw HTML shown in the comment:\n%s", body)
	}
	mustContain(t, body, "@member")
	mustContain(t, panel, `class="md-help"`)
}
