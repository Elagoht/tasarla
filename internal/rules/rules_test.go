package rules_test

import (
	"reflect"
	"testing"

	"kanban/internal/rules"
)

const (
	todo   int64 = 1
	doing  int64 = 2
	review int64 = 3
	done   int64 = 4
)

// base is a board of four columns with no rules, and a bare card in To do.
func base() rules.Snapshot {
	return rules.Snapshot{
		TransitionsMode: rules.ModeOpen,
		Columns: map[int64]rules.Column{
			todo:   {ID: todo, Name: "To do"},
			doing:  {ID: doing, Name: "Doing"},
			review: {ID: review, Name: "Review"},
			done:   {ID: done, Name: "Done", IsDone: true},
		},
		Card: rules.Card{ID: 10},
	}
}

var member = rules.Actor{UserID: 7, TeamRole: rules.RoleMember}

func move(from, to int64) rules.Move {
	return rules.Move{CardID: 10, FromColumnID: from, ToColumnID: to}
}

func codes(vs []rules.Violation) []string {
	out := []string{}
	for _, v := range vs {
		out = append(out, v.Code)
	}
	return out
}

func TestNoRulesNoViolations(t *testing.T) {
	if vs := rules.Evaluate(member, move(todo, doing), base()); len(vs) != 0 {
		t.Fatalf("violations = %v", vs)
	}
}

func TestTransitions(t *testing.T) {
	s := base()
	s.TransitionsMode = rules.ModeRestricted
	s.Transitions = map[rules.Transition]bool{{From: todo, To: doing}: true}
	if vs := rules.Evaluate(member, move(todo, doing), s); len(vs) != 0 {
		t.Errorf("allowed transition: %v", vs)
	}
	vs := rules.Evaluate(member, move(todo, done), s)
	if !reflect.DeepEqual(codes(vs), []string{"rules.transition"}) {
		t.Fatalf("forbidden transition: %v", vs)
	}
	if vs[0].Params["from"] != "To do" || vs[0].Params["to"] != "Done" {
		t.Errorf("params = %v", vs[0].Params)
	}
	// Open mode ignores the table.
	s.TransitionsMode = rules.ModeOpen
	if vs := rules.Evaluate(member, move(todo, done), s); len(vs) != 0 {
		t.Errorf("open mode: %v", vs)
	}
}

func TestPermissions(t *testing.T) {
	lead := rules.Actor{UserID: 1, TeamRole: rules.RoleLead}
	admin := rules.Actor{UserID: 2, IsAdmin: true}
	qa := rules.Actor{UserID: 3, TeamRole: rules.RoleMember, BoardRoles: []int64{99}}
	assignee := rules.Actor{UserID: 4, TeamRole: rules.RoleMember}

	s := base()
	s.Card.AssigneeID = 4
	s.Permissions = []rules.Permission{
		{ToColumnID: done, Subject: rules.SubjectTeamLead},
		{ToColumnID: done, Subject: rules.SubjectBoardRole, BoardRoleID: 99},
		{ToColumnID: review, FromColumnID: doing, Subject: rules.SubjectAssignee},
	}
	cases := []struct {
		name  string
		actor rules.Actor
		move  rules.Move
		ok    bool
	}{
		{"lead into done", lead, move(review, done), true},
		{"admin counts as lead", admin, move(review, done), true},
		{"board role into done", qa, move(review, done), true},
		{"member into done", member, move(review, done), false},
		{"no record for doing: anyone", member, move(todo, doing), true},
		{"assignee doing to review", assignee, move(doing, review), true},
		{"non-assignee doing to review", member, move(doing, review), false},
		{"the record names another source", member, move(todo, review), true},
	}
	for _, c := range cases {
		vs := rules.Evaluate(c.actor, c.move, s)
		if (len(vs) == 0) != c.ok {
			t.Errorf("%s: violations = %v", c.name, vs)
		}
		if !c.ok && (len(vs) != 1 || vs[0].Code != "rules.permission") {
			t.Errorf("%s: codes = %v", c.name, codes(vs))
		}
	}
	anyMember := base()
	anyMember.Permissions = []rules.Permission{{ToColumnID: done, Subject: rules.SubjectAnyMember}}
	if vs := rules.Evaluate(member, move(todo, done), anyMember); len(vs) != 0 {
		t.Errorf("any_member: %v", vs)
	}
	if vs := rules.Evaluate(rules.Actor{UserID: 9}, move(todo, done), anyMember); len(vs) != 1 {
		t.Errorf("not a member: %v", vs)
	}
}

