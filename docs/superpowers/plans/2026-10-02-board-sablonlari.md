# Hazır board şablonları: Uygulama Planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Yeni bir board oluştururken yedi hazır şablondan biri seçilebilir: Basit, Scrum, Hata takibi, Yazılım geliştirme, İçerik takvimi, İşe alım, Destek talepleri. Şablon kolonları, WIP limitlerini, etiketleri, kart şablonlarını (tekrarlar dahil) ve kuralları tek seferde kurar. Hangi grupların aktarılacağı onay kutularıyla seçilir.

**Architecture:**
- `internal/blueprint`, kataloğu dilden bağımsız Go yapıları olarak tutar. `Render` bir şablonu verilen dilde somut bir `store.BoardPlan`'a çevirir, `Trim` kullanıcının seçimlerini uygular.
- `store.CreateBoardFromPlan` planı tek bir veritabanı işleminde kurar. Var olan `CreateBoard` bu yolun ince bir sarmalayıcısı olur.
- Takım sayfasındaki form, şablon radyo kartları ve aktarılacakların onay kutularıyla genişler. Küçük bir JS, seçilen şablonda olmayan grupları devre dışı bırakır.

**Tech Stack:** Go 1.26, collage v0.39.2, collage-i18n (Strict), collage-validate, PostgreSQL 17 + pgx v5, düz CSS, ES modülleri.

**Spec:** `docs/superpowers/specs/2026-10-02-board-sablonlari-design.md`

## Global Constraints

