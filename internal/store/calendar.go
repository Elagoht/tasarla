package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// NewCalendarToken gives userID a new calendar token, replacing any old one,
// and returns it; only its hash is kept.
func (s *Store) NewCalendarToken(ctx context.Context, userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := exactlyOne(s.pool.Exec(ctx, `UPDATE users SET calendar_token_hash = $2 WHERE id = $1`, userID, tokenHash(token))); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) ClearCalendarToken(ctx context.Context, userID int64) error {
	return exactlyOne(s.pool.Exec(ctx, `UPDATE users SET calendar_token_hash = NULL WHERE id = $1`, userID))
}

func (s *Store) HasCalendarToken(ctx context.Context, userID int64) (bool, error) {
	var has bool
	err := s.pool.QueryRow(ctx, `SELECT calendar_token_hash IS NOT NULL FROM users WHERE id = $1`, userID).Scan(&has)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return has, err
}

// UserByCalendarToken is the active user holding token; ErrNotFound otherwise.
func (s *Store) UserByCalendarToken(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNotFound
	}
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users
		WHERE calendar_token_hash = $1 AND disabled_at IS NULL`, tokenHash(token)))
}

// CalendarCard is a card a calendar shows.
type CalendarCard struct {
	Card      Card
	BoardName string
	Modified  time.Time // the card's last activity, or its creation
}

// CalendarCards are the dated, open cards of boardID (0: every board of the
// user's teams, assigned to the user). A board the user is not a member of,
// or an archived one, is ErrNotFound.
func (s *Store) CalendarCards(ctx context.Context, userID, boardID int64) ([]CalendarCard, error) {
	if boardID != 0 {
		var ok bool
		if err := s.pool.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM boards b JOIN team_members m ON m.team_id = b.team_id
			               WHERE b.id = $1 AND m.user_id = $2 AND b.archived_at IS NULL)`, boardID, userID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNotFound
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+`, b.name,
		       coalesce((SELECT max(a.created_at) FROM activity a WHERE a.card_id = k.id), k.created_at)
		FROM cards k
		JOIN boards b ON b.id = k.board_id AND b.archived_at IS NULL
		JOIN team_members m ON m.team_id = b.team_id AND m.user_id = $1
		WHERE k.due_date IS NOT NULL AND k.completed_at IS NULL AND k.archived_at IS NULL
		  AND (($2 = 0 AND k.assignee_id = $1) OR k.board_id = $2)
		ORDER BY k.due_date, k.id`, userID, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarCard
	for rows.Next() {
		var cc CalendarCard
		c := &cc.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt, &c.CompletedAt, &c.CompletedFrom, &c.StartDate,
			&cc.BoardName, &cc.Modified); err != nil {
			return nil, err
		}
		out = append(out, cc)
	}
	return out, rows.Err()
}
