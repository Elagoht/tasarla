# Kanban — tasarım spec'i

Tarih: 2026-09-30 · Collage sürümü: v0.38.0 · Durum: taslak, gözden geçirilecek

Birkaç takımın günlük iş takibi için kullanacağı, kuralları board başına yapılandırılabilen
bir kanban uygulaması. Collage ile sunucu tarafında render edilir; istemcide yalnızca
sürükle-bırak ve kısmi yenileme için küçük bir JS katmanı bulunur.

---

## 1. Kararlar ve gerekçeleri

| Konu             | Karar                                                          | Neden                                                                            |
| ---------------- | -------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Mimari           | Tek binary: collage sayfaları + action'lar + Go domain katmanı | Kurallar tek yerde (Go) kalır; istemcide state kopyası yok                       |
| İstemci          | SortableJS + collage-live (v0.3.0+), framework yok             | Çoklu seçim, anlık filtre, klavyeyle DnD ve anlık senkron ilk sürümde gerekmiyor |
| Kurulum          | Her kuruluşa ayrı deploy + ayrı PostgreSQL                     | Uygulama tek kuruluş varsayar; kodda multi-tenancy yok                           |
| Giriş            | Tek genel OIDC istemcisi (authorization code + PKCE)           | İki IdP'nin farkı yapılandırmada kalır                                           |
| Takım/rol        | Uygulama içinde yönetilir                                      | IdP yalnızca kimliği doğrular; IdP'ler arası claim eşlemesi gerekmez             |
| Kurallar         | Kodda sabit bir **katalog**, parametreleri board başına        | Test edilebilir, sınırlı; serbest kural dili (DSL) yok                           |
| Veritabanı       | PostgreSQL                                                     | Kuruluş tercihi                                                                  |
| Dosyalar         | Diskte bir volume'da, dosya başına 5 MB                        | Depolama ihtiyacı küçük                                                          |
| Canlı güncelleme | collage-live push (tag invalidation), poll'a otomatik düşer    | `InvalidateTags` zaten yazılacak; push bedava geliyor                            |
| Dil              | TR + EN, collage-i18n kataloglarıyla                           | Varsayılan dil kurulum başına yapılandırılır                                     |
| Bildirim         | Uygulama içi + e-posta (SMTP), outbox tablosuyla               | Gönderim yeniden denenebilir, kaybolmaz                                          |

### Kullanılacak collage eklentileri

- `collage-session` (v0.2.1): imzalı/şifreli cookie oturumu, `RequireUser` guard'ı
- `collage-i18n` (v0.2.0, `strict: true`): `{{t}}`, `{{tn}}`, `i18n.T(rc, …)`, render dışında `In(locale)`
- `collage-validate` (v0.1.3): form doğrulama, 422 ile yeniden render
- `collage-flash` (v0.1.2): redirect sonrası "kaydedildi" mesajları
- `collage-live` (v0.3.0): fragment yenileme, push, `data-collage-swap="morph"`, `pause`/`resume`/`put`
- `collage-secure` (v0.1.3): güvenlik başlıkları + CSP nonce

