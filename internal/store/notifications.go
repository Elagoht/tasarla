package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Notification kinds (spec §10).
const (
	NotifyAssigned  = "assigned"
	NotifyMentioned = "mentioned"
	NotifyCommented = "commented"
	NotifyDueSoon   = "due_soon"
	NotifyOverdue   = "overdue"
	NotifyUnblocked = "unblocked"
)

// NotifyKinds lists every kind, in the order settings show them.
var NotifyKinds = []string{NotifyAssigned, NotifyMentioned, NotifyCommented, NotifyDueSoon, NotifyOverdue, NotifyUnblocked}

// emailByDefault is each kind's e-mail setting until a user changes it.
var emailByDefault = map[string]bool{
	NotifyAssigned: true, NotifyMentioned: true, NotifyCommented: false,
	NotifyDueSoon: true, NotifyOverdue: true, NotifyUnblocked: false,
}

// NotificationPayload is what a notification shows.
type NotificationPayload struct {
	CardTitle string `json:"card_title,omitempty"`
	BoardID   int64  `json:"board_id,omitempty"`
	BoardName string `json:"board_name,omitempty"`
	ActorName string `json:"actor_name,omitempty"`
	Text      string `json:"text,omitempty"`
	DueDate   string `json:"due_date,omitempty"`
}

// Notification is one in-app notification.
type Notification struct {
	ID        int64
	Kind      string
	CardID    *int64
	Payload   NotificationPayload
	ReadAt    *time.Time
	CreatedAt time.Time
}

// OutboxMessage is an e-mail waiting to be sent.
type OutboxMessage struct {
	ID      int64
	To      string
	Subject string
	HTML    string
	Text    string
}

// NewNotification is a notification to create, and the e-mail to queue with it.
type NewNotification struct {
	UserID    int64
	Kind      string
	CardID    *int64
	Payload   NotificationPayload
	DedupeKey string // "" for none
	Email     *OutboxMessage
}

// CreateNotification writes a notification and its e-mail in one transaction
// (the outbox pattern, spec §10). A notification whose DedupeKey already exists
// is not created again, and created is false.
func (s *Store) CreateNotification(ctx context.Context, n NewNotification) (created bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var dedupe *string
	if n.DedupeKey != "" {
		dedupe = &n.DedupeKey
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO notifications (user_id, kind, card_id, payload, dedupe_key) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (dedupe_key) DO NOTHING`, n.UserID, n.Kind, n.CardID, n.Payload, dedupe)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if m := n.Email; m != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO email_outbox (to_address, subject, html, text) VALUES ($1, $2, $3, $4)`,
			m.To, m.Subject, m.HTML, m.Text); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}

// Notifications returns a user's newest notifications first.
func (s *Store) Notifications(ctx context.Context, userID int64, limit int) ([]Notification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, card_id, payload, read_at, created_at FROM notifications
		WHERE user_id = $1 ORDER BY id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Notification])
}

// UnreadCount counts a user's unread notifications.
func (s *Store) UnreadCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

// MarkRead marks one of the user's notifications read.
func (s *Store) MarkRead(ctx context.Context, userID, id int64) error {
	return exactlyOne(s.pool.Exec(ctx,
		`UPDATE notifications SET read_at = coalesce(read_at, now()) WHERE id = $2 AND user_id = $1`, userID, id))
}

// MarkAllRead marks every notification of the user read.
func (s *Store) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, userID)
	return err
}