func TestConditions(t *testing.T) {
	enter := func(kind string, p rules.ConditionParams) rules.Snapshot {
		s := base()
		s.Conditions = []rules.Condition{{ColumnID: doing, Phase: rules.PhaseEnter, Kind: kind, Params: p}}
		return s
	}
	type card = rules.Card
	cases := []struct {
		kind   string
		params rules.ConditionParams
		fails  card
		passes card
	}{
		{rules.HasAssignee, rules.ConditionParams{}, card{}, card{AssigneeID: 3}},
		{rules.HasEstimate, rules.ConditionParams{}, card{}, card{HasEstimate: true}},
		{rules.HasDueDate, rules.ConditionParams{}, card{}, card{HasDueDate: true}},
		{rules.HasDescription, rules.ConditionParams{}, card{Description: "  "}, card{Description: "why"}},
		{rules.HasLabel, rules.ConditionParams{}, card{}, card{LabelIDs: []int64{5}}},
		{rules.HasLabel, rules.ConditionParams{LabelIDs: []int64{5, 6}}, card{LabelIDs: []int64{7}}, card{LabelIDs: []int64{7, 6}}},
		{rules.ChecklistComplete, rules.ConditionParams{}, card{ChecklistTotal: 2, ChecklistDone: 1}, card{ChecklistTotal: 2, ChecklistDone: 2}},
		{rules.ChecklistComplete, rules.ConditionParams{}, card{ChecklistTotal: 1}, card{}}, // no items passes
		{rules.BlockersDone, rules.ConditionParams{}, card{OpenBlockers: 1}, card{}},
		{rules.MinAttachments, rules.ConditionParams{Count: 2}, card{Attachments: 1}, card{Attachments: 2}},
	}
	for _, c := range cases {
		s := enter(c.kind, c.params)
		s.Card = c.fails
		vs := rules.Evaluate(member, move(todo, doing), s)
		if !reflect.DeepEqual(codes(vs), []string{"rules.condition." + c.kind}) {
			t.Errorf("%s fails: %v", c.kind, vs)
		} else if vs[0].Params["column"] != "Doing" || vs[0].Params["phase"] != "enter" {
			t.Errorf("%s params: %v", c.kind, vs[0].Params)
		}
		s.Card = c.passes
		if vs := rules.Evaluate(member, move(todo, doing), s); len(vs) != 0 {
			t.Errorf("%s passes: %v", c.kind, vs)
		}
	}
}

func TestExitConditionsAreTheSourceColumns(t *testing.T) {
	s := base()
	s.Conditions = []rules.Condition{{ColumnID: doing, Phase: rules.PhaseExit, Kind: rules.HasEstimate}}
	vs := rules.Evaluate(member, move(doing, review), s)
	if !reflect.DeepEqual(codes(vs), []string{"rules.condition.has_estimate"}) || vs[0].Params["phase"] != "exit" {
		t.Fatalf("exit: %v", vs)
	}
	if vs := rules.Evaluate(member, move(todo, doing), s); len(vs) != 0 {
		t.Errorf("entering a column with an exit condition: %v", vs)
	}
}

func TestColumnWIP(t *testing.T) {
	s := base()
	c := s.Columns[doing]
	c.WIPLimit, c.Count = 2, 1
	s.Columns[doing] = c
	if vs := rules.Evaluate(member, move(todo, doing), s); len(vs) != 0 {
		t.Errorf("one free place: %v", vs)
	}
	c.Count = 2
	s.Columns[doing] = c
	vs := rules.Evaluate(member, move(todo, doing), s)
	if !reflect.DeepEqual(codes(vs), []string{"rules.wip_column"}) || vs[0].Params["limit"] != "2" || vs[0].Params["column"] != "Doing" {
		t.Fatalf("full: %v", vs)
	}
	// A limit lowered below the count blocks entries, and nothing else.
	c.WIPLimit, c.Count = 1, 3
	s.Columns[doing] = c
	if vs := rules.Evaluate(member, move(doing, doing), s); len(vs) != 0 {
		t.Errorf("reordering inside an over-limit column: %v", vs)
	}
	if vs := rules.Evaluate(member, move(doing, review), s); len(vs) != 0 {
		t.Errorf("leaving an over-limit column: %v", vs)
	}
}