Uygulamanın kendi bağımlılıkları (collage'ın değil): `pgx/v5`, `sqlc` (öneri), bir
migration aracı (`goose` ya da `tern`), `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3`.

---

## 2. Collage'da dikkat edilmesi gerekenler

Bu maddeler dokümandan doğrulandı. Hepsi sessizce yanlış davranışa yol açabilir.

1. **Guard'lı sayfalar cache'i paylaşır.** Guard, cache okunmadan önce çalışır. Ancak izin
   verilen tüm okuyucular aynı cache kaydını görür. Kişiye özel içerik gösteren her sayfa
   `Dynamic()` olmalı. Data handler'ı olan sayfa strateji belirtmezse zaten Dynamic
   çözülür. Yine de açıkça yazın ve **hiçbir özel sayfaya `Static()` ya da `Incremental()`
   koymayın**. Bunu bir testle zorunlu kılın. Kayıtlı sayfaların stratejisini okuyan bir
   API'nin var olup olmadığı doğrulanmadı; yoksa, özel sayfaları kuran tek bir yardımcı
   fonksiyon (`privatePage(...)`) `Dynamic()`'i kendisi çağırsın ve test bu fonksiyonu
   sınasın.
2. **Kendi URL'sindeki action'da guard çalışmaz.** `app.RegisterAction` ile kaydedilen
   action'ın sayfası olmadığı için guard'ı da yoktur. Action'ları mümkün olduğunca
   `WithAction` ile sayfanın URL'sine bağlayın; böylece sayfanın guard'ı action'ı da korur.
   Her durumda handler içinde yetki kontrolü yapın (§6).
3. **Gövde limiti.** Varsayılan 4 MB'tır. Dosya yükleme action'ı
   `WithMaxBodyBytes(5<<20 + 64<<10)` almalı. Aksi halde uygulamanın kendi "5 MB'ı aşıyor"
   mesajına hiç ulaşılmaz ve collage 413 döner.
4. **CSRF.** Form'larda `{{csrfToken}}` kullanın. `fetch` ile gönderilen isteklerde token
   `X-CSRF-Token` başlığında ya da `FormData` içindeki hidden input'ta gider. `fetch`
   isteklerine `Collage-Fetch: 1` başlığını ekleyin; redirect bu durumda
   `204 + Collage-Location` olarak döner.
5. **`app.Handle` ile bağlanan handler'larda CSRF, gövde limiti ve cache yoktur.** Bunu
   sadece GET dosya indirme için kullanın (§9).
6. **`rc.Page` sadece `WithAction` ile bağlı action'larda doludur.** Validasyon hatasında
   `validate.Refuse(rc, v, rc.Page)` kullanılır.
7. **Locale yalnızca URL'den belirlenir.** Varsayılan locale prefix'siz, diğeri `/tr/…` ya da
   `/en/…` biçimindedir. `Default` ve `Supported` kurulum başına env'den okunur. Yalnızca bir
   locale'de tanımlanmış bir path, o locale'e erişilemiyorsa kayıt sırasında
   `ErrLocaleUnreachable` hatası verir.
8. **"Any" kullanılmaz.** Data handler'lar `collage.Load[T]` ile tiplendirilir.

---

## 3. Kimlik doğrulama

### Akış

1. `GET /login`: `state`, `nonce` ve PKCE `code_verifier` üretilir, oturuma yazılır.
   Kullanıcı IdP'nin `authorization_endpoint` adresine yönlendirilir (`S256`).
2. `GET /auth/callback`: `state` karşılaştırılır. `code` token'a çevrilir. ID token
   JWKS ile doğrulanır: `iss`, `aud`, `exp` ve `nonce` kontrol edilir.
3. Kullanıcı `(issuer, sub)` çiftiyle bulunur ya da oluşturulur. **E-posta kimlik olarak
   kullanılmaz**; değişebilir. `email` ve `name` her girişte güncellenir.
4. `session.Regenerate()` çağrılır ve oturuma `session.UserKey` = kullanıcı id'si yazılır.
   Ardından `next` parametresindeki adrese dönülür. `next` sadece aynı origin'deki bir
   path olabilir (açık redirect'e karşı).
5. `POST /logout`: `session.Clear()` çağrılır. IdP `end_session_endpoint` sunuyorsa
   oraya yönlendirilir (RP-initiated logout). Sunmuyorsa `/login`'e dönülür.

Oturum sadece kullanıcı id'sini tutar. Her istekte kullanıcı veritabanından okunur, böylece
`disabled_at` dolu bir kullanıcı hemen dışarıda kalır. Bunu bir middleware yapar
(`app.Use`): kullanıcıyı context'e koyar, data handler'lar oradan okur.

> **collage v0.38.0 ya da sonrası gerekir.** Daha önceki sürümlerde `Config.Plugins`
> içindeki eklentilerin middleware'i `app.Use` middleware'inin *içinde* çalışıyordu,
> bu yüzden bu middleware'de `session.FromContext` `nil` dönüyordu
> (`framework-issues/001`). v0.38.0'dan beri bir eklentinin middleware'i, eklentinin
> kaydedildiği yerde çalışıyor: `Config.Plugins` dışta, `app.Use` içte. Bu
> middleware'in koyduğu kullanıcıyı okuması gereken bir eklenti (örneğin kullanıcıya
> göre sınırlama yapan bir rate limit), `app.Use(loadUser)`'dan sonra
> `app.RegisterPlugin` ile kaydedilir.

İlk admin: `ADMIN_EMAILS` env'indeki bir e-posta ile ilk kez giriş yapan kullanıcıya
`is_admin = true` verilir. Sonrasında adminlik uygulama içinden yönetilir.

### IdP gereksinimleri

**Kendi IdP'n** (JWKS'e çevrilirken) şunları sunmalı:

