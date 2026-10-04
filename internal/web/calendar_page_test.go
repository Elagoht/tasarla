package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"kanban/internal/rules"
	"kanban/internal/store"
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
		`draggable="false"`, // a link's own drag would cancel the pointer drag
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
func calendarCreate(title, due string) url.Values {
	return url.Values{"op": {"create_card"}, "title": {title}, "day": {due}}
}

func TestCreatingACardFromTheCalendar(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	page := b.path + "/calendar?month=2026-10"
	res := b.member.SubmitFetch(page, page, calendarCreate("Kickoff", "2026-10-14"))
	if res.Status != http.StatusOK {
		t.Fatalf("create = %d:\n%s", res.Status, res.Body)
	}
	mustContain(t, res.Body, "Kickoff", `data-due="2026-10-14"`)
	cards, err := b.h.store.GanttCards(ctx, b.board.ID, false)
	if err != nil || len(cards) != 1 {
		t.Fatalf("cards = %v, %v", cards, err)
	}
	if c := cards[0].Card; c.ColumnID != b.cols[0].ID || c.DueDate == nil || c.DueDate.Format(time.DateOnly) != "2026-10-14" {
		t.Fatalf("card = %+v", c)
	}
	// Without a script: back to the same month.
	res = b.member.Submit(page, page, calendarCreate("Plain", "2026-10-15"))
	if res.Status != http.StatusSeeOther || !strings.Contains(res.Location(), "month=2026-10") {
		t.Fatalf("form create = %d %q", res.Status, res.Location())
	}
}

func TestCalendarCreateRefusals(t *testing.T) {
	b := newBoardSetup(t)
	page := b.path + "/calendar?month=2026-10"
	if res := b.member.SubmitFetch(page, page, calendarCreate("  ", "2026-10-14")); res.Status != http.StatusUnprocessableEntity {
		t.Errorf("empty title = %d, want 422", res.Status)
	}
	if res := b.member.SubmitFetch(page, page, calendarCreate("X", "14.10.2026")); res.Status != http.StatusBadRequest {
		t.Errorf("bad date = %d, want 400", res.Status)
	}
	if res := b.member.SubmitFetch(page, page, url.Values{"op": {"nope"}}); res.Status != http.StatusBadRequest {
		t.Errorf("unknown op = %d, want 400", res.Status)
	}
}

// The first column wants a due date: the calendar's card has one, so it enters.
func TestCalendarCreatePassesTheDueDateRule(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	if err := b.h.store.AddCondition(ctx, b.board.ID, store.ColumnCondition{ColumnID: b.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	page := b.path + "/calendar?month=2026-10"
	if res := b.member.SubmitFetch(page, page, calendarCreate("Dated", "2026-10-14")); res.Status != http.StatusOK {
		t.Fatalf("create = %d:\n%s", res.Status, res.Body)
	}
	// The column is now full: the next card is refused, with the reason on the grid.
	one := 1
	todo := b.cols[0]
	if err := b.h.store.UpdateColumn(ctx, b.board.ID, todo.ID, store.ColumnUpdate{Name: todo.Name, WIPLimit: &one, IsDone: todo.IsDone, AllowCreate: todo.AllowCreate, CountsPersonWIP: todo.CountsPersonWIP}); err != nil {
		t.Fatal(err)
	}
	res := b.member.SubmitFetch(page, page, calendarCreate("Over", "2026-10-15"))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("over the limit = %d", res.Status)
	}
	mustContain(t, res.Body, "data-cal-alert")
}

// The page's own query rides on the form's action, and a script sends the form
// as multipart, whose URL values come first: the Due filter's "due" must not
// be read as the new card's day.
func TestCalendarCreateUnderADueFilter(t *testing.T) {
	b := newBoardSetup(t)
	page := b.path + "/calendar?month=2026-10&due=overdue"
	res := b.member.Upload(page, page, calendarCreate("Filtered", "2026-10-14"), "unused", "x.txt", []byte("x"))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("create under a due filter = %d", res.Status)
	}
	cards, err := b.h.store.GanttCards(context.Background(), b.board.ID, false)
	if err != nil || len(cards) != 1 || cards[0].Card.DueDate == nil || cards[0].Card.DueDate.Format(time.DateOnly) != "2026-10-14" {
		t.Fatalf("cards = %+v, %v", cards, err)
	}
}

// The product is Pusula, with a compass for its mark, in both languages.
func TestTheBrandIsPusula(t *testing.T) {
	b := newBoardSetup(t)
	page := b.member.Get(b.path).Body
	mustContain(t, page, `<span class="brand__name">Pusula</span>`, compassNeedle)
	if strings.Contains(page, "Kanban") {
		t.Error("the old name is still on the page")
	}
	missing := b.member.Get("/nope").Body
	mustContain(t, missing, "Pusula", compassNeedle)
}

// compassNeedle is the north half of the compass icon's needle.
const compassNeedle = `<path d="M12 5.5 14.5 12h-5z" fill="currentColor"/>`
