package web_test

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"kanban/internal/store"
)

func TestAnAdminCannotChangeTheirOwnRole(t *testing.T) {
	h := newHarness(t, "admin@example.com,second@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	h.signedIn("second", "second@example.com")
	me := id(h.user("admin@example.com").ID)
	for _, form := range []url.Values{
		{"op": {"set_admin"}, "user_id": {me}, "value": {"0"}},
		{"confirm": {"1"}, "op": {"set_disabled"}, "user_id": {me}, "value": {"1"}},
	} {
		if res := admin.Submit("/admin/users", "/admin/users", form); res.Status != http.StatusForbidden {
			t.Errorf("%s on myself = %d, want 403", form.Get("op"), res.Status)
		}
	}
	if u := h.user("admin@example.com"); !u.IsAdmin || u.Disabled() {
		t.Fatalf("the admin changed their own role: %+v", u)
	}
	// Another admin may still be demoted.
	other := id(h.user("second@example.com").ID)
	if res := admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {other}, "value": {"0"}}); res.Status != http.StatusSeeOther {
		t.Fatalf("demoting another admin = %d", res.Status)
	}
	page := admin.Get("/admin/users").Body
	own := regexp.MustCompile(`(?s)name="user_id" value="` + me + `">.*?<button[^>]*>`).FindString(page)
	if !regexp.MustCompile(`disabled`).MatchString(own) {
		t.Errorf("the admin's own buttons are not disabled:\n%s", own)
	}
}

func TestALeadCannotChangeTheirOwnRole(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	path := teamWith(t, h, "lead@example.com", "")
	me := id(h.user("lead@example.com").ID)
	for _, form := range []url.Values{
		{"op": {"set_role"}, "user_id": {me}, "role": {"member"}},
		{"confirm": {"1"}, "op": {"remove_member"}, "user_id": {me}},
	} {
		if res := lead.Submit(path, path, form); res.Status != http.StatusForbidden {
			t.Errorf("%s on myself = %d, want 403", form.Get("op"), res.Status)
		}
	}
	if role, _ := h.store.MemberRole(context.Background(), idOf(path), h.user("lead@example.com").ID); role != store.RoleLead {
		t.Fatalf("role = %q", role)
	}
}