- `/.well-known/openid-configuration` (discovery)
- `authorization_endpoint`: `response_type=code`, PKCE `S256`
- `token_endpoint`: `client_secret_basic` ya da `client_secret_post`
- `jwks_uri`: RS256 ya da ES256 anahtarları, `kid` ile
- ID token claim'leri: `iss`, `aud`, `exp`, `iat`, `nonce`, `sub`, `email`, `name`
- Kayıtlı redirect URI: `https://<kurulum>/auth/callback`
- İsteğe bağlı: `end_session_endpoint`

Bu akış olmadan sadece token üreten bir IdP için ikinci bir adaptör gerekir. Bu spec o
senaryoyu kapsamıyor.

**Authentik:** "OAuth2/OpenID Provider" oluşturulur. Client tipi "confidential" seçilir,
redirect URI girilir, scope'lar `openid email profile` olur. "Subject mode" ayarı sabit
bir değer üretmeli: varsayılan "hashed user ID" uygundur. Kurulumdan sonra bu ayar
değiştirilmemeli, çünkü `sub` değişirse kullanıcılar yeni kullanıcı olarak oluşturulur.

---

## 4. Domain modeli

İsimler İngilizce, tipler PostgreSQL. Tüm tablolarda `created_at timestamptz` bulunur.

```
users            id, issuer, subject, email, name, locale ('tr'|'en'), is_admin,
                 disabled_at                    UNIQUE(issuer, subject)
teams            id, name, archived_at
team_members     team_id, user_id, role ('lead'|'member')        PK(team_id, user_id)

boards           id, team_id, name, archived_at,
                 transitions_mode ('open'|'restricted'),
                 person_wip_limit int NULL
board_roles      id, board_id, name                               UNIQUE(board_id, name)
board_role_members  role_id, user_id                              PK(role_id, user_id)
columns          id, board_id, name, position, wip_limit int NULL,
                 is_done bool, allow_create bool, counts_person_wip bool

transitions      board_id, from_column_id, to_column_id           PK(from, to)
move_permissions id, board_id, to_column_id, from_column_id NULL,
                 subject ('any_member'|'assignee'|'team_lead'|'board_role'),
                 board_role_id NULL
column_conditions id, column_id, phase ('enter'|'exit'), kind, params jsonb

cards            id, board_id, column_id, position int, title, description,
                 assignee_id NULL, estimate numeric NULL, due_date date NULL,
                 priority smallint NULL, created_by, version int, archived_at
labels           id, board_id, name, color
card_labels      card_id, label_id
checklist_items  id, card_id, text, done, position
card_dependencies blocker_id, blocked_id     PK(blocker_id, blocked_id), CHECK(blocker <> blocked)
comments         id, card_id, author_id, body, edited_at, deleted_at
comment_mentions comment_id, user_id
attachments      id, card_id, uploader_id, filename, content_type, size, storage_key
activity         id, board_id, card_id NULL, actor_id, kind, payload jsonb

notifications    id, user_id, kind, card_id NULL, payload jsonb, read_at NULL,
                 dedupe_key UNIQUE
notification_prefs user_id, kind, email bool      PK(user_id, kind)
email_outbox     id, to_address, subject, html, text, status, attempts,
                 next_attempt_at, last_error
```

Notlar:

- **Sıralama:** `position` kolon içinde 0'dan başlayan tamsayıdır. Bir taşıma, hedef
  kolondaki (ve kaynak kolondaki) sırayı aynı transaction içinde yeniden numaralar. Bir
  kolonda en fazla birkaç yüz kart olacağı için bu yöntem yeterli ve basit.
- **`version`:** Kartta yapılan her değişiklikte bir artar. Taşıma ve düzenleme
  isteklerinde gönderilir (§5.3).
- **Bağımlılık döngüsü:** `A → B` eklenmeden önce `B`'den `A`'ya ulaşan bir yol var mı
  diye recursive CTE ile bakılır. Varsa ekleme reddedilir.
- **Rol modeli:**
  - Global: `is_admin`. Tüm takımları ve board'ları yönetebilir.
  - Takım: `lead`. Takımın board'larını ve ayarlarını yönetir. `member` board'ları
    kullanır.
  - Board rolleri serbest isimlidir ("Reviewer", "QA"). Yalnızca kurallarda kullanılır.
  - Board'a erişim takım üyeliğiyle verilir. Board'a özel üyelik yok.

---

## 5. Kural motoru

Uygulamanın özü bu bölüm. Motor saf bir Go paketidir (`internal/rules`): veritabanına,
HTTP'ye ya da collage'a bağımlı değildir.

