package web

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"kanban/internal/store"
)

// laneTarget reads a drop onto a lane from the move form: the lane's field and
// value, and the card it was dropped above. ok is false for anything else.
func laneTarget(lane, value, before string) (store.LaneTarget, bool) {
	var t store.LaneTarget
	if before != "" {
		id, err := strconv.ParseInt(before, 10, 64)
		if err != nil {
			return t, false
		}
		t.BeforeCardID = id
	}
	switch lane {
	case "assignee":
		t.Field = store.FieldAssignee
		if value != "none" {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return t, false
			}
			t.AssigneeID = &id
		}
	case "priority":
		t.Field = store.FieldPriority
		if value != "none" {
			p, err := strconv.Atoi(value)
			if err != nil || p < 1 || p > 4 {
				return t, false
			}
			v := int16(p)
			t.Priority = &v
		}
	default:
		return t, false
	}
	return t, true
}

// laneView is one lane of the board: a row of cells, one per column.
type laneView struct {
	Key    string // "assignee:12", "assignee:none", "priority:4", "priority:none" — stable, for collapsing
	Value  string // the lane_value a drop here sends
	Label  string
	Former bool // assignee lane of someone no longer in the team
	Count  int
	Cells  []laneCell // one per column, in column order
}

// laneCell is a lane's part of a column.
type laneCell struct {
	ColumnID int64
	Done     bool
	Empty    string // the drop hint, as the column's
	Cards    []cardView
}

// buildLanes splits the columns' cards into lanes by field ("assignee" or
// "priority"). members are the team's; unassigned and none are the labels of
// the empty lanes; priorityLabel names a priority.
//
// Lanes come in this order: by assignee, the unassigned first, then members
// by name (Turkish order, case ignored), then those no longer in the team; by priority, urgent first and
// none last. Only lanes with cards are made.
func buildLanes(field string, cols []columnView, members []store.Member, unassigned, none string, priorityLabel func(int16) string) []laneView {
	type lane struct {
		view laneView
		rank int    // order: unassigned/urgent first
		name string // for assignee lanes, the name
	}
	byKey := map[string]*lane{}
	member := map[int64]bool{}
	for _, m := range members {
		member[m.User.ID] = true
	}
	get := func(c cardView) *lane {
		s := c.Summary
		var key, value, label, name string
		rank := 0
		former := false
		if field == "assignee" {
			if s.Card.AssigneeID == nil {
				key, value, label = "assignee:none", "none", unassigned
			} else {
				id := *s.Card.AssigneeID
				value = strconv.FormatInt(id, 10)
				key, label, name = "assignee:"+value, s.AssigneeName, s.AssigneeName
				rank, former = 1, !member[id]
				if former {
					rank = 2
				}
			}
		} else {
			if s.Card.Priority == nil {
				key, value, label, rank = "priority:none", "none", none, 5
			} else {
				p := *s.Card.Priority
				value = strconv.Itoa(int(p))
				key, label, rank = "priority:"+value, priorityLabel(p), int(5-p)
			}
		}
		l := byKey[key]
		if l == nil {
			l = &lane{view: laneView{Key: key, Value: value, Label: label, Former: former}, rank: rank, name: name}
			for _, col := range cols {
				l.view.Cells = append(l.view.Cells, laneCell{ColumnID: col.Column.ID, Done: col.Column.IsDone})
			}
			byKey[key] = l
		}
		return l
	}
	for i, col := range cols {
		for _, c := range col.Cards {
			l := get(c)
			l.view.Cells[i].Cards = append(l.view.Cells[i].Cards, c)
			l.view.Count++
		}
	}
	lanes := slices.Collect(maps.Values(byKey))
	// A collator is not safe for concurrent use, so each call has its own.
	byName := collate.New(language.Turkish, collate.IgnoreCase)
	slices.SortFunc(lanes, func(a, b *lane) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}
		if c := byName.CompareString(a.name, b.name); c != 0 {
			return c
		}
		return strings.Compare(a.view.Key, b.view.Key)
	})
	out := make([]laneView, len(lanes))
	for i, l := range lanes {
		out[i] = l.view
	}
	return out
}
