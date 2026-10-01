package store

import (
	"context"
	"unicode/utf8"
)

// SearchPageSize is how many hits one page of a search shows.
const SearchPageSize = 50

// SearchHit is a card a search found.
type SearchHit struct {
	Card      Card
	BoardName string
	TeamName  string
	InTitle   bool
	// Excerpt is the description or the latest live comment the query was
	// found in, raw; "" when only the title holds it.
	Excerpt string
}

// Search finds the cards of the reader's teams' boards, done and archived
// ones included, whose title, description or a live comment holds q (spec
// 2026-10-01 filtre… §3.2). Title matches come first, then the closer ones,
// then the more recently touched. page counts from 1; more reports whether a
// further page has hits.
func (s *Store) Search(ctx context.Context, userID int64, q string, page int) ([]SearchHit, bool, error) {
	if utf8.RuneCountInString(q) < 2 {
		return nil, false, nil
	}
	if page < 1 {
		page = 1
	}
	rows, err := s.pool.Query(ctx, `
		WITH found AS (
		  SELECT k.*, b.name AS board_name, t.name AS team_name,
		         kanban_fold(k.title) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\' AS in_title,
		         CASE WHEN kanban_fold(k.description) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		              THEN k.description END AS in_description,
		         (SELECT c.body FROM comments c
		          WHERE c.card_id = k.id AND c.deleted_at IS NULL
		            AND kanban_fold(c.body) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		          ORDER BY c.id DESC LIMIT 1) AS in_comment
		  FROM cards k
		  JOIN boards b ON b.id = k.board_id AND b.archived_at IS NULL
		  JOIN teams t ON t.id = b.team_id
		  JOIN team_members m ON m.team_id = b.team_id AND m.user_id = $1
		)
		SELECT `+prefixed("f", cardColumns)+`, f.board_name, f.team_name, f.in_title,
		       coalesce(CASE WHEN f.in_title THEN '' ELSE coalesce(f.in_description, f.in_comment) END, '')
		FROM found f
		WHERE f.in_title OR f.in_description IS NOT NULL OR f.in_comment IS NOT NULL
		ORDER BY f.in_title DESC,
		         similarity(kanban_fold(f.title), kanban_fold($3)) DESC,
		         coalesce((SELECT max(a.created_at) FROM activity a WHERE a.card_id = f.id), f.created_at) DESC,
		         f.id DESC
		LIMIT $4 OFFSET $5`,
		userID, likeEscaper.Replace(q), q, SearchPageSize+1, (page-1)*SearchPageSize)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		c := &h.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt, &c.CompletedAt, &c.CompletedFrom, &c.StartDate,
			&h.BoardName, &h.TeamName, &h.InTitle, &h.Excerpt); err != nil {
			return nil, false, err
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(hits) > SearchPageSize
	if more {
		hits = hits[:SearchPageSize]
	}
	return hits, more, nil
}
