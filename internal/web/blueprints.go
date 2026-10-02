package web

import (
	"fmt"
	"strconv"
	"strings"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/blueprint"
	"kanban/internal/rules"
	"kanban/internal/store"
)

// blueprintView is one ready-made board on the new-board page: a line in the
// list, and the preview of what it builds.
type blueprintView struct {
	Key, Name, Summary string
	Has                string // the parts it holds, for blueprint-form.js
	Checked            bool   // the one the form comes back with
	Columns            []previewColumn
	Labels             []store.PlanLabel
	Templates          []previewTemplate
	Rules              []string // each rule as a sentence
}

type previewColumn struct {
	Name string
	WIP  *int
}

type previewTemplate struct {
	Name     string
	Schedule string // "" when it does not recur
}

func blueprintViews(rc *collage.RenderContext) []blueprintView {
	// A refused form comes back with what was sent; an unknown key falls back
	// to the default so one blueprint is always chosen.
	chosen := ""
	if rc.Request != nil && rc.Request.PostForm != nil {
		chosen = rc.Request.PostForm.Get("blueprint")
	}
	if _, ok := blueprint.Find(chosen); !ok {
		chosen = blueprint.Default
	}
	tr := func(k string) string { return i18n.T(rc, k) }
	var out []blueprintView
	for _, bp := range blueprint.All() {
		plan := blueprint.Render(bp, tr)
		v := blueprintView{Key: bp.Key, Name: tr(bp.NameKey()), Summary: tr(bp.SummaryKey()),
			Has: hasList(bp.Has()), Checked: bp.Key == chosen, Labels: plan.Labels, Rules: ruleSentences(rc, plan)}
		for _, c := range plan.Columns {
			v.Columns = append(v.Columns, previewColumn{Name: c.Name, WIP: c.WIP})
		}
		for _, t := range plan.Templates {
			v.Templates = append(v.Templates, previewTemplate{Name: t.Name, Schedule: scheduleText(rc, t.Schedule)})
		}
		out = append(out, v)
	}
	return out
}

func hasList(h blueprint.Options) string {
	var has []string
	for _, p := range []struct {
		on   bool
		name string
	}{{h.WIP, "wip"}, {h.Labels, "labels"}, {h.Templates, "templates"}, {h.Recurring, "recurring"}, {h.Rules, "rules"}} {
		if p.on {
			has = append(has, p.name)
		}
	}
	return strings.Join(has, " ")
}

// ruleSentences writes a plan's rules as the board settings write them.
func ruleSentences(rc *collage.RenderContext, p store.BoardPlan) []string {
	col := func(i int) string { return p.Columns[i].Name }
	var out []string
	for _, pm := range p.Permissions {
		out = append(out, i18n.T(rc, "settings.sentence.permission", "column", col(pm.ToIndex), "who", i18n.T(rc, "settings.subjects."+pm.Subject)))
	}
	for _, c := range p.Conditions {
		phrase := i18n.T(rc, "settings.phrase."+c.Kind)
		if c.Kind == rules.HasLabel {
			phrase = i18n.T(rc, "settings.phrase.has_any_label")
		}
		out = append(out, i18n.T(rc, "settings.sentence."+c.Phase, "column", col(c.ColumnIndex), "condition", phrase))
	}
	if p.PersonWIP != nil {
		var counted []string
		for _, c := range p.Columns {
			if c.CountsPersonWIP {
				counted = append(counted, c.Name)
			}
		}
		out = append(out, i18n.T(rc, "settings.sentence.person_wip", "columns", listOf(rc, counted), "limit", strconv.Itoa(*p.PersonWIP)))
	}
	return out
}

// scheduleText says when a template recurs: "Her hafta · Cum · 16:00".
func scheduleText(rc *collage.RenderContext, s store.Schedule) string {
	if s.Kind == "" {
		return ""
	}
	parts := []string{i18n.T(rc, "settings.template_schedule_"+s.Kind)}
	switch s.Kind {
	case "weekly":
		var days []string
		for d := 0; d < 7; d++ {
			if s.Weekdays&(1<<d) != 0 {
				days = append(days, i18n.T(rc, "weekdays."+strconv.Itoa(d)))
			}
		}
		parts = append(parts, strings.Join(days, ", "))
	case "monthly":
		parts = append(parts, strconv.Itoa(s.MonthDay))
	}
	return strings.Join(append(parts, fmt.Sprintf("%02d:%02d", s.Hour, s.Minute)), " · ")
}

// includeOptions reads the form's include boxes: a form that had them sends
// include_present, and then only the ticked ones come; any other request
// brings everything.
func includeOptions(rc *collage.RenderContext, present string) blueprint.Options {
	if present == "" {
		return blueprint.AllOptions()
	}
	return blueprint.OptionsFrom(rc.Request.PostForm["include"])
}
