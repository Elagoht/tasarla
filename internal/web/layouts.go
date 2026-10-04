package web

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	flash "github.com/Elagoht/collage-flash"
	session "github.com/Elagoht/collage-session"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/config"
	"kanban/internal/store"
)

var errNoUser = errors.New("web: a guarded page has no signed-in user")

// currentUser is the signed-in user; on a page under appLayout there is always one.
func currentUser(ctx context.Context) (store.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return store.User{}, errNoUser
	}
	return u, nil
}

// paths gives a page one path in every locale: /teams and /en/teams.
func paths(b *collage.PageBuilder, pattern string) *collage.PageBuilder {
	for _, l := range config.Locales {
		b = b.WithPath(l, pattern)
	}
	return b
}

type baseView struct {
	Locale string
}

// baseLayout is the document: <html>, the head, the language switcher. It
// depends only on the URL's locale, so it does not make a page dynamic.
func baseLayout() *collage.Fragment {
	return collage.NewFragment("base", "layouts/base.html").
		WithTitle("Pusula").
		WithDataHandler(collage.Load(func(_ context.Context, rc *collage.RenderContext) (baseView, error) {
			return baseView{Locale: rc.Locale}, nil
		})).
		Static().
		Build()
}

type appView struct {
	User     store.User
	Hue      int64
	Initial  string
	Locale   string
	Page     string // the registered name of the page being shown
	Wide     bool   // the board, which can be widened past the reading width
	Expanded bool   // ...and its viewer did
	Teams    []navTeam
	BoardID  int64
	Flashes  []flashView
}

type navTeam struct {
	Team   store.TeamSummary
	Boards []navBoard
}

type navBoard struct {
	Board  store.Board
	Color  string
	Active bool
}

type flashView struct {
	Kind string
	Text string
}

// boardColor gives each board and column a colour from the label palette.
func boardColor(i int) string { return labelPalette[i%len(labelPalette)] }

// initial is the first letter of a name, for avatars.
func initial(name string) string {
	for _, r := range strings.TrimSpace(name) {
		return strings.ToUpper(string(r))
	}
	return "?"
}

// appLayout is every page for signed-in readers: the sidebar with the
// reader's teams and boards. Its guard sends anyone else to /login; pages
// under it must be Dynamic (spec §2.1).
func (h *handlers) appLayout() *collage.Fragment {
	return collage.NewFragment("app", "layouts/app.html").
		WithSlotFragment("badge", h.badge).
		WithGuard(h.signedInInOwnLocale(session.RequireUser("/login"))).
		WithDataHandler(collage.Load(h.loadApp)).
		Required().
		Build()
}

// signedInInOwnLocale is signedIn, and then shows the interface in the
// account's language: a page asked for in another locale is sent to the same
// path in the account's (spec: the account language, set on My settings).
// Only page loads are moved; a form or a script's fetch is answered where it
// was sent.
func (h *handlers) signedInInOwnLocale(signedIn collage.GuardFunc) collage.GuardFunc {
	return func(ctx context.Context, r *http.Request) (*collage.GuardDecision, error) {
		if d, err := signedIn(ctx, r); d != nil || err != nil {
			return d, err
		}
		u, ok := auth.UserFrom(ctx)
		if !ok || (r.Method != http.MethodGet && r.Method != http.MethodHead) || r.Header.Get(collage.FetchHeader) != "" {
			return nil, nil
		}
		current, rest := h.defaultLocale, r.URL.Path
		for _, l := range config.Locales {
			if l != h.defaultLocale && (rest == "/"+l || strings.HasPrefix(rest, "/"+l+"/")) {
				current, rest = l, strings.TrimPrefix(rest, "/"+l)
			}
		}
		if u.Locale == current || !slices.Contains(config.Locales, u.Locale) {
			return nil, nil
		}
		target := rest
		if u.Locale != h.defaultLocale {
			target = strings.TrimSuffix("/"+u.Locale+rest, "/") // "/en", not "/en/"
		}
		if target == "" {
			target = "/"
		}
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		return &collage.GuardDecision{Status: http.StatusSeeOther, Location: target}, nil
	}
}

func (h *handlers) loadApp(ctx context.Context, rc *collage.RenderContext) (appView, error) {
	u, err := currentUser(ctx)
	if err != nil {
		return appView{}, err
	}
	v := appView{User: u, Hue: u.ID % 8, Initial: initial(u.Name), Locale: rc.Locale}
	if rc.Page != nil {
		v.Page = rc.Page.Name
	}
	v.Wide = v.Page == "board"
	v.Expanded = v.Wide && boardWide(rc.Request)
	if id, err := parseID(rc.Param("id")); err == nil && (strings.HasPrefix(v.Page, "board") || v.Page == "card") {
		v.BoardID = id
	}
	teams, err := h.store.TeamsOf(ctx, u.ID)
	if err != nil {
		return v, err
	}
	for _, t := range teams {
		boards, err := h.store.BoardsOfTeam(ctx, t.Team.ID)
		if err != nil {
			return v, err
		}
		nt := navTeam{Team: t}
		for _, b := range boards {
			nt.Boards = append(nt.Boards, navBoard{Board: b, Color: boardColor(int(b.ID)), Active: b.ID == v.BoardID})
		}
		v.Teams = append(v.Teams, nt)
	}
	for _, m := range flash.Take(rc) {
		v.Flashes = append(v.Flashes, flashView{Kind: m.Kind, Text: m.Text})
	}
	return v, nil
}

// privatePage starts a page under both layouts.
func (h *handlers) privatePage(name string, content *collage.Fragment) *collage.PageBuilder {
	return collage.NewPage(name).WithLayouts(baseLayout(), h.appLayout()).WithContent(content)
}
