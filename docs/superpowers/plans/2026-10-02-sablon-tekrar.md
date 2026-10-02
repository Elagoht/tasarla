# Aşama 4 — Kart şablonları ve tekrar: Uygulama Planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:**
- Board ayarlarında kart şablonları tanımlanabilir. Şablon şunları taşır: başlık, açıklama, öncelik, tahmin, atanan kişi, etiketler, kontrol listesi, hedef kolon ve "son tarih: N gün sonra".
- Board'da "şablondan kart aç" seçeneği olur.
- Bir şablona zamanlama verilebilir (günlük, haftalık ya da aylık). Bir worker zamanı gelen şablondan kartı kendiliğinden açar.
- Kurallar reddederse çalışma `failed` olarak kaydedilir ve bildirim gider.

**Architecture:**
- **Store:** şablonların CRUD işlemleri, şablondan kart açma ve "bir zamanlanmış anı çalıştır". Kart açma bugünkü `createCard` yolundan geçer (kilit, kurallar, activity); alanlar aynı transaction'da doldurulur.
- **Zamanlama:** `internal/schedule` saf Go'dur, en son zamanlanmış anı hesaplar.
- **Worker:** `notify.Recurrer`, var olan `every()` döngüsüyle 5 dakikada bir çalışır. Her anı `template_runs` tablosuna `ON CONFLICT DO NOTHING` ile yazarak yalnız bir kez işler.
- **Arayüz:** board ayarlarında bir "Şablonlar" sekmesi, board'daki kart ekleme formunda bir şablon seçimi.

**Tech Stack:** Go 1.26, collage v0.39.2, PostgreSQL 17, pgx v5.

