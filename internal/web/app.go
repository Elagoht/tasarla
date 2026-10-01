// Package web is the application's pages and actions, drawn with collage.
package web

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/url"
	"strings"
	"time"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	live "github.com/Elagoht/collage-live"
	secure "github.com/Elagoht/collage-secure"
	session "github.com/Elagoht/collage-session"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/config"
	"kanban/internal/files"
	"kanban/internal/notify"
	"kanban/internal/store"
)

// Deps is what the application is built from.
type Deps struct {
	Files       fs.FS
	Attachments *files.Dir
	DevMode     bool
	Host        string
	Port        int
	Config      config.Config
	Store       *store.Store
	OIDC        *auth.Client
	Logger      *slog.Logger
}

type handlers struct {
	store *store.Store
	files *files.Dir
	log   *slog.Logger
	// defaultLocale is the locale whose paths have no prefix.
	defaultLocale string
	// urlIn is a page's path in a given locale; i18n translates in one.
	urlIn func(name, locale string, params map[string]string) (string, error)
	i18n  *i18n.Plugin
	// loc is the time zone "today" and schedules are reckoned in.
	loc *time.Location

	// Fragments that actions answer with; set when their pages are built.
	columns *collage.Fragment
	panel   *collage.Fragment
	gantt   *collage.Fragment
	badge   *collage.Fragment
	confirm *collage.Page

	notifier *notify.Notifier
}

// App is the collage application and the notifier its actions and the
// scheduler share.
type App struct {
	*collage.App
	Notifier *notify.Notifier
}

// New builds the application: plugins, layouts, every page and action.
func New(d Deps) (*App, error) {
	translations := i18n.New(i18n.Options{FS: d.Files, Dir: "locales", Strict: true})
	app, err := collage.New(&collage.Config{
		DevMode: d.DevMode,
		Logger:  d.Logger,
		Server:  collage.ServerConfig{Host: d.Host, Port: d.Port},
		Template: collage.TemplateConfig{
			FS: d.Files, Root: "templates", Extension: ".html",
			Funcs: template.FuncMap{
				"richText": richText,
				"inc":      func(n int) int { return n + 1 },
				"dec":      func(n int) int { return n - 1 },
				// The colour a board is shown with, the same in the sidebar and on
				// every list; an avatar's hue and letter for a user.
				"boardColor":      func(id int64) string { return boardColor(int(id)) },
				"hue":             func(id int64) int64 { return id % 8 },
				"initial":         initial,
				"withQuery":       withQuery,
				"ganttLink":       ganttLink,
				"extraQuery":      func(v url.Values) string { return v.Encode() },
				"priorityChoices": func() []int16 { return []int16{4, 3, 2, 1} },
				"dueChoices":      func() []store.DueFilter { return dueFilters },
			},
		},
		Locale: collage.LocaleConfig{Default: d.Config.DefaultLocale, Supported: config.Locales},
		Cache:  collage.CacheConfig{Enabled: true, Type: "memory", DefaultTTL: 5 * time.Minute},
		Security: collage.SecurityConfig{
			CSRFKey: d.Config.CSRFKey,
		},
		// Every plugin here wraps the middleware added with app.Use below
		// (collage v0.38.0), so auth.Middleware can read the session.
		Plugins: []collage.Plugin{
			session.New(session.Options{
				Key: d.Config.SessionKey, Encrypt: true, MaxAge: 7 * 24 * 60 * 60,
				// A proxy that terminates TLS may not say so; the site's own origin does.
				Secure: strings.HasPrefix(d.Config.BaseURL, "https://"),
			}),
			translations,
			validate.New(validate.Options{LocaleMessages: validationMessages}),
			flash.New(flash.Options{Key: d.Config.FlashKey}),
			live.New(),
			secure.New(secure.Options{CSP: contentSecurityPolicy(d.Config.OIDC.Issuer)}),
		},
	})
	if err != nil {
		return nil, err
	}

	notFound, serverError, authFailed := notFoundPage(), errorPage(), authFailedPage()
	h := &handlers{store: d.Store, files: d.Attachments, log: d.Logger, defaultLocale: d.Config.DefaultLocale,
		urlIn: app.URL, i18n: translations, loc: d.Config.Location}
	h.notifier = &notify.Notifier{
		Store: d.Store, I18n: translations, BaseURL: d.Config.BaseURL,
		URL: app.URL, Invalidate: app.InvalidateTags, Log: d.Logger,
	}
	h.badge = collage.NewFragment("notifications-badge", "fragments/badge.html").
		WithDataHandler(collage.DataHandler(h.loadBadge)).
		Build()
	pages := append([]*collage.Page{notFound, serverError, authFailed}, h.pages()...)
	for _, p := range pages {
		if err := app.RegisterPage(p); err != nil {
			return nil, fmt.Errorf("register page %q: %w", p.Name, err)
		}
	}
	if err := app.RegisterNotFoundPage(notFound); err != nil {
		return nil, err
	}
	if err := app.RegisterErrorPage(serverError); err != nil {
		return nil, err
	}

	signIn := auth.NewService(auth.Options{
		Client: d.OIDC, Users: d.Store, AdminEmails: d.Config.AdminEmails,
		DefaultLocale: d.Config.DefaultLocale, Locales: config.Locales,
		BaseURL: d.Config.BaseURL, CallbackPath: d.Config.OIDC.CallbackPath,
		FailedPage: authFailed, Logger: d.Logger,
	})
	for _, a := range signIn.Actions() {
		if err := app.RegisterAction(a); err != nil {
			return nil, fmt.Errorf("register action %q: %w", a.Name, err)
		}
	}
	if err := app.Use(signIn.Middleware); err != nil {
		return nil, err
	}

	if err := app.Handle("/files/", h.filesHandler()); err != nil {
		return nil, err
	}

	static, err := fs.Sub(d.Files, "static")
	if err != nil {
		return nil, err
	}
	if err := app.Mount("/static/", static); err != nil {
		return nil, fmt.Errorf("mount static files: %w", err)
	}
	return &App{App: app, Notifier: h.notifier}, nil
}

