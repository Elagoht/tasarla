package store

import (
	"context"
	"encoding/json"
	"errors"
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
	if loc == nil {
		loc = time.UTC
	}
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
			if errors.Is(err, pgx.ErrNoRows) {
				return FromTemplate{}, ErrNotFound // a board with no columns
			}
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

// ScheduledTemplate is a template with a schedule, on a board that is not archived.
type ScheduledTemplate struct {
	Template  Template
	TeamID    int64
	BoardName string
}

// ScheduledTemplates lists every template whose schedule is on, on a board
// that is not archived. Labels, checklist and runs are not read.
func (s *Store) ScheduledTemplates(ctx context.Context) ([]ScheduledTemplate, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		SELECT `+templateColumns+` FROM card_templates
		WHERE schedule_kind <> '' AND schedule_since IS NOT NULL
			AND board_id IN (SELECT id FROM boards WHERE archived_at IS NULL)
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledTemplate
	var boardIDs []int64
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ScheduledTemplate{Template: t})
		boardIDs = append(boardIDs, t.BoardID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}

	type board struct {
		team int64
		name string
	}
	boards := map[int64]board{}
	brows, err := tx.Query(ctx, `SELECT id, team_id, name FROM boards WHERE id = ANY($1)`, boardIDs)
	if err != nil {
		return nil, err
	}
	defer brows.Close()
	for brows.Next() {
		var id int64
		var b board
		if err := brows.Scan(&id, &b.team, &b.name); err != nil {
			return nil, err
		}
		boards[id] = b
	}
	if err := brows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		b := boards[out[i].Template.BoardID]
		out[i].TeamID, out[i].BoardName = b.team, b.name
	}
	return out, nil
}

// RunOutcome is what one scheduled moment did.
type RunOutcome struct {
	Ran        bool // false: this moment was already run
	Card       *Card
	Violations []rules.Violation
	OwnerGone  bool  // updated_by is no longer in the team
	NoColumn   bool  // the template has no target column
	Owner      int64 // the template's last editor, as the run read it
}

// The reasons a run failed that no board rule gives, recorded as violations.
const (
	ViolationOwnerGone = "templates.owner_gone"
	ViolationNoColumn  = "templates.no_column"
)

// RunTemplate runs a template's moment at once: it records the run (a second
// call for the same moment does nothing) and makes the card, or records why
// not, in one transaction. A template whose schedule was turned off since the
// caller read it records nothing (Ran is false). The card is made by the template's last editor in
// the template's own column, due DueInDays after at's day in loc. A board
// archived meanwhile is ErrNotFound, and nothing is recorded.
func (s *Store) RunTemplate(ctx context.Context, st ScheduledTemplate, at time.Time, loc *time.Location) (RunOutcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RunOutcome{}, err
	}
	defer tx.Rollback(ctx)
	boardID := st.Template.BoardID
	// The board's lock first, as every card write takes it: a second run of
	// the same moment waits here, then finds the moment recorded.
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return RunOutcome{}, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO template_runs (template_id, scheduled_for, status) VALUES ($1, $2, 'failed')
		ON CONFLICT DO NOTHING`, st.Template.ID, at)
	if err != nil {
		return RunOutcome{}, err
	}
	if tag.RowsAffected() == 0 {
		return RunOutcome{}, nil
	}
	// The template as it is now, not as the caller read it.
	tpl, err := templateOf(ctx, tx, boardID, st.Template.ID)
	if err != nil {
		return RunOutcome{}, err
	}
	if tpl.Schedule.Kind == "" || tpl.ScheduleSince == nil {
		return RunOutcome{}, nil // turned off meanwhile: the rollback drops the run row
	}
	out := RunOutcome{Ran: true, Owner: tpl.UpdatedBy}
	var inTeam bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM team_members m JOIN boards b ON b.team_id = m.team_id
		               WHERE b.id = $1 AND m.user_id = $2)`, boardID, tpl.UpdatedBy).Scan(&inTeam); err != nil {
		return RunOutcome{}, err
	}
	switch {
	case !inTeam:
		out.OwnerGone = true
		out.Violations = []rules.Violation{{Code: ViolationOwnerGone}}
	case tpl.ColumnID == nil:
		// A scheduled card needs its column: 0 would open it in the board's first.
		out.NoColumn = true
		out.Violations = []rules.Violation{{Code: ViolationNoColumn}}
	default:
		made, err := createFromTemplate(ctx, tx, boardID, tpl.ID, *tpl.ColumnID, tpl.UpdatedBy, loc, at)
		var re *RuleError
		switch {
		case errors.As(err, &re):
			out.Violations = re.Violations
		case err != nil:
			return RunOutcome{}, err
		default:
			out.Card = &made.Card
			if _, err := tx.Exec(ctx, `
				UPDATE template_runs SET status = 'created', card_id = $3
				WHERE template_id = $1 AND scheduled_for = $2`, tpl.ID, at, made.Card.ID); err != nil {
				return RunOutcome{}, err
			}
		}
	}
	if len(out.Violations) > 0 {
		raw, err := json.Marshal(out.Violations)
		if err != nil {
			return RunOutcome{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE template_runs SET violations = $3 WHERE template_id = $1 AND scheduled_for = $2`,
			tpl.ID, at, raw); err != nil {
			return RunOutcome{}, err
		}
	}
	return out, tx.Commit(ctx)
}

// TeamLeads lists the team's leads.
func (s *Store) TeamLeads(ctx context.Context, teamID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT user_id FROM team_members WHERE team_id = $1 AND role = 'lead' ORDER BY user_id`, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}
