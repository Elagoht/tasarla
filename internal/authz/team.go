// Package authz decides who may do what. It reads no database; callers hand
// it what they loaded.
package authz

import "kanban/internal/store"

// TeamAccess is what a user may do with one team.
type TeamAccess struct {
	CanView   bool // see the team and its members
	CanManage bool // add and remove members, change roles
}

// Team is user's access to a team in which they hold role ("" if none).
func Team(user store.User, role store.Role) TeamAccess {
	switch {
	case user.IsAdmin, role == store.RoleLead:
		return TeamAccess{CanView: true, CanManage: true}
	case role == store.RoleMember:
		return TeamAccess{CanView: true}
	default:
		return TeamAccess{}
	}
}
