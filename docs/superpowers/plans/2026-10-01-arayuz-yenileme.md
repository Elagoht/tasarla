# Arayüz yenileme: Uygulama Planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Spec'teki davranış düzeltmeleri, "sıcak kağıt" görsel dili, sol çubuklu iskelet, sağdan açılan kart paneli, sekmeli ayarlar ve cümle cümle kurallar.

**Architecture:** Sunucu tarafında render aynen kalır. Yeni action işlemleri mevcut store'un üstüne küçük fonksiyonlar olarak eklenir. Görsel dil `static/css/` altındaki token tabanlı CSS dosyalarıyla, etkileşim `static/js/` altındaki ES modülleriyle kurulur. JS yokken her şey form olarak çalışır.

**Tech Stack:** Go, collage v0.39.0 ve eklentileri, PostgreSQL, SortableJS 1.15.6. Görsel doğrulama için puppeteer-core ile headless Chrome kullanılır; yalnız geliştirme makinesinde, repoya girmez.

**Spec:** `docs/superpowers/specs/2026-10-01-arayuz-yenileme-design.md`

## Global Constraints

- Go'da `any` / `interface{}` yazılmaz. Inline `style` ve inline `<script>` kullanılmaz (CSP: `style-src 'self'`, `script-src 'self' 'nonce-…'`).
- İki katalog (TR, EN) aynı anahtarları taşır (i18n `Strict`).
- Veri modeli ve `internal/rules` değişmez. Yeni migration yoktur.
- Her URL ve form kimliği kendi board'una bağlanır (IDOR). Takım dışındaki herkes 404 alır.
- JS yokken her işlem form olarak çalışır.
- Framework kuralı geçerlidir: dokümanla çelişen bir davranış görülürse `framework-issues/NNN` açılır ve o kısımda durulur.

## Review Focus

1. **Alan kaydında yarış:** iki kullanıcı aynı kartın farklı alanlarını art arda kaydeder. İkinci kaydın sürümü bayattır, ama değişmeyen alanın üzerine yazılmamalı. Kayıt reddedilir ve alan geri alınır. Testi Task 2'de.
2. **Toplu kolon kaydı yarıda hata verir:** ör. üçüncü satır geçersiz. Hiçbir değişiklik kaydedilmemeli ve girilen değerler kaybolmamalı. Testi Task 2'de.
3. **Kendi rolü:** admin kendi adminliğini, lider kendi rolünü elle gönderilen bir POST'la değiştiremez. Testi Task 1'de.
4. **Gruplanmış yetki cümlesi silinir:** yalnız o gruptaki satırlar silinmeli; başka bir hedef ya da kaynağın satırları kalmalı. Testi Task 3'te.
5. **Kısıtlı modda son geçiş cümlesi silinir:** board `open` moduna döner, kartlar kilitli kalmaz. Testi Task 3'te.

---

### Task 1: Kendi rolünü düşürememe

**Files:** `internal/web/pages_admin.go`, `internal/web/pages_teams.go`, `templates/pages/admin_users.html`, `templates/pages/team.html`, kataloglar. Test: `internal/web/self_role_test.go`.

- [ ] **Testleri yaz:**
  - Admin kendi kaydına `set_admin value=0` ya da `set_disabled value=1` gönderir → 403 ve kayıt değişmez.
  - Lider kendisine `set_role role=member` ya da `remove_member` gönderir → 403.
  - Kendi satırında düğmeler `disabled` olarak render edilir.
  - Admin başkasının adminliğini kaldırabilir.
- [ ] **Kalan testleri gör.**
- [ ] **Uygula:**
  - `adminUsersPost`'ta hedef kendisiyse 403 ve flash `users.self_change`.
  - `changeMember`'da hedef kendisiyse 403 ve flash `team.self_change`.
  - Template'lerde `.Me` / `.MyID` karşılaştırmasıyla düğmeler pasif ve açıklamalı (`title`).
- [ ] **Testler geçer; commit:** `fix(web): nobody changes their own role`.

### Task 2: Store eklemeleri (alan kaydı, toplu kolon, rol adı)

**Files:** `internal/store/cards.go`, `internal/store/columns.go`, `internal/store/rules.go`. Test: `internal/store/redesign_test.go`.

**Interfaces (Produces):**

