package web

import (
	"context"
	"strings"
	"time"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type activityView struct {
	Text      string
	CardTitle string
	CardID    int64
	BoardID   int64
	When      string
	ISO       string
}

// activityViews turns entries into sentences in the reader's language.
func activityViews(rc *collage.RenderContext, entries []store.Activity) []activityView {
	views := make([]activityView, 0, len(entries))
	for _, a := range entries {
		actor := a.ActorName
		if actor == "" {
			actor = i18n.T(rc, "activity.someone")
		}
		fields := make([]string, len(a.Payload.Fields))
		for i, f := range a.Payload.Fields {
			fields[i] = i18n.T(rc, "activity.fields."+f)
		}
		p := a.Payload
		v := activityView{
			Text: i18n.T(rc, "activity."+a.Kind, "actor", actor, "from", p.From, "to", p.To,
				"fields", strings.Join(fields, ", "), "text", p.Text, "filename", p.Filename),
			CardTitle: a.CardTitle,
			BoardID:   a.BoardID,
			When:      a.CreatedAt.Local().Format("2006-01-02 15:04"),
			ISO:       a.CreatedAt.UTC().Format(time.RFC3339),
		}
		if a.CardID != nil {
			v.CardID = *a.CardID
		}
		views = append(views, v)
	}
	return views
}

type boardActivityView struct {
	Board    store.Board
	Activity []activityView
}

func (h *handlers) boardActivityPage() *collage.Page {
	content := collage.NewFragment("board-activity-content", "pages/board_activity.html").
		WithDataHandler(collage.Load(h.loadBoardActivity)).
		Required().
		Build()
	return paths(privatePage("board-activity", content), "/boards/{id}/activity").Dynamic().Build()
}

func (h *handlers) loadBoardActivity(ctx context.Context, rc *collage.RenderContext) (boardActivityView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return boardActivityView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "activity.board_title") + " · " + bc.Board.Name)
	entries, err := h.store.BoardActivity(ctx, bc.Board.ID, 100)
	return boardActivityView{Board: bc.Board, Activity: activityViews(rc, entries)}, err
}
