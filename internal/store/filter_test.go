package store_test

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"kanban/internal/store"
)

// filterFixture is a board with cards that differ in each thing a filter looks at.
type filterFixture struct {
	boardFixture
	plain, mine, urgent, labelled, late, today, sunday, commented, deletedComment store.Card
	label                                                                         store.Label
	now                                                                           time.Time
}

func newFilterFixture(t *testing.T) filterFixture {
	t.Helper()
	f := filterFixture{boardFixture: newBoardFixture(t)}
	ctx := context.Background()
	// Wednesday 2026-09-30: the week runs Monday 28th to Sunday 4 October.
	f.now = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	card := func(title string) store.Card {
		c, err := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, title, f.lead.ID)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	set := func(c store.Card, field store.CardField, v store.CardFields) store.Card {
		out, err := f.s.UpdateCardField(ctx, f.board.ID, c.ID, c.Version, field, v, f.lead.ID)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	date := func(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }
	prio := func(p int16) *int16 { return &p }

	f.plain = card("Plain")
	f.mine = set(card("Mine"), store.FieldAssignee, store.CardFields{AssigneeID: &f.member.ID})
	f.urgent = set(card("Urgent"), store.FieldPriority, store.CardFields{Priority: prio(4)})
	f.labelled = card("Labelled")
	var err error
	if f.label, err = f.s.CreateLabel(ctx, f.board.ID, "bug", "#d94f4f"); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetCardLabels(ctx, f.board.ID, f.labelled.ID, []int64{f.label.ID}); err != nil {
		t.Fatal(err)
	}
	f.late = set(card("Late"), store.FieldDueDate, store.CardFields{DueDate: date("2026-09-29")})
	f.today = set(card("Today"), store.FieldDueDate, store.CardFields{DueDate: date("2026-09-30")})
	f.sunday = set(card("Sunday"), store.FieldDueDate, store.CardFields{DueDate: date("2026-10-04")})
	f.commented = card("Commented")
	if _, err := f.s.AddComment(ctx, f.board.ID, f.commented.ID, f.lead.ID, "IĞDIR raporu hazır", nil); err != nil {
		t.Fatal(err)
	}
	f.deletedComment = card("Deleted comment")
	gone, err := f.s.AddComment(ctx, f.board.ID, f.deletedComment.ID, f.lead.ID, "gizli kelime", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteComment(ctx, f.board.ID, f.deletedComment.ID, gone.ID, f.lead.ID, true); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f filterFixture) match(t *testing.T, filter store.BoardFilter) []int64 {
	t.Helper()
	filter.Today = f.now
	got, err := f.s.MatchingCardIDs(context.Background(), f.board.ID, filter)
	if err != nil {
		t.Fatal(err)
	}
	return slices.Sorted(maps.Keys(got))
}

func ids(cards ...store.Card) []int64 {
	out := make([]int64, len(cards))
	for i, c := range cards {
		out[i] = c.ID
	}
	slices.Sort(out)
	return out
}

func TestMatchingCardIDsByEachDimension(t *testing.T) {
	f := newFilterFixture(t)
	cases := []struct {
		name   string
		filter store.BoardFilter
		want   []int64
	}{
		{"assignee", store.BoardFilter{AssigneeIDs: []int64{f.member.ID}}, ids(f.mine)},
		{"unassigned or member", store.BoardFilter{AssigneeIDs: []int64{f.member.ID}, Unassigned: true},
			ids(f.plain, f.mine, f.urgent, f.labelled, f.late, f.today, f.sunday, f.commented, f.deletedComment)},
		{"label", store.BoardFilter{LabelIDs: []int64{f.label.ID}}, ids(f.labelled)},
		{"priority", store.BoardFilter{Priorities: []int16{4}}, ids(f.urgent)},
		{"overdue", store.BoardFilter{Due: store.DueOverdue}, ids(f.late)},
		{"today", store.BoardFilter{Due: store.DueToday}, ids(f.today)},
		{"this week runs to Sunday", store.BoardFilter{Due: store.DueWeek}, ids(f.today, f.sunday)},
		{"no due date", store.BoardFilter{Due: store.DueNone},
			ids(f.plain, f.mine, f.urgent, f.labelled, f.commented, f.deletedComment)},
		{"text in title", store.BoardFilter{Text: "urg"}, ids(f.urgent)},
		{"dimensions are and-ed", store.BoardFilter{Text: "late", Due: store.DueToday}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := f.match(t, c.filter); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// ı, I, i and İ match one another, in a comment as in a title; a deleted
// comment matches nothing; a two-letter query works without the trigram index.
func TestMatchingCardIDsText(t *testing.T) {
	f := newFilterFixture(t)
	for _, q := range []string{"ığdır", "IĞDIR", "iğdir", "İĞDİR", "rapor"} {
		if got := f.match(t, store.BoardFilter{Text: q}); !slices.Equal(got, ids(f.commented)) {
			t.Errorf("%q: got %v, want the commented card", q, got)
		}
	}
	if got := f.match(t, store.BoardFilter{Text: "gizli"}); len(got) != 0 {
		t.Errorf("a deleted comment matched: %v", got)
	}
	if got := f.match(t, store.BoardFilter{Text: "ğd"}); !slices.Equal(got, ids(f.commented)) {
		t.Errorf("two letters: got %v", got)
	}
	if got := f.match(t, store.BoardFilter{Text: "50%_"}); len(got) != 0 {
		t.Errorf("%% and _ are wildcards: %v", got)
	}
}

func TestMatchingCardIDsEmptyFilterIsNil(t *testing.T) {
	f := newFilterFixture(t)
	got, err := f.s.MatchingCardIDs(context.Background(), f.board.ID, store.BoardFilter{Today: f.now})
	if err != nil || got != nil {
		t.Fatalf("empty filter = %v, %v; want nil, nil", got, err)
	}
}
