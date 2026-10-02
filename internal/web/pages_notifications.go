package web

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/config"
	"kanban/internal/notify"
	"kanban/internal/store"
)

func notificationsTag(userID int64) string { return "notifications:" + strconv.FormatInt(userID, 10) }

type badgeView struct {
	Count int
}

// loadBadge counts the reader's unread notifications. It is pushed again
// whenever one is created for them or marked read.
func (h *handlers) loadBadge(ctx context.Context, _ *collage.RenderContext) (badgeView, []string, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return badgeView{}, nil, err
	}
	n, err := h.store.UnreadCount(ctx, user.ID)
	return badgeView{Count: n}, []string{notificationsTag(user.ID)}, err
}

type notificationsView struct {
	Items []notificationView
}

type notificationView struct {
	ID      int64
	Text    string
	BoardID int64
	CardID  int64
	Unread  bool
	When    string
	ISO     string
}

// translateIn adapts i18n.T to notify.Translate for one render.
func translateIn(rc *collage.RenderContext) notify.Translate {
	return func(key, actor, card, board, due string) string {
		return i18n.T(rc, key, "actor", actor, "card", card, "board", board, "due", due)
	}
}

func (h *handlers) notificationsPage() *collage.Page {
	content := collage.NewFragment("notifications-content", "pages/notifications.html").
		WithDataHandler(collage.Load(h.loadNotifications)).
		Required().
		Build()
	b := paths(h.privatePage("notifications", content), "/notifications")
	for _, l := range config.Locales {
		b = b.WithFragmentPath(l, "/notifications/badge", h.badge)
	}
	return b.WithAction(http.MethodPost, h.notificationsPost).Dynamic().Build()
}

func (h *handlers) loadNotifications(ctx context.Context, rc *collage.RenderContext) (notificationsView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return notificationsView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "notify.title"))
	list, err := h.store.Notifications(ctx, user.ID, 50)
	if err != nil {
		return notificationsView{}, err
	}
	var view notificationsView
	someone := i18n.T(rc, "notify.someone")
	for _, n := range list {
		item := notificationView{
			ID: n.ID, Text: notify.Sentence(translateIn(rc), someone, n.Kind, n.Payload),
			BoardID: n.Payload.BoardID, Unread: n.ReadAt == nil,
			When: n.CreatedAt.Local().Format("2006-01-02 15:04"), ISO: n.CreatedAt.UTC().Format(time.RFC3339),
		}
		if n.CardID != nil {
			item.CardID = *n.CardID
		}
		view.Items = append(view.Items, item)
	}
	return view, nil
}

