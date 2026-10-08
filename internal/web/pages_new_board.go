package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/blueprint"
	"kanban/internal/store"
)

// newBoardView is the new-board page: a name, a ready-made board to start
// from, and the parts of it to bring along.
type newBoardView struct {
	Team       store.Team
	Blueprints []blueprintView
}

func (h *handlers) newBoardPage() *collage.Page {
	content := collage.NewFragment("new-board-content", "pages/new_board.html").
		WithData(collage.Load(h.loadNewBoard)).
		Required().
		Build()
	return paths(h.privatePage("team-new-board", content), "/teams/{id}/boards/new").
		WithAction(http.MethodPost, h.newBoardPost).
		Dynamic().
		Build()
}

// managedTeamFor is teamFor for the team's leads: anyone else is told the
// page does not exist.
func (h *handlers) managedTeamFor(ctx context.Context, rc *collage.RenderContext) (store.Team, error) {
	team, access, err := h.teamFor(ctx, rc)
	if err != nil {
		return team, err
	}
	if !access.CanManage {
		return store.Team{}, fmt.Errorf("new board in team %d: %w", team.ID, collage.ErrNotFound)
	}
	return team, nil
}

func (h *handlers) loadNewBoard(ctx context.Context, rc *collage.RenderContext) (newBoardView, error) {
	team, err := h.managedTeamFor(ctx, rc)
	if err != nil {
		return newBoardView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "board.new_board"))
	return newBoardView{Team: team, Blueprints: blueprintViews(rc)}, nil
}

// newBoardPost builds a board from the chosen blueprint, in the creator's
// language, with the parts they brought along.
func (h *handlers) newBoardPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	team, err := h.managedTeamFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	if badText(rc) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	v.Field("board_name").Required().MaxLen(100)
	key := v.Value("blueprint")
	if key == "" {
		key = blueprint.Default
	}
	bp, ok := blueprint.Find(key)
	if !ok {
		v.Fail("blueprint", i18n.T(rc, "board.blueprint_unknown"))
	}
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	plan := blueprint.Plan(bp, func(k string) string { return i18n.T(rc, k) }, includeOptions(rc, v.Value("include_present")))
	board, err := h.store.CreateBoardFromPlan(ctx, team.ID, strings.TrimSpace(v.Value("board_name")), plan, u.ID)
	if err != nil {
		return nil, err
	}
	return h.redirectTo(rc, "board", "id", strconv.FormatInt(board.ID, 10))
}
