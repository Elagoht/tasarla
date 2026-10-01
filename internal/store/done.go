package store

import "context"

// DoneCard is a completed card of a board.
type DoneCard struct {
	Card Card
	// CompletedBy is who moved it into a done column, "" when the history
	// does not say.
	CompletedBy string
}

// DoneCards returns a board's completed cards that are not archived, last
// completed first; with query, only those whose title holds it.
func (s *Store) DoneCards(ctx context.Context, boardID int64, query string) ([]DoneCard, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+`,
		       coalesce((SELECT u.name FROM activity a JOIN users u ON u.id = a.actor_id
		                 WHERE a.card_id = k.id AND a.kind = $3 ORDER BY a.id DESC LIMIT 1), '')
		FROM cards k
		WHERE k.board_id = $1 AND k.completed_at IS NOT NULL AND k.archived_at IS NULL
		  AND ($2 = '' OR k.title ILIKE '%' || $2 || '%' ESCAPE '\')
		ORDER BY k.completed_at DESC, k.id DESC
		LIMIT 500`, boardID, likeEscaper.Replace(query), ActivityCardCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DoneCard
	for rows.Next() {
		var d DoneCard
		c := &d.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt, &c.CompletedAt, &c.CompletedFrom, &c.StartDate,
			&d.CompletedBy); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
