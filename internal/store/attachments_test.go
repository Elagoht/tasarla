package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/store"
)

func TestAttachments(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	a := f.card(t, 0, "A")
	att, err := f.s.AddAttachment(ctx, f.board.ID, a.ID, store.Attachment{UploaderID: f.member.ID, Filename: "plan.pdf", ContentType: "application/pdf", Size: 10, StorageKey: "00112233445566778899aabbccddeeff"})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := f.s.CardAttachments(ctx, a.ID)
	if len(list) != 1 || list[0].UploaderName != "User member" || list[0].Filename != "plan.pdf" {
		t.Fatalf("list = %+v", list)
	}

	// Reachable by a team member, by an admin, and by nobody else.
	if got, err := f.s.AttachmentFor(ctx, att.ID, f.member); err != nil || got.BoardID != f.board.ID {
		t.Fatalf("member: %+v, %v", got, err)
	}
	admin := mustUpsert(t, f.s, identity("admin", "admin@example.com"), "admin@example.com")
	if _, err := f.s.AttachmentFor(ctx, att.ID, admin); err != nil {
		t.Fatalf("admin: %v", err)
	}
	outsider := mustUpsert(t, f.s, identity("out", "out@example.com"))
	if _, err := f.s.AttachmentFor(ctx, att.ID, outsider); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("outsider: %v", err)
	}

	if _, err := f.s.DeleteAttachment(ctx, f.board.ID, a.ID, att.ID, f.lead.ID, false); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("not the uploader, not a manager: %v", err)
	}
	key, err := f.s.DeleteAttachment(ctx, f.board.ID, a.ID, att.ID, f.member.ID, false)
	if err != nil || key != att.StorageKey {
		t.Fatalf("delete = %q, %v", key, err)
	}
	act, _ := f.s.CardActivity(ctx, a.ID, 10)
	if join(kinds(act)) != "attachment_deleted,attachment_added,card_created" {
		t.Errorf("activity = %v", kinds(act))
	}
}
