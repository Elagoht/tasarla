package web

import (
	"context"
	"fmt"
	"html/template"
	"net/url"
	"slices"
	"strconv"
	"time"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/config"
	"kanban/internal/store"
)

// The Gantt chart's geometry, in pixels.
const (
	ganttHeader = 52
	ganttRow    = 36
	ganttBar    = 22
)

var (
	ganttScales = []string{"day", "week", "month"}
	ganttGroups = []string{"column", "assignee"}
	// dayWidth is how wide one day is at each scale.
	dayWidth = map[string]int{"day": 36, "week": 14, "month": 5}
	// ganttPad is how many days the chart shows past its first and last date.
	ganttPad = map[string]int{"day": 3, "week": 7, "month": 14}
)

type ganttPageView struct {
	Board  store.Board
	Scale  string
	Group  string
	Done   bool
	Scales []string
	Groups []string
	Query  string // the chart's settings and filter, for its fragment URL
	Filter filterView
}

type ganttView struct {
	BoardID  int64
	CanEdit  bool
	DayWidth int
	From     string // the first day shown, YYYY-MM-DD
	Width    int
	Height   int
	Header   int
	RowH     int
	Months   []ganttTick
	Ticks    []ganttTick
	Weekends []ganttBand
	Today    int // x of today's line; -1 when today is not shown
	Rows     []ganttRowView
	Bars     []ganttBarView
	Arrows   []ganttArrow
	Undated  []store.GanttCard
	Empty    bool
}

type ganttTick struct {
	X     int
	Label string
}

type ganttBand struct{ X, W int }

type ganttRowView struct {
	Y      int
	Group  bool
	Label  string
	Color  string
	CardID int64
	Done   bool
	Sub    string
	Dimmed bool
}

type ganttBarView struct {
	CardID    int64
	Version   int
	Title     string
	Color     string
	Done      bool
	Milestone bool
	X, Y, W   int
	CX, CY    int
	Points    string // the milestone's diamond
	TextX     int
	EndX      int  // where the handle that moves the due date sits
	TextIn    bool // the title fits inside the bar
	Start     string
	Due       string
	Range     string // the dates, for a tooltip
	Dimmed    bool   // the card is left out by the board filter
}

type ganttArrow struct {
	D    string
	Late bool
}

func (h *handlers) boardGanttPage() *collage.Page {
	h.gantt = collage.NewFragment("board-gantt-chart", "fragments/gantt.html").
		WithData(collage.DataHandler(h.loadGantt)).
		Required().
		Build()
	content := collage.NewFragment("board-gantt-content", "pages/board_gantt.html").
		WithData(collage.Load(h.loadGanttPage)).
		WithSlotFragment("chart", h.gantt).
		Required().
		Build()
	b := paths(h.privatePage("board-gantt", content), "/boards/{id}/gantt")
	for _, l := range config.Locales {
		b = b.WithFragmentPath(l, "/boards/{id}/gantt/chart", h.gantt)
	}
	return b.Dynamic().Build()
}

// ganttSettings reads the chart's scale, grouping and whether done cards show.
func ganttSettings(rc *collage.RenderContext) (scale, group string, done bool) {
	q := rc.Request.URL.Query()
	scale, group, done = q.Get("scale"), q.Get("group"), q.Get("done") == "1"
	if !slices.Contains(ganttScales, scale) {
		scale = "day"
	}
	if !slices.Contains(ganttGroups, group) {
		group = "column"
	}
	return scale, group, done
}

func (h *handlers) loadGanttPage(ctx context.Context, rc *collage.RenderContext) (ganttPageView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return ganttPageView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "gantt.title") + " · " + bc.Board.Name)
	v := ganttPageView{Board: bc.Board, Scales: ganttScales, Groups: ganttGroups}
	v.Scale, v.Group, v.Done = ganttSettings(rc)
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return ganttPageView{}, err
	}
	// The bar keeps the chart's settings as hidden fields, so a new filter
	// reloads the page with them.
	fv.Extra = url.Values{"scale": {v.Scale}, "group": {v.Group}}
	if v.Done {
		fv.Extra.Set("done", "1")
	}
	fv.Action, err = h.urlIn("board-gantt", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
	if err != nil {
		return ganttPageView{}, err
	}
	q := fv.Filter.Values()
	for k, vals := range fv.Extra {
		q[k] = vals
	}
	v.Query = q.Encode()
	v.Filter = fv
	return v, nil
}

func (h *handlers) loadGantt(ctx context.Context, rc *collage.RenderContext) (ganttView, []string, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return ganttView{}, nil, err
	}
	tags := []string{boardTag(bc.Board.ID)}
	scale, group, done := ganttSettings(rc)
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return ganttView{}, tags, err
	}
	cards, err := h.store.GanttCards(ctx, bc.Board.ID, done)
	if err != nil {
		return ganttView{}, tags, err
	}
	deps, err := h.store.BoardDependencies(ctx, bc.Board.ID)
	if err != nil {
		return ganttView{}, tags, err
	}
	matching, err := h.boardMatching(ctx, rc, bc)
	if err != nil {
		return ganttView{}, tags, err
	}
	now := time.Now().In(h.loc)
	return layoutGantt(rc, bc, scale, group, cols, cards, deps, now, matching), tags, nil
}

