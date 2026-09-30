package web

import (
	"context"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type homeView struct {
	Teams []store.TeamSummary
}

func (h *handlers) homePage() *collage.Page {
	content := collage.NewFragment("home-content", "pages/home.html").
		WithDataHandler(collage.Load(h.loadHome)).
		Required().
		Build()
	return paths(privatePage("home", content), "/").Dynamic().Build()
}

func (h *handlers) loadHome(ctx context.Context, rc *collage.RenderContext) (homeView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return homeView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "home.title"))
	teams, err := h.store.TeamsOf(ctx, user.ID)
	return homeView{Teams: teams}, err
}
