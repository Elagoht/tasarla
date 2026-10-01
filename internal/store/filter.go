package store

import (
	"context"
	"time"
)

// DueFilter picks cards by their due date, against today.
type DueFilter string

const (
	DueAny     DueFilter = ""
	DueOverdue DueFilter = "overdue" // before today
	DueToday   DueFilter = "today"
	DueWeek    DueFilter = "week" // today to the coming Sunday
	DueNone    DueFilter = "none" // no due date
)

// BoardFilter is what a board's cards are matched against (spec 2026-10-01
// filtre… §3.1). Dimensions are and-ed; the values within one are or-ed.
type BoardFilter struct {
	Text        string // matched in the title, the description and comments
	AssigneeIDs []int64
	Unassigned  bool
	LabelIDs    []int64
	Priorities  []int16
	Due         DueFilter
	Today       time.Time // the date today is, in the application's zone
}

// Empty reports whether the filter lets every card through.
func (f BoardFilter) Empty() bool {
	return f.Text == "" && len(f.AssigneeIDs) == 0 && !f.Unassigned && len(f.LabelIDs) == 0 &&
		len(f.Priorities) == 0 && f.Due == DueAny
}

// MatchingCardIDs returns the board's cards, done ones included, that are not
// archived and match f. An empty filter is nil: every card matches.
func (s *Store) MatchingCardIDs(ctx context.Context, boardID int64, f BoardFilter) (map[int64]bool, error) {
	if f.Empty() {
		return nil, nil
	}
	today := f.Today.Format(time.DateOnly)
	weekday := (int(f.Today.Weekday()) + 6) % 7 // Monday = 0
	sunday := f.Today.AddDate(0, 0, 6-weekday).Format(time.DateOnly)
	rows, err := s.pool.Query(ctx, `
		SELECT k.id FROM cards k
		WHERE k.board_id = $1 AND k.archived_at IS NULL
		  AND ($2 = '' OR kanban_fold(k.title) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		       OR kanban_fold(k.description) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		       OR EXISTS (SELECT 1 FROM comments c
		                  WHERE c.card_id = k.id AND c.deleted_at IS NULL
		                    AND kanban_fold(c.body) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'))
		  AND ((cardinality($3::bigint[]) = 0 AND NOT $4)
		       OR k.assignee_id = ANY ($3) OR ($4 AND k.assignee_id IS NULL))
		  AND (cardinality($5::bigint[]) = 0
		       OR EXISTS (SELECT 1 FROM card_labels cl WHERE cl.card_id = k.id AND cl.label_id = ANY ($5)))
		  AND (cardinality($6::smallint[]) = 0 OR k.priority = ANY ($6))
		  AND CASE $7
		        WHEN 'overdue' THEN k.due_date < $8::date
		        WHEN 'today' THEN k.due_date = $8::date
		        WHEN 'week' THEN k.due_date BETWEEN $8::date AND $9::date
		        WHEN 'none' THEN k.due_date IS NULL
		        ELSE true
		      END`,
		boardID, likeEscaper.Replace(f.Text), orEmpty(f.AssigneeIDs), f.Unassigned,
		orEmpty(f.LabelIDs), orEmpty(f.Priorities), string(f.Due), today, sunday)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// orEmpty sends a nil slice as an empty array, not NULL: cardinality(NULL) is
// NULL, which would make the whole condition false.
func orEmpty[T int64 | int16](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