### 5.1 Katalog

| Tür       | Yapılandırma                                           | Anlamı                                                                                                                                                       |
| --------- | ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Geçiş     | `transitions_mode`, `transitions`                      | `open`: her kolondan her kolona gidilebilir. `restricted`: yalnızca tabloda olan `(from, to)` çiftleri                                                       |
| Yetki     | `move_permissions`                                     | Bir kolona (ya da belirli bir geçişe) kimin taşıyabileceği. Hedef için hiç kayıt yoksa takımın her üyesi taşıyabilir. Birden fazla kayıt "veya" ile birleşir |
| Kolon WIP | `columns.wip_limit`                                    | Kolondaki arşivlenmemiş kart sayısı limiti aşamaz                                                                                                            |
| Kişi WIP  | `boards.person_wip_limit`, `columns.counts_person_wip` | Atanan kişinin işaretli kolonlardaki kart sayısı limiti aşamaz                                                                                               |
| Koşul     | `column_conditions`                                    | Kolona girerken (`enter`) ya da kolondan çıkarken (`exit`) kartın sağlaması gereken durum                                                                    |

Koşul türleri (`kind`):

| `kind`               | `params`                                    | Kart şu durumda geçer                                              |
| -------------------- | ------------------------------------------- | ------------------------------------------------------------------ |
| `has_assignee`       | —                                           | Atanan kişi var                                                    |
| `has_estimate`       | —                                           | Tahmin girilmiş                                                    |
| `has_due_date`       | —                                           | Son tarih var                                                      |
| `has_description`    | —                                           | Açıklama boş değil                                                 |
| `has_label`          | `{"label_ids": [..]}` (boşsa herhangi biri) | Etiketlerden en az biri var                                        |
| `checklist_complete` | —                                           | Tüm checklist maddeleri işaretli (hiç madde yoksa geçer)           |
| `blockers_done`      | —                                           | Bu kartı bloklayan tüm kartlar `is_done` bir kolonda ya da arşivde |
| `min_attachments`    | `{"count": n}`                              | En az n dosya eki var                                              |

Yeni bir kural türü eklemek; bir `kind` sabiti, bir değerlendirme fonksiyonu, bir ayar
formu parçası ve iki çeviri anahtarı demektir. Katalog bilinçli olarak kapalı tutulur.

### 5.2 Değerlendirme

```go
type Move struct {
    CardID, FromColumnID, ToColumnID int64
    ToIndex                           int
}

type Violation struct {
    Code   string            // "rules.wip_column", "rules.condition.has_estimate", ...
    Params map[string]string // çeviri için: {"column": "In Progress", "limit": "3"}
}

// Evaluate returns every rule the move breaks, not only the first.
func Evaluate(actor Actor, move Move, snap Snapshot) []Violation
```

`Snapshot`: board'un kuralları, kolonlar ve kolon sayımları, kart ve alanları, kartın
blokçuları, kişi WIP sayımı. Bir taşımayla ilgili her veri tek sorguda okunur.

Semantik:

- **Aynı kolonda sıra değiştirmek hiçbir kurala tabi değildir.** Yalnızca yetki kontrolü
  yapılır: board'un bir üyesi olmak yeterlidir.
- Sırası: geçiş, yetki, çıkış koşulları (kaynak), giriş koşulları (hedef), kolon WIP,
  kişi WIP. **Tüm ihlaller toplanır** ve kullanıcıya birlikte gösterilir.
- **Koşullar kapıdır, değişmez değildir (invariant değil).** Kart kolona girdikten sonra
  tahmini silinirse ihlal oluşmaz. Koşullar yalnızca kolona giriş ve kolondan çıkış anında
  değerlendirilir.
- **Kart oluşturma**, `allow_create` işaretli bir kolona yapılan bir "giriş" sayılır. O
  kolonun giriş koşulları ve WIP limiti uygulanır. Hiçbir kolonda `allow_create` yoksa ilk
  kolon kullanılır.
- **Kural değişikliği geriye dönük değildir.** WIP limiti mevcut sayının altına
  düşürülebilir. Kolon "limit aşıldı" olarak gösterilir; yalnızca yeni girişler engellenir.
- **Atanan kişinin değişmesi** kişi WIP kontrolünü tetikler. Kart sayılan bir kolondaysa,
  yeni atanan kişinin limiti kontrol edilir.
- Board ayarlarını değiştirebilen kişiler: admin ve takımın lead'leri. Kurallardan muaf
  "zorla taşı" yetkisi yok (§12).

