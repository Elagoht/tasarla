# Aşama 1 — Filtre ve arama: Uygulama Planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Board'da sunucu tarafında soluklaştıran bir filtre (metin, atanan kişi, etiket, öncelik, son tarih), aynı filtre Gantt'ta, kullanıcının takımlarında arayan `/search` sayfası ve `TIMEZONE` ortam değişkeni.

**Architecture:** Filtre URL'de durur. Board ve Gantt fragment'lerinin `data-collage-fragment` URL'sine kanonik bir sorgu dizgisi olarak eklenir. collage-live, paylaşılmayan render'ı her bağlantının kendi isteğiyle yaptığı için her okuyucu kendi filtresiyle render edilmiş bir push alır. Eşleşme kümesini store tek bir SQL sorgusuyla döndürür. Metin karşılaştırması migration'daki `kanban_fold()` fonksiyonu ve trigram indeksleriyle yapılır.

**Tech Stack:** Go 1.26, collage v0.39.2, collage-live v0.4.0, PostgreSQL 17 + `pg_trgm`, pgx v5, SortableJS, düz ES modülleri.

**Spec:** `docs/superpowers/specs/2026-10-01-filtre-swimlane-ical-sablon-markdown-design.md` §3 ve §6.4 (`TIMEZONE`).

## Global Constraints

- `any` türü hiçbir yerde kullanılmaz (Go'da `interface{}`/`any`, JS'de JSDoc `any` dahil).
- Kullanıcıya görünen her metin `locales/tr.json` ve `locales/en.json`'a birlikte eklenir. Katalog `Strict: true`'dur; eksik anahtar uygulamayı başlatmaz.
- CSP değişmez: satır içi script ve style yoktur. Yeni JS `static/js/` altında bir ES modülüdür.
- Board ve Gantt sayfaları `Dynamic()` kalır.
- Bilinmeyen ya da hatalı bir filtre değeri sessizce düşürülür. Yanıt her durumda 200'dür.
- `q` 2 karakterden (rune) kısaysa yok sayılır.
- Hafta pazartesi başlar. "Bugün", `TIMEZONE`'a göre belirlenir.
- Genel arama sayfa başına 50 sonuç gösterir. Alıntı en fazla 160 karakterdir.
- Commit mesajları semantik, İngilizce ve mevcut üsluptadır. Hepsi şu satırla biter: `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.
- Testler: `export KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable'` ve `go test ./... -race`.

## Kararlar (spec'i somutlaştıran)

- **Katlama:** `kanban_fold(t) = translate(lower(t), 'ı' || chr(775), 'i')`.
  - Ölçüm: yerel PG'de `lower('IĞDIR') = 'iğdir'`. Bu yüzden `ı` ayrıca `i`'ye katlanır.
  - glibc'de `lower('İ')`, `i` + U+0307 verebilir; U+0307 silinir.
  - Böylece `ı`, `I`, `i` ve `İ` birbiriyle eşleşir.
  - İndeksler ve sorgular aynı ifadeyi kullanır.
- **`pg_trgm` güveni:** yerel PG'de `pg_available_extension_versions.trusted = t`. Bu durumda veritabanının sahibi olan kullanıcı, superuser olmadan da extension'ı kurabilir.
- **Kanonik sorgu:** `url.Values.Encode()` anahtarları sıralar: `assignee, due, label, priority, q`. Değerler aşağıdaki sırayla yazılır, tekrarlar atılır:
  - atanan kişi: `me`, `none`, sonra artan id;
  - etiket ve öncelik: artan.

  Aynı filtre iki sekmede aynı URL'yi üretir. Filtre boşsa sorgu dizgisi `""` olur ve URL'ye `?` eklenmez.
- **Action yanıtları filtreyi korur:** `data-move-url` ve "kart ekle" formunun `action`'ı `?<filtre>` taşır. `loadColumns` filtreyi `rc.Request.URL.Query()`'den okur; bu, action isteğinde de aynı şekilde çalışır.
- **JS filtre akışı:** `data-collage-fragment` değişince `collageLive.scan()` çağrılır. Bunun nedeni `client.js`'in elementleri tam URL eşitliğiyle bulması (`byURL`) ve izlenen URL'leri yalnız stream açılırken okuması (`watches()`). `scan()` stream'i yeni URL'lerle yeniden açar. Yalnız `refresh` çağrılsaydı push'lar sessizce dururdu.
- **JS'siz "kart ekle" yönlendirmesi** filtreyi taşımaz. Board, filtresiz açılır. Bu kabul edilmiştir.

## Sapmalar

- Spec §3.3 indeksleri `lower(col)` üzerine kuruyor, sorguyu da `ILIKE` ile yapıyordu. Plan ikisini de `kanban_fold(col)` üzerine kuruyor. Ölçümde `lower('IĞDIR')` sonucu `iğdir` çıktı; `ılık` gibi bir sorgu bu yüzden eşleşmezdi. Ayrıca `ILIKE`, `lower()` üzerine kurulmuş bir indeksi kullanamaz.
- Spec §3.1, filtre değişince `collageLive.refresh` çağrılmasını öngörüyordu. Plan bunun yerine `scan()` ve ardından `refresh` çağırıyor; gerekçe Kararlar bölümünde.
- Gantt sayfasında filtre yazarken uygulanmaz. Değişiklik sayfayı yeniden yükler, metin kutusu da yalnız Enter ile gönderilir. Bunun nedeni, ölçek ve gruplama bağlantılarının filtreyi sunucuda kurulmuş kendi URL'lerinde taşıması.

## Review Focus

1. **Silinmiş yorum:** `comments.deleted_at` dolu olan bir yorumdaki metin board filtresinde eşleşmemeli ve genel aramada alıntı olarak sızmamalı. *(Task 2, Task 7)*
2. **Kişiye göre push:** iki kullanıcı aynı `?assignee=me` URL'sini açtığında, her biri kendi kartlarını eşleşmiş görmeli. Push, her bağlantının kendi oturumuyla render edilir. *(Task 4)*
3. **Hatalı parametreler:** `label=999`, `priority=9`, `assignee=abc` ve `due=yarin` 200 döner, düşürülür ve kanonik URL'de görünmez. *(Task 3, Task 4)*
4. **Gün sınırları:** `due=today`, `due=week` ve `due=overdue` hesaplanırken "bugün" `TIMEZONE`'a göre alınır. Testlerde saat sabit verilir, duvar saatine bağlı test yazılmaz. *(Task 2, Task 3)*
5. **Kısa sorgu:** 2 karakterlik `q` trigram indeksini kullanamaz (trigram 3 karakter ister), ama sonuç yine doğru olmalı. 1 karakterlik `q` hiç filtrelememeli. *(Task 2, Task 7)*

## Dosya haritası

| Dosya | Sorumluluk |
| --- | --- |
| `internal/config/config.go` | `TIMEZONE` → `Config.Location` |
| `internal/db/migrations/008_search.sql` | `pg_trgm`, `kanban_fold`, trigram indeksleri |
| `internal/store/filter.go` (yeni) | `BoardFilter`, `MatchingCardIDs` |
| `internal/store/search.go` (yeni) | `SearchHit`, `Search` |
| `internal/web/filter.go` (yeni) | URL ↔ filtre, kanonik sorgu, filtre çubuğunun verisi |
| `internal/web/highlight.go` (yeni) | Rune bazında katlama ve alıntı/vurgu parçaları |
| `internal/web/pages_board.go` | Kartları soluklaştırma, sayaç, `FilterQuery` |
| `internal/web/pages_gantt.go` | Çubukları soluklaştırma, filtreyi Gantt ayarlarıyla birleştirme |
| `internal/web/pages_search.go` (yeni) | `/search` sayfası |
| `templates/partials/filter_bar.html` (yeni) | Board ve Gantt'ın ortak filtre çubuğu |
| `templates/pages/search.html` (yeni) | Arama sonuçları |
| `templates/pages/board.html`, `templates/fragments/columns.html`, `templates/pages/board_gantt.html`, `templates/fragments/gantt.html`, `templates/layouts/app.html` | İşaretleme |
| `static/js/filter.js` (yeni) | Gönderme, `replaceState`, fragment URL'si, `scan()` |
| `static/css/board.css`, `static/css/pages.css`, `static/css/gantt.css` | Soluk kart ve çubuk, filtre çubuğu, arama sonuçları |
| `locales/tr.json`, `locales/en.json` | `filter.*`, `search.*`, `nav.search` |
| `README.md`, `kanban-spec.md` | `TIMEZONE`, `pg_trgm`, §13 |

---

