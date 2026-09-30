# Aşama 1 — Temel: Uygulama Planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** OIDC ile giriş yapılabilen, kullanıcıları PostgreSQL'de tutan, admin'in takım kurup üye yönetebildiği, TR/EN çalışan ve CSP'li, deploy edilebilir bir collage uygulaması.

**Architecture:** Tek Go binary'si. `main.go` yapılandırmayı okur, veritabanına bağlanıp migration'ları çalıştırır, OIDC discovery yapar ve `internal/web.New` ile collage uygulamasını kurar. Domain ve veri erişimi (`internal/store`, `internal/authz`) collage'dan bağımsızdır. Oturumdaki kullanıcı id'sini her istekte veritabanından kullanıcıya çeviren adım bir `app.Use` middleware'idir (`internal/auth`). Sayfalar `internal/web` içindedir, template'ler, statik dosyalar ve kataloglar kök dizindedir ve binary'ye gömülür.

**Tech Stack:** Go 1.26, collage v0.38.0, collage-session v0.2.1, collage-i18n v0.2.0, collage-validate v0.1.3, collage-flash v0.1.2, collage-live v0.3.0, collage-secure v0.1.3, pgx/v5 v5.11.0, tern/v2 v2.4.3, go-oidc/v3 v3.21.0, x/oauth2 v0.37.0, go-jose/v4 v4.1.5 (yalnızca testlerde), PostgreSQL 17 (Docker).

