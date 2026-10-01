package store

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"kanban/internal/rules"
)

// SentenceKind is what a rule sentence says (settings, "Kurallar").
type SentenceKind string

const (
	SentencePermission SentenceKind = "permission" // into X only {who} (from Y)
	SentenceFrom       SentenceKind = "from"       // into X only from {columns}
	SentenceCondition  SentenceKind = "condition"  // entering/leaving X needs {condition}
	SentenceWIP        SentenceKind = "wip"        // at most N cards in X
	SentencePersonWIP  SentenceKind = "person_wip" // at most N cards per person in {columns}
)

// Sentence is one rule as the settings list shows it; the rows behind it stay
// what internal/rules reads.
type Sentence struct {
	Kind          SentenceKind
	Key           string // identifies the sentence for DeleteSentence
	ColumnID      int64
	FromID        int64    // permission: source column, 0 for any
	Subjects      []string // permission: rules.Subject* other than board_role
	RoleIDs       []int64  // permission: board roles
	Columns       []int64  // from: source columns; person_wip: counted columns
	Phase         string   // condition
	ConditionKind string   // condition
	ConditionID   int64    // condition
	Params        rules.ConditionParams
	Limit         int // wip, person_wip
}

// BoardSentences lists a board's rules as sentences, column by column.
func (s *Store) BoardSentences(ctx context.Context, boardID int64) ([]Sentence, error) {
	board, err := s.Board(ctx, boardID)
	if err != nil {
		return nil, err
	}
	cols, err := s.Columns(ctx, boardID)
	if err != nil {
		return nil, err
	}
	r, err := s.BoardRules(ctx, boardID)
	if err != nil {
		return nil, err
	}
	var out []Sentence
	for _, col := range cols {
		// Permissions, grouped by source.
		groups := map[int64]*Sentence{}
		var order []int64
		for _, p := range r.Permissions {
			if p.ToColumnID != col.ID {
				continue
			}
			from := int64(0)
			if p.FromColumnID != nil {
				from = *p.FromColumnID
			}
			g, ok := groups[from]
			if !ok {
				g = &Sentence{Kind: SentencePermission, ColumnID: col.ID, FromID: from,
					Key: "permission:" + strconv.FormatInt(col.ID, 10) + ":" + strconv.FormatInt(from, 10)}
				groups[from] = g
				order = append(order, from)
			}
			if p.BoardRoleID != nil {
				g.RoleIDs = append(g.RoleIDs, *p.BoardRoleID)
			} else if !slices.Contains(g.Subjects, p.Subject) {
				g.Subjects = append(g.Subjects, p.Subject)
			}
		}
		for _, from := range order {
			out = append(out, *groups[from])
		}
		// Where cards may come from, when that is not "anywhere".
		if board.TransitionsMode == rules.ModeRestricted {
			var sources []int64
			for _, t := range r.Transitions {
				if t.To == col.ID {
					sources = append(sources, t.From)
				}
			}
			if len(sources) != len(cols)-1 {
				out = append(out, Sentence{Kind: SentenceFrom, ColumnID: col.ID, Columns: sources,
					Key: "from:" + strconv.FormatInt(col.ID, 10)})
			}
		}
		for _, c := range r.Conditions {
			if c.ColumnID == col.ID {
				out = append(out, Sentence{Kind: SentenceCondition, ColumnID: col.ID, Phase: c.Phase,
					ConditionKind: c.Kind, ConditionID: c.ID, Params: c.Params,
					Key: "condition:" + strconv.FormatInt(c.ID, 10)})
			}
		}
		if col.WIPLimit != nil {
			out = append(out, Sentence{Kind: SentenceWIP, ColumnID: col.ID, Limit: *col.WIPLimit,
				Key: "wip:" + strconv.FormatInt(col.ID, 10)})
		}
	}
	if board.PersonWIPLimit != nil {
		ps := Sentence{Kind: SentencePersonWIP, Limit: *board.PersonWIPLimit, Key: "person_wip"}
		for _, c := range cols {
			if c.CountsPersonWIP {
				ps.Columns = append(ps.Columns, c.ID)
			}
		}
		out = append(out, ps)
	}
	return out, nil
}