// pages is every page for signed-in readers.
func (h *handlers) pages() []*collage.Page {
	h.confirm = h.confirmPage()
	return []*collage.Page{
		h.confirm, h.homePage(), h.teamsPage(), h.teamPage(), h.adminUsersPage(),
		h.boardPage(), h.cardPage(), h.tasksPage(), h.boardSettingsPage(), h.boardActivityPage(), h.boardArchivePage(), h.boardDonePage(), h.boardGanttPage(),
		h.notificationsPage(), h.meSettingsPage(), h.searchPage(),
	}
}

// contentSecurityPolicy allows scripts only from this origin, and form posts
// to this origin and the provider: signing out redirects a form's post there.
// ViewTransitionStyle opts pages into view transitions between them. It is the
// one inline style, in base.html's head, allowed by its hash: the browser
// reads the opt-in before the stylesheets arrive, and an opt-in in one of them
// is often not there yet (seen: most transitions skipped, "opt-in disabled").
const ViewTransitionStyle = "@view-transition{navigation:auto}"

func styleHash(css string) string {
	sum := sha256.Sum256([]byte(css))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func contentSecurityPolicy(issuer string) string {
	provider := ""
	if u, err := url.Parse(issuer); err == nil {
		provider = " " + u.Scheme + "://" + u.Host
	}
	return "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' " + styleHash(ViewTransitionStyle) + "; img-src 'self' data:; " +
		"object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'" + provider
}

var validationMessages = map[string]map[string]string{
	"tr": {
		"required": "Bu alan zorunludur.",
		"maxLen":   "En fazla {max} karakter olabilir.",
		"email":    "Geçerli bir e-posta adresi girin.",
		"oneOf":    "Geçerli bir seçenek seçin.",
	},
}