### 5.3 Taşıma transaction'ı

```
BEGIN
  SELECT … FROM boards WHERE id = $board FOR UPDATE      -- board başına taşımaları sıraya sokar
  kartı oku; column_id <> expected_from OR version <> expected_version → çakışma
  snapshot'ı oku → rules.Evaluate
  ihlal var → ROLLBACK, 422
  pozisyonları yeniden numarala, kartı taşı, version++
  activity satırı ekle, bildirimleri oluştur
COMMIT
InvalidateTags("board:<id>", "card:<id>")
```

Board satırındaki kilit, WIP yarışını çözer: iki kişi aynı anda son boş yere kart taşırsa
ikincisi güncel sayıyı görür. Board başına taşıma trafiği küçük olduğu için bu kilitlemenin
maliyeti önemsiz.

`expected_from` ve `expected_version`, kullanıcının gördüğü kartın hâlâ o kart olduğunu
doğrular. Başkası kartı arada taşımışsa istek 409 ile reddedilir ve board güncel haliyle
yeniden çizilir.

### 5.4 Testler

- `internal/rules` için tablo tabanlı unit testleri: her kural türü için geçen ve geçmeyen
  vakalar, birden fazla ihlalin birlikte döndüğü vakalar, aynı kolonda sıralama, oluşturma
  anında giriş, limit altına düşürülmüş WIP.
- Gerçek PostgreSQL'e karşı entegrasyon testi: WIP'e bir yer kalmışken aynı anda iki taşıma
  (iki goroutine). Tam olarak biri kabul edilmeli.
- Döngü engelleme: `A → B → C` varken `C → A` reddedilmeli.

---

## 6. Yetkilendirme

`internal/authz` paketindeki fonksiyonlar her data handler'ın ve her action'ın başında
çağrılır:

```go
func BoardAccess(ctx context.Context, user User, boardID int64) (Access, error)
// Access{CanView, CanEdit, CanManage}
```

- Kullanıcı board'un takımında değilse `collage.ErrNotFound` sarılı bir hata döner (404,
  403 değil). Başka takımların board'larının varlığı böylece sızmaz.
- Guard (`session.RequireUser("/login")`) sadece "giriş yapılmış mı" sorusuna cevap verir.
  Board düzeyindeki yetki handler içinde kontrol edilir.
- URL'deki her id (kart, yorum, ek) kendi board'una bağlanarak doğrulanır:
  `WHERE id = $card AND board_id = $board`. Bu, IDOR açığına karşı zorunlu.

---

## 7. Sayfalar, fragment'lar ve action'lar

Path'ler iki locale'de de aynıdır (`/boards/12`, `/tr/boards/12`). Bu tercih kasıtlı: URL'ler
paylaşılabilir kalır ve path'leri çevirmek hiçbir kazanç sağlamaz.

| Sayfa          | Path                        | Fragment path'leri                    | Action'lar (`WithAction`)                                 |
| -------------- | --------------------------- | ------------------------------------- | --------------------------------------------------------- |
| Giriş          | `/login`                    | —                                     | — (GET redirect)                                          |
| Callback       | `/auth/callback`            | —                                     | —                                                         |
| Board listesi  | `/`                         | —                                     | —                                                         |
| Board          | `/boards/{id}`              | `/boards/{id}/columns` (push + morph) | `POST` taşı, kart oluştur                                 |
| Kart detayı    | `/boards/{id}/cards/{card}` | `…/panel` (dialog içinde)             | alan güncelle, checklist, bağımlılık, yorum, ek, arşivle  |
| Board ayarları | `/boards/{id}/settings`     | —                                     | kolonlar, geçişler, yetkiler, koşullar, etiketler, roller |
| Takımlar       | `/teams`, `/teams/{id}`     | —                                     | üye ekle/çıkar, rol değiştir                              |
| Bana atananlar | `/me/tasks`                 | —                                     | —                                                         |
| Bildirimler    | `/notifications`            | `/notifications/badge` (push)         | okundu işaretle                                           |
| Profil         | `/me/settings`              | —                                     | dil, e-posta tercihleri                                   |
| Çıkış          | `/logout`                   | —                                     | `POST`                                                    |

Bir sayfada birden fazla action olduğunda method ya da gizli bir `op` alanıyla ayrılır.
Kart detay sayfasının bütün formları aynı URL'ye post eder ve `op=add_comment` gibi bir
alan taşır. Alternatif olarak action'lar ayrı fragment path'lerine bölünebilir; bu
uygulamayı yazanın tercihine kalmış.

