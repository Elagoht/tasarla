package store

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"kanban/internal/rules"
)

// ErrConflict reports that a card changed since the reader loaded it.
var ErrConflict = errors.New("store: the card was changed by someone else")

// Card is one card on a board.
type Card struct {
	ID          int64
	BoardID     int64
	ColumnID    int64
	Position    int
	Title       string
	Description string
	AssigneeID  *int64
	Estimate    *float64
	DueDate     *time.Time
	Priority    *int16
	CreatedBy   int64
	Version     int
	ArchivedAt  *time.Time
	CreatedAt   time.Time
}

const cardColumns = `id, board_id, column_id, position, title, description, assignee_id, estimate::float8, due_date, priority, created_by, version, archived_at, created_at`

func scanCard(row pgx.Row) (Card, error) {
	var c Card
	err := row.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
		&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	return c, err
}

// CreateCard adds a card at the bottom of a column of boardID.
func (s *Store) CreateCard(ctx context.Context, boardID, columnID int64, title string, createdBy int64) (Card, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback(ctx)
	card, err := createCard(ctx, tx, boardID, columnID, title, createdBy)
	if err != nil {
		return Card{}, err
	}
	return card, tx.Commit(ctx)
}

func createCard(ctx context.Context, tx pgx.Tx, boardID, columnID int64, title string, createdBy int64) (Card, error) {
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return Card{}, err
	}
	snap, err := loadSnapshot(ctx, tx, boardID, Card{})
	if err != nil {
		return Card{}, err
	}
	if _, ok := snap.Columns[columnID]; !ok {
		return Card{}, ErrNotFound
	}
	if err := ruleError(rules.EvaluateCreate(columnID, snap)); err != nil {
		return Card{}, err
	}
	card, err := scanCard(tx.QueryRow(ctx, `
		INSERT INTO cards (board_id, column_id, position, title, created_by)
		SELECT $1, c.id,
		       (SELECT count(*) FROM cards WHERE column_id = c.id AND archived_at IS NULL),
		       $3, $4
		FROM columns c WHERE c.id = $2 AND c.board_id = $1
		RETURNING `+cardColumns, boardID, columnID, title, createdBy))
	if err != nil {
		return Card{}, err
	}
	return card, logActivity(ctx, tx, boardID, &card.ID, createdBy, ActivityCardCreated, ActivityPayload{Title: title})
}

