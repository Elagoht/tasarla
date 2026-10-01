package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"kanban/internal/store"
	"kanban/internal/webtest"
)

// boardSetup is a team with a lead and a member, signed in, and one board.
type boardSetup struct {
	h      *harness
	lead   *webtest.Browser
	member *webtest.Browser
	team   store.Team
	board  store.Board
	cols   []store.Column
	path   string
}

func newBoardSetup(t *testing.T) boardSetup {
	t.Helper()
	h := newHarness(t, "")
	b := boardSetup{h: h}
	b.lead = h.signedIn("lead", "lead@example.com")
	b.member = h.signedIn("member", "member@example.com")
	ctx := context.Background()
	var err error
	if b.team, err = h.store.CreateTeam(ctx, "Platform"); err != nil {
		t.Fatal(err)
	}
	for email, role := range map[string]store.Role{"lead@example.com": store.RoleLead, "member@example.com": store.RoleMember} {
		if err := h.store.AddMember(ctx, b.team.ID, h.user(email).ID, role); err != nil {
			t.Fatal(err)
		}
	}
	if b.board, err = h.store.CreateBoard(ctx, b.team.ID, "Sprint", []string{"Todo", "Doing", "Done"}); err != nil {
		t.Fatal(err)
	}
	b.cols, _ = h.store.Columns(ctx, b.board.ID)
	b.path = "/boards/" + id(b.board.ID)
	return b
}

func (b boardSetup) card(t *testing.T, col int, title string) store.Card {
	t.Helper()
	c, err := b.h.store.CreateCard(context.Background(), b.board.ID, b.cols[col].ID, title, b.h.user("lead@example.com").ID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (b boardSetup) cardPath(c store.Card) string { return b.path + "/cards/" + id(c.ID) }

func id(n int64) string { return strconv.FormatInt(n, 10) }

func TestLeadCreatesABoardFromTheTeamPage(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	path := teamWith(t, h, "lead@example.com", "")
	res := lead.Submit(path, path, url.Values{"op": {"create_board"}, "board_name": {"Sprint 1"}})
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/boards/") {
		t.Fatalf("create board = %d %q:\n%s", res.Status, res.Location(), res.Body)
	}
	page := lead.Get(res.Location())
	mustContain(t, page.Body, "Sprint 1", "Yapılacak", "Yapılıyor", "Bitti")
	mustContain(t, lead.Get(path).Body, "Sprint 1")
	mustContain(t, lead.Get("/").Body, "Sprint 1")
}

func TestMembersCannotCreateBoards(t *testing.T) {
	h := newHarness(t, "")
	h.signedIn("lead", "lead@example.com")
	member := h.signedIn("member", "member@example.com")
	path := teamWith(t, h, "lead@example.com", "member@example.com")
	if res := member.Submit(path, path, url.Values{"op": {"create_board"}, "board_name": {"X"}}); res.Status != http.StatusForbidden {
		t.Fatalf("member create board = %d, want 403", res.Status)
	}
}

func TestBoardShowsItsCards(t *testing.T) {
	b := newBoardSetup(t)
	b.card(t, 0, "Write the spec")
	res := b.member.Get(b.path)
	if res.Status != http.StatusOK {
		t.Fatalf("GET board = %d", res.Status)
	}
	mustContain(t, res.Body, "Sprint", "Todo", "Doing", "Done", "Write the spec",
		`data-collage-fragment="`+b.path+`/columns"`, `data-collage-push`, `data-collage-swap="morph"`, `src="/static/js/board.`, `src="/static/vendor/Sortable.min.`)
	frag := b.member.Get(b.path + "/columns")
	if frag.Status != http.StatusOK || !strings.Contains(frag.Body, "Write the spec") || strings.Contains(frag.Body, "<html") {
		t.Fatalf("columns fragment = %d:\n%s", frag.Status, frag.Body)
	}
}

func TestOutsidersGetNotFoundForBoardsAndCards(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Secret")
	out := b.h.signedIn("out", "out@example.com")
	for _, p := range []string{b.path, b.cardPath(c), b.path + "/settings", "/boards/abc", "/boards/99999"} {
		if res := out.Get(p); res.Status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, res.Status)
		}
	}
	// A board's fragment paths answer 404 as its pages do (framework-issues/003,
	// fixed in collage v0.39.2), and send nothing of the board.
	for _, p := range []string{b.path + "/columns", b.cardPath(c) + "/panel", b.path + "/gantt/chart", "/boards/99999/columns"} {
		res := out.Get(p)
		if res.Status != http.StatusNotFound || strings.Contains(res.Body, "Secret") {
			t.Errorf("GET %s = %d, want 404 with nothing of the card:\n%s", p, res.Status, res.Body)
		}
	}
	res := out.Submit("/", b.path, url.Values{"op": {"create_card"}, "title": {"x"}})
	if res.Status != http.StatusNotFound {
		t.Errorf("outsider POST = %d, want 404", res.Status)
	}
}

