package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// GanttCard is a card as the board's Gantt chart shows it.
type GanttCard struct {
	Card         Card
	AssigneeName string
}

// GanttCards returns a board's cards that are not archived, in column order:
// those on the board, and with done the completed ones too.
func (s *Store) GanttCards(ctx context.Context, boardID int64, done bool) ([]GanttCard, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+`, coalesce(u.name, '')
		FROM cards k
		JOIN columns col ON col.id = k.column_id
		LEFT JOIN users u ON u.id = k.assignee_id
		WHERE k.board_id = $1 AND k.archived_at IS NULL AND ($2 OR k.completed_at IS NULL)
		ORDER BY col.position, k.completed_at NULLS FIRST, k.position, k.id`, boardID, done)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GanttCard
	for rows.Next() {
		var g GanttCard
		c := &g.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt, &c.CompletedAt, &c.CompletedFrom, &c.StartDate,
			&g.AssigneeName); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Dependency is one card blocking another.
type Dependency struct {
	BlockerID int64
	BlockedID int64
}

// BoardDependencies returns every dependency between the cards of a board.
func (s *Store) BoardDependencies(ctx context.Context, boardID int64) ([]Dependency, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.blocker_id, d.blocked_id FROM card_dependencies d
		JOIN cards k ON k.id = d.blocked_id
		WHERE k.board_id = $1
		ORDER BY d.blocker_id, d.blocked_id`, boardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Dependency])
}
