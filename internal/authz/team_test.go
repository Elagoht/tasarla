package authz_test

import (
	"testing"

	"kanban/internal/authz"
	"kanban/internal/store"
)

func TestTeam(t *testing.T) {
	admin := store.User{IsAdmin: true}
	user := store.User{}
	cases := []struct {
		name string
		user store.User
		role store.Role
		want authz.TeamAccess
	}{
		{"admin, not a member", admin, "", authz.TeamAccess{CanView: true, CanManage: true}},
		{"lead", user, store.RoleLead, authz.TeamAccess{CanView: true, CanManage: true}},
		{"member", user, store.RoleMember, authz.TeamAccess{CanView: true}},
		{"outsider", user, "", authz.TeamAccess{}},
	}
	for _, c := range cases {
		if got := authz.Team(c.user, c.role); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}