### Task 1: `TIMEZONE`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/web/app.go` (handlers'a `loc`)
- Modify: `internal/web/pages_board.go` (`today`)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config.Location *time.Location` (hiç nil değildir); `handlers.loc *time.Location`.

- [ ] **Step 1: Başarısız testi yaz.** `config_test.go`'da geçerli ortamı üreten `env(overrides map[string]string) func(string) string` yardımcısı var.

```go
func TestTimezone(t *testing.T) {
	cases := []struct {
		value, want string
		ok          bool
	}{
		{"", time.Local.String(), true},
		{"Europe/Istanbul", "Europe/Istanbul", true},
		{"  UTC  ", "UTC", true},
		{"Mars/Olympus", "", false},
	}
	for _, c := range cases {
		cfg, err := config.Load(env(map[string]string{"TIMEZONE": c.value}))
		if !c.ok {
			if err == nil || !strings.Contains(err.Error(), "TIMEZONE") {
				t.Errorf("TIMEZONE=%q: err = %v, want one naming TIMEZONE", c.value, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("TIMEZONE=%q: %v", c.value, err)
		}
		if cfg.Location.String() != c.want {
			t.Errorf("TIMEZONE=%q: location = %s, want %s", c.value, cfg.Location, c.want)
		}
	}
}
```

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/config -run TestTimezone` çalıştırılır. Beklenen hata: `cfg.Location undefined`.

- [ ] **Step 3: Uygula.** `Config`'e alanı ekle, `Load`'da SMTP bloğundan önce oku:

```go
	// Location is the time zone "today" and schedules are reckoned in.
	Location *time.Location
```

```go
	cfg.Location = time.Local
	if tz := strings.TrimSpace(getenv("TIMEZONE")); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			fail("TIMEZONE", "must be an IANA time zone name, such as Europe/Istanbul")
		} else {
			cfg.Location = loc
		}
	}
```

`import "time"` ekle. `internal/web/app.go`'da `handlers` struct'ına `loc *time.Location` ekle ve `h := &handlers{…, loc: d.Config.Location}` olarak ver. `pages_board.go`'daki `loadColumns` içinde `today := time.Now().Format(time.DateOnly)` satırını `today := time.Now().In(h.loc).Format(time.DateOnly)` yap.

Binary embed nedeniyle sistemde tzdata bulunmayabilir. `main.go`'ya `import _ "time/tzdata"` ekle; ikili dosyaya yaklaşık 450 KB ekler ve container imajında `/usr/share/zoneinfo` olmasa da çalışmasını sağlar.

- [ ] **Step 4: Çalıştır.** `go test ./internal/config ./internal/web -race` çalıştırılır, hepsi PASS olmalı.

- [ ] **Step 5: README.** "Ortam değişkenleri" tablosuna `SMTP_PORT` satırından önce şu satırı ekle:

```
| `TIMEZONE` | hayır | "Bugün" ve zamanlamaların saat dilimi, IANA adı (ör. `Europe/Istanbul`). Verilmezse sunucunun yerel saati |
```

`.env.example`'a `# TIMEZONE=Europe/Istanbul` satırını ekle.

- [ ] **Step 6: Commit.**

```bash
git add internal/config main.go internal/web/app.go internal/web/pages_board.go README.md .env.example
git commit -m "feat(config): TIMEZONE, the zone today is reckoned in"
```

---

### Task 2: Migration 008 ve store filtresi

**Files:**
- Create: `internal/db/migrations/008_search.sql`
- Create: `internal/store/filter.go`
- Test: `internal/store/filter_test.go`

**Interfaces:**
- Produces:

```go
package store

type DueFilter string

const (
	DueAny     DueFilter = ""
	DueOverdue DueFilter = "overdue"
	DueToday   DueFilter = "today"
	DueWeek    DueFilter = "week"
	DueNone    DueFilter = "none"
)

// BoardFilter is what a board's cards are matched against.
type BoardFilter struct {
	Text        string  // at least 2 runes, or ""
	AssigneeIDs []int64
	Unassigned  bool
	LabelIDs    []int64
	Priorities  []int16
	Due         DueFilter
	Today       time.Time // the date today is, in the application's zone
}

func (f BoardFilter) Empty() bool
func (s *Store) MatchingCardIDs(ctx context.Context, boardID int64, f BoardFilter) (map[int64]bool, error)
```

`MatchingCardIDs`, tamamlanmış kartlar dahil arşivlenmemiş bütün kartları süzer, çünkü Gantt'ta "tamamlananlar" görünümü açık olabilir. Filtre `Empty()` ise sorgu çalışmaz ve `nil` döner. Çağıran taraf `nil`'i "her şey eşleşiyor" olarak okur.

- [ ] **Step 1: Migration'ı yaz.**

```sql
-- Searching cards by title, description and comments, ignoring case the way
-- Turkish needs (spec 2026-10-01 filtre-swimlane… §3.3).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- kanban_fold is the text a search compares: lower case, the dotless ı as i,
-- and the combining dot some C libraries leave after lowering İ dropped, so
-- that ı, I, i and İ all match one another.
CREATE FUNCTION kanban_fold(t text) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN translate(lower(t), 'ı' || chr(775), 'i');

CREATE INDEX cards_title_fold_trgm ON cards USING gin (kanban_fold(title) gin_trgm_ops);
CREATE INDEX cards_description_fold_trgm ON cards USING gin (kanban_fold(description) gin_trgm_ops);
CREATE INDEX comments_body_fold_trgm ON comments USING gin (kanban_fold(body) gin_trgm_ops);
```

- [ ] **Step 2: Başarısız testleri yaz.** `filter_test.go`'da `newBoardFixture` kullanılır (`boards_test.go`). Kart, etiket ve yorum, store API'siyle kurulur: `CreateCard`, `UpdateCardField`, `CreateLabel`, `SetCardLabels`, `AddComment`, `DeleteComment`.

```go
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
	label                                                                       store.Label
	now                                                                         time.Time
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
```

`DeleteComment`'in imzası `(ctx, boardID, cardID, commentID, userID int64, manager bool)` şeklindedir (`comments.go:111`); silmenin `deleted_at` alanını doldurduğunu orada doğrula. `AddComment` ise `Comment` döndürür. Alan adı `ID` değilse testi ona göre uyarla.

- [ ] **Step 3: Çalıştır, başarısız olduğunu gör.** `go test ./internal/store -run 'MatchingCardIDs' -race` çalıştırılır. Beklenen hata: `undefined: store.BoardFilter`.

- [ ] **Step 4: `internal/store/filter.go`'yu yaz.**

```go
package store

import (
	"context"
	"time"
)

// DueFilter picks cards by their due date, against today.
type DueFilter string

const (
	DueAny     DueFilter = ""
	DueOverdue DueFilter = "overdue" // before today
	DueToday   DueFilter = "today"
	DueWeek    DueFilter = "week" // today to the coming Sunday
	DueNone    DueFilter = "none" // no due date
)

// BoardFilter is what a board's cards are matched against (spec 2026-10-01
// filtre… §3.1). Dimensions are and-ed; the values within one are or-ed.
type BoardFilter struct {
	Text        string // matched in the title, the description and comments
	AssigneeIDs []int64
	Unassigned  bool
	LabelIDs    []int64
	Priorities  []int16
	Due         DueFilter
	Today       time.Time // the date today is, in the application's zone
}

// Empty reports whether the filter lets every card through.
func (f BoardFilter) Empty() bool {
	return f.Text == "" && len(f.AssigneeIDs) == 0 && !f.Unassigned && len(f.LabelIDs) == 0 &&
		len(f.Priorities) == 0 && f.Due == DueAny
}

// MatchingCardIDs returns the board's cards, done ones included, that are not
// archived and match f. An empty filter is nil: every card matches.
func (s *Store) MatchingCardIDs(ctx context.Context, boardID int64, f BoardFilter) (map[int64]bool, error) {
	if f.Empty() {
		return nil, nil
	}
	today := f.Today.Format(time.DateOnly)
	weekday := (int(f.Today.Weekday()) + 6) % 7 // Monday = 0
	sunday := f.Today.AddDate(0, 0, 6-weekday).Format(time.DateOnly)
	rows, err := s.pool.Query(ctx, `
		SELECT k.id FROM cards k
		WHERE k.board_id = $1 AND k.archived_at IS NULL
		  AND ($2 = '' OR kanban_fold(k.title) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		       OR kanban_fold(k.description) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		       OR EXISTS (SELECT 1 FROM comments c
		                  WHERE c.card_id = k.id AND c.deleted_at IS NULL
		                    AND kanban_fold(c.body) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'))
		  AND ((cardinality($3::bigint[]) = 0 AND NOT $4)
		       OR k.assignee_id = ANY ($3) OR ($4 AND k.assignee_id IS NULL))
		  AND (cardinality($5::bigint[]) = 0
		       OR EXISTS (SELECT 1 FROM card_labels cl WHERE cl.card_id = k.id AND cl.label_id = ANY ($5)))
		  AND (cardinality($6::smallint[]) = 0 OR k.priority = ANY ($6))
		  AND CASE $7
		        WHEN 'overdue' THEN k.due_date < $8::date
		        WHEN 'today' THEN k.due_date = $8::date
		        WHEN 'week' THEN k.due_date BETWEEN $8::date AND $9::date
		        WHEN 'none' THEN k.due_date IS NULL
		        ELSE true
		      END`,
		boardID, likeEscaper.Replace(f.Text), orEmpty(f.AssigneeIDs), f.Unassigned,
		orEmpty(f.LabelIDs), orEmpty(f.Priorities), string(f.Due), today, sunday)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// orEmpty sends a nil slice as an empty array, not NULL: cardinality(NULL) is
// NULL, which would make the whole condition false.
func orEmpty[T int64 | int16](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
```

- [ ] **Step 5: Çalıştır.** `go test ./internal/store -race` çalıştırılır, hepsi PASS olmalı. Var olan testler de migration'dan geçtiği için bu, 008'in temiz bir veritabanına uygulandığını da gösterir.

- [ ] **Step 6: Commit.**

```bash
git add internal/db/migrations/008_search.sql internal/store/filter.go internal/store/filter_test.go
git commit -m "feat(store): match a board's cards against a filter, folding Turkish case"
```

---

### Task 3: URL ↔ filtre (web)

**Files:**
- Create: `internal/web/filter.go`
- Test: `internal/web/filter_test.go` (`package web`, iç test; `text_test.go` gibi)

**Interfaces:**
- Consumes: `store.BoardFilter`, `store.DueFilter`, `store.Member`, `store.Label`.
- Produces:

```go
type boardFilter struct {
	Text       string
	Me         bool
	None       bool
	Assignees  []int64 // members, ascending
	Labels     []int64 // ascending
	Priorities []int16 // ascending
	Due        store.DueFilter
}
func parseBoardFilter(q url.Values, members []store.Member, labels []store.Label) boardFilter
func (f boardFilter) Active() bool
func (f boardFilter) Values() url.Values   // canonical; empty when inactive
func (f boardFilter) Query() string        // Values().Encode(); "" when inactive
func (f boardFilter) Store(me int64, today time.Time) store.BoardFilter
func (f boardFilter) HasAssignee(token string) bool // "me", "none" or an id
func (f boardFilter) HasLabel(id int64) bool
func (f boardFilter) HasPriority(p int16) bool
func withQuery(path, query string) string // path, or path + "?" + query
```

- [ ] **Step 1: Başarısız testi yaz.**

```go
package web

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"kanban/internal/store"
)

func TestParseBoardFilterIsCanonical(t *testing.T) {
	members := []store.Member{{User: store.User{ID: 7}}, {User: store.User{ID: 3}}}
	labels := []store.Label{{ID: 20}, {ID: 10}}
	q, _ := url.ParseQuery("q=+r%C3%A2+&assignee=7&assignee=me&assignee=abc&assignee=99&assignee=3&assignee=7&assignee=none" +
		"&label=20&label=999&label=10&priority=9&priority=4&priority=1&due=yarin")
	f := parseBoardFilter(q, members, labels)
	want := "assignee=me&assignee=none&assignee=3&assignee=7&label=10&label=20&priority=1&priority=4&q=r%C3%A2"
	if got := f.Query(); got != want {
		t.Errorf("Query() = %q\n want %q", got, want)
	}
	if f.Due != store.DueAny {
		t.Errorf("due = %q, want dropped", f.Due)
	}
}

func TestParseBoardFilterDropsShortTextAndEmpty(t *testing.T) {
	q, _ := url.ParseQuery("q=a&due=week")
	f := parseBoardFilter(q, nil, nil)
	if f.Text != "" || f.Query() != "due=week" {
		t.Errorf("got text %q, query %q", f.Text, f.Query())
	}
	if f := parseBoardFilter(url.Values{}, nil, nil); f.Active() || f.Query() != "" {
		t.Errorf("empty filter is active: %q", f.Query())
	}
}

func TestBoardFilterStoreResolvesMe(t *testing.T) {
	q, _ := url.ParseQuery("assignee=me&assignee=3&assignee=none")
	f := parseBoardFilter(q, []store.Member{{User: store.User{ID: 3}}}, nil)
	today := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	s := f.Store(42, today)
	if !slices.Equal(s.AssigneeIDs, []int64{3, 42}) || !s.Unassigned || !s.Today.Equal(today) {
		t.Errorf("store filter = %+v", s)
	}
}

func TestWithQuery(t *testing.T) {
	if withQuery("/boards/1", "") != "/boards/1" || withQuery("/boards/1", "q=ab") != "/boards/1?q=ab" {
		t.Error("withQuery")
	}
}
```

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run 'BoardFilter|WithQuery'` çalıştırılır. Beklenen hata: `undefined: parseBoardFilter`.

- [ ] **Step 3: `internal/web/filter.go`'yu yaz.**

```go
package web

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kanban/internal/store"
)

// boardFilter is the filter in a board's URL (spec 2026-10-01 filtre… §3.1),
// checked against the board: a value naming no member, label or choice is
// dropped, never refused, so an old link keeps working.
type boardFilter struct {
	Text       string
	Me         bool  // assignee=me: whoever is reading
	None       bool  // assignee=none: unassigned
	Assignees  []int64
	Labels     []int64
	Priorities []int16
	Due        store.DueFilter
}

var dueFilters = []store.DueFilter{store.DueOverdue, store.DueToday, store.DueWeek, store.DueNone}

func parseBoardFilter(q url.Values, members []store.Member, labels []store.Label) boardFilter {
	var f boardFilter
	if text := strings.TrimSpace(q.Get("q")); utf8.RuneCountInString(text) >= 2 {
		f.Text = text
	}
	isMember := map[int64]bool{}
	for _, m := range members {
		isMember[m.User.ID] = true
	}
	for _, v := range q["assignee"] {
		switch v {
		case "me":
			f.Me = true
		case "none":
			f.None = true
		default:
			if id, err := strconv.ParseInt(v, 10, 64); err == nil && isMember[id] && !slices.Contains(f.Assignees, id) {
				f.Assignees = append(f.Assignees, id)
			}
		}
	}
	isLabel := map[int64]bool{}
	for _, l := range labels {
		isLabel[l.ID] = true
	}
	for _, v := range q["label"] {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && isLabel[id] && !slices.Contains(f.Labels, id) {
			f.Labels = append(f.Labels, id)
		}
	}
	for _, v := range q["priority"] {
		if p, err := strconv.Atoi(v); err == nil && p >= 1 && p <= 4 && !slices.Contains(f.Priorities, int16(p)) {
			f.Priorities = append(f.Priorities, int16(p))
		}
	}
	if d := store.DueFilter(q.Get("due")); slices.Contains(dueFilters, d) {
		f.Due = d
	}
	slices.Sort(f.Assignees)
	slices.Sort(f.Labels)
	slices.Sort(f.Priorities)
	return f
}

// Active reports whether the filter dims anything.
func (f boardFilter) Active() bool { return len(f.Values()) > 0 }

// Values is the filter as a canonical query: one filter, one URL.
func (f boardFilter) Values() url.Values {
	v := url.Values{}
	if f.Text != "" {
		v.Set("q", f.Text)
	}
	if f.Me {
		v.Add("assignee", "me")
	}
	if f.None {
		v.Add("assignee", "none")
	}
	for _, id := range f.Assignees {
		v.Add("assignee", strconv.FormatInt(id, 10))
	}
	for _, id := range f.Labels {
		v.Add("label", strconv.FormatInt(id, 10))
	}
	for _, p := range f.Priorities {
		v.Add("priority", strconv.Itoa(int(p)))
	}
	if f.Due != store.DueAny {
		v.Set("due", string(f.Due))
	}
	return v
}

// Query is Values encoded; "" when the filter is not active.
func (f boardFilter) Query() string { return f.Values().Encode() }

// Store is the filter for reader me on the day today.
func (f boardFilter) Store(me int64, today time.Time) store.BoardFilter {
	ids := slices.Clone(f.Assignees)
	if f.Me && !slices.Contains(ids, me) {
		ids = append(ids, me)
		slices.Sort(ids)
	}
	return store.BoardFilter{Text: f.Text, AssigneeIDs: ids, Unassigned: f.None, LabelIDs: f.Labels,
		Priorities: f.Priorities, Due: f.Due, Today: today}
}

func (f boardFilter) HasAssignee(token string) bool {
	switch token {
	case "me":
		return f.Me
	case "none":
		return f.None
	}
	id, err := strconv.ParseInt(token, 10, 64)
	return err == nil && slices.Contains(f.Assignees, id)
}

func (f boardFilter) HasLabel(id int64) bool    { return slices.Contains(f.Labels, id) }
func (f boardFilter) HasPriority(p int16) bool  { return slices.Contains(f.Priorities, p) }

// withQuery is path, with ?query when there is one.
func withQuery(path, query string) string {
	if query == "" {
		return path
	}
	return path + "?" + query
}
```

- [ ] **Step 4: Çalıştır.** `go test ./internal/web -run 'BoardFilter|WithQuery' -race` çalıştırılır, PASS olmalı.

- [ ] **Step 5: Commit.**

```bash
git add internal/web/filter.go internal/web/filter_test.go
git commit -m "feat(web): read a board filter from the URL, canonically"
```

---

### Task 4: Board'da soluklaştırma, filtre çubuğu ve canlı güncelleme

**Files:**
- Modify: `internal/web/pages_board.go`
- Create: `templates/partials/filter_bar.html`
- Modify: `templates/pages/board.html`, `templates/fragments/columns.html`
- Modify: `locales/tr.json`, `locales/en.json`
- Test: `internal/web/filter_board_test.go`

**Interfaces:**
- Consumes: Task 3 (`parseBoardFilter`, `withQuery`), Task 2 (`MatchingCardIDs`), Task 1 (`h.loc`).
- Produces:

```go
// boardFilterFor is the board's filter for this render, parsed once.
func (h *handlers) boardFilterFor(ctx context.Context, rc *collage.RenderContext, bc boardContext) (filterView, error)

type filterView struct {
	Filter    boardFilter
	Query     string // Filter.Query()
	Action    string // the page the bar submits to, without a query
	Members   []store.Member
	Labels    []store.Label
	Extra     url.Values // other settings the page keeps (Gantt's scale…), as hidden fields
}
```

`columnsView`'e şu alanlar eklenir: `Filter filterView`, `Matching`, `Total int` ve `MoveURL string`. `cardView`'e `Dimmed bool` eklenir.

- [ ] **Step 1: Başarısız testleri yaz.** `filter_board_test.go`, `package web_test` olarak:

```go
package web_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"kanban/internal/store"
)

// The card the filter leaves out stays on the board, dimmed; the count says so.
func TestBoardFilterDimsCards(t *testing.T) {
	b := newBoardSetup(t)
	urgent := b.card(t, 0, "Urgent card")
	plain := b.card(t, 0, "Plain card")
	p := int16(4)
	if _, err := b.h.store.UpdateCardField(context.Background(), b.board.ID, urgent.ID, urgent.Version,
		store.FieldPriority, store.CardFields{Priority: &p}, b.h.user("lead@example.com").ID); err != nil {
		t.Fatal(err)
	}
	page := b.member.Get(b.path + "?priority=4")
	if page.Status != http.StatusOK {
		t.Fatalf("filtered board = %d", page.Status)
	}
	mustContain(t, page.Body,
		`class="card" id="card-`+id(urgent.ID)+`"`,
		`class="card card--dimmed" id="card-`+id(plain.ID)+`"`,
		"Eşleşen: 1 / 2",
		`data-collage-fragment="`+b.path+`/columns?priority=4"`,
		`data-move-url="`+b.path+`?priority=4"`)
	unfiltered := b.member.Get(b.path).Body
	if strings.Contains(unfiltered, "card--dimmed") || strings.Contains(unfiltered, "eşleşiyor") {
		t.Error("an unfiltered board dims cards")
	}
}

// Nonsense in the URL is dropped, not refused, and not echoed back.
func TestBoardFilterDropsBadValues(t *testing.T) {
	b := newBoardSetup(t)
	b.card(t, 0, "A card")
	page := b.member.Get(b.path + "?label=999&priority=9&assignee=abc&due=yarin&q=x")
	if page.Status != http.StatusOK {
		t.Fatalf("status = %d", page.Status)
	}
	mustContain(t, page.Body, `data-collage-fragment="`+b.path+`/columns"`)
	if strings.Contains(page.Body, "card--dimmed") {
		t.Error("dropped values still dim")
	}
}

// A move answered from a filtered board keeps the filter.
func TestMoveAnswerKeepsTheFilter(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Moved card")
	other := b.card(t, 0, "Other card")
	page := b.path + "?q=moved"
	r := b.lead.SubmitFetch(page, page, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version))
	if r.Status != http.StatusOK {
		t.Fatalf("move = %d", r.Status)
	}
	mustContain(t, r.Body, `class="card card--dimmed" id="card-`+id(other.ID)+`"`)
}

// Two readers on the same ?assignee=me URL are each pushed their own cards.
func TestFilteredPushIsPerReader(t *testing.T) {
	b := newBoardSetup(t)
	leadsCard := b.card(t, 0, "Lead's card")
	membersCard := b.card(t, 0, "Member's card")
	ctx := context.Background()
	lead, member := b.h.user("lead@example.com"), b.h.user("member@example.com")
	for _, x := range []struct {
		c  store.Card
		to int64
	}{{leadsCard, lead.ID}, {membersCard, member.ID}} {
		if _, err := b.h.store.UpdateCardField(ctx, b.board.ID, x.c.ID, x.c.Version, store.FieldAssignee,
			store.CardFields{AssigneeID: &x.to}, lead.ID); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(b.h.app.Handler())
	t.Cleanup(server.Close)
	fragment := b.path + "/columns?assignee=me"
	first := func(cookies []*http.Cookie) string {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/_live/stream/?f="+url.QueryEscape(fragment), nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "data:") {
				return line
			}
		}
		t.Fatal("no event")
		return ""
	}
	dimmed := func(c store.Card) string { return `class=\"card card--dimmed\" id=\"card-` + id(c.ID) + `\"` }
	leads, members := first(b.lead.Cookies()), first(b.member.Cookies())
	if !strings.Contains(leads, dimmed(membersCard)) || strings.Contains(leads, dimmed(leadsCard)) {
		t.Errorf("the lead's push does not match the lead's cards:\n%s", leads)
	}
	if !strings.Contains(members, dimmed(leadsCard)) || strings.Contains(members, dimmed(membersCard)) {
		t.Errorf("the member's push does not match the member's cards:\n%s", members)
	}
}
```

`moveForm` var olan bir yardımcıdır (`live_test.go`'da kullanılıyor). Board'da kartın `<li>` öğesi `class="card{{if .Summary.Blocked}} card--blocked{{end}}" id="card-…"` biçimindedir; `class` `id`'den önce gelir. Testler blok olmayan kartlar için `class="card"` ve `class="card card--dimmed"` bekler, bu yüzden sınıf sırası şablonda **`card`, `card--dimmed`, `card--blocked`** olmalı.

**Push testi başarısız olursa:** önce testin kendisini kontrol et (çerezler, URL kodlaması). Fragment'in sorgusunun render'a gerçekten taşınmadığı görülürse, `framework-issues/005-…md` dosyasını yaz (tür, paket ve sürüm, beklenen ve gözlenen davranış, minimal repro) ve dur. Geçici bir çözümle devam etme.

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run 'BoardFilter|MoveAnswerKeeps|FilteredPush' -race` çalıştırılır. Beklenen sonuç: `card--dimmed` ve sayaç bulunamaz.

