package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrCycle refuses a dependency that would make a card wait on itself.
var ErrCycle = errors.New("store: the dependency would create a cycle")

// CardRef names a card another card depends on, or blocks.
type CardRef struct {
	ID    int64
	Title string
	Done  bool
}

// Dependencies are the cards a card waits on and the cards waiting on it.
type Dependencies struct {
	Blockers []CardRef
	Blocking []CardRef
}

// AddDependency records that blocker must finish before blocked; both are
// cards of boardID. One that would close a cycle is ErrCycle (spec §4).
func (s *Store) AddDependency(ctx context.Context, boardID, blockerID, blockedID int64) error {
	if blockerID == blockedID {
		return ErrCycle
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM cards WHERE board_id = $1 AND id IN ($2, $3) AND archived_at IS NULL`, boardID, blockerID, blockedID).Scan(&n); err != nil {
		return err
	}
	if n != 2 {
		return ErrNotFound
	}
	// Is there already a path from blocked to blocker?
	var cycle bool
	if err := tx.QueryRow(ctx, `
		WITH RECURSIVE reach(id) AS (
			SELECT blocked_id FROM card_dependencies WHERE blocker_id = $1
			UNION
			SELECT d.blocked_id FROM card_dependencies d JOIN reach r ON d.blocker_id = r.id
		)
		SELECT EXISTS (SELECT 1 FROM reach WHERE id = $2)`, blockedID, blockerID).Scan(&cycle); err != nil {
		return err
	}
	if cycle {
		return ErrCycle
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO card_dependencies (blocker_id, blocked_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, blockerID, blockedID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE cards SET version = version + 1 WHERE id = $1`, blockedID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveDependency drops blocker → blocked on boardID.
func (s *Store) RemoveDependency(ctx context.Context, boardID, blockerID, blockedID int64) error {
	return s.inCard(ctx, boardID, blockedID, func(tx pgx.Tx) error {
		return exactlyOne(tx.Exec(ctx, `
			DELETE FROM card_dependencies d USING cards b
			WHERE d.blocker_id = $1 AND d.blocked_id = $2 AND b.id = d.blocker_id AND b.board_id = $3`,
			blockerID, blockedID, boardID))
	})
}

// CardDependencies returns what a card waits on and what waits on it. A card
// counts as done in a done column or in the archive.
func (s *Store) CardDependencies(ctx context.Context, cardID int64) (Dependencies, error) {
	var d Dependencies
	var err error
	if d.Blockers, err = s.cardRefs(ctx, `
		SELECT c.id, c.title, (c.archived_at IS NOT NULL OR col.is_done)
		FROM card_dependencies d JOIN cards c ON c.id = d.blocker_id JOIN columns col ON col.id = c.column_id
		WHERE d.blocked_id = $1 ORDER BY c.id`, cardID); err != nil {
		return d, err
	}
	d.Blocking, err = s.cardRefs(ctx, `
		SELECT c.id, c.title, (c.archived_at IS NOT NULL OR col.is_done)
		FROM card_dependencies d JOIN cards c ON c.id = d.blocked_id JOIN columns col ON col.id = c.column_id
		WHERE d.blocker_id = $1 ORDER BY c.id`, cardID)
	return d, err
}

func (s *Store) cardRefs(ctx context.Context, query string, cardID int64) ([]CardRef, error) {
	rows, err := s.pool.Query(ctx, query, cardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[CardRef])
}