**Spec:** `kanban-spec.md` (§1, §2, §3, §4'ün users/teams/team_members kısmı, §6, §7'nin layout'ları, §11, §14 Aşama 1)

## Global Constraints

- **Framework kuralı:** collage ya da eklentilerinden biri dokümanından farklı davranırsa ya da gereken bir API yoksa geçici çözüm yazılmaz. `framework-issues/NNN-kisa-baslik.md` açılır: tür, paket ve sürüm, beklenen ve gözlenen davranış, en küçük tekrar kodu, olası geçici çözüm. Ardından **iş durdurulur** ve kullanıcıdan haber beklenir. Örnek: `framework-issues/001-app-middleware-cannot-see-plugin-context.md`.
- **Issue yalnızca dokümanda yazan bir ifade çiğnendiğinde açılır**, ve issue o ifadeyi alıntılar. Doküman bir çıktının biçimi konusunda sessizse (`/en` mi `/en/` mi, `nonce-` başlıkta her zaman var mı, template'te `eq` ile `Role`–`string` karşılaştırması gibi), testteki beklenti gözlenen davranışa göre düzeltilir ve commit mesajında belirtilir. Bu durum bir framework sorunu sayılmaz.
- **`any` / `interface{}` yazılmaz**, testler dahil. JWT claim'leri, discovery JSON'u ve JWKS gibi yapılar tipli struct'larla ifade edilir. Data handler'lar `collage.Load` / `collage.DataHandler` / `collage.Effect` ile yazılır. `pgx`'in `Scan(dest ...any)` gibi kütüphane imzalarını çağırmak serbesttir.
- Modül yolu `kanban`, Go `1.26`, collage **v0.38.0**.
- Eklentiler `Config.Plugins` içinde kaydedilir. collage v0.38.0'dan beri bu eklentilerin middleware'i her `app.Use`'un **dışında** çalışır, bu yüzden `app.Use(auth.Middleware)` `session.FromContext`'i okuyabilir (CHANGELOG v0.38.0 "Breaking").
- Locale yalnızca URL'den gelir. Desteklenen locale'ler `tr` ve `en`; varsayılanı `DEFAULT_LOCALE` belirler. Sayfa path'leri iki locale'de de aynıdır (`/teams`, `/en/teams`).
- Guard'lı her sayfa `Dynamic()`'tir ve bunu açıkça yazar. Hiçbir guard'lı sayfaya `Static()` ya da `Incremental()` konmaz (spec §2.1).
- Kişiye özel her sayfa `layouts/app` altında render edilir. O layout `session.RequireUser("/login")` guard'ını taşır.
- E-posta kimlik olarak kullanılmaz. Kullanıcı `(issuer, subject)` ile bulunur (spec §3).
- `next` yalnızca aynı origin'deki bir path olabilir.
- **IdP'nin kurulu düzenine uyulur; uygulama kendi yolunu ya da beklentisini dayatmaz.** Callback adresi IdP'de kayıtlı olandır ve `OIDC_REDIRECT_URL` ile verilir (Authentik test kurulumunda `https://kanboard-test.unoghub.org/auth/openid/authentik`). Callback action'ı o adresin path'ine kaydedilir.
- `email_verified` claim'ine bakılmaz: IdP yalnızca onaylı kullanıcıları gönderir. `groups` claim'i kullanılmaz: takım ve rol uygulama içinde yönetilir (spec §1).
- Tüm tablolarda `created_at timestamptz NOT NULL DEFAULT now()` bulunur.
- Veritabanı testleri `KANBAN_TEST_DATABASE_URL` yoksa **SKIP** olur. Bir görevin doğrulaması "PASS" diyorsa bu, `docker compose up -d` çalışırken ve `KANBAN_TEST_DATABASE_URL=postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable` tanımlıyken **SKIP'siz** PASS demektir.
- Kod yorumları ve tanımlayıcılar İngilizce, kullanıcıya görünen metinler katalogdan (TR + EN) gelir. `collage-i18n` `Strict: true` ile çalışır; iki katalog aynı anahtarları taşımalıdır.

## Spec'ten bilinçli sapmalar ve eklemeler (gözden geçir)

1. **sqlc yerine elle yazılmış pgx sorguları.** Spec §1'de sqlc "öneri" olarak geçiyor. Aşama 1'de üç tablo ve yaklaşık 15 sorgu var. Kod üretimi bir araç ve build adımı ekler; şimdilik gerekmiyor.
2. **Migration aracı: `tern/v2`.** pgx'e özgü; migration'lar gömülü `.sql` dosyaları, açılışta çalışır.
3. **`/admin/users` sayfası eklendi.** Spec "adminlik uygulama içinden yönetilir" diyor ama bunun için bir sayfa tanımlamıyor. Bu sayfa kullanıcıları listeler; admin yap/kaldır ve devre dışı bırak/etkinleştir işlemlerini sunar. Son aktif admin kaldırılamaz.
4. **Takıma üye ekleme e-postayla yapılır.** Kullanıcının daha önce en az bir kez giriş yapmış olması gerekir, çünkü kullanıcı kaydı ilk girişte oluşur.
5. **Ana sayfa (`/`) bu aşamada "Takımlarım" listesidir.** Board listesi Aşama 2'de gelir.
6. **Bildirim rozeti ve `/me/settings` bu aşamada yok** (Aşama 5). `users.locale` ilk girişte `DEFAULT_LOCALE` ile doldurulur.
7. **`ATTACHMENTS_DIR` ve `SMTP_*` henüz okunmaz.** Onları kullanan aşamada eklenirler.
8. **Callback yolu sabit değil.** Spec §3 `GET /auth/callback` diyor; plan callback'i IdP'de kayıtlı adrese, yani `OIDC_REDIRECT_URL`'nin path'ine kaydeder. Bu adresin origin'i `BASE_URL` ile aynı olmak zorundadır.
9. **Ad boş gelebilir.** Authentik test kurulumunda `name` boş geliyor. Kullanıcının adı sırasıyla `name`, `preferred_username`, e-posta olarak seçilir.

## Authentik'ten öğrenilenler (2026-09-30)

Test provider'ının discovery belgesi ve bir gerçek girişle doğrulandı.

| Konu | Durum | Plandaki karşılığı |
| --- | --- | --- |
| Sürüm | 2026.8.3 | — |
| Issuer | `https://workspace.unoghub.org/application/o/kanboard-test-uygulamasi/`, sonunda `/` var | `OIDC_ISSUER` olduğu gibi kullanılır |
| Redirect URI | `https://kanboard-test.unoghub.org/auth/openid/authentik` | `OIDC_REDIRECT_URL` (Task 1, 5, 9) |
| İmza | RS256, JWKS'te RSA anahtarı | go-oidc varsayılanı; değişiklik yok |
| Client kimlik doğrulaması | `client_secret_basic` ve `client_secret_post` | x/oauth2 otomatik seçer; değişiklik yok |
| PKCE | `S256` destekleniyor | Plan zaten `S256` kullanıyor |
| Claim'ler | ID token'da geliyor: `email`, `preferred_username`, `nickname`, `groups`, `sid`; **`name` boş** | Ad için `preferred_username`'e düşülür (Task 4) |
| `email_verified` | `false` geliyor; henüz e-posta altyapıları yok, onaysız kullanıcı gelmiyor | Claim'e bakılmaz; `ADMIN_EMAILS` e-postayla eşleşir |
| `sub` | 64 hex, "hashed user ID" | `(issuer, sub)` kimliği; değişiklik yok |
| Token ömrü | ID ve access token 5 dk | Token saklanmadığı için etkisi yok |
| Çıkış | `end_session_endpoint` var; back-channel logout destekleniyor | RP-initiated logout (Task 5) |
| MFA / onay | Test kullanıcısında yok (`amr: pwd`) | — |

**Hâlâ açık:** kullanıcılar e-postalarını değiştirebiliyor mu, erişim Authentik'te kısıtlanıyor mu, Authentik'te devre dışı bırakılan kullanıcının açık oturumlarına ne olmalı (back-channel logout bir seçenek), sunucudan erişim ve sertifika, kesintiler. Bunlar Aşama 1'in kodunu değiştirmiyor. Cevaba göre sonraki bir aşamaya ya da ayrı bir göreve eklenir.

**Yerel geliştirmede gerçek giriş:** kayıtlı redirect adresi test alan adında olduğu için `localhost`'ta gerçek Authentik girişi çalışmaz. Ya Authentik tarafında bir `localhost` redirect adresi eklenmeli ya da gerçek giriş test alan adına deploy edilerek denenmeli. Testler sahte issuer kullandığı için bundan etkilenmez.

## Review Focus

1. **`next` ile açık redirect:** `//evil.com`, `/\evil.com`, `https://evil.com`, `/%2F%2Fevil.com`, boş değer ve kontrol karakterleri `/`'e düşmeli. Testleri Task 4 (`SafeNext` tablosu) ve Task 5'te (uçtan uca login).
2. **Geçerli cookie'si olan devre dışı kullanıcı:** bir sonraki istekte `/login`'e yönlendirilmeli, oturum cookie'si silinmeli. Testleri Task 5 (sahte kullanıcılarla) ve Task 8'de (admin'in devre dışı bırakması, uçtan uca).
3. **Callback tekrarı ve eksik akış durumu:** aynı callback URL'si ikinci kez çağrılırsa ya da oturumda `state`/`verifier` yoksa 400 dönmeli ve kullanıcı giriş yapmış sayılmamalı. Testi Task 5'te.
4. **Son admin:** tek aktif admin kendi adminliğini kaldıramamalı ve devre dışı bırakılamamalı. Hiç admin yokken normal kullanıcıları devre dışı bırakmak çalışmalı. Testleri Task 3 (store) ve Task 8'de (sayfa).
5. **`ADMIN_EMAILS` biçimi:** büyük/küçük harf ve boşluklar önemsenmemeli (`" Alice@Example.com "`), boş öğeler atlanmalı, adminlik yalnızca ilk girişte verilmeli. Testleri Task 1 (config) ve Task 3'te (store).

---

## Dosya yapısı

```
go.mod, go.sum
main.go                              süreç girişi: config → db → migrate → OIDC → web.New → ListenAndServe
compose.yaml                         geliştirme/test PostgreSQL'i (port 55432)
.env.example                         collage dev için örnek ortam
README.md                            (collage new'in ürettiği) + "Geliştirme" bölümü
templates/layouts/base.html          <html>, head, liveClient, dil değiştirici
templates/layouts/app.html           üst bar, flash'lar, <main>
templates/pages/home.html
templates/pages/teams.html
templates/pages/team.html
templates/pages/admin_users.html
templates/pages/auth_failed.html
templates/pages/not_found.html
templates/pages/error.html
static/app.css
locales/tr.json, locales/en.json

internal/config/config.go            env → Config, tüm hatalar isimle
internal/db/db.go                    Connect, Migrate (tern, gömülü)
internal/db/migrations/001_users_teams.sql
internal/db/dbtest/dbtest.go         test başına geçici veritabanı
internal/store/store.go              Store, hatalar
internal/store/users.go              kullanıcı sorguları, son-admin koruması
internal/store/teams.go              takım ve üyelik sorguları
internal/authz/team.go               takım erişim kuralları (saf)
internal/auth/client.go              OIDC istemcisi (discovery, PKCE, ID token doğrulama)
internal/auth/next.go                SafeNext
internal/auth/service.go             login/callback/logout action'ları
internal/auth/middleware.go          oturum → kullanıcı, context yardımcıları
internal/auth/authtest/issuer.go     sahte OIDC issuer (httptest)
internal/webtest/browser.go          cookie tutan test tarayıcısı, CSRF, SignIn
internal/web/app.go                  collage.Config, eklentiler, kayıtlar
internal/web/layouts.go              base ve app layout'ları, paths yardımcı
internal/web/pages_home.go
internal/web/pages_errors.go
internal/web/pages_teams.go
internal/web/pages_admin.go
```

---

### Task 1: İskelet ve yapılandırma

**Files:**
- Create (scaffold): `go.mod`, `main.go`, `README.md`, `.gitignore`, `templates/…`, `static/app.css`
- Delete (scaffold artığı): `routes.go`, `pages/`, `fragments/`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  ```go
  package config
  var Locales = []string{"tr", "en"}
  type OIDC struct { Issuer, ClientID, ClientSecret, RedirectURL, CallbackPath string; Scopes []string } // CallbackPath: RedirectURL'nin path'i
  type Config struct {
      BaseURL       string   // "https://pano.kurulus.com", sonunda "/" yok
      DatabaseURL   string
      DefaultLocale string   // "tr" | "en"
      OIDC          OIDC
      AdminEmails   []string // küçük harf, kırpılmış, boşsuz
      SessionKey    []byte   // ≥ 32 bayt
      CSRFKey       []byte
      FlashKey      []byte
  }
  func Load(getenv func(string) string) (Config, error)
  ```

- [ ] **Step 0: Mevcut dokümanları commit'le**

İskelet `--force` ile repo köküne üretileceği için önce var olan dosyaları ayrı bir commit'e al:

```bash
cd /Users/furkan/Desktop/planLA
git add -A && git commit -m "docs: spec, phase 1 plan, framework issue 001"
```

- [ ] **Step 1: İskeleti oluştur**

`collage` CLI'ının v0.38.0 olduğunu doğrula, sonra iskeleti repo köküne üret:

```bash
cd /Users/furkan/Desktop/planLA
collage version            # "collage version 0.38.0" olmalı; değilse: go install github.com/Elagoht/collage/cmd/collage@v0.38.0
collage new kanban --dir . --module kanban --force
rm -rf routes.go pages fragments
go get github.com/Elagoht/collage@v0.38.0
git status && git log --oneline   # collage new repoyu yeniden init etmemiş ve kendi commit'ini atmamış olmalı
```

`main.go` Task 9'da baştan yazılacak. Bu adımda derlenmesi için `main.go` içindeki `register(app)` çağrısını ve `if err := register(app)…` bloğunu sil. `go build ./...` başarılı olmalı.

- [ ] **Step 2: Başarısız testi yaz**

`internal/config/config_test.go`:

```go
package config_test

import (
	"strings"
	"testing"

	"kanban/internal/config"
)

const hexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func env(overrides map[string]string) func(string) string {
	base := map[string]string{
		"BASE_URL":           "https://pano.example.com/",
		"DATABASE_URL":       "postgres://u:p@localhost:5432/kanban",
		"DEFAULT_LOCALE":     "tr",
		"OIDC_ISSUER":        "https://auth.example.com/application/o/kanban/",
		"OIDC_CLIENT_ID":     "kanban",
		"OIDC_CLIENT_SECRET": "secret",
		"OIDC_REDIRECT_URL":  "https://pano.example.com/auth/openid/authentik",
		"ADMIN_EMAILS":       " Alice@Example.com ,,bob@example.com",
		"SESSION_KEY":        hexKey,
		"CSRF_KEY":           hexKey,
		"FLASH_KEY":          hexKey,
	}
	for k, v := range overrides {
		base[k] = v
	}
	return func(k string) string { return base[k] }
}

func TestLoadValid(t *testing.T) {
	cfg, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "https://pano.example.com" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", cfg.BaseURL)
	}
	if got := strings.Join(cfg.AdminEmails, ","); got != "alice@example.com,bob@example.com" {
		t.Errorf("AdminEmails = %q", got)
	}
	if got := strings.Join(cfg.OIDC.Scopes, " "); got != "openid email profile" {
		t.Errorf("Scopes = %q, want the default", got)
	}
	if cfg.OIDC.CallbackPath != "/auth/openid/authentik" {
		t.Errorf("CallbackPath = %q", cfg.OIDC.CallbackPath)
	}
	if len(cfg.SessionKey) != 32 {
		t.Errorf("SessionKey is %d bytes, want 32", len(cfg.SessionKey))
	}
}

func TestLoadDefaultsLocaleToTurkish(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"DEFAULT_LOCALE": ""}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultLocale != "tr" {
		t.Errorf("DefaultLocale = %q, want tr", cfg.DefaultLocale)
	}
}

func TestLoadReportsEveryProblemByName(t *testing.T) {
	_, err := config.Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("Load with an empty environment succeeded")
	}
	for _, name := range []string{"BASE_URL", "DATABASE_URL", "OIDC_ISSUER", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL", "SESSION_KEY", "CSRF_KEY", "FLASH_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not name %s:\n%v", name, err)
		}
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	cases := map[string]map[string]string{
		"BASE_URL":       {"BASE_URL": "pano.example.com"},
		"BASE_URL path":  {"BASE_URL": "https://example.com/kanban"},
		"DEFAULT_LOCALE": {"DEFAULT_LOCALE": "fr"},
		"OIDC_SCOPES":    {"OIDC_SCOPES": "email profile"},
		"OIDC_ISSUER":    {"OIDC_ISSUER": "not a url"},
		"SESSION_KEY":    {"SESSION_KEY": "zz"},
		"CSRF_KEY short": {"CSRF_KEY": "00ff"},
		"OIDC_REDIRECT_URL other origin": {"OIDC_REDIRECT_URL": "https://elsewhere.example.com/auth/openid/authentik"},
		"OIDC_REDIRECT_URL root":         {"OIDC_REDIRECT_URL": "https://pano.example.com/"},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(env(overrides))
			if err == nil {
				t.Fatal("Load succeeded")
			}
			want := strings.Fields(name)[0]
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not name %s: %v", want, err)
			}
		})
	}
}
```

- [ ] **Step 3: Testin başarısız olduğunu gör**

Run: `go test ./internal/config/`
Expected: FAIL — `package kanban/internal/config is not in std` / `undefined: config.Load`

- [ ] **Step 4: Uygulamayı yaz**

`internal/config/config.go`:

```go
// Package config reads the application's configuration from the environment.
package config

import (
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"strings"
)

// Locales are the locales the application is translated into.
var Locales = []string{"tr", "en"}

// OIDC is the identity provider the application signs readers in with.
type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string // registered with the provider
	CallbackPath string // RedirectURL's path, where the callback is served
	Scopes       []string
}

// Config is everything the process reads from its environment.
type Config struct {
	BaseURL       string
	DatabaseURL   string
	DefaultLocale string
	OIDC          OIDC
	AdminEmails   []string
	SessionKey    []byte
	CSRFKey       []byte
	FlashKey      []byte
}

// Load reads the configuration through getenv and reports every missing or
// malformed value by name, not only the first.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	fail := func(name, message string) {
		errs = append(errs, errors.New(name+": "+message))
	}
	required := func(name string) string {
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			fail(name, "is required")
		}
		return v
	}
	key := func(name string) []byte {
		v := required(name)
		if v == "" {
			return nil
		}
		b, err := hex.DecodeString(v)
		if err != nil || len(b) < 32 {
			fail(name, "must be at least 32 bytes, hex-encoded (openssl rand -hex 32)")
			return nil
		}
		return b
	}

	var cfg Config

	if base := required("BASE_URL"); base != "" {
		u, err := url.Parse(base)
		switch {
		case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
			fail("BASE_URL", "must be an absolute http or https URL")
		case strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "":
			fail("BASE_URL", "must be an origin with no path, query or fragment")
		default:
			cfg.BaseURL = u.Scheme + "://" + u.Host
		}
	}

	cfg.DatabaseURL = required("DATABASE_URL")

	cfg.DefaultLocale = strings.TrimSpace(getenv("DEFAULT_LOCALE"))
	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "tr"
	}
	if !slices.Contains(Locales, cfg.DefaultLocale) {
		fail("DEFAULT_LOCALE", "must be one of "+strings.Join(Locales, ", "))
	}

	if issuer := required("OIDC_ISSUER"); issuer != "" {
		u, err := url.Parse(issuer)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail("OIDC_ISSUER", "must be an absolute http or https URL")
		}
		cfg.OIDC.Issuer = issuer
	}
	cfg.OIDC.ClientID = required("OIDC_CLIENT_ID")
	cfg.OIDC.ClientSecret = required("OIDC_CLIENT_SECRET")
	if redirect := required("OIDC_REDIRECT_URL"); redirect != "" {
		u, err := url.Parse(redirect)
		switch {
		case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
			fail("OIDC_REDIRECT_URL", "must be an absolute http or https URL")
		case cfg.BaseURL != "" && u.Scheme+"://"+u.Host != cfg.BaseURL:
			fail("OIDC_REDIRECT_URL", "must be on BASE_URL ("+cfg.BaseURL+")")
		case strings.Trim(u.Path, "/") == "" || u.RawQuery != "" || u.Fragment != "":
			fail("OIDC_REDIRECT_URL", "must name a path, with no query or fragment")
		default:
			cfg.OIDC.RedirectURL = redirect
			cfg.OIDC.CallbackPath = u.Path
		}
	}
	cfg.OIDC.Scopes = strings.Fields(getenv("OIDC_SCOPES"))
	if len(cfg.OIDC.Scopes) == 0 {
		cfg.OIDC.Scopes = []string{"openid", "email", "profile"}
	}
	if !slices.Contains(cfg.OIDC.Scopes, "openid") {
		fail("OIDC_SCOPES", "must include openid")
	}

	for _, e := range strings.Split(getenv("ADMIN_EMAILS"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			cfg.AdminEmails = append(cfg.AdminEmails, e)
		}
	}

	cfg.SessionKey = key("SESSION_KEY")
	cfg.CSRFKey = key("CSRF_KEY")
	cfg.FlashKey = key("FLASH_KEY")

	return cfg, errors.Join(errs...)
}
```


- [ ] **Step 5: Testin geçtiğini gör**

Run: `go test ./internal/config/ -v`
Expected: PASS (4 test)

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: scaffold collage app and read configuration from env"
```

---

### Task 2: Veritabanı, migration'lar, test veritabanı

**Files:**
- Create: `compose.yaml`
- Create: `internal/db/db.go`, `internal/db/migrations/001_users_teams.sql`
- Create: `internal/db/dbtest/dbtest.go`
- Test: `internal/db/db_test.go`

**Interfaces:**
- Produces:
  ```go
  package db
  func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error)
  func Migrate(ctx context.Context, pool *pgxpool.Pool) error

  package dbtest
  func New(t testing.TB) *pgxpool.Pool   // boş, migrate edilmiş, test sonunda silinen veritabanı
  ```

- [ ] **Step 1: PostgreSQL'i başlat**

`compose.yaml`:

```yaml
services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_USER: kanban
      POSTGRES_PASSWORD: kanban
      POSTGRES_DB: kanban
    ports:
      - "55432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
```

```bash
docker compose up -d
export KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable'
go get github.com/jackc/pgx/v5@v5.11.0 github.com/jackc/tern/v2@v2.4.3
```

- [ ] **Step 2: Başarısız testi yaz**

`internal/db/db_test.go`:

```go
package db_test

import (
	"context"
	"testing"

	"kanban/internal/db"
	"kanban/internal/db/dbtest"
)

func TestMigrateCreatesTablesAndIsIdempotent(t *testing.T) {
	pool := dbtest.New(t) // already migrated once
	ctx := context.Background()

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	for _, table := range []string{"users", "teams", "team_members"} {
		var exists bool
		err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("table %s missing (err=%v)", table, err)
		}
	}
}

func TestUsersAreUniqueByIssuerAndSubject(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	insert := `INSERT INTO users (issuer, subject, email, name, locale) VALUES ('https://idp', 'sub-1', 'a@x', 'A', 'tr')`
	if _, err := pool.Exec(ctx, insert); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := pool.Exec(ctx, insert); err == nil {
		t.Fatal("second insert with the same (issuer, subject) succeeded")
	}
}
```

- [ ] **Step 3: Testin başarısız olduğunu gör**

Run: `go test ./internal/db/...`
Expected: FAIL — `undefined: db.Migrate`, `package kanban/internal/db/dbtest is not in std`

- [ ] **Step 4: Migration'ı yaz**

`internal/db/migrations/001_users_teams.sql`:

```sql
CREATE TABLE users (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    issuer      text NOT NULL,
    subject     text NOT NULL,
    email       text NOT NULL,
    name        text NOT NULL,
    locale      text NOT NULL CHECK (locale IN ('tr', 'en')),
    is_admin    boolean NOT NULL DEFAULT false,
    disabled_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);

CREATE INDEX users_email_idx ON users (lower(email));

CREATE TABLE teams (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text NOT NULL CHECK (btrim(name) <> ''),
    archived_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE team_members (
    team_id    bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('lead', 'member')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX team_members_user_idx ON team_members (user_id);
```

- [ ] **Step 5: `db` paketini yaz**

`internal/db/db.go`:

```go
// Package db connects to PostgreSQL and brings its schema up to date.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Connect opens a pool and checks that the database answers.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// Migrate applies every migration the database has not seen yet.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("db: acquire: %w", err)
	}
	defer conn.Release()

	m, err := migrate.NewMigrator(ctx, conn.Conn(), "schema_version")
	if err != nil {
		return fmt.Errorf("db: migrator: %w", err)
	}
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	if err := m.LoadMigrations(files); err != nil {
		return fmt.Errorf("db: load migrations: %w", err)
	}
	if err := m.Migrate(ctx); err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: `dbtest` paketini yaz**

`internal/db/dbtest/dbtest.go`:

```go
// Package dbtest gives each test a database of its own.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kanban/internal/db"
)

// New creates an empty, migrated database for t and drops it when t ends. It
// skips t when KANBAN_TEST_DATABASE_URL is not set.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("KANBAN_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("KANBAN_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("dbtest: connect: %v", err)
	}
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	raw := "kanban_test_" + hex.EncodeToString(suffix)
	name := pgx.Identifier{raw}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("dbtest: create database: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = raw
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("dbtest: pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("dbtest: drop database: %v", err)
		}
		admin.Close(ctx)
	})

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("dbtest: migrate: %v", err)
	}
	return pool
}
```

- [ ] **Step 7: Testlerin geçtiğini gör**

Run: `go test ./internal/db/... -v`
Expected: PASS, `SKIP` yok

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: postgres connection, embedded migrations and per-test databases"
```

---

### Task 3: Store — kullanıcılar ve takımlar

**Files:**
- Create: `internal/store/store.go`, `internal/store/users.go`, `internal/store/teams.go`
- Test: `internal/store/users_test.go`, `internal/store/teams_test.go`

**Interfaces:**
- Consumes: `dbtest.New(t)` (Task 2)
- Produces:
  ```go
  package store
  var ErrNotFound, ErrLastAdmin, ErrAmbiguousEmail error
  type Store struct{ /* pool */ }
  func New(pool *pgxpool.Pool) *Store

  type User struct { ID int64; Issuer, Subject, Email, Name, Locale string; IsAdmin bool; DisabledAt *time.Time; CreatedAt time.Time }
  func (u User) Disabled() bool
  type Identity struct { Issuer, Subject, Email, Name string }

  func (s *Store) UpsertIdentity(ctx context.Context, id Identity, adminEmails []string, locale string) (User, error)
  func (s *Store) UserByID(ctx context.Context, id int64) (User, error)
  func (s *Store) UserByEmail(ctx context.Context, email string) (User, error)   // yalnız aktifler
  func (s *Store) Users(ctx context.Context) ([]User, error)
  func (s *Store) SetAdmin(ctx context.Context, userID int64, admin bool) error
  func (s *Store) SetDisabled(ctx context.Context, userID int64, disabled bool) error

  type Role string                       // RoleLead = "lead", RoleMember = "member"
  var Roles = []Role{RoleLead, RoleMember}
  func ParseRole(s string) (Role, bool)
  type Team struct { ID int64; Name string; ArchivedAt *time.Time; CreatedAt time.Time }
  type TeamSummary struct { Team Team; MemberCount int; Role Role } // Role: izleyicinin rolü, üye değilse ""
  type Member struct { User User; Role Role }

  func (s *Store) CreateTeam(ctx context.Context, name string) (Team, error)
  func (s *Store) Team(ctx context.Context, id int64) (Team, error)
  func (s *Store) ListTeams(ctx context.Context, viewer User) ([]TeamSummary, error) // admin: hepsi; diğerleri: üye oldukları
  func (s *Store) TeamsOf(ctx context.Context, userID int64) ([]TeamSummary, error)  // yalnız üye olunanlar
  func (s *Store) Members(ctx context.Context, teamID int64) ([]Member, error)
  func (s *Store) MemberRole(ctx context.Context, teamID, userID int64) (Role, error)
  func (s *Store) AddMember(ctx context.Context, teamID, userID int64, role Role) error // varsa rolünü günceller
  func (s *Store) SetRole(ctx context.Context, teamID, userID int64, role Role) error
  func (s *Store) RemoveMember(ctx context.Context, teamID, userID int64) error
  ```

- [ ] **Step 1: Kullanıcı testlerini yaz**

`internal/store/users_test.go`:

```go
package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/db/dbtest"
	"kanban/internal/store"
)

func newStore(t *testing.T) *store.Store { return store.New(dbtest.New(t)) }

func identity(sub, email string) store.Identity {
	return store.Identity{Issuer: "https://idp.test", Subject: sub, Email: email, Name: "User " + sub}
}

func mustUpsert(t *testing.T, s *store.Store, id store.Identity, admins ...string) store.User {
	t.Helper()
	u, err := s.UpsertIdentity(context.Background(), id, admins, "tr")
	if err != nil {
		t.Fatalf("UpsertIdentity(%s): %v", id.Subject, err)
	}
	return u
}

func TestUpsertIdentityCreatesThenRefreshes(t *testing.T) {
	s := newStore(t)
	first := mustUpsert(t, s, identity("s1", "old@example.com"))
	if first.Locale != "tr" || first.IsAdmin || first.Disabled() {
		t.Fatalf("new user = %+v", first)
	}
	second := mustUpsert(t, s, store.Identity{Issuer: "https://idp.test", Subject: "s1", Email: "new@example.com", Name: "New Name"})
	if second.ID != first.ID {
		t.Fatalf("same (issuer, subject) produced a new user: %d vs %d", second.ID, first.ID)
	}
	if second.Email != "new@example.com" || second.Name != "New Name" {
		t.Errorf("email/name not refreshed: %+v", second)
	}
}

func TestUpsertIdentityEmptyNameFallsBackToEmail(t *testing.T) {
	s := newStore(t)
	u := mustUpsert(t, s, store.Identity{Issuer: "https://idp.test", Subject: "s1", Email: "a@example.com"})
	if u.Name != "a@example.com" {
		t.Errorf("Name = %q, want the email", u.Name)
	}
}

func TestEmailIsNotIdentity(t *testing.T) {
	s := newStore(t)
	a := mustUpsert(t, s, identity("s1", "same@example.com"))
	b := mustUpsert(t, s, identity("s2", "same@example.com"))
	if a.ID == b.ID {
		t.Fatal("two subjects with one email became one user")
	}
}

func TestAdminEmailsGrantAdminOnlyOnFirstSignIn(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	admins := []string{" alice@example.com "}

	alice := mustUpsert(t, s, identity("alice", "Alice@Example.COM"), admins...)
	if !alice.IsAdmin {
		t.Fatal("ADMIN_EMAILS match (case/space-insensitive) did not grant admin")
	}
	bob := mustUpsert(t, s, identity("bob", "bob@example.com"), admins...)
	if bob.IsAdmin {
		t.Fatal("a non-listed email became admin")
	}

	// Revoked in the app, alice must not get it back by signing in again.
	if err := s.SetAdmin(ctx, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdmin(ctx, alice.ID, false); err != nil {
		t.Fatal(err)
	}
	again := mustUpsert(t, s, identity("alice", "alice@example.com"), admins...)
	if again.IsAdmin {
		t.Fatal("admin was granted again on a later sign-in")
	}
}

func TestUserByEmail(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := mustUpsert(t, s, identity("s1", "Ada@Example.com"))

	got, err := s.UserByEmail(ctx, " ada@example.com ")
	if err != nil || got.ID != a.ID {
		t.Fatalf("UserByEmail = %+v, %v", got, err)
	}
	if _, err := s.UserByEmail(ctx, "nobody@example.com"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown email: err = %v, want ErrNotFound", err)
	}

	mustUpsert(t, s, identity("s2", "ada@example.com"))
	if _, err := s.UserByEmail(ctx, "ada@example.com"); !errors.Is(err, store.ErrAmbiguousEmail) {
		t.Errorf("two users: err = %v, want ErrAmbiguousEmail", err)
	}

	if err := s.SetDisabled(ctx, a.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserByEmail(ctx, "ada@example.com"); err != nil {
		t.Errorf("a disabled user must not count: err = %v", err)
	}
}

func TestTheLastActiveAdminStays(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	alice := mustUpsert(t, s, identity("alice", "alice@example.com"), "alice@example.com")
	bob := mustUpsert(t, s, identity("bob", "bob@example.com"))

	if err := s.SetAdmin(ctx, alice.ID, false); !errors.Is(err, store.ErrLastAdmin) {
		t.Errorf("demoting the only admin: err = %v, want ErrLastAdmin", err)
	}
	if err := s.SetDisabled(ctx, alice.ID, true); !errors.Is(err, store.ErrLastAdmin) {
		t.Errorf("disabling the only admin: err = %v, want ErrLastAdmin", err)
	}
	if err := s.SetDisabled(ctx, bob.ID, true); err != nil {
		t.Errorf("disabling a regular user: %v", err)
	}

	if err := s.SetAdmin(ctx, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDisabled(ctx, bob.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdmin(ctx, alice.ID, false); err != nil {
		t.Errorf("demoting one of two admins: %v", err)
	}
	got, _ := s.UserByID(ctx, alice.ID)
	if got.IsAdmin {
		t.Error("alice is still admin")
	}
}

func TestNoAdminsAtAllDoesNotBlockOtherChanges(t *testing.T) {
	s := newStore(t)
	u := mustUpsert(t, s, identity("s1", "a@example.com"))
	if err := s.SetDisabled(context.Background(), u.ID, true); err != nil {
		t.Fatalf("SetDisabled with no admins: %v", err)
	}
}

func TestUnknownUser(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.UserByID(ctx, 999); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("UserByID: %v", err)
	}
	if err := s.SetDisabled(ctx, 999, true); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetDisabled: %v", err)
	}
	if err := s.SetAdmin(ctx, 999, true); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetAdmin: %v", err)
	}
}
```

- [ ] **Step 2: Takım testlerini yaz**

`internal/store/teams_test.go`:

```go
package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/store"
)

func TestTeamsAndMembers(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	admin := mustUpsert(t, s, identity("admin", "admin@example.com"), "admin@example.com")
	lead := mustUpsert(t, s, identity("lead", "lead@example.com"))
	member := mustUpsert(t, s, identity("member", "member@example.com"))
	outsider := mustUpsert(t, s, identity("out", "out@example.com"))

	team, err := s.CreateTeam(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateTeam(ctx, "Another")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, lead.ID, store.RoleLead); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, member.ID, store.RoleMember); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListTeams(ctx, admin)
	if err != nil || len(all) != 2 {
		t.Fatalf("admin ListTeams = %d teams, %v; want 2", len(all), err)
	}
	mine, err := s.ListTeams(ctx, member)
	if err != nil || len(mine) != 1 || mine[0].Team.ID != team.ID || mine[0].Role != store.RoleMember || mine[0].MemberCount != 2 {
		t.Fatalf("member ListTeams = %+v, %v", mine, err)
	}
	if none, _ := s.ListTeams(ctx, outsider); len(none) != 0 {
		t.Fatalf("outsider sees %d teams", len(none))
	}
	if adminOwn, _ := s.TeamsOf(ctx, admin.ID); len(adminOwn) != 0 {
		t.Fatalf("TeamsOf(admin) = %d, want only teams admin belongs to (0)", len(adminOwn))
	}
	_ = other

	members, err := s.Members(ctx, team.ID)
	if err != nil || len(members) != 2 || members[0].User.ID != lead.ID {
		t.Fatalf("Members = %+v, %v; want lead first", members, err)
	}

	if role, err := s.MemberRole(ctx, team.ID, lead.ID); err != nil || role != store.RoleLead {
		t.Errorf("MemberRole(lead) = %q, %v", role, err)
	}
	if _, err := s.MemberRole(ctx, team.ID, outsider.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("MemberRole(outsider): %v", err)
	}

	if err := s.SetRole(ctx, team.ID, member.ID, store.RoleLead); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, member.ID, store.RoleMember); err != nil {
		t.Fatalf("re-adding an existing member: %v", err)
	}
	if role, _ := s.MemberRole(ctx, team.ID, member.ID); role != store.RoleMember {
		t.Errorf("AddMember on an existing member did not update the role: %q", role)
	}
	if err := s.SetRole(ctx, team.ID, outsider.ID, store.RoleLead); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetRole(outsider): %v", err)
	}

	if err := s.RemoveMember(ctx, team.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveMember(ctx, team.ID, member.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("removing twice: %v", err)
	}
}

