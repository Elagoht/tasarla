package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Label is a board's tag for cards.
type Label struct {
	ID      int64
	BoardID int64
	Name    string
	Color   string
}

// Labels returns a board's labels, by name.
func (s *Store) Labels(ctx context.Context, boardID int64) ([]Label, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, board_id, name, color FROM labels WHERE board_id = $1 ORDER BY lower(name), id`, boardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Label])
}

// CreateLabel adds a label; color is "#rrggbb".
func (s *Store) CreateLabel(ctx context.Context, boardID int64, name, color string) (Label, error) {
	var l Label
	err := s.pool.QueryRow(ctx, `
		INSERT INTO labels (board_id, name, color) VALUES ($1, $2, $3)
		RETURNING id, board_id, name, color`, boardID, name, strings.ToLower(color)).
		Scan(&l.ID, &l.BoardID, &l.Name, &l.Color)
	return l, err
}

// DeleteLabel removes a label from the board, its cards and the conditions
// that name it; a has_label condition left with no label is removed, rather
// than turned into "any label".
func (s *Store) DeleteLabel(ctx context.Context, boardID, labelID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	if err := exactlyOne(tx.Exec(ctx, `DELETE FROM labels WHERE id = $2 AND board_id = $1`, boardID, labelID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE column_conditions k
		SET params = jsonb_set(k.params, '{label_ids}',
		    coalesce((SELECT jsonb_agg(v) FROM jsonb_array_elements(k.params->'label_ids') v WHERE v::bigint <> $2), '[]'::jsonb))
		FROM columns c
		WHERE c.id = k.column_id AND c.board_id = $1 AND k.kind = 'has_label' AND k.params->'label_ids' @> to_jsonb($2::bigint)`,
		boardID, labelID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM column_conditions k USING columns c
		WHERE c.id = k.column_id AND c.board_id = $1 AND k.kind = 'has_label'
		  AND k.params ? 'label_ids' AND jsonb_array_length(k.params->'label_ids') = 0`, boardID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetCardLabels makes labelIDs the card's labels. Ids that are not the board's
// labels are ignored.
func (s *Store) SetCardLabels(ctx context.Context, boardID, cardID int64, labelIDs []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	var id int64
	err = tx.QueryRow(ctx, `
		UPDATE cards SET version = version + 1 WHERE id = $2 AND board_id = $1 RETURNING id`, boardID, cardID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM card_labels WHERE card_id = $1`, cardID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO card_labels (card_id, label_id)
		SELECT $1, l.id FROM labels l WHERE l.board_id = $2 AND l.id = ANY($3::bigint[])`,
		cardID, boardID, labelIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CardLabels returns the ids of a card's labels.
func (s *Store) CardLabels(ctx context.Context, cardID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT label_id FROM card_labels WHERE card_id = $1`, cardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}
