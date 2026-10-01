package web_test

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"kanban/internal/web"
)

// The view transition opt-in sits inline in every page's head, and the CSP
// allows exactly it, by its hash.
func TestPagesOptIntoViewTransitionsUnderTheCSP(t *testing.T) {
	h := newHarness(t, "")
	b := h.signedIn("ada", "ada@example.com")
	res := b.Get("/")
	if res.Status != http.StatusOK {
		t.Fatalf("GET / = %d", res.Status)
	}
	mustContain(t, res.Body, "<style>"+web.ViewTransitionStyle+"</style>")
	sum := sha256.Sum256([]byte(web.ViewTransitionStyle))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	csp := res.Header.Get("Content-Security-Policy") + res.Header.Get("Content-Security-Policy-Report-Only")
	if !strings.Contains(csp, "style-src 'self' "+want) {
		t.Fatalf("the CSP does not allow the opt-in: %s", csp)
	}
}