```go
// cards.go
type CardField string // "title","description","assignee","due_date","priority","estimate"
func (s *Store) UpdateCardField(ctx context.Context, boardID, cardID int64, expectedVersion int, field CardField, f CardFields, actorID int64) (Card, error)
// f'nin yalnız field'a karşılık gelen değeri kullanılır; diğerleri kartın güncel halinden alınır.
// Kişi WIP'i (assignee), activity ve ErrConflict davranışı UpdateCard ile aynıdır.

// columns.go
type ColumnRow struct {
	ID              int64 // 0: yeni kolon
	Name            string
	WIPLimit        *int
	IsDone          bool
	AllowCreate     bool
	CountsPersonWIP bool
	Delete          bool
}
type ColumnRowError struct{ Index int; Err error } // Err: ErrColumnNotEmpty, ErrInUse, ErrNotFound
type ColumnsError struct{ Rows []ColumnRowError }
func (e *ColumnsError) Error() string
func (s *Store) SaveColumns(ctx context.Context, boardID int64, rows []ColumnRow) error
// rows sırası yeni sıradır. Tek transaction, board kilidi. Hata varsa
// *ColumnsError döner ve hiçbir şey yazılmaz. En az bir kolon kalmalı
// (yoksa ErrInUse, Index -1).

// rules.go
func (s *Store) RenameBoardRole(ctx context.Context, boardID, roleID int64, name string) error
```

- [ ] **Testleri yaz:**
  - `UpdateCardField`:
    - `title` değişir, `description` korunur.
    - Bayat sürüm `ErrConflict` döner.
    - `assignee` kişi WIP'ini aşarsa `RuleError` döner.
    - Activity'de yalnız değişen alan görünür.
  - `SaveColumns`:
    - Sıra, ad ve bayraklar değişir; yeni kolon eklenir, boş kolon silinir.
    - Kartlı kolonu silme ve "nereden" yetkisi olan kolonu silme `ColumnsError` döner; satır index'i doğrudur ve hiçbir şey yazılmamıştır.
    - Başka board'un kolon id'si `ErrNotFound` döner.
    - Hepsini silme reddedilir.
  - `RenameBoardRole`: yeniden adlandırma çalışır; başka board'un rolü `ErrNotFound` döner.
- [ ] **Kalan testleri gör, uygula, testler geçer.**
- [ ] **Commit:** `feat(store): field-level card updates, bulk column saves, role renaming`.

### Task 3: Kural cümleleri

**Files:** `internal/store/sentences.go`. Test: `internal/store/sentences_test.go`.

**Interfaces:**

```go
type SentenceKind string
const (
	SentencePermission SentenceKind = "permission" // X'e yalnızca {kimler}; From varsa "Y'den"
	SentenceFrom       SentenceKind = "from"       // X'e yalnızca {kolonlar}dan gelinebilir
	SentenceCondition  SentenceKind = "condition"  // X'e girerken/çıkarken {şart}
	SentenceWIP        SentenceKind = "wip"        // X'te en fazla N
	SentencePersonWIP  SentenceKind = "person_wip" // kişi başına N (işaretli kolonlar)
)
type Sentence struct {
	Kind          SentenceKind
	Key           string   // silme kimliği: "permission:<to>:<from|0>", "from:<to>", "condition:<id>", "wip:<col>", "person_wip"
	ColumnID      int64    // X (person_wip için 0)
	FromID        int64    // permission için kaynak (0: her kolon)
	Subjects      []string // permission: rules.Subject* değerleri (board_role hariç)
	RoleIDs       []int64  // permission: board rolleri
	Columns       []int64  // from: kaynak kolonlar; person_wip: sayılan kolonlar
	Phase         string   // condition: enter | exit
	ConditionKind string   // condition: rules.Kinds'tan biri
	Params        rules.ConditionParams
	Limit         int      // wip, person_wip
}
func (s *Store) BoardSentences(ctx context.Context, boardID int64) ([]Sentence, error)
func (s *Store) DeleteSentence(ctx context.Context, boardID int64, key string) error
// "from:<to>": o hedefin bütün geçişlerini siler; hiç geçiş kalmazsa transitions_mode=open.
// "permission:<to>:<from>": o grubun bütün satırları.
// "wip:<col>": wip_limit=NULL. "person_wip": person_wip_limit=NULL.
func (s *Store) AddFromSentence(ctx context.Context, boardID, to int64, from []int64) error
// O hedefin geçişlerini from ile değiştirir ve board'u restricted yapar. Diğer
// hedeflerin geçişleri korunur; restricted'a ilk geçişte, o ana kadar serbest
// olan diğer hedefler için bütün kaynaklar izinli olarak yazılır, böylece
// cümle eklemek başka kolonları kilitlemez.
func (s *Store) SetWIPLimit(ctx context.Context, boardID, columnID int64, limit *int) error
func (s *Store) SetPersonWIP(ctx context.Context, boardID int64, limit *int, counted []int64) error
```

