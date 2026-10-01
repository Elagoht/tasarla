package web_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/auth/authtest"
	"kanban/internal/config"
	"kanban/internal/db/dbtest"
	"kanban/internal/files"
	"kanban/internal/store"
	"kanban/internal/web"
	"kanban/internal/webtest"
)

type harness struct {
	t      *testing.T
	app    *collage.App
	issuer *authtest.Issuer
	store  *store.Store
}

// newHarness builds the real application over a fresh database and a fake
// provider. adminEmails is ADMIN_EMAILS, raw.
func newHarness(t *testing.T, adminEmails string) *harness {
	t.Helper()
	return build(t, store.New(dbtest.New(t)), adminEmails)
}

// newHarnessWithoutDB builds the application for tests that only register it.
func newHarnessWithoutDB(t *testing.T) *harness {
	t.Helper()
	return build(t, store.New(nil), "")
}

func build(t *testing.T, s *store.Store, adminEmails string) *harness {
	t.Helper()
	return buildAt(t, s, adminEmails, webtest.Origin)
}

// buildAt builds the application with BASE_URL set to base.
func buildAt(t *testing.T, s *store.Store, adminEmails, base string) *harness {
	t.Helper()
	callback := base + "/auth/openid/authentik"
	issuer := authtest.NewIssuer(t, authtest.Options{RedirectURL: callback})
	key := strings.Repeat("ab", 32)
	cfg, err := config.Load(func(k string) string {
		return map[string]string{
			"BASE_URL": base, "DATABASE_URL": "unused", "DEFAULT_LOCALE": "tr",
			"OIDC_ISSUER": issuer.URL, "OIDC_CLIENT_ID": issuer.ClientID, "OIDC_CLIENT_SECRET": issuer.ClientSecret,
			"OIDC_REDIRECT_URL": callback,
			"ADMIN_EMAILS":      adminEmails, "SESSION_KEY": key, "CSRF_KEY": key, "FLASH_KEY": key,
			"ATTACHMENTS_DIR": t.TempDir(),
			"SMTP_HOST":       "smtp.invalid", "SMTP_FROM": "Kanban <noreply@kanban.test>",
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	client, err := auth.NewClient(context.Background(), auth.ClientConfig{
		Issuer: issuer.URL, ClientID: issuer.ClientID, ClientSecret: issuer.ClientSecret,
		RedirectURL: cfg.OIDC.RedirectURL, Scopes: cfg.OIDC.Scopes,
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	dir, err := files.Open(cfg.AttachmentsDir)
	if err != nil {
		t.Fatal(err)
	}
	app, err := web.New(web.Deps{
		Files: root.FS(), Attachments: dir, Config: cfg, Store: s, OIDC: client,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return &harness{t: t, app: app.App, issuer: issuer, store: s}
}

func (h *harness) browser() *webtest.Browser { return webtest.NewBrowser(h.t, h.app.Handler()) }

// signedIn returns a browser signed in as a new user with this subject and email.
func (h *harness) signedIn(subject, email string) *webtest.Browser {
	h.t.Helper()
	b := h.browser()
	webtest.SignIn(h.t, b, h.issuer, authtest.Grant{Subject: subject, Email: email, Name: strings.ToUpper(subject[:1]) + subject[1:]})
	return b
}

func (h *harness) user(email string) store.User {
	h.t.Helper()
	u, err := h.store.UserByEmail(context.Background(), email)
	if err != nil {
		h.t.Fatalf("user %s: %v", email, err)
	}
	return u
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("page does not contain %q:\n%s", w, body)
		}
	}
}