func TestTeamLookups(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.Team(ctx, 12345); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Team(unknown): %v", err)
	}
	if _, err := s.CreateTeam(ctx, "   "); err == nil {
		t.Error("a blank team name was stored")
	}
	if _, ok := store.ParseRole("owner"); ok {
		t.Error("ParseRole accepted an unknown role")
	}
	if r, ok := store.ParseRole("lead"); !ok || r != store.RoleLead {
		t.Error("ParseRole(lead)")
	}
}
```

- [ ] **Step 3: Testlerin başarısız olduğunu gör**

Run: `go test ./internal/store/`
Expected: FAIL — `undefined: store.New` vb.

- [ ] **Step 4: `store.go`'yu yaz**

```go
// Package store reads and writes the application's data in PostgreSQL.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound reports that the row asked for does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrLastAdmin refuses a change that would leave no active admin.
	ErrLastAdmin = errors.New("store: the last active admin cannot be removed")
	// ErrAmbiguousEmail reports that more than one active user has an email.
	ErrAmbiguousEmail = errors.New("store: more than one active user has this email")
)

// Store is the application's data access.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store over pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }
```

- [ ] **Step 5: `users.go`'yu yaz**

```go
package store

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// User is a person who has signed in at least once.
type User struct {
	ID         int64
	Issuer     string
	Subject    string
	Email      string
	Name       string
	Locale     string
	IsAdmin    bool
	DisabledAt *time.Time
	CreatedAt  time.Time
}

// Disabled reports whether the user may no longer sign in.
func (u User) Disabled() bool { return u.DisabledAt != nil }

// Identity is who an identity provider says signed in.
type Identity struct {
	Issuer  string
	Subject string
	Email   string
	Name    string
}

const userColumns = `id, issuer, subject, email, name, locale, is_admin, disabled_at, created_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.Name, &u.Locale, &u.IsAdmin, &u.DisabledAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func collectUsers(rows pgx.Rows, err error) ([]User, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpsertIdentity finds the user an identity provider signed in by (issuer,
// subject) and refreshes their email and name, or creates them. Only a user
// created here can be made admin by adminEmails; after that, adminship is the
// application's to manage.
func (s *Store) UpsertIdentity(ctx context.Context, id Identity, adminEmails []string, locale string) (User, error) {
	name := strings.TrimSpace(id.Name)
	if name == "" {
		name = id.Email
	}
	return scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO users (issuer, subject, email, name, locale, is_admin)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (issuer, subject) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name
		RETURNING `+userColumns,
		id.Issuer, id.Subject, id.Email, name, locale, isAdminEmail(id.Email, adminEmails)))
}

func isAdminEmail(email string, adminEmails []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	return email != "" && slices.ContainsFunc(adminEmails, func(a string) bool {
		return strings.ToLower(strings.TrimSpace(a)) == email
	})
}

// UserByID returns one user.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// UserByEmail returns the one active user with email, ignoring case.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	users, err := collectUsers(s.pool.Query(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE lower(email) = lower($1) AND disabled_at IS NULL
		LIMIT 2`, strings.TrimSpace(email)))
	switch {
	case err != nil:
		return User{}, err
	case len(users) == 0:
		return User{}, ErrNotFound
	case len(users) > 1:
		return User{}, ErrAmbiguousEmail
	}
	return users[0], nil
}

// Users returns every user, by name.
func (s *Store) Users(ctx context.Context) ([]User, error) {
	return collectUsers(s.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY lower(name), id`))
}

// SetAdmin grants or revokes adminship.
func (s *Store) SetAdmin(ctx context.Context, userID int64, admin bool) error {
	return s.changeAdmins(ctx, `UPDATE users SET is_admin = $2 WHERE id = $1`, userID, admin)
}

// SetDisabled disables or re-enables a user.
func (s *Store) SetDisabled(ctx context.Context, userID int64, disabled bool) error {
	return s.changeAdmins(ctx, `
		UPDATE users SET disabled_at = CASE WHEN $2::boolean THEN coalesce(disabled_at, now()) ELSE NULL END
		WHERE id = $1`, userID, disabled)
}

// changeAdmins runs update with the active admins locked, and refuses it when
// there were active admins before and none would remain.
func (s *Store) changeAdmins(ctx context.Context, update string, userID int64, value bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	countAdmins := func() (int, error) {
		var n int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL`).Scan(&n)
		return n, err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE is_admin AND disabled_at IS NULL FOR UPDATE`); err != nil {
		return err
	}
	before, err := countAdmins()
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, update, userID, value)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	after, err := countAdmins()
	if err != nil {
		return err
	}
	if before > 0 && after == 0 {
		return ErrLastAdmin
	}
	return tx.Commit(ctx)
}
```

- [ ] **Step 6: `teams.go`'yu yaz**

```go
package store

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

// Role is what a member may do in a team.
type Role string

const (
	RoleLead   Role = "lead"
	RoleMember Role = "member"
)

// Roles lists every role, lead first.
var Roles = []Role{RoleLead, RoleMember}

// ParseRole reads a role from a form value.
func ParseRole(s string) (Role, bool) {
	r := Role(s)
	return r, slices.Contains(Roles, r)
}

// Team is a group of people who share boards.
type Team struct {
	ID         int64
	Name       string
	ArchivedAt *time.Time
	CreatedAt  time.Time
}

// TeamSummary is a team as a list shows it. Role is the viewer's, "" when the
// viewer is not a member.
type TeamSummary struct {
	Team        Team
	MemberCount int
	Role        Role
}

// Member is a user's membership in a team.
type Member struct {
	User User
	Role Role
}

func scanTeam(row pgx.Row) (Team, error) {
	var t Team
	err := row.Scan(&t.ID, &t.Name, &t.ArchivedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Team{}, ErrNotFound
	}
	return t, err
}

// CreateTeam adds a team with no members.
func (s *Store) CreateTeam(ctx context.Context, name string) (Team, error) {
	return scanTeam(s.pool.QueryRow(ctx,
		`INSERT INTO teams (name) VALUES ($1) RETURNING id, name, archived_at, created_at`, name))
}

// Team returns one team.
func (s *Store) Team(ctx context.Context, id int64) (Team, error) {
	return scanTeam(s.pool.QueryRow(ctx,
		`SELECT id, name, archived_at, created_at FROM teams WHERE id = $1`, id))
}

// ListTeams returns the teams viewer may see: every team for an admin, the
// teams they belong to for anyone else.
func (s *Store) ListTeams(ctx context.Context, viewer User) ([]TeamSummary, error) {
	return s.listTeams(ctx, viewer.ID, viewer.IsAdmin)
}

// TeamsOf returns the teams userID belongs to.
func (s *Store) TeamsOf(ctx context.Context, userID int64) ([]TeamSummary, error) {
	return s.listTeams(ctx, userID, false)
}

func (s *Store) listTeams(ctx context.Context, userID int64, all bool) ([]TeamSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name, t.archived_at, t.created_at,
		       (SELECT count(*) FROM team_members c WHERE c.team_id = t.id),
		       coalesce(m.role, '')
		FROM teams t
		LEFT JOIN team_members m ON m.team_id = t.id AND m.user_id = $1
		WHERE t.archived_at IS NULL AND ($2 OR m.user_id IS NOT NULL)
		ORDER BY lower(t.name), t.id`, userID, all)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []TeamSummary
	for rows.Next() {
		var ts TeamSummary
		var role string
		if err := rows.Scan(&ts.Team.ID, &ts.Team.Name, &ts.Team.ArchivedAt, &ts.Team.CreatedAt, &ts.MemberCount, &role); err != nil {
			return nil, err
		}
		ts.Role = Role(role)
		teams = append(teams, ts)
	}
	return teams, rows.Err()
}

// Members returns a team's members, leads first, then by name.
func (s *Store) Members(ctx context.Context, teamID int64) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.issuer, u.subject, u.email, u.name, u.locale, u.is_admin, u.disabled_at, u.created_at, m.role
		FROM team_members m JOIN users u ON u.id = m.user_id
		WHERE m.team_id = $1
		ORDER BY m.role = 'lead' DESC, lower(u.name), u.id`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []Member
	for rows.Next() {
		var m Member
		var role string
		u := &m.User
		if err := rows.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.Name, &u.Locale, &u.IsAdmin, &u.DisabledAt, &u.CreatedAt, &role); err != nil {
			return nil, err
		}
		m.Role = Role(role)
		members = append(members, m)
	}
	return members, rows.Err()
}

// MemberRole returns userID's role in teamID, or ErrNotFound when they are not
// a member.
func (s *Store) MemberRole(ctx context.Context, teamID, userID int64) (Role, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return Role(role), err
}

// AddMember adds userID to teamID, or changes their role if they are a member.
func (s *Store) AddMember(ctx context.Context, teamID, userID int64, role Role) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`, teamID, userID, string(role))
	return err
}

// SetRole changes a member's role.
func (s *Store) SetRole(ctx context.Context, teamID, userID int64, role Role) error {
	return exactlyOne(s.pool.Exec(ctx,
		`UPDATE team_members SET role = $3 WHERE team_id = $1 AND user_id = $2`, teamID, userID, string(role)))
}

// RemoveMember takes userID out of teamID.
func (s *Store) RemoveMember(ctx context.Context, teamID, userID int64) error {
	return exactlyOne(s.pool.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID))
}
```

`exactlyOne`'ı `store.go`'ya ekle:

```go
import "github.com/jackc/pgx/v5/pgconn"

// exactlyOne turns a statement that touched no row into ErrNotFound.
func exactlyOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 7: Testlerin geçtiğini gör**

Run: `go test ./internal/store/ -v -race`
Expected: PASS, `SKIP` yok

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: store for users and teams with last-admin protection"
```

---

### Task 4: Sahte OIDC issuer, OIDC istemcisi, `SafeNext`

**Files:**
- Create: `internal/auth/authtest/issuer.go`
- Create: `internal/auth/client.go`, `internal/auth/next.go`
- Test: `internal/auth/client_test.go`, `internal/auth/next_test.go`

**Interfaces:**
- Consumes: `store.Identity` (Task 3)
- Produces:
  ```go
  package authtest
  type Options struct { RedirectURL string; EndSession bool }
  type Grant struct { Subject, Email, Name, PreferredUsername string; Nonce, Audience string } // Nonce/Audience: kurcalama (test)
  type Issuer struct { URL, ClientID, ClientSecret string /* … */ }
  func NewIssuer(t testing.TB, opts Options) *Issuer
  func (i *Issuer) Authorize(t testing.TB, authURL string, g Grant) string // redirect_uri'nin RequestURI'si: "/auth/openid/authentik?code=…&state=…"
  func (i *Issuer) EndSessionURL() string

  package auth
  type ClientConfig struct { Issuer, ClientID, ClientSecret, RedirectURL string; Scopes []string }
  type Client struct{ /* … */ }
  func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error)
  func (c *Client) AuthURL(state, nonce, verifier string) string
  func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (store.Identity, error)
  func (c *Client) LogoutURL(postLogoutRedirect string) string // IdP end_session sunmuyorsa ""
  var ErrNonce error
  func SafeNext(next string) string
  ```

- [ ] **Step 1: Bağımlılıkları ekle**

```bash
go get github.com/coreos/go-oidc/v3@v3.21.0 golang.org/x/oauth2@v0.37.0 github.com/go-jose/go-jose/v4@v4.1.5
```

- [ ] **Step 2: Sahte issuer'ı yaz**

Bu bir test yardımcısı ama sonraki testlerin hepsi ona dayanıyor; önce o yazılır.

`internal/auth/authtest/issuer.go`:

```go
// Package authtest is an OpenID Connect provider for tests: discovery, JWKS,
// authorization codes with PKCE, and signed ID tokens.
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Options shapes the provider.
type Options struct {
	// RedirectURL is the only redirect_uri the provider accepts.
	RedirectURL string
	// EndSession advertises an end_session_endpoint in discovery.
	EndSession bool
}

// Grant is who signs in at one authorization, and how the ID token is to be
// tampered with, for tests of what a client must refuse.
type Grant struct {
	Subject           string
	Email             string
	Name              string
	PreferredUsername string

	Nonce    string // replaces the nonce the client asked for
	Audience string // replaces the client id in aud
}

// Issuer is a running provider.
type Issuer struct {
	URL          string
	ClientID     string
	ClientSecret string

	opts   Options
	key    *rsa.PrivateKey
	server *httptest.Server

	mu    sync.Mutex
	codes map[string]pending
}

type pending struct {
	grant     Grant
	nonce     string
	challenge string
}

type discovery struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	EndSessionEndpoint    string   `json:"end_session_endpoint,omitempty"`
	ResponseTypes         []string `json:"response_types_supported"`
	SubjectTypes          []string `json:"subject_types_supported"`
	SigningAlgs           []string `json:"id_token_signing_alg_values_supported"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
}

type idClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Nonce    string `json:"nonce"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Username string `json:"preferred_username,omitempty"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

type tokenError struct {
	Error string `json:"error"`
}

// NewIssuer starts a provider that stops when t ends.
func NewIssuer(t testing.TB, opts Options) *Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	i := &Issuer{
		ClientID:     "kanban-test",
		ClientSecret: "kanban-secret",
		opts:         opts,
		key:          key,
		codes:        map[string]pending{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", i.discovery)
	mux.HandleFunc("GET /jwks", i.jwks)
	mux.HandleFunc("POST /token", i.token)
	i.server = httptest.NewServer(mux)
	i.URL = i.server.URL
	t.Cleanup(i.server.Close)
	return i
}

// EndSessionURL is the provider's end_session_endpoint.
func (i *Issuer) EndSessionURL() string { return i.URL + "/logout" }

// Authorize plays the reader's visit to the authorization endpoint: it checks
// the request the client built, and returns the path and query the browser is
// sent back to on the client.
func (i *Issuer) Authorize(t testing.TB, authURL string, g Grant) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("authtest: authorization URL: %v", err)
	}
	q := u.Query()
	want := map[string]string{
		"response_type":         "code",
		"client_id":             i.ClientID,
		"redirect_uri":          i.opts.RedirectURL,
		"code_challenge_method": "S256",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Fatalf("authtest: %s = %q, want %q", k, q.Get(k), v)
		}
	}
	for _, k := range []string{"state", "nonce", "code_challenge", "scope"} {
		if q.Get(k) == "" {
			t.Fatalf("authtest: authorization request has no %s", k)
		}
	}
	code := random(t)
	i.mu.Lock()
	i.codes[code] = pending{grant: g, nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
	i.mu.Unlock()

	back, err := url.Parse(i.opts.RedirectURL)
	if err != nil {
		t.Fatal(err)
	}
	bq := url.Values{"code": {code}, "state": {q.Get("state")}}
	back.RawQuery = bq.Encode()
	return back.RequestURI()
}

func (i *Issuer) discovery(w http.ResponseWriter, _ *http.Request) {
	d := discovery{
		Issuer:                i.URL,
		AuthorizationEndpoint: i.URL + "/authorize",
		TokenEndpoint:         i.URL + "/token",
		JWKSURI:               i.URL + "/jwks",
		ResponseTypes:         []string{"code"},
		SubjectTypes:          []string{"public"},
		SigningAlgs:           []string{"RS256"},
		CodeChallengeMethods:  []string{"S256"},
	}
	if i.opts.EndSession {
		d.EndSessionEndpoint = i.EndSessionURL()
	}
	writeJSON(w, http.StatusOK, d)
}

func (i *Issuer) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &i.key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig",
	}}})
}

func (i *Issuer) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, tokenError{"invalid_request"})
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != i.ClientID || secret != i.ClientSecret {
		writeJSON(w, http.StatusUnauthorized, tokenError{"invalid_client"})
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("redirect_uri") != i.opts.RedirectURL {
		writeJSON(w, http.StatusBadRequest, tokenError{"invalid_grant"})
		return
	}

	i.mu.Lock()
	p, found := i.codes[r.PostForm.Get("code")]
	delete(i.codes, r.PostForm.Get("code")) // a code is good once
	i.mu.Unlock()

	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !found || base64.RawURLEncoding.EncodeToString(sum[:]) != p.challenge {
		writeJSON(w, http.StatusBadRequest, tokenError{"invalid_grant"})
		return
	}

	claims := idClaims{
		Issuer:   i.URL,
		Subject:  p.grant.Subject,
		Audience: i.ClientID,
		Expiry:   time.Now().Add(5 * time.Minute).Unix(),
		IssuedAt: time.Now().Unix(),
		Nonce:    p.nonce,
		Email:    p.grant.Email,
		Name:     p.grant.Name,
		Username: p.grant.PreferredUsername,
	}
	if p.grant.Nonce != "" {
		claims.Nonce = p.grant.Nonce
	}
	if p.grant.Audience != "" {
		claims.Audience = p.grant.Audience
	}
	idToken, err := i.sign(claims)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, tokenError{"server_error"})
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{AccessToken: "access", TokenType: "Bearer", ExpiresIn: 300, IDToken: idToken})
}

