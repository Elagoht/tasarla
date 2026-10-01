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
	return paths(h.privatePage("admin-users", content), "/admin/users").
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
	if badText(rc) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if res := h.confirmFirst(rc, v); res != nil {
		return res, nil
	}
	userID, err := strconv.ParseInt(v.Value("user_id"), 10, 64)
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	on := v.Value("value") == "1"
	// Nobody changes their own role or locks themselves out; the page shows
	// those buttons disabled, and a crafted post is refused.
	if userID == admin.ID {
		return collage.NoContent(http.StatusForbidden), nil
	}

	switch v.Value("op") {
	case "set_admin":
		err = h.store.SetAdmin(ctx, userID, on)
	case "set_disabled":
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

// redirectTo answers with a 303 to a named page; params are name, value pairs.
func (h *handlers) redirectTo(rc *collage.RenderContext, page string, params ...string) (*collage.ActionResult, error) {
	values := map[string]string{}
	for i := 0; i+1 < len(params); i += 2 {
		values[params[i]] = params[i+1]
	}
	target, err := rc.URL(page, values)
	if err != nil {
		return nil, err
	}
	return collage.SeeOther(target), nil
}
