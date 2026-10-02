package blueprint

import (
	"strconv"

	"kanban/internal/store"
)

// Render is bp with every text in t's language and everything kept.
func Render(bp Blueprint, t func(key string) string) store.BoardPlan {
	var p store.BoardPlan
	for i, c := range bp.Columns {
		col := store.PlanColumn{Name: t(bp.key("columns." + c.Key)), AllowCreate: i == 0, Done: i == len(bp.Columns)-1,
			CountsPersonWIP: c.CountsPersonWIP}
		if c.WIP > 0 {
			col.WIP = intp(c.WIP)
		}
		p.Columns = append(p.Columns, col)
	}
	for _, l := range bp.Labels {
		p.Labels = append(p.Labels, store.PlanLabel{Name: t(bp.key("labels." + l.Key)), Color: l.Color})
	}
	for _, c := range bp.Templates {
		k := "templates." + c.Key + "."
		tp := store.PlanTemplate{Name: t(bp.key(k + "name")), Title: t(bp.key(k + "title")),
			Description: t(bp.key(k + "description")), ColumnIndex: 0, LabelIndexes: c.Labels, Schedule: c.Schedule}
		for n := 1; n <= c.Checklist; n++ {
			tp.Checklist = append(tp.Checklist, t(bp.key(k+"checklist."+strconv.Itoa(n))))
		}
		if c.Priority > 0 {
			pr := c.Priority
			tp.Priority = &pr
		}
		if c.DueInDays > 0 {
			tp.DueInDays = intp(c.DueInDays)
		}
		p.Templates = append(p.Templates, tp)
	}
	if bp.Rules.PersonWIP > 0 {
		p.PersonWIP = intp(bp.Rules.PersonWIP)
	}
	for _, r := range bp.Rules.Permissions {
		p.Permissions = append(p.Permissions, store.PlanPermission{ToIndex: r.To, Subject: r.Subject})
	}
	for _, r := range bp.Rules.Conditions {
		p.Conditions = append(p.Conditions, store.PlanCondition{ColumnIndex: r.Column, Phase: r.Phase, Kind: r.Kind})
	}
	return p
}

// Trim keeps the parts o brings: without WIP no column has a limit and there
// is no person limit; without labels templates carry none; without templates
// nothing recurs; without rules there are no permissions, conditions or
// person limit. The columns always stay.
func Trim(p store.BoardPlan, o Options) store.BoardPlan {
	out := store.BoardPlan{Columns: make([]store.PlanColumn, len(p.Columns))}
	copy(out.Columns, p.Columns)
	if !o.WIP {
		for i := range out.Columns {
			out.Columns[i].WIP = nil
		}
	}
	if o.Labels {
		out.Labels = p.Labels
	}
	if o.Templates {
		for _, t := range p.Templates {
			if !o.Labels {
				t.LabelIndexes = nil
			}
			if !o.Recurring {
				t.Schedule = store.Schedule{}
			}
			out.Templates = append(out.Templates, t)
		}
	}
	if o.Rules {
		out.Permissions = p.Permissions
		out.Conditions = p.Conditions
		if o.WIP {
			out.PersonWIP = p.PersonWIP
		}
	}
	if out.PersonWIP == nil {
		for i := range out.Columns {
			out.Columns[i].CountsPersonWIP = false
		}
	}
	return out
}

// Plan is bp in t's language with the parts o brings.
func Plan(bp Blueprint, t func(key string) string, o Options) store.BoardPlan {
	return Trim(Render(bp, t), o)
}

func intp(n int) *int { return &n }
