package web

import (
	"strings"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/blueprint"
)

// blueprintCard is one ready-made board on the create form.
type blueprintCard struct {
	Key, Name, Summary, Columns string
	Counts                      blueprint.Counts
	Has                         string // the parts it holds, for blueprint-form.js
}

func blueprintCards(rc *collage.RenderContext) []blueprintCard {
	var out []blueprintCard
	for _, bp := range blueprint.All() {
		keys := bp.ColumnKeys()
		names := make([]string, 0, len(keys))
		for _, k := range keys {
			names = append(names, i18n.T(rc, k))
		}
		h := bp.Has()
		var has []string
		for _, p := range []struct {
			on   bool
			name string
		}{{h.WIP, "wip"}, {h.Labels, "labels"}, {h.Templates, "templates"}, {h.Recurring, "recurring"}, {h.Rules, "rules"}} {
			if p.on {
				has = append(has, p.name)
			}
		}
		out = append(out, blueprintCard{Key: bp.Key, Name: i18n.T(rc, bp.NameKey()), Summary: i18n.T(rc, bp.SummaryKey()),
			Columns: strings.Join(names, " → "), Counts: bp.Counts(), Has: strings.Join(has, " ")})
	}
	return out
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