- [ ] **Testleri yaz:**
  - `BoardSentences` her türü doğru gruplar; aynı hedef ve kaynağın iki yetki satırı tek cümle olur.
  - `AddFromSentence`:
    - İlk eklemede diğer hedefler kilitlenmez; kural motoruyla serbest bir taşıma hâlâ geçer.
    - İkinci hedef eklenince o hedefin kaynakları değişir.
  - `DeleteSentence("from:<to>")` son geçişi silince mod `open` olur.
  - `DeleteSentence("permission:…")` yalnız o grubu siler.
  - Başka board'un anahtarı `ErrNotFound` döner.
- [ ] **Uygula, testler geçer.**
- [ ] **Commit:** `feat(store): rules as sentences`.

### Task 4: Web action'ları (alan kaydı, toplu kolon, roller, kurallar, onay)

**Files:** `internal/web/pages_card.go`, `internal/web/pages_board_settings.go`, `internal/web/confirm.go`. Test: `internal/web/redesign_test.go`.

- **Kart:** `op=set_field` (`field`, `value`, `expected_version`). Script'li istekte cevap JSON değil, panel fragment'ıdır. Hata ya da ihlalde 422 ve alanın yanında mesaj. Script'siz istekte karta 303 ve flash.
- **Ayarlar** (`?tab=general|columns|labels|roles|rules`):
  - `columns_save`: `col_id[]`, `col_name[]`, `col_wip[]`, `col_create`, `col_done`, `col_person[]`, `col_delete[]`, `col_order[]`; satır index'li alanlar. Hatada 422 ile tablo, girilen değerler ve satır mesajları yeniden gösterilir.
  - Roller: `role_rename` (`role_id`, `role_name`); `role_members`.
  - Kurallar: `rule_add` (`sentence=permission|from|condition|wip|person_wip` ve alanları) ve `rule_delete` (`key`).
- **Onay akışı:** `confirm.go`, tehlikeli işlemler (`archive`, `*_delete`, `remove_member`, `archive_board`, `set_disabled value=1`) için `confirm=1` yoksa bir onay sayfası render eder. Sayfa aynı formu gizli alanlarla ve `confirm=1` ile tekrar gönderir.
- [ ] **Testler önce:**
  - `set_field` her alan için çalışır.
  - Bayat sürümle başka alan ezilmez (Review Focus 1).
  - `columns_save` başarılı olur; hatada hiçbir şey kaydedilmez ve değerler geri basılır (Review Focus 2).
  - Rolü yeniden adlandırma.
  - Her cümle türünün eklenmesi ve silinmesi.
  - `confirm` olmadan arşivleme onay sayfası döner, `confirm=1` ile arşivler.
  - Eski `column_update`/`column_move`/`condition_add`/`permission_add` testleri yeni işlemlere taşınır.
- [ ] **Uygula, testler geçer.**
- [ ] **Commit:** `feat(web): field saves, bulk columns, role editing, rule sentences, confirmations`.

### Task 5: Görsel dil (CSS) ve iskelet

**Files:**
- `static/css/{tokens,base,components,layout}.css`
- `templates/partials/icons.html` (SVG `<symbol>` seti, `{{template}}` ile)
- `templates/layouts/base.html`, `templates/layouts/app.html` (sol çubuk)
- `static/js/app.js` (çubuğu daralt/aç, mobil çekmece, flash bildirim baloncukları)
- `internal/web/layouts.go`: app layout verisine takımlar ve board'ları eklenir, sol çubuk için.