- [ ] **Step 3: Handler.** `pages_board.go`'ya ekle:

```go
type filterView struct {
	Filter  boardFilter
	Query   string
	Action  string
	Members []store.Member
	Labels  []store.Label
	Extra   url.Values
}

// boardFilterFor is the board's filter for this render — the page's and its
// columns' — read from the request's own URL, so a fragment pushed or answered
// to an action keeps the filter its URL carries.
func (h *handlers) boardFilterFor(ctx context.Context, rc *collage.RenderContext, bc boardContext) (filterView, error) {
	return collage.Once(rc, "filter:"+rc.Param("id"), func(ctx context.Context) (filterView, error) {
		members, err := h.store.Members(ctx, bc.Team.ID)
		if err != nil {
			return filterView{}, err
		}
		labels, err := h.store.Labels(ctx, bc.Board.ID)
		if err != nil {
			return filterView{}, err
		}
		f := parseBoardFilter(rc.Request.URL.Query(), members, labels)
		return filterView{Filter: f, Query: f.Query(), Members: members, Labels: labels}, nil
	})
}
```

`boardView`'e `Filter filterView` ekle. `loadBoard`'da `fv, err := h.boardFilterFor(ctx, rc, bc)` çağır, sonra `fv.Action, _ = h.urlIn("board", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})`.