- Go'da `any` türü yazılmaz: değişkende, alanda, parametrede ya da arayüz tanımında. pgx'in kendi `...any` parametrelerini çağırmak serbesttir, ama onları saran yeni bir arayüz tanımlanmaz.
- **Spec'ten sapma:** Bu yüzden etiket, kural ve WIP satırları `CreateBoardFromPlan` içinde doğrudan SQL ile yazılır. Ortak kod yalnız kart şablonu için ayrılır (`writeTemplate`, `pgx.Tx` alır). Etiket ve kural fonksiyonlarının tek mantığı ya renk küçültme ya da sahiplik denetimidir; yeni kurulan board'da ikisine de gerek yoktur.
- CSS'te px yoktur, her uzunluk rem'dir (1px = 0.0625rem).
- Renkler token'dan gelir (`var(--…)`). Yeni renk yazılmaz.
- Çeviri katalogları Strict'tir: `locales/tr.json` ile `locales/en.json` aynı anahtarları taşımalıdır. JSON 2 boşlukla girintilenir, `ensure_ascii` kapalıdır, sonda yeni satır vardır.
- Etiket renkleri yalnız şu paletten seçilir: `#e03131`, `#f08c00`, `#2f9e44`, `#1971c2`, `#7048e8`, `#c2255c`, `#0c8599`, `#495057`.
- Hiçbir şablonda atanan kişi ya da board rolü yoktur. Taşıma izinlerinin kimlerin olabileceği yalnız şunlardır: `team_lead`, `assignee`, `any_member`.
- Her şablonun kart şablonları ilk kolonu (indeks 0) hedefler. O kolonda kart açılabilir (`allow_create`) ve oraya giriş kuralı konmaz.
- Board, kuran kişinin dilinde kaydedilir.
- Commit mesajları semantik ve İngilizcedir. Biçim: konu satırı, boş satır, gövde, boş satır ve şu satır: `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`. Push yapılmaz.
- Testler: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./... -race -count=1`. Bu değişken tanımlı değilse veritabanı testleri **SKIP** olur ve paket yine "ok" der. "PASS" yalnız SKIP'siz koşu için söylenir; `-v` çıktısında `SKIP` olmamalıdır.

## Review Focus

1. **İngilizce arayüzden kurulan board:** kolon, etiket ve kart şablonu adları İngilizce olmalı, Türkçe ya da anahtar adı (`blueprints.…`) görünmemeli. *(Task 3: `TestCreateBoardFromBlueprintInEnglish`)*
2. **Ad boş gönderilen form:** reddedilince seçilen şablon seçili kalmalı, Basit'e dönmemeli. *(Task 3: `TestBoardFormKeepsTheChosenBlueprint`)*
3. **"Tekrarlar" işaretli ama "Kart şablonları" işaretsiz:** JS kapalıyken gönderilebilir; o durumda hiç kart şablonu ve zamanlama kurulmamalı. *(Task 2: `TestTrimRecurringNeedsTemplates`, Task 3: `TestBoardFormIncludes`)*
4. **Tekrarlı şablonun geçmişi:** Board kurulur kurulmaz geçmiş tarihler için toplu kart açılmamalı; zamanlama kurulum anından başlamalı. *(Task 2: `TestEveryBlueprintBuilds` → `ScheduleSince` kurulumdan sonra)*
5. **Şablonun kartı gerçekten açılabilmeli:** Her şablonun her kart şablonundan, kendi kuralları altında kart açılabilmeli; giriş kuralı ilk kolonu kilitlememeli. *(Task 2: `TestEveryBlueprintBuilds` → `CreateCardFromTemplate`)*

## Dosya haritası

| Dosya | Sorumluluk |
| --- | --- |
| `internal/store/board_plan.go` (yeni) | `BoardPlan` türleri ve `CreateBoardFromPlan` |
| `internal/store/boards.go` | `CreateBoard` → `CreateBoardFromPlan` sarmalayıcısı |
| `internal/store/templates.go` | `saveTemplate`'in gövdesi `writeTemplate(ctx, tx, …)`'e ayrılır |
| `internal/store/board_plan_test.go` (yeni) | Planın kurulduğu, kısıtlandığı ve geri alındığı testler |
| `internal/blueprint/blueprint.go` (yeni) | Türler, `All`, `Find`, `Default`, `Has`, `Counts`, `Options` |
| `internal/blueprint/catalog.go` (yeni) | Yedi şablonun tanımı |
| `internal/blueprint/plan.go` (yeni) | `Render`, `Trim`, `Plan` |
| `internal/blueprint/blueprint_test.go` (yeni) | Katalog ve `Trim` testleri; `locales/*.json`'u doğrudan okur |
| `internal/blueprint/blueprint_db_test.go` (yeni) | Her şablonu her dilde veritabanına kurar |
| `locales/tr.json`, `locales/en.json` | `blueprints.*` metinleri; `board.blueprint*`, `board.include*`; `board.default_columns` silinir |
| `internal/web/pages_teams.go` | `createBoard` şablonla kurar; `teamView.Blueprints` |
| `internal/web/blueprints.go` (yeni) | Takım sayfası için şablon kartı görünümü ve `include` ayrıştırma |
| `templates/pages/team.html` | Form: şablon radyo kartları ve aktarılacaklar |
| `static/css/pages.css` | `.blueprints`, `.blueprint`, `.includes` |
| `static/js/blueprint-form.js` (yeni) | Grupları devre dışı bırakma |
| `internal/web/boards_test.go` | Web testleri |

---

### Task 1: Store, planı tek işlemde kurar

**Files:**
- Create: `internal/store/board_plan.go`
- Modify: `internal/store/boards.go:51-72` (`CreateBoard`)
- Modify: `internal/store/templates.go:227-304` (`saveTemplate`)
- Test: `internal/store/board_plan_test.go`

**Interfaces:**
- Consumes: var olan `Schedule`, `TemplateInput`, `IsUniqueViolation`, `ErrTemplateName`, `Board`, `boardColumns`, `scanBoard`.
- Produces:

```go
type BoardPlan struct {
	Columns     []PlanColumn
	Labels      []PlanLabel
	Templates   []PlanTemplate
	PersonWIP   *int
	Permissions []PlanPermission
	Conditions  []PlanCondition
}
type PlanColumn struct {
	Name                          string
	WIP                           *int
	AllowCreate, Done, CountsPersonWIP bool
}
type PlanLabel struct{ Name, Color string }
type PlanTemplate struct {
	Name, Title, Description string
	Priority                 *int16
	DueInDays                *int
	ColumnIndex              int
	LabelIndexes             []int
	Checklist                []string
	Schedule                 Schedule
}
type PlanPermission struct {
	ToIndex   int
	FromIndex *int
	Subject   string
}
type PlanCondition struct {
	ColumnIndex  int
	Phase, Kind  string
}
func (s *Store) CreateBoardFromPlan(ctx context.Context, teamID int64, name string, plan BoardPlan, actorID int64) (Board, error)
func ColumnsPlan(names []string) BoardPlan // bugünkü CreateBoard kolon kuralları
```

- [ ] **Step 1: Write the failing tests**

`internal/store/board_plan_test.go`:

```go
package store_test

import (
	"context"
	"testing"
	"time"

	"kanban/internal/store"
)

func intp(n int) *int { return &n }

func fullPlan() store.BoardPlan {
	high := int16(3)
	return store.BoardPlan{
		Columns: []store.PlanColumn{
			{Name: "Backlog", AllowCreate: true},
			{Name: "Doing", WIP: intp(3), CountsPersonWIP: true},
			{Name: "Done", Done: true},
		},
		Labels: []store.PlanLabel{{Name: "Bug", Color: "#E03131"}, {Name: "Story", Color: "#1971c2"}},
		Templates: []store.PlanTemplate{{
			Name: "Bug report", Title: "New bug", Description: "**Steps**", Priority: &high,
			ColumnIndex: 0, LabelIndexes: []int{0}, Checklist: []string{"Reproduced", "Fixed"},
			Schedule: store.Schedule{Kind: "weekly", Weekdays: 1 << 4, Hour: 16},
		}},
		PersonWIP:   intp(2),
		Permissions: []store.PlanPermission{{ToIndex: 2, Subject: "team_lead"}},
		Conditions:  []store.PlanCondition{{ColumnIndex: 1, Phase: "enter", Kind: "has_assignee"}},
	}
}

func TestCreateBoardFromPlan(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	before := time.Now().Add(-time.Second)
	b, err := f.s.CreateBoardFromPlan(ctx, f.team.ID, "Bugs", fullPlan(), f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	cols, err := f.s.Columns(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 || cols[0].Name != "Backlog" || !cols[0].AllowCreate || cols[1].AllowCreate ||
		cols[1].WIPLimit == nil || *cols[1].WIPLimit != 3 || !cols[1].CountsPersonWIP || !cols[2].IsDone || cols[0].IsDone {
		t.Fatalf("columns = %+v", cols)
	}
	labels, _ := f.s.Labels(ctx, b.ID)
	if len(labels) != 2 || labels[0].Name != "Bug" || labels[0].Color != "#e03131" {
		t.Fatalf("labels = %+v", labels)
	}
	tpls, err := f.s.Templates(ctx, b.ID)
	if err != nil || len(tpls) != 1 {
		t.Fatalf("templates = %+v, %v", tpls, err)
	}
	tp := tpls[0]
	if tp.ColumnID == nil || *tp.ColumnID != cols[0].ID || len(tp.LabelIDs) != 1 || tp.LabelIDs[0] != labels[0].ID ||
		len(tp.Checklist) != 2 || tp.Priority == nil || *tp.Priority != 3 || tp.Schedule.Kind != "weekly" ||
		tp.ScheduleSince == nil || tp.ScheduleSince.Before(before) || tp.UpdatedBy != f.lead.ID {
		t.Fatalf("template = %+v", tp)
	}
	r, err := f.s.BoardRules(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Permissions) != 1 || r.Permissions[0].ToColumnID != cols[2].ID || r.Permissions[0].Subject != "team_lead" ||
		len(r.Conditions) != 1 || r.Conditions[0].ColumnID != cols[1].ID || r.Conditions[0].Kind != "has_assignee" {
		t.Fatalf("rules = %+v", r)
	}
	got, err := f.s.Board(ctx, b.ID)
	if err != nil || got.PersonWIPLimit == nil || *got.PersonWIPLimit != 2 {
		t.Fatalf("board = %+v, %v", got, err)
	}
}

// A plan that fails half way leaves nothing: no board, no columns.
func TestCreateBoardFromPlanRollsBack(t *testing.T) {
	f := newBoardFixture(t)
	ctx := context.Background()
	p := fullPlan()
	p.Conditions = append(p.Conditions, store.PlanCondition{ColumnIndex: 1, Phase: "enter", Kind: "nonsense"})
	if _, err := f.s.CreateBoardFromPlan(ctx, f.team.ID, "Broken", p, f.lead.ID); err == nil {
		t.Fatal("a plan with a bad condition was built")
	}
	boards, err := f.s.BoardsOfTeam(ctx, f.team.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range boards {
		if b.Name == "Broken" {
			t.Fatal("the failed board is left behind")
		}
	}
}

// An index past the plan's columns or labels is refused, not silently dropped.
func TestCreateBoardFromPlanRefusesBadIndexes(t *testing.T) {
	f := newBoardFixture(t)
	p := fullPlan()
	p.Permissions = []store.PlanPermission{{ToIndex: 9, Subject: "team_lead"}}
	if _, err := f.s.CreateBoardFromPlan(context.Background(), f.team.ID, "X", p, f.lead.ID); err == nil {
		t.Fatal("a permission into column 9 of 3 was built")
	}
}

func TestColumnsPlanKeepsTodaysDefaults(t *testing.T) {
	p := store.ColumnsPlan([]string{"A", "B", "C"})
	if len(p.Columns) != 3 || !p.Columns[0].AllowCreate || p.Columns[1].AllowCreate || !p.Columns[2].Done || p.Columns[0].Done {
		t.Fatalf("plan = %+v", p.Columns)
	}
	if one := store.ColumnsPlan([]string{"Only"}); one.Columns[0].Done {
		t.Fatal("a one-column board's only column is done")
	}
}
```

`Board.PersonWIPLimit *int` alanı var (`internal/store/boards.go:18`) ve `boardColumns` `person_wip_limit` sütununu döndürüyor. `Templates` de `UpdatedBy` ile `ScheduleSince` alanlarını dolduruyor (`templateColumns`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./internal/store -run 'TestCreateBoardFromPlan|TestColumnsPlan' -count=1`
Expected: derleme hatası (`undefined: store.BoardPlan`).

- [ ] **Step 3: Split `saveTemplate`**

`internal/store/templates.go` içinde `saveTemplate`'in `Begin` ile `Commit` arasındaki gövdesini, hiçbir satırını değiştirmeden yeni bir fonksiyona taşı:

```go
// writeTemplate is saveTemplate inside the caller's tx: it creates the
// template when id is 0, otherwise updates it, and returns its id.
func writeTemplate(ctx context.Context, tx pgx.Tx, boardID, id int64, in TemplateInput, actorID int64) (int64, error) {
	// …saveTemplate'in gövdesi: kolon sahipliği, INSERT/UPDATE, etiketler, kontrol listesi…
	return id, nil
}

// saveTemplate creates the template when id is 0, otherwise updates it.
func (s *Store) saveTemplate(ctx context.Context, boardID, id int64, in TemplateInput, actorID int64) (Template, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	id, err = writeTemplate(ctx, tx, boardID, id, in, actorID)
	if err != nil {
		return Template{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return s.Template(ctx, boardID, id)
}
```

Taşınan gövdede `return Template{}, X` dönüşleri `return 0, X` olur. `ErrTemplateName` eşlemesi `writeTemplate`'te kalır.

- [ ] **Step 4: Write `board_plan.go`**

```go
package store

import (
	"context"
	"fmt"
	"strings"
)

// BoardPlan is everything a new board is built with, in one transaction:
// the indexes in it point into its own Columns and Labels.
type BoardPlan struct {
	Columns     []PlanColumn
	Labels      []PlanLabel
	Templates   []PlanTemplate
	PersonWIP   *int
	Permissions []PlanPermission
	Conditions  []PlanCondition
}

// PlanColumn is a column of a planned board.
type PlanColumn struct {
	Name                               string
	WIP                                *int
	AllowCreate, Done, CountsPersonWIP bool
}

// PlanLabel is a label of a planned board; Color is "#rrggbb".
type PlanLabel struct{ Name, Color string }

// PlanTemplate is a card template of a planned board.
type PlanTemplate struct {
	Name, Title, Description string
	Priority                 *int16
	DueInDays                *int
	ColumnIndex              int
	LabelIndexes             []int
	Checklist                []string
	Schedule                 Schedule
}

// PlanPermission says who may move cards into a planned column.
type PlanPermission struct {
	ToIndex   int
	FromIndex *int
	Subject   string
}

// PlanCondition gates entering or leaving a planned column.
type PlanCondition struct {
	ColumnIndex int
	Phase, Kind string
}

// ColumnsPlan is a plan of columns alone, named in order: the first takes new
// cards and the last, when there are two or more, is done.
func ColumnsPlan(names []string) BoardPlan {
	var p BoardPlan
	for i, n := range names {
		p.Columns = append(p.Columns, PlanColumn{Name: n, AllowCreate: i == 0, Done: i == len(names)-1 && len(names) > 1})
	}
	return p
}

// CreateBoardFromPlan builds a board from plan in one transaction: when any
// part fails, nothing of it is left. Card templates are written by actorID.
func (s *Store) CreateBoardFromPlan(ctx context.Context, teamID int64, name string, plan BoardPlan, actorID int64) (Board, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback(ctx)
	board, err := scanBoard(tx.QueryRow(ctx,
		`INSERT INTO boards (team_id, name, person_wip_limit) VALUES ($1, $2, $3) RETURNING `+boardColumns,
		teamID, name, plan.PersonWIP))
	if err != nil {
		return Board{}, err
	}
	cols := make([]int64, len(plan.Columns))
	for i, c := range plan.Columns {
		if err := tx.QueryRow(ctx, `
			INSERT INTO columns (board_id, name, position, wip_limit, allow_create, is_done, counts_person_wip)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			board.ID, c.Name, i, c.WIP, c.AllowCreate, c.Done, c.CountsPersonWIP).Scan(&cols[i]); err != nil {
			return Board{}, err
		}
	}
	column := func(i int) (int64, error) {
		if i < 0 || i >= len(cols) {
			return 0, fmt.Errorf("store: plan names column %d of %d", i, len(cols))
		}
		return cols[i], nil
	}
	labels := make([]int64, len(plan.Labels))
	for i, l := range plan.Labels {
		if err := tx.QueryRow(ctx, `INSERT INTO labels (board_id, name, color) VALUES ($1, $2, $3) RETURNING id`,
			board.ID, l.Name, strings.ToLower(l.Color)).Scan(&labels[i]); err != nil {
			return Board{}, err
		}
	}
	for _, t := range plan.Templates {
		col, err := column(t.ColumnIndex)
		if err != nil {
			return Board{}, err
		}
		in := TemplateInput{Name: t.Name, Title: t.Title, Description: t.Description, Priority: t.Priority,
			ColumnID: &col, DueInDays: t.DueInDays, Checklist: t.Checklist, Schedule: t.Schedule}
		for _, li := range t.LabelIndexes {
			if li < 0 || li >= len(labels) {
				return Board{}, fmt.Errorf("store: plan names label %d of %d", li, len(labels))
			}
			in.LabelIDs = append(in.LabelIDs, labels[li])
		}
		if _, err := writeTemplate(ctx, tx, board.ID, 0, in, actorID); err != nil {
			return Board{}, err
		}
	}
	for _, p := range plan.Permissions {
		to, err := column(p.ToIndex)
		if err != nil {
			return Board{}, err
		}
		var from *int64
		if p.FromIndex != nil {
			id, err := column(*p.FromIndex)
			if err != nil {
				return Board{}, err
			}
			from = &id
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO move_permissions (board_id, to_column_id, from_column_id, subject) VALUES ($1, $2, $3, $4)`,
			board.ID, to, from, p.Subject); err != nil {
			return Board{}, err
		}
	}
	for _, c := range plan.Conditions {
		col, err := column(c.ColumnIndex)
		if err != nil {
			return Board{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO column_conditions (column_id, phase, kind) VALUES ($1, $2, $3)`,
			col, c.Phase, c.Kind); err != nil {
			return Board{}, err
		}
	}
	return board, tx.Commit(ctx)
}
```

`labels` tablosunda renk için bir `CHECK` varsa küçültme gereklidir; `CreateLabel` de bunu yapıyor.

- [ ] **Step 5: `CreateBoard`'u sarmalayıcıya çevir**

`internal/store/boards.go:51-72`:

```go
// CreateBoard adds a board with the named columns: the first takes new cards
// and the last, when there are two or more, is done.
func (s *Store) CreateBoard(ctx context.Context, teamID int64, name string, columns []string) (Board, error) {
	return s.CreateBoardFromPlan(ctx, teamID, name, ColumnsPlan(columns), 0)
}
```

`actorID` 0'dır, ama plan şablon içermediği için hiç kullanılmaz.

- [ ] **Step 6: Run the tests**

Run: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./internal/store -count=1 -race -v 2>&1 | grep -E '^(--- (FAIL|SKIP)|FAIL|ok)'`
Expected: yalnız `ok  	kanban/internal/store`. `--- FAIL` ya da `--- SKIP` satırı olmamalı. Var olan şablon testleri (`templates_test.go`) bölme işleminden sonra da geçmeli.

- [ ] **Step 7: Commit**

```bash
git add internal/store/board_plan.go internal/store/board_plan_test.go internal/store/boards.go internal/store/templates.go
git commit -F - <<'E'
feat(store): build a board from a plan in one transaction

CreateBoardFromPlan writes a board with its columns, labels, card
templates, move permissions, column conditions and person WIP limit, or
nothing when any part fails. CreateBoard is now a plan of columns alone,
and saveTemplate's body is shared as writeTemplate.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
E
```

---

### Task 2: `internal/blueprint`: katalog, metinler, `Render` ve `Trim`

**Files:**
- Create: `internal/blueprint/blueprint.go`, `internal/blueprint/catalog.go`, `internal/blueprint/plan.go`
- Test: `internal/blueprint/blueprint_test.go`, `internal/blueprint/blueprint_db_test.go`
- Modify: `locales/tr.json`, `locales/en.json` (yalnız `blueprints` nesnesi eklenir)

**Interfaces:**
- Consumes (Task 1):
  - `store.BoardPlan`, `store.PlanColumn`, `store.PlanLabel`, `store.PlanTemplate`, `store.PlanPermission`, `store.PlanCondition`;
  - `store.Schedule`;
  - `(*store.Store).CreateBoardFromPlan`.
- Produces (Task 3 bunlara dayanır):

```go
const Default = "simple"
type Options struct{ WIP, Labels, Templates, Recurring, Rules bool }
func AllOptions() Options
func OptionsFrom(values []string) Options // "wip","labels","templates","recurring","rules"
type Counts struct{ Columns, Labels, Templates, Rules int }
func All() []Blueprint
func Find(key string) (Blueprint, bool)
func (bp Blueprint) Has() Options
func (bp Blueprint) Counts() Counts
func (bp Blueprint) NameKey() string    // "blueprints.<key>.name"
func (bp Blueprint) SummaryKey() string // "blueprints.<key>.summary"
func (bp Blueprint) ColumnKeys() []string
func Render(bp Blueprint, t func(key string) string) store.BoardPlan
func Trim(p store.BoardPlan, o Options) store.BoardPlan
func Plan(bp Blueprint, t func(key string) string, o Options) store.BoardPlan // Trim(Render(bp, t), o)
```

- [ ] **Step 1: Write the failing tests**

`internal/blueprint/blueprint_test.go`. Bu test, i18n eklentisini başlatmak yerine `locales/*.json`'u doğrudan okur. Eksik bir anahtar boş metin döndürür ve test bunu yakalar.

```go
package blueprint_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"kanban/internal/blueprint"
	"kanban/internal/store"
)

// catalog reads a locale's catalog flat, "a.b.c" → text; plural objects
// (one/other) are left out, as blueprints use none.
func catalog(t *testing.T, locale string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("../../locales/" + locale + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var walk func(prefix string, m map[string]json.RawMessage)
	walk = func(prefix string, m map[string]json.RawMessage) {
		for k, v := range m {
			var s string
			if json.Unmarshal(v, &s) == nil {
				out[prefix+k] = s
				continue
			}
			var sub map[string]json.RawMessage
			if json.Unmarshal(v, &sub) == nil {
				walk(prefix+k+".", sub)
			}
		}
	}
	walk("", tree)
	return out
}

// strict translates as the Strict catalogs do, and fails on a key that is missing.
func strict(t *testing.T, locale string) func(string) string {
	cat := catalog(t, locale)
	return func(key string) string {
		s, ok := cat[key]
		if !ok || s == "" {
			t.Errorf("%s: no text for %q", locale, key)
		}
		return s
	}
}

var palette = []string{"#e03131", "#f08c00", "#2f9e44", "#1971c2", "#7048e8", "#c2255c", "#0c8599", "#495057"}

func TestCatalogOrderAndDefault(t *testing.T) {
	var keys []string
	for _, bp := range blueprint.All() {
		keys = append(keys, bp.Key)
	}
	want := []string{"simple", "scrum", "bugs", "software", "content", "hiring", "support"}
	if !slices.Equal(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if _, ok := blueprint.Find(blueprint.Default); !ok || blueprint.Default != "simple" {
		t.Fatal("the default is not simple")
	}
	if _, ok := blueprint.Find("nope"); ok {
		t.Fatal("an unknown key was found")
	}
}

// Every blueprint renders in every locale with no text missing, and every
// index it holds points at something that exists.
func TestEveryBlueprintRendersInEveryLocale(t *testing.T) {
	for _, locale := range []string{"tr", "en"} {
		tr := strict(t, locale)
		for _, bp := range blueprint.All() {
			tr(bp.NameKey())
			tr(bp.SummaryKey())
			p := blueprint.Render(bp, tr)
			if len(p.Columns) < 3 {
				t.Errorf("%s: %d columns", bp.Key, len(p.Columns))
			}
			done := 0
			for i, c := range p.Columns {
				if c.Name == "" {
					t.Errorf("%s/%s: column %d has no name", locale, bp.Key, i)
				}
				if c.Done {
					done++
					if i != len(p.Columns)-1 {
						t.Errorf("%s: done column %d is not last", bp.Key, i)
					}
				}
				if c.AllowCreate != (i == 0) {
					t.Errorf("%s: column %d AllowCreate = %v", bp.Key, i, c.AllowCreate)
				}
			}
			if done != 1 {
				t.Errorf("%s: %d done columns", bp.Key, done)
			}
			for _, l := range p.Labels {
				if l.Name == "" || !slices.Contains(palette, l.Color) {
					t.Errorf("%s: label %+v", bp.Key, l)
				}
			}
			for _, tp := range p.Templates {
				if tp.Name == "" || tp.Title == "" || tp.Description == "" || len(tp.Checklist) == 0 || slices.Contains(tp.Checklist, "") {
					t.Errorf("%s/%s: template %+v", locale, bp.Key, tp)
				}
				if tp.ColumnIndex != 0 {
					t.Errorf("%s: template %q targets column %d", bp.Key, tp.Name, tp.ColumnIndex)
				}
				for _, li := range tp.LabelIndexes {
					if li < 0 || li >= len(p.Labels) {
						t.Errorf("%s: template label index %d", bp.Key, li)
					}
				}
			}
			for _, pm := range p.Permissions {
				if pm.ToIndex < 0 || pm.ToIndex >= len(p.Columns) || !slices.Contains([]string{"team_lead", "assignee", "any_member"}, pm.Subject) {
					t.Errorf("%s: permission %+v", bp.Key, pm)
				}
			}
			for _, c := range p.Conditions {
				if c.ColumnIndex <= 0 || c.ColumnIndex >= len(p.Columns) {
					t.Errorf("%s: condition on column %d (the first column takes new cards and keeps no gate)", bp.Key, c.ColumnIndex)
				}
			}
		}
	}
}

func TestHasAndCounts(t *testing.T) {
	simple, _ := blueprint.Find("simple")
	if simple.Has() != (blueprint.Options{}) || simple.Counts() != (blueprint.Counts{Columns: 3}) {
		t.Fatalf("simple: %+v %+v", simple.Has(), simple.Counts())
	}
	scrum, _ := blueprint.Find("scrum")
	if scrum.Has() != blueprint.AllOptions() {
		t.Fatalf("scrum has %+v", scrum.Has())
	}
	if c := scrum.Counts(); c != (blueprint.Counts{Columns: 5, Labels: 4, Templates: 3, Rules: 2}) {
		t.Fatalf("scrum counts %+v", c)
	}
	bugs, _ := blueprint.Find("bugs")
	if h := bugs.Has(); h.Recurring || !h.Templates || !h.Rules || !h.WIP || !h.Labels {
		t.Fatalf("bugs has %+v", h)
	}
	software, _ := blueprint.Find("software")
	if c := software.Counts(); c.Rules != 2 { // one condition and the person WIP limit
		t.Fatalf("software rules = %d", c.Rules)
	}
}

func TestTrim(t *testing.T) {
	scrum, _ := blueprint.Find("scrum")
	full := blueprint.Render(scrum, strict(t, "en"))

	none := blueprint.Trim(full, blueprint.Options{})
	if len(none.Columns) != 5 || len(none.Labels) != 0 || len(none.Templates) != 0 || len(none.Permissions) != 0 ||
		len(none.Conditions) != 0 || none.PersonWIP != nil {
		t.Fatalf("nothing kept %+v", none)
	}
	for _, c := range none.Columns {
		if c.WIP != nil {
			t.Fatalf("WIP kept on %q", c.Name)
		}
	}

	noLabels := blueprint.Trim(full, blueprint.Options{Templates: true, Recurring: true})
	for _, tp := range noLabels.Templates {
		if len(tp.LabelIndexes) != 0 {
			t.Fatalf("template %q keeps labels without labels", tp.Name)
		}
	}

	noRecurring := blueprint.Trim(full, blueprint.Options{Templates: true})
	for _, tp := range noRecurring.Templates {
		if tp.Schedule.Kind != "" {
			t.Fatalf("template %q keeps its schedule", tp.Name)
		}
	}

	if !slices.EqualFunc(blueprint.Trim(full, blueprint.AllOptions()).Columns, full.Columns, func(a, b store.PlanColumn) bool {
		return a.Name == b.Name && (a.WIP == nil) == (b.WIP == nil)
	}) {
		t.Fatal("all options changed the columns")
	}
}

// "Recurring" without "templates" builds no template, so no schedule.
func TestTrimRecurringNeedsTemplates(t *testing.T) {
	scrum, _ := blueprint.Find("scrum")
	p := blueprint.Plan(scrum, strict(t, "tr"), blueprint.Options{Recurring: true})
	if len(p.Templates) != 0 {
		t.Fatalf("templates = %d", len(p.Templates))
	}
}

func TestOptionsFrom(t *testing.T) {
	got := blueprint.OptionsFrom([]string{"labels", "rules", "rules", "bogus"})
	if got != (blueprint.Options{Labels: true, Rules: true}) {
		t.Fatalf("got %+v", got)
	}
}
```

`internal/blueprint/blueprint_db_test.go` dosyası her şablonu her dilde gerçekten kurar ve her kart şablonundan bir kart açar:

```go
package blueprint_test

import (
	"context"
	"testing"
	"time"

	"kanban/internal/blueprint"
	"kanban/internal/db/dbtest"
	"kanban/internal/store"
)

func TestEveryBlueprintBuilds(t *testing.T) {
	s := store.New(dbtest.New(t))
	ctx := context.Background()
	lead, err := s.UpsertIdentity(ctx, store.Identity{Issuer: "https://idp.test", Subject: "lead", Email: "lead@example.com", Name: "Lead"}, nil, "tr")
	if err != nil {
		t.Fatal(err)
	}
	team, err := s.CreateTeam(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, lead.ID, store.RoleLead); err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"tr", "en"} {
		tr := strict(t, locale)
		for _, bp := range blueprint.All() {
			before := time.Now().Add(-time.Second)
			plan := blueprint.Plan(bp, tr, blueprint.AllOptions())
			b, err := s.CreateBoardFromPlan(ctx, team.ID, locale+" "+bp.Key, plan, lead.ID)
			if err != nil {
				t.Fatalf("%s/%s: %v", locale, bp.Key, err)
			}
			cols, _ := s.Columns(ctx, b.ID)
			labels, _ := s.Labels(ctx, b.ID)
			tpls, _ := s.Templates(ctx, b.ID)
			rules, _ := s.BoardRules(ctx, b.ID)
			c := bp.Counts()
			if len(cols) != c.Columns || len(labels) != c.Labels || len(tpls) != c.Templates ||
				len(rules.Permissions)+len(rules.Conditions) > c.Rules {
				t.Errorf("%s/%s: built %d cols %d labels %d templates %+v, want %+v", locale, bp.Key, len(cols), len(labels), len(tpls), rules, c)
			}
			for _, tp := range tpls {
				if tp.Schedule.Kind != "" && (tp.ScheduleSince == nil || tp.ScheduleSince.Before(before)) {
					t.Errorf("%s/%s: %q schedule starts %v, before the board", locale, bp.Key, tp.Name, tp.ScheduleSince)
				}
				if _, err := s.CreateCardFromTemplate(ctx, b.ID, tp.ID, 0, "", lead.ID, time.UTC, time.Now()); err != nil {
					t.Errorf("%s/%s: a card from %q: %v", locale, bp.Key, tp.Name, err)
				}
			}
		}
	}
}
```

`UpsertIdentity` imzasını `internal/store/users.go`'da doğrula: test yardımcısı onu `(ctx, identity, admins, "tr")` diye çağırıyor (`internal/store/users_test.go:20`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./internal/blueprint -count=1`
Expected: derleme hatası (`package kanban/internal/blueprint` yok ya da `undefined: blueprint.All`).

- [ ] **Step 3: Write `blueprint.go`**

```go
// Package blueprint is the catalogue of ready-made boards (spec
// 2026-10-02-board-sablonlari): what each one builds, in no language. Render
// fills its texts in one, Trim keeps the parts the creator chose.
package blueprint

import "kanban/internal/store"

// Default is the blueprint a board is built with when none is chosen: today's
// three columns.
const Default = "simple"

// Blueprint is one ready-made board. Its texts are catalog keys under
// "blueprints.<Key>.".
type Blueprint struct {
	Key       string
	Columns   []Column
	Labels    []Label
	Templates []CardTemplate
	Rules     Rules
}

// Column is a planned column; the first takes new cards, the last is done.
type Column struct {
	Key             string // blueprints.<bp>.columns.<Key>
	WIP             int    // 0: no limit
	CountsPersonWIP bool
}

// Label is a planned label; Color is one of the label palette.
type Label struct{ Key, Color string } // blueprints.<bp>.labels.<Key>

// CardTemplate is a planned card template. Its name, title, description and
// checklist items are under blueprints.<bp>.templates.<Key>.
type CardTemplate struct {
	Key       string
	Priority  int16 // 0: none
	DueInDays int   // 0: none
	Labels    []int // indexes into the blueprint's Labels
	Checklist int   // items: checklist.1 … checklist.<n>
	Schedule  store.Schedule
}

// Rules are a blueprint's rules; indexes are into its Columns.
type Rules struct {
	PersonWIP   int // 0: none
	Permissions []Permission
	Conditions  []Condition
}

// Permission says who may move cards into column To.
type Permission struct {
	To      int
	Subject string // rules.SubjectTeamLead, SubjectAssignee, SubjectAnyMember
}

// Condition gates entering or leaving a column.
type Condition struct {
	Column int
	Phase  string // rules.PhaseEnter, PhaseExit
	Kind   string // rules.HasAssignee, …
}

// Options are the parts the creator brings along; the columns always come.
type Options struct{ WIP, Labels, Templates, Recurring, Rules bool }

// AllOptions brings everything.
func AllOptions() Options { return Options{WIP: true, Labels: true, Templates: true, Recurring: true, Rules: true} }

// OptionsFrom reads the form's include values; unknown ones are ignored.
func OptionsFrom(values []string) Options {
	var o Options
	for _, v := range values {
		switch v {
		case "wip":
			o.WIP = true
		case "labels":
			o.Labels = true
		case "templates":
			o.Templates = true
		case "recurring":
			o.Recurring = true
		case "rules":
			o.Rules = true
		}
	}
	return o
}

// Counts is what a blueprint builds, for its card on the form.
type Counts struct{ Columns, Labels, Templates, Rules int }

// All is the catalogue, in the order the form shows it.
func All() []Blueprint { return catalog }

// Find returns the blueprint with key.
func Find(key string) (Blueprint, bool) {
	for _, bp := range catalog {
		if bp.Key == key {
			return bp, true
		}
	}
	return Blueprint{}, false
}

// Has says which parts bp holds, so the form can turn off the others.
func (bp Blueprint) Has() Options {
	var o Options
	for _, c := range bp.Columns {
		o.WIP = o.WIP || c.WIP > 0
	}
	o.WIP = o.WIP || bp.Rules.PersonWIP > 0
	o.Labels = len(bp.Labels) > 0
	o.Templates = len(bp.Templates) > 0
	for _, t := range bp.Templates {
		o.Recurring = o.Recurring || t.Schedule.Kind != ""
	}
	o.Rules = len(bp.Rules.Permissions) > 0 || len(bp.Rules.Conditions) > 0 || bp.Rules.PersonWIP > 0
	return o
}

// Counts counts bp's parts; the person WIP limit is a rule.
func (bp Blueprint) Counts() Counts {
	rules := len(bp.Rules.Permissions) + len(bp.Rules.Conditions)
	if bp.Rules.PersonWIP > 0 {
		rules++
	}
	return Counts{Columns: len(bp.Columns), Labels: len(bp.Labels), Templates: len(bp.Templates), Rules: rules}
}

func (bp Blueprint) key(rest string) string { return "blueprints." + bp.Key + "." + rest }

// NameKey is the catalog key of bp's name.
func (bp Blueprint) NameKey() string { return bp.key("name") }

// SummaryKey is the catalog key of bp's one-line summary.
func (bp Blueprint) SummaryKey() string { return bp.key("summary") }

// ColumnKeys are the catalog keys of bp's column names, in order.
func (bp Blueprint) ColumnKeys() []string {
	keys := make([]string, len(bp.Columns))
	for i, c := range bp.Columns {
		keys[i] = bp.key("columns." + c.Key)
	}
	return keys
}
```

- [ ] **Step 4: Write `plan.go`**

```go
package blueprint

import (
	"strconv"

	"kanban/internal/store"
)

// Render is bp with every text in t's language and everything kept.
func Render(bp Blueprint, t func(key string) string) store.BoardPlan {
	var p store.BoardPlan
	for i, c := range bp.Columns {
		col := store.PlanColumn{Name: t(bp.key("columns." + c.Key)), AllowCreate: i == 0, Done: i == len(bp.Columns)-1,
			CountsPersonWIP: c.CountsPersonWIP}
		if c.WIP > 0 {
			col.WIP = intp(c.WIP)
		}
		p.Columns = append(p.Columns, col)
	}
	for _, l := range bp.Labels {
		p.Labels = append(p.Labels, store.PlanLabel{Name: t(bp.key("labels." + l.Key)), Color: l.Color})
	}
	for _, c := range bp.Templates {
		k := "templates." + c.Key + "."
		tp := store.PlanTemplate{Name: t(bp.key(k + "name")), Title: t(bp.key(k + "title")),
			Description: t(bp.key(k + "description")), ColumnIndex: 0, LabelIndexes: c.Labels, Schedule: c.Schedule}
		for n := 1; n <= c.Checklist; n++ {
			tp.Checklist = append(tp.Checklist, t(bp.key(k+"checklist."+strconv.Itoa(n))))
		}
		if c.Priority > 0 {
			pr := c.Priority
			tp.Priority = &pr
		}
		if c.DueInDays > 0 {
			tp.DueInDays = intp(c.DueInDays)
		}
		p.Templates = append(p.Templates, tp)
	}
	if bp.Rules.PersonWIP > 0 {
		p.PersonWIP = intp(bp.Rules.PersonWIP)
	}
	for _, r := range bp.Rules.Permissions {
		p.Permissions = append(p.Permissions, store.PlanPermission{ToIndex: r.To, Subject: r.Subject})
	}
	for _, r := range bp.Rules.Conditions {
		p.Conditions = append(p.Conditions, store.PlanCondition{ColumnIndex: r.Column, Phase: r.Phase, Kind: r.Kind})
	}
	return p
}

// Trim keeps the parts o brings: without WIP no column has a limit and there
// is no person limit; without labels templates carry none; without templates
// nothing recurs; without rules there are no permissions, conditions or
// person limit. The columns always stay.
func Trim(p store.BoardPlan, o Options) store.BoardPlan {
	out := store.BoardPlan{Columns: make([]store.PlanColumn, len(p.Columns))}
	copy(out.Columns, p.Columns)
	if !o.WIP {
		for i := range out.Columns {
			out.Columns[i].WIP = nil
		}
	}
	if o.Labels {
		out.Labels = p.Labels
	}
	if o.Templates {
		for _, t := range p.Templates {
			if !o.Labels {
				t.LabelIndexes = nil
			}
			if !o.Recurring {
				t.Schedule = store.Schedule{}
			}
			out.Templates = append(out.Templates, t)
		}
	}
	if o.Rules {
		out.Permissions = p.Permissions
		out.Conditions = p.Conditions
		if o.WIP {
			out.PersonWIP = p.PersonWIP
		}
	}
	return out
}

// Plan is bp in t's language with the parts o brings.
func Plan(bp Blueprint, t func(key string) string, o Options) store.BoardPlan { return Trim(Render(bp, t), o) }

func intp(n int) *int { return &n }
```

Not: Kişi başı WIP hem kural hem WIP limitidir, bu yüzden yalnız ikisi birden seçiliyse kalır. Spec §4 bu iki seçenekten biri kapalıysa onu kaldırıyor; bu kod da tam olarak bunu yapar.

- [ ] **Step 5: Write `catalog.go`**

```go
package blueprint

import (
	"kanban/internal/rules"
	"kanban/internal/store"
)

const (
	red    = "#e03131"
	orange = "#f08c00"
	green  = "#2f9e44"
	blue   = "#1971c2"
	violet = "#7048e8"
	pink   = "#c2255c"
	teal   = "#0c8599"
	grey   = "#495057"

	high = 3 // card.priorities.3: "Yüksek" / "High"
)

// Weekdays: bit 0 is Monday.
const (
	monday = 1 << 0
	friday = 1 << 4
)

var catalog = []Blueprint{
	{Key: "simple", Columns: []Column{{Key: "todo"}, {Key: "doing"}, {Key: "done"}}},
	{
		Key:     "scrum",
		Columns: []Column{{Key: "backlog"}, {Key: "sprint"}, {Key: "doing", WIP: 3}, {Key: "review", WIP: 2}, {Key: "done"}},
		Labels:  []Label{{"story", blue}, {"bug", red}, {"debt", orange}, {"research", violet}},
		Templates: []CardTemplate{
			{Key: "story", Labels: []int{0}, Checklist: 3},
			{Key: "bug", Priority: high, Labels: []int{1}, Checklist: 3},
			{Key: "retro", Checklist: 3, Schedule: store.Schedule{Kind: "weekly", Weekdays: friday, Hour: 16}},
		},
		Rules: Rules{Conditions: []Condition{
			{Column: 2, Phase: rules.PhaseEnter, Kind: rules.HasAssignee},
			{Column: 4, Phase: rules.PhaseEnter, Kind: rules.ChecklistComplete},
		}},
	},
	{
		Key:       "bugs",
		Columns:   []Column{{Key: "new"}, {Key: "triaged"}, {Key: "fixing", WIP: 3}, {Key: "testing"}, {Key: "closed"}},
		Labels:    []Label{{"critical", red}, {"regression", pink}, {"ui", blue}, {"server", teal}},
		Templates: []CardTemplate{{Key: "report", Priority: high, Checklist: 3}},
		Rules: Rules{
			Permissions: []Permission{{To: 4, Subject: rules.SubjectTeamLead}},
			Conditions:  []Condition{{Column: 2, Phase: rules.PhaseEnter, Kind: rules.HasAssignee}},
		},
	},
	{
		Key: "software",
		Columns: []Column{{Key: "backlog"}, {Key: "ready"}, {Key: "dev", WIP: 3, CountsPersonWIP: true},
			{Key: "review", WIP: 2, CountsPersonWIP: true}, {Key: "test"}, {Key: "live"}},
		Labels: []Label{{"feature", green}, {"improvement", blue}, {"infra", grey}},
		Templates: []CardTemplate{
			{Key: "feature", Labels: []int{0}, Checklist: 4},
			{Key: "task", Labels: []int{2}, Checklist: 3},
		},
		Rules: Rules{PersonWIP: 2, Conditions: []Condition{{Column: 3, Phase: rules.PhaseExit, Kind: rules.BlockersDone}}},
	},
	{
		Key:     "content",
		Columns: []Column{{Key: "ideas"}, {Key: "writing"}, {Key: "editing"}, {Key: "scheduled"}, {Key: "published"}},
		Labels:  []Label{{"blog", blue}, {"social", pink}, {"newsletter", orange}, {"video", violet}},
		Templates: []CardTemplate{
			{Key: "blog", DueInDays: 7, Labels: []int{0}, Checklist: 4},
			{Key: "newsletter", Labels: []int{2}, Checklist: 4, Schedule: store.Schedule{Kind: "weekly", Weekdays: monday, Hour: 9}},
		},
		Rules: Rules{Conditions: []Condition{{Column: 3, Phase: rules.PhaseEnter, Kind: rules.HasDueDate}}},
	},
	{
		Key:       "hiring",
		Columns:   []Column{{Key: "applied"}, {Key: "screen"}, {Key: "interview"}, {Key: "offer"}, {Key: "hired"}},
		Labels:    []Label{{"frontend", blue}, {"backend", teal}, {"design", pink}, {"intern", green}},
		Templates: []CardTemplate{{Key: "candidate", Checklist: 4}},
		Rules:     Rules{Permissions: []Permission{{To: 3, Subject: rules.SubjectTeamLead}}},
	},
	{
		Key:       "support",
		Columns:   []Column{{Key: "new"}, {Key: "investigating", WIP: 5}, {Key: "waiting"}, {Key: "solved"}},
		Labels:    []Label{{"urgent", red}, {"billing", orange}, {"account", blue}, {"bug", pink}},
		Templates: []CardTemplate{{Key: "ticket", Priority: high, Checklist: 3}},
		Rules:     Rules{Conditions: []Condition{{Column: 1, Phase: rules.PhaseEnter, Kind: rules.HasAssignee}}},
	},
}
```

`rules.BlockersDone` ve `rules.ChecklistComplete` sabitleri `internal/rules/rules.go:45-46`'da tanımlı. `Label{"story", blue}` yazımı `go vet`'in "composite literal uses unkeyed fields" uyarısına takılırsa, aynı paket içinde olduğu için uyarı çıkmaz; çıkarsa alan adlı yaz (`{Key: "story", Color: blue}`).

- [ ] **Step 6: Add the texts**

`locales/tr.json` ve `locales/en.json` dosyalarına en üst seviyede bir `blueprints` nesnesi ekle. Diğer anahtarlara dokunma. Python'la ekle: `json.load(..., object_pairs_hook=collections.OrderedDict)`, `d["blueprints"] = …`, `json.dumps(d, ensure_ascii=False, indent=2) + "\n"`.

`tr`:

```json
{
  "simple": {
    "name": "Basit",
    "summary": "Üç kolonlu, kuralsız düz bir board.",
    "columns": {"todo": "Yapılacak", "doing": "Yapılıyor", "done": "Bitti"}
  },
  "scrum": {
    "name": "Scrum",
    "summary": "Backlog'dan sprinte, incelemeden bitişe; kullanıcı hikâyeleri ve haftalık retrospektifle.",
    "columns": {"backlog": "Backlog", "sprint": "Sprint", "doing": "Yapılıyor", "review": "İncelemede", "done": "Bitti"},
    "labels": {"story": "Hikâye", "bug": "Hata", "debt": "Teknik borç", "research": "Araştırma"},
    "templates": {
      "story": {
        "name": "Kullanıcı hikâyesi", "title": "Yeni kullanıcı hikâyesi",
        "description": "**Hikâye**\n[rol] olarak [ne] istiyorum, böylece [neden].\n\n**Kabul kriterleri**\nAşağıdaki kontrol listesinde.",
        "checklist": {"1": "Kabul kriteri 1", "2": "Kabul kriteri 2", "3": "Testler yazıldı"}
      },
      "bug": {
        "name": "Hata", "title": "Yeni hata",
        "description": "**Adımlar**\n\n**Beklenen**\n\n**Gerçekleşen**",
        "checklist": {"1": "Yeniden üretildi", "2": "Düzeltildi", "3": "Test eklendi"}
      },
      "retro": {
        "name": "Haftalık retrospektif", "title": "Retrospektif",
        "description": "**İyi gidenler**\n\n**Geliştirilecekler**\n\n**Aksiyonlar**",
        "checklist": {"1": "Önceki aksiyonlar gözden geçirildi", "2": "Notlar toplandı", "3": "Aksiyonlar atandı"}
      }
    }
  },
  "bugs": {
    "name": "Hata takibi",
    "summary": "Bildirilen hatalar önceliklendirilir, düzeltilir ve test edilir; kapatmayı lider yapar.",
    "columns": {"new": "Yeni", "triaged": "Önceliklendirildi", "fixing": "Düzeltiliyor", "testing": "Test ediliyor", "closed": "Kapandı"},
    "labels": {"critical": "Kritik", "regression": "Regresyon", "ui": "Arayüz", "server": "Sunucu"},
    "templates": {
      "report": {
        "name": "Hata raporu", "title": "Yeni hata raporu",
        "description": "**Adımlar**\n\n**Beklenen**\n\n**Gerçekleşen**\n\n**Ortam**\nTarayıcı, sürüm, cihaz",
        "checklist": {"1": "Yeniden üretildi", "2": "Kök neden bulundu", "3": "Düzeltme test edildi"}
      }
    }
  },
  "software": {
    "name": "Yazılım geliştirme",
    "summary": "Hazır işler geliştirilir, kod incelemesinden ve testten geçip yayına çıkar.",
    "columns": {"backlog": "Backlog", "ready": "Hazır", "dev": "Geliştirme", "review": "Kod incelemesi", "test": "Test", "live": "Yayında"},
    "labels": {"feature": "Özellik", "improvement": "İyileştirme", "infra": "Altyapı"},
    "templates": {
      "feature": {
        "name": "Özellik", "title": "Yeni özellik",
        "description": "**Amaç**\n\n**Kapsam**\n\n**Kapsam dışı**",
        "checklist": {"1": "Tasarım onaylandı", "2": "Kod yazıldı", "3": "Kod incelendi", "4": "Belgelendi"}
      },
      "task": {
        "name": "Teknik görev", "title": "Yeni teknik görev",
        "description": "**Ne**\n\n**Neden**",
        "checklist": {"1": "Yapıldı", "2": "İncelendi", "3": "Yayına alındı"}
      }
    }
  },
  "content": {
    "name": "İçerik takvimi",
    "summary": "Fikirler yazılır, editörden geçer, tarihlenir ve yayımlanır.",
    "columns": {"ideas": "Fikirler", "writing": "Yazılıyor", "editing": "Editörde", "scheduled": "Planlandı", "published": "Yayında"},
    "labels": {"blog": "Blog", "social": "Sosyal medya", "newsletter": "Bülten", "video": "Video"},
    "templates": {
      "blog": {
        "name": "Blog yazısı", "title": "Yeni blog yazısı",
        "description": "**Konu**\n\n**Hedef okur**\n\n**Anahtar kelimeler**",
        "checklist": {"1": "Taslak", "2": "Görseller", "3": "SEO", "4": "Yayın"}
      },
      "newsletter": {
        "name": "Haftalık bülten", "title": "Haftalık bülten",
        "description": "**Bu haftanın konuları**",
        "checklist": {"1": "İçerik seçildi", "2": "Yazıldı", "3": "Deneme gönderildi", "4": "Gönderildi"}
      }
    }
  },
  "hiring": {
    "name": "İşe alım",
    "summary": "Adaylar başvurudan teklife ilerler; teklif aşamasına adayı lider taşır.",
    "columns": {"applied": "Başvurular", "screen": "Ön görüşme", "interview": "Teknik mülakat", "offer": "Teklif", "hired": "İşe alındı"},
    "labels": {"frontend": "Ön yüz", "backend": "Arka yüz", "design": "Tasarım", "intern": "Stajyer"},
    "templates": {
      "candidate": {
        "name": "Aday", "title": "Yeni aday",
        "description": "**Pozisyon**\n\n**CV**\n\n**Görüşme notları**",
        "checklist": {"1": "CV incelendi", "2": "Ön görüşme yapıldı", "3": "Teknik mülakat yapıldı", "4": "Referanslar arandı"}
      }
    }
  },
  "support": {
    "name": "Destek talepleri",
    "summary": "Talepler incelenir, gerekirse yanıt beklenir ve çözülür.",
    "columns": {"new": "Yeni", "investigating": "İnceleniyor", "waiting": "Yanıt bekleniyor", "solved": "Çözüldü"},
    "labels": {"urgent": "Acil", "billing": "Fatura", "account": "Hesap", "bug": "Hata"},
    "templates": {
      "ticket": {
        "name": "Destek talebi", "title": "Yeni destek talebi",
        "description": "**Talep eden**\n\n**Sorun**\n\n**Yapılanlar**",
        "checklist": {"1": "Talep anlaşıldı", "2": "Yanıt verildi", "3": "Çözüm doğrulandı"}
      }
    }
  }
}
```

`en`:

```json
{
  "simple": {
    "name": "Simple",
    "summary": "A plain board with three columns and no rules.",
    "columns": {"todo": "To do", "doing": "In progress", "done": "Done"}
  },
  "scrum": {
    "name": "Scrum",
    "summary": "From backlog to sprint, from review to done; with user stories and a weekly retrospective.",
    "columns": {"backlog": "Backlog", "sprint": "Sprint", "doing": "In progress", "review": "In review", "done": "Done"},
    "labels": {"story": "Story", "bug": "Bug", "debt": "Tech debt", "research": "Research"},
    "templates": {
      "story": {
        "name": "User story", "title": "New user story",
        "description": "**Story**\nAs a [role], I want [what], so that [why].\n\n**Acceptance criteria**\nIn the checklist below.",
        "checklist": {"1": "Acceptance criterion 1", "2": "Acceptance criterion 2", "3": "Tests written"}
      },
      "bug": {
        "name": "Bug", "title": "New bug",
        "description": "**Steps**\n\n**Expected**\n\n**Actual**",
        "checklist": {"1": "Reproduced", "2": "Fixed", "3": "Test added"}
      },
      "retro": {
        "name": "Weekly retrospective", "title": "Retrospective",
        "description": "**What went well**\n\n**What to improve**\n\n**Actions**",
        "checklist": {"1": "Last actions reviewed", "2": "Notes collected", "3": "Actions assigned"}
      }
    }
  },
  "bugs": {
    "name": "Bug tracking",
    "summary": "Reported bugs are triaged, fixed and tested; the lead closes them.",
    "columns": {"new": "New", "triaged": "Triaged", "fixing": "Fixing", "testing": "Testing", "closed": "Closed"},
    "labels": {"critical": "Critical", "regression": "Regression", "ui": "UI", "server": "Server"},
    "templates": {
      "report": {
        "name": "Bug report", "title": "New bug report",
        "description": "**Steps**\n\n**Expected**\n\n**Actual**\n\n**Environment**\nBrowser, version, device",
        "checklist": {"1": "Reproduced", "2": "Root cause found", "3": "Fix tested"}
      }
    }
  },
  "software": {
    "name": "Software development",
    "summary": "Ready work is built, passes code review and testing, and ships.",
    "columns": {"backlog": "Backlog", "ready": "Ready", "dev": "Development", "review": "Code review", "test": "Testing", "live": "Live"},
    "labels": {"feature": "Feature", "improvement": "Improvement", "infra": "Infrastructure"},
    "templates": {
      "feature": {
        "name": "Feature", "title": "New feature",
        "description": "**Goal**\n\n**Scope**\n\n**Out of scope**",
        "checklist": {"1": "Design approved", "2": "Code written", "3": "Code reviewed", "4": "Documented"}
      },
      "task": {
        "name": "Technical task", "title": "New technical task",
        "description": "**What**\n\n**Why**",
        "checklist": {"1": "Done", "2": "Reviewed", "3": "Deployed"}
      }
    }
  },
  "content": {
    "name": "Content calendar",
    "summary": "Ideas are written, edited, scheduled and published.",
    "columns": {"ideas": "Ideas", "writing": "Writing", "editing": "Editing", "scheduled": "Scheduled", "published": "Published"},
    "labels": {"blog": "Blog", "social": "Social media", "newsletter": "Newsletter", "video": "Video"},
    "templates": {
      "blog": {
        "name": "Blog post", "title": "New blog post",
        "description": "**Topic**\n\n**Audience**\n\n**Keywords**",
        "checklist": {"1": "Draft", "2": "Images", "3": "SEO", "4": "Publish"}
      },
      "newsletter": {
        "name": "Weekly newsletter", "title": "Weekly newsletter",
        "description": "**This week's topics**",
        "checklist": {"1": "Content picked", "2": "Written", "3": "Test sent", "4": "Sent"}
      }
    }
  },
  "hiring": {
    "name": "Hiring",
    "summary": "Candidates move from application to offer; the lead moves them to offer.",
    "columns": {"applied": "Applications", "screen": "Screening", "interview": "Technical interview", "offer": "Offer", "hired": "Hired"},
    "labels": {"frontend": "Frontend", "backend": "Backend", "design": "Design", "intern": "Intern"},
    "templates": {
      "candidate": {
        "name": "Candidate", "title": "New candidate",
        "description": "**Role**\n\n**CV**\n\n**Interview notes**",
        "checklist": {"1": "CV reviewed", "2": "Screening done", "3": "Technical interview done", "4": "References checked"}
      }
    }
  },
  "support": {
    "name": "Support requests",
    "summary": "Requests are investigated, wait for a reply when needed, and get solved.",
    "columns": {"new": "New", "investigating": "Investigating", "waiting": "Waiting for reply", "solved": "Solved"},
    "labels": {"urgent": "Urgent", "billing": "Billing", "account": "Account", "bug": "Bug"},
    "templates": {
      "ticket": {
        "name": "Support request", "title": "New support request",
        "description": "**Requested by**\n\n**Problem**\n\n**Done so far**",
        "checklist": {"1": "Request understood", "2": "Replied", "3": "Solution confirmed"}
      }
    }
  }
}
```

- [ ] **Step 7: Run the tests**

Run: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./internal/blueprint -count=1 -race -v 2>&1 | grep -E '^(--- (FAIL|SKIP)|FAIL|ok)'`
Expected: yalnız `ok  	kanban/internal/blueprint`. `--- SKIP: TestEveryBlueprintBuilds` olmamalı.

Ardından çeviri kataloglarının hâlâ aynı anahtarları taşıdığını görmek için uygulamanın kendi testlerini çalıştır, çünkü Strict modda eksik bir anahtar uygulamayı başlatmaz: `go test ./internal/web -count=1 -run TestLeadCreatesABoardFromTheTeamPage` (veritabanıyla).

- [ ] **Step 8: Commit**

```bash
git add internal/blueprint locales/tr.json locales/en.json
git commit -F - <<'E'
feat(blueprint): seven ready-made boards

Simple, Scrum, bug tracking, software development, content calendar,
hiring and support requests: columns with WIP limits, labels, card
templates (some recurring) and rules, in no language. Render fills the
texts from the catalogs, Trim keeps the parts the creator chose.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
E
```

---

### Task 3: Takım sayfası formu: şablonla kurulum

**Files:**
- Create: `internal/web/blueprints.go`
- Modify: `internal/web/pages_teams.go` (`teamView`, `loadTeam`, `createBoard`)
- Modify: `templates/pages/team.html:27-38` (board oluşturma formu)
- Modify: `locales/tr.json`, `locales/en.json` (`board.*` anahtarları; `board.default_columns` silinir)
- Test: `internal/web/boards_test.go`

**Interfaces:**
- Consumes (Task 2):
  - `blueprint.All`, `Find`, `Default`;
  - `Blueprint.Has`, `Counts`, `NameKey`, `SummaryKey`, `ColumnKeys`;
  - `blueprint.Plan`, `AllOptions`, `OptionsFrom`, `Options`.
- Consumes (Task 1): `(*store.Store).CreateBoardFromPlan(ctx, teamID, name, plan, actorID)`.
- Produces (Task 4 bunlara dayanır):
  - **Form işaretleri:**
    - `data-blueprint-form`, formun kendisi;
    - `input[type=radio][name=blueprint]`, her biri `data-has` özniteliğiyle. Değeri boşlukla ayrılmış grup adlarıdır: `wip labels templates recurring rules`.
    - `input[type=checkbox][name=include]`, değerleri `wip|labels|templates|recurring|rules`.
  - **CSS sınıfları:** `.blueprints`, `.blueprint`, `.blueprint__name`, `.blueprint__summary`, `.blueprint__columns`, `.blueprint__counts`, `.includes`.

- [ ] **Step 1: Write the failing tests**

`internal/web/boards_test.go` dosyasına ekle. `TestLeadCreatesABoardFromTheTeamPage` ve `TestMembersCannotCreateBoards` olduğu gibi kalır, çünkü `blueprint` alanı olmadan Basit kurulur.

```go
func TestCreateBoardFromBlueprint(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	path := teamWith(t, h, "lead@example.com", "")
	form := lead.Get(path).Body
	mustContain(t, form, `name="blueprint" value="simple" checked`, `name="blueprint" value="scrum"`, `data-has="wip labels templates recurring rules"`,
		`name="include" value="recurring" checked`, `name="include_present" value="1"`, "Hata takibi", "Backlog → Sprint → Yapılıyor → İncelemede → Bitti")
	res := lead.Submit(path, path, url.Values{"op": {"create_board"}, "board_name": {"Platform sprint"}, "blueprint": {"scrum"},
		"include_present": {"1"}, "include": {"wip", "labels", "templates", "recurring", "rules"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("create = %d:\n%s", res.Status, res.Body)
	}
	mustContain(t, lead.Get(res.Location()).Body, "Backlog", "Sprint", "İncelemede")
	settings := lead.Get(res.Location() + "/settings?tab=templates").Body
	mustContain(t, settings, "Kullanıcı hikâyesi", "Haftalık retrospektif")
}

func TestCreateBoardFromBlueprintInEnglish(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	path := teamWith(t, h, "lead@example.com", "")
	// An English speaker's pages are under /en; their board is built in English.
	h.speaks("lead@example.com", "en")
	en := "/en" + path
	res := lead.Submit(en, en, url.Values{"op": {"create_board"}, "board_name": {"Bugs"}, "blueprint": {"bugs"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("create = %d:\n%s", res.Status, res.Body)
	}
	body := lead.Get(res.Location()).Body
	mustContain(t, body, "Triaged", "Fixing", "Closed")
	for _, bad := range []string{"Önceliklendirildi", "blueprints."} {
		if strings.Contains(body, bad) {
			t.Errorf("an English board shows %q", bad)
		}
	}
}

// Unticked boxes are not sent; include_present says the form had them.
func TestBoardFormIncludes(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	path := teamWith(t, h, "lead@example.com", "")
	res := lead.Submit(path, path, url.Values{"op": {"create_board"}, "board_name": {"Bare"}, "blueprint": {"scrum"},
		"include_present": {"1"}, "include": {"recurring"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("create = %d", res.Status)
	}
	boardID, err := strconv.ParseInt(strings.TrimPrefix(res.Location(), "/boards/"), 10, 64)
	if err != nil {
		t.Fatalf("location %q", res.Location())
	}
	ctx := context.Background()
	labels, _ := h.store.Labels(ctx, boardID)
	tpls, _ := h.store.Templates(ctx, boardID)
	rules, _ := h.store.BoardRules(ctx, boardID)
	cols, _ := h.store.Columns(ctx, boardID)
	if len(cols) != 5 || len(labels) != 0 || len(tpls) != 0 || len(rules.Conditions) != 0 || cols[2].WIPLimit != nil {
		t.Fatalf("bare scrum built %d cols, %d labels, %d templates, %+v", len(cols), len(labels), len(tpls), rules)
	}
}

func TestBoardFormRefusesAnUnknownBlueprint(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	path := teamWith(t, h, "lead@example.com", "")
	res := lead.Submit(path, path, url.Values{"op": {"create_board"}, "board_name": {"X"}, "blueprint": {"kanban-pro"}})
	if res.Status == http.StatusSeeOther {
		t.Fatal("an unknown blueprint built a board")
	}
	mustContain(t, res.Body, "Bu şablon yok.")
}

func TestBoardFormKeepsTheChosenBlueprint(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	path := teamWith(t, h, "lead@example.com", "")
	res := lead.Submit(path, path, url.Values{"op": {"create_board"}, "board_name": {""}, "blueprint": {"hiring"}})
	if res.Status == http.StatusSeeOther {
		t.Fatal("an empty name built a board")
	}
	mustContain(t, res.Body, `name="blueprint" value="hiring" checked`)
	if strings.Contains(res.Body, `name="blueprint" value="simple" checked`) {
		t.Error("the refused form went back to Simple")
	}
}
```

`boards_test.go` içinde `strconv`, `context` ve `strings` importlarının bulunduğunu kontrol et (`id()` zaten `strconv` kullanıyor). İngilizce kullanıcı `h.speaks(email, "en")` ile kurulur; sayfaları `/en` altında açılır (`cards_test.go:136`'daki gibi).

- [ ] **Step 2: Run tests to verify they fail**

Run: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./internal/web -count=1 -run 'TestCreateBoardFromBlueprint|TestBoardForm'`
Expected: FAIL. Sayfada `name="blueprint"` yok ve `scrum` kolonları kurulmamış.

- [ ] **Step 3: Add the UI texts and drop the old defaults**

`board` nesnesine ekle (tr / en):

| Anahtar | tr | en |
| --- | --- | --- |
| `blueprint` | Şablon | Template |
| `blueprint_unknown` | Bu şablon yok. | There is no such template. |
| `include` | Aktarılacaklar | Bring along |
| `include_wip` | WIP limitleri | WIP limits |
| `include_labels` | Etiketler | Labels |
| `include_templates` | Kart şablonları | Card templates |
| `include_recurring` | Tekrarlar | Recurring cards |
| `include_rules` | Kurallar | Rules |
| `include_hint` | Kolonlar her zaman gelir. | Columns always come along. |

`board.blueprint_count` altında çoğul nesneler (`one` / `other`):

| Anahtar | tr one / other | en one / other |
| --- | --- | --- |
| `columns` | {count} kolon / {count} kolon | {count} column / {count} columns |
| `labels` | {count} etiket / {count} etiket | {count} label / {count} labels |
| `templates` | {count} kart şablonu / {count} kart şablonu | {count} card template / {count} card templates |
| `rules` | {count} kural / {count} kural | {count} rule / {count} rules |

`board.default_columns` nesnesini iki dosyadan da sil. Önce `grep -rn default_columns internal templates` ile yalnız `pages_teams.go` içinde kullanıldığını doğrula.

- [ ] **Step 4: Write `internal/web/blueprints.go`**

```go
package web

import (
	"strings"

	"github.com/elagoht/collage"
	i18n "github.com/elagoht/collage-i18n"

	"kanban/internal/blueprint"
)

// blueprintCard is one ready-made board on the create form.
type blueprintCard struct {
	Key, Name, Summary, Columns string
	Counts                      blueprint.Counts
	Has                         string // the parts it holds, for blueprint-form.js
}

func blueprintCards(rc *collage.RenderContext) []blueprintCard {
	var out []blueprintCard
	for _, bp := range blueprint.All() {
		names := make([]string, 0, len(bp.Columns))
		for _, k := range bp.ColumnKeys() {
			names = append(names, i18n.T(rc, k))
		}
		h := bp.Has()
		var has []string
		for _, p := range []struct {
			on   bool
			name string
		}{{h.WIP, "wip"}, {h.Labels, "labels"}, {h.Templates, "templates"}, {h.Recurring, "recurring"}, {h.Rules, "rules"}} {
			if p.on {
				has = append(has, p.name)
			}
		}
		out = append(out, blueprintCard{Key: bp.Key, Name: i18n.T(rc, bp.NameKey()), Summary: i18n.T(rc, bp.SummaryKey()),
			Columns: strings.Join(names, " → "), Counts: bp.Counts(), Has: strings.Join(has, " ")})
	}
	return out
}

// includeOptions reads the form's include boxes: a form that had them sends
// include_present, and then only the ticked ones come; any other request
// brings everything.
func includeOptions(rc *collage.RenderContext, present string) blueprint.Options {
	if present == "" {
		return blueprint.AllOptions()
	}
	return blueprint.OptionsFrom(rc.Request.PostForm["include"])
}
```

`collage` ve `collage-i18n` import yollarını `pages_teams.go`'nun importlarından olduğu gibi kopyala. Yukarıda yazılanlar varsayımdır.

- [ ] **Step 5: Wire the page and the action**

`teamView`'e `Blueprints []blueprintCard` ekle. `loadTeam` içinde `v.CanManage` doğruysa `v.Blueprints = blueprintCards(rc)` ata.

`createBoard`'u şöyle değiştir (`pages_teams.go:285-301`):

```go
// createBoard builds a board from the chosen blueprint, in the creator's
// language, with the parts they brought along.
func (h *handlers) createBoard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, team store.Team) (*collage.ActionResult, error) {
	v.Field("board_name").Required().MaxLen(100)
	key := v.Value("blueprint")
	if key == "" {
		key = blueprint.Default
	}
	bp, ok := blueprint.Find(key)
	if !ok {
		v.Fail("blueprint", i18n.T(rc, "board.blueprint_unknown"))
	}
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	plan := blueprint.Plan(bp, func(k string) string { return i18n.T(rc, k) }, includeOptions(rc, v.Value("include_present")))
	board, err := h.store.CreateBoardFromPlan(ctx, team.ID, strings.TrimSpace(v.Value("board_name")), plan, u.ID)
	if err != nil {
		return nil, err
	}
	return h.redirectTo(rc, "board", "id", strconv.FormatInt(board.ID, 10))
}
```

- [ ] **Step 6: The form**

`templates/pages/team.html` içindeki `create_board` formunu şununla değiştir:

```html
<form class="stack" method="post" action="{{$self}}" data-blueprint-form>
  {{csrfToken}}
  <input type="hidden" name="op" value="create_board">
  <input type="hidden" name="include_present" value="1">
  <label class="field">
    <span class="field__label">{{t "board.name"}}</span>
    <input class="input" id="board-name" name="board_name" value="{{fieldValue "board_name"}}" required maxlength="100">
    {{with fieldError "board_name"}}<span class="field__error">{{.}}</span>{{end}}
  </label>
  {{$chosen := or (fieldValue "blueprint") "simple"}}
  <fieldset class="field">
    <legend class="field__label">{{t "board.blueprint"}}</legend>
    <div class="blueprints">
      {{range .Blueprints}}
      <label class="blueprint">
        <input type="radio" name="blueprint" value="{{.Key}}"{{if eq .Key $chosen}} checked{{end}} data-has="{{.Has}}">
        <span class="blueprint__name">{{.Name}}</span>
        <span class="blueprint__summary">{{.Summary}}</span>
        <span class="blueprint__columns">{{.Columns}}</span>
        <span class="blueprint__counts">{{tn "board.blueprint_count.columns" .Counts.Columns}}{{if .Counts.Labels}} · {{tn "board.blueprint_count.labels" .Counts.Labels}}{{end}}{{if .Counts.Templates}} · {{tn "board.blueprint_count.templates" .Counts.Templates}}{{end}}{{if .Counts.Rules}} · {{tn "board.blueprint_count.rules" .Counts.Rules}}{{end}}</span>
      </label>
      {{end}}
    </div>
    {{with fieldError "blueprint"}}<span class="field__error">{{.}}</span>{{end}}
  </fieldset>
  <fieldset class="field">
    <legend class="field__label">{{t "board.include"}}</legend>
    <div class="includes">
      <label class="check-pill"><input type="checkbox" name="include" value="wip" checked> {{t "board.include_wip"}}</label>
      <label class="check-pill"><input type="checkbox" name="include" value="labels" checked> {{t "board.include_labels"}}</label>
      <label class="check-pill"><input type="checkbox" name="include" value="templates" checked> {{t "board.include_templates"}}</label>
      <label class="check-pill"><input type="checkbox" name="include" value="recurring" checked> {{t "board.include_recurring"}}</label>
      <label class="check-pill"><input type="checkbox" name="include" value="rules" checked> {{t "board.include_rules"}}</label>
    </div>
    <span class="field__hint">{{t "board.include_hint"}}</span>
  </fieldset>
  <div><button type="submit" class="btn btn--primary">{{template "icon" "plus"}} {{t "board.create"}}</button></div>
</form>
```

Betik satırı (`blueprint-form.js`) Task 4'te eklenir.

Reddedilen formda işaretli kutular yeniden doldurulmaz, hepsi tekrar işaretli gelir. `validate` her alan için tek bir değer saklıyor. Bu bilinen ve kabul edilen bir sınır.

- [ ] **Step 7: Run the tests**

Run: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./... -count=1 -race -v 2>&1 | grep -E '^(--- (FAIL|SKIP)|FAIL|ok)'`
Expected: her paket `ok`. `--- FAIL` ya da `--- SKIP` olmamalı.

- [ ] **Step 8: Commit**

```bash
git add internal/web/blueprints.go internal/web/pages_teams.go internal/web/boards_test.go templates/pages/team.html locales/tr.json locales/en.json
git commit -F - <<'E'
feat(teams): create a board from a ready-made template

The create form offers the seven blueprints as cards, each with its
columns and what it brings, and boxes for the parts to bring along:
WIP limits, labels, card templates, recurring cards, rules. A form
without the boxes brings everything; an unknown blueprint is refused.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
E
```

---

### Task 4: Görünüm ve `blueprint-form.js`

**Files:**
- Modify: `static/css/pages.css` (sona ekle)
- Create: `static/js/blueprint-form.js`
- Modify: `templates/pages/team.html` (formun hemen altına `<script type="module" src="{{asset "/static/js/blueprint-form.js"}}"></script>`)

**Interfaces:**
- Consumes (Task 3): `data-blueprint-form`, `input[name=blueprint][data-has]`, `input[name=include]`, `.blueprints`, `.blueprint`, `.blueprint__*`, `.includes`.
- Produces: —

- [ ] **Step 1: CSS**

`static/css/pages.css` dosyasının sonuna ekle. Yalnız rem ve token kullan:

```css
/* The create form's ready-made boards: a radio card each. */
.blueprints { display: grid; gap: var(--s3); grid-template-columns: repeat(auto-fill, minmax(15rem, 1fr)); }
.blueprint {
  position: relative; display: grid; gap: var(--s1); align-content: start;
  padding: var(--s3) var(--s4); border-radius: var(--r-md); cursor: pointer;
  background: var(--sheet); border: 0.0625rem solid var(--line); box-shadow: var(--shadow-paper);
  transition: border-color 120ms var(--ease), background-color 120ms var(--ease);
}
.blueprint:hover { border-color: var(--line-strong); }
.blueprint:has(input:checked) { border-color: var(--accent); background: var(--accent-soft); }
.blueprint:has(input:focus-visible) { box-shadow: 0 0 0 0.1875rem var(--focus); }
.blueprint input { position: absolute; opacity: 0; pointer-events: none; }
.blueprint__name { font-weight: 800; color: var(--ink); }
.blueprint__summary { font-size: var(--text-sm); color: var(--ink-soft); }
.blueprint__columns { font-size: var(--text-xs); color: var(--ink-muted); overflow-wrap: anywhere; }
.blueprint__counts { font-size: var(--text-xs); font-weight: 700; color: var(--accent-strong); }
.includes { display: flex; flex-wrap: wrap; gap: var(--s2); }
.includes .check-pill:has(input:disabled) { opacity: 0.5; cursor: not-allowed; }
@media (max-width: 40rem) { .blueprints { grid-template-columns: minmax(0, 1fr); } }
```

`.check-pill` zaten var (`components.css:60`). Radyo girdisi görsel olarak gizli kalır ama klavyeyle seçilebilir; odak halkası kartın üstünde görünür.

- [ ] **Step 2: JS**

`static/js/blueprint-form.js`:

```js
// blueprint-form.js: the create-board form turns off the parts the chosen
// blueprint does not hold, and recurring cards without card templates. The
// server applies the same rules, so the form works without this script.

