package authz_test

import (
	"testing"

	"kanban/internal/authz"
	"kanban/internal/store"
)

func TestBoard(t *testing.T) {
	admin := store.User{IsAdmin: true}
	user := store.User{}
	cases := []struct {
		name string
		user store.User
		role store.Role
		want authz.BoardAccess
	}{
		{"admin, not in the team", admin, "", authz.BoardAccess{CanView: true, CanEdit: true, CanManage: true}},
		{"lead", user, store.RoleLead, authz.BoardAccess{CanView: true, CanEdit: true, CanManage: true}},
		{"member", user, store.RoleMember, authz.BoardAccess{CanView: true, CanEdit: true}},
		{"outsider", user, "", authz.BoardAccess{}},
	}
	for _, c := range cases {
		if got := authz.Board(c.user, c.role); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}
