package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"kanban/internal/rules"
)

// Move is a request to put a card at an index of a column, made by a reader
// who saw it in ExpectedFrom at ExpectedVersion.
type Move struct {
	BoardID         int64
	CardID          int64
	ToColumnID      int64
	ToIndex         int
	ExpectedFrom    int64
	ExpectedVersion int
	Actor           rules.Actor
}

// MoveCard moves a card inside one transaction that holds the board's lock, so
// moves on one board happen one at a time (spec §5.3). A card that is no
// longer where the reader saw it, or at the version they saw, is ErrConflict.
func (s *Store) MoveCard(ctx context.Context, m Move) (Card, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback(ctx)
	card, err := moveCard(ctx, tx, m)
	if err != nil {
		return Card{}, err
	}
	return card, tx.Commit(ctx)
}

func moveCard(ctx context.Context, tx pgx.Tx, m Move) (Card, error) {
	if err := lockBoard(ctx, tx, m.BoardID); err != nil {
		return Card{}, err
	}
	card, err := scanCard(tx.QueryRow(ctx,
		`SELECT `+cardColumns+` FROM cards WHERE id = $2 AND board_id = $1 AND archived_at IS NULL`, m.BoardID, m.CardID))
	if err != nil {
		return Card{}, err
	}
	var to int64
	err = tx.QueryRow(ctx, `SELECT id FROM columns WHERE id = $1 AND board_id = $2`, m.ToColumnID, m.BoardID).Scan(&to)
	if errors.Is(err, pgx.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, err
	}
	if card.ColumnID != m.ExpectedFrom || card.Version != m.ExpectedVersion {
		return Card{}, ErrConflict
	}
	if card.ColumnID != m.ToColumnID {
		snap, err := loadSnapshot(ctx, tx, m.BoardID, card)
		if err != nil {
			return Card{}, err
		}
		actor, err := actorRoles(ctx, tx, m.BoardID, m.Actor)
		if err != nil {
			return Card{}, err
		}
		move := rules.Move{CardID: card.ID, FromColumnID: card.ColumnID, ToColumnID: m.ToColumnID, ToIndex: m.ToIndex}
		if err := ruleError(rules.Evaluate(actor, move, snap)); err != nil {
			return Card{}, err
		}
		moved, err := placeCard(ctx, tx, card, m.ToColumnID, m.ToIndex)
		if err != nil {
			return Card{}, err
		}
		p := ActivityPayload{From: snap.Columns[card.ColumnID].Name, To: snap.Columns[m.ToColumnID].Name}
		return moved, logActivity(ctx, tx, m.BoardID, &card.ID, m.Actor.UserID, ActivityCardMoved, p)
	}
	return placeCard(ctx, tx, card, m.ToColumnID, m.ToIndex)
}

// placeCard puts card at index of column and renumbers both columns it touched.
func placeCard(ctx context.Context, tx pgx.Tx, card Card, column int64, index int) (Card, error) {
	rows, err := tx.Query(ctx, `
		SELECT id FROM cards
		WHERE column_id = $1 AND archived_at IS NULL AND id <> $2
		ORDER BY position, id`, column, card.ID)
	if err != nil {
		return Card{}, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return Card{}, err
	}
	index = max(0, min(index, len(ids)))
	order := make([]int64, 0, len(ids)+1)
	order = append(order, ids[:index]...)
	order = append(order, card.ID)
	order = append(order, ids[index:]...)

	moved, err := scanCard(tx.QueryRow(ctx, `
		UPDATE cards SET column_id = $2, version = version + 1 WHERE id = $1
		RETURNING `+cardColumns, card.ID, column))
	if err != nil {
		return Card{}, err
	}
	if err := setPositions(ctx, tx, order); err != nil {
		return Card{}, err
	}
	if card.ColumnID != column {
		if err := renumberCards(ctx, tx, card.ColumnID); err != nil {
			return Card{}, err
		}
	}
	moved.Position = index
	return moved, nil
}

func setPositions(ctx context.Context, tx pgx.Tx, order []int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE cards c SET position = o.n - 1
		FROM unnest($1::bigint[]) WITH ORDINALITY AS o(id, n)
		WHERE c.id = o.id`, order)
	return err
}

// renumberCards closes gaps in a column's positions.
func renumberCards(ctx context.Context, tx pgx.Tx, column int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE cards c SET position = r.n
		FROM (SELECT id, row_number() OVER (ORDER BY position, id) - 1 AS n
		      FROM cards WHERE column_id = $1 AND archived_at IS NULL) r
		WHERE c.id = r.id`, column)
	return err
}