`h.urlIn`'in imzası `func(name, locale string, params map[string]string) (string, error)` biçimindedir. Hata, bilinmeyen bir sayfa adı anlamına gelir; bunu `return boardView{}, err` ile yukarı ilet.

`loadColumns`'da kartları dağıtmadan önce:

```go
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return columnsView{}, tags, err
	}
	today := time.Now().In(h.loc)
	matching, err := h.store.MatchingCardIDs(ctx, bc.Board.ID, fv.Filter.Store(bc.User.ID, today))
	if err != nil {
		return columnsView{}, tags, err
	}
	view.Filter = fv
	movePath, err := h.urlIn("board", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
	if err != nil {
		return columnsView{}, tags, err
	}
	view.MoveURL = withQuery(movePath, fv.Query)
```

Var olan `today := time.Now().In(h.loc).Format(time.DateOnly)` satırını `todayISO := today.Format(time.DateOnly)` yap ve `cv.Overdue` karşılaştırmasında `todayISO`'yu kullan. Kart döngüsüne şunu ekle:

```go
		view.Total++
		if matching == nil || matching[s.Card.ID] {
			view.Matching++
		} else {
			cv.Dimmed = true
		}
```

Etiket listesi `boardFilterFor` içinde zaten yüklendiği için tekrar sorgulanmaz.

- [ ] **Step 4: Şablonlar.** `templates/partials/filter_bar.html`:

```html
{{/* filter-bar: the board filter (spec 2026-10-01 filtre… §3.1), a GET form that works without scripts; filter.js submits it as it changes. Takes a filterView. */}}
{{define "filter-bar"}}
<form class="filter-bar" method="get" action="{{.Action}}" role="search" data-filter-bar>
  {{range $k, $vs := .Extra}}{{range $vs}}<input type="hidden" name="{{$k}}" value="{{.}}">{{end}}{{end}}
  <label class="sr-only" for="filter-q">{{t "filter.text"}}</label>
  <input class="input filter-bar__text" id="filter-q" name="q" type="search" value="{{.Filter.Text}}" placeholder="{{t "filter.text"}}" data-filter-text>
  <details class="filter-pick{{if or .Filter.Me .Filter.None .Filter.Assignees}} is-on{{end}}">
    <summary class="btn btn--quiet btn--sm">{{template "icon" "user"}} {{t "filter.assignee"}}</summary>
    <div class="filter-pick__list">
      <label><input type="checkbox" name="assignee" value="me"{{if .Filter.HasAssignee "me"}} checked{{end}}> {{t "filter.me"}}</label>
      <label><input type="checkbox" name="assignee" value="none"{{if .Filter.HasAssignee "none"}} checked{{end}}> {{t "board.unassigned"}}</label>
      {{range .Members}}<label><input type="checkbox" name="assignee" value="{{.User.ID}}"{{if $.Filter.HasAssignee (printf "%d" .User.ID)}} checked{{end}}> {{.User.Name}}</label>{{end}}
    </div>
  </details>
  {{if .Labels}}
  <details class="filter-pick{{if .Filter.Labels}} is-on{{end}}">
    <summary class="btn btn--quiet btn--sm">{{template "icon" "tag"}} {{t "filter.label"}}</summary>
    <div class="filter-pick__list">
      {{range .Labels}}<label><input type="checkbox" name="label" value="{{.ID}}"{{if $.Filter.HasLabel .ID}} checked{{end}}> <span class="chip label" data-color="{{.Color}}">{{.Name}}</span></label>{{end}}
    </div>
  </details>
  {{end}}
  <details class="filter-pick{{if .Filter.Priorities}} is-on{{end}}">
    <summary class="btn btn--quiet btn--sm">{{template "icon" "flag"}} {{t "filter.priority"}}</summary>
    <div class="filter-pick__list">
      {{range $p := priorityChoices}}<label><input type="checkbox" name="priority" value="{{$p}}"{{if $.Filter.HasPriority $p}} checked{{end}}> {{t (printf "card.priorities.%d" $p)}}</label>{{end}}
    </div>
  </details>
  <label class="sr-only" for="filter-due">{{t "filter.due"}}</label>
  <select class="input input--sm filter-bar__due" id="filter-due" name="due">
    <option value="">{{t "filter.due_any"}}</option>
    {{range dueChoices}}<option value="{{.}}"{{if eq (printf "%s" .) (printf "%s" $.Filter.Due)}} selected{{end}}>{{t (printf "filter.due_%s" .)}}</option>{{end}}
  </select>
  <noscript><button type="submit" class="btn btn--secondary btn--sm">{{t "filter.apply"}}</button></noscript>
  {{if .Filter.Active}}<a class="btn btn--quiet btn--sm" href="{{withQuery .Action (extraQuery .Extra)}}" data-filter-clear>{{t "filter.clear"}}</a>{{end}}
</form>
{{end}}
```

`app.go`'daki `Funcs`'a şu fonksiyonları ekle (`any` kullanılmaz):

```go
				"withQuery":       withQuery,
				"extraQuery":      func(v url.Values) string { return v.Encode() },
				"priorityChoices": func() []int16 { return []int16{4, 3, 2, 1} },
				"dueChoices":      func() []store.DueFilter { return dueFilters },
```

`user` ikonu `templates/partials/icons.html`'de var. `tag` ve `search` (Task 8) yok; `icons.html`'deki `if/else if` zincirine var olan biçimde ekle:

```html
{{- else if eq . "tag"}}<path d="M20 12l-8 8-9-9V3h8z"/><circle cx="7.5" cy="7.5" r="1.5"/>
{{- else if eq . "search"}}<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>
```

`board.html`'de `</header>`'dan sonra `{{template "filter-bar" .Filter}}` koy. `#board` öğesini şöyle değiştir:

```html
<div id="board" class="board" data-move-url="{{withQuery (pageURL "board" "id" .Board.ID) .Filter.Query}}"
  data-collage-fragment="{{withQuery (fragmentURL "board" "board-columns" "id" .Board.ID) .Filter.Query}}" data-collage-push data-collage-swap="morph">
```

Script listesine `<script type="module" src="{{asset "/static/js/filter.js"}}"></script>` ekle (Task 5).

`columns.html`'de, `.columns` öğesinden önce:

```html
{{if .Filter.Filter.Active}}<p class="filter-count" role="status">{{t "filter.count" "matching" (printf "%d" .Matching) "total" (printf "%d" .Total)}}</p>{{end}}
```

Kart öğesinin sınıfını şöyle yap: `class="card{{if .Dimmed}} card--dimmed{{end}}{{if .Summary.Blocked}} card--blocked{{end}}"`. Kart ekleme formunun `action`'ını `{{$.MoveURL}}` yap; `range` içinde `$` kökü gösterir.

- [ ] **Step 5: Dil dosyaları.** `tr.json`'a:

```json
"filter": {
  "text": "Kartlarda ara",
  "assignee": "Atanan",
  "me": "Ben",
  "label": "Etiket",
  "priority": "Öncelik",
  "due": "Son tarih",
  "due_any": "Her tarih",
  "due_overdue": "Gecikmiş",
  "due_today": "Bugün",
  "due_week": "Bu hafta",
  "due_none": "Tarihsiz",
  "apply": "Uygula",
  "clear": "Temizle",
  "count": "Eşleşen: {matching} / {total}"
}
```

`en.json`'a:

```json
"filter": {
  "text": "Search cards",
  "assignee": "Assignee",
  "me": "Me",
  "label": "Label",
  "priority": "Priority",
  "due": "Due date",
  "due_any": "Any date",
  "due_overdue": "Overdue",
  "due_today": "Today",
  "due_week": "This week",
  "due_none": "No date",
  "apply": "Apply",
  "clear": "Clear",
  "count": "{matching} of {total} cards match"
}
```

Türkçe metin bilerek ek almaz ("1'i", "2'si", "3'ü" sayıya göre değişirdi). Placeholder sözdizimi `{name}`'dir (`done.completed_by` gibi).

- [ ] **Step 6: Çalıştır.** `go test ./internal/web -race` çalıştırılır. Yeni testler ve var olan board testleri PASS olmalı. Var olan bir test `class="card"` biçimine bağlıysa, yeni sınıf sırasıyla uyumlu olduğunu kontrol et.

- [ ] **Step 7: Commit.**

```bash
git add internal/web templates locales
git commit -m "feat: board filter dims the cards it leaves out, pushed per reader"
```

---

### Task 5: `filter.js` ve stiller

**Files:**
- Create: `static/js/filter.js`
- Modify: `static/css/board.css`, `static/css/pages.css`

**Interfaces:**
- Consumes: `[data-filter-bar]` formu, `#board` (ya da `#gantt`) ve onun `data-collage-fragment`/`data-move-url` öznitelikleri, `window.collageLive.scan()` ve `refresh()`.

- [ ] **Step 1: `static/js/filter.js`'i yaz.**

```js
// filter.js: the board filter (spec 2026-10-01 filtre… §3.1). The bar is a GET
// form that works without this script; here it is sent as it changes, without
// a page load: the address bar takes the new query, and so does the live
// element's fragment URL.
//
// collage-live finds an element by its exact fragment URL and reads the URLs
// it watches only when its stream opens, so after the URL changes scan() must
// reopen the stream; refresh() alone would leave pushes going to the old URL,
// which no element shows any more.

const bar = document.querySelector("[data-filter-bar]");
const target = document.getElementById("board") || document.getElementById("gantt");
// The Gantt page's scale and grouping links carry the filter in their own
// URLs, made on the server: a filter changed there loads the page, so the
// links are made again; its text box is sent with Enter only.
const navigates = target?.id === "gantt";

function query() {
  const params = new URLSearchParams();
  for (const [k, v] of new FormData(bar)) {
    if (typeof v === "string" && v.trim() !== "") params.append(k, v.trim());
  }
  return params.toString();
}

function withQuery(url, q) {
  const base = url.split("?")[0];
  return q ? base + "?" + q : base;
}

let timer = 0;
function apply() {
  clearTimeout(timer);
  const q = query();
  history.replaceState(history.state, "", withQuery(location.pathname, q));
  target.dataset.collageFragment = withQuery(target.dataset.collageFragment, q);
  if (target.dataset.moveUrl) target.dataset.moveUrl = withQuery(target.dataset.moveUrl, q);
  const live = window.collageLive;
  if (live && !navigates) {
    live.scan();
    live.refresh(target);
  } else {
    location.search = q;
  }
}

if (bar && target) {
  bar.addEventListener("submit", (e) => {
    e.preventDefault();
    apply();
  });
  bar.addEventListener("change", (e) => {
    if (e.target instanceof HTMLInputElement && e.target.matches("[data-filter-text]")) return;
    apply();
  });
  bar.querySelector("[data-filter-text]")?.addEventListener("input", () => {
    if (navigates) return;
    clearTimeout(timer);
    timer = setTimeout(apply, 300);
  });
}
```

"Temizle" bağlantısı ve sayaç, sunucu tarafından render edilir. Temizle bir sayfa yüklemesi yapar; bu kabul edilmiştir. Sayaç ise board fragment'inin içinde olduğu için refresh ile güncellenir.

`filter.js`'i `board.html`'de `board.js`'den sonra yükle.

- [ ] **Step 2: Stiller.** `board.css`'e:

```css
/* A card the filter leaves out: still there, still draggable, out of the way. */
.card--dimmed { opacity: 0.38; filter: saturate(0.4); }
.card--dimmed:hover, .card--dimmed:focus-within { opacity: 0.8; }
.filter-count { margin: 0 0 var(--s3); font-size: var(--text-sm); color: var(--ink-soft); }
```

`pages.css`'e:

