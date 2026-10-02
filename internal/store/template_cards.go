package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"kanban/internal/rules"
)

// FromTemplate is how a card made from a template came out.
type FromTemplate struct {
	Card            Card
	AssigneeDropped bool // the template's assignee is not in the team, or over their WIP
}

// CreateCardFromTemplate makes a card from a template in columnID (0: the
// template's column, or the board's first creatable column). Making it is
// entering the column: its rules apply (a *RuleError). The template's fields,
// labels and checklist are written in the same transaction; the due date is
// today plus DueInDays, today taken in loc.
func (s *Store) CreateCardFromTemplate(ctx context.Context, boardID, templateID, columnID, createdBy int64, loc *time.Location, now time.Time) (FromTemplate, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FromTemplate{}, err
	}
	defer tx.Rollback(ctx)
	out, err := createFromTemplate(ctx, tx, boardID, templateID, columnID, createdBy, loc, now)
	if err != nil {
		return FromTemplate{}, err
	}
	return out, tx.Commit(ctx)
}

// createFromTemplate is CreateCardFromTemplate inside the caller's tx. Every
// rule is decided before the first write, so a *RuleError leaves tx as it was
// and the caller can go on using it.
func createFromTemplate(ctx context.Context, tx pgx.Tx, boardID, templateID, columnID, createdBy int64, loc *time.Location, now time.Time) (FromTemplate, error) {
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return FromTemplate{}, err
	}
	tpl, err := templateOf(ctx, tx, boardID, templateID)
	if err != nil {
		return FromTemplate{}, err
	}
	if columnID == 0 && tpl.ColumnID != nil {
		columnID = *tpl.ColumnID
	}
	if columnID == 0 {
		// The store's side of the web's creatableColumns: the first column
		// marked for creating, or the first column when none is.
		if err := tx.QueryRow(ctx, `
			SELECT id FROM columns WHERE board_id = $1
			ORDER BY NOT allow_create, position, id LIMIT 1`, boardID).Scan(&columnID); err != nil {
			return FromTemplate{}, err
		}
	}

	next := Card{Title: tpl.Title, Description: tpl.Description, AssigneeID: tpl.AssigneeID,
		Estimate: tpl.Estimate, Priority: tpl.Priority}
	if tpl.DueInDays != nil {
		y, m, d := now.In(loc).Date()
		due := time.Date(y, m, d+*tpl.DueInDays, 0, 0, 0, 0, time.UTC)
		next.DueDate = &due
	}

	var out FromTemplate
	if next.AssigneeID != nil {
		var inTeam bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM team_members m JOIN boards b ON b.team_id = m.team_id
			               WHERE b.id = $1 AND m.user_id = $2)`, boardID, *next.AssigneeID).Scan(&inTeam); err != nil {
			return FromTemplate{}, err
		}
		if !inTeam {
			next.AssigneeID, out.AssigneeDropped = nil, true
		}
	}
	snap, err := loadSnapshot(ctx, tx, boardID, next)
	if err != nil {
		return FromTemplate{}, err
	}
	col, ok := snap.Columns[columnID]
	if !ok {
		return FromTemplate{}, ErrNotFound
	}
	if next.AssigneeID != nil && len(rules.EvaluateAssign(columnID, snap)) > 0 {
		next.AssigneeID, out.AssigneeDropped = nil, true
		snap.Card.AssigneeID, snap.AssigneeWIP = 0, 0
	}
	// The rules see the card as it will be, labels and checklist included.
	snap.Card.LabelIDs = tpl.LabelIDs
	snap.Card.ChecklistTotal, snap.Card.ChecklistDone = len(tpl.Checklist), 0
	if err := ruleError(rules.EvaluateCreate(columnID, snap)); err != nil {
		return FromTemplate{}, err
	}

	if out.Card, err = insertCard(ctx, tx, boardID, col, next, createdBy); err != nil {
		return FromTemplate{}, err
	}
	if len(tpl.LabelIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO card_labels (card_id, label_id)
			SELECT $1, l.id FROM labels l WHERE l.board_id = $2 AND l.id = ANY($3::bigint[])`,
			out.Card.ID, boardID, tpl.LabelIDs); err != nil {
			return FromTemplate{}, err
		}
	}
	for i, text := range tpl.Checklist {
		if _, err := tx.Exec(ctx, `INSERT INTO checklist_items (card_id, text, position) VALUES ($1, $2, $3)`,
			out.Card.ID, text, i); err != nil {
			return FromTemplate{}, err
		}
	}
	return out, nil
}
