package store

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"kanban/internal/rules"
)

// RuleError refuses a change that breaks the board's rules; it carries every
// rule broken.
type RuleError struct {
	Violations []rules.Violation
}

func (e *RuleError) Error() string {
	codes := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		codes[i] = v.Code
	}
	return "store: rules broken: " + strings.Join(codes, ", ")
}

func ruleError(vs []rules.Violation) error {
	if len(vs) == 0 {
		return nil
	}
	return &RuleError{Violations: vs}
}

// BoardRole is a named role on a board, used only by rules.
type BoardRole struct {
	ID        int64
	BoardID   int64
	Name      string
	MemberIDs []int64
}

// MovePermission says who may move cards into a column.
type MovePermission struct {
	ID           int64
	ToColumnID   int64
	FromColumnID *int64
	Subject      string
	BoardRoleID  *int64
}

// ColumnCondition gates entering or leaving a column.
type ColumnCondition struct {
	ID       int64
	ColumnID int64
	Phase    string
	Kind     string
	Params   rules.ConditionParams
}

// BoardRules is a board's rule configuration.
type BoardRules struct {
	Roles       []BoardRole
	Transitions []rules.Transition
	Permissions []MovePermission
	Conditions  []ColumnCondition
}

// BoardRules reads a board's rules.
func (s *Store) BoardRules(ctx context.Context, boardID int64) (BoardRules, error) {
	var r BoardRules
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.board_id, r.name, coalesce(array_agg(m.user_id ORDER BY m.user_id) FILTER (WHERE m.user_id IS NOT NULL), '{}')
		FROM board_roles r LEFT JOIN board_role_members m ON m.role_id = r.id
		WHERE r.board_id = $1 GROUP BY r.id ORDER BY lower(r.name), r.id`, boardID)
	if err != nil {
		return r, err
	}
	if r.Roles, err = pgx.CollectRows(rows, pgx.RowToStructByPos[BoardRole]); err != nil {
		return r, err
	}
	rows, err = s.pool.Query(ctx, `
		SELECT t.from_column_id, t.to_column_id FROM transitions t
		JOIN columns f ON f.id = t.from_column_id JOIN columns c ON c.id = t.to_column_id
		WHERE t.board_id = $1 ORDER BY f.position, c.position`, boardID)
	if err != nil {
		return r, err
	}
	if r.Transitions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[rules.Transition]); err != nil {
		return r, err
	}
	rows, err = s.pool.Query(ctx, `
		SELECT id, to_column_id, from_column_id, subject, board_role_id FROM move_permissions
		WHERE board_id = $1 ORDER BY id`, boardID)
	if err != nil {
		return r, err
	}
	if r.Permissions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[MovePermission]); err != nil {
		return r, err
	}
	rows, err = s.pool.Query(ctx, `
		SELECT k.id, k.column_id, k.phase, k.kind, k.params FROM column_conditions k
		JOIN columns c ON c.id = k.column_id
		WHERE c.board_id = $1 ORDER BY c.position, k.id`, boardID)
	if err != nil {
		return r, err
	}
	r.Conditions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[ColumnCondition])
	return r, err
}

// SetBoardPolicy sets a board's transitions mode and person WIP limit (nil: none).
func (s *Store) SetBoardPolicy(ctx context.Context, boardID int64, mode string, personWIPLimit *int) error {
	return exactlyOne(s.pool.Exec(ctx,
		`UPDATE boards SET transitions_mode = $2, person_wip_limit = $3 WHERE id = $1`, boardID, mode, personWIPLimit))
}

// columnsOfBoard reports whether every id is a column of boardID.
func columnsOfBoard(ctx context.Context, q pgx.Tx, boardID int64, ids []int64) error {
	var n int
	if err := q.QueryRow(ctx, `SELECT count(DISTINCT id) FROM columns WHERE board_id = $1 AND id = ANY($2::bigint[])`, boardID, ids).Scan(&n); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if n != len(seen) {
		return ErrNotFound
	}
	return nil
}

// SetTransitions replaces a board's allowed transitions.
func (s *Store) SetTransitions(ctx context.Context, boardID int64, pairs []rules.Transition) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	var ids []int64
	for _, p := range pairs {
		ids = append(ids, p.From, p.To)
	}
	if len(ids) > 0 {
		if err := columnsOfBoard(ctx, tx, boardID, ids); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM transitions WHERE board_id = $1`, boardID); err != nil {
		return err
	}
	for _, p := range pairs {
		if p.From == p.To {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transitions (board_id, from_column_id, to_column_id) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, boardID, p.From, p.To); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AddMovePermission adds a permission; its columns and role must be the board's.
func (s *Store) AddMovePermission(ctx context.Context, boardID int64, p MovePermission) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ids := []int64{p.ToColumnID}
	if p.FromColumnID != nil {
		ids = append(ids, *p.FromColumnID)
	}
	if err := columnsOfBoard(ctx, tx, boardID, ids); err != nil {
		return err
	}
	if p.BoardRoleID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM board_roles WHERE id = $1 AND board_id = $2)`, *p.BoardRoleID, boardID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO move_permissions (board_id, to_column_id, from_column_id, subject, board_role_id)
		VALUES ($1, $2, $3, $4, $5)`, boardID, p.ToColumnID, p.FromColumnID, p.Subject, p.BoardRoleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteMovePermission removes a permission of boardID.
