package web

import (
	"strings"
	"testing"

	"kanban/internal/store"
)

func card(id int64, col int64, assignee *int64, name string, prio *int16) cardView {
	return cardView{Summary: store.CardSummary{Card: store.Card{ID: id, ColumnID: col, AssigneeID: assignee, Priority: prio}, AssigneeName: name}}
}

func TestBuildLanesByAssignee(t *testing.T) {
	zeynep, ali, gone := int64(1), int64(2), int64(3)
	members := []store.Member{{User: store.User{ID: zeynep, Name: "Zeynep"}}, {User: store.User{ID: ali, Name: "ali"}}, {User: store.User{ID: 9, Name: "Boş"}}}
	cols := []columnView{
		{Column: store.Column{ID: 10}, Cards: []cardView{card(100, 10, &zeynep, "Zeynep", nil), card(101, 10, nil, "", nil)}},
		{Column: store.Column{ID: 20, IsDone: true}, Cards: []cardView{card(200, 20, &gone, "Eski", nil), card(201, 20, &ali, "ali", nil)}},
	}
	lanes := buildLanes("assignee", cols, members, "Atanmamış", "Yok", nil)
	var keys []string
	for _, l := range lanes {
		keys = append(keys, l.Key)
	}
	want := []string{"assignee:none", "assignee:2", "assignee:1", "assignee:3"}
	if len(keys) != len(want) {
		t.Fatalf("lanes = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("lanes = %v, want %v", keys, want)
		}
	}
	if !lanes[3].Former || lanes[3].Label != "Eski" || lanes[1].Former {
		t.Errorf("former member lane: %+v", lanes[3])
	}
	if lanes[0].Value != "none" || lanes[1].Value != "2" || lanes[0].Count != 1 {
		t.Errorf("values/counts: %+v", lanes[0])
	}
	if len(lanes[1].Cells) != 2 || len(lanes[1].Cells[1].Cards) != 1 || !lanes[1].Cells[1].Done || len(lanes[1].Cells[0].Cards) != 0 {
		t.Errorf("ali's cells: %+v", lanes[1].Cells)
	}
}

// Turkish alphabet: Ç after C, İ after I, Z last.
func TestBuildLanesByAssigneeTurkishOrder(t *testing.T) {
	names := []string{"Zeynep", "Çağla", "Can", "İpek", "Ilgın"}
	var members []store.Member
	var cards []cardView
	for i, n := range names {
		id := int64(i + 1)
		members = append(members, store.Member{User: store.User{ID: id, Name: n}})
		cards = append(cards, card(100+id, 10, &id, n, nil))
	}
	lanes := buildLanes("assignee", []columnView{{Column: store.Column{ID: 10}, Cards: cards}}, members, "Atanmamış", "Yok", nil)
	var got []string
	for _, l := range lanes {
		got = append(got, l.Label)
	}
	want := []string{"Can", "Çağla", "Ilgın", "İpek", "Zeynep"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("lanes = %v, want %v", got, want)
	}
}

func TestBuildLanesByPriority(t *testing.T) {
	high, low := int16(3), int16(1)
	cols := []columnView{{Column: store.Column{ID: 10}, Cards: []cardView{card(1, 10, nil, "", &low), card(2, 10, nil, "", nil), card(3, 10, nil, "", &high)}}}
	lanes := buildLanes("priority", cols, nil, "Atanmamış", "Yok", func(p int16) string { return map[int16]string{3: "Yüksek", 1: "Düşük"}[p] })
	var got []string
	for _, l := range lanes {
		got = append(got, l.Key+"="+l.Label)
	}
	want := "priority:3=Yüksek priority:1=Düşük priority:none=Yok"
	if s := strings.Join(got, " "); s != want {
		t.Errorf("lanes = %s, want %s", s, want)
	}
}