// NotificationPrefs returns whether the user wants e-mail for each kind.
func (s *Store) NotificationPrefs(ctx context.Context, userID int64) (map[string]bool, error) {
	prefs := map[string]bool{}
	for k, v := range emailByDefault {
		prefs[k] = v
	}
	rows, err := s.pool.Query(ctx, `SELECT kind, email FROM notification_prefs WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var email bool
		if err := rows.Scan(&kind, &email); err != nil {
			return nil, err
		}
		prefs[kind] = email
	}
	return prefs, rows.Err()
}

// SetNotificationPrefs stores the user's e-mail choice for the kinds given.
func (s *Store) SetNotificationPrefs(ctx context.Context, userID int64, prefs map[string]bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for kind, email := range prefs {
		if _, ok := emailByDefault[kind]; !ok {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO notification_prefs (user_id, kind, email) VALUES ($1, $2, $3)
			ON CONFLICT (user_id, kind) DO UPDATE SET email = EXCLUDED.email`, userID, kind, email); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SetLocale sets the language a user's e-mails are written in.
func (s *Store) SetLocale(ctx context.Context, userID int64, locale string) error {
	return exactlyOne(s.pool.Exec(ctx, `UPDATE users SET locale = $2 WHERE id = $1`, userID, locale))
}

// maxAttempts is how often an e-mail is tried before it is given up (spec §10).
const maxAttempts = 8

// claimLease is how long a claimed e-mail is kept from other workers while it
// is being sent; a worker that dies mid-send leaves it to be tried again after.
const claimLease = 10 * time.Minute

// ProcessOutbox sends up to limit e-mails due at now, one at a time: each is
// claimed in a short transaction (FOR UPDATE SKIP LOCKED), sent with no lock
// held, and its result recorded at once, even when ctx is cancelled, so a sent
// e-mail is never sent again after a shutdown. A failure is retried 2^attempts
// minutes later, at most six hours; after maxAttempts the e-mail is marked
// failed and handed to gaveUp. No new e-mail is claimed once ctx is done.
func (s *Store) ProcessOutbox(ctx context.Context, now time.Time, limit int, send func(OutboxMessage) error, gaveUp func(OutboxMessage, error)) (sent, failed int, err error) {
	record := context.WithoutCancel(ctx)
	for range limit {
		if ctx.Err() != nil {
			return sent, failed, ctx.Err()
		}
		var m OutboxMessage
		var attempts int
		err := s.pool.QueryRow(ctx, `
			UPDATE email_outbox SET next_attempt_at = $2::timestamptz + $3::interval
			WHERE id = (SELECT id FROM email_outbox WHERE status = 'pending' AND next_attempt_at <= $1
			            ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED)
			RETURNING id, to_address, subject, html, text, attempts`, now, now, claimLease).
			Scan(&m.ID, &m.To, &m.Subject, &m.HTML, &m.Text, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return sent, failed, nil
		}
		if err != nil {
			return sent, failed, err
		}
		if sendErr := send(m); sendErr == nil {
			if _, err := s.pool.Exec(record, `UPDATE email_outbox SET status = 'sent', attempts = attempts + 1, last_error = NULL WHERE id = $1`, m.ID); err != nil {
				return sent, failed, err
			}
			sent++
		} else {
			failed++
			if _, err := s.pool.Exec(record, `
				UPDATE email_outbox SET attempts = attempts + 1, last_error = $2,
				       status = CASE WHEN attempts + 1 >= $3 THEN 'failed' ELSE 'pending' END,
				       next_attempt_at = $4::timestamptz + least(power(2, attempts + 1) * interval '1 minute', interval '6 hours')
				WHERE id = $1`, m.ID, sendErr.Error(), maxAttempts, now); err != nil {
				return sent, failed, err
			}
			if attempts+1 >= maxAttempts && gaveUp != nil {
				gaveUp(m, sendErr)
			}
		}
	}
	return sent, failed, nil
}

// OutboxCount counts e-mails by status.
func (s *Store) OutboxCount(ctx context.Context, status string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE status = $1`, status).Scan(&n)
	return n, err
}

// DueCandidate is a card whose due date calls for a reminder.
type DueCandidate struct {
	Card      Card
	BoardName string
	Kind      string // NotifyDueSoon or NotifyOverdue
}

// DueCandidates finds assigned cards, not archived and not done, due within the
// next 24 hours (due_soon) or past their due day (overdue). A due date is the
// start of that day.
func (s *Store) DueCandidates(ctx context.Context, now time.Time) ([]DueCandidate, error) {
	today := now.Format(time.DateOnly)
	tomorrow := now.Add(24 * time.Hour).Format(time.DateOnly)
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+`, b.name,
		       CASE WHEN k.due_date < $1::date THEN 'overdue' ELSE 'due_soon' END
		FROM cards k
		JOIN columns c ON c.id = k.column_id
		JOIN boards b ON b.id = k.board_id
		WHERE k.archived_at IS NULL AND b.archived_at IS NULL AND NOT c.is_done
		  AND k.assignee_id IS NOT NULL AND k.due_date IS NOT NULL AND k.due_date <= $2::date
		ORDER BY k.id`, today, tomorrow)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueCandidate
	for rows.Next() {
		var d DueCandidate
		c := &d.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt,
			&d.BoardName, &d.Kind); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// NewlyUnblocked returns the cards blockerID blocks that no longer wait on
// anything: every blocker is done or archived.
func (s *Store) NewlyUnblocked(ctx context.Context, blockerID int64) ([]Card, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+` FROM card_dependencies d
		JOIN cards k ON k.id = d.blocked_id
		WHERE d.blocker_id = $1 AND k.archived_at IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM card_dependencies o JOIN cards b ON b.id = o.blocker_id JOIN columns bc ON bc.id = b.column_id
		      WHERE o.blocked_id = k.id AND b.archived_at IS NULL AND NOT bc.is_done)
		ORDER BY k.id`, blockerID)
	if err != nil {
		return nil, err
	}
	var out []Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	rows.Close()
	return out, rows.Err()
}

// CanSeeBoard reports whether userID may see boardID: an admin, or a member of
// the board's team.
func (s *Store) CanSeeBoard(ctx context.Context, userID, boardID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND is_admin)
		    OR EXISTS (SELECT 1 FROM boards b JOIN team_members m ON m.team_id = b.team_id
		               WHERE b.id = $2 AND m.user_id = $1)`, userID, boardID).Scan(&ok)
	return ok, err
}
