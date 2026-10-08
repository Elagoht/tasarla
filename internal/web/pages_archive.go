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

type archiveView struct {
	Board   store.Board
	Query   string
	CanEdit bool
	Cards   []archivedCardView
}

type archivedCardView struct {
	Card       store.Card
	ColumnName string
	ArchivedBy string
	When       string
	ISO        string
}

// boardArchivePage lists a board's archived cards, to find and restore them.
func (h *handlers) boardArchivePage() *collage.Page {
	content := collage.NewFragment("board-archive-content", "pages/board_archive.html").
		WithData(collage.Load(h.loadBoardArchive)).
		Required().
		Build()
	return paths(h.privatePage("board-archive", content), "/boards/{id}/archive").
		WithAction(http.MethodPost, h.boardArchivePost).
		Dynamic().
		Build()
}

func (h *handlers) loadBoardArchive(ctx context.Context, rc *collage.RenderContext) (archiveView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return archiveView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "archive.title") + " · " + bc.Board.Name)
	v := archiveView{Board: bc.Board, CanEdit: bc.Access.CanEdit,
		Query: strings.TrimSpace(rc.Request.URL.Query().Get("q"))}
	cards, err := h.store.ArchivedCards(ctx, bc.Board.ID, v.Query)
	for _, c := range cards {
		cv := archivedCardView{Card: c.Card, ColumnName: c.ColumnName, ArchivedBy: c.ArchivedBy}
		if c.Card.ArchivedAt != nil {
			cv.When = c.Card.ArchivedAt.Local().Format("2006-01-02 15:04")
			cv.ISO = c.Card.ArchivedAt.UTC().Format(time.RFC3339)
		}
		v.Cards = append(v.Cards, cv)
	}
	return v, err
}

func (h *handlers) boardArchivePost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
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
	card, ok := formInt64(v, "card")
	if v.Value("op") != "restore" || !ok {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if found, err := h.restoreCard(ctx, rc, bc, card); err != nil || !found {
		if err != nil {
			return nil, err
		}
		return collage.NoContent(http.StatusNotFound), nil
	}
	res, err := h.redirectTo(rc, "board-archive", "id", strconv.FormatInt(bc.Board.ID, 10))
	if res != nil {
		res.InvalidateTags = []string{boardTag(bc.Board.ID), cardTag(card)}
	}
	return res, err
}

// restoreCard takes a card out of the archive and says so, or why not, in a
// flash message; found is false for a card not in this board's archive.
func (h *handlers) restoreCard(ctx context.Context, rc *collage.RenderContext, bc boardContext, cardID int64) (found bool, err error) {
	err = h.store.RestoreCard(ctx, bc.Board.ID, cardID, bc.User.ID)
	if msgs := violationMessages(rc, err); msgs != nil {
		for _, m := range msgs {
			flash.Add(rc, flash.Error, m)
		}
		return true, nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	flash.Add(rc, flash.Success, i18n.T(rc, "archive.restored"))
	return true, nil
}
