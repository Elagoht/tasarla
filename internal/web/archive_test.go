package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestTheArchiveListsAndRestoresCards(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Gone for now")
	if res := b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"confirm": {"1"}, "op": {"archive"}}); res.Status != http.StatusSeeOther {
		t.Fatalf("archive = %d", res.Status)
	}
	// The board links to its archive; the archive lists the card.
	mustContain(t, b.member.Get(b.path).Body, `href="`+b.path+`/archive"`)
	page := b.member.Get(b.path + "/archive")
	if page.Status != http.StatusOK {
		t.Fatalf("archive page = %d", page.Status)
	}
	mustContain(t, page.Body, "Gone for now", "Todo kolonundan", "Member arşivledi", `name="op" value="restore"`)
	if strings.Contains(b.member.Get(b.path+"/archive?q=nothing").Body, "Gone for now") {
		t.Error("the search kept a card that does not match")
	}
	// An outsider sees no archive.
	out := b.h.signedIn("out", "out@example.com")
	if res := out.Get(b.path + "/archive"); res.Status != http.StatusNotFound {
		t.Errorf("outsider archive = %d", res.Status)
	}
	// Restore from the archive page.
	res := b.member.Submit(b.path+"/archive", b.path+"/archive", url.Values{"op": {"restore"}, "card": {id(c.ID)}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("restore = %d", res.Status)
	}
	mustContain(t, b.member.Get(b.path+"/archive").Body, "Kart arşivden geri yüklendi.")
	if got, _ := b.h.store.Card(ctx, b.board.ID, c.ID); got.ArchivedAt != nil {
		t.Fatal("not restored")
	}
	mustContain(t, b.member.Get(b.cardPath(c)).Body, "kartı arşivden geri yükledi")
}

func TestAnArchivedCardIsRestoredFromItsPageOnly(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Card")
	// Archiving says it can be undone, in the dialog and on the page without a script.
	mustContain(t, b.member.Get(b.cardPath(c)).Body, `data-confirm-hint="Arşivden geri yükleyebilirsiniz."`)
	mustContain(t, b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"archive"}}).Body, "Arşivden geri yükleyebilirsiniz.")
	b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"confirm": {"1"}, "op": {"archive"}})
	page := b.member.Get(b.cardPath(c)).Body
	mustContain(t, page, "Bu kart arşivde.", `name="op" value="restore"`)
	if res := b.member.Submit(b.cardPath(c), b.cardPath(c), fieldForm(c, "title", "Edited")); res.Status != http.StatusForbidden {
		t.Errorf("editing an archived card = %d, want 403", res.Status)
	}
	res := b.member.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"restore"}})
	if res.Status != http.StatusSeeOther || res.Location() != b.cardPath(c) {
		t.Fatalf("restore from the card = %d %q", res.Status, res.Location())
	}
	if got, _ := b.h.store.Card(context.Background(), b.board.ID, c.ID); got.ArchivedAt != nil {
		t.Fatal("not restored")
	}
}

func TestALeadRestoresAnArchivedBoard(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	team := "/teams/" + id(b.team.ID)
	if err := b.h.store.ArchiveBoard(ctx, b.board.ID); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.member.Get(team).Body, `value="restore_board"`) {
		t.Error("a member is offered to restore a board")
	}
	mustContain(t, b.lead.Get(team).Body, "Arşivlenmiş board&#39;lar", "Sprint", `value="restore_board"`)
	if res := b.member.Submit(team, team, url.Values{"op": {"restore_board"}, "board_id": {id(b.board.ID)}}); res.Status != http.StatusForbidden {
		t.Errorf("member restore = %d, want 403", res.Status)
	}
	if res := b.lead.Submit(team, team, url.Values{"op": {"restore_board"}, "board_id": {id(b.board.ID)}}); res.Status != http.StatusSeeOther {
		t.Fatalf("lead restore = %d", res.Status)
	}
	if res := b.member.Get(b.path); res.Status != http.StatusOK {
		t.Fatalf("restored board = %d", res.Status)
	}
}

// A card moved into a done column is completed: it leaves the board for the
// Done page, from where it is reopened into a column, through its rules.
func TestCompletingAndReopeningACard(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Ship it")
	if res := b.member.SubmitFetch(b.path, b.path, moveForm(c, b.cols[2].ID, 0, b.cols[0].ID, c.Version)); res.Status != http.StatusOK || strings.Contains(res.Body, "Ship it") {
		t.Fatalf("complete = %d; the card is still on the board: %v", res.Status, strings.Contains(res.Body, "Ship it"))
	}
	mustContain(t, b.member.Get(b.path).Body, `href="`+b.path+`/done"`, "Buraya bırakılan kart tamamlanır")
	done := b.member.Get(b.path + "/done").Body
	mustContain(t, done, "Ship it", "Member tamamladı", `name="op" value="reopen"`, `<option value="`+id(b.cols[0].ID)+`" selected>`)
	if strings.Contains(done, `<option value="`+id(b.cols[2].ID)+`"`) {
		t.Error("a done column is offered to reopen into")
	}
	panel := b.member.Get(b.cardPath(c)).Body
	mustContain(t, panel, "Tamamlandı", "Bitenlere git")
	got, _ := b.h.store.Card(ctx, b.board.ID, c.ID)
	// Reopening into a done column is refused; into another one it goes back.
	if res := b.member.Submit(b.path+"/done", b.path+"/done", url.Values{"op": {"reopen"}, "card": {id(c.ID)},
		"to_column": {id(b.cols[2].ID)}, "expected_version": {strconv.Itoa(got.Version)}}); res.Status != http.StatusBadRequest {
		t.Errorf("reopen into done = %d, want 400", res.Status)
	}
	res := b.member.Submit(b.path+"/done", b.path+"/done", url.Values{"op": {"reopen"}, "card": {id(c.ID)},
		"to_column": {id(b.cols[1].ID)}, "expected_version": {strconv.Itoa(got.Version)}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("reopen = %d", res.Status)
	}
	if got, _ := b.h.store.Card(ctx, b.board.ID, c.ID); got.CompletedAt != nil || got.ColumnID != b.cols[1].ID {
		t.Fatalf("after reopen: %+v", got)
	}
	mustContain(t, b.member.Get(b.path).Body, "Ship it")
	mustContain(t, b.member.Get(b.cardPath(c)).Body, "kartı tamamladı", "kartı yeniden açtı")
}
