package web

import (
	"context"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type homeView struct {
	Teams []homeTeam
}

type homeTeam struct {
	Team   store.TeamSummary
	Boards []store.Board
}

func (h *handlers) homePage() *collage.Page {
	content := collage.NewFragment("home-content", "pages/home.html").
		WithData(collage.Load(h.loadHome)).
		Required().
		Build()
	return paths(h.privatePage("home", content), "/").Dynamic().Build()
}

func (h *handlers) loadHome(ctx context.Context, rc *collage.RenderContext) (homeView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return homeView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "home.title"))
	teams, err := h.store.TeamsOf(ctx, user.ID)
	if err != nil {
		return homeView{}, err
	}
	var view homeView
	for _, t := range teams {
		boards, err := h.store.BoardsOfTeam(ctx, t.Team.ID)
		if err != nil {
			return homeView{}, err
		}
		view.Teams = append(view.Teams, homeTeam{Team: t, Boards: boards})
	}
	return view, nil
}
