package web_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTheCalendarView(t *testing.T) {
	b := newBoardSetup(t)
	long := b.card(t, 0, "Design")
	pin := b.card(t, 1, "Launch")
	b.card(t, 0, "Someday")
	if res := b.member.SubmitFetch(b.cardPath(long), b.cardPath(long), datesForm(long.Version, "2026-10-05", "2026-10-09")); res.Status != http.StatusNoContent {
		t.Fatalf("set_dates = %d", res.Status)
	}
	if res := b.member.SubmitFetch(b.cardPath(pin), b.cardPath(pin), fieldForm(pin, "due_date", "2026-10-08")); res.Status != http.StatusOK {
		t.Fatalf("due date = %d", res.Status)
	}
	mustContain(t, b.member.Get(b.path).Body, `href="`+b.path+`/calendar"`)
	page := b.member.Get(b.path + "/calendar?month=2026-10")
	if page.Status != http.StatusOK {
		t.Fatalf("calendar = %d", page.Status)
	}
	mustContain(t, page.Body,
		"Ekim 2026", "Pzt", `data-date="2026-10-05"`,
		`data-card="`+id(long.ID)+`"`, `data-start="2026-10-05" data-due="2026-10-09"`,
		`data-card="`+id(pin.ID)+`"`,
		`href="`+b.path+`/calendar?month=2026-09"`, `href="`+b.path+`/calendar?month=2026-11"`,
		`data-collage-fragment="`+b.path+`/calendar/grid?month=2026-10"`)
	if strings.Contains(page.Body, "Someday") {
		t.Error("an undated card is on the calendar")
	}
	if strings.Contains(page.Body, `style="`) {
		t.Error("inline style: the CSP blocks it")
	}
	if frag := b.member.Get(b.path + "/calendar/grid?month=2026-10"); frag.Status != http.StatusOK || strings.Contains(frag.Body, "<html") {
		t.Fatalf("grid fragment = %d", frag.Status)
	}
	out := b.h.signedIn("out", "out@example.com")
	if res := out.Get(b.path + "/calendar"); res.Status != http.StatusNotFound {
		t.Errorf("outsider calendar = %d", res.Status)
	}
}

func TestCalendarFallsBackToThisMonth(t *testing.T) {
	b := newBoardSetup(t)
	this := time.Now().Format("2006-01")
	for _, q := range []string{"", "?month=2026-13", "?month=abc"} {
		res := b.member.Get(b.path + "/calendar" + q)
		if res.Status != http.StatusOK {
			t.Fatalf("calendar%s = %d", q, res.Status)
		}
		mustContain(t, res.Body, `/calendar/grid?month=`+this)
	}
}

func TestCalendarShowsDoneCardsOnRequest(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 2, "Shipped") // the third column is the done column
	if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "due_date", "2026-10-08")); res.Status != http.StatusOK {
		t.Fatalf("due date = %d", res.Status)
	}
	if strings.Contains(b.member.Get(b.path+"/calendar?month=2026-10").Body, "Shipped") {
		t.Error("a done card shows without done=1")
	}
	mustContain(t, b.member.Get(b.path+"/calendar?month=2026-10&done=1").Body, "Shipped", "is-done")
}

func TestCalendarKeepsTheFilterAcrossMonths(t *testing.T) {
	b := newBoardSetup(t)
	page := b.member.Get(b.path + "/calendar?month=2026-10&q=design").Body
	mustContain(t, page, `/calendar?month=2026-11&amp;q=design"`, `name="month" value="2026-10"`)
}
