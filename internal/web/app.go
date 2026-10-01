// Package web is the application's pages and actions, drawn with collage.
package web

import (
	"fmt"
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
	"kanban/internal/store"
)

// Deps is what the application is built from.
type Deps struct {
	Files   fs.FS
	DevMode bool
	Host    string
	Port    int
	Config  config.Config
	Store   *store.Store
	OIDC    *auth.Client
	Logger  *slog.Logger
}

type handlers struct {
	store *store.Store
	log   *slog.Logger

	// Fragments that actions answer with; set when their pages are built.
	columns *collage.Fragment
	panel   *collage.Fragment
}

// New builds the application: plugins, layouts, every page and action.
func New(d Deps) (*collage.App, error) {
	app, err := collage.New(&collage.Config{
		DevMode: d.DevMode,
		Logger:  d.Logger,
		Server:  collage.ServerConfig{Host: d.Host, Port: d.Port},
		Template: collage.TemplateConfig{
			FS: d.Files, Root: "templates", Extension: ".html",
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
			i18n.New(i18n.Options{FS: d.Files, Dir: "locales", Strict: true}),
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
	h := &handlers{store: d.Store, log: d.Logger}
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

	static, err := fs.Sub(d.Files, "static")
	if err != nil {
		return nil, err
	}
	if err := app.Mount("/static/", static); err != nil {
		return nil, fmt.Errorf("mount static files: %w", err)
	}
	return app, nil
}

// pages is every page for signed-in readers.
func (h *handlers) pages() []*collage.Page {
	return []*collage.Page{
		h.homePage(), h.teamsPage(), h.teamPage(), h.adminUsersPage(),
		h.boardPage(), h.cardPage(), h.tasksPage(),
	}
}

// contentSecurityPolicy allows scripts only from this origin, and form posts
// to this origin and the provider: signing out redirects a form's post there.
func contentSecurityPolicy(issuer string) string {
	provider := ""
	if u, err := url.Parse(issuer); err == nil {
		provider = " " + u.Scheme + "://" + u.Host
	}
	return "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self'; img-src 'self' data:; " +
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
