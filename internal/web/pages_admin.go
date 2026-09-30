package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type adminUsersView struct {
	Users []store.User
	Me    int64
}

func (h *handlers) adminUsersPage() *collage.Page {
	content := collage.NewFragment("admin-users-content", "pages/admin_users.html").
		WithDataHandler(collage.Load(h.loadAdminUsers)).
		Required().
		Build()
	return paths(privatePage("admin-users", content), "/admin/users").
		WithAction(http.MethodPost, h.adminUsersPost).
		Dynamic().
		Build()
}

// currentAdmin is the signed-in user if they are an admin; anyone else is told
// the page does not exist.
func currentAdmin(ctx context.Context) (store.User, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return store.User{}, err
	}
	if !user.IsAdmin {
		return store.User{}, fmt.Errorf("admin page for user %d: %w", user.ID, collage.ErrNotFound)
	}
	return user, nil
}

func (h *handlers) loadAdminUsers(ctx context.Context, rc *collage.RenderContext) (adminUsersView, error) {
	admin, err := currentAdmin(ctx)
	if err != nil {
		return adminUsersView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "users.title"))
	users, err := h.store.Users(ctx)
	return adminUsersView{Users: users, Me: admin.ID}, err
}

func (h *handlers) adminUsersPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	admin, err := currentAdmin(ctx)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	userID, err := strconv.ParseInt(v.Value("user_id"), 10, 64)
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	on := v.Value("value") == "1"

	switch v.Value("op") {
	case "set_admin":
		err = h.store.SetAdmin(ctx, userID, on)
	case "set_disabled":
		if on && userID == admin.ID {
			flash.Add(rc, flash.Error, i18n.T(rc, "users.self_disable"))
			return h.redirectTo(rc, "admin-users")
		}
		err = h.store.SetDisabled(ctx, userID, on)
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return collage.NoContent(http.StatusNotFound), nil
	case errors.Is(err, store.ErrLastAdmin):
		flash.Add(rc, flash.Error, i18n.T(rc, "users.last_admin"))
	case err != nil:
		return nil, err
	default:
		flash.Add(rc, flash.Success, i18n.T(rc, "users.saved"))
	}
	return h.redirectTo(rc, "admin-users")
}

func (h *handlers) redirectTo(rc *collage.RenderContext, page string) (*collage.ActionResult, error) {
	target, err := rc.URL(page, nil)
	if err != nil {
		return nil, err
	}
	return collage.SeeOther(target), nil
}
