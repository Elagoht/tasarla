package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type doneView struct {
	Board   store.Board
	Query   string
	CanEdit bool
	// Open are the columns a card can be reopened into: those not done.
	Open  []store.Column
	Cards []doneCardView
}

type doneCardView struct {
	Card        store.Card
	CompletedBy string
	When        string
	ISO         string
	// Back is the column offered for reopening: the one it came from.
	Back int64
}

// boardDonePage lists a board's completed cards, which are not on the board.
func (h *handlers) boardDonePage() *collage.Page {
	content := collage.NewFragment("board-done-content", "pages/board_done.html").
		WithDataHandler(collage.Load(h.loadBoardDone)).
		Required().
		Build()
	return paths(h.privatePage("board-done", content), "/boards/{id}/done").
		WithAction(http.MethodPost, h.boardDonePost).
		Dynamic().
		Build()
}

// openColumns are the columns of cols that are not done.
func openColumns(cols []store.Column) []store.Column {
	var out []store.Column
	for _, c := range cols {
		if !c.IsDone {
			out = append(out, c)
		}
	}
	return out
}

// reopenColumn is where a completed card is offered to go back: the column it
// came from while that is still an open column, else the first open one.
func reopenColumn(c store.Card, open []store.Column) int64 {
	for _, o := range open {
		if c.CompletedFrom != nil && o.ID == *c.CompletedFrom {
			return o.ID
		}
	}
	if len(open) > 0 {
		return open[0].ID
	}
	return 0
}

func (h *handlers) loadBoardDone(ctx context.Context, rc *collage.RenderContext) (doneView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return doneView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "done.title") + " · " + bc.Board.Name)
	v := doneView{Board: bc.Board, CanEdit: bc.Access.CanEdit, Query: strings.TrimSpace(rc.Request.URL.Query().Get("q"))}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return v, err
	}
	v.Open = openColumns(cols)
	cards, err := h.store.DoneCards(ctx, bc.Board.ID, v.Query)
	for _, c := range cards {
		cv := doneCardView{Card: c.Card, CompletedBy: c.CompletedBy, Back: reopenColumn(c.Card, v.Open)}
		if c.Card.CompletedAt != nil {
			cv.When = c.Card.CompletedAt.Local().Format("2006-01-02 15:04")
			cv.ISO = c.Card.CompletedAt.UTC().Format(time.RFC3339)
		}
		v.Cards = append(v.Cards, cv)
	}
	return v, err
}

// boardDonePost reopens a completed card into a column, as a move: through
// that column's rules.
func (h *handlers) boardDonePost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
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
	if badText(rc) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	cardID, ok1 := formInt64(v, "card")
	to, ok2 := formInt64(v, "to_column")
	version, ok3 := formInt64(v, "expected_version")
	if v.Value("op") != "reopen" || !ok1 || !ok2 || !ok3 {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	card, err := h.store.Card(ctx, bc.Board.ID, cardID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && (card.CompletedAt == nil || card.ArchivedAt != nil)) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return nil, err
	}
	if !containsColumn(openColumns(cols), to) {
		return collage.NoContent(http.StatusBadRequest), nil // reopening is into a column not done
	}
	_, err = h.store.MoveCard(ctx, store.Move{BoardID: bc.Board.ID, CardID: cardID, ToColumnID: to, ToIndex: 1 << 30,
		ExpectedFrom: card.ColumnID, ExpectedVersion: int(version), Actor: bc.actor()})
	switch msgs := violationMessages(rc, err); {
	case msgs != nil:
		for _, m := range msgs {
			flash.Add(rc, flash.Error, m)
		}
	case errors.Is(err, store.ErrConflict):
		flash.Add(rc, flash.Error, i18n.T(rc, "board.conflict"))
	case err != nil:
		return nil, err
	default:
		flash.Add(rc, flash.Success, i18n.T(rc, "done.reopened"))
	}
	res, err := h.redirectTo(rc, "board-done", "id", strconv.FormatInt(bc.Board.ID, 10))
	if res != nil {
		res.InvalidateTags = []string{boardTag(bc.Board.ID), cardTag(cardID)}
	}
	return res, err
}

func containsColumn(cols []store.Column, id int64) bool {
	for _, c := range cols {
		if c.ID == id {
			return true
		}
	}
	return false
}
