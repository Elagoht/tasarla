package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"kanban/internal/rules"
)

// ErrTemplateName reports a second template of a board with the same name.
var ErrTemplateName = errors.New("store: a template of this board already has this name")

// Schedule says when a template makes a card by itself. Kind "" is no schedule.
type Schedule struct {
	Kind     string // "", "daily", "weekly", "monthly"
	Weekdays uint8  // weekly: bit 0 Monday … bit 6 Sunday
	MonthDay int    // monthly: 1–31
	Hour     int    // 0–23
	Minute   int    // 0–59
}

// Template is a card blueprint of a board, with an optional schedule.
type Template struct {
	ID            int64
	BoardID       int64
	Name          string
	Title         string
	Description   string
	Priority      *int16
	Estimate      *float64
	AssigneeID    *int64
	ColumnID      *int64 // nil: the target column was deleted, or none was set
	DueInDays     *int
	LabelIDs      []int64
	Checklist     []string
	Schedule      Schedule
	ScheduleSince *time.Time
	UpdatedBy     int64
	UpdatedAt     time.Time
	LastRun       *TemplateRun // the latest run, if any
}

// TemplateRun is what a schedule did for one of its due times.
type TemplateRun struct {
	ScheduledFor time.Time
	Status       string // "created" | "failed"
	CardID       *int64
	Violations   []rules.Violation
}

// TemplateInput is what the settings form saves.
type TemplateInput struct {
	Name, Title, Description string
	Priority                 *int16
	Estimate                 *float64
	AssigneeID               *int64
	ColumnID                 *int64
	DueInDays                *int
	LabelIDs                 []int64
	Checklist                []string
	Schedule                 Schedule
}

const templateColumns = `id, board_id, name, title, description, priority, estimate::float8, assignee_id, column_id,
	due_in_days, schedule_kind, schedule_weekdays, schedule_monthday, schedule_time, schedule_since, updated_by, updated_at`

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// Templates lists the board's templates by name, each with its latest run.
func (s *Store) Templates(ctx context.Context, boardID int64) ([]Template, error) {
	return s.loadTemplates(ctx, boardID, 0)
}

// Template returns one template of the board, or ErrNotFound.
func (s *Store) Template(ctx context.Context, boardID, id int64) (Template, error) {
	ts, err := s.loadTemplates(ctx, boardID, id)
	if err != nil {
		return Template{}, err
	}
	if len(ts) == 0 {
		return Template{}, ErrNotFound
	}
	return ts[0], nil
}