Ortak layout'lar:

- `layouts/base`: `<html>`, `{{liveClient}}`, CSS, dil değiştirici (`{{localeURL}}`).
- `layouts/app`: `WithGuard(session.RequireUser("/login"))`, üst bar ve bildirim rozeti.
  Özel sayfaların hepsi bu layout'u kullanır.

Data handler'ların döndürdüğü tag'ler:

| Tag                      | Invalidate eden                                         |
| ------------------------ | ------------------------------------------------------- |
| `board:<id>`             | taşıma, kart oluşturma/düzenleme/arşivleme, board ayarı |
| `card:<id>`              | o kartta yapılan her değişiklik                         |
| `notifications:<userId>` | o kullanıcıya bildirim eklenmesi, okundu işaretlenmesi  |

Kart düzenlemesi board'daki kart görünümünü de (başlık, etiket, atanan) değiştirdiği için
her iki tag de invalidate edilir.

---

## 8. İstemci katmanı

Tek bir dosya: `/static/board.js` (ES module), buna ek olarak vendor edilmiş `Sortable.min.js`.
DOM morph'u collage-live yapıyor. Build adımı yok.

### Sürükle-bırak

1. Her `.column__cards` listesinde bir SortableJS örneği çalışır ve hepsi aynı `group`'u
   paylaşır.
2. `onStart`: `collageLive.pause(evt.from)`. Board, sürükleme bitene kadar push ya da
   poll ile değişmez.
3. `onEnd`: `FormData` hazırlanır: `card`, `to_column`, `to_index`, `expected_from`,
   `expected_version`, CSRF alanı. `fetch(POST)` başlıkları `Collage-Fetch: 1` ve
   `X-CSRF-Token` içerir.
4. Cevap, kolonlar fragment'idir:
   - `200`: güncel board.
   - `422`: board'un gerçek hali, ve üstünde ihlal mesajlarını gösteren bir banner.
   - `409`: güncel board, ve "bu kart başka biri tarafından değiştirildi" mesajı.

   Her üç durumda da cevap `collageLive.put(evt.from, html)` ile uygulanır ve board
   `data-collage-swap="morph"` olduğu için yerinde yamanır. **Geri alma
   ayrıca yazılmaz**: sunucunun döndürdüğü board gerçek durumu gösterdiği için, reddedilen
   bir taşımada kart kendiliğinden eski yerine döner.
5. `finally` içinde `collageLive.resume(evt.from)` çağrılır. Sürükleme sırasında gelen
   en son push varsa o an uygulanır.

Diğer kullanıcılar değişikliği collage-live push'u ile görür (`board:<id>` tag'i).

### Kart detayı

Board'daki kart bir `<a href="…/cards/{card}">` bağlantısıdır. JS varsa tıklama engellenir,
`…/panel` fragment'i `<dialog>` içine yüklenir ve `history.pushState` ile URL güncellenir.
JS yoksa bağlantı tam sayfa olarak açılır. Dialog içindeki formlar
`data-collage-target` ile panelin kendisini hedefler.

### Erişilebilir yedek yol

Her kartın detayında bir "Kolona taşı" `<select>` ve formu bulunur. Aynı taşıma action'ına
post eder. Klavyeyle ya da ekran okuyucuyla gezen kullanıcı sürüklemeden kart taşıyabilir.

### CSP

`collage-secure` ayarı: `script-src 'self' 'nonce-{nonce}'`. Tüm scriptler `/static/`
altından dış dosya olarak yüklenir; inline script yok.

---

## 9. Dosya ekleri

- Yükleme action'ı: `WithMaxBodyBytes(5<<20 + 64<<10)`. 5 MB'ı aşan dosya, form üzerinde
  422 ve mesajla reddedilir.
- İçerik türü `http.DetectContentType` ile ilk 512 bayttan belirlenir; tarayıcının
  gönderdiği değer kullanılmaz. Dosya adı saklanır ama dosya diskte `ATTACHMENTS_DIR/<uuid>`
  olarak yazılır.
- İndirme: `app.Handle("/files/", filesHandler)`. Handler oturumu
  `session.FromContext(r.Context())` ile okur ve eki board yetkisiyle kontrol eder.
  `os.OpenRoot` ile açar ve `http.ServeContent` ile sunar.
  - PNG, JPEG, GIF ve WebP: `inline`. Diğerleri: `Content-Disposition: attachment`.
  - `X-Content-Type-Options: nosniff`. SVG her zaman `attachment` (script içerebilir).
