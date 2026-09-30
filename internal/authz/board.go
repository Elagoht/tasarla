package authz

import "kanban/internal/store"

// BoardAccess is what a user may do on one board (spec §6).
type BoardAccess struct {
	CanView   bool // see the board and its cards
	CanEdit   bool // create, change and move cards
	CanManage bool // change the board's settings and rules
}

// Board is user's access to a board of a team in which they hold role ("" if
// none). Access comes from team membership; there is no per-board membership.
func Board(user store.User, role store.Role) BoardAccess {
	switch {
	case user.IsAdmin, role == store.RoleLead:
		return BoardAccess{CanView: true, CanEdit: true, CanManage: true}
	case role == store.RoleMember:
		return BoardAccess{CanView: true, CanEdit: true}
	default:
		return BoardAccess{}
	}
}
