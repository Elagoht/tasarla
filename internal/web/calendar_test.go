package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"kanban/internal/store"
	"kanban/internal/webtest"
)

// feed GETs a calendar path with no session at all, as a calendar app would.
func feed(t *testing.T, b boardSetup, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	b.h.app.Handler().ServeHTTP(rec, req)
	return rec
}

func datedCard(t *testing.T, b boardSetup, title string, assignee int64) store.Card {
	t.Helper()
	ctx := context.Background()
	lead := b.h.user("lead@example.com").ID
	due := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	c := b.card(t, 0, title)
	c, err := b.h.store.UpdateCardField(ctx, b.board.ID, c.ID, c.Version, store.FieldDueDate, store.CardFields{DueDate: &due}, lead)
	if err == nil {
		c, err = b.h.store.UpdateCardField(ctx, b.board.ID, c.ID, c.Version, store.FieldAssignee, store.CardFields{AssigneeID: &assignee}, lead)
	}
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCalendarFeeds(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	member := b.h.user("member@example.com").ID
	c := datedCard(t, b, "Rapor, taslak", member)
	token, err := b.h.store.NewCalendarToken(ctx, member)
	if err != nil {
		t.Fatal(err)
	}
	me := feed(t, b, "/cal/"+token+"/me.ics", nil)
	if me.Code != http.StatusOK || me.Header().Get("Content-Type") != "text/calendar; charset=utf-8" {
		t.Fatalf("me.ics = %d %q", me.Code, me.Header().Get("Content-Type"))
	}
	body := me.Body.String()
	for _, want := range []string{"BEGIN:VCALENDAR", "SUMMARY:[Sprint] Rapor\\, taslak", "DTSTART;VALUE=DATE:20261009",
		"UID:card-" + id(c.ID) + "@", "/boards/" + id(b.board.ID) + "/cards/" + id(c.ID), "X-WR-CALNAME:Kanban — bana atananlar"} {
		if !strings.Contains(body, want) {
			t.Errorf("me.ics lacks %q:\n%s", want, body)
		}
	}
	if got := feed(t, b, "/cal/"+token+"/boards/"+id(b.board.ID)+".ics", nil); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "X-WR-CALNAME:Kanban — Sprint") {
		t.Errorf("board feed = %d:\n%s", got.Code, got.Body.String())
	}

	etag := me.Header().Get("ETag")
	if again := feed(t, b, "/cal/"+token+"/me.ics", http.Header{"If-None-Match": {etag}}); etag == "" || again.Code != http.StatusNotModified {
		t.Errorf("If-None-Match %q = %d, want 304", etag, again.Code)
	}

	for _, path := range []string{"/cal/nope/me.ics", "/cal/" + token + "/boards/999999.ics", "/cal/" + token + "/other", "/cal/" + token + "/boards/x.ics"} {
		if got := feed(t, b, path, nil); got.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, got.Code)
		}
	}
	post := httptest.NewRequest(http.MethodPost, "/cal/"+token+"/me.ics", nil)
	rec := httptest.NewRecorder()
	b.h.app.Handler().ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST = %d, Allow %q", rec.Code, rec.Header().Get("Allow"))
	}

	if err := b.h.store.RemoveMember(ctx, b.team.ID, member); err != nil {
		t.Fatal(err)
	}
	if got := feed(t, b, "/cal/"+token+"/boards/"+id(b.board.ID)+".ics", nil); got.Code != http.StatusNotFound {
		t.Errorf("removed member's board feed = %d", got.Code)
	}
	if got := feed(t, b, "/cal/"+token+"/me.ics", nil); strings.Contains(got.Body.String(), "BEGIN:VEVENT") {
		t.Error("removed member's own feed keeps the board's card")
	}
}

func TestCalendarSettings(t *testing.T) {
	b := newBoardSetup(t)
	page := b.member.Get("/me/settings")
	mustContain(t, page.Body, `id="calendar"`, `value="calendar_create"`)
	res := b.member.Submit("/me/settings", "/me/settings", url.Values{"op": {"calendar_create"}})
	if res.Status != http.StatusOK {
		t.Fatalf("create = %d", res.Status)
	}
	mustContain(t, res.Body, webtest.Origin+"/cal/", "/me.ics", "/boards/"+id(b.board.ID)+".ics", "Sprint")
	token := between(res.Body, webtest.Origin+"/cal/", "/me.ics")
	if feed(t, b, "/cal/"+token+"/me.ics", nil).Code != http.StatusOK {
		t.Fatal("the shown address does not work")
	}
	again := b.member.Get("/me/settings")
	if strings.Contains(again.Body, "/cal/"+token) {
		t.Error("the address is shown again")
	}
	mustContain(t, again.Body, "boards/"+id(b.board.ID)+".ics", "Sprint")
	mustContain(t, again.Body, `value="calendar_reset"`, `value="calendar_clear"`)
	reset := b.member.Submit("/me/settings", "/me/settings", url.Values{"op": {"calendar_reset"}})
	if newToken := between(reset.Body, webtest.Origin+"/cal/", "/me.ics"); newToken == "" || newToken == token {
		t.Fatalf("reset gave %q", newToken)
	}
	if feed(t, b, "/cal/"+token+"/me.ics", nil).Code != http.StatusNotFound {
		t.Error("the old address works after a reset")
	}
	if r := b.member.Submit("/me/settings", "/me/settings", url.Values{"op": {"calendar_clear"}}); r.Status != http.StatusSeeOther {
		t.Errorf("clear = %d, want a redirect", r.Status)
	}
	mustContain(t, b.member.Get(b.path).Body, `href="/me/settings#calendar"`)
}

func TestCalendarFeedsStopWhenTheyShould(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	member := b.h.user("member@example.com").ID
	token, err := b.h.store.NewCalendarToken(ctx, member)
	if err != nil {
		t.Fatal(err)
	}
	me, board := "/cal/"+token+"/me.ics", "/cal/"+token+"/boards/"+id(b.board.ID)+".ics"
	if feed(t, b, me, nil).Code != http.StatusOK || feed(t, b, board, nil).Code != http.StatusOK {
		t.Fatal("the feeds do not work to begin with")
	}

	// A disabled user's token stops working, and works again when enabled.
	if err := b.h.store.SetDisabled(ctx, member, true); err != nil {
		t.Fatal(err)
	}
	if got := feed(t, b, me, nil); got.Code != http.StatusNotFound {
		t.Errorf("disabled user's me.ics = %d, want 404", got.Code)
	}
	if err := b.h.store.SetDisabled(ctx, member, false); err != nil {
		t.Fatal(err)
	}

	// An archived board's feed is gone.
	if err := b.h.store.ArchiveBoard(ctx, b.board.ID); err != nil {
		t.Fatal(err)
	}
	if got := feed(t, b, board, nil); got.Code != http.StatusNotFound {
		t.Errorf("archived board's feed = %d, want 404", got.Code)
	}

	// Turning the calendar off in My settings kills the old token.
	if r := b.member.Submit("/me/settings", "/me/settings", url.Values{"op": {"calendar_clear"}}); r.Status != http.StatusSeeOther {
		t.Fatalf("clear = %d", r.Status)
	}
	if got := feed(t, b, me, nil); got.Code != http.StatusNotFound {
		t.Errorf("cleared token's me.ics = %d, want 404", got.Code)
	}
}

func between(s, from, to string) string {
	_, rest, ok := strings.Cut(s, from)
	if !ok {
		return ""
	}
	v, _, _ := strings.Cut(rest, to)
	return v
}
