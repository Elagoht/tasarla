package store

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

// Role is what a member may do in a team.
type Role string

const (
	RoleLead   Role = "lead"
	RoleMember Role = "member"
)

// Roles lists every role, lead first.
var Roles = []Role{RoleLead, RoleMember}

// ParseRole reads a role from a form value.
func ParseRole(s string) (Role, bool) {
	r := Role(s)
	return r, slices.Contains(Roles, r)
}

// Team is a group of people who share boards.
type Team struct {
	ID         int64
	Name       string
	ArchivedAt *time.Time
	CreatedAt  time.Time
}

// TeamSummary is a team as a list shows it. Role is the viewer's, "" when the
// viewer is not a member.
type TeamSummary struct {
	Team        Team
	MemberCount int
	Role        Role
}

// Member is a user's membership in a team.
type Member struct {
	User User
	Role Role
}

func scanTeam(row pgx.Row) (Team, error) {
	var t Team
	err := row.Scan(&t.ID, &t.Name, &t.ArchivedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Team{}, ErrNotFound
	}
	return t, err
}

// CreateTeam adds a team with no members.
func (s *Store) CreateTeam(ctx context.Context, name string) (Team, error) {
	return scanTeam(s.pool.QueryRow(ctx,
		`INSERT INTO teams (name) VALUES ($1) RETURNING id, name, archived_at, created_at`, name))
}

// Team returns one team.
func (s *Store) Team(ctx context.Context, id int64) (Team, error) {
	return scanTeam(s.pool.QueryRow(ctx,
		`SELECT id, name, archived_at, created_at FROM teams WHERE id = $1`, id))
}

// ListTeams returns the teams viewer may see: every team for an admin, the
// teams they belong to for anyone else.
func (s *Store) ListTeams(ctx context.Context, viewer User) ([]TeamSummary, error) {
	return s.listTeams(ctx, viewer.ID, viewer.IsAdmin)
}

// TeamsOf returns the teams userID belongs to.
func (s *Store) TeamsOf(ctx context.Context, userID int64) ([]TeamSummary, error) {
	return s.listTeams(ctx, userID, false)
}

func (s *Store) listTeams(ctx context.Context, userID int64, all bool) ([]TeamSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name, t.archived_at, t.created_at,
		       (SELECT count(*) FROM team_members c WHERE c.team_id = t.id),
		       coalesce(m.role, '')
		FROM teams t
		LEFT JOIN team_members m ON m.team_id = t.id AND m.user_id = $1
		WHERE t.archived_at IS NULL AND ($2 OR m.user_id IS NOT NULL)
		ORDER BY lower(t.name), t.id`, userID, all)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []TeamSummary
	for rows.Next() {
		var ts TeamSummary
		var role string
		if err := rows.Scan(&ts.Team.ID, &ts.Team.Name, &ts.Team.ArchivedAt, &ts.Team.CreatedAt, &ts.MemberCount, &role); err != nil {
			return nil, err
		}
		ts.Role = Role(role)
		teams = append(teams, ts)
	}
	return teams, rows.Err()
}

// Members returns a team's members, leads first, then by name.
func (s *Store) Members(ctx context.Context, teamID int64) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.issuer, u.subject, u.email, u.name, u.locale, u.is_admin, u.disabled_at, u.created_at, m.role
		FROM team_members m JOIN users u ON u.id = m.user_id
		WHERE m.team_id = $1
		ORDER BY m.role = 'lead' DESC, lower(u.name), u.id`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []Member
	for rows.Next() {
		var m Member
		var role string
		u := &m.User
		if err := rows.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.Name, &u.Locale, &u.IsAdmin, &u.DisabledAt, &u.CreatedAt, &role); err != nil {
			return nil, err
		}
		m.Role = Role(role)
		members = append(members, m)
	}
	return members, rows.Err()
}

// MemberRole returns userID's role in teamID, or ErrNotFound when they are not
// a member.
func (s *Store) MemberRole(ctx context.Context, teamID, userID int64) (Role, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return Role(role), err
}

// AddMember adds userID to teamID, or changes their role if they are a member.
func (s *Store) AddMember(ctx context.Context, teamID, userID int64, role Role) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`, teamID, userID, string(role))
	return err
}

// SetRole changes a member's role.
func (s *Store) SetRole(ctx context.Context, teamID, userID int64, role Role) error {
	return exactlyOne(s.pool.Exec(ctx,
		`UPDATE team_members SET role = $3 WHERE team_id = $1 AND user_id = $2`, teamID, userID, string(role)))
}

// RemoveMember takes userID out of teamID.
func (s *Store) RemoveMember(ctx context.Context, teamID, userID int64) error {
	return exactlyOne(s.pool.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID))
}
