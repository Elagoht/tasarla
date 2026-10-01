package store

import (
	"context"
	"strings"

	"kanban/internal/rules"
)

// ActivityCardRestored is a card taken out of the archive.
const ActivityCardRestored = "card_restored"

// ArchivedCard is a card in a board's archive.
type ArchivedCard struct {
	Card       Card
	ColumnName string
	// ArchivedBy is who archived it, "" when the history does not say.
	ArchivedBy string
}

// likeEscaper makes a search's %, _ and \ match themselves in ILIKE.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// ArchivedCards returns a board's archived cards, last archived first; with
// query, only those whose title holds it.
func (s *Store) ArchivedCards(ctx context.Context, boardID int64, query string) ([]ArchivedCard, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+`, col.name,
		       coalesce((SELECT u.name FROM activity a JOIN users u ON u.id = a.actor_id
		                 WHERE a.card_id = k.id AND a.kind = $3 ORDER BY a.id DESC LIMIT 1), '')
		FROM cards k JOIN columns col ON col.id = k.column_id
		WHERE k.board_id = $1 AND k.archived_at IS NOT NULL
		  AND ($2 = '' OR k.title ILIKE '%' || $2 || '%' ESCAPE '\')
		ORDER BY k.archived_at DESC, k.id DESC
		LIMIT 500`, boardID, likeEscaper.Replace(query), ActivityCardArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchivedCard
	for rows.Next() {
		var a ArchivedCard
		c := &a.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt, &c.CompletedAt, &c.CompletedFrom, &c.StartDate,
			&a.ColumnName, &a.ArchivedBy); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// RestoreCard takes a card out of the archive, to the bottom of the column it
// left. Coming back is entering that column: its entry conditions and its WIP
// limit apply, as for a new card, and a refusal is a *RuleError.
func (s *Store) RestoreCard(ctx context.Context, boardID, cardID, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	card, err := scanCard(tx.QueryRow(ctx,
		`SELECT `+cardColumns+` FROM cards WHERE id = $2 AND board_id = $1 AND archived_at IS NOT NULL FOR UPDATE`, boardID, cardID))
	if err != nil {
		return err
	}
	snap, err := loadSnapshot(ctx, tx, boardID, card)
	if err != nil {
		return err
	}
	if err := ruleError(rules.EvaluateCreate(card.ColumnID, snap)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE cards SET archived_at = NULL, version = version + 1,
		       position = (SELECT count(*) FROM cards WHERE column_id = $3 AND archived_at IS NULL AND completed_at IS NULL)
		WHERE id = $2 AND board_id = $1`, boardID, cardID, card.ColumnID); err != nil {
		return err
	}
	if err := logActivity(ctx, tx, boardID, &cardID, actorID, ActivityCardRestored, ActivityPayload{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ArchivedBoards returns a team's archived boards, last archived first.
func (s *Store) ArchivedBoards(ctx context.Context, teamID int64) ([]Board, error) {
	return collectBoards(s.pool.Query(ctx, `
		SELECT `+boardColumns+` FROM boards
		WHERE team_id = $1 AND archived_at IS NOT NULL
		ORDER BY archived_at DESC, id DESC`, teamID))
}

// RestoreBoard takes a board of teamID out of the archive.
func (s *Store) RestoreBoard(ctx context.Context, teamID, id int64) error {
	return exactlyOne(s.pool.Exec(ctx,
		`UPDATE boards SET archived_at = NULL WHERE id = $1 AND team_id = $2 AND archived_at IS NOT NULL`, id, teamID))
}
