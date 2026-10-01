package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"kanban/internal/rules"
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
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	var owned, used bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM columns WHERE id = $1 AND board_id = $2),
		       EXISTS (SELECT 1 FROM move_permissions WHERE from_column_id = $1)`, columnID, boardID).Scan(&owned, &used); err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}
	if used {
		return ErrInUse // dropping the permission would open the column to everyone
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

// ColumnRow is one row of the settings' column table, in its new order.
// ID 0 is a new column.
type ColumnRow struct {
	ID              int64
	Name            string
	WIPLimit        *int
	IsDone          bool
	AllowCreate     bool
	CountsPersonWIP bool
	Delete          bool
}

// ColumnRowError is why one row could not be saved; Index -1 is about the
// table as a whole.
type ColumnRowError struct {
	Index int
	Err   error
}

// ColumnsError refuses a whole column table; nothing of it was saved.
type ColumnsError struct {
	Rows []ColumnRowError
}

func (e *ColumnsError) Error() string { return "store: the columns could not be saved" }

// SaveColumns applies a whole column table in one transaction under the
// board's lock: order, names, settings, new columns and deleted ones. A row
// that cannot be applied refuses the whole table, every such row named.
func (s *Store) SaveColumns(ctx context.Context, boardID int64, rows []ColumnRow) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	var problems []ColumnRowError
	kept := 0
	for i, r := range rows {
		if r.ID != 0 {
			var owned bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM columns WHERE id = $1 AND board_id = $2)`, r.ID, boardID).Scan(&owned); err != nil {
				return err
			}
			if !owned {
				return ErrNotFound
			}
		}
		if !r.Delete {
			kept++
			continue
		}
		if r.ID == 0 {
			continue
		}
		var cards int
		var used bool
		if err := tx.QueryRow(ctx, `
			SELECT (SELECT count(*) FROM cards WHERE column_id = $1),
			       EXISTS (SELECT 1 FROM move_permissions WHERE from_column_id = $1)`, r.ID).Scan(&cards, &used); err != nil {
			return err
		}
		switch {
		case cards > 0:
			problems = append(problems, ColumnRowError{Index: i, Err: ErrColumnNotEmpty})
		case used:
			problems = append(problems, ColumnRowError{Index: i, Err: ErrInUse})
		}
	}
	if kept == 0 {
		problems = append(problems, ColumnRowError{Index: -1, Err: ErrInUse})
	}
	if len(problems) > 0 {
		return &ColumnsError{Rows: problems}
	}
	// Under restricted transitions a new column must be reachable, like a
	// column no sentence restricts: from every column, and to every column
	// that takes cards from everywhere. Which those are is read before the
	// table changes.
	var mode string
	if err := tx.QueryRow(ctx, `SELECT transitions_mode FROM boards WHERE id = $1`, boardID).Scan(&mode); err != nil {
		return err
	}
	var open []int64
	if mode == rules.ModeRestricted {
		rs, err := tx.Query(ctx, `
			SELECT t.id FROM columns t WHERE t.board_id = $1 AND NOT EXISTS (
				SELECT 1 FROM columns f WHERE f.board_id = $1 AND f.id <> t.id AND NOT EXISTS (
					SELECT 1 FROM transitions x WHERE x.from_column_id = f.id AND x.to_column_id = t.id))`, boardID)
		if err != nil {
			return err
		}
		open, err = pgx.CollectRows(rs, pgx.RowTo[int64])
		if err != nil {
			return err
		}
	}
	var added []int64
	position := 0
	for _, r := range rows {
		switch {
		case r.Delete && r.ID != 0:
			if _, err := tx.Exec(ctx, `DELETE FROM columns WHERE id = $1 AND board_id = $2`, r.ID, boardID); err != nil {
				return err
			}
		case r.Delete:
		case r.ID == 0:
			var id int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO columns (board_id, name, position, wip_limit, is_done, allow_create, counts_person_wip)
				VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
				boardID, r.Name, position, r.WIPLimit, r.IsDone, r.AllowCreate, r.CountsPersonWIP).Scan(&id); err != nil {
				return err
			}
			added = append(added, id)
			position++
		default:
			if _, err := tx.Exec(ctx, `
				UPDATE columns SET name = $3, position = $4, wip_limit = $5, is_done = $6, allow_create = $7, counts_person_wip = $8
				WHERE id = $1 AND board_id = $2`,
				r.ID, boardID, r.Name, position, r.WIPLimit, r.IsDone, r.AllowCreate, r.CountsPersonWIP); err != nil {
				return err
			}
			position++
		}
	}
	// A form loaded before another save may not name every column; the order
	// it gives is kept, and the columns it left out follow, numbered 0..n-1.
	if _, err := tx.Exec(ctx, `
		UPDATE columns c SET position = o.n - 1
		FROM (SELECT id, row_number() OVER (ORDER BY position, id) AS n FROM columns WHERE board_id = $1) o
		WHERE c.id = o.id AND c.position <> o.n - 1`, boardID); err != nil {
		return err
	}
	if mode == rules.ModeRestricted {
		for _, n := range added {
			if _, err := tx.Exec(ctx, `
				INSERT INTO transitions (board_id, from_column_id, to_column_id)
				SELECT $1, c.id, $2 FROM columns c WHERE c.board_id = $1 AND c.id <> $2
				UNION ALL
				SELECT $1, $2, c.id FROM columns c WHERE c.board_id = $1 AND c.id = ANY($3)
				ON CONFLICT DO NOTHING`, boardID, n, open); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
