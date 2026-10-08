package web

import (
	"context"
	"net/http"
	"slices"
	"strings"

	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
)

// destructive lists the operations that ask before they act (spec §2.6).
// set_disabled asks only when it disables.
var destructive = []string{
	"archive", "comment_delete", "attachment_delete", "label_delete", "role_delete",
	"rule_delete", "archive_board", "remove_member", "set_disabled", "template_delete",
}

type confirmView struct {
	Question string
	// Hint says what follows: undone only from the archive, or not at all.
	Hint   string
	Action string
	Fields []confirmField
	Back   string
}

type confirmField struct {
	Name  string
	Value string
}

var confirmKey = collage.NewKey[confirmView]("confirm")

// confirmFirst answers a destructive operation posted without confirm=1 with
// a page asking for it; confirm.js adds confirm=1 after its own dialog, so
// with a script this page is never seen. It returns nil when the operation
// may go ahead.
func (h *handlers) confirmFirst(rc *collage.RenderContext, v *validate.Validator) *collage.ActionResult {
	op := v.Value("op")
	if !slices.Contains(destructive, op) || v.Value("confirm") == "1" || (op == "set_disabled" && v.Value("value") != "1") {
		return nil
	}
	hint := i18n.T(rc, "confirm.irreversible")
	if op == "archive" || op == "archive_board" {
		hint = i18n.T(rc, "confirm.archive_hint")
	}
	view := confirmView{Question: i18n.T(rc, "confirm."+op), Hint: hint, Action: rc.Request.URL.RequestURI(), Back: rc.Request.URL.Path}
	// A local path goes through the login's own check, which refuses "//host".
	if ref := rc.Request.Referer(); (strings.HasPrefix(ref, "/") && auth.SafeNext(ref) == ref) || sameOrigin(rc.Request, ref) {
		view.Back = ref
	}
	keys := make([]string, 0, len(rc.Request.PostForm))
	for k := range rc.Request.PostForm {
		if k != "_csrf" && k != "confirm" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		for _, val := range rc.Request.PostForm[k] {
			view.Fields = append(view.Fields, confirmField{Name: k, Value: val})
		}
	}
	confirmKey.Set(rc, view)
	return collage.RenderPage(h.confirm)
}

// sameOrigin reports whether ref points at the request's own host.
func sameOrigin(r *http.Request, ref string) bool {
	for _, scheme := range []string{"http://", "https://"} {
		if strings.HasPrefix(ref, scheme+r.Host+"/") {
			return true
		}
	}
	return false
}

// confirmPage has no path: actions answer with it.
func (h *handlers) confirmPage() *collage.Page {
	content := collage.NewFragment("confirm-content", "pages/confirm.html").
		WithData(collage.Load(func(_ context.Context, rc *collage.RenderContext) (confirmView, error) {
			v, _ := confirmKey.Get(rc)
			rc.HoistTitle(i18n.T(rc, "confirm.title"))
			return v, nil
		})).
		Required().
		Build()
	return h.privatePage("confirm", content).Dynamic().Build()
}