func (i *Issuer) sign(claims idClaims) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: i.key, KeyID: "test", Algorithm: "RS256"}},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return obj.CompactSerialize()
}

func writeJSON[T discovery | jose.JSONWebKeySet | tokenResponse | tokenError](w http.ResponseWriter, status int, v T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func random(t testing.TB) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
```

- [ ] **Step 3: İstemci ve `SafeNext` testlerini yaz**

`internal/auth/next_test.go`:

```go
package auth_test

import (
	"testing"

	"kanban/internal/auth"
)

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"":                       "/",
		"/":                      "/",
		"/teams/3":               "/teams/3",
		"/en/teams?tab=members":  "/en/teams?tab=members",
		"//evil.com":             "/",
		"/\\evil.com":            "/",
		"https://evil.com/x":     "/",
		"evil.com":               "/",
		"/%2F%2Fevil.com":        "/%2F%2Fevil.com", // a path on this site, not a host
		"/ok\r\nSet-Cookie: x=1": "/",
		"javascript:alert(1)":    "/",
	}
	for in, want := range cases {
		if got := auth.SafeNext(in); got != want {
			t.Errorf("SafeNext(%q) = %q, want %q", in, got, want)
		}
	}
	long := "/" + string(make([]byte, 600))
	if got := auth.SafeNext(long); got != "/" {
		t.Errorf("an overlong next was kept")
	}
}
```

`internal/auth/client_test.go`:

```go
package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"kanban/internal/auth"
	"kanban/internal/auth/authtest"
)

const redirectURL = "http://kanban.test/auth/openid/authentik"

func newClient(t *testing.T, opts authtest.Options) (*auth.Client, *authtest.Issuer) {
	t.Helper()
	opts.RedirectURL = redirectURL
	issuer := authtest.NewIssuer(t, opts)
	client, err := auth.NewClient(context.Background(), auth.ClientConfig{
		Issuer: issuer.URL, ClientID: issuer.ClientID, ClientSecret: issuer.ClientSecret,
		RedirectURL: redirectURL, Scopes: []string{"openid", "email", "profile"},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, issuer
}

// codeFor runs the authorization step and returns the code the provider issued.
func codeFor(t *testing.T, issuer *authtest.Issuer, authURL string, g authtest.Grant) string {
	t.Helper()
	back, err := url.Parse(issuer.Authorize(t, authURL, g))
	if err != nil {
		t.Fatal(err)
	}
	return back.Query().Get("code")
}

func TestExchangeReturnsTheIdentity(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	authURL := client.AuthURL("state-1", "nonce-1", "verifier-verifier-verifier-verifier-verifier")
	code := codeFor(t, issuer, authURL, authtest.Grant{Subject: "sub-1", Email: "ada@example.com", Name: "Ada"})

	id, err := client.Exchange(context.Background(), code, "verifier-verifier-verifier-verifier-verifier", "nonce-1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if id.Issuer != issuer.URL || id.Subject != "sub-1" || id.Email != "ada@example.com" || id.Name != "Ada" {
		t.Errorf("identity = %+v", id)
	}
}

func TestExchangeFallsBackToThePreferredUsername(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	v := "verifier-verifier-verifier-verifier-verifier"
	// What Authentik's test provider sends: no name, a username.
	code := codeFor(t, issuer, client.AuthURL("s", "n", v), authtest.Grant{Subject: "s", Email: "e@example.com", PreferredUsername: "elagoht"})
	id, err := client.Exchange(context.Background(), code, v, "n")
	if err != nil {
		t.Fatal(err)
	}
	if id.Name != "elagoht" {
		t.Errorf("Name = %q, want the preferred username", id.Name)
	}
}

func TestExchangeRefusesTamperedTokens(t *testing.T) {
	cases := map[string]authtest.Grant{
		"nonce":    {Subject: "s", Email: "e@example.com", Nonce: "someone-elses"},
		"audience": {Subject: "s", Email: "e@example.com", Audience: "another-client"},
	}
	for name, g := range cases {
		t.Run(name, func(t *testing.T) {
			client, issuer := newClient(t, authtest.Options{})
			v := "verifier-verifier-verifier-verifier-verifier"
			code := codeFor(t, issuer, client.AuthURL("s", "n", v), g)
			if _, err := client.Exchange(context.Background(), code, v, "n"); err == nil {
				t.Fatal("Exchange accepted a tampered ID token")
			} else if name == "nonce" && !errors.Is(err, auth.ErrNonce) {
				t.Errorf("err = %v, want ErrNonce", err)
			}
		})
	}
}

func TestExchangeRefusesAWrongVerifier(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	code := codeFor(t, issuer, client.AuthURL("s", "n", "verifier-verifier-verifier-verifier-verifier"), authtest.Grant{Subject: "s", Email: "e@example.com"})
	if _, err := client.Exchange(context.Background(), code, "another-verifier-another-verifier-another", "n"); err == nil {
		t.Fatal("Exchange succeeded with a verifier that does not match the challenge")
	}
}

func TestExchangeRefusesAMissingEmail(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	v := "verifier-verifier-verifier-verifier-verifier"
	code := codeFor(t, issuer, client.AuthURL("s", "n", v), authtest.Grant{Subject: "s"})
	if _, err := client.Exchange(context.Background(), code, v, "n"); err == nil {
		t.Fatal("Exchange accepted an ID token without email")
	}
}

func TestLogoutURL(t *testing.T) {
	without, _ := newClient(t, authtest.Options{})
	if got := without.LogoutURL("http://kanban.test/login"); got != "" {
		t.Errorf("LogoutURL without end_session_endpoint = %q, want empty", got)
	}
	with, issuer := newClient(t, authtest.Options{EndSession: true})
	got := with.LogoutURL("http://kanban.test/login")
	if !strings.HasPrefix(got, issuer.EndSessionURL()+"?") {
		t.Fatalf("LogoutURL = %q", got)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("client_id") != issuer.ClientID || u.Query().Get("post_logout_redirect_uri") != "http://kanban.test/login" {
		t.Errorf("LogoutURL query = %v", u.Query())
	}
}

func TestNewClientFailsWhenTheIssuerIsUnreachable(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	_, err := auth.NewClient(context.Background(), auth.ClientConfig{Issuer: dead.URL, ClientID: "x", RedirectURL: redirectURL})
	if err == nil {
		t.Fatal("NewClient succeeded against an unreachable issuer")
	}
}
```

- [ ] **Step 4: Testlerin başarısız olduğunu gör**

Run: `go test ./internal/auth/...`
Expected: FAIL — `undefined: auth.NewClient`, `undefined: auth.SafeNext`

- [ ] **Step 5: `next.go`'yu yaz**

```go
package auth

import (
	"net/url"
	"strings"
)

// SafeNext returns next when it is a path on this site, and "/" otherwise, so
// a link cannot use sign-in to send a reader somewhere else.
func SafeNext(next string) string {
	if next == "" || len(next) > 512 || !strings.HasPrefix(next, "/") ||
		strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n\t") {
		return "/"
	}
	for _, r := range next {
		if r < 0x20 || r == 0x7f {
			return "/"
		}
	}
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return "/"
	}
	return next
}
```

- [ ] **Step 6: `client.go`'yu yaz**

```go
// Package auth signs readers in with OpenID Connect and puts the signed-in
// user in the request's context.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"kanban/internal/store"
)

var (
	// ErrNonce reports an ID token that does not answer this sign-in.
	ErrNonce = errors.New("auth: id token nonce does not match")
	// ErrNoIDToken reports a token response with no ID token in it.
	ErrNoIDToken = errors.New("auth: token response has no id_token")
	// ErrNoEmail reports an ID token without the email claim.
	ErrNoEmail = errors.New("auth: id token has no email claim")
)

// ClientConfig names the provider and this application's registration with it.
type ClientConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Client is an OpenID Connect relying party using the authorization code flow
// with PKCE.
type Client struct {
	oauth      oauth2.Config
	verifier   *oidc.IDTokenVerifier
	clientID   string
	endSession string
}

type providerExtras struct {
	EndSessionEndpoint string `json:"end_session_endpoint"`
}

type profileClaims struct {
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

// NewClient runs discovery against the issuer; an unreachable or malformed
// provider is an error, so the application does not start without one.
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discovery at %s: %w", cfg.Issuer, err)
	}
	var extras providerExtras
	if err := provider.Claims(&extras); err != nil {
		return nil, fmt.Errorf("auth: discovery document: %w", err)
	}
	return &Client{
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
		},
		verifier:   provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		clientID:   cfg.ClientID,
		endSession: extras.EndSessionEndpoint,
	}, nil
}

// AuthURL is where the reader is sent to sign in.
func (c *Client) AuthURL(state, nonce, verifier string) string {
	return c.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

// Exchange trades the code for tokens and returns who the verified ID token
// says signed in. The token's issuer, audience and expiry are checked by the
// verifier; its nonce is checked here.
func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (store.Identity, error) {
	token, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return store.Identity{}, fmt.Errorf("auth: token exchange: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return store.Identity{}, ErrNoIDToken
	}
	idToken, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return store.Identity{}, fmt.Errorf("auth: id token: %w", err)
	}
	if nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return store.Identity{}, ErrNonce
	}
	var claims profileClaims
	if err := idToken.Claims(&claims); err != nil {
		return store.Identity{}, fmt.Errorf("auth: id token claims: %w", err)
	}
	if claims.Email == "" {
		return store.Identity{}, ErrNoEmail
	}
	// Authentik may send an empty name; its username is the next best thing,
	// and the store falls back to the email after that.
	name := claims.Name
	if strings.TrimSpace(name) == "" {
		name = claims.PreferredUsername
	}
	return store.Identity{Issuer: idToken.Issuer, Subject: idToken.Subject, Email: claims.Email, Name: name}, nil
}

// LogoutURL is the provider's RP-initiated logout for this client, returning
// the reader to postLogoutRedirect, or "" when the provider offers none.
func (c *Client) LogoutURL(postLogoutRedirect string) string {
	if c.endSession == "" {
		return ""
	}
	u, err := url.Parse(c.endSession)
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Set("client_id", c.clientID)
	q.Set("post_logout_redirect_uri", postLogoutRedirect)
	u.RawQuery = q.Encode()
	return u.String()
}
```

- [ ] **Step 7: Testlerin geçtiğini gör**

Run: `go test ./internal/auth/... -v -race`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: OIDC client with PKCE and nonce checks, fake issuer for tests"
```

---

### Task 5: Giriş/çıkış action'ları ve kullanıcı middleware'i

**Files:**
- Create: `internal/auth/service.go`, `internal/auth/middleware.go`
- Create: `internal/webtest/browser.go`
- Test: `internal/auth/service_test.go`

**Interfaces:**
- Consumes: `auth.Client`, `auth.SafeNext` (Task 4); `authtest.Issuer`, `authtest.Grant` (Task 4); `store.User`, `store.Identity`, `store.ErrNotFound` (Task 3)
- Produces:
  ```go
  package auth
  type Users interface {
      UpsertIdentity(ctx context.Context, id store.Identity, adminEmails []string, locale string) (store.User, error)
      UserByID(ctx context.Context, id int64) (store.User, error)
  }
  type Options struct {
      Client        *Client
      Users         Users
      AdminEmails   []string
      DefaultLocale string
      Locales       []string
      BaseURL       string        // "http://kanban.test", sonunda "/" yok
      CallbackPath  string        // IdP'de kayıtlı redirect URI'nin path'i, ör. "/auth/openid/authentik"
      FailedPage    *collage.Page // reddedilen callback'e 400 ile render edilir; kayıtlı olmalı
      Logger        *slog.Logger
  }
  type Service struct{ /* … */ }
  func NewService(opts Options) *Service
  func (s *Service) Actions() []*collage.Action   // "login" GET, "auth-callback" GET (CallbackPath), "logout" POST
  func (s *Service) Middleware(next http.Handler) http.Handler
  func UserFrom(ctx context.Context) (store.User, bool)
  func WithUser(ctx context.Context, u store.User) context.Context

  package webtest
  const Origin = "http://kanban.test"
  type Response struct { Status int; Header http.Header; Body string }
  func (r Response) Location() string
  type Browser struct{ /* … */ }
  func NewBrowser(t testing.TB, h http.Handler) *Browser
  func (b *Browser) Get(path string) Response
  func (b *Browser) Post(path string, form url.Values) Response
  func (b *Browser) Submit(page, action string, form url.Values) Response // page'deki CSRF token'ıyla post
  func (b *Browser) HasCookie(name string) bool
  func CSRFToken(t testing.TB, body string) string
  func SignIn(t testing.TB, b *Browser, issuer *authtest.Issuer, g authtest.Grant)
  ```

- [ ] **Step 1: Session eklentisini ekle, test tarayıcısını yaz**

```bash
go get github.com/Elagoht/collage-session@v0.2.1
```

`internal/webtest/browser.go`:

```go
// Package webtest drives the application through its handler the way a
// browser would: it keeps cookies and carries forgery tokens.
package webtest

import (
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"kanban/internal/auth/authtest"
)

// Origin is the scheme and host every request is made to.
const Origin = "http://kanban.test"

// Response is what one request was answered with.
type Response struct {
	Status int
	Header http.Header
	Body   string
}

// Location is the redirect target, if any.
func (r Response) Location() string { return r.Header.Get("Location") }

// Browser keeps the cookies a site sets between requests.
type Browser struct {
	t       testing.TB
	handler http.Handler
	cookies map[string]string
}

// NewBrowser returns a browser with no cookies.
func NewBrowser(t testing.TB, h http.Handler) *Browser {
	return &Browser{t: t, handler: h, cookies: map[string]string{}}
}

// Get requests path, without following redirects.
func (b *Browser) Get(path string) Response {
	return b.do(httptest.NewRequest(http.MethodGet, Origin+path, nil))
}

// Post submits form to path from this site's own origin.
func (b *Browser) Post(path string, form url.Values) Response {
	req := httptest.NewRequest(http.MethodPost, Origin+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", Origin)
	return b.do(req)
}

// Submit loads page, copies its forgery token into form and posts it to action.
func (b *Browser) Submit(page, action string, form url.Values) Response {
	b.t.Helper()
	res := b.Get(page)
	if res.Status != http.StatusOK {
		b.t.Fatalf("webtest: GET %s = %d, want 200", page, res.Status)
	}
	form.Set("_csrf", CSRFToken(b.t, res.Body))
	return b.Post(action, form)
}

// HasCookie reports whether the browser holds a cookie named name.
func (b *Browser) HasCookie(name string) bool {
	_, ok := b.cookies[name]
	return ok
}

func (b *Browser) do(req *http.Request) Response {
	b.t.Helper()
	for name, value := range b.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	rec := httptest.NewRecorder()
	b.handler.ServeHTTP(rec, req)
	res := rec.Result()
	for _, c := range res.Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		b.t.Fatal(err)
	}
	return Response{Status: res.StatusCode, Header: res.Header, Body: string(body)}
}

var csrfInput = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

// CSRFToken returns the forgery token in the first form of body.
func CSRFToken(t testing.TB, body string) string {
	t.Helper()
	m := csrfInput.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("webtest: no forgery token in the page:\n%s", body)
	}
	return html.UnescapeString(m[1])
}

// SignIn signs b in through /login and the provider as g.
func SignIn(t testing.TB, b *Browser, issuer *authtest.Issuer, g authtest.Grant) {
	t.Helper()
	res := b.Get("/login")
	if res.Status != http.StatusSeeOther {
		t.Fatalf("webtest: GET /login = %d, want 303", res.Status)
	}
	res = b.Get(issuer.Authorize(t, res.Location(), g))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("webtest: callback = %d, want 303:\n%s", res.Status, res.Body)
	}
}
```

- [ ] **Step 2: Başarısız testi yaz**

`internal/auth/service_test.go`:

