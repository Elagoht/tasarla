package web

import (
	"errors"
	"strings"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/rules"
	"kanban/internal/store"
)

// actor is the signed-in user as the rules see them on this board.
func (bc boardContext) actor() rules.Actor {
	return rules.Actor{UserID: bc.User.ID, IsAdmin: bc.User.IsAdmin, TeamRole: string(bc.Role)}
}

// violationMessages translates the rules err broke into the reader's language,
// or returns nil when err is not a rule error.
func violationMessages(rc *collage.RenderContext, err error) []string {
	var re *store.RuleError
	if !errors.As(err, &re) {
		return nil
	}
	return ruleMessages(rc, re.Violations)
}

// ruleMessages translates violations into the reader's language. A code that
// is not a condition's is its own catalog key.
func ruleMessages(rc *collage.RenderContext, vs []rules.Violation) []string {
	messages := make([]string, 0, len(vs))
	for _, v := range vs {
		key := v.Code
		if kind, ok := strings.CutPrefix(v.Code, "rules.condition."); ok {
			if kind == rules.HasLabel && v.Params["labels"] == "" {
				kind = "has_any_label"
			}
			key = "rules." + v.Params["phase"] + "." + kind
		}
		p := v.Params
		messages = append(messages, i18n.T(rc, key,
			"column", p["column"], "from", p["from"], "to", p["to"],
			"limit", p["limit"], "labels", p["labels"], "count", p["count"]))
	}
	return messages
}
