package web

import (
	"strconv"

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