// DeleteSentence removes the rule a sentence stands for. Deleting a "from"
// sentence frees its column; when no column is restricted any more, the board
// goes back to open transitions.
func (s *Store) DeleteSentence(ctx context.Context, boardID int64, key string) error {
	parts := strings.Split(key, ":")
	ids := make([]int64, 0, 2)
	for _, p := range parts[1:] {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return ErrNotFound
		}
		ids = append(ids, n)
	}
	switch {
	case parts[0] == "permission" && len(ids) == 2:
		return exactlyOne(s.pool.Exec(ctx, `
			DELETE FROM move_permissions
			WHERE board_id = $1 AND to_column_id = $2 AND coalesce(from_column_id, 0) = $3`, boardID, ids[0], ids[1]))
	case parts[0] == "from" && len(ids) == 1:
		return s.freeColumn(ctx, boardID, ids[0])
	case parts[0] == "condition" && len(ids) == 1:
		return s.DeleteCondition(ctx, boardID, ids[0])
	case parts[0] == "wip" && len(ids) == 1:
		return exactlyOne(s.pool.Exec(ctx, `UPDATE columns SET wip_limit = NULL WHERE id = $2 AND board_id = $1`, boardID, ids[0]))
	case key == "person_wip":
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := exactlyOne(tx.Exec(ctx, `UPDATE boards SET person_wip_limit = NULL WHERE id = $1`, boardID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE columns SET counts_person_wip = false WHERE board_id = $1`, boardID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return ErrNotFound
}

// freeColumn lets cards reach column from every other column again.
func (s *Store) freeColumn(ctx context.Context, boardID, column int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	if err := columnsOfBoard(ctx, tx, boardID, []int64{column}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO transitions (board_id, from_column_id, to_column_id)
		SELECT $1, c.id, $2 FROM columns c WHERE c.board_id = $1 AND c.id <> $2
		ON CONFLICT DO NOTHING`, boardID, column); err != nil {
		return err
	}
	// Every column reachable from every other is the same as open.
	var missing int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM columns f JOIN columns t ON t.board_id = f.board_id AND t.id <> f.id
		WHERE f.board_id = $1 AND NOT EXISTS (
			SELECT 1 FROM transitions x WHERE x.from_column_id = f.id AND x.to_column_id = t.id)`, boardID).Scan(&missing); err != nil {
		return err
	}
	if missing == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM transitions WHERE board_id = $1`, boardID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE boards SET transitions_mode = 'open' WHERE id = $1`, boardID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AddFromSentence says cards may enter to only from the given columns. The
// first such sentence switches the board to restricted transitions, with
// every other column still reachable from everywhere, so that one sentence
// restricts one column.
func (s *Store) AddFromSentence(ctx context.Context, boardID, to int64, from []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return err
	}
	if err := columnsOfBoard(ctx, tx, boardID, append([]int64{to}, from...)); err != nil {
		return err
	}
	var mode string
	if err := tx.QueryRow(ctx, `SELECT transitions_mode FROM boards WHERE id = $1`, boardID).Scan(&mode); err != nil {
		return err
	}
	if mode == rules.ModeOpen {
		if _, err := tx.Exec(ctx, `DELETE FROM transitions WHERE board_id = $1`, boardID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transitions (board_id, from_column_id, to_column_id)
			SELECT $1, f.id, t.id FROM columns f JOIN columns t ON t.board_id = f.board_id AND t.id <> f.id
			WHERE f.board_id = $1`, boardID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE boards SET transitions_mode = 'restricted' WHERE id = $1`, boardID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM transitions WHERE board_id = $1 AND to_column_id = $2`, boardID, to); err != nil {
		return err
	}
	for _, f := range from {
		if f == to {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transitions (board_id, from_column_id, to_column_id) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, boardID, f, to); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SetWIPLimit sets a column's WIP limit; nil removes it.
func (s *Store) SetWIPLimit(ctx context.Context, boardID, columnID int64, limit *int) error {
	return exactlyOne(s.pool.Exec(ctx, `UPDATE columns SET wip_limit = $3 WHERE id = $2 AND board_id = $1`, boardID, columnID, limit))
}

// SetPersonWIP sets the per-person limit and which columns count toward it.
func (s *Store) SetPersonWIP(ctx context.Context, boardID int64, limit *int, counted []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if len(counted) > 0 {
		if err := columnsOfBoard(ctx, tx, boardID, counted); err != nil {
			return err
		}
	}
	if err := exactlyOne(tx.Exec(ctx, `UPDATE boards SET person_wip_limit = $2 WHERE id = $1`, boardID, limit)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE columns SET counts_person_wip = (id = ANY($2::bigint[])) WHERE board_id = $1`, boardID, counted); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
