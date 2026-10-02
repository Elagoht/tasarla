package blueprint

import (
	"kanban/internal/rules"
	"kanban/internal/store"
)

const (
	red    = "#e03131"
	orange = "#f08c00"
	green  = "#2f9e44"
	blue   = "#1971c2"
	violet = "#7048e8"
	pink   = "#c2255c"
	teal   = "#0c8599"
	grey   = "#495057"

	high = 3 // card.priorities.3: "Yüksek" / "High"
)

// Weekdays: bit 0 is Monday.
const (
	monday = 1 << 0
	friday = 1 << 4
)

var catalog = []Blueprint{
	{Key: "simple", Columns: []Column{{Key: "todo"}, {Key: "doing"}, {Key: "done"}}},
	{
		Key:     "scrum",
		Columns: []Column{{Key: "backlog"}, {Key: "sprint"}, {Key: "doing", WIP: 3}, {Key: "review", WIP: 2}, {Key: "done"}},
		Labels:  []Label{{"story", blue}, {"bug", red}, {"debt", orange}, {"research", violet}},
		Templates: []CardTemplate{
			{Key: "story", Labels: []int{0}, Checklist: 3},
			{Key: "bug", Priority: high, Labels: []int{1}, Checklist: 3},
			{Key: "retro", Checklist: 3, Schedule: store.Schedule{Kind: "weekly", Weekdays: friday, Hour: 16}},
		},
		Rules: Rules{Conditions: []Condition{
			{Column: 2, Phase: rules.PhaseEnter, Kind: rules.HasAssignee},
			{Column: 4, Phase: rules.PhaseEnter, Kind: rules.ChecklistComplete},
		}},
	},
	{
		Key:       "bugs",
		Columns:   []Column{{Key: "new"}, {Key: "triaged"}, {Key: "fixing", WIP: 3}, {Key: "testing"}, {Key: "closed"}},
		Labels:    []Label{{"critical", red}, {"regression", pink}, {"ui", blue}, {"server", teal}},
		Templates: []CardTemplate{{Key: "report", Priority: high, Checklist: 3}},
		Rules: Rules{
			Permissions: []Permission{{To: 4, Subject: rules.SubjectTeamLead}},
			Conditions:  []Condition{{Column: 2, Phase: rules.PhaseEnter, Kind: rules.HasAssignee}},
		},
	},
	{
		Key: "software",
		Columns: []Column{{Key: "backlog"}, {Key: "ready"}, {Key: "dev", WIP: 3, CountsPersonWIP: true},
			{Key: "review", WIP: 2, CountsPersonWIP: true}, {Key: "test"}, {Key: "live"}},
		Labels: []Label{{"feature", green}, {"improvement", blue}, {"infra", grey}},
		Templates: []CardTemplate{
			{Key: "feature", Labels: []int{0}, Checklist: 4},
			{Key: "task", Labels: []int{2}, Checklist: 3},
		},
		Rules: Rules{PersonWIP: 2, Conditions: []Condition{{Column: 3, Phase: rules.PhaseExit, Kind: rules.BlockersDone}}},
	},
	{
		Key:     "content",
		Columns: []Column{{Key: "ideas"}, {Key: "writing"}, {Key: "editing"}, {Key: "scheduled"}, {Key: "published"}},
		Labels:  []Label{{"blog", blue}, {"social", pink}, {"newsletter", orange}, {"video", violet}},
		Templates: []CardTemplate{
			{Key: "blog", DueInDays: 7, Labels: []int{0}, Checklist: 4},
			{Key: "newsletter", Labels: []int{2}, Checklist: 4, Schedule: store.Schedule{Kind: "weekly", Weekdays: monday, Hour: 9}},
		},
		Rules: Rules{Conditions: []Condition{{Column: 3, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}}},
	},
	{
		Key:       "hiring",
		Columns:   []Column{{Key: "applied"}, {Key: "screen"}, {Key: "interview"}, {Key: "offer"}, {Key: "hired"}},
		Labels:    []Label{{"frontend", blue}, {"backend", teal}, {"design", pink}, {"intern", green}},
		Templates: []CardTemplate{{Key: "candidate", Checklist: 4}},
		Rules:     Rules{Permissions: []Permission{{To: 3, Subject: rules.SubjectTeamLead}}},
	},
	{
		Key:       "support",
		Columns:   []Column{{Key: "new"}, {Key: "investigating", WIP: 5}, {Key: "waiting"}, {Key: "solved"}},
		Labels:    []Label{{"urgent", red}, {"billing", orange}, {"account", blue}, {"bug", pink}},
		Templates: []CardTemplate{{Key: "ticket", Priority: high, Checklist: 3}},
		Rules:     Rules{Conditions: []Condition{{Column: 1, Phase: rules.PhaseEnter, Kind: rules.HasAssignee}}},
	},
}