```css
/* The board filter: a row of controls that wraps on a narrow screen. */
.filter-bar { display: flex; flex-wrap: wrap; gap: var(--s2); align-items: center; margin: 0 0 var(--s4); }
.filter-bar__text { width: min(260px, 100%); }
.filter-pick { position: relative; }
.filter-pick.is-on > summary { background: var(--accent-soft); color: var(--accent-ink, var(--ink)); }
.filter-pick__list {
  position: absolute; z-index: 20; top: calc(100% + var(--s1)); left: 0; min-width: 14rem; max-height: 18rem; overflow: auto;
  display: grid; gap: var(--s1); padding: var(--s2); border-radius: var(--r-md);
  background: var(--sheet); border: 1px solid var(--line); box-shadow: var(--shadow-float);
}
.filter-pick__list label { display: flex; gap: var(--s2); align-items: center; padding: var(--s1); border-radius: var(--r-sm); cursor: pointer; }
.filter-pick__list label:hover { background: var(--shelf); }
```

`tokens.css`'te kullanılan değişkenlerin (`--accent-soft`, `--r-sm`, `--shadow-float`, `--shelf`) var olduğunu doğrula. Olmayan varsa en yakın token'ı kullan.

- [ ] **Step 3: Elle doğrula.** `collage dev` çalıştır. Board'da şunları kontrol et:
  - metin yazınca yaklaşık 300 ms sonra kartlar soluklaşıyor;
  - URL değişiyor;
  - başka bir sekmede kart taşınınca filtreli sekme push alıyor ve soluk kartlar soluk kalıyor;
  - filtreliyken bir kartı sürükleyince filtre kaybolmuyor;
  - JS kapalıyken "Uygula" çalışıyor.

  Bu son madde Global Constraints'teki JS'siz yol gereksinimidir. Push alınmıyorsa `scan()` davranışı belgedeki gibi değildir. Bu durumda framework issue'su yaz ve dur.

- [ ] **Step 4: Commit.**

```bash
git add static/js/filter.js static/css templates/pages/board.html
git commit -m "feat(ui): the board filter applies as it is typed, and keeps live updates"
```

---

### Task 6: Gantt'ta filtre

**Files:**
- Modify: `internal/web/pages_gantt.go`
- Modify: `templates/pages/board_gantt.html`, `templates/fragments/gantt.html`, `static/css/gantt.css`
- Test: `internal/web/gantt_test.go`

**Interfaces:**
- Consumes: `boardFilterFor`, `filterView.Extra`, `withQuery`, `MatchingCardIDs`.
- Produces: `layoutGantt(rc, bc, scale, group, cols, cards, deps, now, matching map[int64]bool)`. Burada `matching` `nil` ise hiçbir çubuk soluk olmaz. `ganttBarView.Dimmed` ve `ganttRowView.Dimmed` alanları eklenir.

- [ ] **Step 1: Başarısız testi yaz.** `gantt_test.go`'ya ekle. Dosyadaki var olan Gantt testlerinin, bir kartı tarihlendirmek için kullandığı yardımcıyı kullan:

```go
func TestGanttFilterDimsBars(t *testing.T) {
	b := newBoardSetup(t)
	ctx := context.Background()
	lead := b.h.user("lead@example.com").ID
	due := time.Now().AddDate(0, 0, 2)
	dated := func(title string) store.Card {
		c := b.card(t, 0, title)
		out, err := b.h.store.UpdateCardField(ctx, b.board.ID, c.ID, c.Version, store.FieldDueDate, store.CardFields{DueDate: &due}, lead)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	alpha, beta := dated("Alpha"), dated("Beta")
	page := b.member.Get(b.path + "/gantt?scale=week&q=alpha")
	if page.Status != http.StatusOK {
		t.Fatalf("gantt = %d", page.Status)
	}
	mustContain(t, page.Body,
		`/gantt/chart?group=column&amp;q=alpha&amp;scale=week"`,
		`name="scale" value="week"`)
	// A bar's class comes a line before its data-card.
	dimmedBar := func(c store.Card) *regexp.Regexp {
		return regexp.MustCompile(`class="gantt__bar[^"]*\bis-dimmed\b[^"]*"[^>]*data-card="` + id(c.ID) + `"`)
	}
	if !dimmedBar(beta).MatchString(page.Body) {
		t.Error("the bar the filter leaves out is not dimmed")
	}
	if dimmedBar(alpha).MatchString(page.Body) {
		t.Error("the matching bar is dimmed")
	}
}
```

Test `regexp` ve `strings` import eder. Çubuk işaretlemesi `fragments/gantt.html:34`'tedir; `is-dimmed` sınıfı `class`'ın sonuna eklenir.

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run GanttFilter -race` çalıştırılır.

- [ ] **Step 3: Uygula.**
  - `loadGanttPage`:
    - `bc` yüklendikten sonra `fv, err := h.boardFilterFor(ctx, rc, bc)` çağır.
    - `fv.Extra` değeri `url.Values{"scale": {v.Scale}, "group": {v.Group}}` olur; `v.Done` ise `fv.Extra.Set("done", "1")` ekle.
    - `fv.Action`, `pageURL "board-gantt"` olur (`h.urlIn("board-gantt", …)`).
    - `v.Query` şöyle kurulur: `fv.Filter.Values()` alınır, `fv.Extra`'nın anahtarları üzerine yazılır, sonra `Encode()` edilir.
    - `ganttPageView`'e `Filter filterView` ekle.
  - `board_gantt.html`'de `</header>`'dan sonra `{{template "filter-bar" .Filter}}` koy. Segmented bağlantılarda (ölçek, gruplama, tamamlananlar) şu an `?scale=…&group=…` elle yazılıyor. Bunları filtreyi de taşıyacak şekilde değiştir: her bağlantı aşağıdaki `ganttLink` yardımcısıyla kurulur. Yardımcıyı `pages_gantt.go`'da yaz ve `Funcs`'a ekle:

```go
// ganttLink is the chart's query with the filter kept and one setting changed.
func ganttLink(f boardFilter, scale, group string, done bool) string {
	v := f.Values()
	v.Set("scale", scale)
	v.Set("group", group)
	if done {
		v.Set("done", "1")
	}
	return v.Encode()
}
```

  Şablonda: `href="{{$self}}?{{ganttLink $.Filter.Filter . $.Group $.Done}}"`. Gruplama bağlantısı için `{{ganttLink $.Filter.Filter $.Scale . $.Done}}`, tamamlananlar düğmesi için `{{ganttLink .Filter.Filter .Scale .Group (not .Done)}}`.
  - `loadGantt`'da `fv`'yi al ve `matching, err := h.store.MatchingCardIDs(ctx, bc.Board.ID, fv.Filter.Store(bc.User.ID, time.Now().In(h.loc)))` çağır. Sonucu `layoutGantt`'a ver ve orada, çubuk ve satır oluşturulurken `Dimmed: matching != nil && !matching[c.ID]` olarak ayarla. `layoutGantt`'ın `now` parametresine de `time.Now().In(h.loc)` ver.
  - `gantt.html`'de çubuğun sınıfına `{{if .Dimmed}} is-dimmed{{end}}` ekle. Satır etiketinde de aynısını yap; satır öğesinin sınıfını şablonda bul.
  - `gantt.css`:

```css
.gantt__bar.is-dimmed, .gantt__label.is-dimmed { opacity: 0.3; }
```

  Satır etiketinin sınıf adı `.gantt__label` değilse, şablondaki gerçek adı kullan.
  - `board_gantt.html`'de `gantt.js`'den sonra `filter.js`'i yükle. Gantt'ta `filter.js` sayfayı yeniden yükler (`navigates`), böylece segmented bağlantılar yeni filtreyle sunucuda yeniden kurulur.

- [ ] **Step 4: Çalıştır.** `go test ./internal/web -race` çalıştırılır, hepsi PASS olmalı.

- [ ] **Step 5: Commit.**

```bash
git add internal/web templates static
git commit -m "feat: the board filter dims the Gantt chart's bars too"
```

---

### Task 7: Store'da genel arama

**Files:**
- Create: `internal/store/search.go`
- Test: `internal/store/search_test.go`

**Interfaces:**
- Produces:

```go
type SearchHit struct {
	Card      Card
	BoardName string
	TeamName  string
	InTitle   bool
	// Excerpt is the description or the latest comment the query was found
	// in, raw; "" when only the title holds it.
	Excerpt   string
}
const SearchPageSize = 50
func (s *Store) Search(ctx context.Context, userID int64, q string, page int) ([]SearchHit, bool, error)
```

`page` 1'den başlar. İkinci dönüş değeri "bir sonraki sayfa var mı" bilgisidir. `q`, 2 rune'dan kısaysa sorgu çalışmaz ve `nil, false, nil` döner.

- [ ] **Step 1: Başarısız testleri yaz.**

```go
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

func TestSearchExcerptsAndOrder(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	inComment, _ := f.s.CreateCard(ctx, f.board.ID, f.cols[0].ID, "Toplantı", f.lead.ID)
	f.s.AddComment(ctx, f.board.ID, inComment.ID, f.lead.ID, "bütçe ışığında karar", nil)
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
	if hits[1].Excerpt != "bütçe ışığında karar" {
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
```

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/store -run Search -race` çalıştırılır. Beklenen hata: `undefined: store.SearchHit`.

- [ ] **Step 3: `internal/store/search.go`'yu yaz.**

```go
package store

import (
	"context"
	"unicode/utf8"
)

