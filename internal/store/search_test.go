package store_test

import (
	"context"
	"strconv"
	"testing"

	"kanban/internal/store"
)

func TestSearchFindsOnlyTheReadersTeams(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	mine, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Rapor taslağı", f.lead.ID)
	other, _ := f.s.CreateTeam(ctx, "Other")
	otherBoard, _ := f.s.CreateBoard(ctx, other.ID, "Theirs", []string{"Todo"})
	otherCols, _ := f.s.Columns(ctx, otherBoard.ID)
	f.s.CreateCard(ctx, otherBoard.ID, otherCols[0].ID, "Rapor başkasının", f.lead.ID)

	hits, more, err := f.s.Search(ctx, f.member.ID, "RAPOR", 1)
	if err != nil {
		t.Fatal(err)
	}
	if more || len(hits) != 1 || hits[0].Card.ID != mine.ID || !hits[0].InTitle || hits[0].BoardName != "Sprint" || hits[0].TeamName != "Platform" {
		t.Fatalf("hits = %+v, more %v", hits, more)
	}
}

// The fold must not depend on the database's collation.
func TestFoldIgnoresTheCollation(t *testing.T) {
	f := newBoardFixture(t)
	var got string
	if err := f.s.Pool().QueryRow(context.Background(),
		`SELECT kanban_fold('IĞDIR İSTANBUL ŞÇÖÜ' COLLATE "C")`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if want := "iğdir istanbul şçöü"; got != want {
		t.Errorf("fold = %q, want %q", got, want)
	}
}

func TestSearchHugePageIsClamped(t *testing.T) {
	f := newBoardFixture(t)
	if _, _, err := f.s.Search(context.Background(), f.member.ID, "rapor", int(^uint(0)>>1)); err != nil {
		t.Fatal(err)
	}
}

func TestSearchExcerptsAndOrder(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	inComment, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Toplantı", f.lead.ID)
	f.s.AddComment(ctx, f.board.ID, inComment.ID, f.lead.ID, "ışık bütçesine göre karar", nil)
	gone, _ := f.s.AddComment(ctx, f.board.ID, inComment.ID, f.lead.ID, "ışık gizli taslak", nil)
	f.s.DeleteComment(ctx, f.board.ID, inComment.ID, gone.ID, f.lead.ID, true)
	inTitle, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Işık testi", f.lead.ID)

	hits, _, err := f.s.Search(ctx, f.member.ID, "ışık", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Card.ID != inTitle.ID || hits[1].Card.ID != inComment.ID {
		t.Fatalf("order = %+v", hits)
	}
	if hits[1].Excerpt != "ışık bütçesine göre karar" {
		t.Errorf("excerpt = %q, want the live comment, never the deleted one", hits[1].Excerpt)
	}
}

func TestSearchSkipsArchivedBoardsAndPages(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	for i := range store.SearchPageSize + 1 {
		f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Kart "+strconv.Itoa(i), f.lead.ID)
	}
	first, more, _ := f.s.Search(ctx, f.member.ID, "kart", 1)
	second, more2, _ := f.s.Search(ctx, f.member.ID, "kart", 2)
	if len(first) != store.SearchPageSize || !more || len(second) != 1 || more2 {
		t.Fatalf("pages: %d (more %v), %d (more %v)", len(first), more, len(second), more2)
	}
	if err := f.s.ArchiveBoard(ctx, f.board.ID); err != nil {
		t.Fatal(err)
	}
	if hits, _, _ := f.s.Search(ctx, f.member.ID, "kart", 1); len(hits) != 0 {
		t.Errorf("an archived board is searched: %d hits", len(hits))
	}
	if hits, _, _ := f.s.Search(ctx, f.member.ID, "k", 1); hits != nil {
		t.Error("a one-letter query ran")
	}
}