```go
package auth_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	session "github.com/Elagoht/collage-session"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/auth/authtest"
	"kanban/internal/store"
	"kanban/internal/webtest"
)

type fakeUsers struct {
	mu     sync.Mutex
	nextID int64
	byKey  map[string]int64
	users  map[int64]store.User
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byKey: map[string]int64{}, users: map[int64]store.User{}}
}

func (f *fakeUsers) UpsertIdentity(_ context.Context, id store.Identity, _ []string, locale string) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := id.Issuer + "|" + id.Subject
	uid, ok := f.byKey[key]
	if !ok {
		f.nextID++
		uid = f.nextID
		f.byKey[key] = uid
	}
	u := f.users[uid]
	u.ID, u.Issuer, u.Subject, u.Email, u.Name = uid, id.Issuer, id.Subject, id.Email, id.Name
	if u.Locale == "" {
		u.Locale = locale
	}
	f.users[uid] = u
	return u, nil
}

func (f *fakeUsers) UserByID(_ context.Context, id int64) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) disable(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, u := range f.users {
		if u.Subject == subject {
			now := time.Now()
			u.DisabledAt = &now
			f.users[id] = u
		}
	}
}

type fixture struct {
	users   *fakeUsers
	issuer  *authtest.Issuer
	handler http.Handler
}

func newFixture(t *testing.T, opts authtest.Options) *fixture {
	t.Helper()
	client, issuer := newClient(t, opts)
	users := newFakeUsers()
	key := []byte(strings.Repeat("k", 32))

	app, err := collage.New(&collage.Config{
		Template: collage.TemplateConfig{
			FS: fstest.MapFS{
				"t/private.html": {Data: []byte(`<p>hello {{.Name}}</p><form method="post" action="{{actionURL "logout"}}">{{csrfToken}}</form>`)},
				"t/failed.html":  {Data: []byte(`<p>sign-in failed</p>`)},
			},
			Root: "t", Extension: ".html",
		},
		Locale:   collage.LocaleConfig{Default: "tr", Supported: []string{"tr", "en"}},
		Security: collage.SecurityConfig{CSRFKey: key},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Plugins:  []collage.Plugin{session.New(session.Options{Key: key})},
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := collage.NewPage("auth-failed").
		WithContent(collage.NewFragment("auth-failed-content", "failed.html").Build()).Build()
	if err := app.RegisterPage(failed); err != nil {
		t.Fatal(err)
	}
	svc := auth.NewService(auth.Options{
		Client: client, Users: users, DefaultLocale: "tr", Locales: []string{"tr", "en"},
		BaseURL: webtest.Origin, CallbackPath: "/auth/openid/authentik", FailedPage: failed,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	for _, a := range svc.Actions() {
		if err := app.RegisterAction(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.Use(svc.Middleware); err != nil {
		t.Fatal(err)
	}
	private := collage.NewFragment("private-content", "private.html").
		WithGuard(session.RequireUser("/login")).
		WithDataHandler(collage.Load(func(ctx context.Context, _ *collage.RenderContext) (store.User, error) {
			u, _ := auth.UserFrom(ctx)
			return u, nil
		})).
		Build()
	page := collage.NewPage("private").WithContent(private).
		WithPath("tr", "/private").WithPath("en", "/private").Dynamic().Build()
	if err := app.RegisterPage(page); err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	return &fixture{users: users, issuer: issuer, handler: app.Handler()}
}

var ada = authtest.Grant{Subject: "ada", Email: "ada@example.com", Name: "Ada"}

func TestLoginSendsTheReaderToTheProvider(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	res := webtest.NewBrowser(t, f.handler).Get("/login?next=/private")
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), f.issuer.URL+"/authorize?") {
		t.Fatalf("GET /login = %d %q", res.Status, res.Location())
	}
	u, _ := url.Parse(res.Location())
	if u.Query().Get("code_challenge_method") != "S256" || !strings.Contains(u.Query().Get("scope"), "openid") {
		t.Errorf("authorization request = %v", u.Query())
	}
}

func TestSignInThenOut(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)

	if res := b.Get("/private"); res.Status != http.StatusSeeOther || res.Location() != "/login?next=%2Fprivate" {
		t.Fatalf("anonymous /private = %d %q", res.Status, res.Location())
	}
	res := b.Get("/login?next=/private")
	res = b.Get(f.issuer.Authorize(t, res.Location(), ada))
	if res.Status != http.StatusSeeOther || res.Location() != "/private" {
		t.Fatalf("callback = %d %q, want 303 /private:\n%s", res.Status, res.Location(), res.Body)
	}
	if res := b.Get("/private"); res.Status != http.StatusOK || !strings.Contains(res.Body, "hello Ada") {
		t.Fatalf("signed-in /private = %d:\n%s", res.Status, res.Body)
	}
	if u, err := f.users.UserByID(context.Background(), 1); err != nil || u.Issuer != f.issuer.URL || u.Subject != "ada" {
		t.Errorf("stored user = %+v, %v", u, err)
	}

	res = b.Submit("/private", "/logout", url.Values{})
	if res.Status != http.StatusSeeOther || res.Location() != "/login" {
		t.Fatalf("logout without end_session = %d %q, want 303 /login", res.Status, res.Location())
	}
	if res := b.Get("/private"); res.Status != http.StatusSeeOther {
		t.Fatalf("/private after logout = %d, want a redirect to /login", res.Status)
	}
}

func TestLogoutGoesThroughTheProviderWhenItOffersEndSession(t *testing.T) {
	f := newFixture(t, authtest.Options{EndSession: true})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)

	res := b.Submit("/private", "/logout", url.Values{})
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), f.issuer.EndSessionURL()+"?") {
		t.Fatalf("logout = %d %q", res.Status, res.Location())
	}
	u, _ := url.Parse(res.Location())
	if u.Query().Get("post_logout_redirect_uri") != webtest.Origin+"/login" {
		t.Errorf("post_logout_redirect_uri = %q", u.Query().Get("post_logout_redirect_uri"))
	}
}

func TestNextCannotLeaveTheSite(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	res := b.Get("/login?next=" + url.QueryEscape("//evil.example/x"))
	res = b.Get(f.issuer.Authorize(t, res.Location(), ada))
	if res.Location() != "/" {
		t.Fatalf("callback redirected to %q, want /", res.Location())
	}
}

func TestCallbackRefusals(t *testing.T) {
	cases := map[string]func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response{
		"wrong state": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			cb := f.issuer.Authorize(t, res.Location(), ada)
			return b.Get(strings.Replace(cb, "state=", "state=x", 1))
		},
		"no flow in the session": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			other := webtest.NewBrowser(t, f.handler)
			res := other.Get("/login")
			return b.Get(f.issuer.Authorize(t, res.Location(), ada)) // b never visited /login
		},
		"wrong nonce": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			return b.Get(f.issuer.Authorize(t, res.Location(), authtest.Grant{Subject: "ada", Email: "ada@example.com", Nonce: "evil"}))
		},
		"wrong audience": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			return b.Get(f.issuer.Authorize(t, res.Location(), authtest.Grant{Subject: "ada", Email: "ada@example.com", Audience: "other"}))
		},
		"provider error": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			u, _ := url.Parse(res.Location())
			return b.Get("/auth/openid/authentik?error=access_denied&state=" + url.QueryEscape(u.Query().Get("state")))
		},
		"replayed callback": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			cb := f.issuer.Authorize(t, res.Location(), ada)
			if first := b.Get(cb); first.Status != http.StatusSeeOther {
				t.Fatalf("first callback = %d", first.Status)
			}
			b.Submit("/private", "/logout", url.Values{})
			return b.Get(cb)
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, authtest.Options{})
			b := webtest.NewBrowser(t, f.handler)
			res := run(t, f, b)
			if res.Status != http.StatusBadRequest || !strings.Contains(res.Body, "sign-in failed") {
				t.Fatalf("callback = %d, want 400 with the failure page:\n%s", res.Status, res.Body)
			}
			if res := b.Get("/private"); res.Status != http.StatusSeeOther {
				t.Fatalf("/private after a refused callback = %d, want a redirect", res.Status)
			}
		})
	}
}

func TestADisabledUserIsSignedOutOnTheNextRequest(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)
	if res := b.Get("/private"); res.Status != http.StatusOK {
		t.Fatalf("/private = %d", res.Status)
	}

	f.users.disable("ada")

	res := b.Get("/private")
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/login") {
		t.Fatalf("/private for a disabled user = %d %q", res.Status, res.Location())
	}
	if b.HasCookie("collage_session") {
		t.Error("the session cookie was kept")
	}
}

func TestADisabledUserCannotSignIn(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)
	f.users.disable("ada")

	fresh := webtest.NewBrowser(t, f.handler)
	res := fresh.Get("/login")
	res = fresh.Get(f.issuer.Authorize(t, res.Location(), ada))
	if res.Status != http.StatusBadRequest {
		t.Fatalf("callback for a disabled user = %d, want 400", res.Status)
	}
}

func TestLoginWhileSignedInGoesToNext(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)
	res := b.Get("/login?next=/private")
	if res.Status != http.StatusSeeOther || res.Location() != "/private" {
		t.Fatalf("GET /login signed in = %d %q", res.Status, res.Location())
	}
}
```

- [ ] **Step 3: Testin başarısız olduğunu gör**

Run: `go test ./internal/auth/`
Expected: FAIL — `undefined: auth.NewService`, `undefined: auth.UserFrom`

- [ ] **Step 4: `middleware.go`'yu yaz**

```go
package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	session "github.com/Elagoht/collage-session"

	"kanban/internal/store"
)

type userKey struct{}

// WithUser returns ctx carrying u as the signed-in user.
func WithUser(ctx context.Context, u store.User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

// UserFrom returns the signed-in user, if there is one.
func UserFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userKey{}).(store.User)
	return u, ok
}

// Middleware turns the session's user id into the user, read from the
// database on every request, so a disabled or deleted user is signed out on
// their next request. It must run inside collage-session's middleware, which
// it does when the session plugin is in Config.Plugins (collage v0.38.0+).
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		raw := sess.Get(session.UserKey)
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			sess.Clear()
			next.ServeHTTP(w, r)
			return
		}
		user, err := s.opts.Users.UserByID(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrNotFound), err == nil && user.Disabled():
			sess.Clear()
		case err != nil:
			s.opts.Logger.Error("auth: load signed-in user", "user", id, "err", err)
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		default:
			r = r.WithContext(WithUser(r.Context(), user))
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: `service.go`'yu yaz**

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"

	session "github.com/Elagoht/collage-session"
	"github.com/Elagoht/collage/pkg/collage"
	"golang.org/x/oauth2"

	"kanban/internal/store"
)

// Users is what signing in needs from the store.
type Users interface {
	UpsertIdentity(ctx context.Context, id store.Identity, adminEmails []string, locale string) (store.User, error)
	UserByID(ctx context.Context, id int64) (store.User, error)
}

// Options configures the sign-in flow.
type Options struct {
	Client        *Client
	Users         Users
	AdminEmails   []string
	DefaultLocale string
	Locales       []string
	BaseURL       string
	CallbackPath  string
	FailedPage    *collage.Page
	Logger        *slog.Logger
}

// Service is sign-in, its callback and sign-out.
type Service struct {
	opts Options
}

// NewService returns the sign-in flow.
func NewService(opts Options) *Service { return &Service{opts: opts} }

// Session keys holding one sign-in attempt between /login and the callback.
const (
	keyState    = "oidc_state"
	keyNonce    = "oidc_nonce"
	keyVerifier = "oidc_verifier"
	keyNext     = "oidc_next"
)

// Actions returns the routes of the flow: GET /login and POST /logout in every
// locale, and the callback at the path of the redirect URI registered with the
// provider, in the default locale only.
func (s *Service) Actions() []*collage.Action {
	login := collage.NewAction("login").WithMethods(http.MethodGet).WithHandler(s.login)
	logout := collage.NewAction("logout").WithMethods(http.MethodPost).WithHandler(s.logout)
	for _, l := range s.opts.Locales {
		login = login.WithPath(l, "/login")
		logout = logout.WithPath(l, "/logout")
	}
	callback := collage.NewAction("auth-callback").WithMethods(http.MethodGet).
		WithPath(s.opts.DefaultLocale, s.opts.CallbackPath).WithHandler(s.callback)
	return []*collage.Action{login.Build(), callback.Build(), logout.Build()}
}

func (s *Service) login(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	next := SafeNext(rc.Request.URL.Query().Get("next"))
	if _, ok := UserFrom(ctx); ok {
		return collage.SeeOther(next), nil
	}
	state, err := randomToken()
	if err != nil {
		return nil, err
	}
	nonce, err := randomToken()
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	sess := session.Get(rc)
	for _, kv := range [][2]string{{keyState, state}, {keyNonce, nonce}, {keyVerifier, verifier}, {keyNext, next}} {
		if err := sess.Set(kv[0], kv[1]); err != nil {
			return nil, err
		}
	}
	return collage.SeeOther(s.opts.Client.AuthURL(state, nonce, verifier)), nil
}

func (s *Service) callback(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	sess := session.Get(rc)
	state, nonce, verifier, next := sess.Get(keyState), sess.Get(keyNonce), sess.Get(keyVerifier), sess.Get(keyNext)
	// One attempt per visit to /login: the flow is spent before anything can fail.
	for _, k := range []string{keyState, keyNonce, keyVerifier, keyNext} {
		sess.Delete(k)
	}

	q := rc.Request.URL.Query()
	if state == "" || verifier == "" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
		return s.fail("state does not match this session's sign-in"), nil
	}
	if e := q.Get("error"); e != "" {
		return s.fail("provider answered " + e), nil
	}
	id, err := s.opts.Client.Exchange(ctx, q.Get("code"), verifier, nonce)
	if err != nil {
		return s.fail(err.Error()), nil
	}
	user, err := s.opts.Users.UpsertIdentity(ctx, id, s.opts.AdminEmails, s.opts.DefaultLocale)
	if err != nil {
		return nil, err
	}
	if user.Disabled() {
		return s.fail("user " + strconv.FormatInt(user.ID, 10) + " is disabled"), nil
	}
	sess.Regenerate()
	if err := sess.Set(session.UserKey, strconv.FormatInt(user.ID, 10)); err != nil {
		return nil, err
	}
	return collage.SeeOther(SafeNext(next)), nil
}

func (s *Service) logout(_ context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	session.Get(rc).Clear()
	if u := s.opts.Client.LogoutURL(s.opts.BaseURL + "/login"); u != "" {
		return collage.SeeOther(u), nil
	}
	return collage.SeeOther("/login"), nil
}

func (s *Service) fail(reason string) *collage.ActionResult {
	s.opts.Logger.Warn("auth: sign-in refused", "reason", reason)
	return &collage.ActionResult{Status: http.StatusBadRequest, Page: s.opts.FailedPage}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

> Varsayılan locale dışındaki bir kullanıcı çıkış yaptığında `/login`'e, yani varsayılan dile döner. Bu aşamada bu kabul edilebilir.

- [ ] **Step 6: Testlerin geçtiğini gör**

Run: `go test ./internal/auth/... ./internal/webtest/... -v -race`
Expected: PASS

Bir test collage ya da session eklentisinin dokümana aykırı davranışı yüzünden geçmiyorsa (örneğin `Delete` bir `Page` cevabıyla cookie'ye yazılmıyorsa ya da GET action'da `Regenerate` çalışmıyorsa), Global Constraints'teki framework kuralını uygula: issue dosyasını yaz ve **dur**.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: OIDC sign-in, callback, logout and per-request user loading"
```

---

### Task 6: Uygulama kabuğu — eklentiler, layout'lar, kataloglar, hata sayfaları, ana sayfa

**Files:**
- Create: `internal/web/app.go`, `internal/web/layouts.go`, `internal/web/pages_errors.go`, `internal/web/pages_home.go`
- Create: `templates/layouts/base.html`, `templates/layouts/app.html`, `templates/pages/home.html`, `templates/pages/auth_failed.html`, `templates/pages/not_found.html`, `templates/pages/error.html`
- Delete (scaffold): `templates/layouts/default.html`, `templates/pages/home.html` (yenisiyle değiştirilir)
- Modify: `static/app.css`
- Create: `locales/tr.json`, `locales/en.json`
- Test: `internal/web/helpers_test.go`, `internal/web/app_test.go`

