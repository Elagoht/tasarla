package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func datesForm(version int, start, due string) url.Values {
	return url.Values{"op": {"set_dates"}, "start": {start}, "due": {due}, "expected_version": {strconv.Itoa(version)}}
}

func TestTheGanttChart(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	bar := b.card(t, 0, "Design")
	pin := b.card(t, 1, "Launch")
	b.card(t, 0, "Someday")
	if res := b.member.SubmitFetch(b.cardPath(bar), b.cardPath(bar), datesForm(bar.Version, "2026-10-05", "2026-10-09")); res.Status != http.StatusNoContent {
		t.Fatalf("set_dates = %d", res.Status)
	}
	if res := b.member.SubmitFetch(b.cardPath(pin), b.cardPath(pin), fieldForm(pin, "due_date", "2026-10-08")); res.Status != http.StatusOK {
		t.Fatalf("due date = %d", res.Status)
	}
	if err := b.h.store.AddDependency(ctx, b.board.ID, bar.ID, pin.ID); err != nil {
		t.Fatal(err)
	}
	mustContain(t, b.member.Get(b.path).Body, `href="`+b.path+`/gantt"`)
	page := b.member.Get(b.path + "/gantt?scale=day")
	if page.Status != http.StatusOK {
		t.Fatalf("gantt = %d", page.Status)
	}
	mustContain(t, page.Body,
		`data-card="`+id(bar.ID)+`"`, `data-start="2026-10-05" data-due="2026-10-09"`, // a bar
		`class="gantt__bar is-milestone"`,         // the due date alone: a milestone
		`class="gantt__arrow gantt__arrow--late"`, // Launch is due before Design ends
		"Tarihsiz kartlar", "Someday", `data-collage-fragment="`+b.path+`/gantt/chart?scale=day&amp;group=column"`)
	// The chart's own URL, as pushes fetch it.
	if frag := b.member.Get(b.path + "/gantt/chart?scale=week&group=assignee"); frag.Status != http.StatusOK || strings.Contains(frag.Body, "<html") {
		t.Fatalf("chart fragment = %d", frag.Status)
	}
	out := b.h.signedIn("out", "out@example.com")
	if res := out.Get(b.path + "/gantt"); res.Status != http.StatusNotFound {
		t.Errorf("outsider gantt = %d", res.Status)
	}
}

func TestSettingDatesFromTheChart(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), datesForm(c.Version, "2026-10-09", "2026-10-05")); res.Status != http.StatusUnprocessableEntity {
		t.Errorf("start after due = %d, want 422", res.Status)
	}
	if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), datesForm(c.Version+5, "2026-10-01", "2026-10-02")); res.Status != http.StatusConflict {
		t.Errorf("stale = %d, want 409", res.Status)
	}
	if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), datesForm(c.Version, "x", "")); res.Status != http.StatusBadRequest {
		t.Errorf("bad date = %d, want 400", res.Status)
	}
	if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), datesForm(c.Version, "2026-10-01", "")); res.Status != http.StatusNoContent {
		t.Fatalf("start only = %d", res.Status)
	}
	got, _ := b.h.store.Card(ctx, b.board.ID, c.ID)
	if got.StartDate == nil || got.StartDate.Format(time.DateOnly) != "2026-10-01" || got.DueDate != nil {
		t.Fatalf("card = %+v", got)
	}
	// The panel shows and saves the start date; history names it.
	page := b.member.Get(b.cardPath(c)).Body
	mustContain(t, page, "Başlangıç", `value="start_date"`, `value="2026-10-01"`, "başlangıç")
	res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(got, "due_date", "2026-09-01"))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("due before start = %d", res.Status)
	}
	mustContain(t, res.Body, "Başlangıç tarihi son tarihten sonra olamaz.")
}
