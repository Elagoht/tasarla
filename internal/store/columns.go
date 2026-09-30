package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrColumnNotEmpty refuses to delete a column that still holds cards.
var ErrColumnNotEmpty = errors.New("store: the column still has cards")

// Column is one column of a board.
type Column struct {
	ID              int64
	BoardID         int64
	Name            string
	Position        int
	WIPLimit        *int
	IsDone          bool
	AllowCreate     bool
	CountsPersonWIP bool
	CreatedAt       time.Time
}

// ColumnUpdate is what a column's settings form changes.
type ColumnUpdate struct {
	Name            string
	WIPLimit        *int
	IsDone          bool
	AllowCreate     bool
	CountsPersonWIP bool
}

const columnColumns = `id, board_id, name, position, wip_limit, is_done, allow_create, counts_person_wip, created_at`

func scanColumn(row pgx.Row) (Column, error) {
	var c Column
	err := row.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &c.WIPLimit, &c.IsDone, &c.AllowCreate, &c.CountsPersonWIP, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Column{}, ErrNotFound
	}
	return c, err
}

// Columns returns a board's columns, left to right.
func (s *Store) Columns(ctx context.Context, boardID int64) ([]Column, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+columnColumns+` FROM columns WHERE board_id = $1 ORDER BY position, id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []Column
	for rows.Next() {
		c, err := scanColumn(rows)
		if err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// AddColumn appends a column at the right of the board.
func (s *Store) AddColumn(ctx context.Context, boardID int64, name string) (Column, error) {
	return scanColumn(s.pool.QueryRow(ctx, `
		INSERT INTO columns (board_id, name, position)
		VALUES ($1, $2, (SELECT coalesce(max(position) + 1, 0) FROM columns WHERE board_id = $1))
		RETURNING `+columnColumns, boardID, name))
}

// UpdateColumn changes a column's name and settings.
func (s *Store) UpdateColumn(ctx context.Context, boardID, columnID int64, u ColumnUpdate) error {
	return exactlyOne(s.pool.Exec(ctx, `
		UPDATE columns SET name = $3, wip_limit = $4, is_done = $5, allow_create = $6, counts_person_wip = $7
		WHERE id = $2 AND board_id = $1`,
		boardID, columnID, u.Name, u.WIPLimit, u.IsDone, u.AllowCreate, u.CountsPersonWIP))
}

// MoveColumn swaps a column with its neighbour: dir -1 is left, +1 right. A
// column already at that edge stays where it is.
func (s *Store) MoveColumn(ctx context.Context, boardID, columnID int64, dir int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM boards WHERE id = $1 FOR UPDATE`, boardID); err != nil {
		return err
	}
	var pos int
	err = tx.QueryRow(ctx, `SELECT position FROM columns WHERE id = $1 AND board_id = $2`, columnID, boardID).Scan(&pos)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var neighbour int64
	err = tx.QueryRow(ctx, `SELECT id FROM columns WHERE board_id = $1 AND position = $2`, boardID, pos+dir).Scan(&neighbour)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE columns SET position = $2 WHERE id = $1`, neighbour, pos); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE columns SET position = $2 WHERE id = $1`, columnID, pos+dir); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteColumn removes a column that has no cards, archived ones included, and
// closes the gap it leaves.
func (s *Store) DeleteColumn(ctx context.Context, boardID, columnID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM boards WHERE id = $1 FOR UPDATE`, boardID); err != nil {
		return err
	}
	var cards int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM cards WHERE column_id = $1`, columnID).Scan(&cards); err != nil {
		return err
	}
	if cards > 0 {
		return ErrColumnNotEmpty
	}
	if err := exactlyOne(tx.Exec(ctx, `DELETE FROM columns WHERE id = $1 AND board_id = $2`, columnID, boardID)); err != nil {
		return err
	}
	if err := renumberColumns(ctx, tx, boardID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func renumberColumns(ctx context.Context, tx pgx.Tx, boardID int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE columns c SET position = r.n
		FROM (SELECT id, row_number() OVER (ORDER BY position, id) - 1 AS n FROM columns WHERE board_id = $1) r
		WHERE c.id = r.id`, boardID)
	return err
}
