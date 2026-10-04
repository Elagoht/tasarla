# Board takvim görünümü — uygulama planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Board'a Gantt'ın yanında bir Takvim sekmesi: kartları ay ızgarasında tarihleriyle gösterir, sürükleyerek tarih değiştirir, boş güne tıklayınca o güne bitiş tarihli kart açar.

**Architecture:** Gantt'ın kalıbı: `board-calendar` sayfası (başlık, sekmeler, filtre) ve içinde dinamik `board-calendar-grid` parçası. Izgara saf bir Go fonksiyonuyla (`layoutCalendar`) hesaplanır, sunucuda çizilir; `static/js/calendar.js` yalnız sürükleme, klavye ve yeni kart formunu yönetir. Tarih kaydı kartın mevcut `set_dates` işlemini kullanır; kart açma takvim sayfasının kendi `create_card` eylemidir ve yeni `store.CreateCardDue`'yu çağırır.

**Tech Stack:** Go, collage (sayfa/parça/eylem), html/template, PostgreSQL (pgx), düz JS modülü, CSS.

**Spec:** `docs/superpowers/specs/2026-10-04-takvim-gorunumu-design.md`

## Global Constraints

- Testler: `export KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable'`, ardından `go test ./... -race -count=1`. Bu değişken yoksa veritabanı testleri atlanır; o zaman "geçti" sayılmaz.
- Go'da `any` / `interface{}` tipi kullanılmaz (kullanıcı kuralı).
- CSP `style-src 'self' <hash>`: şablonlarda satır içi `style="…"` YASAK. Çubuklar sınıflarla konumlanır (`c1…c7`, `s1…s7`, `l1…l3`).
- CSS ölçüleri rem; renkler tema değişkenleri (`--sheet`, `--shelf`, `--line`, `--ink*`, `--accent`, `--danger`, `--ok`).
- Haftalar Pazartesi başlar. Ay parametresi `?month=YYYY-MM`.
- Çeviri anahtarları `calendar_view.*` altında (spec "calendar.*" der, ama `calendar.*` iCal aboneliğine ait — çakışmasın diye). Gün adları mevcut `weekdays.0`…`weekdays.6` (0 = Pazartesi).
- Görünen şerit sayısı: `calendarLanes = 3`.
- Kodun yorum dili, adlandırma ve deyimi çevredeki kodla aynı (İngilizce yorumlar, kısa ve "ne/neden").
- `href="{{$self}}?{{…}}"` KULLANMA: html/template `?`'ten sonraki metindeki `=` ve `&`'yi `%3d`/`%26` yapar ve bağlantı bozulur (Gantt'ın ölçek bağlantıları bugün bu yüzden bozuk). Bağlantının tamamı Go'da kurulur ve `template.URL` olarak döner (`monthLink`).
- Ad çakışması: `calendarLink` adı `internal/web/pages_notifications.go`'da zaten var; yeni bağlantı fonksiyonunun adı `monthLink`. Planda geçen diğer adlar (`calendarView`, `calendarMonth`, `calendarGrid`, `boardMatching` …) kontrol edildi, çakışmıyor.

## Spec'ten sapmalar (bilerek)

- **"Kart açılabilen kolon yoksa 409" yok.** `creatableColumns` hiçbir kolon işaretli değilse ilk kolonu döndürüyor (board'un kendi davranışı). Takvim de aynısını yapar; 409 yalnız board'da hiç kolon yokken.
- **Okuma yetkili üye testi yok.** `authz.Board` takım üyesine her zaman `CanEdit` verir, arşivli board 404 döner; `CanEdit=false` görebilen kullanıcı bugün yok. `CanEdit` kapısı şablonda ve eylemde yine de durur; testler dışarıdaki kullanıcının 404 aldığını doğrular.

## Review Focus

1. **Ay sınırını aşan kart** (ör. 28 Eyl – 3 Eki): Ekim ızgarasının ilk haftasında Pazartesi'den başlayıp "öncesi var" işaretiyle çizilmeli, kaybolmamalı. → Görev 2 `TestLayoutCalendarSplitsAtWeekEdges`.
2. **Yalnız başlangıç tarihi olan kart sürüklenince** bitiş tarihi uydurulmamalı, boş kalmalı. → Görev 5 elle doğrulama adımı; sunucu tarafı `set_dates` zaten kapsanıyor (`TestSettingDatesFromTheChart`).
3. **Filtre açıkken ay değiştirmek** filtreyi kaybetmemeli; ay bağlantıları filtre sorgusunu taşımalı. → Görev 3 `TestCalendarKeepsTheFilterAcrossMonths`.
4. **Geçersiz `month`** (`2026-13`, `abc`, boş) 500 değil bu ay olmalı. → Görev 2 `TestCalendarMonth`, Görev 3 sayfa testi.
5. **Bitiş tarihi zorunlu kuralı olan ilk kolon**: takvimden açılan kart kabul edilmeli (tarih kurala görünür), board'dan tarihsiz açılan reddedilmeli. → Görev 1 store testi, Görev 4 web testi.

---

## Dosya haritası

| Dosya | Sorumluluk |
| --- | --- |
| `internal/store/cards.go` (değişir) | `createCard` bir `Card` alır; yeni `CreateCardDue` |
| `internal/store/cards_due_test.go` (yeni) | `CreateCardDue` testleri |
| `internal/web/calendar_layout.go` (yeni) | `calendarView` tipleri, `calendarMonth`, `layoutCalendar` — saf, HTTP yok |
| `internal/web/calendar_layout_test.go` (yeni, `package web`) | düzen birim testleri |
| `internal/web/pages_calendar.go` (yeni) | sayfa + parça kurulumu, yükleyiciler, `monthLink`, `calendarPost` |
| `internal/web/calendar_page_test.go` (yeni, `package web_test`) | sayfa ve eylem testleri |
| `internal/web/pages_gantt.go` (değişir) | filtre eşleşmesi `boardMatching` yardımcısına çıkar |
| `internal/web/app.go` (değişir) | `calendarGrid` alanı, sayfa kaydı, `monthLink` şablon fonksiyonu |
| `templates/pages/board_calendar.html` (yeni) | başlık, ay gezinmesi, sekmeler, filtre, parça kabı |
| `templates/fragments/calendar.html` (yeni) | ızgara |
| `templates/partials/board_tabs.html`, `templates/partials/icons.html` (değişir) | Takvim sekmesi, `gantt` ikonu |
| `templates/layouts/base.html` (değişir) | `calendar.css` bağlantısı |
| `static/css/calendar.css` (yeni) | ızgara stili |
| `static/js/calendar.js` (yeni) | sürükleme, klavye, satır içi kart açma |
| `locales/tr.json`, `locales/en.json` (değişir) | `calendar_view.*` |

---

### Task 1: `CreateCardDue` — bitiş tarihiyle kart açma

**Files:**
- Modify: `internal/store/cards.go:54-79`
- Test: `internal/store/cards_due_test.go`

**Interfaces:**
- Consumes: —
- Produces: `func (s *Store) CreateCardDue(ctx context.Context, boardID, columnID int64, title string, due time.Time, createdBy int64) (Card, error)` — kural ihlalinde `*store.RuleError`, kolon yoksa `store.ErrNotFound`.

- [ ] **Step 1: Başarısız testi yaz** — `internal/store/cards_due_test.go`:

```go
package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"kanban/internal/rules"
	"kanban/internal/store"
)

// A card made with a due date enters a column that requires one; the same
// card without the date is refused.
func TestCreateCardDueMeetsTheEntryRule(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	if err := f.s.AddCondition(ctx, f.board.ID, store.ColumnCondition{ColumnID: f.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	var re *store.RuleError
	if _, err := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Undated", f.lead.ID); !errors.As(err, &re) {
		t.Fatalf("undated card = %v, want a rule error", err)
	}
	due := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	c, err := f.s.CreateCardDue(ctx, f.board.ID, f.cols[0].ID, "Dated", due, f.lead.ID)
	if err != nil {
		t.Fatalf("dated card: %v", err)
	}
	got, err := f.s.Card(ctx, f.board.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Dated" || got.DueDate == nil || got.DueDate.Format(time.DateOnly) != "2026-10-14" || got.StartDate != nil {
		t.Fatalf("card = %+v", got)
	}
}

func TestCreateCardDueInAnotherBoardsColumn(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	other, err := f.s.CreateBoard(ctx, f.team.ID, "Other", []string{"A"})
	if err != nil {
		t.Fatal(err)
	}
	cols, err := f.s.Columns(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.CreateCardDue(ctx, f.board.ID, cols[0].ID, "X", time.Now(), f.lead.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign column = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/store -run CreateCardDue -race -count=1`. Beklenen: `f.s.CreateCardDue undefined`.

- [ ] **Step 3: Uygula** — `internal/store/cards.go`'da `CreateCard` ve `createCard`'ı şu hale getir:

```go
// CreateCard adds a card at the bottom of a column of boardID.
func (s *Store) CreateCard(ctx context.Context, boardID, columnID int64, title string, createdBy int64) (Card, error) {
	return s.createCardTx(ctx, boardID, columnID, Card{Title: title}, createdBy)
}

// CreateCardDue adds a card due on due at the bottom of a column of boardID;
// the column's entry rules see the date.
func (s *Store) CreateCardDue(ctx context.Context, boardID, columnID int64, title string, due time.Time, createdBy int64) (Card, error) {
	return s.createCardTx(ctx, boardID, columnID, Card{Title: title, DueDate: &due}, createdBy)
}

func (s *Store) createCardTx(ctx context.Context, boardID, columnID int64, next Card, createdBy int64) (Card, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback(ctx)
	card, err := createCard(ctx, tx, boardID, columnID, next, createdBy)
	if err != nil {
		return Card{}, err
	}
	return card, tx.Commit(ctx)
}

// createCard checks next against the column's rules and inserts it.
func createCard(ctx context.Context, tx pgx.Tx, boardID, columnID int64, next Card, createdBy int64) (Card, error) {
	if err := lockBoard(ctx, tx, boardID); err != nil {
		return Card{}, err
	}
	snap, err := loadSnapshot(ctx, tx, boardID, next)
	if err != nil {
		return Card{}, err
	}
	if _, ok := snap.Columns[columnID]; !ok {
		return Card{}, ErrNotFound
	}
	if err := ruleError(rules.EvaluateCreate(columnID, snap)); err != nil {
		return Card{}, err
	}
	return insertCard(ctx, tx, boardID, snap.Columns[columnID], next, createdBy,
		ActivityCardCreated, ActivityPayload{Title: next.Title})
}
```

`loadSnapshot` yalnız `card.ID != 0` iken etiket/kontrol listesi sorgular (`internal/store/snapshot.go:62`), `next.ID` sıfır olduğu için yeni kart güvenle geçer; `HasDueDate` `next.DueDate`'ten gelir. `createCard`'ı `CreateCard`'tan başka çağıran yok (kontrol edildi).

- [ ] **Step 4: Çalıştır, geçtiğini gör.** `go test ./internal/store -race -count=1`. Beklenen: PASS (mevcut testler de).

- [ ] **Step 5: Commit**

```bash
git add internal/store/cards.go internal/store/cards_due_test.go
git commit -m "feat(store): create a card with its due date, through the column rules"
```

---

### Task 2: Izgara düzeni — `layoutCalendar`

**Files:**
- Create: `internal/web/calendar_layout.go`
- Test: `internal/web/calendar_layout_test.go` (`package web`)

**Interfaces:**
- Consumes: `span(store.Card)`, `day`, `daysBetween` (`internal/web/pages_gantt.go:200-214`); `store.GanttCard`.
- Produces (Görev 3 ve 5 kullanır):

```go
const calendarLanes = 3

type calendarView struct {
	BoardID  int64
	CanEdit  bool
	Month    string // the month shown, YYYY-MM
	Prev     string // YYYY-MM
	Next     string // YYYY-MM
	Title    string // "Ekim 2026", set by the loader
	Weekdays []string
	Action   string // where the new-card form posts, set by the loader
	Notices  []string
	Weeks    []calendarWeek
}

type calendarWeek struct {
	Days []calendarDay // Monday to Sunday
	Bars []calendarBar // only those in the shown lanes
}

type calendarDay struct {
	Date    string // YYYY-MM-DD
	Num     int
	Col     int  // 1–7
	Outside bool // in a neighbouring month
	Today   bool
	More    int  // cards on the day beyond the shown lanes
	Cards   []calendarCardRef
}

type calendarCardRef struct {
	CardID int64
	Title  string
	Done   bool
	Late   bool
}

type calendarBar struct {
	CardID  int64
	Version int
	Title   string
	Color   string
	Col     int // first day in the week, 1–7
	Span    int // days in the week, 1–7
	Lane    int // 1–calendarLanes
	Start   string
	Due     string
	Before  bool // the card began in an earlier week
	After   bool // the card goes on into a later week
	Done    bool
	Late    bool
	Dimmed  bool // left out by the board filter
}

func calendarMonth(raw string, now time.Time) time.Time
func layoutCalendar(month time.Time, cards []store.GanttCard, colors map[int64]string, matching map[int64]bool, now time.Time) calendarView
```

- [ ] **Step 1: Başarısız testleri yaz** — `internal/web/calendar_layout_test.go`:

```go
package web

import (
	"testing"
	"time"

	"kanban/internal/store"
)

func date(s string) *time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func gc(id int64, title string, start, due string) store.GanttCard {
	c := store.Card{ID: id, ColumnID: 1, Title: title, Version: 1}
	if start != "" {
		c.StartDate = date(start)
	}
	if due != "" {
		c.DueDate = date(due)
	}
	return store.GanttCard{Card: c}
}

var calNow = time.Date(2026, 10, 4, 15, 0, 0, 0, time.UTC)

func TestCalendarMonth(t *testing.T) {
	for raw, want := range map[string]string{"2026-02": "2026-02", "": "2026-10", "2026-13": "2026-10", "abc": "2026-10", "2026-1": "2026-10"} {
		if got := calendarMonth(raw, calNow).Format("2006-01"); got != want {
			t.Errorf("calendarMonth(%q) = %s, want %s", raw, got, want)
		}
	}
}

func TestLayoutCalendarGrid(t *testing.T) {
	v := layoutCalendar(*date("2026-10-01"), nil, nil, nil, calNow)
	// October 2026: Thursday the 1st to Saturday the 31st → Mon 28 Sep … Sun 1 Nov.
	if len(v.Weeks) != 5 {
		t.Fatalf("weeks = %d, want 5", len(v.Weeks))
	}
	first, last := v.Weeks[0].Days[0], v.Weeks[4].Days[6]
	if first.Date != "2026-09-28" || !first.Outside || last.Date != "2026-11-01" || !last.Outside {
		t.Fatalf("grid = %s … %s", first.Date, last.Date)
	}
	if d := v.Weeks[0].Days[6]; d.Date != "2026-10-04" || !d.Today || d.Outside || d.Col != 7 {
		t.Errorf("today = %+v", d)
	}
	if v.Month != "2026-10" || v.Prev != "2026-09" || v.Next != "2026-11" {
		t.Errorf("months = %s %s %s", v.Prev, v.Month, v.Next)
	}
	// March 2026 starts on Sunday: six weeks.
	if n := len(layoutCalendar(*date("2026-03-01"), nil, nil, nil, calNow).Weeks); n != 6 {
		t.Errorf("March weeks = %d, want 6", n)
	}
}

func TestLayoutCalendarSplitsAtWeekEdges(t *testing.T) {
	v := layoutCalendar(*date("2026-10-01"), []store.GanttCard{gc(1, "Long", "2026-10-02", "2026-10-07")}, map[int64]string{1: "#1971c2"}, nil, calNow)
	a, b := v.Weeks[0].Bars, v.Weeks[1].Bars
	if len(a) != 1 || a[0].Col != 5 || a[0].Span != 3 || a[0].Before || !a[0].After || a[0].Color != "#1971c2" {
		t.Fatalf("first week = %+v", a)
	}
	if len(b) != 1 || b[0].Col != 1 || b[0].Span != 3 || !b[0].Before || b[0].After {
		t.Fatalf("second week = %+v", b)
	}
	// A card from the previous month shows in the grid's first week.
	v = layoutCalendar(*date("2026-10-01"), []store.GanttCard{gc(2, "Sept", "", "2026-09-29")}, nil, nil, calNow)
	if bars := v.Weeks[0].Bars; len(bars) != 1 || bars[0].Col != 2 || bars[0].Span != 1 {
		t.Fatalf("outside card = %+v", bars)
	}
}

func TestLayoutCalendarLanesAndOverflow(t *testing.T) {
	cards := []store.GanttCard{
		gc(1, "A", "2026-10-05", "2026-10-07"),
		gc(2, "B", "", "2026-10-06"),
		gc(3, "C", "2026-10-08", ""), // start only: one day
		gc(4, "D", "", "2026-10-06"),
		gc(5, "E", "", "2026-10-06"),
		gc(6, "F", "", ""), // no dates: not on the calendar
	}
	v := layoutCalendar(*date("2026-10-01"), cards, nil, nil, calNow)
	w := v.Weeks[1] // 5–11 October
	lane := map[int64]int{}
	for _, b := range w.Bars {
		lane[b.CardID] = b.Lane
	}
	// A is longest and earliest: lane 1; C fits beside it on the 8th.
	if lane[1] != 1 || lane[3] != 1 || lane[2] != 2 || lane[4] != 3 {
		t.Fatalf("lanes = %v", lane)
	}
	if _, shown := lane[5]; shown {
		t.Error("the fourth card on the 6th is drawn")
	}
	tue := w.Days[1] // 6 October
	if tue.More != 1 || len(tue.Cards) != 4 {
		t.Fatalf("6 Oct: more %d, cards %d", tue.More, len(tue.Cards))
	}
	for _, wk := range v.Weeks {
		for _, b := range wk.Bars {
			if b.CardID == 6 {
				t.Error("an undated card is on the calendar")
			}
		}
	}
}

func TestLayoutCalendarMarksLateDoneAndDimmed(t *testing.T) {
	done := gc(2, "Done", "", "2026-10-01")
	done.Card.CompletedAt = date("2026-10-02")
	cards := []store.GanttCard{gc(1, "Late", "", "2026-10-02"), done, gc(3, "Future", "", "2026-10-20")}
	v := layoutCalendar(*date("2026-10-01"), cards, nil, map[int64]bool{1: true, 2: true}, calNow)
	got := map[int64]calendarBar{}
	for _, w := range v.Weeks {
		for _, b := range w.Bars {
			got[b.CardID] = b
		}
	}
	if !got[1].Late || got[1].Done || got[1].Dimmed {
		t.Errorf("late card = %+v", got[1])
	}
	if got[2].Late || !got[2].Done {
		t.Errorf("done card = %+v", got[2])
	}
	if got[3].Late || !got[3].Dimmed || got[3].Due != "2026-10-20" {
		t.Errorf("filtered-out card = %+v", got[3])
	}
}
```

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run 'Calendar(Month|Grid)|LayoutCalendar' -count=1`. Beklenen: `undefined: calendarMonth`.

- [ ] **Step 3: Uygula** — `internal/web/calendar_layout.go`, yukarıdaki tiplerin tamamı ve:

```go
package web

import (
	"slices"
	"time"

	"kanban/internal/store"
)

// calendarLanes is how many cards a day shows before "+k more".
const calendarLanes = 3

// (types from the Interfaces block above)

// calendarMonth is the month ?month=YYYY-MM names; anything else is now's.
func calendarMonth(raw string, now time.Time) time.Time {
	if t, err := time.Parse("2006-01", raw); err == nil {
		return t
	}
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// layoutCalendar lays out month's weeks, Monday to Sunday, with the days of
// the neighbouring months that fill them, and the cards on their days.
func layoutCalendar(month time.Time, cards []store.GanttCard, colors map[int64]string, matching map[int64]bool, now time.Time) calendarView {
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	today := day(now)
	first := month.AddDate(0, 0, -((int(month.Weekday()) + 6) % 7)) // back to Monday
	last := month.AddDate(0, 1, -1)
	last = last.AddDate(0, 0, (7-int(last.Weekday()))%7) // on to Sunday
	v := calendarView{
		Month: month.Format("2006-01"),
		Prev:  month.AddDate(0, -1, 0).Format("2006-01"),
		Next:  month.AddDate(0, 1, 0).Format("2006-01"),
	}
	for ws := first; !ws.After(last); ws = ws.AddDate(0, 0, 7) {
		v.Weeks = append(v.Weeks, layoutWeek(ws, month.Month(), cards, colors, matching, today))
	}
	return v
}

// layoutWeek cuts the week's part out of every card that touches it and gives
// each part a lane: earlier first, then longer, into the first lane free.
func layoutWeek(ws time.Time, month time.Month, cards []store.GanttCard, colors map[int64]string, matching map[int64]bool, today time.Time) calendarWeek {
	we := ws.AddDate(0, 0, 6)
	var w calendarWeek
	for i := range 7 {
		d := ws.AddDate(0, 0, i)
		w.Days = append(w.Days, calendarDay{Date: d.Format(time.DateOnly), Num: d.Day(), Col: i + 1,
			Outside: d.Month() != month, Today: d.Equal(today)})
	}
	var bars []calendarBar
	for _, g := range cards {
		s, e, ok := span(g.Card)
		if !ok {
			continue
		}
		s, e = day(s), day(e)
		if e.Before(ws) || s.After(we) {
			continue
		}
		from, to := s, e
		if from.Before(ws) {
			from = ws
		}
		if to.After(we) {
			to = we
		}
		c := g.Card
		done := c.CompletedAt != nil
		b := calendarBar{CardID: c.ID, Version: c.Version, Title: c.Title, Color: colors[c.ColumnID],
			Col: daysBetween(ws, from) + 1, Span: daysBetween(from, to) + 1,
			Before: s.Before(ws), After: e.After(we), Done: done,
			Late:   !done && c.DueDate != nil && day(*c.DueDate).Before(today),
			Dimmed: matching != nil && !matching[c.ID]}
		if c.StartDate != nil {
			b.Start = c.StartDate.Format(time.DateOnly)
		}
		if c.DueDate != nil {
			b.Due = c.DueDate.Format(time.DateOnly)
		}
		bars = append(bars, b)
	}
	slices.SortStableFunc(bars, func(a, b calendarBar) int {
		if a.Col != b.Col {
			return a.Col - b.Col
		}
		return b.Span - a.Span
	})
	var laneEnd []int // the last day taken in each lane
	for _, b := range bars {
		lane := 0
		for lane < len(laneEnd) && laneEnd[lane] >= b.Col {
			lane++
		}
		if lane == len(laneEnd) {
			laneEnd = append(laneEnd, 0)
		}
		laneEnd[lane] = b.Col + b.Span - 1
		b.Lane = lane + 1
		for i := b.Col - 1; i < b.Col-1+b.Span; i++ {
			w.Days[i].Cards = append(w.Days[i].Cards, calendarCardRef{CardID: b.CardID, Title: b.Title, Done: b.Done, Late: b.Late})
			if b.Lane > calendarLanes {
				w.Days[i].More++
			}
		}
		if b.Lane <= calendarLanes {
			w.Bars = append(w.Bars, b)
		}
	}
	return w
}
```

- [ ] **Step 4: Çalıştır, geçtiğini gör.** `go test ./internal/web -run 'Calendar(Month|Grid)|LayoutCalendar' -count=1 -v`. Beklenen: 5 test PASS. Bir şerit beklentisi tutmazsa testteki sıralama varsayımını (önce erken, sonra uzun) koda göre değil, spec §3'e göre düzelt.

- [ ] **Step 5: Commit**

```bash
git add internal/web/calendar_layout.go internal/web/calendar_layout_test.go
git commit -m "feat(calendar): lay a month out in weeks, cards in lanes"
```

---

### Task 3: Takvim sayfası, sekme, şablonlar, stil

**Files:**
- Create: `internal/web/pages_calendar.go`, `templates/pages/board_calendar.html`, `templates/fragments/calendar.html`, `static/css/calendar.css`
- Modify: `internal/web/pages_gantt.go:168-197`, `internal/web/app.go:61,94,186`, `templates/partials/board_tabs.html:6`, `templates/partials/icons.html`, `templates/layouts/base.html:19`, `locales/tr.json`, `locales/en.json`
- Test: `internal/web/calendar_page_test.go`

**Interfaces:**
- Consumes: `layoutCalendar`, `calendarMonth`, `calendarView` (Görev 2); `h.boardFor`, `h.boardFilterFor`, `h.urlIn`, `boardColor`, `boardTag`, `boardTabs`.
- Produces: sayfa adı `board-calendar` (yol `/boards/{id}/calendar`), parça adı `board-calendar-grid` (yol `/boards/{id}/calendar/grid`), `h.calendarGrid *collage.Fragment`, `func (h *handlers) boardMatching(ctx context.Context, rc *collage.RenderContext, bc boardContext) (map[int64]bool, error)`, şablon fonksiyonu `monthLink(self string, f boardFilter, month string, done bool) template.URL`. Görev 4, `boardCalendarPage`'deki `WithAction` satırını ekler; bu görevde eylem yok.

- [ ] **Step 1: Başarısız testleri yaz** — `internal/web/calendar_page_test.go`:

```go
package web_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTheCalendarView(t *testing.T) {
	b := newBoardSetup(t)
	long := b.card(t, 0, "Design")
	pin := b.card(t, 1, "Launch")
	b.card(t, 0, "Someday")
	if res := b.member.SubmitFetch(b.cardPath(long), b.cardPath(long), datesForm(long.Version, "2026-10-05", "2026-10-09")); res.Status != http.StatusNoContent {
		t.Fatalf("set_dates = %d", res.Status)
	}
	if res := b.member.SubmitFetch(b.cardPath(pin), b.cardPath(pin), fieldForm(pin, "due_date", "2026-10-08")); res.Status != http.StatusOK {
		t.Fatalf("due date = %d", res.Status)
	}
	mustContain(t, b.member.Get(b.path).Body, `href="`+b.path+`/calendar"`)
	page := b.member.Get(b.path + "/calendar?month=2026-10")
	if page.Status != http.StatusOK {
		t.Fatalf("calendar = %d", page.Status)
	}
	mustContain(t, page.Body,
		"Ekim 2026", "Pzt", `data-date="2026-10-05"`,
		`data-card="`+id(long.ID)+`"`, `data-start="2026-10-05" data-due="2026-10-09"`,
		`data-card="`+id(pin.ID)+`"`,
		`href="`+b.path+`/calendar?month=2026-09"`, `href="`+b.path+`/calendar?month=2026-11"`,
		`data-collage-fragment="`+b.path+`/calendar/grid?month=2026-10"`)
	if strings.Contains(page.Body, "Someday") {
		t.Error("an undated card is on the calendar")
	}
	if strings.Contains(page.Body, `style="`) {
		t.Error("inline style: the CSP blocks it")
	}
	if frag := b.member.Get(b.path + "/calendar/grid?month=2026-10"); frag.Status != http.StatusOK || strings.Contains(frag.Body, "<html") {
		t.Fatalf("grid fragment = %d", frag.Status)
	}
	out := b.h.signedIn("out", "out@example.com")
	if res := out.Get(b.path + "/calendar"); res.Status != http.StatusNotFound {
		t.Errorf("outsider calendar = %d", res.Status)
	}
}

func TestCalendarFallsBackToThisMonth(t *testing.T) {
	b := newBoardSetup(t)
	this := time.Now().Format("2006-01")
	for _, q := range []string{"", "?month=2026-13", "?month=abc"} {
		res := b.member.Get(b.path + "/calendar" + q)
		if res.Status != http.StatusOK {
			t.Fatalf("calendar%s = %d", q, res.Status)
		}
		mustContain(t, res.Body, `/calendar/grid?month=`+this)
	}
}

func TestCalendarShowsDoneCardsOnRequest(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 2, "Shipped") // the third column is the done column
	if res := b.member.SubmitFetch(b.cardPath(c), b.cardPath(c), fieldForm(c, "due_date", "2026-10-08")); res.Status != http.StatusOK {
		t.Fatalf("due date = %d", res.Status)
	}
	if strings.Contains(b.member.Get(b.path+"/calendar?month=2026-10").Body, "Shipped") {
		t.Error("a done card shows without done=1")
	}
	mustContain(t, b.member.Get(b.path+"/calendar?month=2026-10&done=1").Body, "Shipped", "is-done")
}

func TestCalendarKeepsTheFilterAcrossMonths(t *testing.T) {
	b := newBoardSetup(t)
	page := b.member.Get(b.path + "/calendar?month=2026-10&q=design").Body
	mustContain(t, page, `/calendar?month=2026-11&amp;q=design"`, `name="month" value="2026-10"`)
}
```

Bilinenler: `CreateBoard`'un son kolonu (`Done`) bitti kolonudur (`internal/store/board_plan.go:59`), oraya açılan kart baştan tamamlanmıştır. Filtrenin metin anahtarı `q`'dur (`internal/web/filter.go:100`). `url.Values.Encode()` anahtarları alfabetik sıralar (`month` < `q`).

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run 'Calendar(View|FallsBack|ShowsDone|KeepsTheFilter)' -race -count=1`. Beklenen: `/calendar` 404 → FAIL.

- [ ] **Step 3: Filtre eşleşmesini paylaş** — `internal/web/pages_gantt.go`'da `loadGantt`'ın filtre kısmını yeni yardımcıya taşı:

```go
// boardMatching is the cards the board filter lets through, nil when the
// filter is empty.
func (h *handlers) boardMatching(ctx context.Context, rc *collage.RenderContext, bc boardContext) (map[int64]bool, error) {
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return nil, err
	}
	return h.store.MatchingCardIDs(ctx, bc.Board.ID, fv.Filter.Store(bc.User.ID, time.Now().In(h.loc)))
}
```

ve `loadGantt`'ta:

```go
	matching, err := h.boardMatching(ctx, rc, bc)
	if err != nil {
		return ganttView{}, tags, err
	}
	now := time.Now().In(h.loc)
	return layoutGantt(rc, bc, scale, group, cols, cards, deps, now, matching), tags, nil
```

`go test ./internal/web -run Gantt -race -count=1` → PASS (davranış değişmedi).

- [ ] **Step 4: Sayfa kodu** — `internal/web/pages_calendar.go`:

```go
package web

import (
	"context"
	"html/template"
	"net/url"
	"strconv"
	"time"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/config"
	"kanban/internal/store"
)

type calendarPageView struct {
	Board  store.Board
	Month  string // YYYY-MM
	Prev   string
	Next   string
	This   string // today's month
	Title  string
	Done   bool
	Query  string // the grid's settings and filter, for its fragment URL
	Filter filterView
}
```

Devamı:

```go
func (h *handlers) boardCalendarPage() *collage.Page {
	h.calendarGrid = collage.NewFragment("board-calendar-grid", "fragments/calendar.html").
		WithDataHandler(collage.DataHandler(h.loadCalendar)).
		Required().
		Build()
	content := collage.NewFragment("board-calendar-content", "pages/board_calendar.html").
		WithDataHandler(collage.Load(h.loadCalendarPage)).
		WithSlotFragment("grid", h.calendarGrid).
		Required().
		Build()
	b := paths(h.privatePage("board-calendar", content), "/boards/{id}/calendar")
	for _, l := range config.Locales {
		b = b.WithFragmentPath(l, "/boards/{id}/calendar/grid", h.calendarGrid)
	}
	return b.Dynamic().Build()
}

// calendarSettings reads the month shown and whether done cards show.
func (h *handlers) calendarSettings(rc *collage.RenderContext) (time.Time, bool) {
	q := rc.Request.URL.Query()
	return calendarMonth(q.Get("month"), time.Now().In(h.loc)), q.Get("done") == "1"
}

// calendarTitle is "Ekim 2026" in the reader's language.
func calendarTitle(rc *collage.RenderContext, month time.Time) string {
	return i18n.T(rc, "calendar_view.months."+strconv.Itoa(int(month.Month()))) + " " + strconv.Itoa(month.Year())
}

func (h *handlers) loadCalendarPage(ctx context.Context, rc *collage.RenderContext) (calendarPageView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return calendarPageView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "calendar_view.title") + " · " + bc.Board.Name)
	month, done := h.calendarSettings(rc)
	v := calendarPageView{Board: bc.Board, Done: done, Month: month.Format("2006-01"),
		Prev: month.AddDate(0, -1, 0).Format("2006-01"), Next: month.AddDate(0, 1, 0).Format("2006-01"),
		This: time.Now().In(h.loc).Format("2006-01"), Title: calendarTitle(rc, month)}
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return calendarPageView{}, err
	}
	// The bar keeps the month as a hidden field, so a new filter stays on it.
	fv.Extra = url.Values{"month": {v.Month}}
	if done {
		fv.Extra.Set("done", "1")
	}
	fv.Action, err = h.urlIn("board-calendar", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
	if err != nil {
		return calendarPageView{}, err
	}
	q := fv.Filter.Values()
	for k, vals := range fv.Extra {
		q[k] = vals
	}
	v.Query = q.Encode()
	v.Filter = fv
	return v, nil
}

func (h *handlers) loadCalendar(ctx context.Context, rc *collage.RenderContext) (calendarView, []string, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return calendarView{}, nil, err
	}
	tags := []string{boardTag(bc.Board.ID)}
	month, done := h.calendarSettings(rc)
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return calendarView{}, tags, err
	}
	colors := map[int64]string{}
	for i, c := range cols {
		colors[c.ID] = boardColor(i)
	}
	cards, err := h.store.GanttCards(ctx, bc.Board.ID, done)
	if err != nil {
		return calendarView{}, tags, err
	}
	matching, err := h.boardMatching(ctx, rc, bc)
	if err != nil {
		return calendarView{}, tags, err
	}
	v := layoutCalendar(month, cards, colors, matching, time.Now().In(h.loc))
	v.BoardID, v.CanEdit, v.Title = bc.Board.ID, bc.Access.CanEdit && len(cols) > 0, calendarTitle(rc, month)
	for i := range 7 {
		v.Weekdays = append(v.Weekdays, i18n.T(rc, "weekdays."+strconv.Itoa(i)))
	}
	if v.Action, err = h.urlIn("board-calendar", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)}); err != nil {
		return calendarView{}, tags, err
	}
	// The form posts back to the month and filter it was drawn with.
	v.Action = withQuery(v.Action, rc.Request.URL.RawQuery)
	v.Notices, _ = collage.Get[[]string](rc, noticeKey)
	return v, tags, nil
}

// monthLink is the calendar at self for month, with the filter kept. It is
// the whole URL: a query written after "?" in a template gets its "=" and "&"
// escaped.
func monthLink(self string, f boardFilter, month string, done bool) template.URL {
	v := f.Values()
	v.Set("month", month)
	if done {
		v.Set("done", "1")
	}
	return template.URL(self + "?" + v.Encode())
}
```

Renk Gantt'la aynıdır: kolonun sırasına göre `boardColor(i)` (`pages_gantt.go:304`). `withQuery(path, query string) string` `internal/web/filter.go:162`'de; sorgu boşsa yolu olduğu gibi döndürür.

- [ ] **Step 5: Kayıt** — `internal/web/app.go`:
  - satır 61 yanına: `calendarGrid *collage.Fragment`
  - satır 94 yanına: `"monthLink":       monthLink,`
  - satır 186 listesinde `h.boardGanttPage(),` ardından `h.boardCalendarPage(),`

`templates/layouts/base.html:19` ardından:

```html
  <link rel="stylesheet" href="{{asset "/static/css/calendar.css"}}">
```

- [ ] **Step 6: Sekme ve ikon** — `templates/partials/board_tabs.html` Gantt satırını ve ardına takvimi:

```html
  <a class="tabs__tab{{if eq $on "gantt"}} is-active{{end}}" href="{{pageURL "board-gantt" "id" $id}}"{{if eq $on "gantt"}} aria-current="page"{{end}}>{{template "icon" "gantt"}} {{t "gantt.button"}}</a>
  <a class="tabs__tab{{if eq $on "calendar"}} is-active{{end}}" href="{{pageURL "board-calendar" "id" $id}}"{{if eq $on "calendar"}} aria-current="page"{{end}}>{{template "icon" "calendar"}} {{t "calendar_view.tab"}}</a>
```

`templates/partials/icons.html`'e `calendar` dalının yanına:

```html
{{- else if eq . "gantt"}}<path d="M4 6h9M8 12h10M6 18h7"/>
```

`gantt` ikonunu başka yerde (`grep -rn '"calendar"' templates`) Gantt'ı göstermek için `calendar` kullanan yer varsa onu da `gantt`'a çevir; iCal ile ilgili olanlara dokunma.

- [ ] **Step 7: Çeviriler** — `locales/tr.json` ve `locales/en.json` köküne `calendar_view` ekle (anahtar sırası dosyadaki `gantt`'ın hemen ardına):

```json
"calendar_view": {
  "title": "Takvim",
  "tab": "Takvim",
  "nav": "Ay",
  "prev": "Önceki ay",
  "next": "Sonraki ay",
  "today": "Bugün",
  "more": "+{n} daha",
  "day_cards": "{date} günündeki kartlar",
  "add": "Bu güne kart ekle",
  "new_card": "Kart başlığı",
  "save_failed": "Kaydedilemedi.",
  "months": {"1": "Ocak", "2": "Şubat", "3": "Mart", "4": "Nisan", "5": "Mayıs", "6": "Haziran", "7": "Temmuz", "8": "Ağustos", "9": "Eylül", "10": "Ekim", "11": "Kasım", "12": "Aralık"}
}
```

```json
"calendar_view": {
  "title": "Calendar",
  "tab": "Calendar",
  "nav": "Month",
  "prev": "Previous month",
  "next": "Next month",
  "today": "Today",
  "more": "+{n} more",
  "day_cards": "Cards on {date}",
  "add": "Add a card on this day",
  "new_card": "Card title",
  "save_failed": "Could not save.",
  "months": {"1": "January", "2": "February", "3": "March", "4": "April", "5": "May", "6": "June", "7": "July", "8": "August", "9": "September", "10": "October", "11": "November", "12": "December"}
}
```

Yer tutucular `{ad}` biçimindedir ve şablonda `{{t "anahtar" "ad" "değer"}}` ile, değer **dizgi** olarak geçirilir (bkz. `templates/fragments/columns.html:2`). JSON'u `python3 -m json.tool locales/tr.json >/dev/null` ile doğrula.

- [ ] **Step 8: Sayfa şablonu** — `templates/pages/board_calendar.html`:

```html
{{$self := pageURL "board-calendar" "id" .Board.ID}}
<header class="page-head">
  <div>
    <a class="crumb" href="{{pageURL "board" "id" .Board.ID}}">{{template "icon" "arrow-left"}} {{.Board.Name}}</a>
    <h1>{{.Title}}</h1>
  </div>
  <nav class="gantt-tools" aria-label="{{t "calendar_view.nav"}}">
    <div class="segmented" role="group" aria-label="{{t "calendar_view.nav"}}">
      <a class="segmented__item" href="{{monthLink $self .Filter.Filter .Prev .Done}}" aria-label="{{t "calendar_view.prev"}}">{{template "icon" "arrow-left"}}</a>
      <a class="segmented__item{{if eq .Month .This}} is-active{{end}}" href="{{monthLink $self .Filter.Filter .This .Done}}">{{t "calendar_view.today"}}</a>
      <a class="segmented__item" href="{{monthLink $self .Filter.Filter .Next .Done}}" aria-label="{{t "calendar_view.next"}}">{{template "icon" "arrow-right"}}</a>
    </div>
    <a class="btn btn--quiet btn--sm{{if .Done}} is-on{{end}}" href="{{monthLink $self .Filter.Filter .Month (not .Done)}}" aria-pressed="{{if .Done}}true{{else}}false{{end}}">{{template "icon" "check"}} {{t "gantt.show_done"}}</a>
  </nav>
</header>
{{template "board-tabs" (boardTabs .Board.ID "calendar")}}
{{template "filter-bar" .Filter}}
<div class="cal" id="calendar" data-collage-fragment="{{fragmentURL "board-calendar" "board-calendar-grid" "id" .Board.ID}}?{{.Query}}" data-collage-push>
  {{slot "grid"}}
</div>
<script type="module" src="{{asset "/static/js/filter.js"}}"></script>
```

`icons.html`'de `arrow-right` yok; `arrow-left`'in yanına ekle: `{{- else if eq . "arrow-right"}}<path d="M5 12h14M13 6l6 6-6 6"/>`. `calendar.js`'in `<script>` satırı Görev 5'te eklenir (dosya o zaman var olur). "Bitenleri göster" ve ay bağlantıları tam sayfa gezinmesidir (Gantt'taki gibi); başlık ayla değiştiği için parça değil sayfa yenilenir.

- [ ] **Step 9: Izgara şablonu** — `templates/fragments/calendar.html`:

```html
{{$board := .BoardID}}{{$v := .}}
{{if .Notices}}<div class="alert board-alert" role="alert" data-cal-alert>{{template "icon" "flag"}}<ul>{{range .Notices}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
<div class="cal__frame" data-cal {{if .CanEdit}}data-editable{{end}}
  data-error-conflict="{{t "board.conflict"}}" data-error-order="{{t "card.date_order"}}" data-error="{{t "calendar_view.save_failed"}}">
  <ol class="cal__weekdays" aria-hidden="true">{{range .Weekdays}}<li>{{.}}</li>{{end}}</ol>
  {{range .Weeks}}
  <div class="cal__week">
    {{range .Days}}
    <div class="cal__day c{{.Col}}{{if .Outside}} is-outside{{end}}{{if .Today}} is-today{{end}}" data-date="{{.Date}}">
      <span class="cal__num">{{.Num}}</span>
      <div class="cal__foot">
        {{if .More}}
        <details class="cal__more">
          <summary>{{t "calendar_view.more" "n" (printf "%d" .More)}}</summary>
          <ul class="cal__list" aria-label="{{t "calendar_view.day_cards" "date" .Date}}">
            {{range .Cards}}<li><a class="{{if .Done}}is-done{{end}}{{if .Late}} is-late{{end}}" href="{{pageURL "card" "id" $board "card" .CardID}}">{{.Title}}</a></li>{{end}}
          </ul>
        </details>
        {{end}}
        {{if $v.CanEdit}}
        <details class="cal__add" data-cal-add>
          <summary aria-label="{{t "calendar_view.add"}}">{{template "icon" "plus"}}</summary>
          <form class="cal__form" method="post" action="{{$v.Action}}">
            {{csrfToken}}
            <input type="hidden" name="op" value="create_card">
            <input type="hidden" name="due" value="{{.Date}}">
            <input class="input input--sm" name="title" maxlength="200" required placeholder="{{t "calendar_view.new_card"}}" aria-label="{{t "calendar_view.new_card"}}">
          </form>
        </details>
        {{end}}
      </div>
    </div>
    {{end}}
    {{range .Bars}}
    <a class="cal__bar c{{.Col}} s{{.Span}} l{{.Lane}}{{if .Before}} is-before{{end}}{{if .After}} is-after{{end}}{{if .Done}} is-done{{end}}{{if .Late}} is-late{{end}}{{if .Dimmed}} is-dimmed{{end}}"
      href="{{pageURL "card" "id" $board "card" .CardID}}" data-color="{{.Color}}" title="{{.Title}}"
      data-card="{{.CardID}}" data-version="{{.Version}}" data-start="{{.Start}}" data-due="{{.Due}}" data-url="{{pageURL "card" "id" $board "card" .CardID}}">{{.Title}}</a>
    {{end}}
  </div>
  {{end}}
</div>
```

(`input--sm` `static/css/components.css`'te tanımlı.)

- [ ] **Step 10: Stil** — `static/css/calendar.css`:

```css
/* Board calendar: a month of weeks; each week is a grid of seven days with
   the cards' bars laid over them in lanes. Bars are placed by class, not by
   style attributes, which the CSP does not allow. */

.cal { display: grid; gap: var(--s4); }
.cal__frame { border: 0.0625rem solid var(--line); border-radius: var(--r-lg); background: var(--sheet); overflow-x: auto; box-shadow: var(--shadow-paper); }
.cal__weekdays, .cal__week { display: grid; grid-template-columns: repeat(7, minmax(6.5rem, 1fr)); min-width: 45.5rem; }
.cal__weekdays { list-style: none; margin: 0; padding: 0; background: var(--shelf); border-bottom: 0.0625rem solid var(--line); }
.cal__weekdays li { padding: var(--s2) var(--s3); font-size: var(--text-xs); font-weight: 700; color: var(--ink-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.cal__week { grid-template-rows: 1.75rem repeat(3, 1.625rem) minmax(1.75rem, auto); border-bottom: 0.0625rem solid var(--line); }
.cal__week:last-child { border-bottom: 0; }

.cal__day { grid-row: 1 / -1; display: flex; flex-direction: column; justify-content: space-between; padding: var(--s1) var(--s2); border-right: 0.0625rem solid var(--line); min-height: 7.5rem; }
.cal__day.c7 { border-right: 0; }
.cal__day.is-outside { background: var(--shelf); }
.cal__day.is-outside .cal__num { color: var(--ink-faint); }
.cal__day.is-drop { background: var(--accent-soft); outline: 0.125rem dashed var(--accent); outline-offset: -0.125rem; }
.cal__num { font-size: var(--text-sm); font-weight: 700; color: var(--ink-soft); font-variant-numeric: tabular-nums; }
.cal__day.is-today .cal__num { display: inline-grid; place-items: center; width: 1.5rem; height: 1.5rem; border-radius: 50%; background: var(--accent); color: var(--sheet); }
.cal__foot { display: flex; align-items: flex-end; justify-content: space-between; gap: var(--s1); }

.cal__bar {
  grid-row: 2; z-index: 1; margin: 0.125rem 0.25rem; padding: 0 var(--s2); height: 1.375rem; display: flex; align-items: center;
  border-radius: 0.375rem; background: var(--chip-bg, var(--shelf)); color: var(--chip-ink, var(--ink));
  border-left: 0.1875rem solid var(--chip-dot, var(--line-strong));
  font-size: var(--text-xs); font-weight: 700; text-decoration: none; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  cursor: pointer; touch-action: none;
}
.cal__bar:hover { filter: brightness(0.97); }
.cal__bar:focus-visible { outline: 0.125rem solid var(--accent); outline-offset: 0.0625rem; }
.cal__bar.is-before { border-top-left-radius: 0; border-bottom-left-radius: 0; border-left-style: dotted; margin-left: 0; }
.cal__bar.is-after { border-top-right-radius: 0; border-bottom-right-radius: 0; margin-right: 0; }
.cal__bar.is-done { color: var(--ink-muted); text-decoration: line-through; }
.cal__bar.is-late { box-shadow: inset 0 0 0 0.0625rem var(--danger); }
.cal__bar.is-dimmed { opacity: 0.35; }
.cal__bar.is-dragging { opacity: 0.6; pointer-events: none; }
.cal__bar.is-saving { opacity: 0.6; }

.cal__bar.c1 { grid-column-start: 1; } .cal__bar.c2 { grid-column-start: 2; } .cal__bar.c3 { grid-column-start: 3; }
.cal__bar.c4 { grid-column-start: 4; } .cal__bar.c5 { grid-column-start: 5; } .cal__bar.c6 { grid-column-start: 6; }
.cal__bar.c7 { grid-column-start: 7; }
.cal__day.c1 { grid-column: 1; } .cal__day.c2 { grid-column: 2; } .cal__day.c3 { grid-column: 3; } .cal__day.c4 { grid-column: 4; }
.cal__day.c5 { grid-column: 5; } .cal__day.c6 { grid-column: 6; } .cal__day.c7 { grid-column: 7; }
.cal__bar.s1 { grid-column-end: span 1; } .cal__bar.s2 { grid-column-end: span 2; } .cal__bar.s3 { grid-column-end: span 3; }
.cal__bar.s4 { grid-column-end: span 4; } .cal__bar.s5 { grid-column-end: span 5; } .cal__bar.s6 { grid-column-end: span 6; }
.cal__bar.s7 { grid-column-end: span 7; }
.cal__bar.l1 { grid-row: 2; } .cal__bar.l2 { grid-row: 3; } .cal__bar.l3 { grid-row: 4; }

.cal__more { position: relative; font-size: var(--text-xs); }
.cal__more summary { list-style: none; cursor: pointer; color: var(--ink-muted); font-weight: 700; }
.cal__more summary::-webkit-details-marker { display: none; }
.cal__list, .cal__form {
  position: absolute; z-index: 3; left: 0; top: 100%; min-width: 12rem; margin: var(--s1) 0 0; padding: var(--s2);
  list-style: none; background: var(--sheet); border: 0.0625rem solid var(--line); border-radius: var(--r-sm); box-shadow: var(--shadow-paper);
}
.cal__list li + li { margin-top: var(--s1); }
.cal__list a { color: var(--ink); text-decoration: none; font-weight: 700; }
.cal__list a.is-done { color: var(--ink-muted); text-decoration: line-through; }
.cal__list a.is-late { color: var(--danger); }
.cal__add { position: relative; margin-left: auto; }
.cal__add summary { list-style: none; cursor: pointer; display: grid; place-items: center; width: 1.5rem; height: 1.5rem; border-radius: var(--r-sm); color: var(--ink-faint); opacity: 0; }
.cal__add summary::-webkit-details-marker { display: none; }
.cal__day:hover .cal__add summary, .cal__add summary:focus-visible, .cal__add[open] summary { opacity: 1; }
.cal__add summary:hover { background: var(--shelf); color: var(--ink); }
.cal__form { left: auto; right: 0; }
.cal__day.c1 .cal__form, .cal__day.c2 .cal__form { left: 0; right: auto; }
/* The frame scrolls sideways, which clips what leaves it: near the bottom the
   lists open upward instead. */
.cal__week:nth-last-child(-n+2) .cal__list, .cal__week:nth-last-child(-n+2) .cal__form { top: auto; bottom: 100%; margin: 0 0 var(--s1); }
```

Kullanılan bütün tema değişkenleri `static/css/tokens.css`'te tanımlı (kontrol edildi). Çubuk renkleri `tokens.css:151`'deki `[data-color]` seçicilerinden (`--chip-bg/--chip-ink/--chip-dot`) gelir.

- [ ] **Step 11: Çalıştır, geçtiğini gör.** `go test ./internal/web -race -count=1`. Beklenen: PASS, Gantt testleri dahil.

- [ ] **Step 12: Commit**

```bash
git add internal/web/pages_calendar.go internal/web/calendar_page_test.go internal/web/pages_gantt.go internal/web/app.go \
  templates/pages/board_calendar.html templates/fragments/calendar.html templates/partials/board_tabs.html templates/partials/icons.html \
  templates/layouts/base.html static/css/calendar.css locales/tr.json locales/en.json
git commit -m "feat(calendar): a month view of the board beside the Gantt chart"
```

(Bu görevden sonra takvim yalnız okunur ve tam sayfa formla kart açar; sürükleme Görev 5'te gelir.)

---

### Task 4: Takvimden kart açma — `create_card`

**Files:**
- Modify: `internal/web/pages_calendar.go`
- Test: `internal/web/calendar_page_test.go`

**Interfaces:**
- Consumes: `store.CreateCardDue` (Görev 1); `creatableColumns`, `violationMessages`, `isFetch`, `badText`, `noticeKey`, `h.notifyAssigned`, `validate.Form`, `validate.Refuse` (`internal/web/pages_board.go`).
- Produces: `POST /boards/{id}/calendar` `op=create_card`, `title`, `due=YYYY-MM-DD`. Fetch'te başarı: 200 + ızgara parçası; kural ihlali: 422 + uyarılı ızgara parçası; boş başlık: 422; bozuk tarih: 400; yetki yok: 403; board yok/dışarıdan: 404 (`webtest.Submit*` önce sayfayı GET eder ve 200 değilse testi durdurur; dışarıdakinin 404'ü Görev 3'te GET ile sınanır). Fetch değilse başarıda aynı ay ve filtreyle sayfaya 303.

- [ ] **Step 1: Başarısız testleri yaz** — `internal/web/calendar_page_test.go`'ya ekle (gerekirse `context`, `net/url`, `kanban/internal/rules`, `kanban/internal/store` importları):

```go
func calendarCreate(title, due string) url.Values {
	return url.Values{"op": {"create_card"}, "title": {title}, "due": {due}}
}

func TestCreatingACardFromTheCalendar(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	page := b.path + "/calendar?month=2026-10"
	res := b.member.SubmitFetch(page, page, calendarCreate("Kickoff", "2026-10-14"))
	if res.Status != http.StatusOK {
		t.Fatalf("create = %d:\n%s", res.Status, res.Body)
	}
	mustContain(t, res.Body, "Kickoff", `data-due="2026-10-14"`)
	cards, err := b.h.store.GanttCards(ctx, b.board.ID, false)
	if err != nil || len(cards) != 1 {
		t.Fatalf("cards = %v, %v", cards, err)
	}
	if c := cards[0].Card; c.ColumnID != b.cols[0].ID || c.DueDate == nil || c.DueDate.Format(time.DateOnly) != "2026-10-14" {
		t.Fatalf("card = %+v", c)
	}
	// Without a script: back to the same month.
	res = b.member.Submit(page, page, calendarCreate("Plain", "2026-10-15"))
	if res.Status != http.StatusSeeOther || !strings.Contains(res.Location(), "month=2026-10") {
		t.Fatalf("form create = %d %q", res.Status, res.Location())
	}
}

func TestCalendarCreateRefusals(t *testing.T) {
	b := newBoardSetup(t)
	page := b.path + "/calendar?month=2026-10"
	if res := b.member.SubmitFetch(page, page, calendarCreate("  ", "2026-10-14")); res.Status != http.StatusUnprocessableEntity {
		t.Errorf("empty title = %d, want 422", res.Status)
	}
	if res := b.member.SubmitFetch(page, page, calendarCreate("X", "14.10.2026")); res.Status != http.StatusBadRequest {
		t.Errorf("bad date = %d, want 400", res.Status)
	}
	if res := b.member.SubmitFetch(page, page, url.Values{"op": {"nope"}}); res.Status != http.StatusBadRequest {
		t.Errorf("unknown op = %d, want 400", res.Status)
	}
}

// The first column wants a due date: the calendar's card has one, so it enters.
func TestCalendarCreatePassesTheDueDateRule(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	if err := b.h.store.AddCondition(ctx, b.board.ID, store.ColumnCondition{ColumnID: b.cols[0].ID, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}); err != nil {
		t.Fatal(err)
	}
	page := b.path + "/calendar?month=2026-10"
	if res := b.member.SubmitFetch(page, page, calendarCreate("Dated", "2026-10-14")); res.Status != http.StatusOK {
		t.Fatalf("create = %d:\n%s", res.Status, res.Body)
	}
	// The column is now full: the next card is refused, with the reason on the grid.
	one := 1
	todo := b.cols[0]
	if err := b.h.store.UpdateColumn(ctx, b.board.ID, todo.ID, store.ColumnUpdate{Name: todo.Name, WIPLimit: &one, IsDone: todo.IsDone, AllowCreate: todo.AllowCreate, CountsPersonWIP: todo.CountsPersonWIP}); err != nil {
		t.Fatal(err)
	}
	res := b.member.SubmitFetch(page, page, calendarCreate("Over", "2026-10-15"))
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("over the limit = %d", res.Status)
	}
	mustContain(t, res.Body, "data-cal-alert")
}
```

`store.Column`'un alanları `ColumnUpdate`'inkilerle aynı adlardadır (`internal/store/columns.go:17-36`); güncelleme diğer ayarları korumak için hepsini geri yazar.

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run 'CalendarCreate|CreatingACardFromTheCalendar' -race -count=1`. Beklenen: POST 405 ya da 404 → FAIL.

- [ ] **Step 3: Uygula** — `boardCalendarPage`'in son satırı:

```go
	return b.WithAction(http.MethodPost, h.calendarPost).Dynamic().Build()
```

ve `pages_calendar.go`'ya (importlar: `errors`, `net/http`, `strings`, `validate` paketi — `pages_board.go`'daki import yolunu kopyala, `kanban/internal/store`):

```go
func (h *handlers) calendarPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	bc, err := h.boardFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if !bc.Access.CanEdit {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v := validate.Form(rc)
	if badText(rc) || v.Value("op") != "create_card" {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	return h.createCalendarCard(ctx, rc, v, bc)
}

// createCalendarCard makes a card due on the day it was asked for, in the
// column the board's own "add card" uses.
func (h *handlers) createCalendarCard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	due, err := time.Parse(time.DateOnly, v.Value("due"))
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	v.Field("title").Required().MaxLen(200)
	if !v.Valid() {
		res := validate.Refuse(rc, v, rc.Page)
		if isFetch(rc) {
			res.Page, res.Fragment = nil, h.calendarGrid
		}
		return res, nil
	}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return collage.NoContent(http.StatusConflict), nil
	}
	card, err := h.store.CreateCardDue(ctx, bc.Board.ID, creatableColumns(cols)[0].ID, strings.TrimSpace(v.Value("title")), due, bc.User.ID)
	if msgs := violationMessages(rc, err); msgs != nil {
		rc.Set(noticeKey, msgs)
		res := collage.RenderPage(rc.Page)
		if isFetch(rc) {
			res = collage.RenderFragment(h.calendarGrid)
		}
		res.Status = http.StatusUnprocessableEntity
		return res, nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusBadRequest), nil // the column was deleted meanwhile
	}
	if err != nil {
		return nil, err
	}
	h.notifyAssigned(ctx, bc, store.Card{}, card)
	if isFetch(rc) {
		res := collage.RenderFragment(h.calendarGrid)
		res.InvalidateTags = []string{boardTag(bc.Board.ID)}
		return res, nil
	}
	target, err := h.urlIn("board-calendar", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
	if err != nil {
		return nil, err
	}
	res := collage.SeeOther(withQuery(target, rc.Request.URL.RawQuery))
	res.InvalidateTags = []string{boardTag(bc.Board.ID)}
	return res, nil
}
```

Boş başlık fetch ile 422 döner (board'un aynı yolu için `internal/web/battle_test.go:123`). Eylem yanıtında parçanın yeni kartı görmesi için önbelleğin geçersiz kılınması gerekebilir; test `Kickoff`'u göremezse `RenderFragment`'tan önce `rc`'ye etiket geçersizliğinin nasıl uygulandığını `createCard` → `h.columns` yolunda incele ve aynısını yap.

- [ ] **Step 4: Çalıştır, geçtiğini gör.** `go test ./internal/web -race -count=1`. Beklenen: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/pages_calendar.go internal/web/calendar_page_test.go
git commit -m "feat(calendar): add a card on a day"
```

---

### Task 5: `calendar.js` — sürükleme, klavye, satır içi kart açma

**Files:**
- Create: `static/js/calendar.js`
- Modify: `templates/pages/board_calendar.html` (filter.js satırının üstüne `<script type="module" src="{{asset "/static/js/calendar.js"}}"></script>`)

**Interfaces:**
- Consumes: ızgara işaretlemesi (Görev 3): `#calendar`, `[data-cal]` (`data-editable`, `data-error-*`), `.cal__day[data-date]`, `.cal__bar[data-card][data-version][data-start][data-due][data-url]`, `details[data-cal-add]` içindeki `form`, `[data-cal-alert] li`; kartın `set_dates` işlemi (409 çakışma, 422 tarih sırası); `window.collageLive` (`pause`, `resume`, `refresh`), `[data-toasts]`.
- Produces: —

- [ ] **Step 1: Yaz** — `static/js/calendar.js`:

```js
// calendar.js: dates are changed on the calendar. Dragging a card to another
// day moves both its dates by as many days; the arrow keys on a focused card do
// the same, a day left or right, a week up or down. Each change is saved as the
// card's dates together, at the version the grid was drawn from. Clicking a
// day's empty space opens its new-card field.

const container = document.getElementById("calendar");

const live = () => window.collageLive;
const csrf = () => document.querySelector('input[name="_csrf"]')?.value ?? "";

// Dates are days: "YYYY-MM-DD" in, the same out.
const parse = (s) => (s ? new Date(s + "T00:00:00Z") : null);
const shift = (d, days) => (d ? new Date(d.getTime() + days * 86400000) : null);
const format = (d) => (d ? d.toISOString().slice(0, 10) : "");
const daysBetween = (a, b) => Math.round((parse(b) - parse(a)) / 86400000);

function toast(text, error) {
  const host = document.querySelector("[data-toasts]");
  if (!host) return;
  const p = document.createElement("p");
  p.className = "toast" + (error ? " toast--error" : "");
  p.textContent = text;
  host.append(p);
  setTimeout(() => {
    p.classList.add("is-leaving");
    setTimeout(() => p.remove(), 300);
  }, error ? 6000 : 2500);
}

const editable = () => container?.querySelector("[data-cal]")?.hasAttribute("data-editable");

// The day under a point, through the bars drawn over it.
const dayAt = (x, y) => document.elementsFromPoint(x, y).find((el) => el.matches?.(".cal__day[data-date]")) ?? null;

let focusAfter = null;

async function save(bar, days) {
  const frame = bar.closest("[data-cal]");
  const body = new FormData();
  body.set("op", "set_dates");
  body.set("start", format(shift(parse(bar.dataset.start), days)));
  body.set("due", format(shift(parse(bar.dataset.due), days)));
  body.set("expected_version", bar.dataset.version);
  body.set("_csrf", csrf());
  bar.classList.add("is-saving");
  focusAfter = bar.dataset.card;
  try {
    const res = await fetch(bar.dataset.url, {
      method: "POST",
      body,
      headers: { "Collage-Fetch": "1", "X-CSRF-Token": csrf() },
      credentials: "same-origin",
    });
    if (res.status === 409) toast(frame.dataset.errorConflict, true);
    else if (res.status === 422) toast(frame.dataset.errorOrder, true);
    else if (!res.ok) toast(frame.dataset.error, true);
  } catch {
    toast(frame.dataset.error, true);
  } finally {
    live()?.resume(container);
    // The grid as it now is: saved, or put back.
    live()?.refresh(container);
  }
}

// The card a save was about is focused again once the grid is redrawn.
container?.addEventListener("collage:swap", () => {
  if (!focusAfter) return;
  container.querySelector(`.cal__bar[data-card="${CSS.escape(focusAfter)}"]`)?.focus();
  focusAfter = null;
});

// Dragging: the card is picked up on the day under the pointer and put down on
// the day under it at the end; the move is the days between the two.
let drag = null;
let dragged = false;

const clearDrop = () => container?.querySelectorAll(".cal__day.is-drop").forEach((d) => d.classList.remove("is-drop"));

container?.addEventListener("pointerdown", (e) => {
  const bar = e.target instanceof Element && e.target.closest(".cal__bar");
  if (!bar || !editable() || e.button !== 0) return;
  const from = dayAt(e.clientX, e.clientY);
  if (!from) return;
  drag = { bar, from: from.dataset.date, x: e.clientX, y: e.clientY, id: e.pointerId, over: from };
  dragged = false;
});

container?.addEventListener("pointermove", (e) => {
  if (!drag || e.pointerId !== drag.id) return;
  if (!dragged && Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < 4) return;
  if (!dragged) {
    dragged = true;
    live()?.pause(container);
    drag.bar.classList.add("is-dragging");
    drag.bar.setPointerCapture?.(e.pointerId);
  }
  e.preventDefault();
  const over = dayAt(e.clientX, e.clientY);
  if (over && over !== drag.over) {
    clearDrop();
    over.classList.add("is-drop");
    drag.over = over;
  }
});

function endDrag(e) {
  if (!drag || e.pointerId !== drag.id) return;
  const d = drag;
  drag = null;
  if (!dragged) return;
  d.bar.classList.remove("is-dragging");
  clearDrop();
  const days = d.over ? daysBetween(d.from, d.over.dataset.date) : 0;
  if (days === 0 || e.type === "pointercancel") {
    live()?.resume(container);
    return;
  }
  save(d.bar, days);
}
container?.addEventListener("pointerup", endDrag);
container?.addEventListener("pointercancel", endDrag);

// A drag is not a click: the card's link opens it only when not dragged.
container?.addEventListener("click", (e) => {
  if (dragged && e.target instanceof Element && e.target.closest(".cal__bar")) {
    e.preventDefault();
    dragged = false;
  }
}, true);

// Keys: moves add up while they are pressed, and save a moment after.
const steps = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };
let pending = null;
container?.addEventListener("keydown", (e) => {
  const bar = e.target instanceof Element && e.target.closest(".cal__bar");
  if (!bar || !editable() || !(e.key in steps)) return;
  e.preventDefault();
  if (!pending || pending.bar !== bar) {
    if (pending) clearTimeout(pending.timer);
    else live()?.pause(container);
    pending = { bar, days: 0 };
  }
  pending.days += steps[e.key];
  // Where the card will land, shown on its day.
  clearDrop();
  const to = format(shift(parse(bar.dataset.due || bar.dataset.start), pending.days));
  container.querySelector(`.cal__day[data-date="${to}"]`)?.classList.add("is-drop");
  clearTimeout(pending.timer);
  const p = pending;
  p.timer = setTimeout(() => {
    pending = null;
    clearDrop();
    if (p.days === 0) {
      live()?.resume(container);
      return;
    }
    save(p.bar, p.days);
  }, 600);
});

// New cards: a click on a day's empty space opens its field; Escape closes it.
// The form is sent as it is, and the grid comes back with the card or the reason
// it was refused.
container?.addEventListener("click", (e) => {
  if (!(e.target instanceof Element) || !editable()) return;
  if (e.target.closest("a, summary, form, details, button")) return;
  const dayEl = e.target.closest(".cal__day");
  const add = dayEl?.querySelector("[data-cal-add]");
  if (!add) return;
  container.querySelectorAll("[data-cal-add][open]").forEach((d) => d !== add && d.removeAttribute("open"));
  add.setAttribute("open", "");
  add.querySelector('input[name="title"]')?.focus();
});

container?.addEventListener("toggle", (e) => {
  const add = e.target;
  if (add instanceof HTMLDetailsElement && add.matches("[data-cal-add]") && add.open) {
    add.querySelector('input[name="title"]')?.focus();
  }
}, true);

container?.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  const add = e.target instanceof Element && e.target.closest("[data-cal-add]");
  if (!add) return;
  add.removeAttribute("open");
  add.querySelector("summary")?.focus();
});

container?.addEventListener("submit", async (e) => {
  const form = e.target;
  if (!(form instanceof HTMLFormElement) || !form.closest("[data-cal-add]")) return;
  e.preventDefault();
  const frame = form.closest("[data-cal]");
  try {
    const res = await fetch(form.action, {
      method: "POST",
      body: new FormData(form),
      headers: { "Collage-Fetch": "1", "X-CSRF-Token": csrf() },
      credentials: "same-origin",
    });
    if (res.status === 422) {
      const doc = new DOMParser().parseFromString(await res.text(), "text/html");
      const reasons = [...doc.querySelectorAll("[data-cal-alert] li, .field__error")].map((li) => li.textContent.trim());
      toast(reasons.join(" ") || frame.dataset.error, true);
      return;
    }
    if (!res.ok) toast(frame.dataset.error, true);
  } catch {
    toast(frame.dataset.error, true);
    return;
  }
  live()?.refresh(container);
});
```

- [ ] **Step 2: Elle doğrula (gerçek uygulamada).** `run` becerisiyle uygulamayı başlat (ya da README'deki komut), bir board aç, Takvim sekmesine geç ve şunları gör:
  1. Ay geçişi ‹ / Bugün / › çalışıyor, filtre (ör. arama kutusu) ay değişince kalıyor.
  2. Çok günlük bir kartı başka güne sürükle → iki tarihi birlikte kayıyor; sayfayı yenileyince yerinde.
  3. **Yalnız başlangıç tarihi olan kartı** sürükle → kartın panelinde bitiş tarihi hâlâ boş (Review Focus #2).
  4. Bir karta Tab ile odaklan, → → ↓ bas → 600 ms sonra 9 gün ileri kaydediliyor, odak kartta kalıyor.
  5. Boş güne tıkla → başlık alanı açılıyor; Enter kartı o güne bitişle açıyor; Esc kapatıyor.
  6. İki sekmede aynı kartı farklı yerlere sürükle → ikincisinde çakışma bildirimi, ızgara geri yükleniyor.
  7. Dar pencere (≈ 375px) → ızgara yatay kayıyor, sayfa kaymıyor.
  8. Açık ve gece moru temada okunurluk.
  9. Tarayıcı konsolunda CSP hatası yok.
  10. İlk ve son haftada "+k daha" listesi ve yeni kart alanı çerçeveye kırpılmadan açılıyor (ilk haftalarda aşağı, son iki haftada yukarı).

Bir madde tutmazsa düzelt ve aynı listeyi yeniden çalıştır.

- [ ] **Step 3: Tüm testler.** `go test ./... -race -count=1` → PASS.

- [ ] **Step 4: Commit**

```bash
git add static/js/calendar.js templates/pages/board_calendar.html
git commit -m "feat(calendar): drag and keys to move a card, a click to add one"
```
