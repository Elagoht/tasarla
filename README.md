# Kanban

Birkaç takımın günlük iş takibi için kullandığı, kuralları board başına yapılandırılabilen bir kanban uygulaması. [collage](https://collage.furkanbaytekin.dev/) ile sunucu tarafında render edilir. İstemcide yalnızca sürükle-bırak ve kart diyaloğu için küçük bir JS katmanı vardır. Tasarım: [`kanban-spec.md`](kanban-spec.md). Aşama planları: [`docs/superpowers/plans/`](docs/superpowers/plans/).

## Neler var

- **Giriş:** OIDC (authorization code + PKCE). Kullanıcı `(issuer, sub)` ile tanınır. `ADMIN_EMAILS` ilk admini belirler; adminlik `/admin/users` üzerinden yönetilir.
- **Takımlar ve board'lar:** takım lead'leri board açar. Board'a erişim takım üyeliğiyle verilir, takım dışındakiler 404 görür.
- **Kartlar:** başlık, açıklama, atanan kişi, tahmin, son tarih, öncelik, etiketler, kontrol listesi, bağımlılıklar (döngü engellenir), arşiv.
- **Taşıma:**
  - SortableJS ile sürükle-bırak; değişiklik açık board'lara collage-live ile anında itilir.
  - Bayat bir kartı taşımak 409 döner.
  - JS olmadan kart sayfasındaki "kolona taşı" formu kullanılabilir.
- **Kural motoru** (`internal/rules`, saf Go):
  - Kurallar: geçişler, taşıma yetkileri, giriş ve çıkış koşulları, kolon WIP'i, kişi WIP'i.
  - Kural çiğneyen bir istek, ihlallerin hepsi kullanıcının dilinde listelenerek 422 ile reddedilir.
- **İşbirliği:** yorumlar ve `@` ile anma, kart ve board düzeyinde activity akışı, dosya ekleri (en fazla 5 MB; SVG her zaman indirilir).
- **Bildirimler:**
  - Uygulama içi bildirimler ve canlı rozet.
  - Outbox üzerinden e-posta, alıcının dilinde.
  - Son tarih hatırlatmaları.
  - `/me/settings` üzerinden dil ve e-posta tercihleri.
- **Dil:** TR + EN. Dil yalnızca URL'den belirlenir: `/…` varsayılan dil, `/en/…` İngilizce.

## Geliştirme

```bash
docker compose up -d                 # PostgreSQL 17, localhost:55432
cp .env.example .env.development      # OIDC istemcisini ve anahtarları doldur
collage dev                           # http://localhost:6060

export KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable'
go test ./... -race                   # veritabanı testleri bu değişken yoksa atlanır
```

- **Gerçek giriş yerelde denenemeyebilir.** Callback adresi IdP'de kayıtlı redirect URI'dir, `OIDC_REDIRECT_URL` ile verilir ve `BASE_URL` ile aynı origin'de olmalıdır. Kayıtlı adres `localhost` değilse gerçek giriş yerelde yapılamaz; testler sahte bir issuer kullanır (`internal/auth/authtest`).
- **Çıkış:** sonrasında `<BASE_URL>/login` adresine dönülür.
- **Migration'lar** binary'ye gömülüdür ve açılışta çalışır.

## Ortam değişkenleri

| Değişken | Zorunlu | Açıklama |
| --- | --- | --- |
| `BASE_URL` | evet | Sitenin kendi origin'i, ör. `https://kanboard-test.unoghub.org` |
| `DATABASE_URL` | evet | PostgreSQL bağlantısı |
| `DEFAULT_LOCALE` | hayır | `tr` (varsayılan) ya da `en` |
| `OIDC_ISSUER` | evet | IdP'nin issuer URL'si, sondaki `/` dahil |
| `OIDC_REDIRECT_URL` | evet | IdP'de kayıtlı redirect URI; `BASE_URL` üzerinde olmalı |
| `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` | evet | IdP istemcisi |
| `OIDC_SCOPES` | hayır | Varsayılan `openid email profile` |
| `ADMIN_EMAILS` | hayır | İlk girişte admin olacak e-postalar, virgülle ayrılmış |
| `SESSION_KEY`, `CSRF_KEY`, `FLASH_KEY` | evet | Her biri en az 32 bayt, hex (`openssl rand -hex 32`) |
| `ATTACHMENTS_DIR` | evet | Dosya eklerinin dizini |
| `SMTP_HOST`, `SMTP_FROM` | evet | E-posta sunucusu ve gönderen |
| `SMTP_PORT` | hayır | Varsayılan `587` |
| `SMTP_USER`, `SMTP_PASSWORD` | hayır | Yalnız TLS üzerinden ya da localhost'ta kullanılır |

Uygulama, eksik ya da hatalı her değeri adıyla raporlar ve başlamaz. IdP'ye ulaşılamazsa da başlamaz.

## Yapı

```
main.go                    süreç: config → db → migrate → OIDC → web → worker + scheduler
internal/config            ortam değişkenleri
internal/db                bağlantı, gömülü migration'lar, test veritabanı (dbtest)
internal/store             PostgreSQL erişimi; taşıma, kural ve activity transaction'ları
internal/rules             kural motoru (saf)
internal/authz             takım ve board yetkileri (saf)
internal/auth              OIDC istemcisi, giriş/çıkış, oturumdaki kullanıcı
internal/web               sayfalar, fragment'lar, action'lar, dosya indirme
internal/notify            bildirimler, outbox worker'ı, son tarih zamanlayıcısı
internal/mail              SMTP gönderimi
internal/files             dosya eklerinin diski
templates/, static/, locales/
```

## Framework sorunları

collage ya da eklentilerinde karşılaşılan sorunlar [`framework-issues/`](framework-issues/) altında. 001 ve 002 çözüldü. 003 (fragment adresinin `ErrNotFound`'da 500 dönmesi) açık; etkisi o dosyada anlatılıyor.