**Interfaces:**
- Consumes: `config.Config`, `config.Locales` (Task 1); `store.*` (Task 3); `auth.Client`, `auth.NewService`, `auth.UserFrom` (Task 4–5); `authtest`, `webtest`, `dbtest`
- Produces:
  ```go
  package web
  type Deps struct {
      Files   fs.FS // templates/, static/, locales/ içeren kök
      DevMode bool
      Host    string
      Port    int
      Config  config.Config
      Store   *store.Store
      OIDC    *auth.Client
      Logger  *slog.Logger
  }
  func New(d Deps) (*collage.App, error)
  ```
  Sayfa adları (sonraki task'lar `pageURL` ile kullanır): `"home"` `/`, `"teams"` `/teams`, `"team"` `/teams/{id}`, `"admin-users"` `/admin/users`. Action adları: `"login"`, `"logout"`, `"auth-callback"`, sayfa action'ları `"teams:POST"` vb.

- [ ] **Step 1: Eklentileri ekle, katalogları yaz** (Aşama 1'in tüm anahtarları; sonraki task'lar anahtar eklemez)

```bash
go get github.com/Elagoht/collage-i18n@v0.2.0 github.com/Elagoht/collage-validate@v0.1.3 \
  github.com/Elagoht/collage-flash@v0.1.2 github.com/Elagoht/collage-live@v0.3.0 github.com/Elagoht/collage-secure@v0.1.3
```


`locales/tr.json`:

```json
{
  "app": { "name": "Kanban" },
  "locale": { "switcher": "Dil" },
  "nav": { "home": "Ana sayfa", "teams": "Takımlar", "users": "Kullanıcılar" },
  "auth": {
    "logout": "Çıkış yap",
    "failed": {
      "title": "Giriş yapılamadı",
      "body": "Giriş işlemi tamamlanamadı. Lütfen yeniden deneyin.",
      "retry": "Yeniden dene"
    }
  },
  "errors": {
    "not_found": { "title": "Sayfa bulunamadı", "body": "Aradığınız sayfa yok ya da görme izniniz yok." },
    "server": { "title": "Bir şeyler ters gitti", "body": "İsteğiniz tamamlanamadı. Lütfen biraz sonra yeniden deneyin." }
  },
  "home": { "title": "Takımlarım", "empty": "Henüz bir takımda değilsiniz." },
  "teams": {
    "title": "Takımlar",
    "empty": "Henüz takım yok.",
    "name": "Takım adı",
    "create": "Takım oluştur",
    "created": "{name} oluşturuldu.",
    "member_count": { "one": "{count} üye", "other": "{count} üye" }
  },
  "team": {
    "members": "Üyeler",
    "role": "Rol",
    "roles": { "lead": "Lider", "member": "Üye" },
    "add": "Üye ekle",
    "email": "E-posta",
    "save": "Kaydet",
    "remove": "Çıkar",
    "added": "{name} takıma eklendi.",
    "removed": "{name} takımdan çıkarıldı.",
    "role_changed": "{name} için rol güncellendi.",
    "user_not_found": "Bu e-postayla kayıtlı bir kullanıcı yok. Kullanıcının önce bir kez giriş yapması gerekir.",
    "user_ambiguous": "Bu e-posta birden fazla kullanıcıya ait."
  },
  "users": {
    "title": "Kullanıcılar",
    "name": "Ad",
    "email": "E-posta",
    "admin": "Admin",
    "status": "Durum",
    "active": "Aktif",
    "disabled": "Devre dışı",
    "you": "(siz)",
    "make_admin": "Admin yap",
    "revoke_admin": "Adminliği kaldır",
    "disable": "Devre dışı bırak",
    "enable": "Etkinleştir",
    "saved": "Kaydedildi.",
    "last_admin": "En az bir aktif admin kalmalı.",
    "self_disable": "Kendi hesabınızı devre dışı bırakamazsınız."
  }
}
```

`locales/en.json`:

```json
{
  "app": { "name": "Kanban" },
  "locale": { "switcher": "Language" },
  "nav": { "home": "Home", "teams": "Teams", "users": "Users" },
  "auth": {
    "logout": "Sign out",
    "failed": {
      "title": "Sign-in failed",
      "body": "Signing in could not be completed. Please try again.",
      "retry": "Try again"
    }
  },
  "errors": {
    "not_found": { "title": "Page not found", "body": "The page does not exist, or you may not see it." },
    "server": { "title": "Something went wrong", "body": "Your request could not be completed. Please try again shortly." }
  },
  "home": { "title": "My teams", "empty": "You are not in a team yet." },
  "teams": {
    "title": "Teams",
    "empty": "There are no teams yet.",
    "name": "Team name",
    "create": "Create team",
    "created": "{name} was created.",
    "member_count": { "one": "{count} member", "other": "{count} members" }
  },
  "team": {
    "members": "Members",
    "role": "Role",
    "roles": { "lead": "Lead", "member": "Member" },
    "add": "Add member",
    "email": "Email",
    "save": "Save",
    "remove": "Remove",
    "added": "{name} was added to the team.",
    "removed": "{name} was removed from the team.",
    "role_changed": "Role updated for {name}.",
    "user_not_found": "No user has this email. They need to sign in once first.",
    "user_ambiguous": "More than one user has this email."
  },
  "users": {
    "title": "Users",
    "name": "Name",
    "email": "Email",
    "admin": "Admin",
    "status": "Status",
    "active": "Active",
    "disabled": "Disabled",
    "you": "(you)",
    "make_admin": "Make admin",
    "revoke_admin": "Revoke admin",
    "disable": "Disable",
    "enable": "Enable",
    "saved": "Saved.",
    "last_admin": "At least one active admin must remain.",
    "self_disable": "You cannot disable your own account."
  }
}
```

- [ ] **Step 2: Template'leri yaz**

`templates/layouts/base.html`:

```html
<!doctype html>
<html lang="{{.Locale}}">

<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  {{hoist "head"}}
  <link rel="stylesheet" href="{{asset "/static/app.css"}}">
  {{liveClient}}
</head>

<body>
  <nav class="locale-switcher" aria-label="{{t "locale.switcher"}}">
    {{with localeURL "tr"}}<a hreflang="tr" lang="tr" href="{{.}}">Türkçe</a>{{end}}
    {{with localeURL "en"}}<a hreflang="en" lang="en" href="{{.}}">English</a>{{end}}
  </nav>
  {{slot "content"}}
</body>

</html>
```

`templates/layouts/app.html`:

```html
<header class="topbar">
  <a class="topbar__home" href="{{pageURL "home"}}">{{t "app.name"}}</a>
  <nav class="topbar__nav">
    <a href="{{pageURL "teams"}}">{{t "nav.teams"}}</a>
    {{if .User.IsAdmin}}<a href="{{pageURL "admin-users"}}">{{t "nav.users"}}</a>{{end}}
  </nav>
  <span class="topbar__user">{{.User.Name}}</span>
  <form class="topbar__logout" method="post" action="{{actionURL "logout"}}">
    {{csrfToken}}
    <button type="submit">{{t "auth.logout"}}</button>
  </form>
</header>
{{range flashes}}<p class="flash flash--{{.Kind}}" role="status">{{.Text}}</p>{{end}}
<main class="page">{{slot "content"}}</main>
```

`templates/pages/home.html`:

```html
<h1>{{t "home.title"}}</h1>
{{if .Teams}}
<ul class="list">
  {{range .Teams}}
  <li><a href="{{pageURL "team" "id" .Team.ID}}">{{.Team.Name}}</a> <span class="muted">{{tn "teams.member_count" .MemberCount}}</span></li>
  {{end}}
</ul>
{{else}}
<p class="muted">{{t "home.empty"}}</p>
{{end}}
```

`templates/pages/auth_failed.html`:

```html
<main class="page page--narrow">
  <h1>{{t "auth.failed.title"}}</h1>
  <p>{{t "auth.failed.body"}}</p>
  <p><a href="{{actionURL "login"}}">{{t "auth.failed.retry"}}</a></p>
</main>
```

`templates/pages/not_found.html`:

```html
<main class="page page--narrow">
  <h1>{{t "errors.not_found.title"}}</h1>
  <p>{{t "errors.not_found.body"}}</p>
  <p><a href="{{pageURL "home"}}">{{t "nav.home"}}</a></p>
</main>
```

`templates/pages/error.html`:

```html
<main class="page page--narrow">
  <h1>{{t "errors.server.title"}}</h1>
  <p>{{t "errors.server.body"}}</p>
</main>
```

`static/app.css` (scaffold'unkini değiştir):

```css
:root { --fg: #1d1f23; --muted: #6b7280; --bg: #fafafa; --line: #e5e7eb; --accent: #2563eb; --ok: #15803d; --bad: #b91c1c; }
@media (prefers-color-scheme: dark) { :root { --fg: #e5e7eb; --muted: #9ca3af; --bg: #111317; --line: #2a2f37; --accent: #60a5fa; --ok: #4ade80; --bad: #f87171; } }
* { box-sizing: border-box; }
body { margin: 0; font: 15px/1.5 system-ui, sans-serif; color: var(--fg); background: var(--bg); }
a { color: var(--accent); }
.locale-switcher { display: flex; gap: .75rem; justify-content: flex-end; padding: .25rem 1rem; font-size: .85rem; }
.topbar { display: flex; align-items: center; gap: 1rem; padding: .5rem 1rem; border-bottom: 1px solid var(--line); }
.topbar__home { font-weight: 600; text-decoration: none; }
.topbar__nav { display: flex; gap: 1rem; flex: 1; }
.topbar__user { color: var(--muted); }
.page { max-width: 960px; margin: 0 auto; padding: 1rem; }
.page--narrow { max-width: 560px; }
.muted { color: var(--muted); }
.list { padding-left: 1.25rem; }
.table { width: 100%; border-collapse: collapse; }
.table th, .table td { text-align: left; padding: .4rem .5rem; border-bottom: 1px solid var(--line); }
.form { display: grid; gap: .5rem; max-width: 420px; margin-top: 1rem; }
.inline { display: inline-flex; gap: .25rem; align-items: center; margin: 0; }
.error { color: var(--bad); margin: 0; }
.flash { max-width: 960px; margin: .5rem auto; padding: .5rem 1rem; border-left: 3px solid var(--accent); }
.flash--success { border-color: var(--ok); }
.flash--error { border-color: var(--bad); }
```

```bash
rm -f templates/layouts/default.html
```

- [ ] **Step 3: Başarısız testi yaz**

`internal/web/helpers_test.go`:

```go
package web_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/auth/authtest"
	"kanban/internal/config"
	"kanban/internal/db/dbtest"
	"kanban/internal/store"
	"kanban/internal/web"
	"kanban/internal/webtest"
)

type harness struct {
	t      *testing.T
	app    *collage.App
	issuer *authtest.Issuer
	store  *store.Store
}

// newHarness builds the real application over a fresh database and a fake
// provider. adminEmails is ADMIN_EMAILS, raw.
func newHarness(t *testing.T, adminEmails string) *harness {
	t.Helper()
	return build(t, store.New(dbtest.New(t)), adminEmails)
}

// newHarnessWithoutDB builds the application for tests that only register it.
func newHarnessWithoutDB(t *testing.T) *harness {
	t.Helper()
	return build(t, store.New(nil), "")
}

func build(t *testing.T, s *store.Store, adminEmails string) *harness {
	t.Helper()
	callback := webtest.Origin + "/auth/openid/authentik"
	issuer := authtest.NewIssuer(t, authtest.Options{RedirectURL: callback})
	key := strings.Repeat("ab", 32)
	cfg, err := config.Load(func(k string) string {
		return map[string]string{
			"BASE_URL": webtest.Origin, "DATABASE_URL": "unused", "DEFAULT_LOCALE": "tr",
			"OIDC_ISSUER": issuer.URL, "OIDC_CLIENT_ID": issuer.ClientID, "OIDC_CLIENT_SECRET": issuer.ClientSecret,
			"OIDC_REDIRECT_URL": callback,
			"ADMIN_EMAILS": adminEmails, "SESSION_KEY": key, "CSRF_KEY": key, "FLASH_KEY": key,
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	client, err := auth.NewClient(context.Background(), auth.ClientConfig{
		Issuer: issuer.URL, ClientID: issuer.ClientID, ClientSecret: issuer.ClientSecret,
		RedirectURL: cfg.OIDC.RedirectURL, Scopes: cfg.OIDC.Scopes,
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	app, err := web.New(web.Deps{
		Files: root.FS(), Config: cfg, Store: s, OIDC: client,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return &harness{t: t, app: app, issuer: issuer, store: s}
}

func (h *harness) browser() *webtest.Browser { return webtest.NewBrowser(h.t, h.app.Handler()) }

// signedIn returns a browser signed in as a new user with this subject and email.
func (h *harness) signedIn(subject, email string) *webtest.Browser {
	h.t.Helper()
	b := h.browser()
	webtest.SignIn(h.t, b, h.issuer, authtest.Grant{Subject: subject, Email: email, Name: strings.ToUpper(subject[:1]) + subject[1:]})
	return b
}

func (h *harness) user(email string) store.User {
	h.t.Helper()
	u, err := h.store.UserByEmail(context.Background(), email)
	if err != nil {
		h.t.Fatalf("user %s: %v", email, err)
	}
	return u
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("page does not contain %q:\n%s", w, body)
		}
	}
}
```

`internal/web/app_test.go`:

```go
package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Elagoht/collage/pkg/collage"
)

// Spec §2.1: a guarded page must render per request, never from a shared cache.
func TestEveryGuardedPageIsDynamic(t *testing.T) {
	h := newHarnessWithoutDB(t)
	guarded := 0
	for _, p := range h.app.Pages() {
		if !p.Guarded() {
			continue
		}
		guarded++
		if p.Strategy != collage.StrategyDynamic {
			t.Errorf("guarded page %q has strategy %v, want Dynamic", p.Name, p.Strategy)
		}
	}
	if guarded == 0 {
		t.Fatal("no guarded pages were registered; the check above checked nothing")
	}
}

func TestAnonymousReadersAreSentToLogin(t *testing.T) {
	h := newHarnessWithoutDB(t)
	b := h.browser()
	for path, want := range map[string]string{"/": "/login?next=%2F", "/en/": "/login?next=%2Fen%2F"} {
		res := b.Get(path)
		if res.Status != http.StatusSeeOther || res.Location() != want {
			t.Errorf("GET %s = %d %q, want 303 %q", path, res.Status, res.Location(), want)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := newHarnessWithoutDB(t)
	res := h.browser().Get("/no-such-page")
	if res.Status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Status)
	}
	csp := res.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self' 'nonce-", "form-action 'self' " + h.issuer.URL, "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q lacks %q", csp, want)
		}
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff missing")
	}
	mustContain(t, res.Body, "Sayfa bulunamadı", `lang="tr"`)
}

func TestNotFoundInEnglish(t *testing.T) {
	h := newHarnessWithoutDB(t)
	res := h.browser().Get("/en/no-such-page")
	mustContain(t, res.Body, "Page not found", `lang="en"`)
}

func TestHomeInBothLanguages(t *testing.T) {
	h := newHarness(t, "")
	b := h.signedIn("ada", "ada@example.com")

	tr := b.Get("/")
	if tr.Status != http.StatusOK {
		t.Fatalf("GET / = %d:\n%s", tr.Status, tr.Body)
	}
	mustContain(t, tr.Body, "Takımlarım", "Henüz bir takımda değilsiniz.", `hreflang="en" lang="en" href="/en`, "Çıkış yap", "Ada")

	en := b.Get("/en/")
	mustContain(t, en.Body, "My teams", "Sign out", `hreflang="tr" lang="tr" href="/"`)
	if !strings.Contains(en.Body, `href="/en/teams"`) {
		t.Errorf("links on an English page must stay English:\n%s", en.Body)
	}
}

func TestAuthFailedPageIsTranslated(t *testing.T) {
	h := newHarnessWithoutDB(t)
	res := h.browser().Get("/auth/openid/authentik?state=x&code=y")
	if res.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Status)
	}
	mustContain(t, res.Body, "Giriş yapılamadı")
}
```

- [ ] **Step 4: Testin başarısız olduğunu gör**

Run: `go test ./internal/web/`
Expected: FAIL — `undefined: web.New`

- [ ] **Step 5: `layouts.go`'yu yaz**

```go
package web

import (
	"context"
	"errors"

	session "github.com/Elagoht/collage-session"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/config"
	"kanban/internal/store"
)

var errNoUser = errors.New("web: a guarded page has no signed-in user")

// currentUser is the signed-in user; on a page under appLayout there is always one.
func currentUser(ctx context.Context) (store.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return store.User{}, errNoUser
	}
	return u, nil
}

// paths gives a page one path in every locale: /teams and /en/teams.
func paths(b *collage.PageBuilder, pattern string) *collage.PageBuilder {
	for _, l := range config.Locales {
		b = b.WithPath(l, pattern)
	}
	return b
}

type baseView struct {
	Locale string
}

// baseLayout is the document: <html>, the head, the language switcher. It
// depends only on the URL's locale, so it does not make a page dynamic.
func baseLayout() *collage.Fragment {
	return collage.NewFragment("base", "layouts/base.html").
		WithTitle("Kanban").
		WithDataHandler(collage.Load(func(_ context.Context, rc *collage.RenderContext) (baseView, error) {
			return baseView{Locale: rc.Locale}, nil
		})).
		Static().
		Build()
}

type appView struct {
	User store.User
}

// appLayout is every page for signed-in readers. Its guard sends anyone else
// to /login; pages under it must be Dynamic (spec §2.1).
func appLayout() *collage.Fragment {
	return collage.NewFragment("app", "layouts/app.html").
		WithGuard(session.RequireUser("/login")).
		WithDataHandler(collage.Load(func(ctx context.Context, _ *collage.RenderContext) (appView, error) {
			u, err := currentUser(ctx)
			return appView{User: u}, err
		})).
		Required().
		Build()
}

// privatePage starts a page under both layouts.
func privatePage(name string, content *collage.Fragment) *collage.PageBuilder {
	return collage.NewPage(name).WithLayouts(baseLayout(), appLayout()).WithContent(content)
}
```

- [ ] **Step 6: `pages_errors.go` ve `pages_home.go`'yu yaz**

`internal/web/pages_errors.go`:

```go
package web

import (
	"context"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"
)

// publicPage is a page with no path of its own, drawn in the base layout only:
// error pages and the sign-in failure.
func publicPage(name, template, titleKey string) *collage.Page {
	content := collage.NewFragment(name+"-content", template).
		WithDataHandler(collage.Effect(func(_ context.Context, rc *collage.RenderContext) error {
			rc.HoistTitle(i18n.T(rc, titleKey))
			return nil
		})).
		Build()
	return collage.NewPage(name).WithLayouts(baseLayout()).WithContent(content).Build()
}

func notFoundPage() *collage.Page {
	return publicPage("not-found", "pages/not_found.html", "errors.not_found.title")
}

func errorPage() *collage.Page {
	return publicPage("server-error", "pages/error.html", "errors.server.title")
}

func authFailedPage() *collage.Page {
	return publicPage("auth-failed", "pages/auth_failed.html", "auth.failed.title")
}
```

`internal/web/pages_home.go`:

```go
package web

import (
	"context"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type homeView struct {
	Teams []store.TeamSummary
}

func (h *handlers) homePage() *collage.Page {
	content := collage.NewFragment("home-content", "pages/home.html").
		WithDataHandler(collage.Load(h.loadHome)).
		Required().
		Build()
	return paths(privatePage("home", content), "/").Dynamic().Build()
}

func (h *handlers) loadHome(ctx context.Context, rc *collage.RenderContext) (homeView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return homeView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "home.title"))
	teams, err := h.store.TeamsOf(ctx, user.ID)
	return homeView{Teams: teams}, err
}
```

- [ ] **Step 7: `app.go`'yu yaz**

```go
// Package web is the application's pages and actions, drawn with collage.
package web

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"time"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	live "github.com/Elagoht/collage-live"
	secure "github.com/Elagoht/collage-secure"
	session "github.com/Elagoht/collage-session"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/config"
	"kanban/internal/store"
)

// Deps is what the application is built from.
type Deps struct {
	Files   fs.FS
	DevMode bool
	Host    string
	Port    int
	Config  config.Config
	Store   *store.Store
	OIDC    *auth.Client
	Logger  *slog.Logger
}

type handlers struct {
	store *store.Store
	log   *slog.Logger
}

// New builds the application: plugins, layouts, every page and action.
func New(d Deps) (*collage.App, error) {
	app, err := collage.New(&collage.Config{
		DevMode: d.DevMode,
		Logger:  d.Logger,
		Server:  collage.ServerConfig{Host: d.Host, Port: d.Port},
		Template: collage.TemplateConfig{
			FS: d.Files, Root: "templates", Extension: ".html",
		},
		Locale: collage.LocaleConfig{Default: d.Config.DefaultLocale, Supported: config.Locales},
		Cache:  collage.CacheConfig{Enabled: true, Type: "memory", DefaultTTL: 5 * time.Minute},
		Security: collage.SecurityConfig{
			CSRFKey: d.Config.CSRFKey,
		},
		// Every plugin here wraps the middleware added with app.Use below
		// (collage v0.38.0), so auth.Middleware can read the session.
		Plugins: []collage.Plugin{
			session.New(session.Options{Key: d.Config.SessionKey, Encrypt: true, MaxAge: 7 * 24 * 60 * 60}),
			i18n.New(i18n.Options{FS: d.Files, Dir: "locales", Strict: true}),
			validate.New(validate.Options{LocaleMessages: validationMessages}),
			flash.New(flash.Options{Key: d.Config.FlashKey}),
			live.New(),
			secure.New(secure.Options{CSP: contentSecurityPolicy(d.Config.OIDC.Issuer)}),
		},
	})
	if err != nil {
		return nil, err
	}

	notFound, serverError, authFailed := notFoundPage(), errorPage(), authFailedPage()
	h := &handlers{store: d.Store, log: d.Logger}
	pages := append([]*collage.Page{notFound, serverError, authFailed}, h.pages()...)
	for _, p := range pages {
		if err := app.RegisterPage(p); err != nil {
			return nil, fmt.Errorf("register page %q: %w", p.Name, err)
		}
	}
	if err := app.RegisterNotFoundPage(notFound); err != nil {
		return nil, err
	}
	if err := app.RegisterErrorPage(serverError); err != nil {
		return nil, err
	}

	signIn := auth.NewService(auth.Options{
		Client: d.OIDC, Users: d.Store, AdminEmails: d.Config.AdminEmails,
		DefaultLocale: d.Config.DefaultLocale, Locales: config.Locales,
		BaseURL: d.Config.BaseURL, CallbackPath: d.Config.OIDC.CallbackPath,
		FailedPage: authFailed, Logger: d.Logger,
	})
	for _, a := range signIn.Actions() {
		if err := app.RegisterAction(a); err != nil {
			return nil, fmt.Errorf("register action %q: %w", a.Name, err)
		}
	}
	if err := app.Use(signIn.Middleware); err != nil {
		return nil, err
	}

	static, err := fs.Sub(d.Files, "static")
	if err != nil {
		return nil, err
	}
	if err := app.Mount("/static/", static); err != nil {
		return nil, fmt.Errorf("mount static files: %w", err)
	}
	return app, nil
}