// SearchPageSize is how many hits one page of a search shows.
const SearchPageSize = 50

// SearchHit is a card a search found.
type SearchHit struct {
	Card      Card
	BoardName string
	TeamName  string
	InTitle   bool
	// Excerpt is the description or the latest live comment the query was
	// found in, raw; "" when only the title holds it.
	Excerpt string
}

// Search finds the cards of the reader's teams' boards, done and archived
// ones included, whose title, description or a live comment holds q (spec
// 2026-10-01 filtre… §3.2). Title matches come first, then the closer ones,
// then the more recently touched. page counts from 1; more reports whether a
// further page has hits.
func (s *Store) Search(ctx context.Context, userID int64, q string, page int) ([]SearchHit, bool, error) {
	if utf8.RuneCountInString(q) < 2 {
		return nil, false, nil
	}
	if page < 1 {
		page = 1
	}
	rows, err := s.pool.Query(ctx, `
		WITH found AS (
		  SELECT k.*, b.name AS board_name, t.name AS team_name,
		         kanban_fold(k.title) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\' AS in_title,
		         CASE WHEN kanban_fold(k.description) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		              THEN k.description END AS in_description,
		         (SELECT c.body FROM comments c
		          WHERE c.card_id = k.id AND c.deleted_at IS NULL
		            AND kanban_fold(c.body) LIKE '%' || kanban_fold($2) || '%' ESCAPE '\'
		          ORDER BY c.id DESC LIMIT 1) AS in_comment
		  FROM cards k
		  JOIN boards b ON b.id = k.board_id AND b.archived_at IS NULL
		  JOIN teams t ON t.id = b.team_id
		  JOIN team_members m ON m.team_id = b.team_id AND m.user_id = $1
		)
		SELECT `+prefixed("f", cardColumns)+`, f.board_name, f.team_name, f.in_title,
		       coalesce(CASE WHEN f.in_title THEN '' ELSE coalesce(f.in_description, f.in_comment) END, '')
		FROM found f
		WHERE f.in_title OR f.in_description IS NOT NULL OR f.in_comment IS NOT NULL
		ORDER BY f.in_title DESC,
		         similarity(kanban_fold(f.title), kanban_fold($3)) DESC,
		         coalesce((SELECT max(a.created_at) FROM activity a WHERE a.card_id = f.id), f.created_at) DESC,
		         f.id DESC
		LIMIT $4 OFFSET $5`,
		userID, likeEscaper.Replace(q), q, SearchPageSize+1, (page-1)*SearchPageSize)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		c := &h.Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Position, &c.Title, &c.Description, &c.AssigneeID,
			&c.Estimate, &c.DueDate, &c.Priority, &c.CreatedBy, &c.Version, &c.ArchivedAt, &c.CreatedAt, &c.CompletedAt, &c.CompletedFrom, &c.StartDate,
			&h.BoardName, &h.TeamName, &h.InTitle, &h.Excerpt); err != nil {
			return nil, false, err
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(hits) > SearchPageSize
	if more {
		hits = hits[:SearchPageSize]
	}
	return hits, more, nil
}
```

`cardColumns` içindeki `estimate::float8` ifadesi, `prefixed("f", …)` ile `f.estimate::float8` olur; bu CTE üzerinde de geçerlidir. Başlıkta eşleşme varsa, açıklamada da eşleşme olsa bile alıntı boş bırakılır; vurgu başlıkta yapılır.

- [ ] **Step 4: Çalıştır.** `go test ./internal/store -race` çalıştırılır, PASS olmalı.

- [ ] **Step 5: Commit.**

```bash
git add internal/store/search.go internal/store/search_test.go
git commit -m "feat(store): search the cards of the reader's teams"
```

---

### Task 8: `/search` sayfası ve kenar çubuğundaki kutu

**Files:**
- Create: `internal/web/highlight.go`, `internal/web/highlight_test.go` (`package web`)
- Create: `internal/web/pages_search.go`, `templates/pages/search.html`, `templates/partials/highlight.html`
- Modify: `internal/web/app.go` (`pages()`), `templates/layouts/app.html`, `static/css/pages.css`, `locales/*.json`
- Test: `internal/web/search_test.go` (`package web_test`)

**Interfaces:**
- Consumes: `store.Search`, `store.SearchHit`, `store.SearchPageSize`.
- Produces:

```go
type highlighted struct {
	Before, Match, After string
	CutStart, CutEnd     bool // text was left out before Before / after After
}
// highlight finds q in text, folded as kanban_fold folds, and keeps up to
// width runes around it (0: all of it). No match: the first width runes.
func highlight(text, q string, width int) highlighted
```

- [ ] **Step 1: `highlight` için başarısız testi yaz.**

```go
package web

import "testing"

func TestHighlightFoldsLikeTheDatabase(t *testing.T) {
	h := highlight("Bütçe IŞIĞINDA karar", "ışığında", 0)
	if h.Before != "Bütçe " || h.Match != "IŞIĞINDA" || h.After != " karar" || h.CutStart || h.CutEnd {
		t.Errorf("got %+v", h)
	}
}

func TestHighlightKeepsAWindow(t *testing.T) {
	long := ""
	for range 100 {
		long += "a"
	}
	h := highlight(long+"HEDEF"+long, "hedef", 40)
	if h.Match != "HEDEF" || !h.CutStart || !h.CutEnd || len([]rune(h.Before+h.Match+h.After)) != 40 {
		t.Errorf("got %+v", h)
	}
	if h := highlight("kısa metin", "yok", 160); h.Before != "kısa metin" || h.Match != "" {
		t.Errorf("no match: %+v", h)
	}
}
```

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run Highlight` çalıştırılır.

- [ ] **Step 3: `internal/web/highlight.go`'yu yaz.**

```go
package web

import (
	"slices"
	"unicode"
)

type highlighted struct {
	Before, Match, After string
	CutStart, CutEnd     bool
}

// fold is kanban_fold (migration 008) rune by rune, keeping where each folded
// rune came from, so a match in the folded text is a span of the original.
func fold(s []rune) ([]rune, []int) {
	out, from := make([]rune, 0, len(s)), make([]int, 0, len(s))
	for i, r := range s {
		switch r {
		case '̇': // the dot lowering İ may leave
			continue
		case 'İ', 'I', 'ı':
			r = 'i'
		default:
			r = unicode.ToLower(r)
		}
		out, from = append(out, r), append(from, i)
	}
	return out, from
}

func highlight(text, q string, width int) highlighted {
	runes := []rune(text)
	folded, from := fold(runes)
	needle, _ := fold([]rune(q))
	start, end := -1, -1
	if len(needle) > 0 {
		for i := 0; i+len(needle) <= len(folded); i++ {
			if slices.Equal(folded[i:i+len(needle)], needle) {
				start, end = from[i], from[i+len(needle)-1]+1
				break
			}
		}
	}
	lo, hi := 0, len(runes)
	if width > 0 && len(runes) > width {
		if start < 0 {
			hi = width
		} else {
			lo = max(0, start-(width-(end-start))/3)
			hi = min(len(runes), lo+width)
			lo = max(0, hi-width)
		}
	}
	h := highlighted{CutStart: lo > 0, CutEnd: hi < len(runes)}
	if start < 0 {
		h.Before = string(runes[lo:hi])
		return h
	}
	h.Before, h.Match, h.After = string(runes[lo:max(lo, start)]), string(runes[max(lo, start):min(hi, end)]), string(runes[min(hi, end):hi])
	return h
}
```

- [ ] **Step 4: Sayfa için başarısız testi yaz.** `search_test.go`:

```go
package web_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestSearchPage(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Rapor <b>kalın</b>")
	if err := b.h.store.ArchiveCard(context.Background(), b.board.ID, c.ID, b.h.user("lead@example.com").ID); err != nil {
		t.Fatal(err)
	}
	page := b.member.Get("/search?q=rapor")
	if page.Status != http.StatusOK {
		t.Fatalf("search = %d", page.Status)
	}
	mustContain(t, page.Body, "<mark>Rapor</mark>", "&lt;b&gt;kalın&lt;/b&gt;", "Sprint", "Arşivde",
		`href="`+b.cardPath(c)+`"`)
	if strings.Contains(page.Body, "<b>kalın</b>") {
		t.Error("a title's markup reached the page")
	}
	if empty := b.member.Get("/search?q=r"); empty.Status != http.StatusOK || strings.Contains(empty.Body, "<mark>") {
		t.Errorf("one letter = %d", empty.Status)
	}
	b.h.speaks("member@example.com", "en")
	mustContain(t, b.member.Get("/en/search?q=rapor").Body, "Archived")
}

func TestSearchPageIsPrivate(t *testing.T) {
	b := newBoardSetup(t)
	if r := b.h.browser().Get("/search?q=rapor"); r.Status != http.StatusSeeOther {
		t.Fatalf("signed out = %d, want a redirect to sign in", r.Status)
	}
}
```

Arşivlenmiş kartın bağlantısı kart sayfasına gider; o sayfanın arşivlenmiş kartı nasıl gösterdiği bugünkü davranışına bırakılır. `/en/search`, hesap dili `tr` olduğu sürece `/search`'e yönlendirilir; bu yüzden İngilizce kontrolden önce `speaks` çağrılır.

- [ ] **Step 5: Çalıştır, başarısız olduğunu gör.** `go test ./internal/web -run SearchPage -race` çalıştırılır. Beklenen sonuç: 404.

- [ ] **Step 6: Sayfayı yaz.** `pages_search.go`:

```go
package web

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type searchView struct {
	Query  string
	Ran    bool // the query was long enough to run
	Groups []searchGroup
	Page   int
	More   bool
}

type searchGroup struct {
	BoardID   int64
	BoardName string
	TeamName  string
	Hits      []searchHitView
}

type searchHitView struct {
	Hit     store.SearchHit
	Title   highlighted
	Excerpt highlighted
	Done    bool
}

func (h *handlers) searchPage() *collage.Page {
	content := collage.NewFragment("search-content", "pages/search.html").
		WithDataHandler(collage.Load(h.loadSearch)).
		Required().
		Build()
	return paths(h.privatePage("search", content), "/search").Dynamic().Build()
}

func (h *handlers) loadSearch(ctx context.Context, rc *collage.RenderContext) (searchView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return searchView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "search.title"))
	q := rc.Request.URL.Query()
	v := searchView{Query: strings.TrimSpace(q.Get("q")), Page: 1}
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 1 {
		v.Page = p
	}
	hits, more, err := h.store.Search(ctx, user.ID, v.Query, v.Page)
	if err != nil {
		return v, err
	}
	v.Ran, v.More = utf8.RuneCountInString(v.Query) >= 2, more
	// Grouped by board, in the order the boards' best hits came.
	at := map[int64]int{}
	for _, hit := range hits {
		i, ok := at[hit.Card.BoardID]
		if !ok {
			i = len(v.Groups)
			at[hit.Card.BoardID] = i
			v.Groups = append(v.Groups, searchGroup{BoardID: hit.Card.BoardID, BoardName: hit.BoardName, TeamName: hit.TeamName})
		}
		hv := searchHitView{Hit: hit, Title: highlight(hit.Card.Title, v.Query, 0), Done: hit.Card.CompletedAt != nil}
		if hit.Excerpt != "" {
			hv.Excerpt = highlight(hit.Excerpt, v.Query, 160)
		}
		v.Groups[i].Hits = append(v.Groups[i].Hits, hv)
	}
	return v, nil
}
```

`app.go`'daki `pages()` listesine `h.searchPage()` ekle.

`templates/partials/highlight.html` (partials bütün sayfalara yüklenir, `field-state` gibi):

```html
{{/* hl: a highlighted piece of text — the search's match marked, the cut ends shown. Takes a highlighted. */}}
{{define "hl"}}{{if .CutStart}}…{{end}}{{.Before}}{{with .Match}}<mark>{{.}}</mark>{{end}}{{.After}}{{if .CutEnd}}…{{end}}{{end}}
```

`templates/pages/search.html`:

```html
<header class="page-head">
  <div><h1>{{t "search.title"}}</h1></div>
  <form class="search" method="get" action="{{pageURL "search"}}" role="search">
    <label class="sr-only" for="search-q">{{t "search.title"}}</label>
    <input class="input" id="search-q" name="q" type="search" value="{{.Query}}" placeholder="{{t "search.placeholder"}}" minlength="2" autofocus>
    <button type="submit" class="btn btn--secondary">{{t "archive.search_button"}}</button>
  </form>
