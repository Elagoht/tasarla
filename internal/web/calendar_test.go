package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kanban/internal/store"
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
