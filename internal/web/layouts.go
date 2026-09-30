package web

import (
	"context"
	"errors"

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
		WithTitle("Kanban").
		WithDataHandler(collage.Load(func(_ context.Context, rc *collage.RenderContext) (baseView, error) {
			return baseView{Locale: rc.Locale}, nil
		})).
		Static().
		Build()
}

type appView struct {
	User store.User
}

// appLayout is every page for signed-in readers. Its guard sends anyone else
// to /login; pages under it must be Dynamic (spec §2.1).
func appLayout() *collage.Fragment {
	return collage.NewFragment("app", "layouts/app.html").
		WithGuard(session.RequireUser("/login")).
		WithDataHandler(collage.Load(func(ctx context.Context, _ *collage.RenderContext) (appView, error) {
			u, err := currentUser(ctx)
			return appView{User: u}, err
		})).
		Required().
		Build()
}

// privatePage starts a page under both layouts.
func privatePage(name string, content *collage.Fragment) *collage.PageBuilder {
	return collage.NewPage(name).WithLayouts(baseLayout(), appLayout()).WithContent(content)
}
