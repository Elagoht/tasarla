package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Board is a team's kanban board.
type Board struct {
	ID              int64
	TeamID          int64
	Name            string
	ArchivedAt      *time.Time
	TransitionsMode string
	PersonWIPLimit  *int
	CreatedAt       time.Time
}

const boardColumns = `id, team_id, name, archived_at, transitions_mode, person_wip_limit, created_at`

func scanBoard(row pgx.Row) (Board, error) {
	var b Board
	err := row.Scan(&b.ID, &b.TeamID, &b.Name, &b.ArchivedAt, &b.TransitionsMode, &b.PersonWIPLimit, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Board{}, ErrNotFound
	}
	return b, err
}

func collectBoards(rows pgx.Rows, err error) ([]Board, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var boards []Board
	for rows.Next() {
		b, err := scanBoard(rows)
		if err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

// CreateBoard adds a board with the named columns: the first takes new cards
// and the last, when there are two or more, is done.
func (s *Store) CreateBoard(ctx context.Context, teamID int64, name string, columns []string) (Board, error) {
	return s.CreateBoardFromPlan(ctx, teamID, name, ColumnsPlan(columns), 0)
}

// Board returns one board.
func (s *Store) Board(ctx context.Context, id int64) (Board, error) {
	return scanBoard(s.pool.QueryRow(ctx, `SELECT `+boardColumns+` FROM boards WHERE id = $1`, id))
}

// BoardsOfTeam returns a team's boards that are not archived, by name.
func (s *Store) BoardsOfTeam(ctx context.Context, teamID int64) ([]Board, error) {
	return collectBoards(s.pool.Query(ctx, `
		SELECT `+boardColumns+` FROM boards
		WHERE team_id = $1 AND archived_at IS NULL
		ORDER BY lower(name), id`, teamID))
}

// RenameBoard changes a board's name.
func (s *Store) RenameBoard(ctx context.Context, id int64, name string) error {
	return exactlyOne(s.pool.Exec(ctx, `UPDATE boards SET name = $2 WHERE id = $1`, id, name))
}

// ArchiveBoard hides a board from lists.
func (s *Store) ArchiveBoard(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// The board's lock, so that no write to it is half done when it goes.
	if err := lockBoard(ctx, tx, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE boards SET archived_at = now() WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