func TestPersonWIP(t *testing.T) {
	s := base()
	s.PersonWIPLimit = 2
	for _, id := range []int64{doing, review} {
		c := s.Columns[id]
		c.CountsPersonWIP = true
		s.Columns[id] = c
	}
	s.Card.AssigneeID = 3
	s.AssigneeWIP = 1
	if vs := rules.Evaluate(member, move(todo, doing), s); len(vs) != 0 {
		t.Errorf("under the limit: %v", vs)
	}
	s.AssigneeWIP = 2
	vs := rules.Evaluate(member, move(todo, doing), s)
	if !reflect.DeepEqual(codes(vs), []string{"rules.wip_person"}) || vs[0].Params["limit"] != "2" {
		t.Fatalf("at the limit: %v", vs)
	}
	if vs := rules.Evaluate(member, move(doing, review), s); len(vs) != 0 {
		t.Errorf("between counted columns: %v", vs)
	}
	s.Card.AssigneeID = 0
	if vs := rules.Evaluate(member, move(todo, doing), s); len(vs) != 0 {
		t.Errorf("unassigned card: %v", vs)
	}
}

func TestEveryViolationIsReportedInOrder(t *testing.T) {
	s := base()
	s.TransitionsMode = rules.ModeRestricted
	s.Permissions = []rules.Permission{{ToColumnID: doing, Subject: rules.SubjectTeamLead}}
	s.Conditions = []rules.Condition{
		{ColumnID: todo, Phase: rules.PhaseExit, Kind: rules.HasAssignee},
		{ColumnID: doing, Phase: rules.PhaseEnter, Kind: rules.HasEstimate},
		{ColumnID: doing, Phase: rules.PhaseEnter, Kind: rules.HasDueDate},
	}
	c := s.Columns[doing]
	c.WIPLimit, c.Count, c.CountsPersonWIP = 1, 1, true
	s.Columns[doing] = c
	s.PersonWIPLimit, s.AssigneeWIP, s.Card.AssigneeID = 1, 1, 0
	want := []string{"rules.transition", "rules.permission", "rules.condition.has_assignee",
		"rules.condition.has_estimate", "rules.condition.has_due_date", "rules.wip_column"}
	if got := codes(rules.Evaluate(member, move(todo, doing), s)); !reflect.DeepEqual(got, want) {
		t.Fatalf("codes = %v\nwant    %v", got, want)
	}
}

func TestReorderingInPlaceIsFree(t *testing.T) {
	s := base()
	s.TransitionsMode = rules.ModeRestricted
	s.Permissions = []rules.Permission{{ToColumnID: todo, Subject: rules.SubjectTeamLead}}
	s.Conditions = []rules.Condition{{ColumnID: todo, Phase: rules.PhaseEnter, Kind: rules.HasEstimate}}
	if vs := rules.Evaluate(member, move(todo, todo), s); len(vs) != 0 {
		t.Fatalf("reorder: %v", vs)
	}
}

func TestCreatingIsEnteringTheColumn(t *testing.T) {
	s := base()
	s.Conditions = []rules.Condition{{ColumnID: todo, Phase: rules.PhaseEnter, Kind: rules.HasEstimate}}
	c := s.Columns[todo]
	c.WIPLimit, c.Count = 1, 1
	s.Columns[todo] = c
	got := codes(rules.EvaluateCreate(todo, s))
	if !reflect.DeepEqual(got, []string{"rules.condition.has_estimate", "rules.wip_column"}) {
		t.Fatalf("create = %v", got)
	}
}

func TestAssigningChecksPersonWIP(t *testing.T) {
	s := base()
	s.PersonWIPLimit = 1
	c := s.Columns[doing]
	c.CountsPersonWIP = true
	s.Columns[doing] = c
	s.AssigneeWIP = 1
	s.Card.AssigneeID = 3
	if vs := rules.EvaluateAssign(doing, s); !reflect.DeepEqual(codes(vs), []string{"rules.wip_person"}) {
		t.Fatalf("assign in a counted column: %v", vs)
	}
	if vs := rules.EvaluateAssign(todo, s); len(vs) != 0 {
		t.Fatalf("assign in an uncounted column: %v", vs)
	}
	s.AssigneeWIP = 0
	if vs := rules.EvaluateAssign(doing, s); len(vs) != 0 {
		t.Fatalf("under the limit: %v", vs)
	}
}
