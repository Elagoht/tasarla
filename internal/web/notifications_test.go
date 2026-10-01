package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"kanban/internal/store"
)

func pendingMails(t *testing.T, h *harness) []store.OutboxMessage {
	t.Helper()
	var got []store.OutboxMessage
	// Leave them pending: refuse every send, then look at what was tried.
	h.store.ProcessOutbox(context.Background(), time.Now().Add(time.Minute), 100, func(m store.OutboxMessage) error {
		got = append(got, m)
		return errSMTPDown
	})
	return got
}

type smtpDown struct{}

func (smtpDown) Error() string { return "smtp down" }

var errSMTPDown = smtpDown{}

func TestAssigningNotifiesTheAssignee(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Payments")
	memberID := id(b.h.user("member@example.com").ID)
	res := b.lead.Submit(b.cardPath(c), b.cardPath(c), updateForm(c, map[string]string{"assignee_id": memberID}))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("assign = %d (the action must succeed even with SMTP down)", res.Status)
	}
	page := b.member.Get("/notifications").Body
	mustContain(t, page, "Lead sizi bir karta atadı: Payments", `href="`+b.cardPath(c)+`"`)
	mustContain(t, b.member.Get("/").Body, `data-collage-fragment="/notifications/badge"`, `class="badge">1<`)
	badge := b.member.Get("/notifications/badge")
	if badge.Status != http.StatusOK || !strings.Contains(badge.Body, ">1<") {
		t.Fatalf("badge = %d %q", badge.Status, badge.Body)
	}
	if strings.Contains(b.lead.Get("/").Body, `class="badge"`) {
		t.Error("the lead was notified of their own action")
	}
	mails := pendingMails(t, b.h)
	if len(mails) != 1 || mails[0].To != "member@example.com" || !strings.Contains(mails[0].Subject, "Lead sizi bir karta atadı") {
		t.Fatalf("mails = %+v", mails)
	}
	if n, _ := b.h.store.OutboxCount(context.Background(), "pending"); n != 1 {
		t.Fatalf("outbox pending = %d, want the mail kept for later", n)
	}

	res = b.member.Submit("/notifications", "/notifications", url.Values{"op": {"read_all"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("read_all = %d", res.Status)
	}
	if strings.Contains(b.member.Get("/").Body, `class="badge"`) {
		t.Error("the badge is still shown after reading everything")
	}
}

func TestMentionsAndCommentsNotify(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	c := b.card(t, 0, "Card")
	memberID := b.h.user("member@example.com").ID
	c, _ = b.h.store.UpdateCard(ctx, b.board.ID, c.ID, c.Version, store.CardFields{Title: "Card", AssigneeID: &memberID}, 0)
	b.lead.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_add"}, "comment": {"FYI @member"}})
	page := b.member.Get("/notifications").Body
	mustContain(t, page, "Lead sizi bir yorumda andı: Card")
	if strings.Contains(page, "yorum yaptı") {
		t.Error("a mentioned assignee is told twice")
	}
	b.lead.Submit(b.cardPath(c), b.cardPath(c), url.Values{"op": {"comment_add"}, "comment": {"another"}})
	mustContain(t, b.member.Get("/notifications").Body, "Lead atandığınız karta yorum yaptı: Card")
}

func TestFinishingABlockerNotifiesTheBlockedCardsAssignee(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	blocker, blocked := b.card(t, 0, "Blocker"), b.card(t, 0, "Blocked")
	memberID := b.h.user("member@example.com").ID
	b.h.store.UpdateCard(ctx, b.board.ID, blocked.ID, blocked.Version, store.CardFields{Title: "Blocked", AssigneeID: &memberID}, 0)
	b.h.store.AddDependency(ctx, b.board.ID, blocker.ID, blocked.ID)
	blocker, _ = b.h.store.Card(ctx, b.board.ID, blocker.ID)
	if res := b.lead.SubmitFetch(b.path, b.path, moveForm(blocker, b.cols[2].ID, 0, b.cols[0].ID, blocker.Version)); res.Status != http.StatusOK {
		t.Fatalf("move = %d", res.Status)
	}
	mustContain(t, b.member.Get("/notifications").Body, "Blocked kartını bloklayan kartların hepsi bitti.")
}

func TestMySettings(t *testing.T) {
	b := newBoardSetup(t)
	res := b.member.Submit("/me/settings", "/me/settings", url.Values{"op": {"save"}, "locale": {"en"}, "email_assigned": {"1"}, "email_commented": {"1"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("save = %d", res.Status)
	}
	member := b.h.user("member@example.com")
	prefs, _ := b.h.store.NotificationPrefs(context.Background(), member.ID)
	if member.Locale != "en" || !prefs["assigned"] || prefs["mentioned"] || !prefs["commented"] {
		t.Fatalf("locale %q prefs %v", member.Locale, prefs)
	}
	mustContain(t, b.member.Get("/me/settings").Body, "Kaydedildi.", `value="en" selected`)
	if res := b.member.Submit("/me/settings", "/me/settings", url.Values{"op": {"save"}, "locale": {"fr"}}); res.Status != http.StatusBadRequest {
		t.Fatalf("unknown locale = %d, want 400", res.Status)
	}

	// The e-mail now goes in English, while the lead works in Turkish.
	c := b.card(t, 0, "Report")
	b.lead.Submit(b.cardPath(c), b.cardPath(c), updateForm(c, map[string]string{"assignee_id": id(member.ID)}))
	mails := pendingMails(t, b.h)
	if len(mails) != 1 || !strings.Contains(mails[0].Subject, "Lead assigned you to a card: Report") || !strings.Contains(mails[0].Text, "/en/boards/") {
		t.Fatalf("mails = %+v", mails)
	}
}
