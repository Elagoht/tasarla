package web

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/config"
	"kanban/internal/store"
)

type calendarPageView struct {
	Board  store.Board
	Month  string // YYYY-MM
	Prev   string
	Next   string
	This   string // today's month
	Title  string
	Done   bool
	Query  string // the grid's settings and filter, for its fragment URL
	Filter filterView
}

func (h *handlers) boardCalendarPage() *collage.Page {
	h.calendarGrid = collage.NewFragment("board-calendar-grid", "fragments/calendar.html").
		WithData(collage.DataHandler(h.loadCalendar)).
		Required().
		Build()
	content := collage.NewFragment("board-calendar-content", "pages/board_calendar.html").
		WithData(collage.Load(h.loadCalendarPage)).
		WithSlotFragment("grid", h.calendarGrid).
		Required().
		Build()
	b := paths(h.privatePage("board-calendar", content), "/boards/{id}/calendar")
	for _, l := range config.Locales {
		b = b.WithFragmentPath(l, "/boards/{id}/calendar/grid", h.calendarGrid)
	}
	return b.WithAction(http.MethodPost, h.calendarPost).Dynamic().Build()
}

// calendarSettings reads the month shown and whether done cards show.
func (h *handlers) calendarSettings(rc *collage.RenderContext) (time.Time, bool) {
	q := rc.Request.URL.Query()
	return calendarMonth(q.Get("month"), time.Now().In(h.loc)), q.Get("done") == "1"
}

// calendarTitle is "Ekim 2026" in the reader's language.
func calendarTitle(rc *collage.RenderContext, month time.Time) string {
	return i18n.T(rc, "calendar_view.months."+strconv.Itoa(int(month.Month()))) + " " + strconv.Itoa(month.Year())
}

func (h *handlers) loadCalendarPage(ctx context.Context, rc *collage.RenderContext) (calendarPageView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return calendarPageView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "calendar_view.title") + " · " + bc.Board.Name)
	month, done := h.calendarSettings(rc)
	v := calendarPageView{Board: bc.Board, Done: done, Month: month.Format("2006-01"),
		Prev: month.AddDate(0, -1, 0).Format("2006-01"), Next: month.AddDate(0, 1, 0).Format("2006-01"),
		This: time.Now().In(h.loc).Format("2006-01"), Title: calendarTitle(rc, month)}
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return calendarPageView{}, err
	}
	// The bar keeps the month as a hidden field, so a new filter stays on it.
	fv.Extra = url.Values{"month": {v.Month}}
	if done {
		fv.Extra.Set("done", "1")
	}
	fv.Action, err = h.urlIn("board-calendar", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
	if err != nil {
		return calendarPageView{}, err
	}
	q := fv.Filter.Values()
	for k, vals := range fv.Extra {
		q[k] = vals
	}
	v.Query = q.Encode()
	v.Filter = fv
	return v, nil
}

func (h *handlers) loadCalendar(ctx context.Context, rc *collage.RenderContext) (calendarView, []string, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return calendarView{}, nil, err
	}
	tags := []string{boardTag(bc.Board.ID)}
	month, done := h.calendarSettings(rc)
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return calendarView{}, tags, err
	}
	colors := map[int64]string{}
	for i, c := range cols {
		colors[c.ID] = boardColor(i)
	}
	cards, err := h.store.GanttCards(ctx, bc.Board.ID, done)
	if err != nil {
		return calendarView{}, tags, err
	}
	matching, err := h.boardMatching(ctx, rc, bc)
	if err != nil {
		return calendarView{}, tags, err
	}
	v := layoutCalendar(month, cards, colors, matching, time.Now().In(h.loc))
	v.BoardID, v.CanEdit, v.Title = bc.Board.ID, bc.Access.CanEdit && len(cols) > 0, calendarTitle(rc, month)
	for i := range 7 {
		v.Weekdays = append(v.Weekdays, i18n.T(rc, "weekdays."+strconv.Itoa(i)))
	}
	if v.Action, err = h.urlIn("board-calendar", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)}); err != nil {
		return calendarView{}, tags, err
	}
	// The form posts back to the month and filter it was drawn with.
	v.Action = withQuery(v.Action, rc.Request.URL.RawQuery)
	v.Notices, _ = noticeKey.Get(rc)
	return v, tags, nil
}

// monthLink is the calendar at self for month, with the filter kept. It is
// the whole URL: a query written after "?" in a template gets its "=" and "&"
// escaped.
func monthLink(self string, f boardFilter, month string, done bool) template.URL {
	v := f.Values()
	v.Set("month", month)
	if done {
		v.Set("done", "1")
	}
	return template.URL(self + "?" + v.Encode())
}

func (h *handlers) calendarPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
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
	if badText(rc) || v.Value("op") != "create_card" {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	return h.createCalendarCard(ctx, rc, v, bc)
}

// createCalendarCard makes a card due on the day it was asked for, in the
// column the board's own "add card" uses.
func (h *handlers) createCalendarCard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	// "day", not "due": the page's query rides on the form's action, and the
	// Due filter's "due" would come first.
	due, err := time.Parse(time.DateOnly, v.Value("day"))
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	v.Field("title").Required().MaxLen(200)
	if !v.Valid() {
		res := validate.Refuse(rc, v, rc.Page)
		if isFetch(rc) {
			res.Page, res.Fragment = nil, h.calendarGrid
		}
		return res, nil
	}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return collage.NoContent(http.StatusConflict), nil
	}
	card, err := h.store.CreateCardDue(ctx, bc.Board.ID, creatableColumns(cols)[0].ID, strings.TrimSpace(v.Value("title")), due, bc.User.ID)
	if msgs := violationMessages(rc, err); msgs != nil {
		noticeKey.Set(rc, msgs)
		res := collage.RenderPage(rc.Page)
		if isFetch(rc) {
			res = collage.RenderFragment(h.calendarGrid)
		}
		res.Status = http.StatusUnprocessableEntity
		return res, nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusBadRequest), nil // the column was deleted meanwhile
	}
	if err != nil {
		return nil, err
	}
	h.notifyAssigned(ctx, bc, store.Card{}, card)
	if isFetch(rc) {
		res := collage.RenderFragment(h.calendarGrid)
		res.InvalidateTags = []string{boardTag(bc.Board.ID)}
		return res, nil
	}
	target, err := h.urlIn("board-calendar", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
	if err != nil {
		return nil, err
	}
	res := collage.SeeOther(withQuery(target, rc.Request.URL.RawQuery))
	res.InvalidateTags = []string{boardTag(bc.Board.ID)}
	return res, nil
}
