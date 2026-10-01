package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

// Spec §2.1: a guarded page must render per request, never from a shared cache.
func TestEveryGuardedPageIsDynamic(t *testing.T) {
	h := newHarnessWithoutDB(t)
	guarded := 0
	for _, p := range h.app.Pages() {
		if !p.Guarded() {
			continue
		}
		guarded++
		if p.Strategy != collage.StrategyDynamic {
			t.Errorf("guarded page %q has strategy %v, want Dynamic", p.Name, p.Strategy)
		}
	}
	if guarded == 0 {
		t.Fatal("no guarded pages were registered; the check above checked nothing")
	}
}

func TestAnonymousReadersAreSentToLogin(t *testing.T) {
	h := newHarnessWithoutDB(t)
	b := h.browser()
	for path, want := range map[string]string{"/": "/login?next=%2F", "/en": "/login?next=%2Fen"} {
		res := b.Get(path)
		if res.Status != http.StatusSeeOther || res.Location() != want {
			t.Errorf("GET %s = %d %q, want 303 %q", path, res.Status, res.Location(), want)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := newHarnessWithoutDB(t)
	res := h.browser().Get("/no-such-page")
	if res.Status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Status)
	}
	csp := res.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self' 'nonce-", "form-action 'self' " + h.issuer.URL, "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q lacks %q", csp, want)
		}
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff missing")
	}
	mustContain(t, res.Body, "Sayfa bulunamadı", `lang="tr"`)
}

func TestNotFoundInEnglish(t *testing.T) {
	h := newHarnessWithoutDB(t)
	res := h.browser().Get("/en/no-such-page")
	mustContain(t, res.Body, "Page not found", `lang="en"`)
}

func TestHomeInBothLanguages(t *testing.T) {
	h := newHarness(t, "")
	b := h.signedIn("ada", "ada@example.com")

	tr := b.Get("/")
	if tr.Status != http.StatusOK {
		t.Fatalf("GET / = %d:\n%s", tr.Status, tr.Body)
	}
	mustContain(t, tr.Body, "Board&#39;larım", "Henüz bir takımda değilsiniz.", "Çıkış yap", "Ada")

	// The interface is in the account's language: an English address is
	// sent to the Turkish one, and the other way round once it is English.
	if res := b.Get("/en/teams?x=1"); res.Status != http.StatusSeeOther || res.Location() != "/teams?x=1" {
		t.Fatalf("GET /en/teams for a Turkish account = %d %q", res.Status, res.Location())
	}
	h.speaks("ada@example.com", "en")
	if res := b.Get("/"); res.Status != http.StatusSeeOther || res.Location() != "/en" {
		t.Fatalf("GET / for an English account = %d %q", res.Status, res.Location())
	}
	en := b.Get("/en")
	mustContain(t, en.Body, "My boards", "Sign out")
}

func TestAuthFailedPageIsTranslated(t *testing.T) {
	h := newHarnessWithoutDB(t)
	res := h.browser().Get("/auth/openid/authentik?state=x&code=y")
	if res.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Status)
	}
	mustContain(t, res.Body, "Giriş yapılamadı")
}

// Behind a proxy that terminates TLS without saying so, the session cookie must
// still be Secure when the site's own origin is https.
func TestCookiesAreSecureWhenTheSiteIsHTTPS(t *testing.T) {
	h := buildAt(t, store.New(nil), "", "https://kanban.test")
	res := h.browser().Get("/login")
	cookie := res.Header.Get("Set-Cookie")
	if !strings.Contains(cookie, "collage_session=") || !strings.Contains(cookie, "Secure") {
		t.Fatalf("Set-Cookie = %q, want a Secure session cookie", cookie)
	}
}
