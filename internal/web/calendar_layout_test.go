package web

import (
	"testing"
	"time"

	"kanban/internal/store"
)

func date(s string) *time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func gc(id int64, title string, start, due string) store.GanttCard {
	c := store.Card{ID: id, ColumnID: 1, Title: title, Version: 1}
	if start != "" {
		c.StartDate = date(start)
	}
	if due != "" {
		c.DueDate = date(due)
	}
	return store.GanttCard{Card: c}
}

var calNow = time.Date(2026, 10, 4, 15, 0, 0, 0, time.UTC)

func TestCalendarMonth(t *testing.T) {
	for raw, want := range map[string]string{"2026-02": "2026-02", "": "2026-10", "2026-13": "2026-10", "abc": "2026-10", "2026-1": "2026-10"} {
		if got := calendarMonth(raw, calNow).Format("2006-01"); got != want {
			t.Errorf("calendarMonth(%q) = %s, want %s", raw, got, want)
		}
	}
}

func TestLayoutCalendarGrid(t *testing.T) {
	v := layoutCalendar(*date("2026-10-01"), nil, nil, nil, calNow)
	// October 2026: Thursday the 1st to Saturday the 31st → Mon 28 Sep … Sun 1 Nov.
	if len(v.Weeks) != 5 {
		t.Fatalf("weeks = %d, want 5", len(v.Weeks))
	}
	first, last := v.Weeks[0].Days[0], v.Weeks[4].Days[6]
	if first.Date != "2026-09-28" || !first.Outside || last.Date != "2026-11-01" || !last.Outside {
		t.Fatalf("grid = %s … %s", first.Date, last.Date)
	}
	if d := v.Weeks[0].Days[6]; d.Date != "2026-10-04" || !d.Today || d.Outside || d.Col != 7 {
		t.Errorf("today = %+v", d)
	}
	if v.Month != "2026-10" || v.Prev != "2026-09" || v.Next != "2026-11" {
		t.Errorf("months = %s %s %s", v.Prev, v.Month, v.Next)
	}
	// March 2026 starts on Sunday: six weeks.
	if n := len(layoutCalendar(*date("2026-03-01"), nil, nil, nil, calNow).Weeks); n != 6 {
		t.Errorf("March weeks = %d, want 6", n)
	}
}

func TestLayoutCalendarSplitsAtWeekEdges(t *testing.T) {
	v := layoutCalendar(*date("2026-10-01"), []store.GanttCard{gc(1, "Long", "2026-10-02", "2026-10-07")}, map[int64]string{1: "#1971c2"}, nil, calNow)
	a, b := v.Weeks[0].Bars, v.Weeks[1].Bars
	if len(a) != 1 || a[0].Col != 5 || a[0].Span != 3 || a[0].Before || !a[0].After || a[0].Color != "#1971c2" {
		t.Fatalf("first week = %+v", a)
	}
	if len(b) != 1 || b[0].Col != 1 || b[0].Span != 3 || !b[0].Before || b[0].After {
		t.Fatalf("second week = %+v", b)
	}
	// A card from the previous month shows in the grid's first week.
	v = layoutCalendar(*date("2026-10-01"), []store.GanttCard{gc(2, "Sept", "", "2026-09-29")}, nil, nil, calNow)
	if bars := v.Weeks[0].Bars; len(bars) != 1 || bars[0].Col != 2 || bars[0].Span != 1 {
		t.Fatalf("outside card = %+v", bars)
	}
}

func TestLayoutCalendarLanesAndOverflow(t *testing.T) {
	cards := []store.GanttCard{
		gc(1, "A", "2026-10-05", "2026-10-07"),
		gc(2, "B", "", "2026-10-06"),
		gc(3, "C", "2026-10-08", ""), // start only: one day
		gc(4, "D", "", "2026-10-06"),
		gc(5, "E", "", "2026-10-06"),
		gc(6, "F", "", ""), // no dates: not on the calendar
	}
	v := layoutCalendar(*date("2026-10-01"), cards, nil, nil, calNow)
	w := v.Weeks[1] // 5–11 October
	lane := map[int64]int{}
	for _, b := range w.Bars {
		lane[b.CardID] = b.Lane
	}
	// A is longest and earliest: lane 1; C fits beside it on the 8th.
	if lane[1] != 1 || lane[3] != 1 || lane[2] != 2 || lane[4] != 3 {
		t.Fatalf("lanes = %v", lane)
	}
	if _, shown := lane[5]; shown {
		t.Error("the fourth card on the 6th is drawn")
	}
	tue := w.Days[1] // 6 October
	if tue.More != 1 || len(tue.Cards) != 4 {
		t.Fatalf("6 Oct: more %d, cards %d", tue.More, len(tue.Cards))
	}
	for _, wk := range v.Weeks {
		for _, b := range wk.Bars {
			if b.CardID == 6 {
				t.Error("an undated card is on the calendar")
			}
		}
	}
}

func TestLayoutCalendarMarksLateDoneAndDimmed(t *testing.T) {
	done := gc(2, "Done", "", "2026-10-01")
	done.Card.CompletedAt = date("2026-10-02")
	cards := []store.GanttCard{gc(1, "Late", "", "2026-10-02"), done, gc(3, "Future", "", "2026-10-20")}
	v := layoutCalendar(*date("2026-10-01"), cards, nil, map[int64]bool{1: true, 2: true}, calNow)
	got := map[int64]calendarBar{}
	for _, w := range v.Weeks {
		for _, b := range w.Bars {
			got[b.CardID] = b
		}
	}
	if !got[1].Late || got[1].Done || got[1].Dimmed {
		t.Errorf("late card = %+v", got[1])
	}
	if got[2].Late || !got[2].Done {
		t.Errorf("done card = %+v", got[2])
	}
	if got[3].Late || !got[3].Dimmed || got[3].Due != "2026-10-20" {
		t.Errorf("filtered-out card = %+v", got[3])
	}
}
