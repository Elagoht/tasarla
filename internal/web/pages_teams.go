package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/authz"
	"kanban/internal/store"
)

type teamsView struct {
	Teams     []store.TeamSummary
	CanCreate bool
}

type teamView struct {
	Me        int64
	Team      store.Team
	Boards    []store.Board
	Members   []store.Member
	Roles     []store.Role
	CanManage bool
}

func (h *handlers) teamsPage() *collage.Page {
	content := collage.NewFragment("teams-content", "pages/teams.html").
		WithDataHandler(collage.Load(h.loadTeams)).
		Required().
		Build()
	return paths(h.privatePage("teams", content), "/teams").
		WithAction(http.MethodPost, h.teamsPost).
		Dynamic().
		Build()
}

func (h *handlers) loadTeams(ctx context.Context, rc *collage.RenderContext) (teamsView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return teamsView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "teams.title"))
	teams, err := h.store.ListTeams(ctx, user)
	return teamsView{Teams: teams, CanCreate: user.IsAdmin}, err
}

func (h *handlers) teamsPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	if v.Value("op") != "create" {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if !user.IsAdmin {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v.Field("name").Required().MaxLen(100) // Required refuses white space alone
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	team, err := h.store.CreateTeam(ctx, strings.TrimSpace(v.Value("name")))
	if err != nil {
		return nil, err
	}
	flash.Add(rc, flash.Success, i18n.T(rc, "teams.created", "name", team.Name))
	return h.redirectToTeam(rc, team.ID)
}

func (h *handlers) redirectToTeam(rc *collage.RenderContext, id int64) (*collage.ActionResult, error) {
	target, err := rc.URL("team", map[string]string{"id": strconv.FormatInt(id, 10)})
	if err != nil {
		return nil, err
	}
	return collage.SeeOther(target), nil
}

func (h *handlers) teamPage() *collage.Page {
	content := collage.NewFragment("team-content", "pages/team.html").
		WithDataHandler(collage.Load(h.loadTeam)).
		Required().
		Build()
	return paths(h.privatePage("team", content), "/teams/{id}").
		WithAction(http.MethodPost, h.teamPost).
		Dynamic().
		Build()
}

// teamFor loads the team in the URL and the signed-in user's access to it. A
// team the user may not see is reported as not found, so its existence does
// not leak (spec §6).
func (h *handlers) teamFor(ctx context.Context, rc *collage.RenderContext) (store.Team, authz.TeamAccess, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return store.Team{}, authz.TeamAccess{}, err
	}
	id, err := strconv.ParseInt(rc.Param("id"), 10, 64)
	if err != nil {
		return store.Team{}, authz.TeamAccess{}, fmt.Errorf("team %q: %w", rc.Param("id"), collage.ErrNotFound)
	}
	team, err := h.store.Team(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.Team{}, authz.TeamAccess{}, fmt.Errorf("team %d: %w", id, collage.ErrNotFound)
	}
	if err != nil {
		return store.Team{}, authz.TeamAccess{}, err
	}
	role, err := h.store.MemberRole(ctx, team.ID, user.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.Team{}, authz.TeamAccess{}, err
	}
	access := authz.Team(user, role)
	if !access.CanView {
		return store.Team{}, authz.TeamAccess{}, fmt.Errorf("team %d for user %d: %w", id, user.ID, collage.ErrNotFound)
	}
	return team, access, nil
}

func (h *handlers) loadTeam(ctx context.Context, rc *collage.RenderContext) (teamView, error) {
	team, access, err := h.teamFor(ctx, rc)
	if err != nil {
		return teamView{}, err
	}
	rc.HoistTitle(team.Name)
	boards, err := h.store.BoardsOfTeam(ctx, team.ID)
	if err != nil {
		return teamView{}, err
	}
	members, err := h.store.Members(ctx, team.ID)
	user, _ := currentUser(ctx)
	return teamView{Me: user.ID, Team: team, Boards: boards, Members: members, Roles: store.Roles, CanManage: access.CanManage}, err
}

func (h *handlers) teamPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	team, access, err := h.teamFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if !access.CanManage {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v := validate.Form(rc)
	switch v.Value("op") {
	case "add_member":
		return h.addMember(ctx, rc, v, team)
	case "set_role", "remove_member":
		return h.changeMember(ctx, rc, v, team)
	case "create_board":
		return h.createBoard(ctx, rc, v, team)
	}
	return collage.NoContent(http.StatusBadRequest), nil
}

func (h *handlers) addMember(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, team store.Team) (*collage.ActionResult, error) {
	v.Field("new_email").Required().Email()
	v.Field("new_role").Required().OneOf(string(store.RoleLead), string(store.RoleMember))
	var user store.User
	if v.Valid() {
		var err error
		user, err = h.store.UserByEmail(ctx, v.Value("new_email"))
		switch {
		case errors.Is(err, store.ErrNotFound):
			v.Fail("new_email", i18n.T(rc, "team.user_not_found"))
		case errors.Is(err, store.ErrAmbiguousEmail):
			v.Fail("new_email", i18n.T(rc, "team.user_ambiguous"))
		case err != nil:
			return nil, err
		}
	}
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	role, _ := store.ParseRole(v.Value("new_role"))
	if err := h.store.AddMember(ctx, team.ID, user.ID, role); err != nil {
		return nil, err
	}
	flash.Add(rc, flash.Success, i18n.T(rc, "team.added", "name", user.Name))
	return h.redirectToTeam(rc, team.ID)
}

// changeMember changes the role of, or removes, a member of team. The user id
// comes from the form, so it is checked against this team: an id from another
// team is not found here (spec §6, IDOR).
func (h *handlers) changeMember(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, team store.Team) (*collage.ActionResult, error) {
	userID, err := strconv.ParseInt(v.Value("user_id"), 10, 64)
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if me, err := currentUser(ctx); err == nil && me.ID == userID {
		return collage.NoContent(http.StatusForbidden), nil // nobody changes their own role
	}
	target, err := h.store.UserByID(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}

	if v.Value("op") == "remove_member" {
		err = h.store.RemoveMember(ctx, team.ID, userID)
		if err == nil {
			flash.Add(rc, flash.Success, i18n.T(rc, "team.removed", "name", target.Name))
		}
	} else {
		role, ok := store.ParseRole(v.Value("role"))
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.SetRole(ctx, team.ID, userID, role)
		if err == nil {
			flash.Add(rc, flash.Success, i18n.T(rc, "team.role_changed", "name", target.Name))
		}
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.redirectToTeam(rc, team.ID)
}

// createBoard adds a board with three columns named in the creator's language.
func (h *handlers) createBoard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, team store.Team) (*collage.ActionResult, error) {
	v.Field("board_name").Required().MaxLen(100)
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	columns := []string{
		i18n.T(rc, "board.default_columns.todo"),
		i18n.T(rc, "board.default_columns.doing"),
		i18n.T(rc, "board.default_columns.done"),
	}
	board, err := h.store.CreateBoard(ctx, team.ID, strings.TrimSpace(v.Value("board_name")), columns)
	if err != nil {
		return nil, err
	}
	return h.redirectTo(rc, "board", "id", strconv.FormatInt(board.ID, 10))
}
