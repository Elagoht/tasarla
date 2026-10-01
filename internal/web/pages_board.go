package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/authz"
	"kanban/internal/config"
	"kanban/internal/store"
)

// boardContext is a board and what the signed-in user may do on it, loaded
// once per render.
type boardContext struct {
	User   store.User
	Board  store.Board
	Team   store.Team
	Role   store.Role
	Access authz.BoardAccess
}

// boardFor loads the board in the URL. A board the user may not see, or an
// archived one, is reported as not found (spec §6).
func (h *handlers) boardFor(ctx context.Context, rc *collage.RenderContext) (boardContext, error) {
	return collage.Once(rc, "board:"+rc.Param("id"), func(ctx context.Context) (boardContext, error) {
		user, err := currentUser(ctx)
		if err != nil {
			return boardContext{}, err
		}
		id, err := strconv.ParseInt(rc.Param("id"), 10, 64)
		if err != nil {
			return boardContext{}, fmt.Errorf("board %q: %w", rc.Param("id"), collage.ErrNotFound)
		}
		board, err := h.store.Board(ctx, id)
		if errors.Is(err, store.ErrNotFound) || (err == nil && board.ArchivedAt != nil) {
			return boardContext{}, fmt.Errorf("board %d: %w", id, collage.ErrNotFound)
		}
		if err != nil {
			return boardContext{}, err
		}
		team, err := h.store.Team(ctx, board.TeamID)
		if err != nil {
			return boardContext{}, err
		}
		role, err := h.store.MemberRole(ctx, team.ID, user.ID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return boardContext{}, err
		}
		access := authz.Board(user, role)
		if !access.CanView {
			return boardContext{}, fmt.Errorf("board %d for user %d: %w", id, user.ID, collage.ErrNotFound)
		}
		return boardContext{User: user, Board: board, Team: team, Role: role, Access: access}, nil
	})
}

func boardTag(id int64) string { return "board:" + strconv.FormatInt(id, 10) }
func cardTag(id int64) string  { return "card:" + strconv.FormatInt(id, 10) }

// isFetch reports whether a script sent the request (board.js, collage-live).
func isFetch(rc *collage.RenderContext) bool { return rc.Request.Header.Get(collage.FetchHeader) != "" }

// noticeKey holds the messages ([]string) for the fragment or page an action
// answers with.
const noticeKey = "notices"

type boardView struct {
	Notices []string
	Board   store.Board
	Team    store.Team
	Access  authz.BoardAccess
}

type columnsView struct {
	Notices []string
	CanEdit bool
	Columns []columnView
}

type columnView struct {
	Column    store.Column
	Limit     int
	OverLimit bool
	Cards     []cardView
}

type cardView struct {
	Summary  store.CardSummary
	Due      string
	Overdue  bool
	Priority string
}

func (h *handlers) boardPage() *collage.Page {
	h.columns = collage.NewFragment("board-columns", "fragments/columns.html").
		WithDataHandler(collage.DataHandler(h.loadColumns)).
		Required().
		Build()
	content := collage.NewFragment("board-content", "pages/board.html").
		WithDataHandler(collage.Load(h.loadBoard)).
		WithSlotFragment("columns", h.columns).
		Required().
		Build()
	b := paths(h.privatePage("board", content), "/boards/{id}")
	for _, l := range config.Locales {
		b = b.WithFragmentPath(l, "/boards/{id}/columns", h.columns)
	}
	return b.WithAction(http.MethodPost, h.boardPost).Dynamic().Build()
}

func (h *handlers) loadBoard(ctx context.Context, rc *collage.RenderContext) (boardView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return boardView{}, err
	}
	rc.HoistTitle(bc.Board.Name)
	notices, _ := collage.Get[[]string](rc, noticeKey)
	return boardView{Notices: notices, Board: bc.Board, Team: bc.Team, Access: bc.Access}, nil
}

