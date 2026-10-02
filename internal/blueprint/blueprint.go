// Package blueprint is the catalogue of ready-made boards (spec
// 2026-10-02-board-sablonlari): what each one builds, in no language. Render
// fills its texts in one, Trim keeps the parts the creator chose.
package blueprint

import "kanban/internal/store"

// Default is the blueprint a board is built with when none is chosen: today's
// three columns.
const Default = "simple"

// Blueprint is one ready-made board. Its texts are catalog keys under
// "blueprints.<Key>.".
type Blueprint struct {
	Key       string
	Columns   []Column
	Labels    []Label
	Templates []CardTemplate
	Rules     Rules
}

// Column is a planned column; the first takes new cards, the last is done.
type Column struct {
	Key             string // blueprints.<bp>.columns.<Key>
	WIP             int    // 0: no limit
	CountsPersonWIP bool
}

// Label is a planned label; Color is one of the label palette.
type Label struct{ Key, Color string } // blueprints.<bp>.labels.<Key>

// CardTemplate is a planned card template. Its name, title, description and
// checklist items are under blueprints.<bp>.templates.<Key>.
type CardTemplate struct {
	Key       string
	Priority  int16 // 0: none
	DueInDays int   // 0: none
	Labels    []int // indexes into the blueprint's Labels
	Checklist int   // items: checklist.1 … checklist.<n>
	Schedule  store.Schedule
}

// Rules are a blueprint's rules; indexes are into its Columns.
type Rules struct {
	PersonWIP   int // 0: none
	Permissions []Permission
	Conditions  []Condition
}

// Permission says who may move cards into column To.
type Permission struct {
	To      int
	Subject string // rules.SubjectTeamLead, SubjectAssignee, SubjectAnyMember
}

// Condition gates entering or leaving a column.
type Condition struct {
	Column int
	Phase  string // rules.PhaseEnter, PhaseExit
	Kind   string // rules.HasAssignee, …
}

// Options are the parts the creator brings along; the columns always come.
type Options struct{ WIP, Labels, Templates, Recurring, Rules bool }

// AllOptions brings everything.
func AllOptions() Options {
	return Options{WIP: true, Labels: true, Templates: true, Recurring: true, Rules: true}
}

// OptionsFrom reads the form's include values; unknown ones are ignored.
func OptionsFrom(values []string) Options {
	var o Options
	for _, v := range values {
		switch v {
		case "wip":
			o.WIP = true
		case "labels":
			o.Labels = true
		case "templates":
			o.Templates = true
		case "recurring":
			o.Recurring = true
		case "rules":
			o.Rules = true
		}
	}
	return o
}

// Counts is what a blueprint builds, for its card on the form.
type Counts struct{ Columns, Labels, Templates, Rules int }

// All is the catalogue, in the order the form shows it.
func All() []Blueprint { return catalog }

// Find returns the blueprint with key.
func Find(key string) (Blueprint, bool) {
	for _, bp := range catalog {
		if bp.Key == key {
			return bp, true
		}
	}
	return Blueprint{}, false
}

// Has says which parts bp holds, so the form can turn off the others.
func (bp Blueprint) Has() Options {
	var o Options
	for _, c := range bp.Columns {
		o.WIP = o.WIP || c.WIP > 0
	}
	o.WIP = o.WIP || bp.Rules.PersonWIP > 0
	o.Labels = len(bp.Labels) > 0
	o.Templates = len(bp.Templates) > 0
	for _, t := range bp.Templates {
		o.Recurring = o.Recurring || t.Schedule.Kind != ""
	}
	o.Rules = len(bp.Rules.Permissions) > 0 || len(bp.Rules.Conditions) > 0 || bp.Rules.PersonWIP > 0
	return o
}

// Counts counts bp's parts; the person WIP limit is a rule.
func (bp Blueprint) Counts() Counts {
	rules := len(bp.Rules.Permissions) + len(bp.Rules.Conditions)
	if bp.Rules.PersonWIP > 0 {
		rules++
	}
	return Counts{Columns: len(bp.Columns), Labels: len(bp.Labels), Templates: len(bp.Templates), Rules: rules}
}

func (bp Blueprint) key(rest string) string { return "blueprints." + bp.Key + "." + rest }

// NameKey is the catalog key of bp's name.
func (bp Blueprint) NameKey() string { return bp.key("name") }

// SummaryKey is the catalog key of bp's one-line summary.
func (bp Blueprint) SummaryKey() string { return bp.key("summary") }

// ColumnKeys are the catalog keys of bp's column names, in order.
func (bp Blueprint) ColumnKeys() []string {
	keys := make([]string, len(bp.Columns))
	for i, c := range bp.Columns {
		keys[i] = bp.key("columns." + c.Key)
	}
	return keys
}
