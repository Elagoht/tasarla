package web

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"kanban/internal/store"
)

func TestParseBoardFilterIsCanonical(t *testing.T) {
	members := []store.Member{{User: store.User{ID: 7}}, {User: store.User{ID: 3}}}
	labels := []store.Label{{ID: 20}, {ID: 10}}
	q, _ := url.ParseQuery("q=+r%C3%A2+&assignee=7&assignee=me&assignee=abc&assignee=99&assignee=3&assignee=7&assignee=none" +
		"&label=20&label=999&label=10&priority=9&priority=4&priority=1&due=yarin")
	f := parseBoardFilter(q, members, labels)
	want := "assignee=me&assignee=none&assignee=3&assignee=7&label=10&label=20&priority=1&priority=4&q=r%C3%A2"
	if got := f.Query(); got != want {
		t.Errorf("Query() = %q\n want %q", got, want)
	}
	if f.Due != store.DueAny {
		t.Errorf("due = %q, want dropped", f.Due)
	}
}

func TestParseBoardFilterDropsShortTextAndEmpty(t *testing.T) {
	q, _ := url.ParseQuery("q=a&due=week")
	f := parseBoardFilter(q, nil, nil)
	if f.Text != "" || f.Query() != "due=week" {
		t.Errorf("got text %q, query %q", f.Text, f.Query())
	}
	if f := parseBoardFilter(url.Values{}, nil, nil); f.Active() || f.Query() != "" {
		t.Errorf("empty filter is active: %q", f.Query())
	}
}

func TestBoardFilterStoreResolvesMe(t *testing.T) {
	q, _ := url.ParseQuery("assignee=me&assignee=3&assignee=none")
	f := parseBoardFilter(q, []store.Member{{User: store.User{ID: 3}}}, nil)
	today := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	s := f.Store(42, today)
	if !slices.Equal(s.AssigneeIDs, []int64{3, 42}) || !s.Unassigned || !s.Today.Equal(today) {
		t.Errorf("store filter = %+v", s)
	}
}

func TestWithQuery(t *testing.T) {
	if withQuery("/boards/1", "") != "/boards/1" || withQuery("/boards/1", "q=ab") != "/boards/1?q=ab" {
		t.Error("withQuery")
	}
}

func TestBoardFilterLaneIsASettingNotAFilter(t *testing.T) {
	q, _ := url.ParseQuery("lane=assignee&priority=4")
	f := parseBoardFilter(q, nil, nil)
	if f.Lane != "assignee" || f.Query() != "lane=assignee&priority=4" || f.ClearQuery() != "lane=assignee" {
		t.Errorf("lane %q, query %q, clear %q", f.Lane, f.Query(), f.ClearQuery())
	}
	only, _ := url.ParseQuery("lane=priority")
	if g := parseBoardFilter(only, nil, nil); g.Active() || g.Query() != "lane=priority" {
		t.Errorf("a lane alone is active (%v) or lost (%q)", g.Active(), g.Query())
	}
	bad, _ := url.ParseQuery("lane=label")
	if g := parseBoardFilter(bad, nil, nil); g.Lane != "" || g.Query() != "" || g.ClearQuery() != "" {
		t.Errorf("an unknown lane is kept: %+v", g)
	}
}
