// Package rules decides whether a card may move, be created or be assigned
// under a board's rules (spec §5). It is pure: it reads no database and knows
// nothing of HTTP; callers hand it a Snapshot of everything it needs.
package rules

import (
	"slices"
	"strconv"
	"strings"
)

// Transition modes.
const (
	ModeOpen       = "open"
	ModeRestricted = "restricted"
)

// Team roles, as an Actor holds them.
const (
	RoleLead   = "lead"
	RoleMember = "member"
)

// Permission subjects.
const (
	SubjectAnyMember = "any_member"
	SubjectAssignee  = "assignee"
	SubjectTeamLead  = "team_lead"
	SubjectBoardRole = "board_role"
)

// Condition phases.
const (
	PhaseEnter = "enter"
	PhaseExit  = "exit"
)

// Condition kinds: the closed catalogue of §5.1.
const (
	HasAssignee       = "has_assignee"
	HasEstimate       = "has_estimate"
	HasDueDate        = "has_due_date"
	HasDescription    = "has_description"
	HasLabel          = "has_label"
	ChecklistComplete = "checklist_complete"
	BlockersDone      = "blockers_done"
	MinAttachments    = "min_attachments"
)

// Kinds lists every condition kind, in the order settings show them.
var Kinds = []string{HasAssignee, HasEstimate, HasDueDate, HasDescription, HasLabel, ChecklistComplete, BlockersDone, MinAttachments}

// Subjects lists every permission subject.
var Subjects = []string{SubjectAnyMember, SubjectAssignee, SubjectTeamLead, SubjectBoardRole}

// Actor is who asks for the move.
type Actor struct {
	UserID     int64
	IsAdmin    bool
	TeamRole   string  // RoleLead, RoleMember, or "" outside the team
	BoardRoles []int64 // board roles the actor holds on this board
}

// Move is a card going from one column to another.
type Move struct {
	CardID       int64
	FromColumnID int64
	ToColumnID   int64
	ToIndex      int
}

// Column is a board column as the rules see it. Count is its cards that are not
// archived, not counting the card being moved.
type Column struct {
	ID              int64
	Name            string
	WIPLimit        int // 0: none
	IsDone          bool
	CountsPersonWIP bool
	Count           int
}

// Card is what conditions read of the card being moved.
type Card struct {
	ID             int64
	AssigneeID     int64 // 0: unassigned
	HasEstimate    bool
	HasDueDate     bool
	Description    string
	LabelIDs       []int64
	ChecklistTotal int
	ChecklistDone  int
	OpenBlockers   int // blockers neither archived nor in a done column
	Attachments    int
}

// Transition is an allowed (from, to) pair under ModeRestricted.
type Transition struct {
	From int64
	To   int64
}

// Permission says who may move cards into a column, or along one transition
// when FromColumnID is set.
type Permission struct {
	ToColumnID   int64
	FromColumnID int64 // 0: from any column
	Subject      string
	BoardRoleID  int64 // for SubjectBoardRole
}

// ConditionParams are a condition's parameters.
type ConditionParams struct {
	LabelIDs []int64 `json:"label_ids,omitempty"` // HasLabel: any of these; empty means any label
	Count    int     `json:"count,omitempty"`     // MinAttachments
}

// Condition is a gate on entering or leaving a column.
type Condition struct {
	ColumnID int64
	Phase    string
	Kind     string
	Params   ConditionParams
}

// Snapshot is everything about the board that a decision reads.
type Snapshot struct {
	TransitionsMode string
	Transitions     map[Transition]bool
	Permissions     []Permission
	Conditions      []Condition
	Columns         map[int64]Column
	Card            Card
	PersonWIPLimit  int // 0: none
	AssigneeWIP     int // the assignee's cards in counted columns, not counting this card
	LabelNames      map[int64]string
}

// Violation is one rule a decision breaks. Code names the message; Params fill
// it.
type Violation struct {
	Code   string
	Params map[string]string
}

// Evaluate returns every rule the move breaks, not only the first, in the order
// transition, permission, exit, entry, column WIP, person WIP. Reordering inside
// one column breaks none.
func Evaluate(actor Actor, m Move, s Snapshot) []Violation {
	if m.FromColumnID == m.ToColumnID {
		return nil
	}
	from, to := s.Columns[m.FromColumnID], s.Columns[m.ToColumnID]
	var vs []Violation
	if s.TransitionsMode == ModeRestricted && !s.Transitions[Transition{From: from.ID, To: to.ID}] {
		vs = append(vs, Violation{Code: "rules.transition", Params: map[string]string{"from": from.Name, "to": to.Name}})
	}
	if !permitted(actor, m, s) {
		vs = append(vs, Violation{Code: "rules.permission", Params: map[string]string{"column": to.Name}})
	}
	vs = append(vs, conditions(s, from, PhaseExit)...)
	vs = append(vs, conditions(s, to, PhaseEnter)...)
	vs = append(vs, columnWIP(to)...)
	if !from.CountsPersonWIP {
		vs = append(vs, personWIP(s, to)...)
	}
	return vs
}

