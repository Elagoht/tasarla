package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"kanban/internal/rules"
)

// loadSnapshot reads what the rules need about boardID and card, inside tx and
// under the board's lock, so counts cannot change before the decision is
// written (spec §5.3). Counts leave card out.
func loadSnapshot(ctx context.Context, tx pgx.Tx, boardID int64, card Card) (rules.Snapshot, error) {
	s := rules.Snapshot{
		Transitions: map[rules.Transition]bool{},
		Columns:     map[int64]rules.Column{},
		LabelNames:  map[int64]string{},
	}
	var personLimit *int
	if err := tx.QueryRow(ctx, `SELECT transitions_mode, person_wip_limit FROM boards WHERE id = $1`, boardID).
		Scan(&s.TransitionsMode, &personLimit); err != nil {
		return s, err
	}
	if personLimit != nil {
		s.PersonWIPLimit = *personLimit
	}

	rows, err := tx.Query(ctx, `
		SELECT c.id, c.name, coalesce(c.wip_limit, 0), c.is_done, c.counts_person_wip,
		       (SELECT count(*) FROM cards k WHERE k.column_id = c.id AND k.archived_at IS NULL AND k.id <> $2)
		FROM columns c WHERE c.board_id = $1`, boardID, card.ID)
	if err != nil {
		return s, err
	}
	cols, err := pgx.CollectRows(rows, pgx.RowToStructByPos[rules.Column])
	if err != nil {
		return s, err
	}
	for _, c := range cols {
		s.Columns[c.ID] = c
	}

	rows, err = tx.Query(ctx, `SELECT from_column_id, to_column_id FROM transitions WHERE board_id = $1`, boardID)
	if err != nil {
		return s, err
	}
	pairs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[rules.Transition])
	if err != nil {
		return s, err
	}
	for _, p := range pairs {
		s.Transitions[p] = true
	}

	rows, err = tx.Query(ctx, `
		SELECT to_column_id, coalesce(from_column_id, 0), subject, coalesce(board_role_id, 0)
		FROM move_permissions WHERE board_id = $1`, boardID)
	if err != nil {
		return s, err
	}
	if s.Permissions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[rules.Permission]); err != nil {
		return s, err
	}

	rows, err = tx.Query(ctx, `
		SELECT k.column_id, k.phase, k.kind, k.params FROM column_conditions k
		JOIN columns c ON c.id = k.column_id WHERE c.board_id = $1 ORDER BY c.position, k.id`, boardID)
	if err != nil {
		return s, err
	}
	if s.Conditions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[rules.Condition]); err != nil {
		return s, err
	}

	rows, err = tx.Query(ctx, `SELECT id, name FROM labels WHERE board_id = $1`, boardID)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return s, err
		}
		s.LabelNames[id] = name
	}
	if err := rows.Err(); err != nil {
		return s, err
	}

	s.Card = rules.Card{
		ID:          card.ID,
		HasEstimate: card.Estimate != nil,
		HasDueDate:  card.DueDate != nil,
		Description: card.Description,
	}
	if card.AssigneeID != nil {
		s.Card.AssigneeID = *card.AssigneeID
	}
	if card.ID != 0 {
		rows, err = tx.Query(ctx, `SELECT label_id FROM card_labels WHERE card_id = $1`, card.ID)
		if err != nil {
			return s, err
		}
		if s.Card.LabelIDs, err = pgx.CollectRows(rows, pgx.RowTo[int64]); err != nil {
			return s, err
		}
		if err := tx.QueryRow(ctx, `
			SELECT (SELECT count(*) FROM checklist_items WHERE card_id = $1),
			       (SELECT count(*) FROM checklist_items WHERE card_id = $1 AND done),
			       (SELECT count(*) FROM card_dependencies d JOIN cards b ON b.id = d.blocker_id
			        JOIN columns bc ON bc.id = b.column_id
			        WHERE d.blocked_id = $1 AND b.archived_at IS NULL AND NOT bc.is_done),
			       (SELECT count(*) FROM attachments WHERE card_id = $1)`, card.ID).
			Scan(&s.Card.ChecklistTotal, &s.Card.ChecklistDone, &s.Card.OpenBlockers, &s.Card.Attachments); err != nil {
			return s, err
		}
	}
	if err := assigneeWIP(ctx, tx, &s, boardID, card.ID); err != nil {
		return s, err
	}
	return s, nil
}

// assigneeWIP counts s.Card's assignee's cards in counted columns of boardID,
// leaving cardID out.
func assigneeWIP(ctx context.Context, tx pgx.Tx, s *rules.Snapshot, boardID, cardID int64) error {
	s.AssigneeWIP = 0
	if s.Card.AssigneeID == 0 {
		return nil
	}
	return tx.QueryRow(ctx, `
		SELECT count(*) FROM cards k JOIN columns c ON c.id = k.column_id
		WHERE k.board_id = $1 AND k.assignee_id = $2 AND k.archived_at IS NULL
		  AND c.counts_person_wip AND k.id <> $3`, boardID, s.Card.AssigneeID, cardID).Scan(&s.AssigneeWIP)
}

// actorRoles fills in the board roles actor holds on boardID.
func actorRoles(ctx context.Context, tx pgx.Tx, boardID int64, actor rules.Actor) (rules.Actor, error) {
	rows, err := tx.Query(ctx, `
		SELECT m.role_id FROM board_role_members m JOIN board_roles r ON r.id = m.role_id
		WHERE r.board_id = $1 AND m.user_id = $2`, boardID, actor.UserID)
	if err != nil {
		return actor, err
	}
	actor.BoardRoles, err = pgx.CollectRows(rows, pgx.RowTo[int64])
	return actor, err
}
