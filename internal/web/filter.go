package web

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kanban/internal/store"
)

// boardFilter is the filter in a board's URL (spec 2026-10-01 filtre… §3.1),
// checked against the board: a value naming no member, label or choice is
// dropped, never refused, so an old link keeps working.
type boardFilter struct {
	Lane       string
	Text       string
	Me         bool // assignee=me: whoever is reading
	None       bool // assignee=none: unassigned
	Assignees  []int64
	Labels     []int64
	Priorities []int16
	Due        store.DueFilter
}

var (
	laneChoices = []string{"assignee", "priority"}
	dueFilters  = []store.DueFilter{store.DueOverdue, store.DueToday, store.DueWeek, store.DueNone}
)

func parseBoardFilter(q url.Values, members []store.Member, labels []store.Label) boardFilter {
	var f boardFilter
	if l := q.Get("lane"); slices.Contains(laneChoices, l) {
		f.Lane = l
	}
	if text := strings.TrimSpace(q.Get("q")); utf8.RuneCountInString(text) >= 2 {
		f.Text = text
	}
	isMember := map[int64]bool{}
	for _, m := range members {
		isMember[m.User.ID] = true
	}
	for _, v := range q["assignee"] {
		switch v {
		case "me":
			f.Me = true
		case "none":
			f.None = true
		default:
			if id, err := strconv.ParseInt(v, 10, 64); err == nil && isMember[id] && !slices.Contains(f.Assignees, id) {
				f.Assignees = append(f.Assignees, id)
			}
		}
	}
	isLabel := map[int64]bool{}
	for _, l := range labels {
		isLabel[l.ID] = true
	}
	for _, v := range q["label"] {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && isLabel[id] && !slices.Contains(f.Labels, id) {
			f.Labels = append(f.Labels, id)
		}
	}
	for _, v := range q["priority"] {
		if p, err := strconv.Atoi(v); err == nil && p >= 1 && p <= 4 && !slices.Contains(f.Priorities, int16(p)) {
			f.Priorities = append(f.Priorities, int16(p))
		}
	}
	if d := store.DueFilter(q.Get("due")); slices.Contains(dueFilters, d) {
		f.Due = d
	}
	slices.Sort(f.Assignees)
	slices.Sort(f.Labels)
	slices.Sort(f.Priorities)
	return f
}

// Active reports whether the filter dims anything; the lane is a view, not a filter.
func (f boardFilter) Active() bool {
	g := f
	g.Lane = ""
	return len(g.Values()) > 0
}

// ClearQuery is the query of the board with the filter cleared: the lane kept.
func (f boardFilter) ClearQuery() string { return f.cleared().Query() }

// cleared is the filter with only the lane left.
func (f boardFilter) cleared() boardFilter { return boardFilter{Lane: f.Lane} }

// Values is the filter as a canonical query: one filter, one URL.
func (f boardFilter) Values() url.Values {
	v := url.Values{}
	if f.Lane != "" {
		v.Set("lane", f.Lane)
	}
	if f.Text != "" {
		v.Set("q", f.Text)
	}
	if f.Me {
		v.Add("assignee", "me")
	}
	if f.None {
		v.Add("assignee", "none")
	}
	for _, id := range f.Assignees {
		v.Add("assignee", strconv.FormatInt(id, 10))
	}
	for _, id := range f.Labels {
		v.Add("label", strconv.FormatInt(id, 10))
	}
	for _, p := range f.Priorities {
		v.Add("priority", strconv.Itoa(int(p)))
	}
	if f.Due != store.DueAny {
		v.Set("due", string(f.Due))
	}
	return v
}

// Query is Values encoded; "" when the filter is not active.
func (f boardFilter) Query() string { return f.Values().Encode() }

// Store is the filter for reader me on the day today.
func (f boardFilter) Store(me int64, today time.Time) store.BoardFilter {
	ids := slices.Clone(f.Assignees)
	if f.Me && !slices.Contains(ids, me) {
		ids = append(ids, me)
		slices.Sort(ids)
	}
	return store.BoardFilter{Text: f.Text, AssigneeIDs: ids, Unassigned: f.None, LabelIDs: f.Labels,
		Priorities: f.Priorities, Due: f.Due, Today: today}
}

func (f boardFilter) HasAssignee(token string) bool {
	switch token {
	case "me":
		return f.Me
	case "none":
		return f.None
	}
	id, err := strconv.ParseInt(token, 10, 64)
	return err == nil && slices.Contains(f.Assignees, id)
}

func (f boardFilter) HasLabel(id int64) bool   { return slices.Contains(f.Labels, id) }
func (f boardFilter) HasPriority(p int16) bool { return slices.Contains(f.Priorities, p) }

// clearQuery is the query of the bar's Clear link: the page's own settings
// (Extra) and the filter cleared, the lane kept — ClearQuery with Extra.
func clearQuery(v filterView) string {
	q := v.Filter.cleared().Values()
	for k, vals := range v.Extra {
		q[k] = vals
	}
	return q.Encode()
}

// withQuery is path, with ?query when there is one.
func withQuery(path, query string) string {
	if query == "" {
		return path
	}
	return path + "?" + query
}