// loadTemplates reads the board's templates (only id, when it is not 0) with
// their labels, checklist and latest run.
func (s *Store) loadTemplates(ctx context.Context, boardID, id int64) ([]Template, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+templateColumns+` FROM card_templates
		WHERE board_id = $1 AND ($2 = 0 OR id = $2)
		ORDER BY lower(name), id`, boardID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Template
	index := map[int64]int{}
	var ids []int64
	for rows.Next() {
		var t Template
		var at pgtype.Time
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Name, &t.Title, &t.Description, &t.Priority, &t.Estimate, &t.AssigneeID,
			&t.ColumnID, &t.DueInDays, &t.Schedule.Kind, &t.Schedule.Weekdays, &t.Schedule.MonthDay, &at, &t.ScheduleSince,
			&t.UpdatedBy, &t.UpdatedAt); err != nil {
			return nil, err
		}
		minutes := int(at.Microseconds / int64(time.Minute/time.Microsecond))
		t.Schedule.Hour, t.Schedule.Minute = minutes/60, minutes%60
		index[t.ID] = len(out)
		ids = append(ids, t.ID)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}

	lrows, err := s.pool.Query(ctx, `SELECT template_id, label_id FROM card_template_labels WHERE template_id = ANY($1) ORDER BY label_id`, ids)
	if err != nil {
		return nil, err
	}
	defer lrows.Close()
	for lrows.Next() {
		var tid, lid int64
		if err := lrows.Scan(&tid, &lid); err != nil {
			return nil, err
		}
		out[index[tid]].LabelIDs = append(out[index[tid]].LabelIDs, lid)
	}
	if err := lrows.Err(); err != nil {
		return nil, err
	}

	crows, err := s.pool.Query(ctx, `SELECT template_id, text FROM card_template_checklist WHERE template_id = ANY($1) ORDER BY template_id, position`, ids)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var tid int64
		var text string
		if err := crows.Scan(&tid, &text); err != nil {
			return nil, err
		}
		out[index[tid]].Checklist = append(out[index[tid]].Checklist, text)
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}

	rrows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (template_id) template_id, scheduled_for, status, card_id, violations
		FROM template_runs WHERE template_id = ANY($1)
		ORDER BY template_id, scheduled_for DESC`, ids)
	if err != nil {
		return nil, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var tid int64
		var run TemplateRun
		var raw []byte
		if err := rrows.Scan(&tid, &run.ScheduledFor, &run.Status, &run.CardID, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &run.Violations); err != nil {
			return nil, err
		}
		out[index[tid]].LastRun = &run
	}
	return out, rrows.Err()
}

// CreateTemplate adds a template to the board. Labels of other boards are
// ignored; a column of another board is ErrNotFound.
func (s *Store) CreateTemplate(ctx context.Context, boardID int64, in TemplateInput, actorID int64) (Template, error) {
	return s.saveTemplate(ctx, boardID, 0, in, actorID)
}

// UpdateTemplate replaces a template's settings. The schedule restarts (its
// schedule_since moves to now) only when the schedule itself changed.
func (s *Store) UpdateTemplate(ctx context.Context, boardID, id int64, in TemplateInput, actorID int64) (Template, error) {
	return s.saveTemplate(ctx, boardID, id, in, actorID)
}

// saveTemplate creates the template when id is 0, otherwise updates it.
func (s *Store) saveTemplate(ctx context.Context, boardID, id int64, in TemplateInput, actorID int64) (Template, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)

	if in.ColumnID != nil {
		var owned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM columns WHERE id = $1 AND board_id = $2)`, *in.ColumnID, boardID).Scan(&owned); err != nil {
			return Template{}, err
		}
		if !owned {
			return Template{}, ErrNotFound
		}
	}
	sc := in.Schedule
	monthDay := sc.MonthDay
	if monthDay == 0 {
		monthDay = 1 // unused unless monthly; keeps the column's CHECK happy
	}
	at := fmt.Sprintf("%02d:%02d", sc.Hour, sc.Minute)
	if id == 0 {
		err = tx.QueryRow(ctx, `
			INSERT INTO card_templates (board_id, name, title, description, priority, estimate, assignee_id, column_id, due_in_days,
				schedule_kind, schedule_weekdays, schedule_monthday, schedule_time, schedule_since, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::time,
				CASE WHEN $10 <> '' THEN now() END, $14)
			RETURNING id`,
			boardID, in.Name, in.Title, in.Description, in.Priority, in.Estimate, in.AssigneeID, in.ColumnID, in.DueInDays,
			sc.Kind, int16(sc.Weekdays), int16(monthDay), at, actorID).Scan(&id)
	} else {
		// Right-hand sides see the row as it was, so the comparison is old against new.
		err = exactlyOne(tx.Exec(ctx, `
			UPDATE card_templates SET name = $3, title = $4, description = $5, priority = $6, estimate = $7,
				assignee_id = $8, column_id = $9, due_in_days = $10,
				schedule_since = CASE
					WHEN $11 = '' THEN NULL
					WHEN schedule_since IS NULL OR schedule_kind <> $11 OR schedule_weekdays <> $12
						OR schedule_monthday <> $13 OR schedule_time <> $14::time THEN now()
					ELSE schedule_since END,
				schedule_kind = $11, schedule_weekdays = $12, schedule_monthday = $13, schedule_time = $14::time,
				updated_by = $15, updated_at = now()
			WHERE id = $2 AND board_id = $1`,
			boardID, id, in.Name, in.Title, in.Description, in.Priority, in.Estimate, in.AssigneeID, in.ColumnID, in.DueInDays,
			sc.Kind, int16(sc.Weekdays), int16(monthDay), at, actorID))
	}
	if err != nil {
		if isUniqueViolation(err) {
			return Template{}, ErrTemplateName
		}
		return Template{}, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM card_template_labels WHERE template_id = $1`, id); err != nil {
		return Template{}, err
	}
	if len(in.LabelIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO card_template_labels (template_id, label_id)
			SELECT $1, id FROM labels WHERE board_id = $2 AND id = ANY($3)`, id, boardID, in.LabelIDs); err != nil {
			return Template{}, err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM card_template_checklist WHERE template_id = $1`, id); err != nil {
		return Template{}, err
	}
	for i, text := range in.Checklist {
		if _, err := tx.Exec(ctx, `INSERT INTO card_template_checklist (template_id, position, text) VALUES ($1, $2, $3)`, id, i, text); err != nil {
			return Template{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return s.Template(ctx, boardID, id)
}

// DeleteTemplate removes a template, with its labels, checklist and runs.
func (s *Store) DeleteTemplate(ctx context.Context, boardID, id int64) error {
	return exactlyOne(s.pool.Exec(ctx, `DELETE FROM card_templates WHERE id = $2 AND board_id = $1`, boardID, id))
}