// pages is every page for signed-in readers.
func (h *handlers) pages() []*collage.Page {
	return []*collage.Page{h.homePage()}
}

// contentSecurityPolicy allows scripts only from this origin, and form posts
// to this origin and the provider: signing out redirects a form's post there.
func contentSecurityPolicy(issuer string) string {
	provider := ""
	if u, err := url.Parse(issuer); err == nil {
		provider = " " + u.Scheme + "://" + u.Host
	}
	return "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self'; img-src 'self' data:; " +
		"object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'" + provider
}

var validationMessages = map[string]map[string]string{
	"tr": {
		"required": "Bu alan zorunludur.",
		"maxLen":   "En fazla {max} karakter olabilir.",
		"email":    "Geçerli bir e-posta adresi girin.",
		"oneOf":    "Geçerli bir seçenek seçin.",
	},
}
```

- [ ] **Step 8: Testlerin geçtiğini gör**

Run: `go test ./internal/web/ -v -race`
Expected: PASS, `SKIP` yok (`TestHomeInBothLanguages` veritabanı ister)

`localeURL` çıktısı (`/en` ya da `/en/`) beklentiden farklıysa testin `href="/en` önekini kontrol ettiğini unutma; önek zaten ikisini de kabul ediyor.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat: app shell with plugins, layouts, TR/EN catalogs, CSP and error pages"
```

---

### Task 7: Takımlar — liste, oluşturma, üye yönetimi

**Files:**
- Create: `internal/authz/team.go`, `internal/authz/team_test.go`
- Create: `internal/web/pages_teams.go`, `templates/pages/teams.html`, `templates/pages/team.html`
- Modify: `internal/web/app.go` (`pages()`)
- Test: `internal/web/teams_test.go`

**Interfaces:**
- Consumes: `store.ListTeams`, `store.Team`, `store.Members`, `store.MemberRole`, `store.AddMember`, `store.SetRole`, `store.RemoveMember`, `store.UserByEmail`, `store.CreateTeam`, `store.ParseRole`, `store.Roles` (Task 3); `privatePage`, `paths`, `currentUser`, `handlers` (Task 6)
- Produces:
  ```go
  package authz
  type TeamAccess struct { CanView, CanManage bool }
  func Team(user store.User, role store.Role) TeamAccess
  ```

- [ ] **Step 1: `authz` testini yaz**

`internal/authz/team_test.go`:

```go
package authz_test

import (
	"testing"

	"kanban/internal/authz"
	"kanban/internal/store"
)

func TestTeam(t *testing.T) {
	admin := store.User{IsAdmin: true}
	user := store.User{}
	cases := []struct {
		name string
		user store.User
		role store.Role
		want authz.TeamAccess
	}{
		{"admin, not a member", admin, "", authz.TeamAccess{CanView: true, CanManage: true}},
		{"lead", user, store.RoleLead, authz.TeamAccess{CanView: true, CanManage: true}},
		{"member", user, store.RoleMember, authz.TeamAccess{CanView: true}},
		{"outsider", user, "", authz.TeamAccess{}},
	}
	for _, c := range cases {
		if got := authz.Team(c.user, c.role); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: `authz`'ı yaz ve geçtiğini gör**

`internal/authz/team.go`:

```go
// Package authz decides who may do what. It reads no database; callers hand
// it what they loaded.
package authz

import "kanban/internal/store"

// TeamAccess is what a user may do with one team.
type TeamAccess struct {
	CanView   bool // see the team and its members
	CanManage bool // add and remove members, change roles
}

// Team is user's access to a team in which they hold role ("" if none).
func Team(user store.User, role store.Role) TeamAccess {
	switch {
	case user.IsAdmin, role == store.RoleLead:
		return TeamAccess{CanView: true, CanManage: true}
	case role == store.RoleMember:
		return TeamAccess{CanView: true}
	default:
		return TeamAccess{}
	}
}
```

Run: `go test ./internal/authz/ -v` → PASS

- [ ] **Step 3: Sayfa testlerini yaz**

`internal/web/teams_test.go`:

```go
package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"kanban/internal/store"
)

func TestAdminCreatesATeam(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")

	res := admin.Submit("/teams", "/teams", url.Values{"op": {"create"}, "name": {"  Platform  "}})
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/teams/") {
		t.Fatalf("create = %d %q:\n%s", res.Status, res.Location(), res.Body)
	}
	page := admin.Get(res.Location())
	mustContain(t, page.Body, "<h1>Platform</h1>", "Platform oluşturuldu.")
	mustContain(t, admin.Get("/teams").Body, "Platform")
}

func TestCreatingATeamValidatesTheName(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	res := admin.Submit("/teams", "/teams", url.Values{"op": {"create"}, "name": {"   "}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("blank name = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Bu alan zorunludur.")

	res = admin.Submit("/en/teams", "/en/teams", url.Values{"op": {"create"}, "name": {""}})
	mustContain(t, res.Body, "This field is required.")
}

func TestOnlyAdminsCreateTeams(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")
	page := bob.Get("/teams")
	if strings.Contains(page.Body, `name="op" value="create"`) {
		t.Error("a non-admin is shown the create form")
	}
	res := bob.Submit("/teams", "/teams", url.Values{"op": {"create"}, "name": {"Sneaky"}})
	if res.Status != http.StatusForbidden {
		t.Fatalf("non-admin create = %d, want 403", res.Status)
	}
}

// teamWith creates a team as admin with lead and member, and returns its path.
func teamWith(t *testing.T, h *harness, lead, member string) string {
	t.Helper()
	ctx := context.Background()
	team, err := h.store.CreateTeam(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	if lead != "" {
		if err := h.store.AddMember(ctx, team.ID, h.user(lead).ID, store.RoleLead); err != nil {
			t.Fatal(err)
		}
	}
	if member != "" {
		if err := h.store.AddMember(ctx, team.ID, h.user(member).ID, store.RoleMember); err != nil {
			t.Fatal(err)
		}
	}
	return "/teams/" + strconv.FormatInt(team.ID, 10)
}

func TestLeadManagesMembers(t *testing.T) {
	h := newHarness(t, "")
	lead := h.signedIn("lead", "lead@example.com")
	member := h.signedIn("member", "member@example.com")
	h.signedIn("carol", "carol@example.com")
	path := teamWith(t, h, "lead@example.com", "member@example.com")

	res := lead.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"Carol@Example.com"}, "new_role": {"member"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("add = %d:\n%s", res.Status, res.Body)
	}
	mustContain(t, lead.Get(path).Body, "carol@example.com", "Carol takıma eklendi.")

	carolID := strconv.FormatInt(h.user("carol@example.com").ID, 10)
	res = lead.Submit(path, path, url.Values{"op": {"set_role"}, "user_id": {carolID}, "role": {"lead"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("set_role = %d", res.Status)
	}
	if role, _ := h.store.MemberRole(context.Background(), idOf(path), h.user("carol@example.com").ID); role != store.RoleLead {
		t.Errorf("carol's role = %q", role)
	}

	res = lead.Submit(path, path, url.Values{"op": {"remove_member"}, "user_id": {carolID}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("remove = %d", res.Status)
	}
	if strings.Contains(lead.Get(path).Body, "carol@example.com") {
		t.Error("carol is still listed")
	}

	// A member sees the team but may not change it.
	page := member.Get(path)
	if page.Status != http.StatusOK || strings.Contains(page.Body, `value="add_member"`) {
		t.Fatalf("member view = %d, manage form shown: %v", page.Status, strings.Contains(page.Body, `value="add_member"`))
	}
	res = member.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"carol@example.com"}, "new_role": {"lead"}})
	if res.Status != http.StatusForbidden {
		t.Fatalf("member add = %d, want 403", res.Status)
	}
}

func TestAddingAnUnknownEmailIsRefusedOnTheForm(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	path := teamWith(t, h, "", "")
	res := admin.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"ghost@example.com"}, "new_role": {"member"}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", res.Status)
	}
	mustContain(t, res.Body, "Bu e-postayla kayıtlı bir kullanıcı yok.", `value="ghost@example.com"`)

	res = admin.Submit(path, path, url.Values{"op": {"add_member"}, "new_email": {"x@example.com"}, "new_role": {"owner"}})
	if res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown role = %d, want 422", res.Status)
	}
}

func TestOutsidersCannotTellATeamExists(t *testing.T) {
	h := newHarness(t, "")
	h.signedIn("lead", "lead@example.com")
	outsider := h.signedIn("out", "out@example.com")
	path := teamWith(t, h, "lead@example.com", "")

	for _, p := range []string{path, "/teams/999999", "/teams/abc"} {
		if res := outsider.Get(p); res.Status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, res.Status)
		}
	}
	// The outsider needs a token from a page they can see.
	res := outsider.Submit("/teams", path, url.Values{"op": {"add_member"}, "new_email": {"out@example.com"}, "new_role": {"lead"}})
	if res.Status != http.StatusNotFound {
		t.Fatalf("outsider POST = %d, want 404", res.Status)
	}
	if _, err := h.store.MemberRole(context.Background(), idOf(path), h.user("out@example.com").ID); err == nil {
		t.Fatal("the outsider added themselves")
	}
}

func idOf(path string) int64 {
	id, _ := strconv.ParseInt(path[strings.LastIndex(path, "/")+1:], 10, 64)
	return id
}
```

- [ ] **Step 4: Testin başarısız olduğunu gör**

Run: `go test ./internal/web/ -run 'Team|Outsider|Lead' -v`
Expected: FAIL — `/teams` 404 döner (sayfa yok)

- [ ] **Step 5: Template'leri yaz**

`templates/pages/teams.html`:

```html
<h1>{{t "teams.title"}}</h1>
{{if .Teams}}
<table class="table">
  <thead>
    <tr><th>{{t "teams.name"}}</th><th>{{t "team.members"}}</th><th>{{t "team.role"}}</th></tr>
  </thead>
  <tbody>
    {{range .Teams}}
    <tr>
      <td><a href="{{pageURL "team" "id" .Team.ID}}">{{.Team.Name}}</a></td>
      <td>{{tn "teams.member_count" .MemberCount}}</td>
      <td>{{if .Role}}{{t (printf "team.roles.%s" .Role)}}{{end}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p class="muted">{{t "teams.empty"}}</p>
{{end}}

{{if .CanCreate}}
<form class="form" method="post" action="{{pageURL "teams"}}">
  {{csrfToken}}
  <input type="hidden" name="op" value="create">
  <label for="team-name">{{t "teams.name"}}</label>
  <input id="team-name" name="name" value="{{fieldValue "name"}}" required maxlength="100">
  {{with fieldError "name"}}<p class="error">{{.}}</p>{{end}}
  <button type="submit">{{t "teams.create"}}</button>
</form>
{{end}}
```

`templates/pages/team.html`:

```html
{{$self := pageURL "team" "id" .Team.ID}}
<h1>{{.Team.Name}}</h1>

<h2>{{t "team.members"}}</h2>
<table class="table">
  <thead>
    <tr><th>{{t "users.name"}}</th><th>{{t "users.email"}}</th><th>{{t "team.role"}}</th>{{if .CanManage}}<th></th>{{end}}</tr>
  </thead>
  <tbody>
    {{range .Members}}
    <tr>
      <td>{{.User.Name}}</td>
      <td>{{.User.Email}}</td>
      <td>
        {{if $.CanManage}}
        <form class="inline" method="post" action="{{$self}}">
          {{csrfToken}}
          <input type="hidden" name="op" value="set_role">
          <input type="hidden" name="user_id" value="{{.User.ID}}">
          {{$current := .Role}}
          <select name="role" aria-label="{{t "team.role"}}">
            {{range $.Roles}}<option value="{{.}}" {{if eq . $current}}selected{{end}}>{{t (printf "team.roles.%s" .)}}</option>{{end}}
          </select>
          <button type="submit">{{t "team.save"}}</button>
        </form>
        {{else}}
        {{t (printf "team.roles.%s" .Role)}}
        {{end}}
      </td>
      {{if $.CanManage}}
      <td>
        <form class="inline" method="post" action="{{$self}}">
          {{csrfToken}}
          <input type="hidden" name="op" value="remove_member">
          <input type="hidden" name="user_id" value="{{.User.ID}}">
          <button type="submit">{{t "team.remove"}}</button>
        </form>
      </td>
      {{end}}
    </tr>
    {{end}}
  </tbody>
</table>

{{if .CanManage}}
<h2>{{t "team.add"}}</h2>
<form class="form" method="post" action="{{$self}}">
  {{csrfToken}}
  <input type="hidden" name="op" value="add_member">
  <label for="new-email">{{t "team.email"}}</label>
  <input id="new-email" name="new_email" type="email" value="{{fieldValue "new_email"}}" required>
  {{with fieldError "new_email"}}<p class="error">{{.}}</p>{{end}}
  <label for="new-role">{{t "team.role"}}</label>
  <select id="new-role" name="new_role">
    {{range .Roles}}<option value="{{.}}" {{if eq . "member"}}selected{{end}}>{{t (printf "team.roles.%s" .)}}</option>{{end}}
  </select>
  {{with fieldError "new_role"}}<p class="error">{{.}}</p>{{end}}
  <button type="submit">{{t "team.add"}}</button>
</form>
{{end}}
```

> `{{if eq . "member"}}`: `.` bir `store.Role`, `"member"` ise `string`. `text/template`'in `eq`'i temel türü karşılaştırdığı için bu çalışır. Çalışmazsa `(printf "%s" .)` ile string'e çevir.

- [ ] **Step 6: `pages_teams.go`'yu yaz**

```go
package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/authz"
	"kanban/internal/store"
)

type teamsView struct {
	Teams     []store.TeamSummary
	CanCreate bool
}

type teamView struct {
	Team      store.Team
	Members   []store.Member
	Roles     []store.Role
	CanManage bool
}

func (h *handlers) teamsPage() *collage.Page {
	content := collage.NewFragment("teams-content", "pages/teams.html").
		WithDataHandler(collage.Load(h.loadTeams)).
		Required().
		Build()
	return paths(privatePage("teams", content), "/teams").
		WithAction(http.MethodPost, h.teamsPost).
		Dynamic().
		Build()
}

func (h *handlers) loadTeams(ctx context.Context, rc *collage.RenderContext) (teamsView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return teamsView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "teams.title"))
	teams, err := h.store.ListTeams(ctx, user)
	return teamsView{Teams: teams, CanCreate: user.IsAdmin}, err
}

func (h *handlers) teamsPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	if v.Value("op") != "create" {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if !user.IsAdmin {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v.Field("name").Required().MaxLen(100) // Required refuses white space alone
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	team, err := h.store.CreateTeam(ctx, strings.TrimSpace(v.Value("name")))
	if err != nil {
		return nil, err
	}
	flash.Add(rc, flash.Success, i18n.T(rc, "teams.created", "name", team.Name))
	return h.redirectToTeam(rc, team.ID)
}
```

Devamı, aynı dosyada:

```go
func (h *handlers) redirectToTeam(rc *collage.RenderContext, id int64) (*collage.ActionResult, error) {
	target, err := rc.URL("team", map[string]string{"id": strconv.FormatInt(id, 10)})
	if err != nil {
		return nil, err
	}
	return collage.SeeOther(target), nil
}

func (h *handlers) teamPage() *collage.Page {
	content := collage.NewFragment("team-content", "pages/team.html").
		WithDataHandler(collage.Load(h.loadTeam)).
		Required().
		Build()
	return paths(privatePage("team", content), "/teams/{id}").
		WithAction(http.MethodPost, h.teamPost).
		Dynamic().
		Build()
}

// teamFor loads the team in the URL and the signed-in user's access to it. A
// team the user may not see is reported as not found, so its existence does
// not leak (spec §6).
func (h *handlers) teamFor(ctx context.Context, rc *collage.RenderContext) (store.Team, authz.TeamAccess, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return store.Team{}, authz.TeamAccess{}, err
	}
	id, err := strconv.ParseInt(rc.Param("id"), 10, 64)
	if err != nil {
		return store.Team{}, authz.TeamAccess{}, fmt.Errorf("team %q: %w", rc.Param("id"), collage.ErrNotFound)
	}
	team, err := h.store.Team(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.Team{}, authz.TeamAccess{}, fmt.Errorf("team %d: %w", id, collage.ErrNotFound)
	}
	if err != nil {
		return store.Team{}, authz.TeamAccess{}, err
	}
	role, err := h.store.MemberRole(ctx, team.ID, user.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.Team{}, authz.TeamAccess{}, err
	}
	access := authz.Team(user, role)
	if !access.CanView {
		return store.Team{}, authz.TeamAccess{}, fmt.Errorf("team %d for user %d: %w", id, user.ID, collage.ErrNotFound)
	}
	return team, access, nil
}

func (h *handlers) loadTeam(ctx context.Context, rc *collage.RenderContext) (teamView, error) {
	team, access, err := h.teamFor(ctx, rc)
	if err != nil {
		return teamView{}, err
	}
	rc.HoistTitle(team.Name)
	members, err := h.store.Members(ctx, team.ID)
	return teamView{Team: team, Members: members, Roles: store.Roles, CanManage: access.CanManage}, err
}

func (h *handlers) teamPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	team, access, err := h.teamFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if !access.CanManage {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v := validate.Form(rc)
	switch v.Value("op") {
	case "add_member":
		return h.addMember(ctx, rc, v, team)
	case "set_role", "remove_member":
		return h.changeMember(ctx, rc, v, team)
	}
	return collage.NoContent(http.StatusBadRequest), nil
}

func (h *handlers) addMember(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, team store.Team) (*collage.ActionResult, error) {
	v.Field("new_email").Required().Email()
	v.Field("new_role").OneOf(string(store.RoleLead), string(store.RoleMember))
	var user store.User
	if v.Valid() {
		var err error
		user, err = h.store.UserByEmail(ctx, v.Value("new_email"))
		switch {
		case errors.Is(err, store.ErrNotFound):
			v.Fail("new_email", i18n.T(rc, "team.user_not_found"))
		case errors.Is(err, store.ErrAmbiguousEmail):
			v.Fail("new_email", i18n.T(rc, "team.user_ambiguous"))
		case err != nil:
			return nil, err
		}
	}
	if !v.Valid() {
		return validate.Refuse(rc, v, rc.Page), nil
	}
	role, _ := store.ParseRole(v.Value("new_role"))
	if err := h.store.AddMember(ctx, team.ID, user.ID, role); err != nil {
		return nil, err
	}
	flash.Add(rc, flash.Success, i18n.T(rc, "team.added", "name", user.Name))
	return h.redirectToTeam(rc, team.ID)
}

// changeMember changes the role of, or removes, a member of team. The user id
// comes from the form, so it is checked against this team: an id from another
// team is not found here (spec §6, IDOR).
func (h *handlers) changeMember(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, team store.Team) (*collage.ActionResult, error) {
	userID, err := strconv.ParseInt(v.Value("user_id"), 10, 64)
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	target, err := h.store.UserByID(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}

	if v.Value("op") == "remove_member" {
		err = h.store.RemoveMember(ctx, team.ID, userID)
		if err == nil {
			flash.Add(rc, flash.Success, i18n.T(rc, "team.removed", "name", target.Name))
		}
	} else {
		role, ok := store.ParseRole(v.Value("role"))
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.SetRole(ctx, team.ID, userID, role)
		if err == nil {
			flash.Add(rc, flash.Success, i18n.T(rc, "team.role_changed", "name", target.Name))
		}
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.redirectToTeam(rc, team.ID)
}
```

`app.go`'daki `pages()`'i güncelle:

```go
func (h *handlers) pages() []*collage.Page {
	return []*collage.Page{h.homePage(), h.teamsPage(), h.teamPage()}
}
```

- [ ] **Step 7: Testlerin geçtiğini gör**

Run: `go test ./internal/web/ ./internal/authz/ -v -race`
Expected: PASS, `SKIP` yok

`rc.Param("id")` bir `WithAction` action'ında boş geliyorsa bu, collage'ın "action sayfanın URL'sini devralır" davranışından bir sapmadır: issue yaz ve **dur**.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: teams list, creation and member management with 404 for outsiders"
```

---

### Task 8: Admin — kullanıcı yönetimi

**Files:**
- Create: `internal/web/pages_admin.go`, `templates/pages/admin_users.html`
- Modify: `internal/web/app.go` (`pages()`)
- Test: `internal/web/admin_test.go`

**Interfaces:**
- Consumes: `store.Users`, `store.SetAdmin`, `store.SetDisabled`, `store.ErrLastAdmin`, `store.UserByID` (Task 3); `privatePage`, `paths`, `currentUser` (Task 6)

- [ ] **Step 1: Başarısız testi yaz**

`internal/web/admin_test.go`:

```go
package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestADMIN_EMAILSMakesTheFirstAdmin(t *testing.T) {
	h := newHarness(t, " Admin@Example.com , ")
	admin := h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")

	if !h.user("admin@example.com").IsAdmin || h.user("bob@example.com").IsAdmin {
		t.Fatal("ADMIN_EMAILS granted the wrong users")
	}
	mustContain(t, admin.Get("/").Body, `href="/admin/users"`)
	if strings.Contains(bob.Get("/").Body, `href="/admin/users"`) {
		t.Error("a non-admin sees the users link")
	}
	if res := bob.Get("/admin/users"); res.Status != http.StatusNotFound {
		t.Errorf("non-admin GET /admin/users = %d, want 404", res.Status)
	}
	res := bob.Submit("/", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {"1"}, "value": {"1"}})
	if res.Status != http.StatusNotFound {
		t.Errorf("non-admin POST /admin/users = %d, want 404", res.Status)
	}
}

func TestTheOnlyAdminCannotStepDown(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	id := strconv.FormatInt(h.user("admin@example.com").ID, 10)

	res := admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {id}, "value": {"0"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("status = %d", res.Status)
	}
	mustContain(t, admin.Get("/admin/users").Body, "En az bir aktif admin kalmalı.")
	if !h.user("admin@example.com").IsAdmin {
		t.Fatal("the only admin was demoted")
	}

	res = admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_disabled"}, "user_id": {id}, "value": {"1"}})
	mustContain(t, admin.Get("/admin/users").Body, "Kendi hesabınızı devre dışı bırakamazsınız.")
}

func TestDisablingAUserSignsThemOut(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")
	if res := bob.Get("/"); res.Status != http.StatusOK {
		t.Fatalf("bob GET / = %d", res.Status)
	}
	id := strconv.FormatInt(h.user("bob@example.com").ID, 10)

	res := admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_disabled"}, "user_id": {id}, "value": {"1"}})
	if res.Status != http.StatusSeeOther {
		t.Fatalf("disable = %d", res.Status)
	}
	res = bob.Get("/")
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/login") {
		t.Fatalf("disabled bob GET / = %d %q, want a redirect to /login", res.Status, res.Location())
	}

	admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_disabled"}, "user_id": {id}, "value": {"0"}})
	again := h.signedIn("bob", "bob@example.com")
	if res := again.Get("/"); res.Status != http.StatusOK {
		t.Fatalf("re-enabled bob GET / = %d", res.Status)
	}
}