func (h *handlers) notificationsPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	if badText(rc) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	switch v.Value("op") {
	case "read":
		id, ok := formInt64(v, "notification_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.MarkRead(ctx, user.ID, id)
	case "read_all":
		err = h.store.MarkAllRead(ctx, user.ID)
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	res, err := h.redirectTo(rc, "notifications")
	if res != nil {
		res.InvalidateTags = []string{notificationsTag(user.ID)}
	}
	return res, err
}

type meSettingsView struct {
	Locale      string
	Locales     []string
	Prefs       []prefView
	HasCalendar bool
	// Calendar holds the addresses just made; it is set only in the answer
	// to creating or resetting, since the token is not kept in the clear.
	Calendar *calendarLinks
	// CalendarBoards are the endings (boards/{id}.ics) of the user's boards,
	// shown while a token exists so a later-joined board can be subscribed.
	CalendarBoards []calendarBoardLink
}

const calendarKey = "calendar"

type calendarLinks struct {
	Me     string
	Boards []calendarBoardLink
}

type calendarBoardLink struct {
	Name, URL string
}

// calendarLinks builds the feed addresses for a fresh token: one for the
// cards assigned to the user and one per board of their teams.
func (h *handlers) calendarLinks(ctx context.Context, userID int64, token string) (*calendarLinks, error) {
	base := h.baseURL + "/cal/" + token
	links := &calendarLinks{Me: base + "/me.ics"}
	boards, err := h.userBoards(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, b := range boards {
		links.Boards = append(links.Boards, calendarBoardLink{Name: b.Name, URL: base + "/" + boardFeedEnding(b.ID)})
	}
	return links, nil
}

func boardFeedEnding(boardID int64) string {
	return "boards/" + strconv.FormatInt(boardID, 10) + ".ics"
}

// userBoards are the boards, not archived, of every team the user is in.
func (h *handlers) userBoards(ctx context.Context, userID int64) ([]store.Board, error) {
	teams, err := h.store.TeamsOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []store.Board
	for _, t := range teams {
		boards, err := h.store.BoardsOfTeam(ctx, t.Team.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, boards...)
	}
	return out, nil
}

type prefView struct {
	Kind  string
	Email bool
}

func (h *handlers) meSettingsPage() *collage.Page {
	content := collage.NewFragment("me-settings-content", "pages/me_settings.html").
		WithDataHandler(collage.Load(h.loadMeSettings)).
		Required().
		Build()
	return paths(h.privatePage("me-settings", content), "/me/settings").
		WithAction(http.MethodPost, h.meSettingsPost).
		Dynamic().
		Build()
}

func (h *handlers) loadMeSettings(ctx context.Context, rc *collage.RenderContext) (meSettingsView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return meSettingsView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "me.title"))
	prefs, err := h.store.NotificationPrefs(ctx, user.ID)
	if err != nil {
		return meSettingsView{}, err
	}
	view := meSettingsView{Locale: user.Locale, Locales: config.Locales}
	for _, k := range store.NotifyKinds {
		view.Prefs = append(view.Prefs, prefView{Kind: k, Email: prefs[k]})
	}
	if view.HasCalendar, err = h.store.HasCalendarToken(ctx, user.ID); err != nil {
		return meSettingsView{}, err
	}
	if view.HasCalendar {
		boards, err := h.userBoards(ctx, user.ID)
		if err != nil {
			return meSettingsView{}, err
		}
		for _, b := range boards {
			view.CalendarBoards = append(view.CalendarBoards, calendarBoardLink{Name: b.Name, URL: boardFeedEnding(b.ID)})
		}
	}
	if l, ok := collage.Get[*calendarLinks](rc, calendarKey); ok {
		view.Calendar = l
	}
	return view, nil
}

func (h *handlers) meSettingsPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	if badText(rc) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	switch v.Value("op") {
	case "calendar_create", "calendar_reset":
		token, err := h.store.NewCalendarToken(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		links, err := h.calendarLinks(ctx, user.ID, token)
		if err != nil {
			return nil, err
		}
		rc.Set(calendarKey, links)
		return collage.RenderPage(rc.Page), nil
	case "calendar_clear":
		if err := h.store.ClearCalendarToken(ctx, user.ID); err != nil {
			return nil, err
		}
		flash.Add(rc, flash.Success, i18n.T(rc, "calendar.cleared"))
		return h.redirectTo(rc, "me-settings")
	case "save":
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if !slices.Contains(config.Locales, v.Value("locale")) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	prefs := map[string]bool{}
	for _, k := range store.NotifyKinds {
		prefs[k] = v.Value("email_"+k) == "1"
	}
	if err := h.store.SetLocale(ctx, user.ID, v.Value("locale")); err != nil {
		return nil, err
	}
	if err := h.store.SetNotificationPrefs(ctx, user.ID, prefs); err != nil {
		return nil, err
	}
	// The interface follows the account's language: back to these settings in it.
	locale := v.Value("locale")
	flash.Add(rc, flash.Success, h.i18n.In(locale).T("me.saved"))
	target, err := h.urlIn("me-settings", locale, nil)
	if err != nil {
		return nil, err
	}
	return collage.SeeOther(target), nil
}
