package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ChecklistItem is one line of a card's checklist.
type ChecklistItem struct {
	ID       int64
	CardID   int64
	Text     string
	Done     bool
	Position int
}

// ChecklistItems returns a card's checklist in order.
func (s *Store) ChecklistItems(ctx context.Context, cardID int64) ([]ChecklistItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, card_id, text, done, position FROM checklist_items
		WHERE card_id = $1 ORDER BY position, id`, cardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[ChecklistItem])
}

// AddChecklistItem appends an item to a card of boardID.
func (s *Store) AddChecklistItem(ctx context.Context, boardID, cardID int64, text string) (ChecklistItem, error) {
	var item ChecklistItem
	err := s.inCard(ctx, boardID, cardID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO checklist_items (card_id, text, position)
			VALUES ($1, $2, (SELECT count(*) FROM checklist_items WHERE card_id = $1))
			RETURNING id, card_id, text, done, position`, cardID, text).
			Scan(&item.ID, &item.CardID, &item.Text, &item.Done, &item.Position)
	})
	return item, err
}

// SetChecklistItemDone ticks or unticks an item of a card of boardID.
func (s *Store) SetChecklistItemDone(ctx context.Context, boardID, cardID, itemID int64, done bool) error {
	return s.inCard(ctx, boardID, cardID, func(tx pgx.Tx) error {
		return exactlyOne(tx.Exec(ctx,
			`UPDATE checklist_items SET done = $3 WHERE id = $2 AND card_id = $1`, cardID, itemID, done))
	})
}

// DeleteChecklistItem removes an item and closes the gap.
func (s *Store) DeleteChecklistItem(ctx context.Context, boardID, cardID, itemID int64) error {
	return s.inCard(ctx, boardID, cardID, func(tx pgx.Tx) error {
		if err := exactlyOne(tx.Exec(ctx,
			`DELETE FROM checklist_items WHERE id = $2 AND card_id = $1`, cardID, itemID)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE checklist_items i SET position = r.n
			FROM (SELECT id, row_number() OVER (ORDER BY position, id) - 1 AS n
			      FROM checklist_items WHERE card_id = $1) r
			WHERE i.id = r.id`, cardID)
		return err
	})
}

// inCard runs fn in a transaction after bumping the version of a card of
// boardID; a card of another board is ErrNotFound.
func (s *Store) inCard(ctx context.Context, boardID, cardID int64, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `
		UPDATE cards SET version = version + 1 WHERE id = $2 AND board_id = $1 RETURNING id`, boardID, cardID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