İçerik:
- Spec §3'teki tokenlar `:root` ve `@media (prefers-color-scheme: dark)` içinde.
- Bileşenler: `.btn` (`--primary`, `--secondary`, `--quiet`, `--danger`, `--sm`), `.field`, `.input`, `.select`, `.check`, `.card`, `.panel`, `.chip[data-color]`, `.avatar[data-hue]`, `.badge`, `.tabs`, `.table`, `.dialog`, `.toast`, `.empty`, `.alert`, `.wip[data-state]`.
- Avatar rengi: `data-hue="0..7"`, `userID % 8`.

- [ ] Uygula.
- [ ] Mevcut web testleri geçer (template adları ve metinler korunur, gerekirse test beklentileri güncellenir).
- [ ] **Commit:** `feat(ui): warm paper design tokens, components and sidebar shell`.

### Task 6: Board ve kart paneli

**Files:** `templates/pages/board.html`, `templates/fragments/columns.html`, `templates/pages/card.html`, `templates/fragments/card_panel.html`, `static/css/board.css`, `static/css/card.css`, `static/js/{board,drawer,autosave,confirm}.js`.

- **Board:** spec §4.2. Kolon başlığında renk noktası (`data-color` = palette[index % 8]), WIP durumu `data-state="ok|full|over"`, kolon altında satır içi "Kart ekle" (kart açılabilen kolonlarda), kartta ek sayısı. Bunun için `CardSummary`'ye `Attachments int` eklenir (store).
- **Panel:** spec §4.3. Özellik ızgarasında her alan `data-autosave` formudur; sekmeler (Detay, Yorumlar, Etkinlik) URL hash'iyle seçilir.
- **`drawer.js`:**
  - Board'da kart linkini yakalar ve `…/panel`'i sağdan açılan `<aside class="drawer">` içine yükler.
  - `pushState` kullanır; Esc ve geri tuşu paneli kapatır.
  - collage-live push'u için paneli `data-collage-fragment` ile işaretler.
- **`autosave.js`:**
  - `change` ya da `blur`'da form'u fetch ile gönderir.
  - 200'de panel `collageLive.put` ile yerleştirilir; ancak kullanıcı başka bir alana yazıyorsa o alanın değeri korunur. Korunacak alanın adı istekten önce saklanır ve put'tan sonra geri yazılır.
  - 422'de alan eski değerine döner ve mesaj alanın yanında görünür.
- **`confirm.js`:** `form[data-confirm]` gönderilirken `<dialog>` açar; onaylanırsa `confirm=1` ekleyip gönderir.
- [ ] Uygula.
- [ ] `node --check` her modül için geçer; web testleri geçer.
- [ ] **Commit:** `feat(ui): board and card drawer with field autosave`.

### Task 7: Ayarlar sekmeleri

**Files:** `templates/pages/board_settings.html` (sekme başına bölümler), `static/css/settings.css`, `static/js/columns-editor.js` (Sortable ile satır sırası, `col_order` alanını yazar, kirli sayaç, `beforeunload` uyarısı), `static/js/rules.js` (cümle türü seçilince ilgili alanları göster/gizle).

- [ ] Uygula; web testleri geçer.
- [ ] **Commit:** `feat(ui): settings tabs, column table and rule sentences`.

### Task 8: Diğer sayfalar

Home ("Board'larım"), Takımlar, takım sayfası, Kullanıcılar, Bildirimler, Ayarlarım, Bana atananlar, board etkinliği, hata sayfaları ve onay sayfası yeni bileşenlerle yazılır. Her listenin bir boş durumu olur.

- [ ] Uygula; web testleri geçer.
- [ ] **Commit:** `feat(ui): restyle the remaining pages`.

### Task 9: Görsel doğrulama

- **Hazırlık:** puppeteer-core scratchpad'e kurulur. Yerel Authentik üzerinden `login.sh` ile alınan cookie'ler tarayıcıya verilir.
- **Ekranlar:** board, açık kart paneli, ayarlar (kolonlar ve kurallar), takım, bildirimler ve onay diyaloğu; açık ve koyu temada (`emulateMediaFeatures`), 1440 px ve 390 px genişlikte.
- **Kontrol:** görüntüler okunur. Kayan, taşan, okunmayan ya da ayrışmayan öğe varsa düzeltilir ve tekrar alınır.
- [ ] Bulunan düzeltmeler commit'lenir: `fix(ui): …`.

### Task 10: Son gözden geçirme

Bağımsız bir gözden geçirici (en yetkin model) değişiklik aralığını okur. Kritik ve önemli bulgular testle düzeltilir.