**Spec:** `docs/superpowers/specs/2026-10-01-filtre-swimlane-ical-sablon-markdown-design.md` §6 (§6.4 `TIMEZONE` Aşama 1'de yapıldı: `config.Config.Location`, `handlers.loc`).

## Global Constraints

- `any` türü hiçbir yerde kullanılmaz.
- Kullanıcıya görünen her metin `locales/tr.json` ve `locales/en.json`'a birlikte eklenir.
- CSP değişmez: satır içi script ve `style="…"` yok.
- Şablonları yalnız board'u yönetebilenler düzenler (`managedBoardFor`, diğer ayar sekmeleri gibi).
- Şablon adı board içinde benzersizdir (`UNIQUE (board_id, name)`); en fazla 60 karakterdir. Kart başlığı en fazla 200, açıklama en fazla 10000 karakterdir (kartlardaki sınırlar).
- `due_in_days`: boş ya da 0–365.
- Zamanlama türleri ve alanları:
  - türler `daily`, `weekly` (haftanın günleri, pazartesi = bit 0) ve `monthly` (ayın günü 1–31; kısa aylarda ayın son günü);
  - saat `HH:MM`, varsayılan 09:00;
  - saat dilimi `TIMEZONE`, yani `config.Location`.
- Yalnız en son zamanlanmış an işlenir, kaçırılanlar geriye doldurulmaz. `schedule_since`'ten önceki anlar işlenmez. Aynı an iki kez işlenmez.
- Zamanlanmış kartın `created_by` değeri şablonun `updated_by`'ıdır.
  - Bu kişi artık takımda değilse kart açılmaz. Run `failed` olarak kaydedilir ve takımın lead'lerine `template_failed` bildirimi gider.
  - Kural reddi ya da hedef kolonun olmaması da `failed` sayılır. Bildirim `updated_by`'a, o takımda değilse lead'lere gider.
- Board arşivliyse zamanlanmış şablon atlanır ve run kaydı yazılmaz.
- Atanan kişi takımda değilse ya da kişi WIP'ini aşıyorsa kart atanmamış açılır ve elle açmada formda bir uyarı gösterilir.
- Commit mesajları semantik ve İngilizcedir; biçim konu satırı, boş satır, `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.
- Testler: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./... -race -count=1`.

## Kararlar

- **Bildirim türü:** `template_failed` yeni bir bildirim türüdür. Migration, `notifications.kind` ve `notification_prefs.kind` CHECK kısıtlarını genişletir. E-posta varsayılanı açıktır.
- **Bildirim içeriği:** bildirim bir karta bağlı değildir (`card_id` boş). Yükü şablonun adını (`CardTitle` alanında) ve board'un adını taşır; bildirim sayfası bağlantısız metin gösterir.
- **Hedef kolon:** elle açmada formda bir kolon seçildiyse o kolona açılır, seçilmediyse şablonun hedef kolonuna. Şablonun hedef kolonu yoksa ve formda da kolon seçilmediyse, kart board'un ilk kart açılabilen kolonuna açılır (`creatableColumns`).
- **`schedule_since`:** zamanlama alanlarından biri değişince `now()` yapılır. Böylece yeni ya da değiştirilmiş bir zamanlama geçmişteki anlar için kart açmaz.
- **Yaz saati geçişleri** (`internal/schedule`): var olmayan bir saat ileriye kayar (Go'nun `time.Date` davranışı). İki kez yaşanan saatte ilki kullanılır.
- **Recurrer'ın yeri:** `internal/notify`'da durur, çünkü bildirim göndermesi gerekir ve `Scheduler` de orada.

## Review Focus

1. **Eşzamanlı iki tick:** aynı an için tek kart açılmalı ve tek run kaydı olmalı. *(Task 4)*
2. **Ayın 31'i:** şubatta ayın son günü (28/29), nisanda 30'u kullanılmalı. Aynı ay içinde iki kez çalışmamalı. *(Task 2, Task 4)*
3. **Silinen hedef kolon:** şablon kalmalı, hedefi boşalmalı ve ayarlar sayfası uyarmalı. Zamanlamada run `failed` olmalı. *(Task 1, Task 4, Task 5)*
4. **Takımdan ayrılan şablon sahibi:** kart açılmamalı, lead'lere bildirim gitmeli. *(Task 4)*
5. **Silinen etiket:** şablondan da düşmeli; şablondan açılan kartta görünmemeli. *(Task 1, Task 3)*

## Dosya haritası

| Dosya | Sorumluluk |
| --- | --- |
| `internal/db/migrations/010_templates.sql` | Tablolar, bildirim türü |
| `internal/store/templates.go` (yeni) | `Template`, CRUD |
| `internal/store/template_cards.go` (yeni) | `CreateCardFromTemplate`, `ScheduledTemplates`, `RunTemplate` |
| `internal/store/templates_test.go` (yeni) | Store testleri |
| `internal/schedule/schedule.go`, `schedule_test.go` (yeni) | `Latest` |
| `internal/notify/recurrer.go`, `recurrer_test.go` (yeni) | Worker |
| `main.go` | Recurrer'ı başlatmak |
| `internal/store/notifications.go` | `NotifyTemplateFailed` |
| `internal/web/pages_board_settings.go`, `internal/web/templates_settings.go` (yeni) | Şablonlar sekmesi |
| `internal/web/pages_board.go` | Şablondan kart açma |
| `templates/pages/board_settings.html`, `templates/fragments/columns.html` | İşaretleme |
| `locales/*.json` | Metinler |

---

### Task 1: Migration 010 ve şablon CRUD

**Files:** Create `internal/db/migrations/010_templates.sql`, `internal/store/templates.go`, `internal/store/templates_test.go`; Modify `internal/store/notifications.go`

**Interfaces — Produces:**

```go
type Schedule struct {
	Kind     string // "", "daily", "weekly", "monthly"
	Weekdays uint8  // weekly: bit 0 Monday … bit 6 Sunday
	MonthDay int    // monthly: 1–31
	Hour     int    // 0–23
	Minute   int    // 0–59
}

type Template struct {
	ID          int64
	BoardID     int64
	Name        string
	Title       string
	Description string
	Priority    *int16
	Estimate    *float64
	AssigneeID  *int64
	ColumnID    *int64 // nil: the target column was deleted, or none was set
	DueInDays   *int
	LabelIDs    []int64
	Checklist   []string
	Schedule    Schedule
	ScheduleSince *time.Time
	UpdatedBy   int64
	UpdatedAt   time.Time
	LastRun     *TemplateRun // the latest run, if any
}

type TemplateRun struct {
	ScheduledFor time.Time
	Status       string // "created" | "failed"
	CardID       *int64
	Violations   []rules.Violation
}

type TemplateInput struct { // what the settings form saves
	Name, Title, Description string
	Priority   *int16
	Estimate   *float64
	AssigneeID *int64
	ColumnID   *int64
	DueInDays  *int
	LabelIDs   []int64
	Checklist  []string
	Schedule   Schedule
}

var ErrTemplateName = errors.New("store: a template of this board already has this name")

func (s *Store) Templates(ctx context.Context, boardID int64) ([]Template, error)        // by lower(name)
func (s *Store) Template(ctx context.Context, boardID, id int64) (Template, error)        // ErrNotFound
func (s *Store) CreateTemplate(ctx context.Context, boardID int64, in TemplateInput, actorID int64) (Template, error)
func (s *Store) UpdateTemplate(ctx context.Context, boardID, id int64, in TemplateInput, actorID int64) (Template, error)
func (s *Store) DeleteTemplate(ctx context.Context, boardID, id int64) error

const NotifyTemplateFailed = "template_failed"
```

- [ ] **Step 1: Migration.**

```sql
-- Card templates, their schedules and the runs a schedule made (spec
-- 2026-10-01 filtre… §6).
CREATE TABLE card_templates (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id          bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name              text NOT NULL CHECK (btrim(name) <> ''),
    title             text NOT NULL CHECK (btrim(title) <> ''),
    description       text NOT NULL DEFAULT '',
    priority          smallint CHECK (priority BETWEEN 1 AND 4),
    estimate          numeric CHECK (estimate >= 0),
    assignee_id       bigint REFERENCES users (id) ON DELETE SET NULL,
    column_id         bigint REFERENCES columns (id) ON DELETE SET NULL,
    due_in_days       int CHECK (due_in_days BETWEEN 0 AND 365),
    schedule_kind     text NOT NULL DEFAULT '' CHECK (schedule_kind IN ('', 'daily', 'weekly', 'monthly')),
    schedule_weekdays smallint NOT NULL DEFAULT 0 CHECK (schedule_weekdays BETWEEN 0 AND 127),
    schedule_monthday smallint NOT NULL DEFAULT 1 CHECK (schedule_monthday BETWEEN 1 AND 31),
    schedule_time     time NOT NULL DEFAULT '09:00',
    schedule_since    timestamptz,
    updated_by        bigint NOT NULL REFERENCES users (id),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (board_id, name),
    CHECK (schedule_kind <> 'weekly' OR schedule_weekdays > 0)
);

CREATE TABLE card_template_labels (
    template_id bigint NOT NULL REFERENCES card_templates (id) ON DELETE CASCADE,
    label_id    bigint NOT NULL REFERENCES labels (id) ON DELETE CASCADE,
    PRIMARY KEY (template_id, label_id)
);

CREATE TABLE card_template_checklist (
    template_id bigint NOT NULL REFERENCES card_templates (id) ON DELETE CASCADE,
    position    int NOT NULL,
    text        text NOT NULL CHECK (btrim(text) <> ''),
    PRIMARY KEY (template_id, position)
);

CREATE TABLE template_runs (
    template_id   bigint NOT NULL REFERENCES card_templates (id) ON DELETE CASCADE,
    scheduled_for timestamptz NOT NULL,
    status        text NOT NULL CHECK (status IN ('created', 'failed')),
    card_id       bigint REFERENCES cards (id) ON DELETE SET NULL,
    violations    jsonb NOT NULL DEFAULT '[]',
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (template_id, scheduled_for)
);

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('assigned', 'mentioned', 'commented', 'due_soon', 'overdue', 'unblocked', 'template_failed'));
ALTER TABLE notification_prefs DROP CONSTRAINT notification_prefs_kind_check;
ALTER TABLE notification_prefs ADD CONSTRAINT notification_prefs_kind_check
    CHECK (kind IN ('assigned', 'mentioned', 'commented', 'due_soon', 'overdue', 'unblocked', 'template_failed'));
```

Kısıt adlarını `\d notifications` ile doğrula (PostgreSQL varsayılanı `<tablo>_<sütun>_check`). Farklıysa gerçek adı kullan.

`notifications.go`'ya ekle: `NotifyTemplateFailed = "template_failed"`; `NotifyKinds`'ın sonuna ekle; `emailByDefault[NotifyTemplateFailed] = true`.

- [ ] **Step 2: Başarısız testleri yaz** (`templates_test.go`):

```go
package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"kanban/internal/store"
)

func templateInput(f boardFixture, name string) store.TemplateInput {
	p, d := int16(3), 2
	col := f.cols[1].ID
	return store.TemplateInput{Name: name, Title: "Haftalık rapor", Description: "Ne yapıldı?", Priority: &p,
		AssigneeID: &f.member.ID, ColumnID: &col, DueInDays: &d, Checklist: []string{"Topla", "Yaz"},
		Schedule: store.Schedule{Kind: "weekly", Weekdays: 1, Hour: 9}}
}

func TestTemplateCRUD(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	label, _ := f.s.CreateLabel(ctx, f.board.ID, "rapor", "#228be6")
	in := templateInput(f, "Rapor")
	in.LabelIDs = []int64{label.ID}
	tpl, err := f.s.CreateTemplate(ctx, f.board.ID, in, f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.s.Template(ctx, f.board.ID, tpl.ID)
	if err != nil || got.Title != "Haftalık rapor" || !slices.Equal(got.Checklist, []string{"Topla", "Yaz"}) ||
		!slices.Equal(got.LabelIDs, []int64{label.ID}) || got.Schedule.Weekdays != 1 || got.ScheduleSince == nil || got.UpdatedBy != f.lead.ID {
		t.Fatalf("template = %+v, %v", got, err)
	}
	if _, err := f.s.CreateTemplate(ctx, f.board.ID, templateInput(f, "Rapor"), f.lead.ID); !errors.Is(err, store.ErrTemplateName) {
		t.Errorf("duplicate name = %v", err)
	}
	// An edit that leaves the schedule alone keeps schedule_since.
	in.Title = "Rapor (güncel)"
	upd, err := f.s.UpdateTemplate(ctx, f.board.ID, tpl.ID, in, f.member.ID)
	if err != nil || upd.Title != "Rapor (güncel)" || !upd.ScheduleSince.Equal(*got.ScheduleSince) || upd.UpdatedBy != f.member.ID {
		t.Fatalf("update = %+v, %v", upd, err)
	}
	if err := f.s.DeleteLabel(ctx, f.board.ID, label.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteColumn(ctx, f.board.ID, f.cols[1].ID); err != nil {
		t.Fatal(err)
	}
	after, _ := f.s.Template(ctx, f.board.ID, tpl.ID)
	if len(after.LabelIDs) != 0 || after.ColumnID != nil {
		t.Errorf("deleted label/column kept: %+v", after)
	}
	if err := f.s.DeleteTemplate(ctx, f.board.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Template(ctx, f.board.ID, tpl.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleted template = %v", err)
	}
}
```

`DeleteColumn` kartı olan bir kolonu reddedebilir (`columns.go:122`); fixture'da `cols[1]` boştur. Yine de reddederse rapora yaz.

- [ ] **Step 3: Çalıştır, başarısız olduğunu gör.**

- [ ] **Step 4: `templates.go`'yu yaz.**
  - `CreateTemplate` ve `UpdateTemplate` tek bir transaction'da şunları yapar: `card_templates` satırını yazar (`updated_by = actorID`, `updated_at = now()`), sonra etiketleri ve kontrol listesini sil-yeniden-yaz biçiminde yazar.
  - `schedule_since`:
    - Create'te `CASE WHEN kind <> '' THEN now() END`.
    - Update'te zamanlama alanlarından biri değiştiyse ya da önceki `schedule_since` boşsa ve yeni tür doluysa `now()`; tür boşsa `NULL`; aksi halde eski değer kalır.
  - Etiket id'leri aynı board'un etiketleri değilse yok sayılır (`INSERT … SELECT … FROM labels WHERE board_id = $1 AND id = ANY($2)`).
  - Hedef kolon başka bir board'unsa `ErrNotFound` döner.
  - Benzersiz ad ihlali (`isUniqueViolation`, store'da var olan yardımcı) `ErrTemplateName` döner.
  - `Templates` her şablonun son run'ını da doldurur: `SELECT DISTINCT ON (template_id) … ORDER BY template_id, scheduled_for DESC`. `violations` JSON'u `[]rules.Violation`'a açılır.
  - `schedule_time`, pgx'te `pgtype.Time` olarak okunur; `Hour` ve `Minute`'a çevir. Yazarken `fmt.Sprintf("%02d:%02d", h, m)` ve `$n::time` kullan.

- [ ] **Step 5: Çalıştır.** `go test ./internal/store -race -count=1` çalıştırılır.
- [ ] **Step 6: Commit.** `feat(store): card templates`

---

### Task 2: `internal/schedule`

**Files:** Create `internal/schedule/schedule.go`, `internal/schedule/schedule_test.go`

**Interfaces — Produces:**

```go
package schedule

// Spec is a schedule: its kind, days and time of day.
type Spec struct {
	Kind     string // "daily", "weekly", "monthly"
	Weekdays uint8  // bit 0 Monday … bit 6 Sunday
	MonthDay int    // 1–31; a shorter month uses its last day
	Hour     int
	Minute   int
}

// Latest is the last moment at or before now that s names, in loc; zero when
// s names none (an unknown kind, or weekly with no day).
func Latest(s Spec, loc *time.Location, now time.Time) time.Time
```

`store.Schedule` ile alanları aynıdır. Store kendi tipini tutar; worker `schedule.Spec(t.Schedule)` ile çevirir. Alan sırası ve tipleri aynı olduğu için bu dönüşüm derlenir.

- [ ] **Step 1: Başarısız testleri yaz.**

```go
package schedule

import (
	"testing"
	"time"
)

func TestLatest(t *testing.T) {
	ist, _ := time.LoadLocation("Europe/Istanbul")
	ber, _ := time.LoadLocation("Europe/Berlin")
	at := func(loc *time.Location, s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := []struct {
		name string
		spec Spec
		loc  *time.Location
		now  string
		want string // "" for zero
	}{
		{"daily before the time", Spec{Kind: "daily", Hour: 9}, ist, "2026-10-02 08:59", "2026-10-01 09:00"},
		{"daily at the time", Spec{Kind: "daily", Hour: 9}, ist, "2026-10-02 09:00", "2026-10-02 09:00"},
		{"weekly mon+thu on fri", Spec{Kind: "weekly", Weekdays: 1 | 1<<3, Hour: 9}, ist, "2026-10-02 12:00", "2026-10-01 09:00"},
		{"weekly no day", Spec{Kind: "weekly", Hour: 9}, ist, "2026-10-02 12:00", ""},
		{"monthly 31 in february", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2027-03-10 12:00", "2027-02-28 09:00"},
		{"monthly 31 leap february", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2028-03-10 12:00", "2028-02-29 09:00"},
		{"monthly 31 in april", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2026-04-30 10:00", "2026-04-30 09:00"},
		{"monthly before this month's day", Spec{Kind: "monthly", MonthDay: 15, Hour: 9}, ist, "2026-10-02 12:00", "2026-09-15 09:00"},
		{"dst gap moves forward", Spec{Kind: "daily", Hour: 2, Minute: 30}, ber, "2026-03-29 12:00", "2026-03-29 03:30"},
		{"unknown kind", Spec{Kind: "hourly"}, ist, "2026-10-02 12:00", ""},
	}
	for _, c := range cases {
		got := Latest(c.spec, c.loc, at(c.loc, c.now))
		if c.want == "" {
			if !got.IsZero() {
				t.Errorf("%s: got %v, want zero", c.name, got)
			}
			continue
		}
		if want := at(c.loc, c.want); !got.Equal(want) {
			t.Errorf("%s: got %v, want %v", c.name, got.In(c.loc), want)
		}
	}
}
```

"dst gap" satırı: Go, var olmayan 02:30'u `time.Date` ile 03:30'a çevirir. Bu, 02:30'un `now`'dan önce kaldığı bir günde olur. Test bu davranışı sabitler.

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.**

- [ ] **Step 3: `schedule.go`'yu yaz.**

```go
// Package schedule finds the moments a card template's schedule names.
package schedule

import "time"

func Latest(s Spec, loc *time.Location, now time.Time) time.Time {
	now = now.In(loc)
	y, m, d := now.Date()
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, s.Hour, s.Minute, 0, 0, loc) }
	switch s.Kind {
	case "daily":
		for i := 0; i < 2; i++ {
			if t := at(y, m, d-i); !t.After(now) {
				return t
			}
		}
	case "weekly":
		if s.Weekdays&0x7f == 0 {
			return time.Time{}
		}
		for i := 0; i < 8; i++ {
			t := at(y, m, d-i)
			bit := (int(t.Weekday()) + 6) % 7
			if s.Weekdays&(1<<bit) != 0 && !t.After(now) {
				return t
			}
		}
	case "monthly":
		for i := 0; i < 2; i++ {
			first := time.Date(y, m-time.Month(i), 1, 0, 0, 0, 0, loc)
			last := first.AddDate(0, 1, -1).Day()
			t := at(first.Year(), first.Month(), min(s.MonthDay, last))
			if !t.After(now) {
				return t
			}
		}
	}
	return time.Time{}
}
```

- [ ] **Step 4: Çalıştır.** `go test ./internal/schedule -race -count=1` çalıştırılır.
- [ ] **Step 5: Commit.** `feat(schedule): the latest moment a schedule names`

---

### Task 3: Şablondan kart açma (store)

**Files:** Create `internal/store/template_cards.go`; Test `internal/store/templates_test.go`

**Interfaces — Produces:**

```go
// FromTemplate is how a card made from a template came out.
type FromTemplate struct {
	Card            Card
	AssigneeDropped bool // the template's assignee is not in the team, or over their WIP
}

// CreateCardFromTemplate makes a card from a template in columnID (0: the
// template's column, or the board's first creatable column). Making it is
// entering the column: its rules apply (a *RuleError). The template's fields,
// labels and checklist are written in the same transaction; the due date is
// today plus DueInDays, today taken in loc.
func (s *Store) CreateCardFromTemplate(ctx context.Context, boardID, templateID, columnID, createdBy int64, loc *time.Location, now time.Time) (FromTemplate, error)
```

- [ ] **Step 1: Başarısız testleri yaz.**
  - `TestCardFromTemplate`:
    - Şablon (Task 1'deki `templateInput`, etiketli) ile `CreateCardFromTemplate(…, 0, f.lead.ID, time.UTC, now)` çağrılır. Kartın kolonu `cols[1]`, başlığı, açıklaması, önceliği, atanan kişisi `member` olmalı. Son tarih `now`'ın tarihi + 2 gün olmalı. Etiketler (`CardLabels`) ve kontrol listesi (`checklist_items`) iki madde olmalı, `AssigneeDropped` false olmalı.
    - `cols[0]` verilince kart oraya açılmalı.
  - `TestCardFromTemplateDropsTheAssignee`:
    - `SetPersonWIP(1, cols[1])` ve üyeye `cols[1]`'de bir kart verilir. Şablondan kart açılınca kart atanmamış olmalı ve `AssigneeDropped` true olmalı.
    - Ayrıca `RemoveMember(member)` sonrası da atanmamış olmalı.
  - `TestCardFromTemplateRefused`:
    - `cols[1]`'e `has_due_date` giriş koşulu konur ve şablonun `DueInDays`'ı nil yapılır. `*RuleError` dönmeli ve kart açılmamalı (`BoardCards` boş).
    - `DueInDays` doluyken aynı koşul geçmeli, çünkü kurallar birleşik son durumu görür.

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.**

- [ ] **Step 3: Uygula.**
  1. Transaction açılır, `lockBoard` çağrılır.
  2. Şablon okunur (`Template` sorgusunu `tx` ile çalıştıran iç bir yardımcıyla).
  3. Hedef kolon belirlenir: `columnID`, sonra `ColumnID`, sonra board'un `allow_create` işaretli ilk kolonu, o da yoksa ilk kolon. Bu, web'deki `creatableColumns`'ın store karşılığıdır.
  4. `next := Card{Title, Description, AssigneeID, Estimate, Priority, DueDate}` kurulur; son tarih `now.In(loc)`'un tarihine `DueInDays` eklenerek bulunur.
  5. Atanan kişi kontrolü:
     - Kişi takımda değilse `next.AssigneeID = nil` ve `dropped = true`.
     - Kişi takımdaysa `snap := loadSnapshot(ctx, tx, boardID, next)` ile snapshot kurulur. `rules.EvaluateAssign(col, snap)` ihlal veriyorsa `next.AssigneeID = nil` ve `dropped = true`.
  6. Oluşturma kuralları **son durumla** değerlendirilir: `snap := loadSnapshot(ctx, tx, boardID, next)` ve `rules.EvaluateCreate(col, snap)`. Etiketler snapshot'a elle verilir: `snap.Card.LabelIDs = template labels`.
  7. Kart `INSERT` edilir. Bugünkü `createCard` gövdesinden INSERT, done kolonu ve activity kısmını küçük bir `insertCard(ctx, tx, boardID, col, next, createdBy)` yardımcısına çıkar ve `createCard` da onu kullansın. Bu yardımcı bütün alanları yazar.
  8. Etiketler ve kontrol listesi yazılır.
  9. Commit.

  `createCard`'ın kendi `EvaluateCreate`'i boş bir kartla çalışıyordu. Bu davranış değişmez; şablon yolu aynı değerlendirmeyi son kartla yapar.

- [ ] **Step 4: Çalıştır.** Bütün store testleri.
- [ ] **Step 5: Commit.** `feat(store): a card made from a template`

---

### Task 4: Zamanlanmış çalıştırma ve Recurrer

**Files:** Modify `internal/store/template_cards.go`; Create `internal/notify/recurrer.go`, `internal/notify/recurrer_test.go`; Modify `main.go`

**Interfaces — Produces:**

```go
// ScheduledTemplate is a template with a schedule, on a board that is not archived.
type ScheduledTemplate struct {
	Template Template
	TeamID   int64
	BoardName string
}
func (s *Store) ScheduledTemplates(ctx context.Context) ([]ScheduledTemplate, error)

// RunOutcome is what one scheduled moment did.
type RunOutcome struct {
	Ran        bool // false: this moment was already run
	Card       *Card
	Violations []rules.Violation
	OwnerGone  bool // updated_by is no longer in the team
	NoColumn   bool // the template has no target column
}

// RunTemplate runs a template's moment `at` once: it records the run (a second
// call for the same moment does nothing) and makes the card, or records why
// not, in one transaction.
func (s *Store) RunTemplate(ctx context.Context, st ScheduledTemplate, at time.Time, loc *time.Location) (RunOutcome, error)

func (s *Store) TeamLeads(ctx context.Context, teamID int64) ([]int64, error)
```

```go
package notify

// Recurrer makes the cards that card templates' schedules call for.
type Recurrer struct {
	Store    *store.Store
	Notifier *Notifier
	Location *time.Location
	Interval time.Duration // default 5m
	Now      func() time.Time
}
func (r Recurrer) Tick(ctx context.Context) error
func (r Recurrer) Run(ctx context.Context)
```

- [ ] **Step 1: Başarısız testleri yaz** (`recurrer_test.go`). Var olan notify testlerinin store ve notifier kurulumunu kullan (`notify_test.go`'ya bak).
  - **Tek kart:** haftalık pazartesi 09:00 zamanlaması olan bir şablon. `schedule_since`'i geçmişe çekmek için testte doğrudan bir `UPDATE` yap ya da store'a test için bir yardımcı açma; tercih edilen yol `pool.Exec`'tir.
    - `Now = 2026-10-05 10:00` (pazartesi) ile `Tick` iki kez çalıştırılır. Tek kart açılmalı ve `template_runs`'ta tek satır olmalı.
    - Ardından iki `Tick`'i aynı anda (iki goroutine) çalıştır. Toplamda yine tek kart olmalı.
  - **Kaçırılan anlar:** `Now` üç hafta ileri (`2026-10-26 10:00`) alınır. Toplam iki kart olmalı (yalnız en son an).
  - **Ret:** hedef kolona `has_due_date` koşulu konur, `DueInDays` nil yapılır. Bir sonraki an `failed` kaydı ve `updated_by`'a bir `template_failed` bildirimi üretmeli.
  - **Sahibi ayrılmış:** `updated_by` takımdan çıkarılır. `failed` kaydı ve lead'lere bildirim olmalı.
  - **Arşivli board:** run kaydı yazılmamalı.
  - **Yeni zamanlama:** `schedule_since = now` olduğu için ilk tick'te geçmiş an işlenmez.

- [ ] **Step 2: Çalıştır, başarısız olduğunu gör.**

- [ ] **Step 3: Uygula.**
  - `RunTemplate` tek transaction'da şunları yapar:
    1. `INSERT INTO template_runs (template_id, scheduled_for, status) VALUES ($1, $2, 'failed') ON CONFLICT DO NOTHING`. Eklenen satır yoksa `Ran: false` döner.
    2. Sahip takımda değilse `OwnerGone` olur.
    3. Hedef kolon yoksa `NoColumn` olur.
    4. Bunlar yoksa `CreateCardFromTemplate`'in iç yardımcısıyla (`tx` alan sürümüyle) kart açılır.
       - `*RuleError` → violations.
       - Başarı → `UPDATE template_runs SET status = 'created', card_id = $3`.
    5. Hata durumunda `violations` JSON olarak yazılır.
    6. Commit.

    Store'un iç yardımcıları `tx` almalı; Task 3'teki fonksiyonu `createFromTemplate(ctx, tx, …)` iç fonksiyonuna böl. `NoColumn` için `ColumnID == nil` kontrolü, `columnID`'yi boş geçerek board'un ilk kolonuna açılmayı engeller: zamanlanmış çalışmada hedef kolon zorunludur.
  - `ScheduledTemplates`: `schedule_kind <> ''`, `schedule_since IS NOT NULL` olan ve board'u arşivlenmemiş şablonlar.
  - `TeamLeads`: `SELECT user_id FROM team_members WHERE team_id = $1 AND role = 'lead'`.
  - `Recurrer.Tick`, her şablon için:
    1. `at := schedule.Latest(schedule.Spec(st.Template.Schedule), r.Location, now())`. `at.IsZero() || at.Before(*st.Template.ScheduleSince)` ise atla.
    2. `out, err := r.Store.RunTemplate(ctx, st, at, r.Location)`. Hata loglanır ve devam edilir.
    3. `out.Ran` ise ve kart açılmadıysa bildirim gönderilir:
       - alıcılar: `OwnerGone` ise `TeamLeads`, değilse `[updated_by]`;
       - olay: `Event{Kind: store.NotifyTemplateFailed, Card: store.Card{BoardID: st.Template.BoardID, Title: st.Template.Name}, BoardName: st.BoardName, DedupeKey: "template_failed:" + id + ":" + at.Format(time.RFC3339) + ":" + recipient}`.
  - `notify.go`'nun `emit`'i `e.Card.ID == 0` iken `CardID` vermemeli. Bunu kontrol et; şu an `e.Card.ID != 0` ise kart bağlantısı kuruluyor. `CreateNotification`'a giden `CardID` de koşullu olmalı.
  - `main.go`: `recurrer := notify.Recurrer{Store: st, Notifier: app.Notifier, Location: cfg.Location}`, `wg.Add(3)` ve bir goroutine'de `recurrer.Run(background)`.

- [ ] **Step 4: Çalıştır.** Tam test koşusu.
- [ ] **Step 5: Commit.** `feat: scheduled templates make their cards, once per moment`

---

### Task 5: Ayarlarda "Şablonlar" sekmesi

**Files:** Create `internal/web/templates_settings.go`; Modify `internal/web/pages_board_settings.go`, `templates/pages/board_settings.html`, `locales/*.json`; Test `internal/web/templates_test.go`

**Interfaces:**
- `settingsTabs`'a `"templates"` eklenir (`rules`'tan sonra).
- `settingsView`'a `Templates []templateView` ve `EditTemplate *templateForm` eklenir. Düzenleme `?tab=templates&edit={id}` adresiyle açılır, yeni şablon `?tab=templates&edit=new` ile.
- İşlemler: `op=template_save` (`template_id` boşsa oluşturur, doluysa günceller) ve `op=template_delete` (onaylı; `h.confirmFirst` var olan akışı kullanır).

- [ ] **Step 1: Başarısız testleri yaz.**
  - **Lead:** `?tab=templates` sayfası şablon listesini ve "Yeni şablon" bağlantısını gösterir. Tam bir form gönderilir: ad, başlık, açıklama, öncelik, atanan kişi, kolon, `due_in_days`, etiket checkbox'ları, satır satır kontrol listesi (textarea, her satır bir madde), zamanlama türü, haftanın günleri checkbox'ları (`weekday` değerleri 0–6), ayın günü, saat. Yanıt `303` olur, ardından store'da şablon görünür.
  - **Üye:** aynı istek `403` alır.
  - **Hatalı değerler** 422 alır ve form, alanın hatasıyla yeniden gösterilir: boş ad, aynı ad, `due_in_days=400`, `weekly` türünde gün seçilmemesi, saat `25:00`.
  - **Silme:** onay sayfasından sonra şablon silinir.
  - **Hedef kolonu silinmiş şablon:** listede `settings.template_no_column` uyarısıyla görünür.
  - **Son çalışma:** `failed` ise listede `settings.template_failed` metni ve ihlallerin kullanıcı dilindeki karşılıkları görünür (`violationMessages`'ın mantığı; bir `[]rules.Violation`'ı mesajlara çeviren küçük bir yardımcıyla).

- [ ] **Step 2: Uygula.**
  - `templateForm`, formun ham değerlerini ve alan hatalarını tutar. `validate.Form(rc)` ile okunur ve kurallar `v.Field(...)` ile uygulanır; mevcut ayar işlemleri örnek alınır.
  - Haftanın günleri `v.Values("weekday")` gibi çoklu bir okuma ister. `validate.Validator`'ın çoklu değer okuyan bir yöntemi olup olmadığına bak. Yoksa `rc.Request.PostForm["weekday"]`'i kullan; `badText`, `validate.Form` çağrısından sonra form ayrıştırılmış olur.
  - Kontrol listesi: textarea satırlara bölünür, boş satırlar atılır. En fazla 50 madde, her biri en fazla 200 karakter.
  - Şablonun atanan kişi seçenekleri takım üyeleridir, kolon seçenekleri board'un kolonlarıdır.
  - Şablon işaretlemesi diğer sekmelerin bileşenlerini kullanır (`panel`, `field`, `list-rows`, `chip`, `swatch` yerine checkbox, `select`); yeni CSS ancak gerekirse `settings.css`'e eklenir.
  - Dil dosyaları (tr ve en birlikte):
    - `settings.tabs.templates` ("Şablonlar" / "Templates")
    - `settings.template_*`: `new`, `name`, `title`, `description`, `priority`, `assignee`, `column`, `due_in_days` ("Son tarih: açılıştan N gün sonra"), `labels`, `checklist` ("Kontrol listesi (her satır bir madde)"), `schedule` ("Tekrar"), `schedule_none` ("Tekrarlama"), `schedule_daily`, `schedule_weekly`, `schedule_monthly`, `weekdays`, `monthday`, `time`, `save`, `delete`, `no_column` ("Hedef kolon silinmiş; şablon zamanlamayla kart açamaz."), `failed` ("Son çalışma başarısız:"), `created` ("Son çalışma: kart açıldı"), `name_exists`, `invalid_due`, `invalid_time`, `no_weekday`, `empty`
    - `weekdays.0`…`weekdays.6` ("Pzt"…"Paz" / "Mon"…"Sun")
    - `confirm.template_delete`

- [ ] **Step 3: Çalıştır.** Tam test koşusu.
- [ ] **Step 4: Commit.** `feat(ui): card templates in the board settings`

---

### Task 6: Board'da şablondan kart açma ve bildirim metni

**Files:** Modify `internal/web/pages_board.go`, `templates/fragments/columns.html`, `templates/pages/notifications.html` (gerekirse), `locales/*.json`; Test `internal/web/templates_test.go`

- [ ] **Step 1: Başarısız testleri yaz.**
  - **Board'da seçim:** board'da bir şablon varken her "kart ekle" formunda bir "Şablondan" seçimi bulunur (`name="template"`). Şablon seçili ve başlık boş olarak gönderilince kart şablondan açılır. Başlık şablonun başlığı olur ve kart formun kolonuna gelir.
  - **Atanan kişi düşerse:** yanıtta `board.template_assignee_dropped` uyarısı görünür, kart yine açılır.
  - **Şablon seçilmeden:** başlık hâlâ zorunludur.
  - **Başka board'un şablon id'si:** `400` döner.
  - **Bildirim:** `template_failed` bildirimi sayfada Türkçe ve İngilizce cümleyle, kart bağlantısı olmadan görünür. Ayarlardaki e-posta tercihleri listesinde de görünür.

- [ ] **Step 2: Uygula.**
  - `createCard`, `v.Value("template")` doluysa başlık zorunluluğunu atlar ve `CreateCardFromTemplate`'i çağırır. Kolon `column` alanından, `today` ve `loc` `h.loc`'tan gelir. `RuleError` ve 422 yolu bugünkü gibidir.
  - `AssigneeDropped` ise `noticeKey`'e uyarı eklenir ve 200 ile kolonlar döner (JS yolu). JS'siz yolda flash ile bildirilir.
  - `columnsView`'a `Templates []store.Template` eklenir (yalnız ad ve id gerekir). `loadColumns` şablonları yükler. Push'ta her render bunu tekrar sorgular; sorgu küçük olduğu için kabul edilmiştir.
  - `columns.html`'de add-card formuna:

    ```html
    {{if $.Templates}}<select class="select" name="template" aria-label="{{t "board.from_template"}}"><option value="">{{t "board.no_template"}}</option>{{range $.Templates}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select>{{end}}
    ```

    Textarea'nın `required` özniteliği şablon seçiliyken engel olur. Bunun için textarea'dan `required`'ı kaldır (sunucu zaten zorunlu kılıyor). `board.js`'in `addCard` akışı değişmez.
    - Şeritli görünümde de add-card formu başlık hücrelerindedir (`columns.html`'in iki dalı); ikisine de ekle.
  - Dil dosyaları:
    - `board.from_template` ("Şablondan" / "From a template"), `board.no_template` ("Şablonsuz" / "No template"), `board.template_assignee_dropped` ("Şablonun atanan kişisi kartı alamadı; kart atanmamış açıldı." / "The template's assignee could not take the card; it was made unassigned.")
    - `notify.template_failed` ("{board} board'unda {card} şablonu kart açamadı." / "The template {card} on {board} could not make its card.")
    - `notify.kinds.template_failed` ("Zamanlanmış bir şablon kart açamadığında" / "When a scheduled template cannot make its card")
    - E-posta konu ve gövdesi için `mail.*` altında hangi anahtarlar kullanılıyorsa (`internal/notify/notify.go`'daki `render`'a bak), yeni tür için de onları ekle.

- [ ] **Step 3: Çalıştır.** Tam test koşusu.
- [ ] **Step 4: Commit.** `feat: make a card from a template on the board`

---

### Task 7: Belgeler

- [ ] README "Neler var" → "Kartlar" maddesinin altına: "**Şablonlar:** board ayarlarında kart şablonları; şablondan kart açma; günlük, haftalık ya da aylık zamanlamayla kartı kendiliğinden açma. Kurallar reddederse şablon sahibine bildirim gider."
- [ ] `kanban-spec.md` §13 cümlesine "şablonlar ve tekrarlayan kartlar"ı ekle.
- [ ] `gofmt -l .` boş, `go vet ./...` temiz, tam test koşusu PASS.
- [ ] Commit: `docs: card templates`