// EvaluateCreate treats creating a card in a column as entering it: its entry
// conditions and its WIP limit apply (§5.2).
func EvaluateCreate(columnID int64, s Snapshot) []Violation {
	to := s.Columns[columnID]
	return append(conditions(s, to, PhaseEnter), columnWIP(to)...)
}

// EvaluateAssign checks the new assignee's WIP when the card sits in a counted
// column; s.Card.AssigneeID is the new assignee.
func EvaluateAssign(columnID int64, s Snapshot) []Violation {
	return personWIP(s, s.Columns[columnID])
}

func permitted(actor Actor, m Move, s Snapshot) bool {
	var applicable []Permission
	for _, p := range s.Permissions {
		if p.ToColumnID == m.ToColumnID && (p.FromColumnID == 0 || p.FromColumnID == m.FromColumnID) {
			applicable = append(applicable, p)
		}
	}
	if len(applicable) == 0 {
		return true // no record: any member of the team
	}
	for _, p := range applicable {
		switch p.Subject {
		case SubjectAnyMember:
			if actor.IsAdmin || actor.TeamRole != "" {
				return true
			}
		case SubjectAssignee:
			if s.Card.AssigneeID != 0 && s.Card.AssigneeID == actor.UserID {
				return true
			}
		case SubjectTeamLead:
			if actor.IsAdmin || actor.TeamRole == RoleLead {
				return true
			}
		case SubjectBoardRole:
			if slices.Contains(actor.BoardRoles, p.BoardRoleID) {
				return true
			}
		}
	}
	return false
}

func conditions(s Snapshot, col Column, phase string) []Violation {
	var vs []Violation
	for _, c := range s.Conditions {
		if c.ColumnID != col.ID || c.Phase != phase || holds(c, s.Card) {
			continue
		}
		params := map[string]string{"column": col.Name, "phase": phase}
		if c.Kind == HasLabel && len(c.Params.LabelIDs) > 0 {
			names := []string{}
			for _, id := range c.Params.LabelIDs {
				if n, ok := s.LabelNames[id]; ok {
					names = append(names, n)
				}
			}
			params["labels"] = strings.Join(names, ", ")
		}
		if c.Kind == MinAttachments {
			params["count"] = strconv.Itoa(c.Params.Count)
		}
		vs = append(vs, Violation{Code: "rules.condition." + c.Kind, Params: params})
	}
	return vs
}

func holds(c Condition, card Card) bool {
	switch c.Kind {
	case HasAssignee:
		return card.AssigneeID != 0
	case HasEstimate:
		return card.HasEstimate
	case HasDueDate:
		return card.HasDueDate
	case HasDescription:
		return strings.TrimSpace(card.Description) != ""
	case HasLabel:
		if len(c.Params.LabelIDs) == 0 {
			return len(card.LabelIDs) > 0
		}
		for _, id := range card.LabelIDs {
			if slices.Contains(c.Params.LabelIDs, id) {
				return true
			}
		}
		return false
	case ChecklistComplete:
		return card.ChecklistDone >= card.ChecklistTotal
	case BlockersDone:
		return card.OpenBlockers == 0
	case MinAttachments:
		return card.Attachments >= c.Params.Count
	}
	return true // an unknown kind constrains nothing
}

func columnWIP(to Column) []Violation {
	if to.WIPLimit > 0 && to.Count+1 > to.WIPLimit {
		return []Violation{{Code: "rules.wip_column", Params: map[string]string{"column": to.Name, "limit": strconv.Itoa(to.WIPLimit)}}}
	}
	return nil
}

func personWIP(s Snapshot, to Column) []Violation {
	if s.PersonWIPLimit > 0 && to.CountsPersonWIP && s.Card.AssigneeID != 0 && s.AssigneeWIP+1 > s.PersonWIPLimit {
		return []Violation{{Code: "rules.wip_person", Params: map[string]string{"limit": strconv.Itoa(s.PersonWIPLimit)}}}
	}
	return nil
}