</header>
{{if .Groups}}
<div class="stack stack--lg">
  {{range .Groups}}
  <section class="stack stack--sm">
    <div class="section-head">
      <h2 class="h3"><a class="section-head__link" href="{{pageURL "board" "id" .BoardID}}"><span class="dot" data-color="{{boardColor .BoardID}}" aria-hidden="true"></span>{{.BoardName}}</a></h2>
      <span class="muted small">{{.TeamName}}</span>
    </div>
    <ul class="list-rows">
      {{range .Hits}}
      <li class="list-rows__row search-hit">
        <div class="list-rows__main search-hit__main">
          <a class="list-rows__title" href="{{pageURL "card" "id" .Hit.Card.BoardID "card" .Hit.Card.ID}}">{{template "hl" .Title}}</a>
          {{if .Hit.Excerpt}}<p class="search-hit__excerpt">{{template "hl" .Excerpt}}</p>{{end}}
        </div>
        <div class="actions">
          {{if .Hit.Card.ArchivedAt}}<span class="chip">{{t "search.archived"}}</span>{{else if .Done}}<span class="chip">{{t "search.done"}}</span>{{end}}
        </div>
      </li>
      {{end}}
    </ul>
  </section>
  {{end}}
  <nav class="pager">
    {{if gt .Page 1}}<a class="btn btn--quiet" href="{{pageURL "search"}}?q={{.Query}}&page={{dec .Page}}">{{t "search.previous"}}</a>{{end}}
    {{if .More}}<a class="btn btn--quiet" href="{{pageURL "search"}}?q={{.Query}}&page={{inc .Page}}">{{t "search.next"}}</a>{{end}}
  </nav>
</div>
{{else if .Ran}}
<div class="empty"><span class="empty__icon">{{template "icon" "search"}}</span><p>{{t "search.empty"}}</p></div>
{{end}}
```

`html/template`, `href` içindeki `?q={{.Query}}` değerini URL olarak escape eder. `Funcs`'a `"dec": func(n int) int { return n - 1 }` ekle; `inc` zaten var.

`app.html`'de `.sidebar__top`'tan sonra:

```html
    <form class="sidebar__search" method="get" action="{{pageURL "search"}}" role="search">
      <label class="sr-only" for="sidebar-q">{{t "nav.search"}}</label>
      <input class="input input--sm" id="sidebar-q" name="q" type="search" placeholder="{{t "nav.search"}}" minlength="2">
    </form>
```

`pages.css`'e şunları ekle: `.search-hit__main { display: grid; gap: var(--s1); }`, `.search-hit__excerpt { margin: 0; font-size: var(--text-sm); color: var(--ink-soft); }`, `mark { background: var(--accent-soft); color: inherit; border-radius: 2px; padding: 0 1px; }` ve `.pager { display: flex; gap: var(--s2); justify-content: flex-end; }`. Kenar çubuğu sınıfının stili `layout.css`'e girer: `.sidebar__search { padding: 0 var(--s3) var(--s3); } .sidebar__search .input { width: 100%; }`. Kenar çubuğu daraltılmışsa bu kutuyu gizle; daraltılmış durumun sınıfını `layout.css`'te bul.

Dil dosyaları. `tr`: `"nav": {"search": "Ara", …}` ve `"search": {"title": "Arama", "placeholder": "Kart, açıklama ya da yorum ara", "empty": "Eşleşen kart yok.", "archived": "Arşivde", "done": "Tamamlandı", "previous": "Önceki", "next": "Sonraki"}`. `en`: `"search": "Search"` ve `{"title": "Search", "placeholder": "Search cards, descriptions and comments", "empty": "No card matches.", "archived": "Archived", "done": "Done", "previous": "Previous", "next": "Next"}`.

- [ ] **Step 7: Çalıştır.** `go test ./... -race` çalıştırılır, hepsi PASS olmalı.

- [ ] **Step 8: Commit.**

```bash
git add internal/web templates static locales
git commit -m "feat: search every board of the reader's teams, from the sidebar"
```

---

### Task 9: Belgeler ve aşama kapanışı

**Files:**
- Modify: `README.md`, `kanban-spec.md`

- [ ] **Step 1: README.**
  - "Neler var" listesine şu iki maddeyi ekle:
    - "**Filtre ve arama:** board'da metin, atanan kişi, etiket, öncelik ve son tarihe göre filtre; uymayan kartlar soluklaşır ve filtre canlı güncellemelerde korunur. `/search`, takımlardaki bütün kartlarda arar."
    - Dil maddesinin altına "Saat dilimi `TIMEZONE` ile verilir."
  - "Geliştirme" bölümünün altına şu notu ekle: "**PostgreSQL eklentisi:** migration 008, `pg_trgm`'i kurar (`CREATE EXTENSION`). `pg_trgm` güvenilir (trusted) bir eklentidir: veritabanının sahibi olan kullanıcı superuser olmadan kurabilir. Uygulamanın kullanıcısı veritabanının sahibi değilse eklentiyi bir kez elle kurun."

- [ ] **Step 2: `kanban-spec.md` §13.** "swimlane'ler" sözcüğünü kapsam dışı listesinden çıkarma; o, Aşama 2'de çıkacak. Kapsam dışı listesinin sonuna şu cümleyi ekle: "Sunucu tarafında soluklaştıran filtre ve genel arama kapsama alındı: bkz. `docs/superpowers/specs/2026-10-01-filtre-swimlane-ical-sablon-markdown-design.md`."

- [ ] **Step 3: Tam doğrulama.** `gofmt -l .` boş çıkmalı, `go vet ./...` temiz olmalı, `go test ./... -race` PASS olmalı. Ardından `collage dev` ile Task 5 Step 3'teki elle doğrulamayı ve `/search` sayfasını kenar çubuğundan dene.

- [ ] **Step 4: Commit ve dur.**

```bash
git add README.md kanban-spec.md
git commit -m "docs: filter, search and TIMEZONE"
```

Aşama 1 burada biter. Kullanıcıya rapor ver ve dur. Aşama 2'nin (swimlane) planı, kullanıcı onayladıktan sonra yazılır.
