package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"kanban/internal/store"
)

func TestAdminCreatesATeam(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")

	res := admin.Submit("/teams", "/teams", url.Values{"op": {"create"}, "name": {"  Platform  "}})
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/teams/") {
		t.Fatalf("create = %d %q:\n%s", res.Status, res.Location(), res.Body)
	}
	page := admin.Get(res.Location())
	mustContain(t, page.Body, "<h1>Platform</h1>", "Platform oluşturuldu.")
	mustContain(t, admin.Get("/teams").Body, "Platform")
}

func TestCreatingATeamValidatesTheName(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	res := admin.Submit("/teams", "/teams", url.Values{"op": {"create"}, "name": {"   "}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("blank name = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Bu alan zorunludur.")

	h.speaks("admin@example.com", "en")
	res = admin.Submit("/en/teams", "/en/teams", url.Values{"op": {"create"}, "name": {""}})
	mustContain(t, res.Body, "This field is required.")
}

func TestOnlyAdminsCreateTeams(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")
	page := bob.Get("/teams")
	if strings.Contains(page.Body, `name="op" value="create"`) {
		t.Error("a non-admin is shown the create form")
	}
	res := bob.Submit("/teams", "/teams", url.Values{"op": {"create"}, "name": {"Sneaky"}})
	if res.Status != http.StatusForbidden {
		t.Fatalf("non-admin create = %d, want 403", res.Status)
	}
}

// teamWith creates a team as admin with lead and member, and returns its path.
func teamWith(t *testing.T, h *harness, lead, member string) string {
	t.Helper()
	ctx := context.Background()
	team, err := h.store.CreateTeam(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	if lead != "" {
		if err := h.store.AddMember(ctx, team.ID, h.user(lead).ID, store.RoleLead); err != nil {
			t.Fatal(err)
		}
	}
	if member != "" {
		if err := h.store.AddMember(ctx, team.ID, h.user(member).ID, store.RoleMember); err != nil {
			t.Fatal(err)
		}
	}
	return "/teams/" + strconv.FormatInt(team.ID, 10)
}

func TestLeadManagesMembers(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	member := h.signedIn("member", "member@example.com")
	h.signedIn("carol", "carol@example.com")
	path := teamWith(t, h, "lead@example.com", "member@example.com")

	res := lead.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"Carol@Example.com"}, "new_role": {"member"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("add = %d:\n%s", res.Status, res.Body)
	}
	mustContain(t, lead.Get(path).Body, "carol@example.com", "Carol takıma eklendi.")

	carolID := strconv.FormatInt(h.user("carol@example.com").ID, 10)
	res = lead.Submit(path, path, url.Values{"op": {"set_role"}, "user_id": {carolID}, "role": {"lead"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("set_role = %d", res.Status)
	}
	if role, _ := h.store.MemberRole(context.Background(), idOf(path), h.user("carol@example.com").ID); role != store.RoleLead {
		t.Errorf("carol's role = %q", role)
	}

	res = lead.Submit(path, path, url.Values{"confirm": {"1"}, "op": {"remove_member"}, "user_id": {carolID}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("remove = %d", res.Status)
	}
	if strings.Contains(lead.Get(path).Body, "carol@example.com") {
		t.Error("carol is still listed")
	}

	// A member sees the team but may not change it.
	page := member.Get(path)
	if page.Status != http.StatusOK || strings.Contains(page.Body, `value="add_member"`) {
		t.Fatalf("member view = %d, manage form shown: %v", page.Status, strings.Contains(page.Body, `value="add_member"`))
	}
	res = member.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"carol@example.com"}, "new_role": {"lead"}})
	if res.Status != http.StatusForbidden {
		t.Fatalf("member add = %d, want 403", res.Status)
	}
}

func TestAddingAnUnknownEmailIsRefusedOnTheForm(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	path := teamWith(t, h, "", "")
	res := admin.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"ghost@example.com"}, "new_role": {"member"}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Bu e-postayla kayıtlı bir kullanıcı yok.", `value="ghost@example.com"`)

	res = admin.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"x@example.com"}, "new_role": {"owner"}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown role = %d, want 422", res.Status)
	}
}

func TestOutsidersCannotTellATeamExists(t *testing.T) {
	h := newHarness(t, "")
	h.signedIn("lead", "lead@example.com")
	outsider := h.signedIn("out", "out@example.com")
	path := teamWith(t, h, "lead@example.com", "")

	for _, p := range []string{path, "/teams/999999", "/teams/abc"} {
		if res := outsider.Get(p); res.Status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, res.Status)
		}
	}
	// The outsider needs a token from a page they can see.
	res := outsider.Submit("/teams", path, url.Values{"op": {"add_member"}, "new_email": {"out@example.com"}, "new_role": {"lead"}})
	if res.Status != http.StatusNotFound {
		t.Fatalf("outsider POST = %d, want 404", res.Status)
	}
	if _, err := h.store.MemberRole(context.Background(), idOf(path), h.user("out@example.com").ID); err == nil {
		t.Fatal("the outsider added themselves")
	}
}

func idOf(path string) int64 {
	id, _ := strconv.ParseInt(path[strings.LastIndex(path, "/")+1:], 10, 64)
	return id
}

func TestEnglishPagesLinkInEnglish(t *testing.T) {
	h := newHarness(t, "")
	b := h.signedIn("ada", "ada@example.com")
	mustContain(t, b.Get("/").Body, `href="/teams"`)
	h.speaks("ada@example.com", "en")
	mustContain(t, b.Get("/en").Body, `href="/en/teams"`)
}

func TestAddingAMemberWithoutARoleIsRefused(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	h.signedIn("carol", "carol@example.com")
	path := teamWith(t, h, "", "")
	res := admin.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"carol@example.com"}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("add without a role = %d, want 422", res.Status)
	}
}
