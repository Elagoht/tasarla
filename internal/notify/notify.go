// Package notify turns what happens on boards into notifications: in the app,
// and by e-mail through an outbox (spec §10). E-mails are written in the
// recipient's language, not the actor's.
package notify

import (
	"bytes"
	"context"
	_ "embed"
	htmltemplate "html/template"
	"log/slog"
	"strconv"
	texttemplate "text/template"
	"time"

	i18n "github.com/Elagoht/collage-i18n"

	"kanban/internal/store"
)

//go:embed mail.html
var mailHTML string

//go:embed mail.txt
var mailText string

// Event is something one user should hear about.
type Event struct {
	Kind      string // store.Notify*
	To        int64  // the recipient
	Actor     int64  // who caused it; 0 for the scheduler
	ActorName string
	Card      store.Card
	BoardName string
	Text      string // a comment's excerpt
	DedupeKey string // "" for none
}

// Notifier writes notifications and queues their e-mails.
type Notifier struct {
	Store      *store.Store
	I18n       *i18n.Plugin
	BaseURL    string // "https://pano.example.com"
	URL        func(name, locale string, params map[string]string) (string, error)
	Invalidate func(ctx context.Context, tags ...string) error
	Log        *slog.Logger
}

type mailView struct {
	Locale      string
	Name        string
	Sentence    string
	Board       string
	Excerpt     string
	CardURL     string
	SettingsURL string
}

// Emit notifies each event's recipient. Nobody is told of their own action,
// and a disabled user is told nothing. A failure for one event is logged and
// does not stop the others; the first is returned.
func (n *Notifier) Emit(ctx context.Context, events ...Event) error {
	var first error
	for _, e := range events {
		if err := n.emit(ctx, e); err != nil {
			n.Log.Error("notify: emit", "kind", e.Kind, "to", e.To, "err", err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func (n *Notifier) emit(ctx context.Context, e Event) error {
	if e.To == 0 || e.To == e.Actor {
		return nil
	}
	user, err := n.Store.UserByID(ctx, e.To)
	if err != nil || user.Disabled() {
		return err
	}
	prefs, err := n.Store.NotificationPrefs(ctx, user.ID)
	if err != nil {
		return err
	}
	tr := n.I18n.In(user.Locale)
	payload := store.NotificationPayload{
		CardTitle: e.Card.Title, BoardID: e.Card.BoardID, BoardName: e.BoardName,
		ActorName: e.ActorName, Text: e.Text,
	}
	if e.Card.DueDate != nil {
		payload.DueDate = e.Card.DueDate.Format(time.DateOnly)
	}
	notification := store.NewNotification{UserID: user.ID, Kind: e.Kind, Payload: payload, DedupeKey: e.DedupeKey}
	if e.Card.ID != 0 {
		id := e.Card.ID
		notification.CardID = &id
	}
	if prefs[e.Kind] {
		msg, err := n.render(tr, user, e, payload)
		if err != nil {
			return err
		}
		notification.Email = &msg
	}
	created, err := n.Store.CreateNotification(ctx, notification)
	if err != nil || !created {
		return err
	}
	return n.Invalidate(ctx, "notifications:"+strconv.FormatInt(user.ID, 10))
}

// Translate looks up a notification sentence with its parameters filled in.
type Translate func(key, actor, card, board, due string) string

// Sentence is a notification in one language; the page and the e-mail share it.
func Sentence(tr Translate, someone, kind string, p store.NotificationPayload) string {
	actor := p.ActorName
	if actor == "" {
		actor = someone
	}
	return tr("notify."+kind, actor, p.CardTitle, p.BoardName, p.DueDate)
}

func (n *Notifier) render(tr i18n.Translator, user store.User, e Event, p store.NotificationPayload) (store.OutboxMessage, error) {
	t := func(key, actor, card, board, due string) string {
		return tr.T(key, "actor", actor, "card", card, "board", board, "due", due)
	}
	view := mailView{
		Locale: tr.Locale(), Name: user.Name, Sentence: Sentence(t, tr.T("notify.someone"), e.Kind, p),
		Board: e.BoardName, Excerpt: e.Text,
	}
	if e.Card.ID != 0 {
		path, err := n.URL("card", tr.Locale(), map[string]string{
			"id": strconv.FormatInt(e.Card.BoardID, 10), "card": strconv.FormatInt(e.Card.ID, 10),
		})
		if err != nil {
			return store.OutboxMessage{}, err
		}
		view.CardURL = n.BaseURL + path
	}
	settings, err := n.URL("me-settings", tr.Locale(), nil)
	if err != nil {
		return store.OutboxMessage{}, err
	}
	view.SettingsURL = n.BaseURL + settings

	var text, html bytes.Buffer
	textTmpl, err := texttemplate.New("mail.txt").Funcs(texttemplate.FuncMap(tr.Funcs())).Parse(mailText)
	if err != nil {
		return store.OutboxMessage{}, err
	}
	if err := textTmpl.Execute(&text, view); err != nil {
		return store.OutboxMessage{}, err
	}
	htmlTmpl, err := htmltemplate.New("mail.html").Funcs(tr.Funcs()).Parse(mailHTML)
	if err != nil {
		return store.OutboxMessage{}, err
	}
	if err := htmlTmpl.Execute(&html, view); err != nil {
		return store.OutboxMessage{}, err
	}
	return store.OutboxMessage{To: user.Email, Subject: "[Kanban] " + view.Sentence, Text: text.String(), HTML: html.String()}, nil
}