// boardMatching is the cards the board filter lets through, nil when the
// filter is empty.
func (h *handlers) boardMatching(ctx context.Context, rc *collage.RenderContext, bc boardContext) (map[int64]bool, error) {
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return nil, err
	}
	return h.store.MatchingCardIDs(ctx, bc.Board.ID, fv.Filter.Store(bc.User.ID, time.Now().In(h.loc)))
}

// span is a card's dates on the chart: start and end, inclusive.
func span(c store.Card) (time.Time, time.Time, bool) {
	switch {
	case c.StartDate != nil && c.DueDate != nil:
		return *c.StartDate, *c.DueDate, true
	case c.DueDate != nil:
		return *c.DueDate, *c.DueDate, true
	case c.StartDate != nil:
		return *c.StartDate, *c.StartDate, true
	}
	return time.Time{}, time.Time{}, false
}

func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }

func daysBetween(a, b time.Time) int { return int(day(b).Sub(day(a)).Hours() / 24) }

func layoutGantt(rc *collage.RenderContext, bc boardContext, scale, group string, cols []store.Column,
	cards []store.GanttCard, deps []store.Dependency, now time.Time, matching map[int64]bool) ganttView {
	v := ganttView{BoardID: bc.Board.ID, CanEdit: bc.Access.CanEdit, DayWidth: dayWidth[scale], Header: ganttHeader, RowH: ganttRow, Today: -1}
	today := day(now)

	// The range: every dated card, and today.
	from, to := today, today
	var dated []store.GanttCard
	for _, g := range cards {
		s, e, ok := span(g.Card)
		if !ok {
			v.Undated = append(v.Undated, g)
			continue
		}
		dated = append(dated, g)
		if s.Before(from) {
			from = day(s)
		}
		if e.After(to) {
			to = day(e)
		}
	}
	v.Empty = len(dated) == 0
	if v.Empty {
		to = today.AddDate(0, 0, 21)
	}
	from = from.AddDate(0, 0, -ganttPad[scale])
	to = to.AddDate(0, 0, ganttPad[scale])
	switch scale {
	case "week":
		from = from.AddDate(0, 0, -((int(from.Weekday()) + 6) % 7)) // back to Monday
	case "month":
		from = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	days := daysBetween(from, to) + 1
	dw := v.DayWidth
	x := func(t time.Time) int { return daysBetween(from, t) * dw }
	v.From = from.Format(time.DateOnly)
	v.Width = days * dw

	month := func(t time.Time) string { return i18n.T(rc, "gantt.months."+strconv.Itoa(int(t.Month()))) }
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if d.Day() == 1 || d.Equal(from) {
			v.Months = append(v.Months, ganttTick{X: x(d), Label: month(d) + " " + strconv.Itoa(d.Year())})
		}
		switch scale {
		case "day":
			v.Ticks = append(v.Ticks, ganttTick{X: x(d), Label: strconv.Itoa(d.Day())})
		case "week":
			if d.Weekday() == time.Monday {
				v.Ticks = append(v.Ticks, ganttTick{X: x(d), Label: strconv.Itoa(d.Day()) + " " + month(d)})
			}
		case "month":
			if d.Day() == 1 || d.Day() == 15 {
				v.Ticks = append(v.Ticks, ganttTick{X: x(d), Label: strconv.Itoa(d.Day())})
			}
		}
		if scale != "month" && (d.Weekday() == time.Saturday || d.Weekday() == time.Sunday) {
			v.Weekends = append(v.Weekends, ganttBand{X: x(d), W: dw})
		}
	}
	if !today.Before(from) && !today.After(to) {
		v.Today = x(today) + dw/2
	}

	// Rows: a heading per group, then its dated cards.
	colIndex := map[int64]int{}
	colByID := map[int64]store.Column{}
	for i, c := range cols {
		colIndex[c.ID], colByID[c.ID] = i, c
	}
	type bucket struct {
		label, color string
		cards        []store.GanttCard
	}
	var buckets []*bucket
	byKey := map[string]*bucket{}
	keyOf := func(g store.GanttCard) (string, string, string) {
		if group == "assignee" {
			if g.AssigneeName == "" {
				return "~", i18n.T(rc, "card.unassigned"), ""
			}
			return g.AssigneeName, g.AssigneeName, ""
		}
		c := colByID[g.Card.ColumnID]
		return strconv.FormatInt(c.ID, 10), c.Name, boardColor(colIndex[c.ID])
	}
	if group == "column" {
		for i, c := range cols {
			b := &bucket{label: c.Name, color: boardColor(i)}
			byKey[strconv.FormatInt(c.ID, 10)] = b
			buckets = append(buckets, b)
		}
	}
	for _, g := range dated {
		key, label, color := keyOf(g)
		b := byKey[key]
		if b == nil {
			b = &bucket{label: label, color: color}
			byKey[key] = b
			buckets = append(buckets, b)
		}
		b.cards = append(b.cards, g)
	}
	if group == "assignee" {
		slices.SortStableFunc(buckets, func(a, b *bucket) int {
			switch {
			case a.label == b.label:
				return 0
			case byKey["~"] == a:
				return 1
			case byKey["~"] == b:
				return -1
			case a.label < b.label:
				return -1
			}
			return 1
		})
	}

	y := ganttHeader
	mid := map[int64]struct{ y, x1, x2 int }{}
	ends := map[int64]time.Time{}
	starts := map[int64]time.Time{}
	doneCard := map[int64]bool{}
	for _, b := range buckets {
		if len(b.cards) == 0 {
			continue
		}
		v.Rows = append(v.Rows, ganttRowView{Y: y, Group: true, Label: b.label, Color: b.color})
		y += ganttRow
		for _, g := range b.cards {
			c := g.Card
			s, e, _ := span(c)
			done := c.CompletedAt != nil
			color := boardColor(colIndex[c.ColumnID])
			dimmed := matching != nil && !matching[c.ID]
			v.Rows = append(v.Rows, ganttRowView{Y: y, Label: c.Title, Color: color, CardID: c.ID, Done: done, Sub: g.AssigneeName, Dimmed: dimmed})
			bar := ganttBarView{CardID: c.ID, Version: c.Version, Title: c.Title, Color: color, Done: done, Dimmed: dimmed,
				Y: y + (ganttRow-ganttBar)/2, CY: y + ganttRow/2}
			if c.StartDate != nil {
				bar.Start = c.StartDate.Format(time.DateOnly)
			}
			if c.DueDate != nil {
				bar.Due = c.DueDate.Format(time.DateOnly)
			}
			bar.Range = s.Format(time.DateOnly)
			if !e.Equal(s) {
				bar.Range += " – " + e.Format(time.DateOnly)
			}
			if c.StartDate != nil && c.DueDate != nil {
				bar.X, bar.W = x(s), (daysBetween(s, e)+1)*dw
				bar.EndX = bar.X + bar.W - 8
				// Inside the bar when it fits: about 7px a letter at 12px bold.
				bar.TextIn = bar.W >= len([]rune(c.Title))*7+16
				bar.TextX = bar.X + 8
				if !bar.TextIn {
					bar.TextX = bar.X + bar.W + 6
				}
				mid[c.ID] = struct{ y, x1, x2 int }{bar.CY, bar.X, bar.X + bar.W}
			} else {
				bar.Milestone = true
				bar.CX = x(s) + dw/2
				r := 8
				bar.Points = fmt.Sprintf("%d,%d %d,%d %d,%d %d,%d", bar.CX, bar.CY-r, bar.CX+r, bar.CY, bar.CX, bar.CY+r, bar.CX-r, bar.CY)
				bar.TextX = bar.CX + r + 6
				mid[c.ID] = struct{ y, x1, x2 int }{bar.CY, bar.CX - r, bar.CX + r}
			}
			starts[c.ID], ends[c.ID], doneCard[c.ID] = s, e, done
			v.Bars = append(v.Bars, bar)
			y += ganttRow
		}
	}
	v.Height = max(y, ganttHeader+ganttRow)

	// Arrows from a blocker's end to the start of the card it blocks; late
	// when that card starts before its blocker ends.
	for _, d := range deps {
		a, ok1 := mid[d.BlockerID]
		b, ok2 := mid[d.BlockedID]
		if !ok1 || !ok2 {
			continue
		}
		x1, x2 := a.x2, b.x1
		var path string
		if x2-x1 >= 16 {
			path = fmt.Sprintf("M%d %d H%d V%d H%d", x1, a.y, x1+8, b.y, x2-2)
		} else {
			midY := (a.y + b.y) / 2
			if a.y == b.y {
				midY = a.y + ganttRow/2
			}
			path = fmt.Sprintf("M%d %d H%d V%d H%d V%d H%d", x1, a.y, x1+8, midY, x2-10, b.y, x2-2)
		}
		late := !doneCard[d.BlockerID] && !starts[d.BlockedID].After(ends[d.BlockerID])
		v.Arrows = append(v.Arrows, ganttArrow{D: path, Late: late})
	}
	return v
}

// ganttLink is the chart at self with the filter kept and one setting
// changed. It is the whole URL: a query written after "?" in a template gets
// its "=" and "&" escaped.
func ganttLink(self string, f boardFilter, scale, group string, done bool) template.URL {
	v := f.Values()
	v.Set("scale", scale)
	v.Set("group", group)
	if done {
		v.Set("done", "1")
	}
	return template.URL(self + "?" + v.Encode())
}
