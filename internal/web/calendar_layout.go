package web

import (
	"slices"
	"time"

	"kanban/internal/store"
)

// calendarLanes is how many cards a day shows before "+k more".
const calendarLanes = 3

// calendarView is the board's calendar: one month, Monday to Sunday.
type calendarView struct {
	BoardID  int64
	CanEdit  bool
	Month    string // the month shown, YYYY-MM
	Prev     string // YYYY-MM
	Next     string // YYYY-MM
	Title    string // "Ekim 2026", set by the loader
	Weekdays []string
	Action   string // where the new-card form posts, set by the loader
	Notices  []string
	Weeks    []calendarWeek
}

type calendarWeek struct {
	Days []calendarDay // Monday to Sunday
	Bars []calendarBar // only those in the shown lanes
}

type calendarDay struct {
	Date    string // YYYY-MM-DD
	Num     int
	Col     int  // 1–7
	Outside bool // in a neighbouring month
	Today   bool
	More    int // cards on the day beyond the shown lanes
	Cards   []calendarCardRef
}

type calendarCardRef struct {
	CardID int64
	Title  string
	Done   bool
	Late   bool
}

// calendarBar is a card's part in one week.
type calendarBar struct {
	CardID  int64
	Version int
	Title   string
	Color   string
	Col     int // first day in the week, 1–7
	Span    int // days in the week, 1–7
	Lane    int // 1–calendarLanes
	Start   string
	Due     string
	Before  bool // the card began in an earlier week
	After   bool // the card goes on into a later week
	Done    bool
	Late    bool
	Dimmed  bool // left out by the board filter
}

// calendarMonth is the month ?month=YYYY-MM names; anything else is now's.
func calendarMonth(raw string, now time.Time) time.Time {
	if t, err := time.Parse("2006-01", raw); err == nil {
		return t
	}
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// layoutCalendar lays out month's weeks, Monday to Sunday, with the days of
// the neighbouring months that fill them, and the cards on their days.
func layoutCalendar(month time.Time, cards []store.GanttCard, colors map[int64]string, matching map[int64]bool, now time.Time) calendarView {
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	today := day(now)
	first := month.AddDate(0, 0, -((int(month.Weekday()) + 6) % 7)) // back to Monday
	last := month.AddDate(0, 1, -1)
	last = last.AddDate(0, 0, (7-int(last.Weekday()))%7) // on to Sunday
	v := calendarView{
		Month: month.Format("2006-01"),
		Prev:  month.AddDate(0, -1, 0).Format("2006-01"),
		Next:  month.AddDate(0, 1, 0).Format("2006-01"),
	}
	for ws := first; !ws.After(last); ws = ws.AddDate(0, 0, 7) {
		v.Weeks = append(v.Weeks, layoutWeek(ws, month.Month(), cards, colors, matching, today))
	}
	return v
}

// layoutWeek cuts the week's part out of every card that touches it and gives
// each part a lane: earlier first, then longer, into the first lane free.
func layoutWeek(ws time.Time, month time.Month, cards []store.GanttCard, colors map[int64]string, matching map[int64]bool, today time.Time) calendarWeek {
	we := ws.AddDate(0, 0, 6)
	var w calendarWeek
	for i := range 7 {
		d := ws.AddDate(0, 0, i)
		w.Days = append(w.Days, calendarDay{Date: d.Format(time.DateOnly), Num: d.Day(), Col: i + 1,
			Outside: d.Month() != month, Today: d.Equal(today)})
	}
	var bars []calendarBar
	for _, g := range cards {
		s, e, ok := span(g.Card)
		if !ok {
			continue
		}
		s, e = day(s), day(e)
		if e.Before(ws) || s.After(we) {
			continue
		}
		from, to := s, e
		if from.Before(ws) {
			from = ws
		}
		if to.After(we) {
			to = we
		}
		c := g.Card
		done := c.CompletedAt != nil
		b := calendarBar{CardID: c.ID, Version: c.Version, Title: c.Title, Color: colors[c.ColumnID],
			Col: daysBetween(ws, from) + 1, Span: daysBetween(from, to) + 1,
			Before: s.Before(ws), After: e.After(we), Done: done,
			Late:   !done && c.DueDate != nil && day(*c.DueDate).Before(today),
			Dimmed: matching != nil && !matching[c.ID]}
		if c.StartDate != nil {
			b.Start = c.StartDate.Format(time.DateOnly)
		}
		if c.DueDate != nil {
			b.Due = c.DueDate.Format(time.DateOnly)
		}
		bars = append(bars, b)
	}
	slices.SortStableFunc(bars, func(a, b calendarBar) int {
		if a.Col != b.Col {
			return a.Col - b.Col
		}
		return b.Span - a.Span
	})
	var laneEnd []int // the last day taken in each lane
	for _, b := range bars {
		lane := 0
		for lane < len(laneEnd) && laneEnd[lane] >= b.Col {
			lane++
		}
		if lane == len(laneEnd) {
			laneEnd = append(laneEnd, 0)
		}
		laneEnd[lane] = b.Col + b.Span - 1
		b.Lane = lane + 1
		for i := b.Col - 1; i < b.Col-1+b.Span; i++ {
			w.Days[i].Cards = append(w.Days[i].Cards, calendarCardRef{CardID: b.CardID, Title: b.Title, Done: b.Done, Late: b.Late})
			if b.Lane > calendarLanes {
				w.Days[i].More++
			}
		}
		if b.Lane <= calendarLanes {
			w.Bars = append(w.Bars, b)
		}
	}
	return w
}
