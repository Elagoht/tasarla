package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Activity kinds.
const (
	ActivityCardCreated       = "card_created"
	ActivityCardMoved         = "card_moved"
	ActivityCardUpdated       = "card_updated"
	ActivityCardArchived      = "card_archived"
	ActivityCardCompleted     = "card_completed"
	ActivityCardReopened      = "card_reopened"
	ActivityLabelsChanged     = "labels_changed"
	ActivityChecklistAdded    = "checklist_added"
	ActivityChecklistChecked  = "checklist_checked"
	ActivityDependencyAdded   = "dependency_added"
	ActivityDependencyRemoved = "dependency_removed"
	ActivityCommentAdded      = "comment_added"
	ActivityAttachmentAdded   = "attachment_added"
	ActivityAttachmentDeleted = "attachment_deleted"
)

// ActivityPayload is what an activity entry records beyond its kind.
type ActivityPayload struct {
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	Fields   []string `json:"fields,omitempty"`
	Title    string   `json:"title,omitempty"`
	Text     string   `json:"text,omitempty"`
	Filename string   `json:"filename,omitempty"`
	// Changes are a card edit's values, before and after; entries written
	// before they were kept have only Fields.
	Changes []FieldChange `json:"changes,omitempty"`
}

// FieldChange is one field of a card edit: its name as in Fields, and its
// value before and after, raw ("" for none): a date as YYYY-MM-DD, a priority
// as its number, an assignee as their name.
type FieldChange struct {
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// Activity is one entry of a board's history.
type Activity struct {
	ID        int64
	BoardID   int64
	CardID    *int64
	ActorName string
	Kind      string
	Payload   ActivityPayload
	CreatedAt time.Time
	CardTitle string
}

// logActivity writes an entry inside tx. actorID 0 records no actor.
func logActivity(ctx context.Context, tx pgx.Tx, boardID int64, cardID *int64, actorID int64, kind string, p ActivityPayload) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO activity (board_id, card_id, actor_id, kind, payload)
		VALUES ($1, $2, NULLIF($3, 0), $4, $5)`, boardID, cardID, actorID, kind, p)
	return err
}

// LogActivity records something done to a board or a card outside the
// transactions that write their own entries.
func (s *Store) LogActivity(ctx context.Context, boardID int64, cardID *int64, actorID int64, kind string, p ActivityPayload) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO activity (board_id, card_id, actor_id, kind, payload)
		VALUES ($1, $2, NULLIF($3, 0), $4, $5)`, boardID, cardID, actorID, kind, p)
	return err
}

const activitySelect = `
	SELECT a.id, a.board_id, a.card_id, coalesce(u.name, ''), a.kind, a.payload, a.created_at, coalesce(c.title, '')
	FROM activity a
	LEFT JOIN users u ON u.id = a.actor_id
	LEFT JOIN cards c ON c.id = a.card_id`

// CardActivity returns a card's newest entries first.
func (s *Store) CardActivity(ctx context.Context, cardID int64, limit int) ([]Activity, error) {
	rows, err := s.pool.Query(ctx, activitySelect+` WHERE a.card_id = $1 ORDER BY a.id DESC LIMIT $2`, cardID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Activity])
}

// BoardActivity returns a board's newest entries first.
func (s *Store) BoardActivity(ctx context.Context, boardID int64, limit int) ([]Activity, error) {
	rows, err := s.pool.Query(ctx, activitySelect+` WHERE a.board_id = $1 ORDER BY a.id DESC LIMIT $2`, boardID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Activity])
}
