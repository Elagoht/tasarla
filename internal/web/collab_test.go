package web_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestCommentsAndMentions(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	res := b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_add"}, "comment": {"Please review @lead\nsee https://example.com/x?a=1&b=<2>"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("comment = %d:\n%s", res.Status, res.Body)
	}
	page := b.member.Get(b.cardPath(c)).Body
	mustContain(t, page, "Please review @lead<br>", `<a href="https://example.com/x?a=1&amp;b=" rel="noopener noreferrer nofollow">https://example.com/x?a=1&amp;b=</a>&lt;2&gt;`,
		`<option value="@lead">`)
	comments, _ := b.h.store.CardComments(ctx, c.ID)
	if len(comments) != 1 || len(comments[0].MentionIDs) != 1 || comments[0].MentionIDs[0] != b.h.user("lead@example.com").ID {
		t.Fatalf("comments = %+v", comments)
	}
	id0 := id(comments[0].ID)
	if res := b.lead.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_edit"}, "comment_id": {id0}, "comment_body": {"mine now"}}); res.Status != http.StatusNotFound {
		t.Errorf("editing someone else's comment = %d, want 404", res.Status)
	}
	if res := b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_edit"}, "comment_id": {id0}, "comment_body": {"Edited text"}}); res.Status != http.StatusSeeOther {
		t.Fatalf("edit = %d", res.Status)
	}
	mustContain(t, b.member.Get(b.cardPath(c)).Body, "Edited text", "düzenlendi")
	if res := b.lead.Submit(b.cardPath(c), b.cardPath(c), url.Values{"confirm": {"1"}, "op": {"comment_delete"}, "comment_id": {id0}}); res.Status != http.StatusSeeOther {
		t.Fatalf("a lead deleting = %d", res.Status)
	}
	page = b.member.Get(b.cardPath(c)).Body
	mustContain(t, page, "Bu yorum silindi.")
	if strings.Contains(page, "Edited text") {
		t.Error("a deleted comment's text is shown")
	}
}

func TestAttachments(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, 64)...)
	res := b.member.Upload(b.cardPath(c), b.cardPath(c), url.Values{"op": {"attachment_add"}}, "file", "shot.png", png)
	if res.Status != http.StatusSeeOther {
		t.Fatalf("upload = %d:\n%s", res.Status, res.Body)
	}
	page := b.member.Get(b.cardPath(c)).Body
	link := regexp.MustCompile(`href="(/files/\d+)"[^>]*>shot\.png`).FindStringSubmatch(page)
	if link == nil {
		t.Fatalf("no link to the attachment:\n%s", page)
	}
	file := b.member.Get(link[1])
	if file.Status != http.StatusOK || file.Body != string(png) || file.Header.Get("Content-Type") != "image/png" ||
		!strings.HasPrefix(file.Header.Get("Content-Disposition"), "inline") || file.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download = %d %v", file.Status, file.Header)
	}

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	b.member.Upload(b.cardPath(c), b.cardPath(c), url.Values{"op": {"attachment_add"}}, "file", "evil.svg", svg)
	page = b.member.Get(b.cardPath(c)).Body
	svgLink := regexp.MustCompile(`href="(/files/\d+)"[^>]*>evil\.svg`).FindStringSubmatch(page)
	if svgLink == nil {
		t.Fatal("no link to the svg")
	}
	got := b.member.Get(svgLink[1])
	if !strings.HasPrefix(got.Header.Get("Content-Disposition"), "attachment") || strings.Contains(got.Header.Get("Content-Type"), "svg") {
		t.Fatalf("svg served as %q, %q", got.Header.Get("Content-Type"), got.Header.Get("Content-Disposition"))
	}

	// Another team's member gets 404 for the same URL, and so does a missing id.
	out := b.h.signedIn("out", "out@example.com")
	for _, p := range []string{link[1], "/files/999999", "/files/abc"} {
		if res := out.Get(p); res.Status != http.StatusNotFound {
			t.Errorf("GET %s by an outsider = %d, want 404", p, res.Status)
		}
	}
	anon := b.h.browser()
	if res := anon.Get(link[1]); res.Status != http.StatusNotFound && res.Status != http.StatusSeeOther {
		t.Errorf("anonymous download = %d", res.Status)
	}
}

func TestAttachmentsOverFiveMegabytesAreRefusedOnTheForm(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	big := bytes.Repeat([]byte("x"), 5<<20+1)
	res := b.member.Upload(b.cardPath(c), b.cardPath(c), url.Values{"op": {"attachment_add"}}, "file", "big.bin", big)
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("upload = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Dosya 5 MB&#39;ı aşıyor.")
	exact := bytes.Repeat([]byte("x"), 5<<20)
	if res := b.member.Upload(b.cardPath(c), b.cardPath(c), url.Values{"op": {"attachment_add"}}, "file", "max.bin", exact); res.Status != http.StatusSeeOther {
		t.Fatalf("a 5 MB file = %d, want 303", res.Status)
	}
}

func TestActivityShowsWhoDidWhatAndWhen(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	b.lead.SubmitFetch(b.path, b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	moved, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID)
	b.member.Submit(b.cardPath(c), b.cardPath(c), fieldForm(moved, "title", "Renamed"))

	page := b.member.Get(b.cardPath(c)).Body
	mustContain(t, page, "Lead kartı Todo kolonundan Doing kolonuna taşıdı", "Member kartı düzenledi",
		`<del class="changes__old">Card</del>`, `<ins class="changes__new">Renamed</ins>`, `datetime="`)
	board := b.member.Get(b.path + "/activity")
	if board.Status != http.StatusOK {
		t.Fatalf("board activity = %d", board.Status)
	}
	mustContain(t, board.Body, "Renamed", "Lead kartı Todo kolonundan Doing kolonuna taşıdı")
	mustContain(t, b.member.Get(b.path).Body, `href="`+b.path+`/activity"`)
	if res := b.h.signedIn("out", "out@example.com").Get(b.path + "/activity"); res.Status != http.StatusNotFound {
		t.Errorf("outsider board activity = %d", res.Status)
	}
}
