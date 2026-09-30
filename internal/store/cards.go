package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrConflict reports that a card changed since the reader loaded it.
var ErrConflict = errors.New("store: the card was changed by someone else")

// Card is one card on a board.
type Card struct {
	ID          int64
	BoardID     int64
	ColumnID    int64
	Position    int
	Title       string
	Description string
	AssigneeID  *int64
	Estimate    *float64
	DueDate     *time.Time
	Priority    *int16
	CreatedBy   int64
	Version     int
	ArchivedAt  *time.Time
	CreatedAt   time.Time
}

const cardColumns = `id, board_id, column_id, position, title, description, assignee_id, estimate::float8, due_date, priority, created_by, version, archived_at, created_at`

func scanCard(row pgx.Row) (Card, error) {
	var c Card
	err := row.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
		&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	return c, err
}

// CreateCard adds a card at the bottom of a column of boardID.
func (s *Store) CreateCard(ctx context.Context, boardID, columnID int64, title string, createdBy int64) (Card, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback(ctx)
	card, err := createCard(ctx, tx, boardID, columnID, title, createdBy)
	if err != nil {
		return Card{}, err
	}
	return card, tx.Commit(ctx)
}

func createCard(ctx context.Context, tx pgx.Tx, boardID, columnID int64, title string, createdBy int64) (Card, error) {
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return Card{}, err
	}
	return scanCard(tx.QueryRow(ctx, `
		INSERT INTO cards (board_id, column_id, position, title, created_by)
		SELECT $1, c.id,
		       (SELECT count(*) FROM cards WHERE column_id = c.id AND archived_at IS NULL),
		       $3, $4
		FROM columns c WHERE c.id = $2 AND c.board_id = $1
		RETURNING `+cardColumns, boardID, columnID, title, createdBy))
}

// lockBoard serialises the moves and creations of one board (spec §5.3).
func lockBoard(ctx context.Context, tx pgx.Tx, boardID int64) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM boards WHERE id = $1 FOR UPDATE`, boardID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Card returns a card of boardID; a card of another board is not found.
func (s *Store) Card(ctx context.Context, boardID, cardID int64) (Card, error) {
	return scanCard(s.pool.QueryRow(ctx,
		`SELECT `+cardColumns+` FROM cards WHERE id = $2 AND board_id = $1`, boardID, cardID))
}
