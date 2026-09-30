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

// CreateBoard adds a board with one column per name. The first column is
// where cards are created, the last is the done column.
func (s *Store) CreateBoard(ctx context.Context, teamID int64, name string, columns []string) (Board, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback(ctx)
	board, err := scanBoard(tx.QueryRow(ctx,
		`INSERT INTO boards (team_id, name) VALUES ($1, $2) RETURNING `+boardColumns, teamID, name))
	if err != nil {
		return Board{}, err
	}
	for i, col := range columns {
		_, err := tx.Exec(ctx, `
			INSERT INTO columns (board_id, name, position, allow_create, is_done)
			VALUES ($1, $2, $3, $4, $5)`,
			board.ID, col, i, i == 0, i == len(columns)-1 && len(columns) > 1)
		if err != nil {
			return Board{}, err
		}
	}
	return board, tx.Commit(ctx)
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
	return exactlyOne(s.pool.Exec(ctx,
		`UPDATE boards SET archived_at = coalesce(archived_at, now()) WHERE id = $1`, id))
}