- `app.Mount` **kullanılmaz**: mount'lar yetki kontrolü yapmaz.

---

## 10. Bildirimler ve e-posta

### Olaylar

| `kind`      | Kime                                                    | E-posta varsayılanı |
| ----------- | ------------------------------------------------------- | ------------------- |
| `assigned`  | Kart kendisine atanan kişiye                            | açık                |
| `mentioned` | Yorumda `@` ile anılan kişiye                           | açık                |
| `commented` | Kartın atanan kişisine (yorumu yazan kendisi değilse)   | kapalı              |
| `due_soon`  | Atanan kişiye, son tarihe 24 saat kala, bir kez         | açık                |
| `overdue`   | Atanan kişiye, son tarih geçince, bir kez               | açık                |
| `unblocked` | Bloklanan kartın atanan kişisine, tüm blokçular bitince | kapalı              |

Kullanıcı kendi yaptığı işlem için bildirim almaz.

`dedupe_key` (ör. `due_soon:<card>:<due_date>`) aynı bildirimin iki kez oluşmasını
engeller. Son tarih değişirse anahtar da değiştiği için yeni hatırlatma üretilir.

### Mention'lar

Yorum formunda `@` yazınca board üyelerinden oluşan bir öneri listesi çıkar
(`<datalist>` ya da küçük bir fragment). Gönderimde metindeki `@kullanıcı` ifadeleri board
üyelerine karşı çözülür ve `comment_mentions` tablosuna yazılır. Board üyesi olmayan bir
isim düz metin olarak kalır.

### Gönderim

- Bildirim oluşturan transaction, e-posta tercihi açık olan her alıcı için
  `email_outbox`'a bir satır ekler (outbox deseni). E-posta, işlemin kendisiyle birlikte
  commit edilir; SMTP'nin çökmüş olması işlemi bozmaz.
- Aynı process içindeki bir worker goroutine her 30 saniyede
  `SELECT … FOR UPDATE SKIP LOCKED` ile bekleyen satırları alır, gönderir, başarısız olanları
  artan gecikmeyle yeniden dener. 8 denemeden sonra `failed` olarak işaretler ve loglar.
- E-postanın dili **alıcının** `users.locale` tercihidir; işlemi yapan kişinin dili değil.
  İçerik `html/template` ile üretilir. HTML ve düz metin sürümü birlikte gönderilir. Kartın
  bağlantısı `BASE_URL` ve `app.URL` ile kurulur.
- Zamanlayıcı: her 15 dakikada `due_soon` ve `overdue` adaylarını tarar.
- Kapanışta worker'lar context iptaliyle durdurulur; o anda gönderilmekte olan e-posta
  tamamlanır.

---

## 11. Yapılandırma (env)

```
BASE_URL                 https://pano.kurulus.com
DATABASE_URL             postgres://…
DEFAULT_LOCALE           tr               # "tr" | "en"
OIDC_ISSUER              https://auth.kurulus.com/application/o/kanban/
OIDC_CLIENT_ID / OIDC_CLIENT_SECRET
OIDC_SCOPES              openid email profile
ADMIN_EMAILS             a@kurulus.com,b@kurulus.com
SESSION_KEY / CSRF_KEY / FLASH_KEY   # her biri 32 bayt, openssl rand -hex 32
ATTACHMENTS_DIR          /data/attachments
SMTP_HOST / SMTP_PORT / SMTP_USER / SMTP_PASSWORD / SMTP_FROM
```

Uygulama açılışta eksik ya da hatalı her değeri isimle raporlar ve başlamaz. OIDC
discovery isteği de açılışta yapılır; IdP'ye ulaşılamazsa uygulama başlamaz.

---

## 12. Açık konular ve riskler

1. **collage-live'da sürükleme sırasında duraklatma — çözüldü (collage-live v0.3.0).**
   `collageLive.pause(el)` / `resume(el)` elemanı sabit tutar; arada gelen en son içerik
   `resume`'da uygulanır. `collageLive.put(el, html)` taşıma cevabını live'ın kendi swap'inden
   geçirir. Akış: `onStart` → `pause(evt.from)`; `onEnd` → `fetch`, sonra
   `put(evt.from, cevap)`, en son `resume(evt.from)` (`finally` içinde).
2. **E-postada çeviri — çözüldü (collage-i18n v0.2.0).** i18n eklentisinin değeri saklanır;
   `tr.In(alıcı.Locale)` bir `Translator` döndürür. Konu satırı `T`/`TN` ile, gövde
   `template.New(…).Funcs(t.Funcs())` ile üretilir. Katalog farklılıklarının deploy'u
   durdurması için `strict: true` kullanılır.