func (h *handlers) loadColumns(ctx context.Context, rc *collage.RenderContext) (columnsView, []string, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return columnsView{}, nil, err
	}
	tags := []string{boardTag(bc.Board.ID)}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return columnsView{}, tags, err
	}
	cards, err := h.store.BoardCards(ctx, bc.Board.ID)
	if err != nil {
		return columnsView{}, tags, err
	}
	view := columnsView{CanEdit: bc.Access.CanEdit}
	view.Notices, _ = collage.Get[[]string](rc, noticeKey)
	byColumn := map[int64]int{}
	for _, c := range cols {
		cv := columnView{Column: c}
		if c.WIPLimit != nil {
			cv.Limit = *c.WIPLimit
		}
		byColumn[c.ID] = len(view.Columns)
		view.Columns = append(view.Columns, cv)
	}
	today := time.Now().Format(time.DateOnly)
	for _, s := range cards {
		i, ok := byColumn[s.Card.ColumnID]
		if !ok {
			continue
		}
		cv := cardView{Summary: s}
		if s.Card.DueDate != nil {
			cv.Due = s.Card.DueDate.Format(time.DateOnly)
			cv.Overdue = cv.Due < today && !view.Columns[i].Column.IsDone
		}
		if s.Card.Priority != nil {
			cv.Priority = i18n.T(rc, "card.priorities."+strconv.Itoa(int(*s.Card.Priority)))
		}
		view.Columns[i].Cards = append(view.Columns[i].Cards, cv)
	}
	for i := range view.Columns {
		c := &view.Columns[i]
		c.OverLimit = c.Limit > 0 && len(c.Cards) > c.Limit
	}
	return view, tags, nil
}

func (h *handlers) boardPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	bc, err := h.boardFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if !bc.Access.CanEdit {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v := validate.Form(rc)
	switch v.Value("op") {
	case "create_card":
		return h.createCard(ctx, rc, v, bc)
	case "move":
		return h.moveCard(ctx, rc, v, bc)
	}
	return collage.NoContent(http.StatusBadRequest), nil
}

func (h *handlers) createCard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	v.Field("title").Required().MaxLen(200)
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return collage.NoContent(http.StatusConflict), nil
	}
	target := cols[0]
	for _, c := range cols {
		if c.AllowCreate {
			target = c
			break
		}
	}
	_, err = h.store.CreateCard(ctx, bc.Board.ID, target.ID, strings.TrimSpace(v.Value("title")), bc.User.ID)
	if msgs := violationMessages(rc, err); msgs != nil {
		rc.Set(noticeKey, msgs)
		res := collage.RenderPage(rc.Page)
		res.Status = http.StatusUnprocessableEntity
		return res, nil
	}
	if err != nil {
		return nil, err
	}
	res, err := h.redirectTo(rc, "board", "id", strconv.FormatInt(bc.Board.ID, 10))
	if res != nil {
		res.InvalidateTags = []string{boardTag(bc.Board.ID)}
	}
	return res, err
}

// formInt64 reads a whole number from the form; ok is false when it is missing
// or malformed.
func formInt64(v *validate.Validator, field string) (int64, bool) {
	n, err := strconv.ParseInt(v.Value(field), 10, 64)
	return n, err == nil
}

func (h *handlers) moveCard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	cardID, ok1 := formInt64(v, "card")
	to, ok2 := formInt64(v, "to_column")
	from, ok3 := formInt64(v, "expected_from")
	version, ok4 := formInt64(v, "expected_version")
	index, err := strconv.Atoi(v.Value("to_index"))
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if err != nil {
		index = 1 << 30 // the fallback form sends none: the bottom of the column
	}
	moved, err := h.store.MoveCard(ctx, store.Move{
		BoardID: bc.Board.ID, CardID: cardID, ToColumnID: to, ToIndex: index,
		ExpectedFrom: from, ExpectedVersion: int(version), Actor: bc.actor(),
	})
	status, notices := http.StatusOK, violationMessages(rc, err)
	switch {
	case notices != nil:
		status = http.StatusUnprocessableEntity
	case errors.Is(err, store.ErrConflict):
		status, notices = http.StatusConflict, []string{i18n.T(rc, "board.conflict")}
	case errors.Is(err, store.ErrNotFound):
		return collage.NoContent(http.StatusNotFound), nil
	case err != nil:
		return nil, err
	}
	tags := []string{boardTag(bc.Board.ID), cardTag(cardID)}
	if err == nil && moved.ColumnID != from && h.finishedBy(ctx, moved) {
		h.notifyUnblocked(ctx, bc, moved.ID)
	}
	if isFetch(rc) {
		rc.Set(noticeKey, notices)
		res := collage.RenderFragment(h.columns)
		res.Status = status
		if status == http.StatusOK {
			res.InvalidateTags = tags
		}
		return res, nil
	}
	if len(notices) > 0 {
		for _, n := range notices {
			flash.Add(rc, flash.Error, n)
		}
	} else {
		flash.Add(rc, flash.Success, i18n.T(rc, "board.moved"))
	}
	res, err := h.redirectTo(rc, "card", "id", strconv.FormatInt(bc.Board.ID, 10), "card", strconv.FormatInt(cardID, 10))
	if res != nil && status == http.StatusOK {
		res.InvalidateTags = tags
	}
	return res, err
}