func TestCreateCard(t *testing.T) {
	b := newBoardSetup(t)
	res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "title": {"New idea"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("create = %d:\n%s", res.Status, res.Body)
	}
	mustContain(t, b.member.Get(b.path).Body, "New idea")
	res = b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "title": {"  "}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("blank title = %d, want 422", res.Status)
	}
}

func moveForm(c store.Card, to int64, index int, from int64, version int) url.Values {
	return url.Values{
		"op": {"move"}, "card": {id(c.ID)}, "to_column": {id(to)}, "to_index": {strconv.Itoa(index)},
		"expected_from": {id(from)}, "expected_version": {strconv.Itoa(version)},
	}
}

func TestMoveAnswersWithTheColumns(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	res := b.member.SubmitFetch(b.path, b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	if res.Status != http.StatusOK || strings.Contains(res.Body, "<html") || !strings.Contains(res.Body, "Card") {
		t.Fatalf("move = %d:\n%s", res.Status, res.Body)
	}
	moved, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID)
	if moved.ColumnID != b.cols[1].ID {
		t.Fatalf("card is in column %d", moved.ColumnID)
	}
}

func TestMovingAStaleCardIsAConflict(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	if res := b.lead.SubmitFetch(b.path, b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version)); res.Status != http.StatusOK {
		t.Fatalf("first move = %d", res.Status)
	}
	res := b.member.SubmitFetch(b.path, b.path, moveForm(c, b.cols[2].ID, 0, b.cols[0].ID, c.Version))
	if res.Status != http.StatusConflict {
		t.Fatalf("stale move = %d, want 409", res.Status)
	}
	mustContain(t, res.Body, "Bu kart başka biri tarafından değiştirildi.")
	moved, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID)
	if moved.ColumnID != b.cols[1].ID {
		t.Fatalf("the stale move was applied")
	}
}

func TestMoveWithoutScriptRedirectsToTheCard(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	form := moveForm(c, b.cols[2].ID, 1<<20, b.cols[0].ID, c.Version)
	res := b.member.Submit(b.cardPath(c), b.path, form)
	if res.Status != http.StatusSeeOther || res.Location() != b.cardPath(c) {
		t.Fatalf("fallback move = %d %q", res.Status, res.Location())
	}
	moved, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID)
	if moved.ColumnID != b.cols[2].ID {
		t.Fatalf("card is in column %d", moved.ColumnID)
	}
	res = b.member.Submit(b.cardPath(c), b.path, form) // stale now
	if res.Status != http.StatusSeeOther {
		t.Fatalf("stale fallback move = %d", res.Status)
	}
	mustContain(t, b.member.Get(res.Location()).Body, "Bu kart başka biri tarafından değiştirildi.")
}

func TestAddingACardInAColumn(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	if err := b.h.store.UpdateColumn(ctx, b.board.ID, b.cols[1].ID, store.ColumnUpdate{Name: "Doing", AllowCreate: true}); err != nil {
		t.Fatal(err)
	}
	page := b.member.Get(b.path).Body
	mustContain(t, page, `name="column" value="`+id(b.cols[1].ID)+`"`)
	if strings.Contains(page, `name="column" value="`+id(b.cols[2].ID)+`"`) {
		t.Error("a column cards are not made in offers to add one")
	}
	res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "column": {id(b.cols[1].ID)}, "title": {"Here"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("create in column = %d:\n%s", res.Status, res.Body)
	}
	cards, _ := b.h.store.BoardCards(ctx, b.board.ID)
	if len(cards) != 1 || cards[0].Card.ColumnID != b.cols[1].ID {
		t.Fatalf("cards = %+v", cards)
	}
	// A column of no board, or one cards are not made in, is refused.
	if res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "column": {"999999"}, "title": {"X"}}); res.Status != http.StatusBadRequest {
		t.Errorf("unknown column = %d, want 400", res.Status)
	}
	if res := b.member.Submit(b.path, b.path, url.Values{"op": {"create_card"}, "column": {id(b.cols[2].ID)}, "title": {"X"}}); res.Status != http.StatusBadRequest {
		t.Errorf("a column cards are not made in = %d, want 400", res.Status)
	}
}