3. **Kart açıklaması** ilk sürümde düz metin olacak: satır sonları korunur, bağlantılar
   otomatik linklenir. Markdown istenirse sanitize edilmesi gerekir (kullanıcı girdisi).
4. **Zorla taşıma (admin override)** kapsam dışı. Gerekirse `activity` kaydı zorunlu bir
   ayrıcalık olarak eklenir.
5. **Tek instance varsayımı.** `InvalidateTags` ve collage-live process içinde çalışır.
   İkinci bir instance eklenirse invalidation'ın instance'lar arasında dağıtılması gerekir
   (Postgres `LISTEN/NOTIFY` ile). İlk sürümde gerekmiyor.

## 13. Kapsam dışı

Multi-tenancy, board düzeyinde özel üyelik, serbest kural dili, çoklu seçim ve toplu taşıma,
istemci tarafında anlık filtre, klavyeyle sürükle-bırak, swimlane'ler, zaman takibi,
raporlar ve grafikler, dışa/içe aktarma, API token'ları, mobil uygulama.

---

## 14. Yapım sırası

Her aşama çalışan, deploy edilebilir bir sonuç verir.

### Aşama 1 — Temel

İskelet (`collage new`), config doğrulama, PostgreSQL ve migration'lar, OIDC girişi ve
çıkışı, kullanıcı middleware'i, takımlar ve üyeler, admin, i18n altyapısı ve TR/EN
katalogları, dil değiştirici, base ve app layout'ları, CSP.

**Kabul:** İki IdP'ye de (Authentik ve kendi IdP'n) bağlanan iki ayrı kurulum girişe izin
veriyor. `ADMIN_EMAILS` ilk admini veriyor. Admin takım oluşturup üye ekleyebiliyor.
Özel sayfaların Dynamic olduğu testle doğrulanıyor (§2, madde 1). OIDC akışı sahte bir issuer'a
karşı (discovery + JWKS sunan bir `httptest` sunucusu) test ediliyor; `nonce`, `state` ve
`aud` hatalarının her biri reddediliyor.

### Aşama 2 — Board ve kart

Board'lar ve kolonlar (henüz kural yok, `transitions_mode = open`). Kart CRUD ve tüm kart
alanları, etiketler, checklist, bağımlılıklar (döngü engelleme dahil). Sürükle-bırak ve
taşıma transaction'ı, version çakışması, collage-live push ile yenileme, "kolona taşı"
yedek yolu, `/me/tasks`.

**Kabul:** İki tarayıcıda aynı board açıkken birinde yapılan taşıma diğerinde birkaç saniye
içinde görünüyor. Bayat bir kartı taşımak 409 ile reddediliyor ve kart güncel yerine dönüyor.
Sürükleme sırasında gelen push board'u bozmuyor.

### Aşama 3 — Kural motoru ve ayarlar

`internal/rules` paketi ve testleri. Board ayarları sayfası: geçiş matrisi, yetkiler, WIP
limitleri, koşullar, board rolleri. İhlal mesajları TR ve EN.

**Kabul:** §5.4'teki testlerin hepsi geçiyor. Kural ihlal eden bir taşıma, tüm ihlalleri
kullanıcının dilinde listeleyen 422 ile reddediliyor ve kart eski yerine dönüyor. Kural
ayarlarını lead ve admin değiştirebiliyor, member değiştiremiyor.

### Aşama 4 — İşbirliği

Yorumlar ve mention'lar, activity akışı (kart detayında ve board düzeyinde), dosya ekleri.

**Kabul:** 5 MB'ı aşan bir dosya form üzerinde mesajla reddediliyor. Başka bir takımın ekine
doğrudan URL ile erişim 404 dönüyor. SVG her zaman indirme olarak sunuluyor. Her taşıma ve
her alan değişikliği activity akışında kim, ne ve ne zaman bilgisiyle görünüyor.

### Aşama 5 — Bildirimler

Uygulama içi bildirimler ve rozet, e-posta tercihleri, outbox ve worker, zamanlayıcı.

**Kabul:** SMTP kapalıyken yapılan işlemler başarılı oluyor, e-postalar kuyrukta bekliyor ve
SMTP açılınca gönderiliyor. `due_soon` bildirimi bir kez geliyor, son tarih değişince yeniden
geliyor. E-posta alıcının dilinde gidiyor.
