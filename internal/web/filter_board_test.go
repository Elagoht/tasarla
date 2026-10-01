package web_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"kanban/internal/store"
)

// The card the filter leaves out stays on the board, dimmed; the count says so.
func TestBoardFilterDimsCards(t *testing.T) {
	b := newBoardSetup(t)
	urgent := b.card(t, 0, "Urgent card")
	plain := b.card(t, 0, "Plain card")
	p := int16(4)
	if _, err := b.h.store.UpdateCardField(context.Background(), b.board.ID, urgent.ID, urgent.Version,
		store.FieldPriority, store.CardFields{Priority: &p}, b.h.user("lead@example.com").ID); err != nil {
		t.Fatal(err)
	}
	page := b.member.Get(b.path + "?priority=4")
	if page.Status != http.StatusOK {
		t.Fatalf("filtered board = %d", page.Status)
	}
	mustContain(t, page.Body,
		`class="card" id="card-`+id(urgent.ID)+`"`,
		`class="card card--dimmed" id="card-`+id(plain.ID)+`"`,
		"Eşleşen: 1 / 2",
		`data-collage-fragment="`+b.path+`/columns?priority=4"`,
		`data-move-url="`+b.path+`?priority=4"`)
	unfiltered := b.member.Get(b.path).Body
	if strings.Contains(unfiltered, "card--dimmed") || strings.Contains(unfiltered, "eşleşiyor") {
		t.Error("an unfiltered board dims cards")
	}
}

// Nonsense in the URL is dropped, not refused, and not echoed back.
func TestBoardFilterDropsBadValues(t *testing.T) {
	b := newBoardSetup(t)
	b.card(t, 0, "A card")
	page := b.member.Get(b.path + "?label=999&priority=9&assignee=abc&due=yarin&q=x")
	if page.Status != http.StatusOK {
		t.Fatalf("status = %d", page.Status)
	}
	mustContain(t, page.Body, `data-collage-fragment="`+b.path+`/columns"`)
	if strings.Contains(page.Body, "card--dimmed") {
		t.Error("dropped values still dim")
	}
}

// A move answered from a filtered board keeps the filter.
func TestMoveAnswerKeepsTheFilter(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Moved card")
	other := b.card(t, 0, "Other card")
	page := b.path + "?q=moved"
	r := b.lead.SubmitFetch(page, page, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	if r.Status != http.StatusOK {
		t.Fatalf("move = %d", r.Status)
	}
	mustContain(t, r.Body, `class="card card--dimmed" id="card-`+id(other.ID)+`"`)
}

// Two readers on the same ?assignee=me URL are each pushed their own cards.
func TestFilteredPushIsPerReader(t *testing.T) {
	b := newBoardSetup(t)
	leadsCard := b.card(t, 0, "Lead's card")
	membersCard := b.card(t, 0, "Member's card")
	ctx := context.Background()
	lead, member := b.h.user("lead@example.com"), b.h.user("member@example.com")
	for _, x := range []struct {
		c  store.Card
		to int64
	}{{leadsCard, lead.ID}, {membersCard, member.ID}} {
		if _, err := b.h.store.UpdateCardField(ctx, b.board.ID, x.c.ID, x.c.Version, store.FieldAssignee,
			store.CardFields{AssigneeID: &x.to}, lead.ID); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(b.h.app.Handler())
	t.Cleanup(server.Close)
	fragment := b.path + "/columns?assignee=me"
	first := func(cookies []*http.Cookie) string {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/_live/stream/?f="+url.QueryEscape(fragment), nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "data:") {
				return line
			}
		}
		t.Fatal("no event")
		return ""
	}
	dimmed := func(c store.Card) string { return `class=\"card card--dimmed\" id=\"card-` + id(c.ID) + `\"` }
	leads, members := first(b.lead.Cookies()), first(b.member.Cookies())
	if !strings.Contains(leads, dimmed(membersCard)) || strings.Contains(leads, dimmed(leadsCard)) {
		t.Errorf("the lead's push does not match the lead's cards:\n%s", leads)
	}
	if !strings.Contains(members, dimmed(leadsCard)) || strings.Contains(members, dimmed(membersCard)) {
		t.Errorf("the member's push does not match the member's cards:\n%s", members)
	}
}