// lockBoard serialises the moves and creations of one board (spec §5.3).
func lockBoard(ctx context.Context, tx pgx.Tx, boardID int64) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM boards WHERE id = $1 FOR UPDATE`, boardID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Card returns a card of boardID; a card of another board is not found.
func (s *Store) Card(ctx context.Context, boardID, cardID int64) (Card, error) {
	return scanCard(s.pool.QueryRow(ctx,
		`SELECT `+cardColumns+` FROM cards WHERE id = $2 AND board_id = $1`, boardID, cardID))
}

// CardFields is what a card's edit form changes.
type CardFields struct {
	Title       string
	Description string
	AssigneeID  *int64
	Estimate    *float64
	DueDate     *time.Time
	Priority    *int16
}

// UpdateCard changes a card's fields if it is still at expectedVersion. Giving
// the card to someone new checks their WIP (spec §5.2).
func (s *Store) UpdateCard(ctx context.Context, boardID, cardID int64, expectedVersion int, f CardFields, actorID int64) (Card, error) {
	return s.updateCard(ctx, boardID, cardID, expectedVersion, func(Card) CardFields { return f }, actorID)
}

// CardField names one field of a card that can be saved on its own.
type CardField string

const (
	FieldTitle       CardField = "title"
	FieldDescription CardField = "description"
	FieldAssignee    CardField = "assignee"
	FieldDueDate     CardField = "due_date"
	FieldPriority    CardField = "priority"
	FieldEstimate    CardField = "estimate"
)

// ErrUnknownField refuses a field UpdateCardField does not save.
var ErrUnknownField = errors.New("store: unknown card field")

// UpdateCardField saves one field of a card, taking it from f; every other
// field keeps the card's current value, so a save cannot overwrite what
// someone else changed meanwhile in another field.
func (s *Store) UpdateCardField(ctx context.Context, boardID, cardID int64, expectedVersion int, field CardField, f CardFields, actorID int64) (Card, error) {
	switch field {
	case FieldTitle, FieldDescription, FieldAssignee, FieldDueDate, FieldPriority, FieldEstimate:
	default:
		return Card{}, ErrUnknownField
	}
	return s.updateCard(ctx, boardID, cardID, expectedVersion, func(c Card) CardFields {
		cur := CardFields{Title: c.Title, Description: c.Description, AssigneeID: c.AssigneeID,
			Estimate: c.Estimate, DueDate: c.DueDate, Priority: c.Priority}
		switch field {
		case FieldTitle:
			cur.Title = f.Title
		case FieldDescription:
			cur.Description = f.Description
		case FieldAssignee:
			cur.AssigneeID = f.AssigneeID
		case FieldDueDate:
			cur.DueDate = f.DueDate
		case FieldPriority:
			cur.Priority = f.Priority
		case FieldEstimate:
			cur.Estimate = f.Estimate
		}
		return cur
	}, actorID)
}

// updateCard writes the fields fieldsFor derives from the card as it stands,
// under the board's lock.
func (s *Store) updateCard(ctx context.Context, boardID, cardID int64, expectedVersion int, fieldsFor func(Card) CardFields, actorID int64) (Card, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return Card{}, err
	}
	card, err := scanCard(tx.QueryRow(ctx,
		`SELECT `+cardColumns+` FROM cards WHERE id = $2 AND board_id = $1 FOR UPDATE`, boardID, cardID))
	if err != nil {
		return Card{}, err
	}
	if card.Version != expectedVersion {
		return Card{}, ErrConflict
	}
	f := fieldsFor(card)
	if f.AssigneeID != nil && (card.AssigneeID == nil || *card.AssigneeID != *f.AssigneeID) && card.ArchivedAt == nil {
		snap, err := loadSnapshot(ctx, tx, boardID, card)
		if err != nil {
			return Card{}, err
		}
		snap.Card.AssigneeID = *f.AssigneeID
		if err := assigneeWIP(ctx, tx, &snap, boardID, card.ID); err != nil {
			return Card{}, err
		}
		if err := ruleError(rules.EvaluateAssign(card.ColumnID, snap)); err != nil {
			return Card{}, err
		}
	}
	before := card
	card, err = scanCard(tx.QueryRow(ctx, `
		UPDATE cards SET title = $3, description = $4, assignee_id = $5, estimate = $6,
		       due_date = $7, priority = $8, version = version + 1
		WHERE id = $2 AND board_id = $1
		RETURNING `+cardColumns,
		boardID, cardID, f.Title, f.Description, f.AssigneeID, f.Estimate, f.DueDate, f.Priority))
	if err != nil {
		return Card{}, err
	}
	if changed := changedFields(before, card); len(changed) > 0 {
		changes, err := fieldChanges(ctx, tx, before, card, changed)
		if err != nil {
			return Card{}, err
		}
		if err := logActivity(ctx, tx, boardID, &card.ID, actorID, ActivityCardUpdated,
			ActivityPayload{Fields: changed, Changes: changes}); err != nil {
			return Card{}, err
		}
	}
	return card, tx.Commit(ctx)
}

// changedFields names the fields that differ between two versions of a card.
func changedFields(a, b Card) []string {
	var out []string
	add := func(name string, differ bool) {
		if differ {
			out = append(out, name)
		}
	}
	add("title", a.Title != b.Title)
	add("description", a.Description != b.Description)
	add("assignee", !samePtr(a.AssigneeID, b.AssigneeID))
	add("estimate", !samePtr(a.Estimate, b.Estimate))
	add("due_date", !sameDate(a.DueDate, b.DueDate))
	add("priority", !samePtr(a.Priority, b.Priority))
	return out
}

// fieldChanges are the values of the changed fields before and after.
func fieldChanges(ctx context.Context, tx pgx.Tx, a, b Card, changed []string) ([]FieldChange, error) {
	name := func(id *int64) (string, error) {
		if id == nil {
			return "", nil
		}
		var n string
		err := tx.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, *id).Scan(&n)
		return n, err
	}
	value := func(c Card, field string) (string, error) {
		switch field {
		case "title":
			return c.Title, nil
		case "description":
			return c.Description, nil
		case "assignee":
			return name(c.AssigneeID)
		case "estimate":
			if c.Estimate == nil {
				return "", nil
			}
			return strconv.FormatFloat(*c.Estimate, 'f', -1, 64), nil
		case "due_date":
			if c.DueDate == nil {
				return "", nil
			}
			return c.DueDate.Format(time.DateOnly), nil
		case "priority":
			if c.Priority == nil {
				return "", nil
			}
			return strconv.Itoa(int(*c.Priority)), nil
		}
		return "", nil
	}
	out := make([]FieldChange, 0, len(changed))
	for _, f := range changed {
		old, err := value(a, f)
		if err != nil {
			return nil, err
		}
		cur, err := value(b, f)
		if err != nil {
			return nil, err
		}
		out = append(out, FieldChange{Field: f, Old: old, New: cur})
	}
	return out, nil
}

func samePtr[T comparable](a, b *T) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func sameDate(a, b *time.Time) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b))
}

// ArchiveCard takes a card off the board.
func (s *Store) ArchiveCard(ctx context.Context, boardID, cardID, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	var column int64
	err = tx.QueryRow(ctx, `
		UPDATE cards SET archived_at = now(), version = version + 1
		WHERE id = $2 AND board_id = $1 AND archived_at IS NULL
		RETURNING column_id`, boardID, cardID).Scan(&column)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := renumberCards(ctx, tx, column); err != nil {
		return err
	}
	if err := logActivity(ctx, tx, boardID, &cardID, actorID, ActivityCardArchived, ActivityPayload{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CardSummary is a card as the board shows it.
type CardSummary struct {
	Card           Card
	AssigneeName   string
	Labels         []Label
	ChecklistDone  int
	ChecklistTotal int
	Attachments    int
	Blocked        bool
}

// BoardCards returns a board's cards that are not archived, by column and
// position.
func (s *Store) BoardCards(ctx context.Context, boardID int64) ([]CardSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixed("k", cardColumns)+`,
		       coalesce(u.name, ''),
		       (SELECT count(*) FILTER (WHERE done) FROM checklist_items i WHERE i.card_id = k.id),
		       (SELECT count(*) FROM checklist_items i WHERE i.card_id = k.id),
		       (SELECT count(*) FROM attachments a WHERE a.card_id = k.id),
		       EXISTS (SELECT 1 FROM card_dependencies d
		               JOIN cards b ON b.id = d.blocker_id
		               JOIN columns bc ON bc.id = b.column_id
		               WHERE d.blocked_id = k.id AND b.archived_at IS NULL AND NOT bc.is_done)
		FROM cards k
		JOIN columns col ON col.id = k.column_id
		LEFT JOIN users u ON u.id = k.assignee_id
		WHERE k.board_id = $1 AND k.archived_at IS NULL
		ORDER BY col.position, k.position, k.id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cards []CardSummary
	index := map[int64]int{}
	for rows.Next() {
		var cs CardSummary
		c := &cs.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt,
			&cs.AssigneeName, &cs.ChecklistDone, &cs.ChecklistTotal, &cs.Attachments, &cs.Blocked); err != nil {
			return nil, err
		}
		index[c.ID] = len(cards)
		cards = append(cards, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	labels, err := s.pool.Query(ctx, `
		SELECT cl.card_id, l.id, l.board_id, l.name, l.color
		FROM card_labels cl JOIN labels l ON l.id = cl.label_id
		WHERE l.board_id = $1
		ORDER BY lower(l.name), l.id`, boardID)
	if err != nil {
		return nil, err
	}
	defer labels.Close()
	for labels.Next() {
		var cardID int64
		var l Label
		if err := labels.Scan(&cardID, &l.ID, &l.BoardID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		if i, ok := index[cardID]; ok {
			cards[i].Labels = append(cards[i].Labels, l)
		}
	}
	return cards, labels.Err()
}
