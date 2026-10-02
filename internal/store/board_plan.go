package store

import (
	"context"
	"fmt"
	"strings"
)

// BoardPlan is everything a new board is built with, in one transaction:
// the indexes in it point into its own Columns and Labels.
type BoardPlan struct {
	Columns     []PlanColumn
	Labels      []PlanLabel
	Templates   []PlanTemplate
	PersonWIP   *int
	Permissions []PlanPermission
	Conditions  []PlanCondition
}

// PlanColumn is a column of a planned board.
type PlanColumn struct {
	Name                               string
	WIP                                *int
	AllowCreate, Done, CountsPersonWIP bool
}

// PlanLabel is a label of a planned board; Color is "#rrggbb".
type PlanLabel struct{ Name, Color string }

// PlanTemplate is a card template of a planned board.
type PlanTemplate struct {
	Name, Title, Description string
	Priority                 *int16
	DueInDays                *int
	ColumnIndex              int
	LabelIndexes             []int
	Checklist                []string
	Schedule                 Schedule
}

// PlanPermission says who may move cards into a planned column.
type PlanPermission struct {
	ToIndex   int
	FromIndex *int
	Subject   string
}

// PlanCondition gates entering or leaving a planned column.
type PlanCondition struct {
	ColumnIndex int
	Phase, Kind string
}

// ColumnsPlan is a plan of columns alone, named in order: the first takes new
// cards and the last, when there are two or more, is done.
func ColumnsPlan(names []string) BoardPlan {
	var p BoardPlan
	for i, n := range names {
		p.Columns = append(p.Columns, PlanColumn{Name: n, AllowCreate: i == 0, Done: i == len(names)-1 && len(names) > 1})
	}
	return p
}

// CreateBoardFromPlan builds a board from plan in one transaction: when any
// part fails, nothing of it is left. Card templates are written by actorID.
func (s *Store) CreateBoardFromPlan(ctx context.Context, teamID int64, name string, plan BoardPlan, actorID int64) (Board, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback(ctx)
	board, err := scanBoard(tx.QueryRow(ctx,
		`INSERT INTO boards (team_id, name, person_wip_limit) VALUES ($1, $2, $3) RETURNING `+boardColumns,
		teamID, name, plan.PersonWIP))
	if err != nil {
		return Board{}, err
	}
	cols := make([]int64, len(plan.Columns))
	for i, c := range plan.Columns {
		if err := tx.QueryRow(ctx, `
			INSERT INTO columns (board_id, name, position, wip_limit, allow_create, is_done, counts_person_wip)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			board.ID, c.Name, i, c.WIP, c.AllowCreate, c.Done, c.CountsPersonWIP).Scan(&cols[i]); err != nil {
			return Board{}, err
		}
	}
	column := func(i int) (int64, error) {
		if i < 0 || i >= len(cols) {
			return 0, fmt.Errorf("store: plan names column %d of %d", i, len(cols))
		}
		return cols[i], nil
	}
	labels := make([]int64, len(plan.Labels))
	for i, l := range plan.Labels {
		if err := tx.QueryRow(ctx, `INSERT INTO labels (board_id, name, color) VALUES ($1, $2, $3) RETURNING id`,
			board.ID, l.Name, strings.ToLower(l.Color)).Scan(&labels[i]); err != nil {
			return Board{}, err
		}
	}
	for _, t := range plan.Templates {
		col, err := column(t.ColumnIndex)
		if err != nil {
			return Board{}, err
		}
		in := TemplateInput{Name: t.Name, Title: t.Title, Description: t.Description, Priority: t.Priority,
			ColumnID: &col, DueInDays: t.DueInDays, Checklist: t.Checklist, Schedule: t.Schedule}
		for _, li := range t.LabelIndexes {
			if li < 0 || li >= len(labels) {
				return Board{}, fmt.Errorf("store: plan names label %d of %d", li, len(labels))
			}
			in.LabelIDs = append(in.LabelIDs, labels[li])
		}
		if _, err := writeTemplate(ctx, tx, board.ID, 0, in, actorID); err != nil {
			return Board{}, err
		}
	}
	for _, p := range plan.Permissions {
		to, err := column(p.ToIndex)
		if err != nil {
			return Board{}, err
		}
		var from *int64
		if p.FromIndex != nil {
			id, err := column(*p.FromIndex)
			if err != nil {
				return Board{}, err
			}
			from = &id
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO move_permissions (board_id, to_column_id, from_column_id, subject) VALUES ($1, $2, $3, $4)`,
			board.ID, to, from, p.Subject); err != nil {
			return Board{}, err
		}
	}
	for _, c := range plan.Conditions {
		col, err := column(c.ColumnIndex)
		if err != nil {
			return Board{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO column_conditions (column_id, phase, kind) VALUES ($1, $2, $3)`,
			col, c.Phase, c.Kind); err != nil {
			return Board{}, err
		}
	}
	return board, tx.Commit(ctx)
}
