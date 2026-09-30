package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestADMIN_EMAILSMakesTheFirstAdmin(t *testing.T) {
	h := newHarness(t, " Admin@Example.com , ")
	admin := h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")

	if !h.user("admin@example.com").IsAdmin || h.user("bob@example.com").IsAdmin {
		t.Fatal("ADMIN_EMAILS granted the wrong users")
	}
	mustContain(t, admin.Get("/").Body, `href="/admin/users"`)
	if strings.Contains(bob.Get("/").Body, `href="/admin/users"`) {
		t.Error("a non-admin sees the users link")
	}
	if res := bob.Get("/admin/users"); res.Status != http.StatusNotFound {
		t.Errorf("non-admin GET /admin/users = %d, want 404", res.Status)
	}
	res := bob.Submit("/", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {"1"}, "value": {"1"}})
	if res.Status != http.StatusNotFound {
		t.Errorf("non-admin POST /admin/users = %d, want 404", res.Status)
	}
}

func TestTheOnlyAdminCannotStepDown(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	id := strconv.FormatInt(h.user("admin@example.com").ID, 10)

	res := admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {id}, "value": {"0"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("status = %d", res.Status)
	}
	mustContain(t, admin.Get("/admin/users").Body, "En az bir aktif admin kalmalı.")
	if !h.user("admin@example.com").IsAdmin {
		t.Fatal("the only admin was demoted")
	}

	res = admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_disabled"}, "user_id": {id}, "value": {"1"}})
	mustContain(t, admin.Get("/admin/users").Body, "Kendi hesabınızı devre dışı bırakamazsınız.")
}

func TestDisablingAUserSignsThemOut(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")
	if res := bob.Get("/"); res.Status != http.StatusOK {
		t.Fatalf("bob GET / = %d", res.Status)
	}
	id := strconv.FormatInt(h.user("bob@example.com").ID, 10)

	res := admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_disabled"}, "user_id": {id}, "value": {"1"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("disable = %d", res.Status)
	}
	res = bob.Get("/")
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/login") {
		t.Fatalf("disabled bob GET / = %d %q, want a redirect to /login", res.Status, res.Location())
	}

	admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_disabled"}, "user_id": {id}, "value": {"0"}})
	again := h.signedIn("bob", "bob@example.com")
	if res := again.Get("/"); res.Status != http.StatusOK {
		t.Fatalf("re-enabled bob GET / = %d", res.Status)
	}
}

func TestAdminPromotesAnother(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")
	id := strconv.FormatInt(h.user("bob@example.com").ID, 10)

	admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {id}, "value": {"1"}})
	if res := bob.Get("/admin/users"); res.Status != http.StatusOK {
		t.Fatalf("promoted bob GET /admin/users = %d", res.Status)
	}
	if res := admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {"999999"}, "value": {"1"}}); res.Status != http.StatusNotFound {
		t.Errorf("unknown user = %d, want 404", res.Status)
	}
}
