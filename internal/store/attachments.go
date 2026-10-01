package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Attachment is a file attached to a card.
type Attachment struct {
	ID           int64
	CardID       int64
	BoardID      int64
	UploaderID   int64
	UploaderName string
	Filename     string
	ContentType  string
	Size         int64
	StorageKey   string
	CreatedAt    time.Time
}

const attachmentSelect = `
	SELECT a.id, a.card_id, k.board_id, a.uploader_id, u.name, a.filename, a.content_type, a.size, a.storage_key, a.created_at
	FROM attachments a JOIN cards k ON k.id = a.card_id JOIN users u ON u.id = a.uploader_id`

// AddAttachment records a saved file on a card of boardID.
func (s *Store) AddAttachment(ctx context.Context, boardID, cardID int64, a Attachment) (Attachment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Attachment{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return Attachment{}, err
	}
	if err := cardOfBoard(ctx, tx, boardID, cardID); err != nil {
		return Attachment{}, err
	}
	a.CardID, a.BoardID = cardID, boardID
	if err := tx.QueryRow(ctx, `
		INSERT INTO attachments (card_id, uploader_id, filename, content_type, size, storage_key)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		cardID, a.UploaderID, a.Filename, a.ContentType, a.Size, a.StorageKey).Scan(&a.ID, &a.CreatedAt); err != nil {
		return Attachment{}, err
	}
	if err := logActivity(ctx, tx, boardID, &cardID, a.UploaderID, ActivityAttachmentAdded, ActivityPayload{Filename: a.Filename}); err != nil {
		return Attachment{}, err
	}
	return a, tx.Commit(ctx)
}

// CardAttachments returns a card's files, oldest first.
func (s *Store) CardAttachments(ctx context.Context, cardID int64) ([]Attachment, error) {
	rows, err := s.pool.Query(ctx, attachmentSelect+` WHERE a.card_id = $1 ORDER BY a.id`, cardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Attachment])
}

// AttachmentFor returns an attachment that user may see: one on a board of a
// team they belong to, or any for an admin. Anything else is ErrNotFound, so
// the existence of another team's file does not leak (spec §6).
func (s *Store) AttachmentFor(ctx context.Context, id int64, user User) (Attachment, error) {
	rows, err := s.pool.Query(ctx, attachmentSelect+`
		JOIN boards b ON b.id = k.board_id
		WHERE a.id = $1 AND b.archived_at IS NULL
		  AND ($3 OR EXISTS (SELECT 1 FROM team_members m WHERE m.team_id = b.team_id AND m.user_id = $2))`,
		id, user.ID, user.IsAdmin)
	if err != nil {
		return Attachment{}, err
	}
	a, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[Attachment])
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	return a, err
}

// DeleteAttachment removes an attachment of a card of boardID; its uploader may,
// and so may whoever manages the board. It returns the storage key, for the
// file to be removed too.
func (s *Store) DeleteAttachment(ctx context.Context, boardID, cardID, id, userID int64, manager bool) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return "", err
	}
	var key, filename string
	err = tx.QueryRow(ctx, `
		DELETE FROM attachments a USING cards k
		WHERE a.id = $1 AND a.card_id = $2 AND k.id = a.card_id AND k.board_id = $3 AND (a.uploader_id = $4 OR $5)
		RETURNING a.storage_key, a.filename`, id, cardID, boardID, userID, manager).Scan(&key, &filename)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if err := logActivity(ctx, tx, boardID, &cardID, userID, ActivityAttachmentDeleted, ActivityPayload{Filename: filename}); err != nil {
		return "", err
	}
	return key, tx.Commit(ctx)
}
