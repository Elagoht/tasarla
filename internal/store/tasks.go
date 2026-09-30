package store

import (
	"context"
)

// Task is a card assigned to someone, with where it is.
type Task struct {
	Card       Card
	BoardName  string
	ColumnName string
	TeamName   string
}

// AssignedTo returns the cards assigned to userID, not archived, on boards of
// teams they belong to, by board and position.
func (s *Store) AssignedTo(ctx context.Context, userID int64) ([]Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+`, b.name, col.name, t.name
		FROM cards k
		JOIN boards b ON b.id = k.board_id
		JOIN teams t ON t.id = b.team_id
		JOIN columns col ON col.id = k.column_id
		JOIN team_members m ON m.team_id = b.team_id AND m.user_id = $1
		WHERE k.assignee_id = $1 AND k.archived_at IS NULL AND b.archived_at IS NULL
		ORDER BY lower(b.name), b.id, col.position, k.position`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		c := &t.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt,
			&t.BoardName, &t.ColumnName, &t.TeamName); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