func (s *Store) DeleteMovePermission(ctx context.Context, boardID, id int64) error {
	return exactlyOne(s.pool.Exec(ctx, `DELETE FROM move_permissions WHERE id = $2 AND board_id = $1`, boardID, id))
}

// AddCondition adds a condition to a column of boardID. Labels it names must be
// the board's.
func (s *Store) AddCondition(ctx context.Context, boardID int64, c ColumnCondition) error {
	if len(c.Params.LabelIDs) > 0 {
		var n int
		if err := s.pool.QueryRow(ctx, `SELECT count(DISTINCT id) FROM labels WHERE board_id = $1 AND id = ANY($2::bigint[])`,
			boardID, c.Params.LabelIDs).Scan(&n); err != nil {
			return err
		}
		if n != len(slices.Compact(slices.Sorted(slices.Values(c.Params.LabelIDs)))) {
			return ErrNotFound
		}
	}
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO column_conditions (column_id, phase, kind, params)
		SELECT id, $3, $4, $5 FROM columns WHERE id = $2 AND board_id = $1`,
		boardID, c.ColumnID, c.Phase, c.Kind, c.Params)
	return exactlyOne(tag, err)
}

// DeleteCondition removes a condition of a column of boardID.
func (s *Store) DeleteCondition(ctx context.Context, boardID, id int64) error {
	return exactlyOne(s.pool.Exec(ctx, `
		DELETE FROM column_conditions k USING columns c
		WHERE k.id = $2 AND c.id = k.column_id AND c.board_id = $1`, boardID, id))
}

// CreateBoardRole adds a role with no members.
func (s *Store) CreateBoardRole(ctx context.Context, boardID int64, name string) (BoardRole, error) {
	r := BoardRole{BoardID: boardID, Name: name}
	err := s.pool.QueryRow(ctx, `INSERT INTO board_roles (board_id, name) VALUES ($1, $2) RETURNING id`, boardID, name).Scan(&r.ID)
	return r, err
}

// DeleteBoardRole removes a role. A role a permission names is ErrInUse:
// deleting it would drop the permission and, with it, the restriction.
func (s *Store) DeleteBoardRole(ctx context.Context, boardID, id int64) error {
	var used bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM move_permissions WHERE board_role_id = $1 AND board_id = $2)`, id, boardID).Scan(&used); err != nil {
		return err
	}
	if used {
		return ErrInUse
	}
	return exactlyOne(s.pool.Exec(ctx, `DELETE FROM board_roles WHERE id = $2 AND board_id = $1`, boardID, id))
}

// SetBoardRoleMembers makes userIDs the role's members; users outside the
// board's team are ignored.
func (s *Store) SetBoardRoleMembers(ctx context.Context, boardID, roleID int64, userIDs []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var teamID int64
	err = tx.QueryRow(ctx, `
		SELECT b.team_id FROM board_roles r JOIN boards b ON b.id = r.board_id
		WHERE r.id = $1 AND r.board_id = $2`, roleID, boardID).Scan(&teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM board_role_members WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO board_role_members (role_id, user_id)
		SELECT $1, m.user_id FROM team_members m WHERE m.team_id = $2 AND m.user_id = ANY($3::bigint[])`,
		roleID, teamID, userIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
