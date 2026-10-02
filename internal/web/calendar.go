package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"kanban/internal/ical"
	"kanban/internal/store"
)

// calendarPriority maps a card's priority (4 urgent … 1 low) to iCalendar's 1–9.
var calendarPriority = map[int16]int{4: 1, 3: 3, 2: 5, 1: 7}

// calendarHandler serves /cal/{token}/me.ics and /cal/{token}/boards/{id}.ics
// (spec 2026-10-01 filtre… §5). A calendar app has no session: the token is
// who is asking, and membership is checked on every request.
func (h *handlers) calendarHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		token, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/cal/"), "/")
		var boardID int64
		switch {
		case rest == "me.ics":
		case strings.HasPrefix(rest, "boards/") && strings.HasSuffix(rest, ".ics"):
			n, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(rest, "boards/"), ".ics"), 10, 64)
			if err != nil || n <= 0 {
				http.NotFound(w, r)
				return
			}
			boardID = n
		default:
			http.NotFound(w, r)
			return
		}
		ctx := r.Context()
		user, err := h.store.UserByCalendarToken(ctx, token)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			h.calendarError(w, err)
			return
		}
		cards, err := h.store.CalendarCards(ctx, user.ID, boardID)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			h.calendarError(w, err)
			return
		}
		t := h.i18n.In(user.Locale)
		cal := ical.Calendar{Name: t.T("calendar.feed_me")}
		if boardID != 0 {
			board, err := h.store.Board(ctx, boardID)
			if err != nil {
				h.calendarError(w, err)
				return
			}
			cal.Name = t.T("calendar.feed_board", "board", board.Name)
		}
		host := ""
		if u, err := url.Parse(h.baseURL); err == nil {
			host = u.Host
		}
		for _, c := range cards {
			link, err := h.urlIn("card", user.Locale, map[string]string{
				"id": strconv.FormatInt(c.Card.BoardID, 10), "card": strconv.FormatInt(c.Card.ID, 10)})
			if err != nil {
				h.calendarError(w, err)
				return
			}
			link = h.baseURL + link
			start := *c.Card.DueDate
			if c.Card.StartDate != nil {
				start = *c.Card.StartDate
			}
			// The store keeps start <= due, but bad data must not make a bad calendar.
			if start.After(*c.Card.DueDate) {
				start = *c.Card.DueDate
			}
			e := ical.Event{
				UID: "card-" + strconv.FormatInt(c.Card.ID, 10) + "@" + host, Start: start, End: *c.Card.DueDate,
				Summary: "[" + c.BoardName + "] " + c.Card.Title, URL: link, Modified: c.Modified,
				Description: strings.TrimSpace(link + "\n\n" + c.Card.Description),
			}
			if c.Card.Priority != nil {
				e.Priority = calendarPriority[*c.Card.Priority]
			}
			cal.Events = append(cal.Events, e)
		}
		body := ical.Encode(cal)
		sum := sha256.Sum256(body)
		etag := `"` + hex.EncodeToString(sum[:16]) + `"`
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "private, max-age=300")
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if r.Method == http.MethodHead {
			return
		}
		w.Write(body)
	})
}

func (h *handlers) calendarError(w http.ResponseWriter, err error) {
	h.log.Error("calendar: feed", "err", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