func TestAdminPromotesAnother(t *testing.T) {
	h := newHarness(t, "admin@example.com")
	admin := h.signedIn("admin", "admin@example.com")
	bob := h.signedIn("bob", "bob@example.com")
	id := strconv.FormatInt(h.user("bob@example.com").ID, 10)

	admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {id}, "value": {"1"}})
	if res := bob.Get("/admin/users"); res.Status != http.StatusOK {
		t.Fatalf("promoted bob GET /admin/users = %d", res.Status)
	}
	if res := admin.Submit("/admin/users", "/admin/users", url.Values{"op": {"set_admin"}, "user_id": {"999999"}, "value": {"1"}}); res.Status != http.StatusNotFound {
		t.Errorf("unknown user = %d, want 404", res.Status)
	}
}
```

- [ ] **Step 2: Testin başarısız olduğunu gör**

Run: `go test ./internal/web/ -run 'ADMIN|Admin|Disabling' -v`
Expected: FAIL — `/admin/users` 404 (admin için de)

- [ ] **Step 3: Template'i yaz**

`templates/pages/admin_users.html`:

```html
{{$self := pageURL "admin-users"}}
<h1>{{t "users.title"}}</h1>
<table class="table">
  <thead>
    <tr><th>{{t "users.name"}}</th><th>{{t "users.email"}}</th><th>{{t "users.admin"}}</th><th>{{t "users.status"}}</th></tr>
  </thead>
  <tbody>
    {{range .Users}}
    <tr>
      <td>{{.Name}} {{if eq .ID $.Me}}<span class="muted">{{t "users.you"}}</span>{{end}}</td>
      <td>{{.Email}}</td>
      <td>
        <form class="inline" method="post" action="{{$self}}">
          {{csrfToken}}
          <input type="hidden" name="op" value="set_admin">
          <input type="hidden" name="user_id" value="{{.ID}}">
          {{if .IsAdmin}}
          <input type="hidden" name="value" value="0"><button type="submit">{{t "users.revoke_admin"}}</button>
          {{else}}
          <input type="hidden" name="value" value="1"><button type="submit">{{t "users.make_admin"}}</button>
          {{end}}
        </form>
      </td>
      <td>
        {{if .Disabled}}{{t "users.disabled"}}{{else}}{{t "users.active"}}{{end}}
        <form class="inline" method="post" action="{{$self}}">
          {{csrfToken}}
          <input type="hidden" name="op" value="set_disabled">
          <input type="hidden" name="user_id" value="{{.ID}}">
          {{if .Disabled}}
          <input type="hidden" name="value" value="0"><button type="submit">{{t "users.enable"}}</button>
          {{else}}
          <input type="hidden" name="value" value="1"><button type="submit">{{t "users.disable"}}</button>
          {{end}}
        </form>
      </td>
    </tr>
    {{end}}
  </tbody>
</table>
```

- [ ] **Step 4: `pages_admin.go`'yu yaz**

```go
package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type adminUsersView struct {
	Users []store.User
	Me    int64
}

func (h *handlers) adminUsersPage() *collage.Page {
	content := collage.NewFragment("admin-users-content", "pages/admin_users.html").
		WithDataHandler(collage.Load(h.loadAdminUsers)).
		Required().
		Build()
	return paths(privatePage("admin-users", content), "/admin/users").
		WithAction(http.MethodPost, h.adminUsersPost).
		Dynamic().
		Build()
}

// currentAdmin is the signed-in user if they are an admin; anyone else is told
// the page does not exist.
func currentAdmin(ctx context.Context) (store.User, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return store.User{}, err
	}
	if !user.IsAdmin {
		return store.User{}, fmt.Errorf("admin page for user %d: %w", user.ID, collage.ErrNotFound)
	}
	return user, nil
}

func (h *handlers) loadAdminUsers(ctx context.Context, rc *collage.RenderContext) (adminUsersView, error) {
	admin, err := currentAdmin(ctx)
	if err != nil {
		return adminUsersView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "users.title"))
	users, err := h.store.Users(ctx)
	return adminUsersView{Users: users, Me: admin.ID}, err
}

func (h *handlers) adminUsersPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	admin, err := currentAdmin(ctx)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	userID, err := strconv.ParseInt(v.Value("user_id"), 10, 64)
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	on := v.Value("value") == "1"

	switch v.Value("op") {
	case "set_admin":
		err = h.store.SetAdmin(ctx, userID, on)
	case "set_disabled":
		if on && userID == admin.ID {
			flash.Add(rc, flash.Error, i18n.T(rc, "users.self_disable"))
			return h.redirectTo(rc, "admin-users")
		}
		err = h.store.SetDisabled(ctx, userID, on)
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return collage.NoContent(http.StatusNotFound), nil
	case errors.Is(err, store.ErrLastAdmin):
		flash.Add(rc, flash.Error, i18n.T(rc, "users.last_admin"))
	case err != nil:
		return nil, err
	default:
		flash.Add(rc, flash.Success, i18n.T(rc, "users.saved"))
	}
	return h.redirectTo(rc, "admin-users")
}

func (h *handlers) redirectTo(rc *collage.RenderContext, page string) (*collage.ActionResult, error) {
	target, err := rc.URL(page, nil)
	if err != nil {
		return nil, err
	}
	return collage.SeeOther(target), nil
}
```

`app.go`'daki `pages()`'i güncelle:

```go
func (h *handlers) pages() []*collage.Page {
	return []*collage.Page{h.homePage(), h.teamsPage(), h.teamPage(), h.adminUsersPage()}
}
```

- [ ] **Step 5: Testlerin geçtiğini gör**

Run: `go test ./... -race`
Expected: PASS, `SKIP` yok

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: admin user management with last-admin and self-disable protection"
```

---

### Task 9: `main.go`, ortam ve çalıştırma

**Files:**
- Modify (baştan yaz): `main.go`
- Create: `.env.example`
- Modify: `README.md` (sonuna "Geliştirme" bölümü), `.gitignore` (`.env.development`)

**Interfaces:**
- Consumes: `config.Load` (Task 1), `db.Connect`, `db.Migrate` (Task 2), `store.New` (Task 3), `auth.NewClient` (Task 4), `web.New`, `web.Deps` (Task 6)

- [ ] **Step 1: `main.go`'yu yaz**

```go
// Command kanban serves the kanban application.
//
// "collage dev" builds this program and runs it with COLLAGE_DEV=1, the
// variables in .env.development, and HOST and PORT set; this file honours all
// three. Every page is private, so "collage export" has nothing to write and
// this program does not implement -collage-build.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"strconv"
	"time"

	"kanban/internal/auth"
	"kanban/internal/config"
	"kanban/internal/db"
	"kanban/internal/store"
	"kanban/internal/web"
)

// The binary carries its templates, static files and catalogs.
//
//go:embed all:templates all:static locales
var embedded embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatalf("kanban: %v", err)
	}
}

func run() error {
	devMode := os.Getenv("COLLAGE_DEV") == "1"
	logger := slog.Default()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("configuration:\n%w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	client, err := auth.NewClient(ctx, auth.ClientConfig{
		Issuer: cfg.OIDC.Issuer, ClientID: cfg.OIDC.ClientID, ClientSecret: cfg.OIDC.ClientSecret,
		RedirectURL: cfg.OIDC.RedirectURL, Scopes: cfg.OIDC.Scopes,
	})
	if err != nil {
		return err
	}

	files, err := appFiles(devMode)
	if err != nil {
		return err
	}
	app, err := web.New(web.Deps{
		Files: files, DevMode: devMode,
		Host: envString("HOST", "localhost"), Port: envInt("PORT", 6060),
		Config: cfg, Store: store.New(pool), OIDC: client, Logger: logger,
	})
	if err != nil {
		return err
	}
	return app.ListenAndServe()
}

// appFiles is the embedded copy, or in development the working directory, so
// an edited template or catalog shows on the next request.
func appFiles(devMode bool) (fs.FS, error) {
	if devMode {
		if root, err := os.OpenRoot("."); err == nil {
			return root.FS(), nil
		}
	}
	return embedded, nil
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
```

- [ ] **Step 2: `.env.example`'ı yaz**

```
# cp .env.example .env.development, then fill in the client and the keys.
# The redirect URL is the one registered with the provider, and must be on BASE_URL.
BASE_URL=https://kanboard-test.unoghub.org
DATABASE_URL=postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable
DEFAULT_LOCALE=tr
OIDC_ISSUER=https://workspace.unoghub.org/application/o/kanboard-test-uygulamasi/
OIDC_REDIRECT_URL=https://kanboard-test.unoghub.org/auth/openid/authentik
OIDC_CLIENT_ID=
OIDC_CLIENT_SECRET=
OIDC_SCOPES=openid email profile
ADMIN_EMAILS=
# openssl rand -hex 32, each different
SESSION_KEY=
CSRF_KEY=
FLASH_KEY=
```

`.gitignore`'a ekle:

```
.env.development
.env
```

- [ ] **Step 3: README'ye "Geliştirme" bölümü ekle**

````markdown
## Geliştirme

```bash
docker compose up -d                 # PostgreSQL, localhost:55432
cp .env.example .env.development      # OIDC ve anahtarları doldur
collage dev                           # http://localhost:6060

export KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable'
go test ./... -race                   # veritabanı testleri bu değişken yoksa atlanır
```

Callback adresi IdP'de kayıtlı redirect URI'dir ve `OIDC_REDIRECT_URL` ile verilir; `BASE_URL` ile aynı origin'de olmalıdır. Çıkış sonrası dönüş adresi: `<BASE_URL>/login`. Kayıtlı adres `localhost` değilse gerçek giriş yerelde denenemez; testler sahte issuer kullanır.
````

- [ ] **Step 4: Açılış doğrulamasını elle dene**

```bash
go build -o /tmp/kanban . && env -i PATH="$PATH" /tmp/kanban; echo "exit=$?"
```
Expected: sıfırdan farklı çıkış kodu. Mesajda `BASE_URL`, `DATABASE_URL`, `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL`, `SESSION_KEY`, `CSRF_KEY`, `FLASH_KEY` isimleri görünmeli.

```bash
K=$(openssl rand -hex 32); env -i PATH="$PATH" BASE_URL=http://localhost:6060 \
  DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' \
  OIDC_ISSUER=http://127.0.0.1:1/ OIDC_REDIRECT_URL=http://localhost:6060/auth/openid/authentik OIDC_CLIENT_ID=x OIDC_CLIENT_SECRET=y \
  SESSION_KEY=$K CSRF_KEY=$K FLASH_KEY=$K /tmp/kanban; echo "exit=$?"
```
Expected: `auth: discovery at http://127.0.0.1:1/: …` ile sıfırdan farklı çıkış. IdP'ye ulaşılamazsa uygulama başlamaz.

- [ ] **Step 5: Tüm testler, vet, yarış denetimi**

Run: `go vet ./... && go test ./... -race -count=1`
Expected: PASS, `SKIP` yok

`grep -rn --include='*.go' -E '\bany\b|interface\{\}' .` çıktısında `any` ya da `interface{}` **yazılmış** bir satır olmamalı. Yorumlardaki "any" kelimesi sayılmaz; kod satırı varsa düzelt.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: process entry point, example environment and dev instructions"
```

---

## Kabul (spec §14, Aşama 1)

| Kriter | Nasıl doğrulanır |
| --- | --- |
| İki IdP'ye bağlanan iki kurulum girişe izin veriyor | **Elle, kullanıcı tarafından** (Authentik test kurulumu: `kanboard-test.unoghub.org`, ve kendi IdP'n). Testlerde sahte issuer ile Task 5; ad boş geldiğinde `TestExchangeFallsBackToThePreferredUsername` |
| `ADMIN_EMAILS` ilk admini veriyor | `TestADMIN_EMAILSMakesTheFirstAdmin`, `TestAdminEmailsGrantAdminOnlyOnFirstSignIn` |
| Admin takım oluşturup üye ekleyebiliyor | `TestAdminCreatesATeam`, `TestLeadManagesMembers` |
| Özel sayfalar Dynamic | `TestEveryGuardedPageIsDynamic` |
| OIDC sahte issuer'a karşı test ediliyor; `nonce`, `state`, `aud` hataları reddediliyor | `TestCallbackRefusals` (wrong state / wrong nonce / wrong audience / replay / provider error / no flow), `TestExchangeRefusesTamperedTokens` |
