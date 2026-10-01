package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"kanban/internal/rules"
)

// Move is a request to put a card at an index of a column, made by a reader
// who saw it in ExpectedFrom at ExpectedVersion. With Lane, the card goes onto
// that lane of the column and ToIndex is ignored.
type Move struct {
	BoardID         int64
	CardID          int64
	ToColumnID      int64
	ToIndex         int
	ExpectedFrom    int64
	ExpectedVersion int
	Actor           rules.Actor
	Lane            *LaneTarget
}

// LaneTarget is where a card dropped on a lane goes: the lane's value for the
// field the board is split by, and the card it was dropped above.
type LaneTarget struct {
	Field        CardField // FieldAssignee or FieldPriority
	AssigneeID   *int64    // for FieldAssignee; nil: unassigned
	Priority     *int16    // for FieldPriority; nil: none
	BeforeCardID int64     // 0: the end of the lane
}

// MoveResult is the card before and after a move.
type MoveResult struct{ Before, After Card }

// MoveCard moves a card; see MoveCardWith.
func (s *Store) MoveCard(ctx context.Context, m Move) (Card, error) {
	r, err := s.MoveCardWith(ctx, m)
	return r.After, err
}

// MoveCardWith moves a card inside one transaction that holds the board's
// lock (spec §5.3); with m.Lane, onto a lane, changing the field the board is
// split by in the same change. A card no longer where the reader saw it, or
// at another version, is ErrConflict; a broken rule is a *RuleError and
// changes nothing.
func (s *Store) MoveCardWith(ctx context.Context, m Move) (MoveResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MoveResult{}, err
	}
	defer tx.Rollback(ctx)
	r, err := moveCard(ctx, tx, m)
	if err != nil {
		return MoveResult{}, err
	}
	return r, tx.Commit(ctx)
}

func moveCard(ctx context.Context, tx pgx.Tx, m Move) (MoveResult, error) {
	if err := lockBoard(ctx, tx, m.BoardID); err != nil {
		return MoveResult{}, err
	}
	card, err := scanCard(tx.QueryRow(ctx,
		`SELECT `+cardColumns+` FROM cards WHERE id = $2 AND board_id = $1 AND archived_at IS NULL FOR UPDATE`, m.BoardID, m.CardID))
	if err != nil {
		return MoveResult{}, err
	}
	var to int64
	err = tx.QueryRow(ctx, `SELECT id FROM columns WHERE id = $1 AND board_id = $2`, m.ToColumnID, m.BoardID).Scan(&to)
	if errors.Is(err, pgx.ErrNoRows) {
		return MoveResult{}, ErrNotFound
	}
	if err != nil {
		return MoveResult{}, err
	}
	if card.ColumnID != m.ExpectedFrom || card.Version != m.ExpectedVersion {
		return MoveResult{}, ErrConflict
	}
	if m.Lane != nil {
		return moveOntoLane(ctx, tx, m, card)
	}
	if card.ColumnID == m.ToColumnID {
		after, err := placeCard(ctx, tx, card, m.ToColumnID, m.ToIndex)
		if err != nil {
			return MoveResult{}, err
		}
		return MoveResult{Before: card, After: after}, nil
	}
	snap, err := loadSnapshot(ctx, tx, m.BoardID, card)
	if err != nil {
		return MoveResult{}, err
	}
	actor, err := actorRoles(ctx, tx, m.BoardID, m.Actor)
	if err != nil {
		return MoveResult{}, err
	}
	move := rules.Move{CardID: card.ID, FromColumnID: card.ColumnID, ToColumnID: m.ToColumnID, ToIndex: m.ToIndex}
	if err := ruleError(rules.Evaluate(actor, move, snap)); err != nil {
		return MoveResult{}, err
	}
	after, err := applyColumnMove(ctx, tx, m, card, snap, m.ToIndex)
	if err != nil {
		return MoveResult{}, err
	}
	return MoveResult{Before: card, After: after}, nil
}