const form = document.querySelector("[data-blueprint-form]");

if (form) {
  const boxes = [...form.querySelectorAll('input[type="checkbox"][name="include"]')];
  const box = (name) => boxes.find((b) => b.value === name);

  const update = () => {
    const chosen = form.querySelector('input[name="blueprint"]:checked');
    const has = new Set((chosen?.dataset.has ?? "").split(" ").filter(Boolean));
    for (const b of boxes) b.disabled = !has.has(b.value);
    const templates = box("templates");
    const recurring = box("recurring");
    if (templates && recurring && has.has("recurring")) {
      recurring.disabled = !templates.checked;
      if (!templates.checked) recurring.checked = false;
    }
  };

  form.addEventListener("change", update);
  update();
}
```

Devre dışı bir onay kutusu forma gönderilmez. Bu doğru davranış: şablonda olmayan bir grup zaten kurulmaz.

- [ ] **Step 3: Check it in a browser, or headless**

Chrome eklentisi bağlıysa takım sayfasını açık ve koyu temada, 1280 ve 390 piksel genişlikte aç. Sonra şunları dene:
- Basit seçiliyken bütün kutular devre dışı olmalı.
- Hata takibi seçiliyken "Tekrarlar" devre dışı olmalı.
- "Kart şablonları" kaldırılınca "Tekrarlar" da kalkmalı.
- Klavyeyle (Tab ve ok tuşları) şablonlar arasında gezilebilmeli.

Eklenti yoksa `static/css/*.css` dosyalarını bağlayan sabit bir HTML'i görünür pencere açmayan Chrome ile çiz ve `.blueprint` kartlarının ölçülerine bak. Yöntem: scratchpad'e HTML yaz, ardından `"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --allow-file-access-from-files --screenshot=… --window-size=1280,900 file://…` çalıştır.

- [ ] **Step 4: Run the tests**

Run: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./... -count=1 -race 2>&1 | grep -v '^ok\|no test files'`
Expected: çıktı yok.

`grep -n "px" static/css/pages.css` boş dönmeli.

- [ ] **Step 5: Commit**

```bash
git add static/css/pages.css static/js/blueprint-form.js templates/pages/team.html
git commit -F - <<'E'
feat(ui): blueprint cards and the parts to bring along

The ready-made boards are radio cards with their columns and counts;
the chosen one is outlined in the accent. blueprint-form.js turns off
the parts a blueprint does not hold, and recurring cards without card
templates.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
E
```
