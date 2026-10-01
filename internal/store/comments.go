package store

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Comment is one comment on a card. A deleted comment keeps its place and
// loses its body.
type Comment struct {
	ID         int64
	CardID     int64
	AuthorID   int64
	AuthorName string
	Body       string
	EditedAt   *time.Time
	DeletedAt  *time.Time
	CreatedAt  time.Time
	MentionIDs []int64
}

var mentionPattern = regexp.MustCompile(`(^|[^\p{L}\p{N}._@-])@([\p{L}\p{N}._-]+)`)

// MentionHandle is how a member is written after @: the part of their email
// before the @.
func MentionHandle(u User) string {
	local, _, _ := strings.Cut(u.Email, "@")
	return strings.ToLower(local)
}

// ResolveMentions finds the members a text mentions with @handle, where a
// handle is a member's MentionHandle or their name without spaces, in any
// case. A handle that names no member is left as text. Ids come in the order
// first mentioned, once each.
func ResolveMentions(text string, members []Member) []int64 {
	var ids []int64
	for _, m := range mentionPattern.FindAllStringSubmatch(text, -1) {
		handle := strings.ToLower(strings.TrimRight(m[2], "._-"))
		for _, member := range members {
			name := strings.ToLower(strings.Join(strings.Fields(member.User.Name), ""))
			if (handle == MentionHandle(member.User) || handle == name) && !slices.Contains(ids, member.User.ID) {
				ids = append(ids, member.User.ID)
			}
		}
	}
	return ids
}

// AddComment adds a comment to a card of boardID and records who it mentions.
func (s *Store) AddComment(ctx context.Context, boardID, cardID, authorID int64, body string, mentions []int64) (Comment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Comment{}, err
	}
	defer tx.Rollback(ctx)
	if err := cardOfBoard(ctx, tx, boardID, cardID); err != nil {
		return Comment{}, err
	}
	c := Comment{CardID: cardID, AuthorID: authorID, Body: body}
	if err := tx.QueryRow(ctx, `
		INSERT INTO comments (card_id, author_id, body) VALUES ($1, $2, $3) RETURNING id, created_at`,
		cardID, authorID, body).Scan(&c.ID, &c.CreatedAt); err != nil {
		return Comment{}, err
	}
	if err := setMentions(ctx, tx, c.ID, mentions); err != nil {
		return Comment{}, err
	}
	excerpt := []rune(body)
	if len(excerpt) > 80 {
		excerpt = append(excerpt[:80], '…')
	}
	if err := logActivity(ctx, tx, boardID, &cardID, authorID, ActivityCommentAdded, ActivityPayload{Text: string(excerpt)}); err != nil {
		return Comment{}, err
	}
	return c, tx.Commit(ctx)
}

// EditComment changes the body of an author's own comment.
func (s *Store) EditComment(ctx context.Context, boardID, cardID, commentID, authorID int64, body string, mentions []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := cardOfBoard(ctx, tx, boardID, cardID); err != nil {
		return err
	}
	if err := exactlyOne(tx.Exec(ctx, `
		UPDATE comments SET body = $4, edited_at = now()
		WHERE id = $1 AND card_id = $2 AND author_id = $3 AND deleted_at IS NULL`,
		commentID, cardID, authorID, body)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM comment_mentions WHERE comment_id = $1`, commentID); err != nil {
		return err
	}
	if err := setMentions(ctx, tx, commentID, mentions); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteComment hides a comment: the author may, and so may whoever manages
// the board.
func (s *Store) DeleteComment(ctx context.Context, boardID, cardID, commentID, userID int64, manager bool) error {
	return exactlyOne(s.pool.Exec(ctx, `
		UPDATE comments c SET deleted_at = now()
		FROM cards k
		WHERE c.id = $1 AND c.card_id = $2 AND k.id = c.card_id AND k.board_id = $3
		  AND c.deleted_at IS NULL AND (c.author_id = $4 OR $5)`,
		commentID, cardID, boardID, userID, manager))
}

// CardComments returns a card's comments, oldest first.
func (s *Store) CardComments(ctx context.Context, cardID int64) ([]Comment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.card_id, c.author_id, u.name,
		       CASE WHEN c.deleted_at IS NULL THEN c.body ELSE '' END,
		       c.edited_at, c.deleted_at, c.created_at,
		       coalesce((SELECT array_agg(m.user_id ORDER BY m.user_id) FROM comment_mentions m WHERE m.comment_id = c.id), '{}')
		FROM comments c JOIN users u ON u.id = c.author_id
		WHERE c.card_id = $1 ORDER BY c.id`, cardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Comment])
}

func setMentions(ctx context.Context, tx pgx.Tx, commentID int64, userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO comment_mentions (comment_id, user_id)
		SELECT $1, unnest($2::bigint[]) ON CONFLICT DO NOTHING`, commentID, userIDs)
	return err
}

// cardOfBoard reports ErrNotFound unless cardID is a card of boardID.
func cardOfBoard(ctx context.Context, tx pgx.Tx, boardID, cardID int64) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM cards WHERE id = $1 AND board_id = $2`, cardID, boardID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