// moveOntoLane moves card, locked and checked by moveCard, onto m.Lane of
// m.ToColumnID. The rules see the card as it will be, in its new column with
// its new value; a refused part refuses the whole.
func moveOntoLane(ctx context.Context, tx pgx.Tx, m Move, card Card) (MoveResult, error) {
	lane := m.Lane
	next := card
	var fieldChanged bool
	switch lane.Field {
	case FieldAssignee:
		next.AssigneeID = lane.AssigneeID
		fieldChanged = !samePtr(card.AssigneeID, next.AssigneeID)
	case FieldPriority:
		next.Priority = lane.Priority
		fieldChanged = !samePtr(card.Priority, next.Priority)
	default:
		return MoveResult{}, ErrUnknownField
	}
	index, err := laneIndex(ctx, tx, card.ID, m.ToColumnID, lane)
	if err != nil {
		return MoveResult{}, err
	}
	columnChanged := card.ColumnID != m.ToColumnID

	var snap rules.Snapshot
	if columnChanged || fieldChanged {
		if snap, err = loadSnapshot(ctx, tx, m.BoardID, next); err != nil {
			return MoveResult{}, err
		}
	}
	var vs []rules.Violation
	if columnChanged {
		actor, err := actorRoles(ctx, tx, m.BoardID, m.Actor)
		if err != nil {
			return MoveResult{}, err
		}
		move := rules.Move{CardID: card.ID, FromColumnID: card.ColumnID, ToColumnID: m.ToColumnID, ToIndex: index}
		vs = rules.Evaluate(actor, move, snap)
	}
	if lane.Field == FieldAssignee && fieldChanged {
		if next.AssigneeID != nil {
			var member bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM team_members m JOIN boards b ON b.team_id = m.team_id
				               WHERE b.id = $1 AND m.user_id = $2)`, m.BoardID, *next.AssigneeID).Scan(&member); err != nil {
				return MoveResult{}, err
			}
			if !member {
				vs = append(vs, rules.Violation{Code: "rules.assignee_not_member"})
			}
		}
		// Evaluate already checks the person's WIP when the card comes from a
		// column that does not count it.
		if !columnChanged || snap.Columns[card.ColumnID].CountsPersonWIP {
			vs = append(vs, rules.EvaluateAssign(m.ToColumnID, snap)...)
		}
	}
	if err := ruleError(vs); err != nil {
		return MoveResult{}, err
	}

	var placed Card
	if columnChanged {
		placed, err = applyColumnMove(ctx, tx, m, card, snap, index)
	} else {
		placed, err = placeCard(ctx, tx, card, m.ToColumnID, index)
	}
	if err != nil {
		return MoveResult{}, err
	}
	if !fieldChanged {
		return MoveResult{Before: card, After: placed}, nil
	}
	after, err := scanCard(tx.QueryRow(ctx, `
		UPDATE cards SET assignee_id = $2, priority = $3, version = version + 1 WHERE id = $1
		RETURNING `+cardColumns, card.ID, next.AssigneeID, next.Priority))
	if err != nil {
		return MoveResult{}, err
	}
	changed := changedFields(placed, after)
	changes, err := fieldChanges(ctx, tx, placed, after, changed)
	if err != nil {
		return MoveResult{}, err
	}
	if err := logActivity(ctx, tx, m.BoardID, &card.ID, m.Actor.UserID, ActivityCardUpdated,
		ActivityPayload{Fields: changed, Changes: changes}); err != nil {
		return MoveResult{}, err
	}
	return MoveResult{Before: card, After: after}, nil
}

// applyColumnMove puts card, which the rules let through, at index of
// m.ToColumnID, another column than its own, completing or reopening it as
// the columns say, and writes the move to the history.
func applyColumnMove(ctx context.Context, tx pgx.Tx, m Move, card Card, snap rules.Snapshot, index int) (Card, error) {
	moved, err := placeCard(ctx, tx, card, m.ToColumnID, index)
	if err != nil {
		return Card{}, err
	}
	p := ActivityPayload{From: snap.Columns[card.ColumnID].Name, To: snap.Columns[m.ToColumnID].Name}
	kind := ActivityCardMoved
	// Entering a done column completes the card: it leaves the board.
	// Leaving one for another column reopens it.
	switch toDone := snap.Columns[m.ToColumnID].IsDone; {
	case toDone && card.CompletedAt == nil:
		kind = ActivityCardCompleted
		moved, err = scanCard(tx.QueryRow(ctx, `
			UPDATE cards SET completed_at = now(), completed_from_column_id = $2 WHERE id = $1
			RETURNING `+cardColumns, card.ID, card.ColumnID))
		if err == nil {
			err = renumberCards(ctx, tx, m.ToColumnID)
		}
	case !toDone && card.CompletedAt != nil:
		kind = ActivityCardReopened
		moved, err = scanCard(tx.QueryRow(ctx, `
			UPDATE cards SET completed_at = NULL, completed_from_column_id = NULL WHERE id = $1
			RETURNING `+cardColumns, card.ID))
	}
	if err != nil {
		return Card{}, err
	}
	return moved, logActivity(ctx, tx, m.BoardID, &card.ID, m.Actor.UserID, kind, p)
}

// laneIndex is where in column a card dropped on lane goes: above the card it
// was dropped on when that card is in the column and the lane, else after the
// lane's last card there, else at the bottom.
func laneIndex(ctx context.Context, tx pgx.Tx, cardID, column int64, lane *LaneTarget) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, assignee_id, priority FROM cards
		WHERE column_id = $1 AND archived_at IS NULL AND completed_at IS NULL AND id <> $2
		ORDER BY position, id`, column, cardID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	inLane := func(a *int64, p *int16) bool {
		if lane.Field == FieldAssignee {
			return samePtr(a, lane.AssigneeID)
		}
		return samePtr(p, lane.Priority)
	}
	i, last, before := 0, -1, -1
	for rows.Next() {
		var id int64
		var a *int64
		var p *int16
		if err := rows.Scan(&id, &a, &p); err != nil {
			return 0, err
		}
		if inLane(a, p) {
			last = i
			if id == lane.BeforeCardID {
				before = i
			}
		}
		i++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	switch {
	case before >= 0:
		return before, nil
	case last >= 0:
		return last + 1, nil
	}
	return i, nil
}

// placeCard puts card at index of column and renumbers both columns it touched.
func placeCard(ctx context.Context, tx pgx.Tx, card Card, column int64, index int) (Card, error) {
	rows, err := tx.Query(ctx, `
		SELECT id FROM cards
		WHERE column_id = $1 AND archived_at IS NULL AND completed_at IS NULL AND id <> $2
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
		      FROM cards WHERE column_id = $1 AND archived_at IS NULL AND completed_at IS NULL) r
		WHERE c.id = r.id`, column)
	return err
}
